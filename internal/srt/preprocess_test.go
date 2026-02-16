package srt

import (
	"reflect"
	"testing"
)

func TestPreprocess(t *testing.T) {
	tests := []struct {
		name     string
		segments []Segment
		expected []Segment
	}{
		{
			name: "Remove brackets",
			segments: []Segment{
				{
					ID:    1,
					Lines: []string{"Hello (world)", "[Action] Good morning"},
				},
			},
			expected: []Segment{
				{
					ID:    1,
					Lines: []string{"Hello", "Good morning"},
				},
			},
		},
		{
			name: "Remove brackets leaving empty line",
			segments: []Segment{
				{ID: 1, Lines: []string{"(Action)"}},
				{ID: 2, Lines: []string{"Hello"}},
			},
			expected: []Segment{
				{ID: 1, Lines: []string{"Hello"}},
			},
		},
		{
			name: "Filter meaningless segments",
			segments: []Segment{
				{ID: 1, Lines: []string{"Hello"}},
				{ID: 2, Lines: []string{"!!!"}},
				{ID: 3, Lines: []string{"(Action only)"}},
				{ID: 4, Lines: []string{"...?"}},
				{ID: 5, Lines: []string{"World 123"}},
			},
			expected: []Segment{
				{ID: 1, Lines: []string{"Hello"}},
				{ID: 2, Lines: []string{"World 123"}},
			},
		},
		{
			name: "Re-indexing after removal",
			segments: []Segment{
				{ID: 10, Lines: []string{"First"}},
				{ID: 11, Lines: []string{"???"}},
				{ID: 12, Lines: []string{"Second"}},
			},
			expected: []Segment{
				{ID: 1, Lines: []string{"First"}},
				{ID: 2, Lines: []string{"Second"}},
			},
		},
		{
			name: "Remove angle brackets only",
			segments: []Segment{
				{ID: 1, Lines: []string{"<Hello>", "Normal text", "Left<Right"}},
			},
			expected: []Segment{
				{ID: 1, Lines: []string{"Hello", "Normal text", "LeftRight"}},
			},
		},
		{
			name: "Remove fullwidth parentheses and brackets",
			segments: []Segment{
				{ID: 1, Lines: []string{"Hello（world）", "［Action］ Good morning"}},
				{ID: 2, Lines: []string{"（SFX）"}},
				{ID: 3, Lines: []string{"Bye"}},
			},
			expected: []Segment{
				{ID: 1, Lines: []string{"Hello", "Good morning"}},
				{ID: 2, Lines: []string{"Bye"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned := Preprocess(tt.segments, "ja")
			if !reflect.DeepEqual(cleaned, tt.expected) {
				t.Errorf("Preprocess() = %+v, want %+v", cleaned, tt.expected)
			}
		})
	}
}

func TestPreprocess_FullwidthBracketsNotAppliedForNonJapanese(t *testing.T) {
	segments := []Segment{
		{ID: 1, Lines: []string{"Hello（world）", "［Action］ Good morning"}},
	}

	cleaned := Preprocess(segments, "en")
	expected := []Segment{
		{ID: 1, Lines: []string{"Hello（world）", "［Action］ Good morning"}},
	}
	if !reflect.DeepEqual(cleaned, expected) {
		t.Errorf("Preprocess() = %+v, want %+v", cleaned, expected)
	}
}

func TestPreprocessWithMapping(t *testing.T) {
	segments := []Segment{
		{ID: 1, Lines: []string{"(Note) Hello"}},
		{ID: 2, Lines: []string{"World"}},
		{ID: 3, Lines: []string{"[Action]"}},
		{ID: 4, Lines: []string{"Bye"}},
	}

	cleaned, mapping := PreprocessWithMapping(segments, "ja")
	if len(cleaned) != 3 {
		t.Fatalf("expected 3 cleaned segments, got %d", len(cleaned))
	}
	if len(mapping) != 3 {
		t.Fatalf("expected 3 mappings, got %d", len(mapping))
	}
	if mapping[0].InternalID != 1 || mapping[0].OriginalID != 1 {
		t.Fatalf("unexpected mapping[0]: %+v", mapping[0])
	}
	if mapping[1].InternalID != 2 || mapping[1].OriginalID != 2 {
		t.Fatalf("unexpected mapping[1]: %+v", mapping[1])
	}
	if mapping[2].InternalID != 3 || mapping[2].OriginalID != 4 {
		t.Fatalf("unexpected mapping[2]: %+v", mapping[2])
	}
}
