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

			var applyResult sightjack.ApplyResult
			if err := json.Unmarshal(data, &applyResult); err != nil {
				return fmt.Errorf("invalid ApplyResult JSON: %w", err)
			}

			// If completeness target reached, output empty plan.
			w := cmd.OutOrStdout()
			if applyResult.NewCompleteness >= 0.95 {
				sightjack.LogOK("Completeness %.0f%% — no follow-up waves needed.", applyResult.NewCompleteness*100)
				emptyPlan, _ := json.MarshalIndent(sightjack.WavePlan{Waves: []sightjack.Wave{}}, "", "  ")
				fmt.Fprintln(w, string(emptyPlan))
				return nil
			}

			// Resolve wave and cluster context — prefer embedded CompletedWave (pipe),
			// fall back to .siren/state.json (interactive session).
			var completedWave sightjack.Wave
			var cluster sightjack.ClusterScanResult
			var allWaves []sightjack.Wave

			if applyResult.CompletedWave != nil {
				completedWave = *applyResult.CompletedWave
				if completedWave.ClusterContext != nil {
					cluster = *completedWave.ClusterContext
				} else {
					cluster = sightjack.ClusterScanResult{Name: completedWave.ClusterName}
				}
				cluster.Completeness = applyResult.NewCompleteness
				allWaves = append([]sightjack.Wave{completedWave}, applyResult.RemainingWaves...)
			} else {
				state, stateErr := sightjack.ReadState(baseDir)
				if stateErr != nil {
					return fmt.Errorf("cannot resolve wave context: no CompletedWave in ApplyResult and no state file.\nUse pipe workflow (apply | nextgen) or run 'sightjack scan' first")
				}

				allWaves = sightjack.RestoreWaves(state.Waves)

				var candidates []sightjack.Wave
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
						cluster = sightjack.ClusterScanResult{
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

			if !sightjack.NeedsMoreWaves(cluster, allWaves) {
				sightjack.LogOK("No more waves needed for %s.", cluster.Name)
				emptyPlan, _ := json.MarshalIndent(sightjack.WavePlan{Waves: []sightjack.Wave{}}, "", "  ")
				fmt.Fprintln(w, string(emptyPlan))
				return nil
			}

			sessionID := fmt.Sprintf("nextgen-%d-%d", time.Now().UnixMilli(), os.Getpid())
			scanDir := sightjack.ScanDir(baseDir, sessionID)
			if err := os.MkdirAll(scanDir, 0755); err != nil {
				return fmt.Errorf("failed to create scan dir: %w", err)
			}

			adrDir := sightjack.ADRDir(baseDir)
			existingADRs, _ := sightjack.ReadExistingADRs(adrDir)
			completedWaves := sightjack.CompletedWavesForCluster(allWaves, cluster.Name)
			strictness := string(sightjack.ResolveStrictness(cfg.Strictness, []string{cluster.Name}))

			if dryRun {
				if err := sightjack.GenerateNextWavesDryRun(cfg, scanDir, completedWave, cluster, completedWaves, existingADRs, nil, strictness, nil); err != nil {
					return fmt.Errorf("dry-run failed: %w", err)
				}
				sightjack.LogOK("Dry-run complete. Check %s for generated prompt.", scanDir)
				return nil
			}

			newWaves, err := sightjack.GenerateNextWaves(cmd.Context(), cfg, scanDir, completedWave, cluster, completedWaves, existingADRs, nil, strictness, nil)
			if err != nil {
				return fmt.Errorf("nextgen failed: %w", err)
			}

			plan := sightjack.WavePlan{Waves: newWaves}
			out, jsonErr := json.MarshalIndent(plan, "", "  ")
			if jsonErr != nil {
				return fmt.Errorf("JSON marshal failed: %w", jsonErr)
			}
			fmt.Fprintln(w, string(out))
			return nil
		},
	}
}
