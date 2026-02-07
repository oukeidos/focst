package translator

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/oukeidos/focst/internal/apperrors"
	"github.com/oukeidos/focst/internal/gemini"
	"github.com/oukeidos/focst/internal/language"
	"github.com/oukeidos/focst/internal/srt"
)

type sequenceClient struct {
	mu        sync.Mutex
	calls     int
	responses []sequenceResponse
}

type sequenceResponse struct {
	resp *gemini.ResponseData
	err  error
}

func (c *sequenceClient) Translate(ctx context.Context, request gemini.RequestData) (*gemini.ResponseData, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	idx := c.calls - 1
	if idx >= len(c.responses) {
		idx = len(c.responses) - 1
	}
	return c.responses[idx].resp, c.responses[idx].err
}

func (c *sequenceClient) SetSystemInstruction(prompt string) {}

func TestRetryPolicy_ValidationRetries(t *testing.T) {
	// Validation errors (hallucination, ID mismatch) should now be retried
	// because LLM outputs are non-deterministic
	client := &sequenceClient{
		responses: []sequenceResponse{
			{
				resp: &gemini.ResponseData{
					Translations: []gemini.TranslatedSegment{
						{ID: 2, Line1: "bad"}, // Wrong ID - hallucination
					},
				},
			},
			{
				resp: &gemini.ResponseData{
					Translations: []gemini.TranslatedSegment{
						{ID: 2, Line1: "bad"}, // Still wrong
					},
				},
			},
			{
				resp: &gemini.ResponseData{
					Translations: []gemini.TranslatedSegment{
						{ID: 2, Line1: "bad"}, // Still wrong
					},
				},
			},
		},
	}
	src, _ := language.GetLanguage("en")
	tgt, _ := language.GetLanguage("ko")
	tr, err := NewTranslator(client, 1, 0, 1, false, src, tgt)
	if err != nil {
		t.Fatalf("NewTranslator failed: %v", err)
	}
	segments := []srt.Segment{{ID: 1, Lines: []string{"hello"}}}

	_, failed, err := tr.TranslateSRT(context.Background(), segments, nil)
	if err != nil {
		t.Fatalf("TranslateSRT failed: %v", err)
	}
	if len(failed) != 1 {
		t.Fatalf("expected 1 failed chunk, got %d", len(failed))
	}
	// Now validation errors are retried, so we expect 3 attempts
	if client.calls != 3 {
		t.Fatalf("expected 3 attempts for validation error (now retryable), got %d", client.calls)
	}
}

func TestRetryPolicy_TransientRetries(t *testing.T) {
	client := &sequenceClient{
		responses: []sequenceResponse{
			{err: apperrors.Transient(errors.New("temporary"))},
			{err: apperrors.Transient(errors.New("temporary"))},
			{
				resp: &gemini.ResponseData{
					Translations: []gemini.TranslatedSegment{
						{ID: 1, Line1: "ok"},
					},
				},
			},
		},
	}
	src, _ := language.GetLanguage("en")
	tgt, _ := language.GetLanguage("ko")
	tr, err := NewTranslator(client, 1, 0, 1, false, src, tgt)
	if err != nil {
		t.Fatalf("NewTranslator failed: %v", err)
	}
	segments := []srt.Segment{{ID: 1, Lines: []string{"hello"}}}

	_, failed, err := tr.TranslateSRT(context.Background(), segments, nil)
	if err != nil {
		t.Fatalf("TranslateSRT failed: %v", err)
	}
	if len(failed) != 0 {
		t.Fatalf("expected 0 failed chunks, got %d", len(failed))
	}
	if client.calls != 3 {
		t.Fatalf("expected 3 attempts for transient errors, got %d", client.calls)
	}
}
