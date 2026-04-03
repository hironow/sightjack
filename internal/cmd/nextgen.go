package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func newNextgenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "nextgen [path]",
		Short: "Generate follow-up waves from stdin ApplyResult",
		Long: `Generate follow-up waves from an ApplyResult on stdin.

Reads an ApplyResult JSON (from 'apply') and evaluates whether
additional waves are needed based on completeness thresholds.
If more waves are needed, calls the AI to generate them.
Outputs a WavePlan JSON suitable for piping back into 'show' or 'select'.`,
		Example: `  # Apply and generate follow-up waves
  sightjack apply | sightjack nextgen | sightjack show

  # Full cycle: select → apply → nextgen → select
  sightjack select | sightjack apply | sightjack nextgen | sightjack select`,
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
				return fmt.Errorf("no input on stdin. Pipe apply result: sightjack apply | sightjack nextgen")
			}

			var applyResult domain.ApplyResult
			if err := json.Unmarshal(data, &applyResult); err != nil {
				return fmt.Errorf("invalid ApplyResult JSON: %w", err)
			}

			logger := loggerFrom(cmd)
			w := cmd.OutOrStdout()

			sessionID := fmt.Sprintf("nextgen-%d-%d", time.Now().UnixMilli(), os.Getpid())
			scanDir := domain.ScanDir(baseDir, sessionID)
			if err := os.MkdirAll(scanDir, 0755); err != nil {
				return fmt.Errorf("failed to create scan dir: %w", err)
			}

			// cacheAndPrint marshals a WavePlan, caches it for pipe replay, and prints to stdout.
			cacheAndPrint := func(plan domain.WavePlan) error {
				out, jsonErr := json.MarshalIndent(plan, "", "  ")
				if jsonErr != nil {
					return fmt.Errorf("JSON marshal failed: %w", jsonErr)
				}
				if err := os.WriteFile(filepath.Join(scanDir, "nextgen_result.json"), out, 0644); err != nil {
					logger.Warn("Failed to cache nextgen result: %v", err)
				}
				fmt.Fprintln(w, string(out))
				return nil
			}

			// If completeness target reached, output empty plan.
			if applyResult.NewCompleteness >= 0.95 {
				logger.OK("Completeness %.0f%% — no follow-up waves needed.", applyResult.NewCompleteness*100)
				return cacheAndPrint(domain.WavePlan{Waves: []domain.Wave{}})
			}

			// Resolve wave and cluster context — prefer embedded CompletedWave (pipe),
			// fall back to event replay (interactive session).
			var completedWave domain.Wave
			var cluster domain.ClusterScanResult
			var allWaves []domain.Wave

			if applyResult.CompletedWave != nil {
				completedWave = *applyResult.CompletedWave
				if completedWave.ClusterContext != nil {
					cluster = *completedWave.ClusterContext
				} else {
					cluster = domain.ClusterScanResult{Name: completedWave.ClusterName}
				}
				cluster.Completeness = applyResult.NewCompleteness
				allWaves = append([]domain.Wave{completedWave}, applyResult.RemainingWaves...)
			} else {
				state, _, stateErr := session.LoadLatestState(cmd.Context(), baseDir)
				if stateErr != nil {
					return fmt.Errorf("cannot resolve wave context: no CompletedWave in ApplyResult and no event data.\nUse pipe workflow (apply | nextgen) or run 'sightjack scan' first")
				}

				allWaves = domain.RestoreWaves(state.Waves)

				var candidates []domain.Wave
				for _, w := range allWaves {
					if w.ID == applyResult.WaveID {
						candidates = append(candidates, w)
					}
				}
				if len(candidates) == 0 {
					return fmt.Errorf("could not find wave %q in state", applyResult.WaveID)
				}
				if len(candidates) > 1 {
					return fmt.Errorf("ambiguous wave ID %q matches %d clusters. Use pipe workflow (apply | nextgen) for unambiguous resolution", applyResult.WaveID, len(candidates))
				}
				completedWave = candidates[0]

				found := false
				for _, cs := range state.Clusters {
					if cs.Name == completedWave.ClusterName {
						cluster = domain.ClusterScanResult{
							Name:         cs.Name,
							Completeness: cs.Completeness,
							IssueCount:   cs.IssueCount,
						}
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("could not find cluster %q for wave %q in state", completedWave.ClusterName, applyResult.WaveID)
				}
			}

			if !domain.NeedsMoreWaves(cluster, allWaves) {
				logger.OK("No more waves needed for %s.", cluster.Name)
				return cacheAndPrint(domain.WavePlan{Waves: []domain.Wave{}})
			}

			adrDir := session.ADRDir(baseDir)
			existingADRs, _ := session.ReadExistingADRs(adrDir)
			completedWaves := domain.CompletedWavesForCluster(allWaves, cluster.Name)
			strictness := string(domain.ResolveStrictness(cfg.Strictness, cfg.Computed.EstimatedStrictness, []string{cluster.Name}))

			if dryRun {
				if err := session.GenerateNextWavesDryRun(cfg, scanDir, completedWave, cluster, completedWaves, existingADRs, nil, strictness, nil, nil, logger); err != nil {
					return fmt.Errorf("dry-run failed: %w", err)
				}
				logger.OK("Dry-run complete. Check %s for generated prompt.", scanDir)
				return nil
			}

			runner := session.NewTrackedRunner(cfg, baseDir, logger)
			newWaves, err := session.GenerateNextWaves(cmd.Context(), cfg, scanDir, completedWave, cluster, completedWaves, existingADRs, nil, strictness, nil, nil, runner, logger)
			if err != nil {
				return fmt.Errorf("nextgen failed: %w", err)
			}

			return cacheAndPrint(domain.WavePlan{Waves: newWaves})
		},
	}
}
