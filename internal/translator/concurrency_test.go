package translator

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/oukeidos/focst/internal/gemini"
	"github.com/oukeidos/focst/internal/language"
	"github.com/oukeidos/focst/internal/srt"
)

type slowMockClient struct {
	gemini.Translator
}

func (m *slowMockClient) SetSystemInstruction(prompt string) {}

func (m *slowMockClient) Translate(ctx context.Context, req gemini.RequestData) (*gemini.ResponseData, error) {
	// Simulate slow API call
	time.Sleep(100 * time.Millisecond)

	translations := make([]gemini.TranslatedSegment, len(req.Target))
	for i, s := range req.Target {
		translations[i] = gemini.TranslatedSegment{
			ID:    s.ID,
			Line1: "translated",
		}
	}

	return &gemini.ResponseData{
		Translations: translations,
		Usage:        gemini.UsageMetadata{},
	}, nil
}

func TestTranslator_GoroutineLimit(t *testing.T) {
	concurrency := 5
	chunkCount := 100

	client := &slowMockClient{}
	tr, err := NewTranslator(client, 1, 0, concurrency, false, language.Languages["en"], language.Languages["ko"])
	if err != nil {
		t.Fatalf("NewTranslator failed: %v", err)
	}

	segments := make([]srt.Segment, chunkCount)
	for i := 0; i < chunkCount; i++ {
		segments[i] = srt.Segment{ID: i + 1, Lines: []string{"test"}}
	}

	initialGoroutines := runtime.NumGoroutine()

	// Run translation in background to check goroutine count during execution
	errChan := make(chan error, 1)
	go func() {
		_, _, err := tr.TranslateSRT(context.Background(), segments, nil)
		errChan <- err
	}()

	// Wait a bit for goroutines to ramp up
	time.Sleep(500 * time.Millisecond)

	currentGoroutines := runtime.NumGoroutine()
	// Total goroutines should be roughly initial + worker count + a few extra for the test runner/TranslateSRT caller
	// Before the fix, it would have been initial + chunkCount
	if currentGoroutines > initialGoroutines+concurrency+10 {
		t.Errorf("Too many goroutines: got %d, initial was %d, concurrency is %d", currentGoroutines, initialGoroutines, concurrency)
	}

	err = <-errChan
	if err != nil {
		t.Errorf("TranslateSRT failed: %v", err)
	}
}
