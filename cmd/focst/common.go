package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/oukeidos/focst/internal/auth"
	"github.com/oukeidos/focst/internal/gemini"
	"github.com/oukeidos/focst/internal/language"
	"github.com/oukeidos/focst/internal/logger"
	"github.com/oukeidos/focst/internal/metadata"
	"github.com/oukeidos/focst/internal/names"
	"github.com/oukeidos/focst/internal/openai"
	"golang.org/x/term"
)

var (
	isTerminal   = term.IsTerminal
	getKey       = auth.GetKey
	getEnvKey    = auth.GetEnvKey
	getStatus    = auth.GetStatus
	promptForKey = auth.PromptForAPIKey
)

// resolveAPIKey handles the logic for finding the API key.
func resolveAPIKey(service string, allowEnv, envOnly bool) (string, string, error) {
	if envOnly {
		allowEnv = true
	}
	if envOnly {
		if key, ok := getEnvKey(service); ok {
			return key, "Environment Variable", nil
		}
		return "", "", fmt.Errorf("env-only set but %s_API_KEY is not set", strings.ToUpper(service))
	}

	if key, source := getKey(service, false); key != "" {
		return key, source, nil
	}

	if allowEnv {
		if key, ok := getEnvKey(service); ok {
			return key, "Environment Variable", nil
		}
	}

	if isTerminal(int(os.Stdin.Fd())) {
		svcName := "Gemini"
		if service == "openai" {
			svcName = "OpenAI"
		}
		key, err := promptForKey(fmt.Sprintf("%s API Key (press Enter to skip): ", svcName))
		if err != nil {
			return "", "", fmt.Errorf("error reading API key: %w", err)
		}
		if strings.TrimSpace(key) != "" {
			return strings.TrimSpace(key), "Terminal Prompt", nil
		}
	}

	if !isTerminal(int(os.Stdin.Fd())) {
		return "", "", fmt.Errorf("no API key available (non-interactive shell); set keychain or use --allow-env")
	}
	if allowEnv {
		return "", "", fmt.Errorf("API key is required; not found in keychain or environment")
	}
	return "", "", fmt.Errorf("API key is required; not found in keychain (environment disabled by default; use --allow-env)")
}

func resolveLanguageCode(input string) (string, error) {
	if lang, ok := language.GetLanguage(input); ok {
		return lang.Code, nil
	}
	needle := strings.TrimSpace(input)
	if needle == "" {
		return "", fmt.Errorf("language is empty")
	}
	for _, entry := range language.GetSupportedLanguages() {
		if strings.EqualFold(entry.Name, needle) {
			return entry.Code, nil
		}
	}
	return "", fmt.Errorf("unsupported language: %s", input)
}

func loadNamesMapping(path, sourceCode, targetCode string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read names mapping file %s: %w", path, err)
	}
	mappings, err := names.DecodeMappings(data, sourceCode, targetCode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse names mapping file %s: %w", path, err)
	}
	nameDict := make(map[string]string)
	for _, m := range mappings {
		nameDict[m.Source] = m.Target
	}
	return nameDict, nil
}

func printUsageStats(usage *gemini.UsageMetadata, duration time.Duration, model string) {
	fmt.Println("\n--- Execution Stats ---")
	fmt.Printf("Time: %s\n", duration)
	fmt.Printf("Model: %s\n", model)
	if usage != nil && usage.TotalTokenCount > 0 {
		fmt.Printf("Tokens: In=%d, Out=%d, Total=%d, Web=%d\n",
			usage.PromptTokenCount, usage.CandidatesTokenCount, usage.TotalTokenCount, usage.WebSearchCount)

		// Gemini Cost Estimation
		// Reasoning tokens are billed as output tokens.
		// Reasoning Tokens = Total - (Prompt + Candidates)
		reasoningTokens := usage.TotalTokenCount - (usage.PromptTokenCount + usage.CandidatesTokenCount)
		if reasoningTokens < 0 {
			reasoningTokens = 0
		}
		billableOutput := usage.CandidatesTokenCount + reasoningTokens

		pricing, _ := metadata.GeminiPricing(model)
		inRate := pricing.InputPerMillion
		outRate := pricing.OutputPerMillion

		inCost := (float64(usage.PromptTokenCount) / 1_000_000) * inRate
		outCost := (float64(billableOutput) / 1_000_000) * outRate

		fmt.Printf("Estimated Cost: $%.5f (Reasoning Tokens: %d)\n", inCost+outCost, reasoningTokens)
	}
}

func estimateOpenAICost(model string, usage openai.Usage) float64 {
	pricing, _ := metadata.OpenAIPricing(model)
	inRate := pricing.InputPerMillion
	outRate := pricing.OutputPerMillion

	// Search Content Tokens are included in InputTokens and billed at input rate
	tokenCost := (float64(usage.InputTokens)/1_000_000)*inRate + (float64(usage.OutputTokens)/1_000_000)*outRate

	// Web Search Tool Calls: $10.00 / 1k calls = $0.01 per call
	searchCost := float64(usage.WebSearchCalls) * metadata.WebSearchCostPerCall

	return tokenCost + searchCost
}

func signalContext() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Warn("Cancellation requested")
		cancel()
	}()
	stop := func() {
		signal.Stop(sigCh)
		cancel()
	}
	return ctx, stop
}
