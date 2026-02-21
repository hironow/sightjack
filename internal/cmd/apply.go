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

func newApplyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "apply [path]",
		Short: "Apply a wave to Linear from stdin Wave JSON",
		Long: `Apply a wave to Linear issues from stdin Wave JSON.

Reads a Wave JSON (from 'select') and executes the wave plan against
the Linear API. Outputs an ApplyResult JSON with updated completeness,
suitable for piping into 'nextgen' for follow-up wave generation.`,
		Example: `  # Apply a selected wave and generate follow-ups
  sightjack select | sightjack apply | sightjack nextgen

  # Apply with dry-run
  sightjack select | sightjack apply --dry-run`,
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
				return fmt.Errorf("no input on stdin. Pipe a wave: sightjack select | sightjack apply")
			}

			// Read Wave + optional remaining_waves context from select output.
			type applyInput struct {
				sightjack.Wave
				RemainingWaves []sightjack.Wave `json:"remaining_waves,omitempty"`
			}
			var input applyInput
			if err := json.Unmarshal(data, &input); err != nil {
				return fmt.Errorf("invalid Wave JSON: %w", err)
			}
			wave := input.Wave

			sessionID := fmt.Sprintf("apply-%d-%d", time.Now().UnixMilli(), os.Getpid())
			scanDir := sightjack.ScanDir(baseDir, sessionID)
			if err := os.MkdirAll(scanDir, 0755); err != nil {
				return fmt.Errorf("failed to create scan dir: %w", err)
			}

			strictness := string(sightjack.ResolveStrictness(cfg.Strictness, []string{wave.ClusterName}))

			logger := loggerFrom(cmd)

			if dryRun {
				logger.OK("Dry-run: would apply wave %s (%s)", wave.ID, wave.ClusterName)
				return nil
			}

			internal, err := sightjack.RunWaveApply(cmd.Context(), cfg, scanDir, wave, strictness, logger)
			if err != nil {
				return fmt.Errorf("apply failed: %w", err)
			}

			result := sightjack.ToApplyResult(wave, internal)
			result.RemainingWaves = input.RemainingWaves
			out, jsonErr := json.MarshalIndent(result, "", "  ")
			if jsonErr != nil {
				return fmt.Errorf("JSON marshal failed: %w", jsonErr)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
}
