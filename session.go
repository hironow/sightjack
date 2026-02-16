package sightjack

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

// RunSession runs the full session: Pass 1-3 (auto), then interactive wave loop.
func RunSession(ctx context.Context, cfg *Config, baseDir string, sessionID string, dryRun bool, input io.Reader) error {
	if !dryRun && input == nil {
		return fmt.Errorf("input reader is required for interactive session")
	}

	scanDir, err := EnsureScanDir(baseDir, sessionID)
	if err != nil {
		return err
	}

	// --- Pass 1+2: Scan (reuse v0.1 RunScan) ---
	scanResult, err := RunScan(ctx, cfg, baseDir, sessionID, dryRun)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	// In dry-run mode, RunScan writes classify prompt but returns nil ScanResult.
	// Continue to Pass 3 with sample cluster data so wave-generation prompts are also generated.
	if dryRun {
		sampleClusters := []ClusterScanResult{{
			Name:         "sample",
			Completeness: 0.5,
			Issues:       []IssueDetail{{ID: "SAMPLE-1", Identifier: "SAMPLE-1", Title: "Sample issue", Completeness: 0.5}},
			Observations: []string{"sample observation for dry-run"},
		}}
		if _, err := RunWaveGenerate(ctx, cfg, scanDir, sampleClusters, true); err != nil {
			return fmt.Errorf("wave generate dry-run: %w", err)
		}
		// Also generate architect discuss prompt for dry-run
		sampleWave := Wave{
			ID:          "sample-w1",
			ClusterName: "sample",
			Title:       "Sample Wave",
			Actions:     []WaveAction{{Type: "add_dod", IssueID: "SAMPLE-1", Description: "Sample DoD"}},
		}
		if err := RunArchitectDiscussDryRun(cfg, scanDir, sampleWave, "sample discussion topic"); err != nil {
			return fmt.Errorf("architect discuss dry-run: %w", err)
		}
		LogOK("Dry-run complete. Check .siren/scans/ for generated prompts.")
		return nil
	}

	// --- Pass 3: Wave Generate ---
	waves, err := RunWaveGenerate(ctx, cfg, scanDir, scanResult.Clusters, false)
	if err != nil {
		return fmt.Errorf("wave generate: %w", err)
	}

	LogOK("%d clusters, %d waves generated", len(scanResult.Clusters), len(waves))

	completed := BuildCompletedWaveMap(waves)
	scanner := bufio.NewScanner(input)

	// --- Interactive Loop ---
	for {
		waves = EvaluateUnlocks(waves, completed)
		available := AvailableWaves(waves, completed)
		if len(available) == 0 {
			LogOK("All waves completed or no available waves.")
			break
		}

		// Display Link Navigator
		nav := RenderNavigatorWithWaves(scanResult, cfg.Linear.Project, waves)
		fmt.Println()
		fmt.Print(nav)

		// Prompt wave selection
		selected, err := PromptWaveSelection(ctx, os.Stdout, scanner, available)
		if err == ErrQuit {
			LogInfo("Session paused. State saved.")
			break
		}
		if err != nil {
			LogWarn("Invalid selection: %v", err)
			continue
		}

		// Prompt wave approval (with discuss loop)
		var applyWave bool
		for {
			choice, err := PromptWaveApproval(ctx, os.Stdout, scanner, selected)
			if err == ErrQuit {
				break
			}
			if err != nil {
				LogWarn("Invalid input: %v", err)
				continue
			}

			switch choice {
			case ApprovalApprove:
				applyWave = true
			case ApprovalReject:
				LogInfo("Wave rejected.")
			case ApprovalDiscuss:
				topic, topicErr := PromptDiscussTopic(ctx, os.Stdout, scanner)
				if topicErr == ErrQuit {
					continue
				}
				if topicErr != nil {
					LogWarn("Invalid topic: %v", topicErr)
					continue
				}
				result, discussErr := RunArchitectDiscuss(ctx, cfg, scanDir, selected, topic)
				if discussErr != nil {
					LogError("Architect discussion failed: %v", discussErr)
					continue
				}
				DisplayArchitectResponse(os.Stdout, result)
				if result.ModifiedWave != nil {
					selected = *result.ModifiedWave
				}
				continue // back to approval prompt with (possibly modified) wave
			}
			break
		}
		if !applyWave {
			continue
		}

		// --- Pass 4: Wave Apply ---
		applyResult, err := RunWaveApply(ctx, cfg, scanDir, selected)
		if err != nil {
			LogError("Apply failed: %v", err)
			continue
		}

		// Display ripple effects
		DisplayRippleEffects(os.Stdout, applyResult.Ripples)

		if !IsWaveApplyComplete(applyResult) {
			LogWarn("Wave %s partially failed (%d errors). Not marking as completed.", WaveKey(selected), len(applyResult.Errors))
			continue
		}

		// Mark wave completed using composite key (ClusterName:ID)
		completed[WaveKey(selected)] = true
		selectedKey := WaveKey(selected)
		for i, w := range waves {
			if WaveKey(w) == selectedKey {
				waves[i].Status = "completed"
				break
			}
		}

		// Update cluster completeness from delta, then recalculate overall
		for i, c := range scanResult.Clusters {
			if c.Name == selected.ClusterName {
				scanResult.Clusters[i].Completeness = selected.Delta.After
				break
			}
		}
		scanResult.CalculateCompleteness()

		LogOK("Completeness: %.0f%%", scanResult.Completeness*100)
	}

	// Save state
	state := &SessionState{
		Version:      "0.2",
		SessionID:    sessionID,
		Project:      cfg.Linear.Project,
		LastScanned:  time.Now(),
		Completeness: scanResult.Completeness,
		Waves:        BuildWaveStates(waves),
	}
	for _, c := range scanResult.Clusters {
		state.Clusters = append(state.Clusters, ClusterState{
			Name:         c.Name,
			Completeness: c.Completeness,
			IssueCount:   len(c.Issues),
		})
	}

	if err := WriteState(baseDir, state); err != nil {
		LogWarn("Failed to save state: %v", err)
	} else {
		LogOK("State saved to %s", StatePath(baseDir))
	}

	return nil
}

// IsWaveApplyComplete returns true when the apply result has no errors,
// indicating all actions were successfully applied.
func IsWaveApplyComplete(result *WaveApplyResult) bool {
	return len(result.Errors) == 0
}

// BuildCompletedWaveMap returns a set of completed waves keyed by WaveKey (ClusterName:ID).
func BuildCompletedWaveMap(waves []Wave) map[string]bool {
	completed := make(map[string]bool)
	for _, w := range waves {
		if w.Status == "completed" {
			completed[WaveKey(w)] = true
		}
	}
	return completed
}

// BuildWaveStates converts Wave list to WaveState list for persistence.
func BuildWaveStates(waves []Wave) []WaveState {
	states := make([]WaveState, len(waves))
	for i, w := range waves {
		states[i] = WaveState{
			ID:            w.ID,
			ClusterName:   w.ClusterName,
			Title:         w.Title,
			Status:        w.Status,
			Prerequisites: w.Prerequisites,
			ActionCount:   len(w.Actions),
		}
	}
	return states
}
