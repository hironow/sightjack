package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/session"
)

// newStatusCmd creates the status subcommand that displays operational status.
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [path]",
		Short: "Show sightjack operational status",
		Long: `Display operational status including scan history, wave statistics,
success rate, and pending d-mail counts.

Output goes to stderr (human-readable) by default.
Use -o json for machine-readable JSON output to stdout.`,
		Example: `  # Show status for current directory
  sightjack status

  # Show status for a specific project
  sightjack status /path/to/project

  # JSON output for scripting
  sightjack status -o json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}

			report := session.Status(baseDir)

			outputFmt, _ := cmd.Flags().GetString("output")
			if outputFmt == "json" {
				data, jsonErr := json.Marshal(report)
				if jsonErr != nil {
					return fmt.Errorf("marshal status: %w", jsonErr)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			// Text output to stderr (human-readable metadata)
			fmt.Fprint(cmd.ErrOrStderr(), report.FormatText())
			return nil
		},
	}
}
