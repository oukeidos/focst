package pipeline

import (
	"context"
	"fmt"
	"os"

	"github.com/oukeidos/focst/internal/files"
	"github.com/oukeidos/focst/internal/gemini"
	"github.com/oukeidos/focst/internal/language"
	"github.com/oukeidos/focst/internal/logger"
	"github.com/oukeidos/focst/internal/names"
	"github.com/oukeidos/focst/internal/recovery"
	"github.com/oukeidos/focst/internal/srt"
	"github.com/oukeidos/focst/internal/translator"
)

// RepairResult contains the result of a repair operation.
type RepairResult struct {
	Model string
	Usage gemini.UsageMetadata
}

// RunRepair executes the session repair pipeline.
func RunRepair(ctx context.Context, cfg Config) (RepairResult, error) {
	// 1. Validation & Load Log
	if cfg.LogPath == "" {
		return RepairResult{}, fmt.Errorf("log file path is required for repair")
	}

	logFile, origHash, err := recovery.LoadSessionLogWithHash(cfg.LogPath)
	if err != nil {
		return RepairResult{}, fmt.Errorf("failed to load recovery log: %w", err)
	}
	if err := logFile.Validate(); err != nil {
		return RepairResult{}, fmt.Errorf("invalid recovery log: %w", err)
	}
	runtimeLog, err := resolveRuntimeSessionLog(cfg.LogPath, logFile)
	if err != nil {
		return RepairResult{}, err
	}

	// Resolve output path relative to log file location.
	resolvedOutputPath := recovery.ResolveOutputPath(cfg.LogPath, logFile.OutputPath)

	if err := cfg.ValidateRepairRuntime(); err != nil {
		return RepairResult{}, fmt.Errorf("invalid configuration: %w", err)
	}
	if err := files.RejectSymlinkPath(resolvedOutputPath); err != nil {
		return RepairResult{}, err
	}
	if err := files.RejectSymlinkPath(cfg.LogPath); err != nil {
		return RepairResult{}, err
	}

	segments, err := srt.Load(runtimeLog.InputPath)
	if err != nil {
		return RepairResult{}, fmt.Errorf("failed to load subtitle file: %w", err)
	}
	if err := srt.Validate(segments); err != nil {
		return RepairResult{}, fmt.Errorf("invalid subtitle file: %w", err)
	}
	inputHash, err := recovery.HashFileHex(runtimeLog.InputPath)
	if err != nil {
		return RepairResult{}, fmt.Errorf("failed to compute input hash: %w", err)
	}
	if inputHash != logFile.InputHash {
		return RepairResult{}, fmt.Errorf("input file content mismatch: expected %s, got %s", logFile.InputHash, inputHash)
	}
	if !logFile.NoPreprocess {
		segments = srt.PreprocessForPathWithOptions(segments, logFile.SourceLang, runtimeLog.InputPath, !logFile.NoLangPreprocess)
	}
	segmentsChecksum := srt.SegmentsChecksumHex(segments)
	if segmentsChecksum != logFile.SegmentsChecksum {
		return RepairResult{}, fmt.Errorf("segment checksum mismatch: expected %s, got %s", logFile.SegmentsChecksum, segmentsChecksum)
	}

	// 2. Setup Client & Translator
	// Use model from log, but allow API key from config (runtime)
	gClient, err := gemini.NewClient(ctx, cfg.APIKey, logFile.Model)
	if err != nil {
		return RepairResult{}, fmt.Errorf("failed to create Gemini client: %w", err)
	}
	defer gClient.Close()

	srcLang, _ := language.GetLanguage(runtimeLog.SourceLang)
	tgtLang, _ := language.GetLanguage(runtimeLog.TargetLang)

	tr, err := translator.NewTranslator(gClient, runtimeLog.ChunkSize, runtimeLog.ContextSize, runtimeLog.Concurrency, cfg.RetryOnLongLines, srcLang, tgtLang)
	if err != nil {
		return RepairResult{}, fmt.Errorf("failed to initialize translator: %w", err)
	}
	tr.SetPromptCPL(!runtimeLog.NoPromptCPL)
	if runtimeLog.NamesPath != "" {
		nameMapping, err := names.LoadMappingFile(runtimeLog.NamesPath, runtimeLog.SourceLang, runtimeLog.TargetLang)
		if err != nil {
			return RepairResult{}, fmt.Errorf("failed to load names mapping: %w", err)
		}
		tr.SetNamesMapping(nameMapping)
		logger.Info("Loaded character name mapping", "count", len(nameMapping), "path", runtimeLog.NamesPath)
	}

	// 3. Repair
	logger.Info("Starting repair", "model", runtimeLog.Model, "failed_chunks", len(runtimeLog.FailedChunks))
	translated, newFailed, err := recovery.Repair(ctx, tr, &runtimeLog, resolvedOutputPath, cfg.ForceRepair, cfg.OnProgress)
	if err != nil {
		return RepairResult{}, fmt.Errorf("repair failed: %w", err)
	}

	// 4. Handle Results
	if len(newFailed) == 0 {
		status := "Success"
		logger.Info("Repair finished", "status", status)

		outSegments := translated
		if !logFile.NoPostprocess {
			logger.Info("Performing post-processing")
			outSegments = srt.PostprocessWithOptions(outSegments, tgtLang.Code, tgtLang.DefaultCPS, !logFile.NoLangPostprocess)
		} else {
			logger.Info("Post-processing skipped")
		}

		// Use resolved output path
		logger.Info("Saving results to output file", "path", resolvedOutputPath)
		if err := srt.Save(resolvedOutputPath, outSegments); err != nil {
			return RepairResult{}, fmt.Errorf("failed to save output file: %w", err)
		}
		logger.Info("Saved results", "path", resolvedOutputPath)

		// Clean up log file on success
		if currentHash, err := recovery.HashFile(cfg.LogPath); err != nil {
			logger.Warn("Failed to read session log for verification", "path", cfg.LogPath, "error", err)
		} else if currentHash != origHash {
			logger.Warn("Session log content changed; skipping delete", "path", cfg.LogPath)
		} else if err := os.Remove(cfg.LogPath); err != nil {
			logger.Warn("Failed to remove session log after success", "path", cfg.LogPath, "error", err)
		}
	} else {
		status := recovery.CalculateStatus(len(newFailed), logFile.TotalChunks)
		logger.Info("Repair finished", "status", status)

		logFile.FailedChunks = newFailed
		logFile.Status = status
		if err := recovery.SaveSessionLog(cfg.LogPath, logFile); err != nil {
			logger.Error("Failed to update recovery log", "error", err)
		} else {
			logger.Warn("Partial repair - session log updated", "path", cfg.LogPath)
		}
		return RepairResult{Model: runtimeLog.Model, Usage: tr.GetUsage()}, fmt.Errorf("repair finished with %d failed chunks", len(newFailed))
	}

	return RepairResult{Model: runtimeLog.Model, Usage: tr.GetUsage()}, nil
}

func resolveRuntimeSessionLog(logPath string, logFile *recovery.SessionLog) (recovery.SessionLog, error) {
	runtimeLog := *logFile
	resolvedInputPath := recovery.ResolveInputPath(logPath, logFile.InputPath)
	if _, err := os.Stat(resolvedInputPath); err != nil {
		return recovery.SessionLog{}, fmt.Errorf("invalid recovery log: input file not found: %s", logFile.InputPath)
	}
	runtimeLog.InputPath = resolvedInputPath

	if logFile.NamesPath != "" {
		resolvedNamesPath := recovery.ResolveInputPath(logPath, logFile.NamesPath)
		if _, err := os.Stat(resolvedNamesPath); err != nil {
			return recovery.SessionLog{}, fmt.Errorf("invalid recovery log: names_path not found: %s", logFile.NamesPath)
		}
		runtimeLog.NamesPath = resolvedNamesPath
	}

	return runtimeLog, nil
}
