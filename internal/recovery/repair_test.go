package recovery

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/oukeidos/focst/internal/gemini"
	"github.com/oukeidos/focst/internal/language"
	"github.com/oukeidos/focst/internal/translator"
)

// Mock Gemini Client
type mockGemini struct{}

func (m *mockGemini) Translate(ctx context.Context, req gemini.RequestData) (*gemini.ResponseData, error) {
	// Return a response that matches the target segments
	resp := &gemini.ResponseData{
		Translations: make([]gemini.TranslatedSegment, len(req.Target)),
	}
	for i, seg := range req.Target {
		resp.Translations[i] = gemini.TranslatedSegment{
			ID:    seg.ID,
			Line1: "번역됨: " + strings.Join(seg.Lines, " "),
		}
	}
	return resp, nil
}

func (m *mockGemini) SetSystemInstruction(prompt string) {}

func TestRepairToggles(t *testing.T) {
	// 1. Create a temporary SRT file
	srtContent := `1
00:00:01,000 --> 00:00:02,000
(Note) [Action] Hello
`
	tmpSRT, err := os.CreateTemp("", "test_*.srt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpSRT.Name())
	tmpSRT.WriteString(srtContent)
	tmpSRT.Close()

	tmpOut, err := os.CreateTemp("", "test_out_*.srt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpOut.Name())
	tmpOut.Close()

	ctx := context.Background()
	mockG := &mockGemini{}
	src, _ := language.GetLanguage("ja")
	tgt, _ := language.GetLanguage("ko")
	tr, err := translator.NewTranslator(mockG, 10, 5, 1, false, src, tgt)
	if err != nil {
		t.Fatalf("NewTranslator failed: %v", err)
	}

	t.Run("Repair with Preprocessing (Default)", func(t *testing.T) {
		log := &SessionLog{
			LogVersion:   CurrentLogVersion,
			InputPath:    tmpSRT.Name(),
			OutputPath:   tmpOut.Name(),
			NoPreprocess: false,
			SourceLang:   "ja",
			TargetLang:   "ko",
			FailedChunks: []int{0},
			ChunkSize:    10,
		}

		results, _, err := Repair(ctx, tr, log, tmpOut.Name(), true, nil)
		if err != nil {
			t.Fatalf("Repair failed: %v", err)
		}

		// Preprocessing should remove "(Note) [Action] "
		if len(results) > 0 {
			line := results[0].Lines[0]
			// The Repair logic loads input, preprocesses it, and THEN translates.
			// In our mock, it will translate the preprocessed text.
			// srt.Preprocess removes brackets.
			expected := "번역됨: Hello"
			if line != expected {
				t.Errorf("Expected '%s' (preprocessed), got '%s'", expected, line)
			}
		}
	})

	t.Run("Repair without Preprocessing", func(t *testing.T) {
		log := &SessionLog{
			LogVersion:   CurrentLogVersion,
			InputPath:    tmpSRT.Name(),
			OutputPath:   tmpOut.Name(),
			NoPreprocess: true,
			SourceLang:   "ja",
			TargetLang:   "ko",
			FailedChunks: []int{0},
			ChunkSize:    10,
		}

		results, _, err := Repair(ctx, tr, log, tmpOut.Name(), true, nil)
		if err != nil {
			t.Fatalf("Repair failed: %v", err)
		}

		// Preprocessing skipped, text should remain
		if len(results) > 0 {
			line := results[0].Lines[0]
			expected := "번역됨: (Note) [Action] Hello"
			if line != expected {
				t.Errorf("Expected '%s' (raw), got '%s'", expected, line)
			}
		}
	})

	t.Run("Repair without force rejects unusable output", func(t *testing.T) {
		log := &SessionLog{
			LogVersion:   CurrentLogVersion,
			InputPath:    tmpSRT.Name(),
			OutputPath:   tmpOut.Name(),
			NoPreprocess: true,
			SourceLang:   "ja",
			TargetLang:   "ko",
			FailedChunks: []int{0},
			ChunkSize:    10,
		}

		_, _, err := Repair(ctx, tr, log, tmpOut.Name(), false, nil)
		if err == nil || !strings.Contains(err.Error(), "existing output could not be reused") {
			t.Fatalf("expected output reuse error, got: %v", err)
		}
	})
}
