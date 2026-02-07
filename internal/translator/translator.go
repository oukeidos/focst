package translator

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"time"

	"github.com/oukeidos/focst/internal/apperrors"
	"github.com/oukeidos/focst/internal/chunker"
	"github.com/oukeidos/focst/internal/gemini"
	"github.com/oukeidos/focst/internal/language"
	"github.com/oukeidos/focst/internal/logger"
	"github.com/oukeidos/focst/internal/srt"
	"github.com/rivo/uniseg"
)

// normalizeLines splits text containing newlines into separate lines.
// Handles both literal "\n" strings and actual newline characters.
func normalizeLines(lines ...string) []string {
	var result []string
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Replace literal \n with actual newline
		normalized := strings.ReplaceAll(line, "\\n", "\n")
		// Split by actual newlines
		parts := strings.Split(normalized, "\n")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
	}
	return result
}

// GetSystemPrompt generates a language-specific system prompt.
func GetSystemPrompt(sourceName, targetName string, cpl int, enforceCPL bool) string {
	lineGuidance := "" +
		"- The output MUST be a JSON object with a 'translations' field, containing an array of objects.\n" +
		"- Each object in the array must have:\n" +
		"  - 'id': The ID from the input segment.\n" +
		"  - 'line1': The main subtitle line.\n" +
		"  - 'line2': Optional; use only when a second line improves readability or a natural line break is needed. Do not repeat line1.\n" +
		"- Respond ONLY with the JSON object.\n"
	if enforceCPL {
		lineGuidance = fmt.Sprintf(""+
			"- The output MUST be a JSON object with a 'translations' field, containing an array of objects.\n"+
			"- Each object in the array must have:\n"+
			"  - 'id': The ID from the input segment.\n"+
			"  - 'line1': Ensure the translated text is **strictly %d characters or less (including spaces)**.\n"+
			"  - 'line2': Use this if the text exceeds the character limit for a single line or if a natural line break is appropriate. If provided, this line must also be **strictly %d characters or less (including spaces)**.\n"+
			"- Respond ONLY with the JSON object.\n", cpl, cpl)
	}

	return fmt.Sprintf(`You are a professional %s to %s translator specializing in subtitles.
Translate the provided %s subtitle segments into %s.

1. Input Structure:
- The input is provided in JSON format with 'context_before', 'target', and 'context_after'.
- 'target': Contains the segments you must translate. 
- 'context_before' and 'context_after': Provided for context only. Do NOT translate them or include them in the output.

2. Output Structure:
%s
3. Rules:
- Maintain the original tone and context.
- Follow **Standard Cinematic Subtitle Punctuation** for %s.
- Write ONLY the %s translation; do not include the %s source text.`,
		sourceName, targetName, sourceName, targetName, lineGuidance, targetName, targetName, sourceName)
}

// Translator orchestrates the translation process.
type Translator struct {
	geminiClient gemini.Translator
	chunkSize    int
	contextSize  int
	concurrency  int
	validateCPL  bool
	promptCPL    bool
	usage        gemini.UsageMetadata
	usageMu      sync.Mutex
	namesMapping map[string]string
	srcLang      language.Language
	tgtLang      language.Language
}

// NewTranslator creates a new Translator instance.
func NewTranslator(client gemini.Translator, chunkSize, contextSize, concurrency int, validateCPL bool, srcLang, tgtLang language.Language) (*Translator, error) {
	if chunkSize <= 0 {
		return nil, fmt.Errorf("chunkSize must be greater than 0, got %d", chunkSize)
	}
	if concurrency <= 0 {
		return nil, fmt.Errorf("concurrency must be greater than 0, got %d", concurrency)
	}
	return &Translator{
		geminiClient: client,
		chunkSize:    chunkSize,
		contextSize:  contextSize,
		concurrency:  concurrency,
		validateCPL:  validateCPL,
		promptCPL:    true,
		srcLang:      srcLang,
		tgtLang:      tgtLang,
	}, nil
}

// SetPromptCPL enables/disables CPL constraints in the prompt.
func (t *Translator) SetPromptCPL(enabled bool) {
	t.promptCPL = enabled
}

// SetNamesMapping sets the character name dictionary.
func (t *Translator) SetNamesMapping(mapping map[string]string) {
	t.namesMapping = mapping
}

// TranslationState represents the current state of a chunk translation.
type TranslationState int

const (
	StateStarted TranslationState = iota
	StateInProgress
	StateCompleted
	StateCanceled
)

var defaultQPS = 3
var defaultRampUp = 2 * time.Second

// TranslationProgress represents the current state of the translation process.
type TranslationProgress struct {
	ChunkIndex  int
	TotalChunks int
	Attempt     int
	State       TranslationState
	Error       error
}

func (t *Translator) setSystemInstruction() {
	prompt := GetSystemPrompt(t.srcLang.Name, t.tgtLang.Name, t.tgtLang.DefaultCPL, t.promptCPL)

	// Inject Names Mapping if present
	if len(t.namesMapping) > 0 {
		mappingStr := "\n\nCRITICAL: The following character names MUST be translated as specified:\n"
		for src, tgt := range t.namesMapping {
			mappingStr += fmt.Sprintf("- %s -> %s\n", src, tgt)
		}
		prompt += mappingStr
	}

	if sc, ok := t.geminiClient.(interface{ SetSystemInstruction(string) }); ok {
		sc.SetSystemInstruction(prompt)
	}
}

func (t *Translator) translateEngine(ctx context.Context, segments []srt.Segment, chunkIndices []int, onProgress func(TranslationProgress)) ([]chunker.Chunk, [][]srt.Segment, []bool, error) {
	t.setSystemInstruction()

	chunks := chunker.SplitIntoChunks(segments, t.chunkSize, t.contextSize)
	translatedChunks := make([][]srt.Segment, len(chunks))
	failedMarks := make([]bool, len(chunks))
	processed := make([]bool, len(chunks))

	toTranslate := make(map[int]bool, len(chunks))
	if chunkIndices == nil {
		for i := range chunks {
			toTranslate[i] = true
		}
	} else {
		for _, idx := range chunkIndices {
			if idx >= 0 && idx < len(chunks) {
				toTranslate[idx] = true
			}
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	rateCh, stopRate := newRateLimiter(defaultQPS)
	defer stopRate()

	jobs := make(chan int, len(chunks))
	for i := range chunks {
		if toTranslate[i] {
			jobs <- i
		}
	}
	close(jobs)

	for w := 0; w < t.concurrency; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			if delay := rampDelay(worker, t.concurrency, defaultRampUp); delay > 0 {
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}
			for i := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if rateCh != nil {
					select {
					case <-ctx.Done():
						return
					case <-rateCh:
					}
				}
				chunk := chunks[i]

				var resp *gemini.ResponseData
				var err error
				const maxAttempts = 3
				attemptsUsed := 0

				for attempt := 1; attempt <= maxAttempts; attempt++ {
					attemptsUsed = attempt
					if onProgress != nil {
						state := StateStarted
						if attempt > 1 {
							state = StateInProgress
						}
						onProgress(TranslationProgress{
							ChunkIndex:  i,
							TotalChunks: len(chunks),
							Attempt:     attempt,
							State:       state,
							Error:       err,
						})
					}

					req := t.prepareRequest(chunk)
					resp, err = t.geminiClient.Translate(ctx, req)
					if err == nil {
						t.usageMu.Lock()
						t.usage.PromptTokenCount += resp.Usage.PromptTokenCount
						t.usage.CandidatesTokenCount += resp.Usage.CandidatesTokenCount
						t.usage.TotalTokenCount += resp.Usage.TotalTokenCount
						t.usageMu.Unlock()

						if t.validateCPL {
							err = t.validateResponse(resp)
							if err != nil {
								err = apperrors.Validation(err)
							}
						}

						if err == nil {
							var translated []srt.Segment
							translated, err = t.mergeResults(chunk.Target, resp)
							if err != nil {
								err = apperrors.Validation(err)
							}
							if err == nil {
								mu.Lock()
								translatedChunks[i] = translated
								processed[i] = true
								mu.Unlock()
							}
						}
					}

					if err == nil {
						if onProgress != nil {
							onProgress(TranslationProgress{
								ChunkIndex:  i,
								TotalChunks: len(chunks),
								Attempt:     attempt,
								State:       StateCompleted,
							})
						}
						break
					}

					retry, backoff := retryDecision(ctx, err, attempt, maxAttempts)
					if !retry {
						break
					}
					select {
					case <-ctx.Done():
						return
					case <-time.After(backoff):
					}
				}

				if err != nil {
					mu.Lock()
					failedMarks[i] = true
					mu.Unlock()
					if attemptsUsed >= maxAttempts && apperrors.IsRetryable(err) {
						logger.Error("Chunk failed after maximum retries", "index", i, "attempts", attemptsUsed, "error", err)
					} else {
						logger.Error("Chunk failed without retry", "index", i, "attempts", attemptsUsed, "error", err)
					}
				}
			}
		}(w)
	}

	wg.Wait()
	if ctx.Err() != nil && onProgress != nil {
		onProgress(TranslationProgress{
			ChunkIndex:  -1,
			TotalChunks: len(chunks),
			State:       StateCanceled,
			Error:       ctx.Err(),
		})
	}
	for idx := range toTranslate {
		if idx >= 0 && idx < len(processed) && !processed[idx] {
			failedMarks[idx] = true
		}
	}

	return chunks, translatedChunks, failedMarks, nil
}

// TranslateSRT translates a slice of SRT segments with retries and concurrency.
func (t *Translator) TranslateSRT(ctx context.Context, segments []srt.Segment, onProgress func(TranslationProgress)) ([]srt.Segment, []int, error) {
	chunks, translatedChunks, failedMarks, err := t.translateEngine(ctx, segments, nil, onProgress)
	if err != nil {
		return nil, nil, err
	}
	for i, tc := range translatedChunks {
		if tc == nil {
			failedMarks[i] = true
			translatedChunks[i] = chunks[i].Target
		}
	}
	var failedChunkIndices []int
	for i, failed := range failedMarks {
		if failed {
			failedChunkIndices = append(failedChunkIndices, i)
		}
	}

	var allTranslated []srt.Segment
	for _, tc := range translatedChunks {
		allTranslated = append(allTranslated, tc...)
	}

	return allTranslated, failedChunkIndices, nil
}

// TranslateChunks translates a list of specific chunks concurrently.
func (t *Translator) TranslateChunks(ctx context.Context, segments []srt.Segment, chunkIndices []int, onProgress func(TranslationProgress)) ([]srt.Segment, []int, error) {
	_, translatedChunks, failedMarks, err := t.translateEngine(ctx, segments, chunkIndices, onProgress)
	if err != nil {
		return nil, nil, err
	}

	translatedSegments := make([]srt.Segment, len(segments))
	copy(translatedSegments, segments)
	for i, translated := range translatedChunks {
		if translated == nil {
			continue
		}
		startIdx := i * t.chunkSize
		for j, seg := range translated {
			if startIdx+j < len(translatedSegments) {
				translatedSegments[startIdx+j] = seg
			}
		}
	}
	var failedChunkIndices []int
	for i, failed := range failedMarks {
		if failed {
			failedChunkIndices = append(failedChunkIndices, i)
		}
	}
	return translatedSegments, failedChunkIndices, nil
}

func (t *Translator) prepareRequest(chunk chunker.Chunk) gemini.RequestData {
	return gemini.RequestData{
		ContextBefore: toSegmentData(chunk.Context.Before),
		Target:        toSegmentData(chunk.Target),
		ContextAfter:  toSegmentData(chunk.Context.After),
	}
}

func toSegmentData(segments []srt.Segment) []gemini.SegmentData {
	data := make([]gemini.SegmentData, len(segments))
	for i, s := range segments {
		data[i] = gemini.SegmentData{
			ID:    s.ID,
			Lines: s.Lines,
		}
	}
	return data
}

func (t *Translator) mergeResults(original []srt.Segment, resp *gemini.ResponseData) ([]srt.Segment, error) {
	expectedIDs := make(map[int]bool)
	for _, s := range original {
		expectedIDs[s.ID] = true
	}

	transMap := make(map[int]gemini.TranslatedSegment)
	for _, tr := range resp.Translations {
		// Check for duplicate IDs in model output
		if _, exists := transMap[tr.ID]; exists {
			return nil, fmt.Errorf("duplicate translation ID detected in model output: %d", tr.ID)
		}

		// Check for unexpected (hallucinated) IDs
		if !expectedIDs[tr.ID] {
			return nil, fmt.Errorf("unexpected translation ID (hallucination) from model: %d", tr.ID)
		}

		transMap[tr.ID] = tr
	}

	// Check if all requested IDs were returned
	if len(transMap) != len(original) {
		return nil, fmt.Errorf("translation count mismatch: expected %d, got %d", len(original), len(transMap))
	}

	results := make([]srt.Segment, len(original))
	for i, orig := range original {
		tr, ok := transMap[orig.ID]
		if !ok {
			return nil, fmt.Errorf("missing translation for segment ID %d", orig.ID)
		}

		// Validation: Ensure translation is not empty if original was not empty
		if tr.Line1 == "" && tr.Line2 == "" && len(orig.Lines) > 0 {
			return nil, fmt.Errorf("hallucination detected: empty translation for segment ID %d", orig.ID)
		}

		newLines := normalizeLines(tr.Line1, tr.Line2)

		results[i] = srt.Segment{
			ID:        orig.ID,
			StartTime: orig.StartTime,
			EndTime:   orig.EndTime,
			Lines:     newLines,
		}
	}

	return results, nil
}

func (t *Translator) validateResponse(resp *gemini.ResponseData) error {
	limit := float64(t.tgtLang.DefaultCPL) * 1.5
	for _, tr := range resp.Translations {
		c1 := uniseg.GraphemeClusterCount(tr.Line1)
		if float64(c1) > limit {
			return fmt.Errorf("line 1 too long: %d chars (max %.0f) for ID %d", c1, limit, tr.ID)
		}
		if tr.Line2 != "" {
			c2 := uniseg.GraphemeClusterCount(tr.Line2)
			if float64(c2) > limit {
				return fmt.Errorf("line 2 too long: %d chars (max %.0f) for ID %d", c2, limit, tr.ID)
			}
		}
	}
	return nil
}

func retryDecision(ctx context.Context, err error, attempt, maxAttempts int) (bool, time.Duration) {
	if err == nil {
		return false, 0
	}
	if attempt >= maxAttempts {
		return false, 0
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false, 0
	}
	if !apperrors.IsRetryable(err) {
		return false, 0
	}
	base := 1 * time.Second
	maxBackoff := 20 * time.Second
	jitterMax := 1 * time.Second

	backoff := base << (attempt - 1)
	if apperrors.IsRateLimit(err) {
		backoff = backoff * 2
	}
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	jitter := time.Duration(rand.Int63n(int64(jitterMax)))
	return true, backoff + jitter
}

func newRateLimiter(qps int) (<-chan time.Time, func()) {
	if qps <= 0 {
		return nil, func() {}
	}
	interval := time.Second / time.Duration(qps)
	ticker := time.NewTicker(interval)
	return ticker.C, ticker.Stop
}

func rampDelay(worker, concurrency int, ramp time.Duration) time.Duration {
	if ramp <= 0 || concurrency <= 1 {
		return 0
	}
	return time.Duration(int64(ramp) * int64(worker) / int64(concurrency-1))
}

// GetUsage returns the total token usage.
func (t *Translator) GetUsage() gemini.UsageMetadata {
	t.usageMu.Lock()
	defer t.usageMu.Unlock()
	return t.usage
}
