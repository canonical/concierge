package cmd

import (
	"github.com/spf13/cobra"
)

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "concierge",
		Version: appVersion(),
		Short:   "A utility for configuring dev/test machines for charm development.",
		Long: `concierge is an opinionated utility for provisioning charm development and testing machines.
	
Its role is to ensure that a given machine has the relevant "craft" tools and providers installed,
then bootstrap a Juju controller onto each of the providers. Additionally, it can install selected
tools from the [snap store](https://snapcraft.io) or the Ubuntu archive.
	`,
		SilenceErrors: true,
		SilenceUsage:  true,
		PreRun: func(cmd *cobra.Command, args []string) {
			parseLoggingFlags(cmd.Flags())
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	flags := cmd.PersistentFlags()
	flags.BoolP("verbose", "v", false, "enable verbose logging")
	flags.Bool("trace", false, "enable trace logging")

	cmd.AddCommand(restoreCmd())
	cmd.AddCommand(prepareCmd())
	cmd.AddCommand(statusCmd())

	return cmd
}
