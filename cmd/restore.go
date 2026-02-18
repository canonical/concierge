package cmd

import (
	"github.com/canonical/concierge/internal/concierge"
	"github.com/canonical/concierge/internal/config"
	"github.com/spf13/cobra"
)

// restoreCmd constructs the `restore` subcommand
func restoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Run the reverse of `concierge prepare`.",
		Long: `Run the reverse of 'concierge prepare'.

If the machine already had a given snap or configuration
prior to running 'prepare', this will not be taken into account during 'restore'.
Running 'restore' is the literal opposite of 'prepare', so any packages,
files or configuration that would normally be created during 'prepare' will be removed.
		`,
		SilenceErrors: true,
		SilenceUsage:  true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			parseLoggingFlags(cmd.Flags())
			return checkUser()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()

			// Restore uses the cached config from prepare, not a config file.
			// We only need CLI flags here; loadRuntimeConfig fills in the rest.
			dryRun, _ := flags.GetBool("dry-run")
			verbose, _ := flags.GetBool("verbose")
			trace, _ := flags.GetBool("trace")

			conf := &config.Config{
				DryRun:  dryRun,
				Verbose: verbose,
				Trace:   trace,
			}

			mgr, err := concierge.NewManager(conf)
			if err != nil {
				return err
			}

			return mgr.Restore()
		},
	}

	flags := cmd.Flags()
	flags.Bool("dry-run", false, "show what would be done without making changes")
	flags.Bool("verbose", false, "enable verbose logging")
	flags.Bool("trace", false, "enable trace logging")

	return cmd
}
