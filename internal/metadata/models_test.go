package metadata

import "testing"

func TestGeminiPricing_Default(t *testing.T) {
	m, ok := GeminiPricing("unknown-model")
	if ok {
		t.Fatalf("expected default pricing for unknown model")
	}
	if m.InputPerMillion != DefaultGeminiInputPerMillion || m.OutputPerMillion != DefaultGeminiOutputPerMillion {
		t.Fatalf("unexpected default gemini pricing: %+v", m)
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

func TestGeminiModelIDs_ExcludeLegacy3ProPreview(t *testing.T) {
	for _, id := range GeminiModelIDs() {
		if id == "gemini-3-pro-preview" {
			t.Fatalf("legacy model id %q must not be listed", id)
		}
	}
}

func TestGeminiPricing_Legacy3ProPreviewFallsBack(t *testing.T) {
	m, ok := GeminiPricing("gemini-3-pro-preview")
	if ok {
		t.Fatalf("expected unknown pricing result for removed model")
	}
	if m.InputPerMillion != DefaultGeminiInputPerMillion || m.OutputPerMillion != DefaultGeminiOutputPerMillion {
		t.Fatalf("unexpected fallback gemini pricing: %+v", m)
	}
}
