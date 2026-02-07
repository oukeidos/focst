package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/oukeidos/focst/internal/licenses"
)

func newDisclaimerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disclaimer",
		Short: "Show the full disclaimer",
		RunE: func(cmd *cobra.Command, args []string) error {
			text := licenses.DisclaimerText()
			if text == "" {
				return fmt.Errorf("embedded DISCLAIMER is empty; run scripts/collect_third_party_licenses.py")
			}
			_, err := cmd.OutOrStdout().Write([]byte(text))
			return err
		},
		SilenceUsage: true,
	}
	cmd.SetUsageTemplate(subcommandUsageTemplate)
	return cmd
}
