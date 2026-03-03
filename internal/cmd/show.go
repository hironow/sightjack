package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
	"github.com/hironow/sightjack/internal/usecase"
)

func newShowCmd() *cobra.Command {
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
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			w := cmd.OutOrStdout()
			if stdinIsPipe() {
				return runShowFromStdin(w)
			}
			return usecase.ShowFromState(w, baseDir, logger)
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
