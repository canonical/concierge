package cmd

import (
	"fmt"

	"github.com/canonical/concierge/internal/concierge"
	"github.com/canonical/concierge/internal/config"
	"github.com/spf13/cobra"
)

// prepareCmd constructs the `prepare` subcommand
func prepareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Provision the machine according to the configuration.",
		Long: `Provision the machine according to the configuration.

Configuration is by flags/environment variables, or by configuration file. The configuration file
must be in the current working directory and named 'concierge.yaml', or the path specified using
the '-c' flag.

There are 3 presets available by default: 'machine', 'k8s' and 'dev'.

Some aspects of presets and config files can be overridden using flags such as '--juju-channel'.
Each of the override flags has an environment variable equivalent, 
such as 'CONCIERGE_JUJU_CHANNEL'.

More information at https://github.com/canonical/concierge.
`,
		SilenceErrors: true,
		SilenceUsage:  true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			parseLoggingFlags(cmd.Flags())
			return checkUser()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := cmd.Flags()

			configFile, _ := flags.GetString("config")
			preset, _ := flags.GetString("preset")

			// Concierge cannot merge a preset & manual configuration
			if len(preset) > 0 && len(configFile) > 0 {
				return fmt.Errorf("cannot proceed with both preset and configuration file specified")
			}

			conf, err := config.NewConfig(cmd, flags)
			if err != nil {
				return fmt.Errorf("failed to configure concierge: %w", err)
			}

			mgr, err := concierge.NewManager(conf)
			if err != nil {
				return err
			}

			return mgr.Prepare()
		},
	}

	flags := cmd.Flags()
	flags.StringP("config", "c", "", "path to a specific config file to use")
	flags.StringP("preset", "p", "", "config preset to use (k8s | machine | dev)")
	flags.Bool("disable-juju", false, "disable the installation and bootstrap of juju")
	flags.String("juju-channel", "", "override the snap channel for juju")
	flags.String("k8s-channel", "", "override snap channel for the k8s snap")
	flags.String("microk8s-channel", "", "override snap channel for microk8s")
	flags.String("lxd-channel", "", "override snap channel for lxd")
	flags.String("charmcraft-channel", "", "override snap channel for charmcraft")
	flags.String("snapcraft-channel", "", "override snap channel for snapcraft")
	flags.String("rockcraft-channel", "", "override snap channel for rockcraft")

	flags.String("google-credential-file", "", "override path to google credentials file")

	// Additional package specification
	flags.StringSlice(
		"extra-snaps",
		[]string{},
		"comma-separated list of extra snaps to install. E.g. 'astral-uv/latest/edge,jhack'",
	)

	flags.StringSlice(
		"extra-debs",
		[]string{},
		"comma-separated list of extra debs to install. E.g. 'make,python3-tox'",
	)

	flags.Bool("dry-run", false, "show what would be done without making changes")

	return cmd
}
