package cmd

import (
	"fmt"

	"github.com/canonical/concierge/internal/concierge"
	"github.com/canonical/concierge/internal/config"
	"github.com/spf13/cobra"
)

// statusCmd reports the status of concierge provisioning on a machine.
func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Report the status of `concierge` on the machine.",
		Long: `Report the status of 'concierge' on the machine.

Reports one of 'provisioning', 'succeeded' or 'failed'.
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

			status, err := mgr.Status()
			if err != nil {
				return err
			}

			fmt.Printf("%s\n", status)
			return nil
		},
	}
}
