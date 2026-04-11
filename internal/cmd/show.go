package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness"
	"github.com/hironow/sightjack/internal/session"
)

func newShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show [path]",
		Short: "Display last scan results",
		Long: `Display scan results or wave plans.

When stdin is a pipe, auto-detects ScanResult or WavePlan JSON
and renders the appropriate navigator view. When run without pipe,
replays events from .siren/events/ and displays the matrix navigator.`,
		Example: `  # Show from saved state
  sightjack show

  # Pipe a scan result
  sightjack scan --json | sightjack show

  # Pipe a wave plan
  sightjack scan --json | sightjack waves | sightjack show`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := loggerFrom(cmd)
			baseDir, err := resolveTargetDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			w := cmd.OutOrStdout()
			if stdinIsPipe() {
				return runShowFromStdin(w)
			}
			return showFromState(cmd.Context(), w, baseDir, logger)
		},
	}
}

func stdinIsPipe() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice == 0
}

// showFromState loads the latest session state and renders the matrix navigator.
// This is the READ MODEL path for the show command — inlined from the deleted
// usecase.ShowFromState to avoid usecase→session dependency.
func showFromState(ctx context.Context, w io.Writer, baseDir string, logger domain.Logger) error {
	state, _, err := session.LoadLatestState(ctx, baseDir)
	if err != nil {
		logger.Info("Run 'sightjack scan' first.")
		return fmt.Errorf("no previous scan found: %w", err)
	}

	result := &domain.ScanResult{
		Completeness: state.Completeness,
	}
	for _, c := range state.Clusters {
		result.Clusters = append(result.Clusters, domain.ClusterScanResult{
			Name:         c.Name,
			Completeness: c.Completeness,
			IssueCount:   c.IssueCount,
		})
		result.TotalIssues += c.IssueCount
	}

	waves := harness.RestoreWaves(state.Waves)
	strictness := state.StrictnessLevel
	if strictness == "" {
		strictness = "fog"
	}
	adrCount := session.CountADRFiles(session.ADRDir(baseDir))
	nav := session.RenderMatrixNavigator(result, state.Project, waves, adrCount, (*time.Time)(nil), strictness, state.ShibitoCount)
	fmt.Fprintln(w)
	fmt.Fprint(w, nav)
	logger.Info("Last scanned: %s", state.LastScanned.Format("2006-01-02 15:04:05"))
	return nil
}

func runShowFromStdin(w io.Writer) error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}

	switch domain.DetectPipeType(data) {
	case domain.PipeTypeScanResult:
		var scanResult domain.ScanResult
		if err := json.Unmarshal(data, &scanResult); err != nil {
			return fmt.Errorf("parse ScanResult: %w", err)
		}
		nav := session.RenderNavigator(&scanResult, "")
		fmt.Fprintln(w)
		fmt.Fprint(w, nav)

	case domain.PipeTypeWavePlan:
		var plan domain.WavePlan
		if err := json.Unmarshal(data, &plan); err != nil {
			return fmt.Errorf("parse WavePlan: %w", err)
		}
		var result *domain.ScanResult
		if plan.ScanResult != nil {
			result = plan.ScanResult
		} else {
			result = &domain.ScanResult{}
		}
		nav := session.RenderMatrixNavigator(result, "", plan.Waves, 0, nil, "fog", 0)
		fmt.Fprintln(w)
		fmt.Fprint(w, nav)

	default:
		return fmt.Errorf("could not parse stdin: expected ScanResult (with \"clusters\" key) or WavePlan (with \"waves\" key)")
	}
	return nil
}
