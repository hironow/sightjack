package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version, commit, and build information",
		Long: `Print version, commit hash, build date, and Go version.

By default outputs a human-readable single line. Use --json
for structured output suitable for scripts and CI.`,
		Example: `  # Print version info
  sightjack version

  # JSON output for scripts
  sightjack version --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			info := map[string]string{
				"version": Version,
				"commit":  Commit,
				"date":    Date,
				"go":      runtime.Version(),
			}

			if jsonOutput {
				data, err := json.MarshalIndent(info, "", "  ")
				if err != nil {
					return fmt.Errorf("JSON marshal failed: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "sightjack v%s (commit: %s, date: %s, go: %s)\n",
				Version, Commit, Date, runtime.Version())
			return nil
		},
	}

	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output version info as JSON")

	return cmd
}
