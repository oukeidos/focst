package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/oukeidos/focst/internal/logger"
	"github.com/oukeidos/focst/internal/pipeline"
	"github.com/oukeidos/focst/internal/translator"
	"github.com/spf13/cobra"
)

var (
	runRepairPipeline    = pipeline.RunRepair
	printRepairStatsFunc = printUsageStats
)

type repairOptions struct {
	forceRepair bool
	allowEnv    bool
	envOnly     bool
	debug       bool
}

func newRepairCmd() *cobra.Command {
	opts := repairOptions{}
	cmd := &cobra.Command{
		Use:   "repair <session_log.json>",
		Short: "Resume a failed translation using a session log",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				_ = cmd.Usage()
				return fmt.Errorf("session_log.json is required")
			}
			return runRepair(cmd, args, &opts)
		},
		SilenceUsage: true,
	}

	cmd.SetUsageTemplate(subcommandUsageTemplate)
	cmd.Flags().BoolVar(&opts.forceRepair, "force-repair", false, "Ignore existing output and re-translate all chunks")
	cmd.Flags().BoolVar(&opts.allowEnv, "allow-env", false, "Allow reading API key from environment variables")
	cmd.Flags().BoolVar(&opts.envOnly, "env-only", false, "Use only environment variables for API keys")
	cmd.Flags().BoolVar(&opts.debug, "debug", false, "Enable debug logging")
	return cmd
}

func runRepair(cmd *cobra.Command, args []string, opts *repairOptions) error {
	startTime := time.Now()
	logPath := args[0]

	logLevel := logger.LevelInfo
	if opts.debug {
		logLevel = logger.LevelDebug
	}
	logger.Init(logLevel, nil)

	actualKey, source, err := resolveAPIKey("gemini", opts.allowEnv, opts.envOnly)
	if err != nil {
		return err
	}
	logger.Info("Using API Key", "service", "gemini", "source", source)

	cfg := pipeline.Config{
		LogPath:          logPath,
		APIKey:           actualKey,
		RetryOnLongLines: false,
		ForceRepair:      opts.forceRepair,
		OnProgress: func(p translator.TranslationProgress) {
			switch p.State {
			case translator.StateCompleted:
				logger.Info("Chunk completed", "chunk", p.ChunkIndex)
			case translator.StateInProgress:
				logger.Warn("Chunk retry", "chunk", p.ChunkIndex, "attempt", p.Attempt, "error", p.Error)
			}
		},
	}

	ctx, stop := signalContext()
	defer stop()
	result, err := runRepairPipeline(ctx, cfg)

	if err != nil {
		if ctx.Err() != nil {
			logger.Warn("Repair canceled", "error", err)
			return nil
		}
		if shouldPrintRepairStats(result) {
			printRepairStatsFunc(&result.Usage, time.Since(startTime), result.Model)
		}
		return err
	}
	printRepairStatsFunc(&result.Usage, time.Since(startTime), result.Model)

	return nil
}

func shouldPrintRepairStats(result pipeline.RepairResult) bool {
	if strings.TrimSpace(result.Model) != "" {
		return true
	}
	usage := result.Usage
	return usage.PromptTokenCount > 0 ||
		usage.CandidatesTokenCount > 0 ||
		usage.TotalTokenCount > 0 ||
		usage.WebSearchCount > 0
}
