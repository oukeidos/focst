package metadata

type GeminiModel struct {
	ID                      string
	Label                   string
	InputPerMillion         float64
	OutputPerMillion        float64
	ReasoningBilledAsOutput bool
}

type OpenAIModel struct {
	ID               string
	Label            string
	InputPerMillion  float64
	OutputPerMillion float64
}

// Standard token prices verified 2026-10-16; see provider pricing pages.
var GeminiModels = []GeminiModel{
	{
		// Promotional prices through 2026-12-31; then input $1.50 / output $7.50.
		ID:                      "gemini-3.8-flash",
		Label:                   "Gemini 3.8 Flash",
		InputPerMillion:         0.75,
		OutputPerMillion:        3.75,
		ReasoningBilledAsOutput: true,
	},
	{
		ID:                      "gemini-3.5-flash-lite",
		Label:                   "Gemini 3.5 Flash-Lite",
		InputPerMillion:         0.30,
		OutputPerMillion:        2.50,
		ReasoningBilledAsOutput: true,
	},
	{
		ID:                      "gemini-3.1-pro-preview",
		Label:                   "Gemini 3.1 Pro (preview)",
		InputPerMillion:         2.00,
		OutputPerMillion:        12.00,
		ReasoningBilledAsOutput: true,
	},
	{
		// Promotional prices through 2026-11-30; then input $4.00 / output $20.00.
		ID:                      "gemini-4-argon",
		Label:                   "Gemini 4 Argon",
		InputPerMillion:         2.00,
		OutputPerMillion:        10.00,
		ReasoningBilledAsOutput: true,
	},
}

var OpenAIModels = []OpenAIModel{
	{
		ID:               "gpt-6.1-sol",
		Label:            "GPT-6.1 Sol",
		InputPerMillion:  2.00,
		OutputPerMillion: 10.00,
	},
}

const (
	DefaultOpenAIInputPerMillion  = 2.50
	DefaultOpenAIOutputPerMillion = 10.00
	WebSearchCostPerCall          = 0.01
)

func GeminiModelIDs() []string {
	ids := make([]string, 0, len(GeminiModels))
	for _, m := range GeminiModels {
		ids = append(ids, m.ID)
	}
	return ids
}

func GeminiPricing(modelID string) (GeminiModel, bool) {
	for _, m := range GeminiModels {
		if m.ID == modelID {
			return m, true
		}
	}
	return GeminiModel{}, false
}

func OpenAIPricing(modelID string) (OpenAIModel, bool) {
	for _, m := range OpenAIModels {
		if m.ID == modelID {
			return m, true
		}
	}
	return OpenAIModel{
		ID:               "default",
		Label:            "Default OpenAI",
		InputPerMillion:  DefaultOpenAIInputPerMillion,
		OutputPerMillion: DefaultOpenAIOutputPerMillion,
	}, false
}
