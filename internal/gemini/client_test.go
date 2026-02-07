package gemini

import (
	"context"
	"testing"

	"github.com/google/generative-ai-go/genai"
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
				{Content: &genai.Content{Parts: []genai.Part{
					genai.Blob{MIMEType: "application/octet-stream", Data: []byte{0x01}},
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
				{Content: &genai.Content{Parts: []genai.Part{
					genai.Text("one"),
					genai.Text("two"),
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
