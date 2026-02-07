package gemini

import "context"

// MockClient for testing
type MockClient struct {
	Response              *ResponseData
	Error                 error
	LastSystemInstruction string
}

func (m *MockClient) Translate(ctx context.Context, request RequestData) (*ResponseData, error) {
	return m.Response, m.Error
}

func (m *MockClient) SetSystemInstruction(prompt string) {
	m.LastSystemInstruction = prompt
}
