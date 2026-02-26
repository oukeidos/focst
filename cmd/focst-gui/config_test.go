package main

import "testing"

func TestNormalizeGeminiModel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty uses default",
			input: "",
			want:  defaultGUIModel,
		},
		{
			name:  "supported model kept",
			input: "gemini-3.1-pro-preview",
			want:  "gemini-3.1-pro-preview",
		},
		{
			name:  "removed legacy model falls back",
			input: "gemini-3-pro-preview",
			want:  defaultGUIModel,
		},
		{
			name:  "unknown model falls back",
			input: "unknown-model",
			want:  defaultGUIModel,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeGeminiModel(tc.input)
			if got != tc.want {
				t.Fatalf("normalizeGeminiModel(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
