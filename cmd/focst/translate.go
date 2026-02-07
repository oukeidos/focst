package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oukeidos/focst/internal/cleanup"
	"github.com/oukeidos/focst/internal/files"
	"github.com/oukeidos/focst/internal/logger"
	"github.com/oukeidos/focst/internal/pipeline"
	"github.com/oukeidos/focst/internal/prompt"
	"github.com/oukeidos/focst/internal/translator"
	"github.com/spf13/cobra"
)

type translateOptions struct {
	modelName         string
	chunkSize         int
	contextSize       int
	concurrency       int
	validateCPL       bool
	noPromptCPL       bool
	yes               bool
	logFilePath       string
	namesPath         string
	noPreprocess      bool
	noPostprocess     bool
	noLangPreprocess  bool
	noLangPostprocess bool
	sourceLangCode    string
	targetLangCode    string
	allowEnv          bool
	envOnly           bool
	debug             bool
}

func newTranslateCmd() *cobra.Command {
	opts := translateOptions{}
	cmd := &cobra.Command{
		Use:   "translate <input.srt> <output.srt>",
		Short: "Translate subtitle files using Gemini",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				_ = cmd.Usage()
				return fmt.Errorf("input and output files are required")
			}
			return runTranslate(cmd, args, &opts)
		},
		SilenceUsage: true,
	}

	cmd.SetUsageTemplate(subcommandUsageTemplate)
	addTranslateFlags(cmd, &opts)
	return cmd
}

func addTranslateFlags(cmd *cobra.Command, opts *translateOptions) {
	cmd.Flags().StringVar(&opts.modelName, "model", "gemini-3-flash-preview", "Gemini model name")
	cmd.Flags().IntVar(&opts.chunkSize, "chunk-size", 100, "Number of segments per chunk")
	cmd.Flags().IntVar(&opts.contextSize, "context-size", 5, "Number of context segments before/after")
	cmd.Flags().IntVar(&opts.concurrency, "concurrency", 7, "Number of concurrent API requests (1-20)")
	cmd.Flags().BoolVar(&opts.validateCPL, "retry-on-long-line", false, "Retry validation if line > 24 graphemes (default false)")
	cmd.Flags().BoolVar(&opts.noPromptCPL, "no-prompt-cpl", false, "Disable CPL constraints in the translation prompt")
	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Overwrite output file without asking")
	cmd.Flags().StringVar(&opts.logFilePath, "log-file", "", "Path to save machine-readable JSONL logs")
	cmd.Flags().StringVar(&opts.namesPath, "names", "", "Path to character name mapping JSON file")
	cmd.Flags().BoolVar(&opts.noPreprocess, "no-preprocess", false, "Disable all preprocessing (bracket removal, symbol filtering)")
	cmd.Flags().BoolVar(&opts.noLangPreprocess, "no-lang-preprocess", false, "Disable language-specific preprocessing only")
	cmd.Flags().BoolVar(&opts.noPostprocess, "no-postprocess", false, "Disable all post-processing (punctuation, timing correction)")
	cmd.Flags().BoolVar(&opts.noLangPostprocess, "no-lang-postprocess", false, "Disable language-specific post-processing only")
	cmd.Flags().StringVar(&opts.sourceLangCode, "source", "ja", "Source language code (default: ja)")
	cmd.Flags().StringVar(&opts.targetLangCode, "target", "ko", "Target language code (default: ko)")
	cmd.Flags().BoolVar(&opts.allowEnv, "allow-env", false, "Allow reading API key from environment variables")
	cmd.Flags().BoolVar(&opts.envOnly, "env-only", false, "Use only environment variables for API keys")
	cmd.Flags().BoolVar(&opts.debug, "debug", false, "Enable debug logging")
}

func runTranslate(cmd *cobra.Command, args []string, opts *translateOptions) error {
	if len(args) < 2 {
		return fmt.Errorf("input and output files are required")
	}
	if len(args) > 2 {
		fmt.Fprintf(os.Stderr, "Warning: expected 2 arguments but got %d. Did you forget quotes around file paths?\n", len(args))
		fmt.Fprintf(os.Stderr, "  Using input: %s\n", args[0])
		fmt.Fprintf(os.Stderr, "  Using output: %s\n", args[1])
	}
	if err := validateSubtitlePathExtensions(args[0], args[1]); err != nil {
		return err
	}

	logLevel := logger.LevelInfo
	if opts.debug {
		logLevel = logger.LevelDebug
	}
	var logFileW io.Writer
	if opts.logFilePath != "" {
		if err := files.RejectSymlinkPath(opts.logFilePath); err != nil {
			return err
		}
		f, err := os.OpenFile(opts.logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		cleanup.Register(f.Close)
		logFileW = f
	}
	logger.Init(logLevel, logFileW)

	startTime := time.Now()

	actualKey, source, err := resolveAPIKey("gemini", opts.allowEnv, opts.envOnly)
	if err != nil {
		return err
	}
	logger.Info("Using API Key", "service", "gemini", "source", source)

	var nameMapping map[string]string
	if opts.namesPath != "" {
		nameMapping, err = loadNamesMapping(opts.namesPath, opts.sourceLangCode, opts.targetLangCode)
		if err != nil {
			return err
		}
	}

	cfg := pipeline.Config{
		InputPath:         args[0],
		OutputPath:        args[1],
		LogPath:           opts.logFilePath,
		APIKey:            actualKey,
		Model:             opts.modelName,
		ChunkSize:         opts.chunkSize,
		ContextSize:       opts.contextSize,
		Concurrency:       opts.concurrency,
		RetryOnLongLines:  opts.validateCPL,
		NoPromptCPL:       opts.noPromptCPL,
		NoPreprocess:      opts.noPreprocess,
		NoPostprocess:     opts.noPostprocess,
		NoLangPreprocess:  opts.noLangPreprocess,
		NoLangPostprocess: opts.noLangPostprocess,
		Overwrite:         opts.yes,
		SourceLang:        opts.sourceLangCode,
		TargetLang:        opts.targetLangCode,
		NamesMapping:      nameMapping,
		NamesPath:         opts.namesPath,
		OnProgress: func(p translator.TranslationProgress) {
			switch p.State {
			case translator.StateCompleted:
				logger.Info("Chunk completed", "index", p.ChunkIndex, "total", p.TotalChunks)
			case translator.StateInProgress:
				logger.Warn("Chunk retry", "index", p.ChunkIndex, "attempt", p.Attempt, "error", p.Error)
			}
		},
		OnConfirmOverwrite: func(path string) bool {
			confirmed, err := prompt.DefaultConfirmer().ConfirmOverwrite(path, opts.yes)
			if err != nil {
				logger.Error("Overwrite confirmation failed", "error", err)
				return false
			}
			return confirmed
		},
	}

	ctx, stop := signalContext()
	defer stop()
	result, err := pipeline.RunTranslation(ctx, cfg)

	// Always print stats (even on partial success)
	printUsageStats(&result.Usage, time.Since(startTime), opts.modelName)

	if err != nil {
		if ctx.Err() != nil {
			logger.Warn("Translation canceled", "error", err)
			return nil
		}
		return err
	}

	return translationStatusError(result)
}

func translationStatusError(result pipeline.TranslationResult) error {
	switch result.Status {
	case pipeline.TranslationStatusSuccess:
		return nil
	case pipeline.TranslationStatusSkipped:
		return nil
	case pipeline.TranslationStatusPartialSuccess, pipeline.TranslationStatusFailure:
		if result.RecoveryLogPath != "" {
			return fmt.Errorf("translation finished with status: %s (recovery log: %s)", result.Status, result.RecoveryLogPath)
		}
		return fmt.Errorf("translation finished with status: %s", result.Status)
	default:
		return fmt.Errorf("translation finished with unknown status: %q", result.Status)
	}
}

var supportedSubtitleExtensions = map[string]struct{}{
	".srt":  {},
	".vtt":  {},
	".ssa":  {},
	".ass":  {},
	".ttml": {},
	".stl":  {},
}

const supportedSubtitleExtensionsLabel = ".srt, .vtt, .ssa, .ass, .ttml, .stl"

func validateSubtitlePathExtensions(inputPath, outputPath string) error {
	if err := validateSubtitleExtension("input", inputPath); err != nil {
		return err
	}
	if err := validateSubtitleExtension("output", outputPath); err != nil {
		return err
	}
	return nil
}

func validateSubtitleExtension(kind, path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	if _, ok := supportedSubtitleExtensions[ext]; ok {
		return nil
	}
	if ext == "" {
		ext = "(none)"
	}
	return fmt.Errorf("unsupported %s extension %q (supported: %s)", kind, ext, supportedSubtitleExtensionsLabel)
}
