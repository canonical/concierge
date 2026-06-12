package cmd

import (
	"text/template"

	"github.com/spf13/cobra"
)

// versionCmd prints the same version information as the `--version` flag,
// so that `concierge version` and `concierge --version` are equivalent.
func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "version",
		Short:         "Print version information.",
		Long:          "Print version information. Equivalent to running 'concierge --version'.",
		SilenceErrors: true,
		SilenceUsage:  true,
		Run: func(cmd *cobra.Command, args []string) {
			root := cmd.Root()
			// Render the same template Cobra uses for the --version flag,
			// against the root command, so both paths produce identical output.
			tmpl := template.Must(template.New("version").Parse(root.VersionTemplate()))
			_ = tmpl.Execute(cmd.OutOrStdout(), root)
		},
	}
}
