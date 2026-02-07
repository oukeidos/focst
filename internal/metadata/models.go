package metadata

type GeminiModel struct {
	ID                     string
	Label                  string
	InputPerMillion        float64
	OutputPerMillion       float64
	ReasoningBilledAsOutput bool
}

type OpenAIModel struct {
	ID               string
	Label            string
	InputPerMillion  float64
	OutputPerMillion float64
}

var GeminiModels = []GeminiModel{
	{
		ID:                     "gemini-3-flash-preview",
		Label:                  "Gemini 3 Flash (preview)",
		InputPerMillion:        0.50,
		OutputPerMillion:       3.00,
		ReasoningBilledAsOutput: true,
	},
	{
		ID:                     "gemini-3-pro-preview",
		Label:                  "Gemini 3 Pro (preview)",
		InputPerMillion:        2.00,
		OutputPerMillion:       12.00,
		ReasoningBilledAsOutput: true,
	},
}

var OpenAIModels = []OpenAIModel{
	{
		ID:               "gpt-5.2",
		Label:            "GPT-5.2",
		InputPerMillion:  1.75,
		OutputPerMillion: 14.00,
	},
}

const (
	DefaultOpenAIInputPerMillion  = 2.50
	DefaultOpenAIOutputPerMillion = 10.00
	DefaultGeminiInputPerMillion  = 2.00
	DefaultGeminiOutputPerMillion = 12.00
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
	return GeminiModel{
		ID:                     "default",
		Label:                  "Default Gemini",
		InputPerMillion:        DefaultGeminiInputPerMillion,
		OutputPerMillion:       DefaultGeminiOutputPerMillion,
		ReasoningBilledAsOutput: true,
	}, false
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
