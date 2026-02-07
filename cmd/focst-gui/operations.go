package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"github.com/oukeidos/focst/internal/auth"
	"github.com/oukeidos/focst/internal/language"
	"github.com/oukeidos/focst/internal/logger"
	"github.com/oukeidos/focst/internal/names"
	"github.com/oukeidos/focst/internal/openai"
	"github.com/oukeidos/focst/internal/pipeline"
	"github.com/oukeidos/focst/internal/srt"
	"github.com/oukeidos/focst/internal/translator"
)

var errOpenAIKeyMissing = errors.New("openai api key is required")

func (a *focstApp) startTranslation(inputPath string) {
	a.setState(StateProcessing)
	a.lastRecoveryLogPath = ""

	// Mock flow for debug files
	if state, ok := debugStateForPath(inputPath); ok {
		time.Sleep(1 * time.Second)
		a.setState(state)
		return
	}

	apiKey := a.sessionKey
	if apiKey == "" {
		apiKey, _ = auth.GetKey("gemini", false)
	}

	if apiKey == "" {
		a.flashRed()
		a.setState(StateNoKey)
		return
	}

	// Prepare Config
	// Note: We need to resolve languages here or inside pipeline? Pipeline expects codes/names.
	// Config has SourceLang/TargetLang strings which pipeline resolves.
	cfg := pipeline.Config{
		InputPath:         inputPath,
		OutputPath:        srt.GenerateOutputPath(inputPath, language.Languages[a.config.TargetLang].Code),
		APIKey:            apiKey,
		Model:             a.config.Model,
		ChunkSize:         a.config.ChunkSize,
		ContextSize:       a.config.ContextSize,
		Concurrency:       a.config.Concurrency,
		RetryOnLongLines:  a.config.RetryOnLongLines,
		NoPromptCPL:       a.config.NoPromptCPL,
		NoPreprocess:      a.config.NoPreprocess,
		NoPostprocess:     a.config.NoPostprocess,
		NoLangPreprocess:  a.config.NoLangPreprocess,
		NoLangPostprocess: a.config.NoLangPostprocess,
		SourceLang:        a.config.SourceLang,
		TargetLang:        a.config.TargetLang,
		NamesMapping:      a.config.NamesMapping,
		OnProgress: func(p translator.TranslationProgress) {
			// Update UI with progress?
			// The original GUI didn't seem to show detailed chunk progress in the main view,
			// just StateProcessing spinner/text.
			// We can log it.
			logger.Info("GUI Progress", "chunk", p.ChunkIndex, "status", p.State)
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancelID := a.setActiveCancel(cancel)
	a.safeGo("ops.translate", func() {
		defer a.clearActiveCancel(cancelID)
		result, err := pipeline.RunTranslation(ctx, cfg)
		if err != nil {
			a.lastRecoveryLogPath = ""
			if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
				a.setState(StateCanceled)
				return
			}
			if isModelNotFound(err) {
				a.safeDo("ops.translate.model_not_found_dialog", func() {
					dialog.ShowError(fmt.Errorf("Model not found or no access. Please choose a different model in Settings."), a.window)
				})
			}
			logger.Error("Translation failed", "error", err)
			a.setState(StateFailure)
			return
		}
		a.lastRecoveryLogPath = result.RecoveryLogPath
		a.setState(stateForTranslationResult(result))
	})
}

func (a *focstApp) startRepair(logPath string) {
	a.setState(StateProcessing)

	// Mock flow for debug files
	if state, ok := debugStateForPath(logPath); ok {
		time.Sleep(1 * time.Second)
		a.setState(state)
		return
	}

	apiKey := a.sessionKey
	if apiKey == "" {
		apiKey, _ = auth.GetKey("gemini", false)
	}

	if apiKey == "" {
		a.flashRed()
		a.setState(StateNoKey)
		return
	}

	cfg := pipeline.Config{
		LogPath:           logPath,
		APIKey:            apiKey,
		RetryOnLongLines:  a.config.RetryOnLongLines,
		NoPromptCPL:       a.config.NoPromptCPL,
		NoPostprocess:     a.config.NoPostprocess,
		NoLangPostprocess: a.config.NoLangPostprocess,
		NamesMapping:      a.config.NamesMapping,
		OnProgress: func(p translator.TranslationProgress) {
			logger.Info("GUI Repair Progress", "chunk", p.ChunkIndex, "state", p.State)
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancelID := a.setActiveCancel(cancel)
	a.safeGo("ops.repair", func() {
		defer a.clearActiveCancel(cancelID)
		_, err := pipeline.RunRepair(ctx, cfg)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
				a.setState(StateCanceled)
				return
			}
			if isModelNotFound(err) {
				a.safeDo("ops.repair.model_not_found_dialog", func() {
					dialog.ShowError(fmt.Errorf("Model not found or no access. Please choose a different model in Settings."), a.window)
				})
			}
			logger.Error("Repair failed", "error", err)
			a.setState(StateFailure)
		} else {
			a.setState(StateSuccess)
		}
	})
}

func (a *focstApp) startNameExtraction(workType, title, year string, parent fyne.Window, onDone func(map[string]string, error)) {
	key, _ := auth.GetKey("openai", false) // names.Extractor currently uses openai client
	if key == "" {
		a.safeDo("ops.names.no_key_dialog", func() {
			dialog.ShowInformation("No API Key", "Please save an OpenAI Key in the Keys tab first.", parent)
		})
		a.safeDo("ops.names.no_key_done", func() {
			onDone(nil, errOpenAIKeyMissing)
		})
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancelID := a.setActiveCancel(cancel)

	a.safeGo("ops.names.extract", func() {
		defer a.clearActiveCancel(cancelID)
		client := openai.NewClient(key, "gpt-5.2")
		ex := names.NewExtractor(client)
		src := a.config.SourceLang
		tgt := a.config.TargetLang

		res, _, err := ex.Extract(ctx, workType, title, year, a.config.ExtractionMaxTokens, src, tgt)
		if errors.Is(err, context.Canceled) {
			return
		}

		mapping := make(map[string]string)
		if err == nil {
			for _, m := range res {
				mapping[m.Source] = m.Target
			}
		}

		a.safeDo("ops.names.done", func() {
			if err != nil && isModelNotFound(err) {
				dialog.ShowError(fmt.Errorf("Model not found or no access. Please choose a different model in Settings."), parent)
			}
			onDone(mapping, err)
		})
	})
}

func isModelNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "model_not_found") {
		return true
	}
	if strings.Contains(msg, "model not found or no access") {
		return true
	}
	if strings.Contains(msg, "does not exist or you do not have access to it") {
		return true
	}
	if strings.Contains(msg, "the model") && strings.Contains(msg, "does not exist") {
		return true
	}
	if strings.Contains(msg, "publisher model") && strings.Contains(msg, "was not found") {
		return true
	}
	if strings.Contains(msg, "models/") && strings.Contains(msg, "not found") && strings.Contains(msg, "generatecontent") {
		return true
	}
	if strings.Contains(msg, "not supported for generatecontent") && strings.Contains(msg, "models/") {
		return true
	}
	return false
}

func stateForTranslationResult(result pipeline.TranslationResult) AppState {
	switch result.Status {
	case pipeline.TranslationStatusSuccess:
		return StateSuccess
	case pipeline.TranslationStatusPartialSuccess:
		return StatePartialSuccess
	case pipeline.TranslationStatusFailure:
		return StateFailure
	case pipeline.TranslationStatusSkipped:
		return StateCanceled
	default:
		// Treat empty/unknown status as failure to avoid false-positive success states.
		return StateFailure
	}
}
