package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestOverwriteFlag_AcceptsYesAndShorthand(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "root_shorthand", args: []string{"-y"}},
		{name: "root_long", args: []string{"--yes"}},
		{name: "names_shorthand", args: []string{"names", "-y"}},
		{name: "names_long", args: []string{"names", "--yes"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := executeCommand(t, tc.args...)
			if err == nil {
				t.Fatalf("expected command error from missing required args, got nil")
			}
			if strings.Contains(out, "unknown shorthand flag: 'y'") || strings.Contains(out, "unknown flag: --yes") {
				t.Fatalf("expected --yes/-y to be parsed, got output: %s", out)
			}
		})
	}
}

func TestOverwriteFlag_RejectsDeprecatedLongY(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "root_deprecated_long_y", args: []string{"--y"}},
		{name: "names_deprecated_long_y", args: []string{"names", "--y"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := executeCommand(t, tc.args...)
			if err == nil {
				t.Fatalf("expected unknown flag error for --y")
			}
			if !strings.Contains(out, "unknown flag: --y") {
				t.Fatalf("expected unknown flag: --y, got output: %s", out)
			}
		})
	}
}

func TestTranslateModelDefault(t *testing.T) {
	for _, tc := range []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "root", cmd: newRootCmd()},
		{name: "translate", cmd: newTranslateCmd()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flag := tc.cmd.Flags().Lookup("model")
			if flag == nil {
				t.Fatal("model flag not found")
			}
			if flag.DefValue != "gemini-3.7-flash" {
				t.Fatalf("model default = %q, want %q", flag.DefValue, "gemini-3.7-flash")
			}
		})
	}
}
