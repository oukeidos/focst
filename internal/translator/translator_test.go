package translator

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/oukeidos/focst/internal/gemini"
	"github.com/oukeidos/focst/internal/language"
	"github.com/oukeidos/focst/internal/srt"
)

func TestTranslator_TranslateSRT(t *testing.T) {
	// Setup mock
	mockClient := &gemini.MockClient{
		Response: &gemini.ResponseData{
			Translations: []gemini.TranslatedSegment{
				{ID: 1, Line1: "안녕"},
				{ID: 2, Line1: "세상아", Line2: "반가워"},
			},
		},
	}

	segments := []srt.Segment{
		{ID: 1, StartTime: "00:00", EndTime: "00:01", Lines: []string{"こんにちは"}},
		{ID: 2, StartTime: "00:01", EndTime: "00:02", Lines: []string{"世界よ", "またな"}},
	}

	src, _ := language.GetLanguage("ja")
	tgt, _ := language.GetLanguage("ko")
	tr, err := NewTranslator(mockClient, 100, 5, 1, false, src, tgt)
	if err != nil {
		t.Fatalf("NewTranslator fail: %v", err)
	}
	results, failed, err := tr.TranslateSRT(context.Background(), segments, nil)
	if err != nil {
		t.Fatalf("TranslateSRT fail: %v", err)
	}
	if len(failed) > 0 {
		t.Errorf("TranslateSRT() failed chunks: %v", failed)
	}

	expected := []srt.Segment{
		{ID: 1, StartTime: "00:00", EndTime: "00:01", Lines: []string{"안녕"}},
		{ID: 2, StartTime: "00:01", EndTime: "00:02", Lines: []string{"세상아", "반가워"}},
	}

	if !reflect.DeepEqual(results, expected) {
		t.Errorf("TranslateSRT() = %+v, want %+v", results, expected)
	}
}

func TestTranslator_ValidateResponse(t *testing.T) {
	tgt, _ := language.GetLanguage("ko")
	tr := &Translator{tgtLang: tgt}

	tests := []struct {
		name    string
		resp    *gemini.ResponseData
		wantErr bool
	}{
		{
			name: "Valid length",
			resp: &gemini.ResponseData{
				Translations: []gemini.TranslatedSegment{
					{ID: 1, Line1: "이것은 적절한 길이의 자막입니다."}, // 16 chars
				},
			},
			wantErr: false,
		},
		{
			name: "Line 1 too long",
			resp: &gemini.ResponseData{
				Translations: []gemini.TranslatedSegment{
					{ID: 1, Line1: "이것은 매우 매우 매우 매우 매우 매우 매우 긴 자막이라서 24자를 초과합니다."}, // > 24 chars
				},
			},
			wantErr: true,
		},
		{
			name: "Line 2 too long",
			resp: &gemini.ResponseData{
				Translations: []gemini.TranslatedSegment{
					{ID: 1, Line1: "정상 라인", Line2: "하지만 두 번째 라인이 너무 길어서 문제가 되는 경우입니다."}, // > 24 chars
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tr.validateResponse(tt.resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
func TestTranslator_EmptyTranslation(t *testing.T) {
	mockClient := &gemini.MockClient{
		Response: &gemini.ResponseData{
			Translations: []gemini.TranslatedSegment{
				{ID: 1, Line1: ""}, // Empty translation
			},
		},
	}

	segments := []srt.Segment{
		{ID: 1, StartTime: "00:00", EndTime: "00:01", Lines: []string{"こんにちは"}},
	}

	tr, err := NewTranslator(mockClient, 1, 0, 1, false, language.Language{Name: "Japanese", Code: "ja"}, language.Language{Name: "Korean", Code: "ko"})
	if err != nil {
		t.Fatalf("NewTranslator fail: %v", err)
	}
	_, failed, _ := tr.TranslateSRT(context.Background(), segments, nil)

	// Currently, this will succeed (failed list empty). We WANT it to fail.
	if len(failed) == 0 {
		t.Errorf("Expected translation to fail for empty line1, but it succeeded")
	}
}
func TestTranslator_MergeResultsStrictValidation(t *testing.T) {
	tr := &Translator{}
	original := []srt.Segment{
		{ID: 1},
		{ID: 2},
	}

	tests := []struct {
		name    string
		resp    *gemini.ResponseData
		wantErr string
	}{
		{
			name: "Duplicate ID",
			resp: &gemini.ResponseData{
				Translations: []gemini.TranslatedSegment{
					{ID: 1, Line1: "T1"},
					{ID: 1, Line1: "T1-dup"},
				},
			},
			wantErr: "duplicate translation ID",
		},
		{
			name: "Hallucinated ID",
			resp: &gemini.ResponseData{
				Translations: []gemini.TranslatedSegment{
					{ID: 1, Line1: "T1"},
					{ID: 99, Line1: "Ghost"},
				},
			},
			wantErr: "unexpected translation ID",
		},
		{
			name: "Missing ID",
			resp: &gemini.ResponseData{
				Translations: []gemini.TranslatedSegment{
					{ID: 1, Line1: "T1"},
				},
			},
			wantErr: "translation count mismatch",
		},
		{
			name: "Too many IDs (even if valid IDs)",
			resp: &gemini.ResponseData{
				Translations: []gemini.TranslatedSegment{
					{ID: 1, Line1: "T1"},
					{ID: 2, Line1: "T2"},
					{ID: 1, Line1: "T1-again"}, // This also triggers duplicate check first
				},
			},
			wantErr: "duplicate translation ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tr.mergeResults(original, tt.resp)
			if err == nil {
				t.Errorf("mergeResults() expected error, got nil")
				return
			}
			if tt.wantErr != "" {
				errStr := err.Error()
				if !strings.Contains(errStr, tt.wantErr) {
					t.Errorf("mergeResults() error = %v, want substring %v", errStr, tt.wantErr)
				}
			}
		})
	}
}
