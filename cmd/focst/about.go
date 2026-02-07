package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newAboutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "about",
		Short: "Show a short description and link",
		Run: func(cmd *cobra.Command, args []string) {
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "focst — Format Constrained Subtitle Translator")
			fmt.Fprintln(out, "https://github.com/oukeidos/focst")
		},
	}
	cmd.SetUsageTemplate(subcommandUsageTemplate)
	return cmd
}
