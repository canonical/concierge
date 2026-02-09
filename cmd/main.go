package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/user"

	"github.com/spf13/pflag"
)

var (
	version string = "dev"
	commit  string = "dev"
)

// Execute runs the root command and exits the program if it fails.
func Execute() {
	cmd := rootCmd()

	err := cmd.Execute()
	if err != nil {
		slog.Error("concierge failed", "error", err.Error())
		os.Exit(1)
	}
}

func parseLoggingFlags(flags *pflag.FlagSet) {
	verbose, _ := flags.GetBool("verbose")
	trace, _ := flags.GetBool("trace")
	dryRun, _ := flags.GetBool("dry-run")

	// Determine log level: --verbose/--trace take precedence, then --dry-run defaults to error
	level := slog.LevelInfo
	if verbose || trace {
		level = slog.LevelDebug
	} else if dryRun {
		level = slog.LevelError
	}

	// Setup the TextHandler and ensure our configured logger is the default.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	logger := slog.New(h)
	slog.SetDefault(logger)
}

func checkUser() error {
	user, err := user.Current()
	if err != nil {
		return err
	}

	if user.Uid != "0" {
		return fmt.Errorf("this command should be run with `sudo`, or as `root`")
	}

	return nil
}
