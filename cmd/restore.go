package cmd

import (
	"fmt"

	"github.com/canonical/concierge/internal/concierge"
	"github.com/canonical/concierge/internal/config"
	"github.com/spf13/cobra"
)

// restoreCmd constructs the `restore` subcommand
func restoreCmd() *cobra.Command {
	return &cobra.Command{
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

			conf, err := config.NewConfig(cmd, flags)
			if err != nil {
				return fmt.Errorf("failed to configure concierge: %w", err)
			}

			mgr, err := concierge.NewManager(conf)
			if err != nil {
				return err
			}

			return mgr.Restore()
		},
	}
}
