package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/oukeidos/focst/internal/apperrors"
	"github.com/oukeidos/focst/internal/httpclient"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"google.golang.org/genai"
)

func TestMockPerformance(t *testing.T) {
	// Simple test to verify our mock works as expected
	expected := &ResponseData{
		Translations: []TranslatedSegment{
			{ID: 1, Line1: "안녕"},
		},
	}
	mock := &MockClient{Response: expected}

	resp, err := mock.Translate(context.Background(), RequestData{})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Translations[0].Line1 != "안녕" {
		t.Errorf("Expected 안녕, got %s", resp.Translations[0].Line1)
	}
}

func TestExtractResponseText(t *testing.T) {
	t.Run("NilResponse", func(t *testing.T) {
		_, err := extractResponseText(nil)
		if err == nil || err.Error() != "no response received from Gemini" {
			t.Fatalf("expected nil response error, got: %v", err)
		}
	})

	t.Run("EmptyCandidates", func(t *testing.T) {
		_, err := extractResponseText(&genai.GenerateContentResponse{})
		if err == nil || err.Error() != "no candidates returned from Gemini" {
			t.Fatalf("expected empty candidates error, got: %v", err)
		}
	})

	t.Run("NoParts", func(t *testing.T) {
		resp := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{Content: &genai.Content{Parts: nil}},
			},
		}
		_, err := extractResponseText(resp)
		if err == nil || err.Error() != "no text parts found in Gemini response" {
			t.Fatalf("expected no text parts error, got: %v", err)
		}
	})

	t.Run("NonTextParts", func(t *testing.T) {
		resp := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{Content: &genai.Content{Parts: []*genai.Part{
					{InlineData: &genai.Blob{MIMEType: "application/octet-stream", Data: []byte{0x01}}},
				}}},
			},
		}
		_, err := extractResponseText(resp)
		if err == nil || err.Error() != "no text parts found in Gemini response" {
			t.Fatalf("expected no text parts error, got: %v", err)
		}
	})

	t.Run("MultiPartText", func(t *testing.T) {
		resp := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{Content: &genai.Content{Parts: []*genai.Part{
					nil,
					{Text: "private reasoning", Thought: true},
					{Text: "one"},
					{Text: "two"},
				}}},
			},
		}
		text, err := extractResponseText(resp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if text != "onetwo" {
			t.Fatalf("expected concatenated text, got: %q", text)
		}
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestTranslateGenAIWire(t *testing.T) {
	for _, model := range []string{"gemini-3.8-flash", "gemini-3.5-flash-lite", "gemini-4-argon", "gemini-3.1-pro-preview"} {
		t.Run(model, func(t *testing.T) {
			calls := 0
			restore := httpclient.SetDefaultClientForTesting(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.Method != "POST" || !strings.HasSuffix(req.URL.Path, "/models/"+model+":generateContent") {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				if req.Header.Get("x-goog-api-key") != "test-key" {
					t.Error("API key missing")
				}
				if _, ok := req.Context().Deadline(); !ok {
					t.Error("missing request deadline")
				}
				var body struct {
					Contents          []*genai.Content             `json:"contents"`
					SystemInstruction *genai.Content               `json:"systemInstruction"`
					GenerationConfig  *genai.GenerateContentConfig `json:"generationConfig"`
				}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.GenerationConfig == nil || body.GenerationConfig.ResponseMIMEType != "application/json" {
					t.Fatal("JSON response mode missing")
				}
				if body.SystemInstruction == nil || body.SystemInstruction.Parts[0].Text != "Translate to Korean" {
					t.Fatal("system instruction missing")
				}
				if len(body.Contents) != 1 || len(body.Contents[0].Parts) != 1 {
					t.Fatal("request content missing")
				}
				var input RequestData
				if err := json.Unmarshal([]byte(body.Contents[0].Parts[0].Text), &input); err != nil || len(input.Target) != 1 || input.Target[0].Lines[0] != "Hello" {
					t.Fatalf("unexpected input: %+v, %v", input, err)
				}
				payload := `{"candidates":[{"content":{"parts":[{"text":"private reasoning","thought":true},{"text":"{\"translations\":[{\"id\":1,\"line1\":\"안녕\"}]}"}]}}],"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":20,"thoughtsTokenCount":30,"totalTokenCount":150}}`
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(payload))}, nil
			})})
			defer restore()
			client, err := NewClient(context.Background(), "test-key", model)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			client.SetSystemInstruction("Translate to Korean")
			result, err := client.Translate(context.Background(), RequestData{Target: []SegmentData{{ID: 1, Lines: []string{"Hello"}}}})
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || len(result.Translations) != 1 || result.Translations[0].Line1 != "안녕" {
				t.Fatalf("unexpected result: %+v (calls=%d)", result, calls)
			}
			if result.Usage.PromptTokenCount != 100 || result.Usage.CandidatesTokenCount != 20 || result.Usage.TotalTokenCount != 150 {
				t.Fatalf("usage lost: %+v", result.Usage)
			}
		})
	}
}

func TestTranslateGenAIHTTPErrors(t *testing.T) {
	for _, tc := range []struct {
		code int
		kind apperrors.Kind
	}{{400, apperrors.KindBadRequest}, {401, apperrors.KindAuth}, {403, apperrors.KindAuth}, {404, apperrors.KindBadRequest}, {429, apperrors.KindRateLimit}, {503, apperrors.KindTransient}} {
		t.Run(fmt.Sprint(tc.code), func(t *testing.T) {
			calls := 0
			restore := httpclient.SetDefaultClientForTesting(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: tc.code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"error":{"code":%d,"message":"SECRET"}}`, tc.code)))}, nil
			})})
			defer restore()
			client, err := NewClient(context.Background(), "test-key", "gemini-3.8-flash")
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Translate(context.Background(), RequestData{})
			assertErrorKind(t, err, tc.kind)
			if calls != 1 {
				t.Fatalf("SDK duplicated application retries: %d calls", calls)
			}
			if strings.Contains(err.Error(), "SECRET") {
				t.Fatal("raw error exposed")
			}
		})
	}
}

func TestTranslateGenAIConcurrent(t *testing.T) {
	restore := httpclient.SetDefaultClientForTesting(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"[]"}]}}]}`))}, nil
	})})
	defer restore()
	client, err := NewClient(context.Background(), "test-key", "gemini-3.8-flash")
	if err != nil {
		t.Fatal(err)
	}
	client.SetSystemInstruction("Translate")
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := client.Translate(context.Background(), RequestData{}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if client.config.SystemInstruction.Role != "" {
		t.Fatal("SDK modified shared system instruction")
	}
}
