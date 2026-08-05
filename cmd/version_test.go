package cmd

import (
	"bytes"
	"testing"
)

// TestVersionSubcommandMatchesFlag asserts that `concierge version` produces
// the same output as `concierge --version`, so the two paths stay in sync.
func TestVersionSubcommandMatchesFlag(t *testing.T) {
	flagOut := &bytes.Buffer{}
	flagCmd := rootCmd()
	flagCmd.SetOut(flagOut)
	flagCmd.SetErr(flagOut)
	flagCmd.SetArgs([]string{"--version"})
	if err := flagCmd.Execute(); err != nil {
		t.Fatalf("--version flag failed: %v", err)
	}

	subOut := &bytes.Buffer{}
	subCmd := rootCmd()
	subCmd.SetOut(subOut)
	subCmd.SetErr(subOut)
	subCmd.SetArgs([]string{"version"})
	if err := subCmd.Execute(); err != nil {
		t.Fatalf("version subcommand failed: %v", err)
	}

	if flagOut.String() != subOut.String() {
		t.Fatalf("output mismatch:\n--version: %q\nversion:   %q", flagOut.String(), subOut.String())
	}

	if subOut.Len() == 0 {
		t.Fatalf("version subcommand produced no output")
	}
}
