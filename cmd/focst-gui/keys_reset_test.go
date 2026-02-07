package main

import (
	"errors"
	"strings"
	"testing"
)

func TestResetKeysInKeychain(t *testing.T) {
	t.Run("deletes_both_keys", func(t *testing.T) {
		var called []string
		err := resetKeysInKeychain(func(service string) error {
			called = append(called, service)
			return nil
		})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if len(called) != 2 || called[0] != "gemini" || called[1] != "openai" {
			t.Fatalf("expected both delete attempts, got %#v", called)
		}
	})

	t.Run("returns_error_and_keeps_trying_other_key", func(t *testing.T) {
		var called []string
		err := resetKeysInKeychain(func(service string) error {
			called = append(called, service)
			if service == "gemini" {
				return errors.New("keychain unavailable")
			}
			return nil
		})
		if err == nil {
			t.Fatalf("expected error")
		}
		if !strings.Contains(err.Error(), "failed to delete Gemini key") {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(called) != 2 || called[0] != "gemini" || called[1] != "openai" {
			t.Fatalf("expected both delete attempts, got %#v", called)
		}
	})

	t.Run("returns_combined_error_when_both_fail", func(t *testing.T) {
		err := resetKeysInKeychain(func(service string) error {
			return errors.New(service + " delete failed")
		})
		if err == nil {
			t.Fatalf("expected error")
		}
		msg := err.Error()
		if !strings.Contains(msg, "failed to delete Gemini key") {
			t.Fatalf("missing gemini error in: %v", msg)
		}
		if !strings.Contains(msg, "failed to delete OpenAI key") {
			t.Fatalf("missing openai error in: %v", msg)
		}
	})
}
