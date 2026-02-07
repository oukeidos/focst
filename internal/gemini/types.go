package gemini

// SegmentData represents the structure of a single segment in the input JSON.
type SegmentData struct {
	ID    int      `json:"id"`
	Lines []string `json:"lines"`
}

// RequestData represents the full input JSON structure sent to Gemini.
type RequestData struct {
	ContextBefore []SegmentData `json:"context_before"`
	Target        []SegmentData `json:"target"`
	ContextAfter  []SegmentData `json:"context_after"`
}

// TranslatedSegment represents the structure of a single translated segment in the output JSON.
type TranslatedSegment struct {
	ID    int    `json:"id"`
	Line1 string `json:"line1"`
	Line2 string `json:"line2,omitempty"`
}

// ResponseData represents the full output JSON structure expected from Gemini.
type ResponseData struct {
	Translations []TranslatedSegment `json:"translations"`
	Usage        UsageMetadata       `json:"-"` // Not part of Gemini's JSON response, filled manually
}

// UsageMetadata holds token usage information.
type UsageMetadata struct {
	PromptTokenCount     int
	CandidatesTokenCount int
	TotalTokenCount      int
	WebSearchCount       int
}
