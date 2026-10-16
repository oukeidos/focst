package gemini

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/oukeidos/focst/internal/apperrors"
	"github.com/oukeidos/focst/internal/httpclient"
	"google.golang.org/genai"
)

// Client handles communication with the Gemini API.
type Client struct {
	client *genai.Client
	model  string
	config genai.GenerateContentConfig
}

// NewClient creates a new Gemini client.
func NewClient(ctx context.Context, apiKey string, modelName string) (*Client, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:     apiKey,
		Backend:    genai.BackendGeminiAPI,
		HTTPClient: httpclient.GetDefaultClient(),
		// The translator owns retries and rate-limit backoff.
		HTTPOptions: genai.HTTPOptions{RetryOptions: &genai.HTTPRetryOptions{Attempts: genai.Ptr(int32(1))}},
	})
	if err != nil {
		return nil, err
	}

	return &Client{
		client: client,
		model:  modelName,
		config: genai.GenerateContentConfig{ResponseMIMEType: "application/json"},
	}, nil
}

// Close preserves the caller lifecycle; the GenAI SDK has no close operation.
func (c *Client) Close() error {
	return nil
}

// SetSystemInstruction sets the system prompt for the model.
func (c *Client) SetSystemInstruction(prompt string) {
	c.config.SystemInstruction = &genai.Content{
		Parts: []*genai.Part{{Text: prompt}},
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
	// Preserve cancellation and bound the whole request, including SDK work.
	ctx, cancel := context.WithTimeout(ctx, httpclient.DefaultTimeout)
	defer cancel()
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// The SDK fills defaults in place; each worker needs its own config/content.
	config := c.config
	if config.SystemInstruction != nil {
		instruction := *config.SystemInstruction
		config.SystemInstruction = &instruction
	}
	resp, err := c.client.Models.GenerateContent(ctx, c.model, genai.Text(string(requestJSON)), &config)
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
	for _, candidate := range resp.Candidates {
		if candidate == nil || candidate.Content == nil || len(candidate.Content.Parts) == 0 {
			continue
		}
		var combined string
		for _, part := range candidate.Content.Parts {
			if part == nil || part.Thought {
				continue
			}
			combined += part.Text
		}
		if combined != "" {
			return combined, nil
		}
	}
	return "", fmt.Errorf("no text parts found in Gemini response")
}
