package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/oukeidos/focst/internal/gemini"
)

type keyStubs struct {
	promptCalls int
	keyCalls    int
	envCalls    int
}

func withKeyStubs(t *testing.T, terminal bool, promptVal string, keychainVal string, envVal string) (*keyStubs, func()) {
	t.Helper()
	stubs := &keyStubs{}

	prevIsTerminal := isTerminal
	prevPrompt := promptForKey
	prevGetKey := getKey
	prevGetEnv := getEnvKey

	isTerminal = func(_ int) bool { return terminal }
	promptForKey = func(_ string) (string, error) {
		stubs.promptCalls++
		return promptVal, nil
	}
	getKey = func(_ string, _ bool) (string, string) {
		stubs.keyCalls++
		if keychainVal == "" {
			return "", ""
		}
		return keychainVal, "Keychain"
	}
	getEnvKey = func(_ string) (string, bool) {
		stubs.envCalls++
		if envVal == "" {
			return "", false
		}
		return envVal, true
	}

	restore := func() {
		isTerminal = prevIsTerminal
		promptForKey = prevPrompt
		getKey = prevGetKey
		getEnvKey = prevGetEnv
	}

	return stubs, restore
}

func TestResolveAPIKey_KeychainFallback(t *testing.T) {
	stubs, restore := withKeyStubs(t, true, "", "keychain-key", "env-key")
	defer restore()

	key, source, err := resolveAPIKey("gemini", true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "keychain-key" || source != "Keychain" {
		t.Fatalf("expected keychain key/source, got key=%q source=%q", key, source)
	}
	if stubs.envCalls != 0 {
		t.Fatalf("expected no env calls, got envCalls=%d", stubs.envCalls)
	}
}

func TestResolveAPIKey_EnvFallbackWhenAllowed(t *testing.T) {
	stubs, restore := withKeyStubs(t, false, "", "", "env-key")
	defer restore()

	key, source, err := resolveAPIKey("gemini", true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "env-key" || source != "Environment Variable" {
		t.Fatalf("expected env key/source, got key=%q source=%q", key, source)
	}
	if stubs.envCalls == 0 {
		t.Fatalf("expected env call")
	}
}

func TestResolveAPIKey_EnvDisabledError(t *testing.T) {
	stubs, restore := withKeyStubs(t, false, "", "", "env-key")
	defer restore()

	key, source, err := resolveAPIKey("gemini", false, false)
	if err == nil {
		t.Fatalf("expected error, got key=%q source=%q", key, source)
	}
	if stubs.envCalls != 0 {
		t.Fatalf("expected no env calls, got envCalls=%d", stubs.envCalls)
	}
}

func TestResolveAPIKey_NonInteractiveError(t *testing.T) {
	stubs, restore := withKeyStubs(t, false, "", "", "")
	defer restore()

	key, source, err := resolveAPIKey("gemini", false, false)
	if err == nil {
		t.Fatalf("expected error, got key=%q source=%q", key, source)
	}
	if stubs.promptCalls != 0 {
		t.Fatalf("expected no prompt, got promptCalls=%d", stubs.promptCalls)
	}
}

func TestResolveAPIKey_EnvOnlySuccess(t *testing.T) {
	stubs, restore := withKeyStubs(t, false, "prompt-key", "keychain-key", "env-key")
	defer restore()

	key, source, err := resolveAPIKey("gemini", false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "env-key" || source != "Environment Variable" {
		t.Fatalf("expected env key/source, got key=%q source=%q", key, source)
	}
	if stubs.promptCalls != 0 || stubs.keyCalls != 0 {
		t.Fatalf("expected no prompt/keychain calls, got promptCalls=%d keyCalls=%d", stubs.promptCalls, stubs.keyCalls)
	}
}

func TestResolveAPIKey_EnvOnlyMissingError(t *testing.T) {
	_, restore := withKeyStubs(t, false, "", "keychain-key", "")
	defer restore()

	_, _, err := resolveAPIKey("gemini", false, true)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveAPIKey_EnvOnlyWithAllowEnvFlag(t *testing.T) {
	stubs, restore := withKeyStubs(t, false, "prompt-key", "keychain-key", "env-key")
	defer restore()

	key, source, err := resolveAPIKey("gemini", true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "env-key" || source != "Environment Variable" {
		t.Fatalf("expected env key/source, got key=%q source=%q", key, source)
	}
	if stubs.promptCalls != 0 || stubs.keyCalls != 0 {
		t.Fatalf("expected no prompt/keychain calls, got promptCalls=%d keyCalls=%d", stubs.promptCalls, stubs.keyCalls)
	}
}

func TestResolveAPIKey_PromptFallback(t *testing.T) {
	stubs, restore := withKeyStubs(t, true, "prompt-key", "", "")
	defer restore()

	key, source, err := resolveAPIKey("gemini", false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "prompt-key" || source != "Terminal Prompt" {
		t.Fatalf("expected prompt key/source, got key=%q source=%q", key, source)
	}
	if stubs.keyCalls == 0 {
		t.Fatalf("expected keychain lookup before prompt")
	}
}

func TestPrintUsageStats_KnownModelPrintsEstimatedCost(t *testing.T) {
	var output bytes.Buffer
	usage := &gemini.UsageMetadata{
		PromptTokenCount:     1_000_000,
		CandidatesTokenCount: 1_000_000,
		TotalTokenCount:      2_000_000,
	}

	printUsageStatsTo(&output, usage, time.Second, "gemini-3.7-flash")

	if !strings.Contains(output.String(), "Estimated Cost: $4.50000") {
		t.Fatalf("expected known-model cost estimate, got: %s", output.String())
	}
}

func TestPrintUsageStats_UnknownModelOmitsEstimatedCost(t *testing.T) {
	var output bytes.Buffer
	usage := &gemini.UsageMetadata{
		PromptTokenCount:     100,
		CandidatesTokenCount: 50,
		TotalTokenCount:      150,
	}

	printUsageStatsTo(&output, usage, time.Second, "unknown-model")

	if !strings.Contains(output.String(), "Tokens:") {
		t.Fatalf("expected token usage for unknown model, got: %s", output.String())
	}
	if strings.Contains(output.String(), "Estimated Cost:") {
		t.Fatalf("unexpected cost estimate for unknown model: %s", output.String())
	}
}
