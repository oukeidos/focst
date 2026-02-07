package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/oukeidos/focst/internal/licenses"
)

func newLicensesCmd() *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:   "licenses",
		Short: "Show third-party license notices",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLicenses(cmd, full)
		},
		SilenceUsage: true,
	}
	cmd.SetUsageTemplate(subcommandUsageTemplate)
	cmd.Flags().BoolVar(&full, "full", false, "Print full license texts")
	return cmd
}

func runLicenses(cmd *cobra.Command, full bool) error {
	if full {
		return printFullLicenses(cmd)
	}
	return printNotices(cmd)
}

func printNotices(cmd *cobra.Command) error {
	text := licenses.NoticesText()
	if text == "" {
		return fmt.Errorf("embedded THIRD_PARTY_NOTICES is empty; run scripts/collect_third_party_licenses.py")
	}
	_, err := cmd.OutOrStdout().Write([]byte(text))
	return err
}

func printFullLicenses(cmd *cobra.Command) error {
	text := licenses.FullText()
	if text == "" {
		return fmt.Errorf("embedded THIRD_PARTY_LICENSES_FULL is empty; run scripts/collect_third_party_licenses.py")
	}
	_, err := cmd.OutOrStdout().Write([]byte(text))
	return err
}
