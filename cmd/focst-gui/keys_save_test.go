package main

import (
	"errors"
	"strings"
	"testing"
)

func TestSaveKeysToKeychain(t *testing.T) {
	t.Run("empty_keys_noop", func(t *testing.T) {
		calls := 0
		result, err := saveKeysToKeychain("", " ", func(service, key string) error {
			calls++
			return nil
		})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if result.GeminiSaved || result.OpenAISaved {
			t.Fatalf("expected no saved flags, got %+v", result)
		}
		if calls != 0 {
			t.Fatalf("expected 0 save calls, got %d", calls)
		}
	})

	t.Run("saves_both_keys", func(t *testing.T) {
		var called []string
		result, err := saveKeysToKeychain("g-key", "o-key", func(service, key string) error {
			called = append(called, service+":"+key)
			return nil
		})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if !result.GeminiSaved || !result.OpenAISaved {
			t.Fatalf("expected both saved flags true, got %+v", result)
		}
		if len(called) != 2 {
			t.Fatalf("expected 2 save calls, got %d", len(called))
		}
		if called[0] != "gemini:g-key" || called[1] != "openai:o-key" {
			t.Fatalf("unexpected calls: %#v", called)
		}
	})

	t.Run("returns_error_and_keeps_trying_other_key", func(t *testing.T) {
		var called []string
		result, err := saveKeysToKeychain("g-key", "o-key", func(service, key string) error {
			called = append(called, service)
			if service == "gemini" {
				return errors.New("keychain unavailable")
			}
			return nil
		})
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "failed to save Gemini key") {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.GeminiSaved {
			t.Fatalf("expected GeminiSaved=false, got %+v", result)
		}
		if !result.OpenAISaved {
			t.Fatalf("expected OpenAISaved=true, got %+v", result)
		}
		if len(called) != 2 || called[0] != "gemini" || called[1] != "openai" {
			t.Fatalf("expected both save attempts, got %#v", called)
		}
	})
}
