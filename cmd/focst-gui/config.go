package main

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"

	"github.com/oukeidos/focst/internal/logger"
	"github.com/oukeidos/focst/internal/names"
	"github.com/oukeidos/focst/internal/pipeline"
)

type AppConfig struct {
	SourceLang   string
	TargetLang   string
	Model        string
	LastDict     string
	NamesMapping map[string]string

	// Advanced Settings
	ChunkSize           int
	ContextSize         int
	Concurrency         int
	RetryOnLongLines    bool
	NoPromptCPL         bool
	NoPreprocess        bool
	NoPostprocess       bool
	NoLangPreprocess    bool
	NoLangPostprocess   bool
	ExtractionMaxTokens int
}

const (
	maxChunkSizeGUI     = 200
	maxContextSizeGUI   = 20
	maxExtractionTokens = 128000
)

func (a *focstApp) loadConfig() {
	p := a.window.Canvas().Scale() // Just to get app instance
	_ = p                          // dummy
	prefs := fyne.CurrentApp().Preferences()

	a.config.SourceLang = prefs.StringWithFallback("SourceLang", "ja")
	a.config.TargetLang = prefs.StringWithFallback("TargetLang", "ko")
	a.config.Model = prefs.StringWithFallback("Model", "gemini-3-flash-preview")
	a.config.LastDict = prefs.String("LastDict")
	a.config.NamesMapping = make(map[string]string)

	// Advanced
	a.config.ChunkSize = prefs.IntWithFallback("ChunkSize", 100)
	if a.config.ChunkSize > maxChunkSizeGUI {
		logger.Warn("Chunk size clamped", "requested", a.config.ChunkSize, "effective", maxChunkSizeGUI)
		a.config.ChunkSize = maxChunkSizeGUI
		prefs.SetInt("ChunkSize", a.config.ChunkSize)
	}
	a.config.ContextSize = prefs.IntWithFallback("ContextSize", 5)
	if a.config.ContextSize > maxContextSizeGUI {
		logger.Warn("Context size clamped", "requested", a.config.ContextSize, "effective", maxContextSizeGUI)
		a.config.ContextSize = maxContextSizeGUI
		prefs.SetInt("ContextSize", a.config.ContextSize)
	}
	a.config.Concurrency = prefs.IntWithFallback("Concurrency", 7)
	if clamped, changed := pipeline.ClampConcurrency(a.config.Concurrency); changed {
		logger.Warn("Concurrency clamped", "requested", a.config.Concurrency, "effective", clamped, "max", pipeline.MaxConcurrency)
		a.config.Concurrency = clamped
		prefs.SetInt("Concurrency", a.config.Concurrency)
	}
	a.config.RetryOnLongLines = prefs.BoolWithFallback("RetryOnLongLines", false)
	a.config.NoPromptCPL = prefs.BoolWithFallback("NoPromptCPL", false)
	a.config.NoPreprocess = prefs.BoolWithFallback("NoPreprocess", false)
	a.config.NoPostprocess = prefs.BoolWithFallback("NoPostprocess", false)
	a.config.NoLangPreprocess = prefs.BoolWithFallback("NoLangPreprocess", false)
	a.config.NoLangPostprocess = prefs.BoolWithFallback("NoLangPostprocess", false)
	a.config.ExtractionMaxTokens = prefs.IntWithFallback("ExtractionMaxTokens", 16384)
	if a.config.ExtractionMaxTokens > maxExtractionTokens {
		logger.Warn("Extraction max tokens clamped", "requested", a.config.ExtractionMaxTokens, "effective", maxExtractionTokens)
		a.config.ExtractionMaxTokens = maxExtractionTokens
		prefs.SetInt("ExtractionMaxTokens", a.config.ExtractionMaxTokens)
	}

	if a.config.LastDict != "" && a.config.LastDict != "None (Empty)" {
		home, _ := os.UserHomeDir()
		path := filepath.Join(home, ".focst", "names", a.config.LastDict+".json")
		sourceCode, targetCode := a.getDictionaryMeta(a.config.LastDict)
		if sourceCode == "" {
			sourceCode = a.config.SourceLang
		}
		if targetCode == "" {
			targetCode = a.config.TargetLang
		}
		if mapping, err := names.LoadMappingFile(path, sourceCode, targetCode); err == nil {
			a.config.NamesMapping = mapping
		} else {
			logger.Error("Failed to load dictionary", "path", path, "source", sourceCode, "target", targetCode, "error", err)
		}
	}

	// Ensure names directory exists
	home, _ := os.UserHomeDir()
	namesDir := filepath.Join(home, ".focst", "names")
	os.MkdirAll(namesDir, 0700)
}

func (a *focstApp) saveConfig() {
	prefs := fyne.CurrentApp().Preferences()
	prefs.SetString("SourceLang", a.config.SourceLang)
	prefs.SetString("TargetLang", a.config.TargetLang)
	prefs.SetString("Model", a.config.Model)
	prefs.SetString("LastDict", a.config.LastDict)

	// Advanced
	prefs.SetInt("ChunkSize", a.config.ChunkSize)
	prefs.SetInt("ContextSize", a.config.ContextSize)
	prefs.SetInt("Concurrency", a.config.Concurrency)
	prefs.SetBool("RetryOnLongLines", a.config.RetryOnLongLines)
	prefs.SetBool("NoPromptCPL", a.config.NoPromptCPL)
	prefs.SetBool("NoPreprocess", a.config.NoPreprocess)
	prefs.SetBool("NoPostprocess", a.config.NoPostprocess)
	prefs.SetBool("NoLangPreprocess", a.config.NoLangPreprocess)
	prefs.SetBool("NoLangPostprocess", a.config.NoLangPostprocess)
	prefs.SetInt("ExtractionMaxTokens", a.config.ExtractionMaxTokens)
}
