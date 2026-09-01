package metadata

import (
	"reflect"
	"testing"
)

func TestGeminiModelIDs(t *testing.T) {
	want := []string{
		"gemini-3.7-flash",
		"gemini-3.5-flash-lite",
		"gemini-3.1-pro-preview",
	}
	if got := GeminiModelIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("GeminiModelIDs() = %v, want %v", got, want)
	}
}

func TestGeminiPricing(t *testing.T) {
	tests := []struct {
		model      string
		wantInput  float64
		wantOutput float64
	}{
		{model: "gemini-3.7-flash", wantInput: 0.75, wantOutput: 3.75},
		{model: "gemini-3.5-flash-lite", wantInput: 0.30, wantOutput: 2.50},
		{model: "gemini-3.1-pro-preview", wantInput: 2.00, wantOutput: 12.00},
	}
	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			m, ok := GeminiPricing(tc.model)
			if !ok {
				t.Fatalf("expected pricing for %q", tc.model)
			}
			if m.InputPerMillion != tc.wantInput || m.OutputPerMillion != tc.wantOutput {
				t.Fatalf("GeminiPricing(%q) = input %.2f, output %.2f; want input %.2f, output %.2f", tc.model, m.InputPerMillion, m.OutputPerMillion, tc.wantInput, tc.wantOutput)
			}
		})
	}
}

func TestGeminiPricing_UnknownHasNoFallback(t *testing.T) {
	m, ok := GeminiPricing("unknown-model")
	if ok {
		t.Fatalf("expected no pricing for unknown model")
	}
	if !reflect.DeepEqual(m, GeminiModel{}) {
		t.Fatalf("unexpected pricing for unknown model: %+v", m)
	}
}

func TestOpenAIPricing_Default(t *testing.T) {
	m, ok := OpenAIPricing("unknown-model")
	if ok {
		t.Fatalf("expected default pricing for unknown model")
	}
	if m.InputPerMillion != DefaultOpenAIInputPerMillion || m.OutputPerMillion != DefaultOpenAIOutputPerMillion {
		t.Fatalf("unexpected default openai pricing: %+v", m)
	}
}

func TestGeminiModelIDs_ExcludeRemovedModels(t *testing.T) {
	removed := map[string]bool{
		"gemini-3-flash-preview": true,
		"gemini-3-pro-preview":   true,
	}
	for _, id := range GeminiModelIDs() {
		if removed[id] {
			t.Fatalf("removed model id %q must not be listed", id)
		}
	}
}

func TestGeminiPricing_RemovedModelsHaveNoPricing(t *testing.T) {
	for _, id := range []string{"gemini-3-flash-preview", "gemini-3-pro-preview"} {
		if _, ok := GeminiPricing(id); ok {
			t.Fatalf("expected no pricing for removed model %q", id)
		}
	}
}
