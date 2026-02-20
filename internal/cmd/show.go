package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	sightjack "github.com/hironow/sightjack"
)

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [path]",
		Short: "Display last scan results",
		Long: `Display scan results or wave plans.

When stdin is a pipe, auto-detects ScanResult or WavePlan JSON
and renders the appropriate navigator view. When run without pipe,
reads from .siren/state.json and displays the matrix navigator.`,
		Example: `  # Show from saved state
  sightjack show

  # Pipe a scan result
  sightjack scan --json | sightjack show

  # Pipe a wave plan
  sightjack scan --json | sightjack waves | sightjack show`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			if stdinIsPipe() {
				return runShowFromStdin()
			}
			return runShowFromState(baseDir)
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

func runShowFromStdin() error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}

	switch sightjack.DetectPipeType(data) {
	case sightjack.PipeTypeScanResult:
		var scanResult sightjack.ScanResult
		if err := json.Unmarshal(data, &scanResult); err != nil {
			return fmt.Errorf("parse ScanResult: %w", err)
		}
		nav := sightjack.RenderNavigator(&scanResult, "")
		fmt.Println()
		fmt.Print(nav)

	case sightjack.PipeTypeWavePlan:
		var plan sightjack.WavePlan
		if err := json.Unmarshal(data, &plan); err != nil {
			return fmt.Errorf("parse WavePlan: %w", err)
		}
		var result *sightjack.ScanResult
		if plan.ScanResult != nil {
			result = plan.ScanResult
		} else {
			result = &sightjack.ScanResult{}
		}
		nav := sightjack.RenderMatrixNavigator(result, "", plan.Waves, 0, nil, "fog", 0)
		fmt.Println()
		fmt.Print(nav)

	default:
		return fmt.Errorf("could not parse stdin: expected ScanResult (with \"clusters\" key) or WavePlan (with \"waves\" key)")
	}
	return nil
}

func runShowFromState(baseDir string) error {
	state, err := sightjack.ReadState(baseDir)
	if err != nil {
		sightjack.LogInfo("Run 'sightjack scan' first.")
		return fmt.Errorf("no previous scan found: %w", err)
	}

	result := &sightjack.ScanResult{
		Completeness: state.Completeness,
	}
	for _, c := range state.Clusters {
		result.Clusters = append(result.Clusters, sightjack.ClusterScanResult{
			Name:         c.Name,
			Completeness: c.Completeness,
			IssueCount:   c.IssueCount,
		})
		result.TotalIssues += c.IssueCount
	}

	waves := sightjack.RestoreWaves(state.Waves)
	strictness := state.StrictnessLevel
	if strictness == "" {
		strictness = "fog"
	}
	nav := sightjack.RenderMatrixNavigator(result, state.Project, waves, state.ADRCount, (*time.Time)(nil), strictness, state.ShibitoCount)
	fmt.Println()
	fmt.Print(nav)
	sightjack.LogInfo("Last scanned: %s", state.LastScanned.Format("2006-01-02 15:04:05"))
	return nil
}
