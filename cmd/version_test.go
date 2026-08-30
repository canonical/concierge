package cmd

import (
	"bytes"
	"testing"
)

// TestVersionSubcommandMatchesFlag asserts that `concierge version` produces
// the same output as `concierge --version`, so the two paths stay in sync.
// Each path gets its own stdout and stderr buffer, so that a regression
// sending the version to stderr fails rather than looking identical.
func TestVersionSubcommandMatchesFlag(t *testing.T) {
	flagOut, flagErr := &bytes.Buffer{}, &bytes.Buffer{}
	flagCmd := rootCmd()
	flagCmd.SetOut(flagOut)
	flagCmd.SetErr(flagErr)
	flagCmd.SetArgs([]string{"--version"})
	if err := flagCmd.Execute(); err != nil {
		t.Fatalf("--version flag failed: %v", err)
	}

	subOut, subErr := &bytes.Buffer{}, &bytes.Buffer{}
	subCmd := rootCmd()
	subCmd.SetOut(subOut)
	subCmd.SetErr(subErr)
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

	if flagErr.Len() != 0 {
		t.Fatalf("--version flag wrote to stderr: %q", flagErr.String())
	}

	if subErr.Len() != 0 {
		t.Fatalf("version subcommand wrote to stderr: %q", subErr.String())
	}
}
