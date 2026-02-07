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
