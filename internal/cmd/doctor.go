package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/session"
)

func newDoctorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor [path]",
		Short: "Check environment and tool availability",
		Long: `Check environment health and tool availability.

Verifies that the sightjack config is valid, required tools
(claude, git) are installed, and the Linear MCP connection
is working. Each check reports one of four statuses:
OK (passed), FAIL (exit 1), SKIP (dependency missing),
WARN (advisory, exit 0).

The context-budget check estimates token consumption per category
(tools, skills, plugins, mcp, hooks) and marks the heaviest.
When the threshold (20,000 tokens) is exceeded, a category-specific
hint recommends adjusting .claude/settings.json.`,
		Example: `  # Run environment check
  sightjack doctor

  # Check a specific project directory
  sightjack doctor /path/to/project

  # JSON output for scripting
  sightjack doctor -o json

  # Auto-fix repairable issues
  sightjack doctor --repair`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := resolveTargetDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			resolved := resolveConfigPath(cmd, baseDir)
			jsonOut := mustBool(cmd, "json")

			logger := loggerFrom(cmd)
			repair := mustBool(cmd, "repair")
			linearFlag := mustBool(cmd, "linear")
			mode := domain.NewTrackingMode(linearFlag)
			results := session.RunDoctor(cmd.Context(), resolved, baseDir, logger, repair, mode)

			if jsonOut {
				return printDoctorJSON(cmd.OutOrStdout(), results)
			}
			pl := platform.NewLogger(cmd.ErrOrStderr(), false)
			return printDoctorText(cmd.ErrOrStderr(), pl, results)
		},
	}

	cmd.Flags().BoolP("json", "j", false, "output as JSON")
	cmd.Flags().Bool("repair", false, "Auto-fix repairable issues")

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
		return &domain.SilentError{Err: fmt.Errorf("some checks failed")}
	}
	return nil
}

func printDoctorText(w io.Writer, logger *platform.Logger, results []domain.DoctorCheck) error {
	fmt.Fprintln(w, "sightjack doctor — environment health check")
	fmt.Fprintln(w)

	var fails, skips, warns int
	for _, r := range results {
		label := logger.Colorize(fmt.Sprintf("%-4s", r.Status.StatusLabel()), platform.StatusColor(r.Status))
		fmt.Fprintf(w, "  [%s] %-16s %s\n", label, r.Name, r.Message)
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
		return &domain.SilentError{Err: fmt.Errorf("%d check(s) failed", fails)}
	}
	return nil
}
