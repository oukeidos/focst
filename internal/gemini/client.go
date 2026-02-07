package gemini

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"github.com/oukeidos/focst/internal/apperrors"
	"github.com/oukeidos/focst/internal/httpclient"
	"google.golang.org/api/option"
)

// Client handles communication with the Gemini API.
type Client struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

// NewClient creates a new Gemini client.
func NewClient(ctx context.Context, apiKey string, modelName string) (*Client, error) {
	// Note: We avoid using option.WithHTTPClient because it interferes with the genai library's
	// internal header injection for API keys, causing 403 errors.
	// Instead, we enforce timeouts via context in the Translate method.
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}

	model := client.GenerativeModel(modelName)
	model.ResponseMIMEType = "application/json"

	// We could define ResponseSchema here if we want strict enforcement by the API.
	// For now, we'll rely on the prompt and ResponseMIMEType.

	return &Client{
		client: client,
		model:  model,
	}, nil
}

// Close closes the underlying genai client.
func (c *Client) Close() error {
	return c.client.Close()
}

// SetSystemInstruction sets the system prompt for the model.
func (c *Client) SetSystemInstruction(prompt string) {
	c.model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(prompt)},
	}
}

// Translator interface for mocking and dependency injection.
type Translator interface {
	Translate(ctx context.Context, request RequestData) (*ResponseData, error)
	SetSystemInstruction(prompt string)
}

// Ensure Client implements Translator
var _ Translator = (*Client)(nil)

// Translate sends a request to Gemini and returns the translated data.
func (c *Client) Translate(ctx context.Context, request RequestData) (*ResponseData, error) {
	// Enforce default timeout to prevent indefinite hangs, since we are not using a custom HTTP client with timeout.
	ctx, cancel := context.WithTimeout(ctx, httpclient.DefaultTimeout)
	defer cancel()
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.model.GenerateContent(ctx, genai.Text(string(requestJSON)))
	if err != nil {
		return nil, classifyGeminiError(err)
	}

	var responseData ResponseData
	text, err := extractResponseText(resp)
	if err != nil {
		return nil, apperrors.Validation(err)
	}
	// Try unmarshaling as the expected object format
	if err := json.Unmarshal([]byte(text), &responseData); err != nil {
		// Fallback: Try unmarshaling as a direct array
		var transArray []TranslatedSegment
		if err2 := json.Unmarshal([]byte(text), &transArray); err2 == nil {
			responseData.Translations = transArray
		} else {
			// If both fail, return the unmarshal error but omit the raw text as requested
			return nil, apperrors.Validation(fmt.Errorf("failed to unmarshal response: %w", err))
		}
	}

	// Extract Usage Metadata
	if resp.UsageMetadata != nil {
		responseData.Usage = UsageMetadata{
			PromptTokenCount:     int(resp.UsageMetadata.PromptTokenCount),
			CandidatesTokenCount: int(resp.UsageMetadata.CandidatesTokenCount),
			TotalTokenCount:      int(resp.UsageMetadata.TotalTokenCount),
		}
	}

	return &responseData, nil
}

func extractResponseText(resp *genai.GenerateContentResponse) (string, error) {
	if resp == nil {
		return "", fmt.Errorf("no response received from Gemini")
	}
	if len(resp.Candidates) == 0 {
		return "", fmt.Errorf("no candidates returned from Gemini")
	}
	for i, candidate := range resp.Candidates {
		if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
			continue
		}
		var combined string
		for _, part := range candidate.Content.Parts {
			text, ok := part.(genai.Text)
			if !ok {
				continue
			}
			combined += string(text)
		}
		if combined != "" {
			return combined, nil
		}
		if i == len(resp.Candidates)-1 {
			break
		}
	}
	return "", fmt.Errorf("no text parts found in Gemini response")
}
