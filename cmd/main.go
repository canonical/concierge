package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"runtime/debug"
	"strings"

	"github.com/spf13/pflag"
)

var (
	// Overriden by ldflags in goreleaser builds/snaps.
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

	logLevel := new(slog.LevelVar)

	// Set the default log level to "DEBUG" if verbose is specified.
	level := slog.LevelInfo
	if verbose || trace {
		level = slog.LevelDebug
	}

	// Setup the TextHandler and ensure our configured logger is the default.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	logger := slog.New(h)
	slog.SetDefault(logger)
	logLevel.Set(level)
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

func appVersion() string {
	// If not built with goreleaser, use runtime version to cover cases like:
	// - go run github.com/...
	// - go install github.com/...
	if info, ok := debug.ReadBuildInfo(); ok {
		if version == "dev" && info.Main.Version != "" {
			version = strings.TrimPrefix(info.Main.Version, "v")
		}
		if commit == "dev" {
			if c := getBuildSetting(info, "vcs.revision"); c != "" {
				commit = c
			}
		}
	}

	return fmt.Sprintf("%s (%s)", version, commit)
}

func getBuildSetting(info *debug.BuildInfo, key string) string {
	for _, s := range info.Settings {
		if s.Key == key {
			return s.Value
		}
	}
	return ""
}
