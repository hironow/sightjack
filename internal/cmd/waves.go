package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	sightjack "github.com/hironow/sightjack"
)

func newWavesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "waves [path]",
		Short: "Generate waves from stdin ScanResult JSON",
		Long: `Generate wave plans from a ScanResult JSON on stdin.

Reads a ScanResult (from 'scan --json') and produces a WavePlan
containing prioritized waves for each cluster. Output is JSON suitable
for piping into 'select' or 'show'.`,
		Example: `  # Full pipe workflow
  sightjack scan --json | sightjack waves | sightjack show

  # Generate waves and save to file
  sightjack scan --json | sightjack waves > plan.json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			cfg, err := loadConfig(cmd, baseDir)
			if err != nil {
				return err
			}
			data, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("failed to read stdin: %w", err)
			}
			if len(data) == 0 {
				return fmt.Errorf("no input on stdin. Pipe scan result: sightjack scan --json | sightjack waves")
			}

			var scanResult sightjack.ScanResult
			if err := json.Unmarshal(data, &scanResult); err != nil {
				return fmt.Errorf("invalid ScanResult JSON: %w", err)
			}

			sessionID := fmt.Sprintf("waves-%d-%d", time.Now().UnixMilli(), os.Getpid())
			scanDir := sightjack.ScanDir(baseDir, sessionID)
			if err := os.MkdirAll(scanDir, 0755); err != nil {
				return fmt.Errorf("failed to create scan dir: %w", err)
			}

			logger := loggerFrom(cmd)

			waves, err := sightjack.RunWaveGenerate(cmd.Context(), cfg, scanDir, scanResult.Clusters, dryRun, logger)
			if err != nil {
				return fmt.Errorf("wave generation failed: %w", err)
			}

			if dryRun {
				logger.OK("Dry-run complete. Check %s for generated prompts.", scanDir)
				return nil
			}

			plan := sightjack.WavePlan{
				Waves:      waves,
				ScanResult: &scanResult,
			}
			out, jsonErr := json.MarshalIndent(plan, "", "  ")
			if jsonErr != nil {
				return fmt.Errorf("JSON marshal failed: %w", jsonErr)
			}
			// Cache result for pipe replay: cat .siren/.run/<id>/waves_result.json | sightjack select
			os.WriteFile(filepath.Join(scanDir, "waves_result.json"), out, 0644)
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
}
