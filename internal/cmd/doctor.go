package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	sightjack "github.com/hironow/sightjack"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor [path]",
		Short: "Check environment and tool availability",
		Long: `Check environment health and tool availability.

Verifies that the sightjack config is valid, required tools
(claude, git) are installed, and the Linear MCP connection
is working. Reports pass/fail/skip for each check.`,
		Example: `  # Run environment check
  sightjack doctor

  # Check a specific project directory
  sightjack doctor /path/to/project`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			resolved := resolveConfigPath(cmd, baseDir)
			w := cmd.OutOrStdout()

			fmt.Fprintln(w, "sightjack doctor — environment health check")
			fmt.Fprintln(w)

			results := sightjack.RunDoctor(cmd.Context(), resolved, baseDir)

			var fails, skips int
			for _, r := range results {
				fmt.Fprintf(w, "[%s] %s: %s\n", r.Status.StatusLabel(), r.Name, r.Message)
				switch r.Status {
				case sightjack.CheckFail:
					fails++
				case sightjack.CheckSkip:
					skips++
				}
			}

			fmt.Fprintln(w)
			if fails == 0 && skips == 0 {
				fmt.Fprintln(w, "All checks passed.")
				return nil
			}
			var parts []string
			if fails > 0 {
				parts = append(parts, fmt.Sprintf("%d check(s) failed", fails))
			}
			if skips > 0 {
				parts = append(parts, fmt.Sprintf("%d skipped", skips))
			}
			fmt.Fprintln(w, strings.Join(parts, ", ")+".")
			if fails > 0 {
				return fmt.Errorf("%d check(s) failed", fails)
			}
			return nil
		},
	}
}
