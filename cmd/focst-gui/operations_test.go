package main

import (
	"path/filepath"
	"testing"

	"github.com/oukeidos/focst/internal/pipeline"
	"github.com/oukeidos/focst/internal/recovery"
)

func TestIsModelNotFound(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"The model `gpt-5.2` does not exist or you do not have access to it.", true},
		{"code: model_not_found", true},
		{"models/gemini-3-flash-preview is not found for API version v1beta, or is not supported for generateContent", true},
		{"Publisher Model foo was not found or your project does not have access to it.", true},
		{"Gemini model not found or no access (404).", true},
		{"random error", false},
	}
	for _, tc := range cases {
		if got := isModelNotFound(errString(tc.msg)); got != tc.want {
			t.Fatalf("isModelNotFound(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}

func TestStateForTranslationResult(t *testing.T) {
	cases := []struct {
		name   string
		status pipeline.TranslationStatus
		want   AppState
	}{
		{name: "success", status: pipeline.TranslationStatusSuccess, want: StateSuccess},
		{name: "partial_success", status: pipeline.TranslationStatusPartialSuccess, want: StatePartialSuccess},
		{name: "failure", status: pipeline.TranslationStatusFailure, want: StateFailure},
		{name: "skipped", status: pipeline.TranslationStatusSkipped, want: StateCanceled},
		{name: "unknown", status: pipeline.TranslationStatus(""), want: StateFailure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stateForTranslationResult(pipeline.TranslationResult{Status: tc.status})
			if got != tc.want {
				t.Fatalf("stateForTranslationResult(%q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

func TestPartialSuccessRepairLogPath(t *testing.T) {
	t.Run("uses_stored_recovery_log_after_translation", func(t *testing.T) {
		app := &focstApp{
			lastInputPath:       "/tmp/input.srt",
			lastRecoveryLogPath: "/tmp/actual_recovery.json",
			lastWasRepair:       false,
		}
		got := app.partialSuccessRepairLogPath()
		if got != "/tmp/actual_recovery.json" {
			t.Fatalf("partialSuccessRepairLogPath() = %q, want %q", got, "/tmp/actual_recovery.json")
		}
	})

	t.Run("uses_current_log_path_when_repair_session", func(t *testing.T) {
		app := &focstApp{
			lastInputPath:       "/tmp/current_session.json",
			lastRecoveryLogPath: "/tmp/old_recovery.json",
			lastWasRepair:       true,
		}
		got := app.partialSuccessRepairLogPath()
		if got != "/tmp/current_session.json" {
			t.Fatalf("partialSuccessRepairLogPath() = %q, want %q", got, "/tmp/current_session.json")
		}
	})

	t.Run("falls_back_to_derived_path_when_missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		inputPath := filepath.Join(tmpDir, "video.srt")
		app := &focstApp{
			lastInputPath: inputPath,
		}
		want := recovery.GenerateRecoveryPath(inputPath)
		got := app.partialSuccessRepairLogPath()
		if got != want {
			t.Fatalf("partialSuccessRepairLogPath() = %q, want %q", got, want)
		}
	})
}

type errString string

func (e errString) Error() string { return string(e) }
