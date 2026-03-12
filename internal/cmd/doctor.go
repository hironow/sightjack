package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
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
			jsonOut, _ := cmd.Flags().GetBool("json")

			logger := loggerFrom(cmd)
			results := session.RunDoctor(cmd.Context(), resolved, baseDir, logger)

			if jsonOut {
				return printDoctorJSON(cmd.OutOrStdout(), results)
			}
			return printDoctorText(cmd.ErrOrStderr(), results)
		},
	}

	cmd.Flags().BoolP("json", "j", false, "output as JSON")

	return cmd
}

type doctorJSONCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

func printDoctorJSON(w io.Writer, results []domain.DoctorCheck) error {
	checks := make([]doctorJSONCheck, len(results))
	hasFail := false
	for i, r := range results {
		checks[i] = doctorJSONCheck{
			Name:    r.Name,
			Status:  r.Status.StatusLabel(),
			Message: r.Message,
			Hint:    r.Hint,
		}
		if r.Status == domain.CheckFail {
			hasFail = true
		}
	}
	data, err := json.MarshalIndent(struct {
		Checks []doctorJSONCheck `json:"checks"`
	}{Checks: checks}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal doctor checks: %w", err)
	}
	fmt.Fprintln(w, string(data))
	if hasFail {
		return fmt.Errorf("some checks failed")
	}
	return nil
}

func printDoctorText(w io.Writer, results []domain.DoctorCheck) error {
	fmt.Fprintln(w, "sightjack doctor — environment health check")
	fmt.Fprintln(w)

	var fails, skips, warns int
	for _, r := range results {
		fmt.Fprintf(w, "  [%-4s] %-16s %s\n", r.Status.StatusLabel(), r.Name, r.Message)
		if r.Hint != "" {
			fmt.Fprintf(w, "         %-16s hint: %s\n", "", r.Hint)
		}
		switch r.Status {
		case domain.CheckFail:
			fails++
		case domain.CheckSkip:
			skips++
		case domain.CheckWarn:
			warns++
		}
	}

	fmt.Fprintln(w)
	if fails == 0 && skips == 0 && warns == 0 {
		fmt.Fprintln(w, "All checks passed.")
		return nil
	}
	var parts []string
	if fails > 0 {
		parts = append(parts, fmt.Sprintf("%d check(s) failed", fails))
	}
	if warns > 0 {
		parts = append(parts, fmt.Sprintf("%d warning(s)", warns))
	}
	if skips > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skips))
	}
	fmt.Fprintln(w, strings.Join(parts, ", ")+".")
	if fails > 0 {
		return fmt.Errorf("%d check(s) failed", fails)
	}
	return nil
}
