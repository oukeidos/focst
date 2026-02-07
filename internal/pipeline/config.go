package pipeline

import (
	"fmt"

	"github.com/oukeidos/focst/internal/translator"
)

// Config holds all configuration required for running a translation or repair session.
type Config struct {
	// IO Paths
	InputPath  string
	OutputPath string
	LogPath    string // Optional: for JSONL logs in CLI or specific log file in GUI

	// API Configuration
	APIKey string
	Model  string

	// Processing Parameters
	ChunkSize        int
	ContextSize      int
	Concurrency      int
	RetryOnLongLines bool
	NoPromptCPL      bool

	// Flags
	NoPreprocess      bool
	NoPostprocess     bool
	Overwrite         bool // If true, overwrite output file without asking (CLI mostly)
	ForceRepair       bool // If true, ignore unusable existing output during repair
	NoLangPreprocess  bool
	NoLangPostprocess bool

	// Languages
	SourceLang string
	TargetLang string

	// names Mapping (Source Name -> Target Name)
	NamesMapping map[string]string
	NamesPath    string

	// Callbacks
	// OnProgress is called with translation progress updates.
	OnProgress func(translator.TranslationProgress)

	// OnConfirmOverwrite is called when the output file exists.
	// It should return true if the file should be overwritten.
	// If nil, it assumes Overwrite flag accounts for it or it's already checked.
	OnConfirmOverwrite func(path string) bool
}

const (
	MinConcurrency = 1
	MaxConcurrency = 20
	MaxChunkSize   = 200
	MaxContextSize = 20
)

func ClampConcurrency(value int) (int, bool) {
	if value < MinConcurrency {
		return MinConcurrency, true
	}
	if value > MaxConcurrency {
		return MaxConcurrency, true
	}
	return value, false
}

// Normalize applies safe bounds to config values and returns any adjustments.
func (c Config) Normalize() (Config, []string) {
	var notes []string
	if clamped, changed := ClampConcurrency(c.Concurrency); changed {
		notes = append(notes, fmt.Sprintf("concurrency clamped from %d to %d (max %d)", c.Concurrency, clamped, MaxConcurrency))
		c.Concurrency = clamped
	}
	if c.ChunkSize > MaxChunkSize {
		notes = append(notes, fmt.Sprintf("chunk-size clamped from %d to %d (max %d)", c.ChunkSize, MaxChunkSize, MaxChunkSize))
		c.ChunkSize = MaxChunkSize
	}
	if c.ContextSize > MaxContextSize {
		notes = append(notes, fmt.Sprintf("context-size clamped from %d to %d (max %d)", c.ContextSize, MaxContextSize, MaxContextSize))
		c.ContextSize = MaxContextSize
	}
	return c, notes
}

// Validate checks if the configuration is valid.
func (c Config) Validate() error {
	if c.ChunkSize <= 0 {
		return fmt.Errorf("chunkSize must be greater than 0, got %d", c.ChunkSize)
	}
	if c.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be greater than 0, got %d", c.Concurrency)
	}
	if c.ContextSize < 0 {
		return fmt.Errorf("contextSize must be 0 or greater, got %d", c.ContextSize)
	}
	if c.APIKey == "" {
		return fmt.Errorf("API key is required")
	}
	return nil
}

// ValidateRepairRuntime checks only runtime config required for repair.
// Log-derived settings (chunk/concurrency/context/model/lang) are validated on the session log.
func (c Config) ValidateRepairRuntime() error {
	if c.APIKey == "" {
		return fmt.Errorf("API key is required")
	}
	return nil
}
