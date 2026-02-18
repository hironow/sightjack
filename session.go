package sightjack

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CalcNewlyUnlocked computes how many waves were newly unlocked after completing a wave.
// oldAvailable is the available count before the wave was completed (includes the completing wave).
// newAvailable is the available count after completion and unlock evaluation.
func CalcNewlyUnlocked(oldAvailable, newAvailable int) int {
	// oldAvailable includes the wave being completed, so subtract 1 to get
	// the baseline of waves that were already available before this action.
	newCount := newAvailable - (oldAvailable - 1)
	if newCount < 0 {
		return 0
	}
	return newCount
}

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
		// Also generate scribe ADR prompt for dry-run
		if cfg.Scribe.Enabled {
			sampleArchitectResp := &ArchitectResponse{
				Analysis:  "Sample architect analysis for dry-run",
				Reasoning: "Sample reasoning",
			}
			if err := RunScribeADRDryRun(cfg, scanDir, sampleWave, sampleArchitectResp, ADRDir(baseDir)); err != nil {
				return fmt.Errorf("scribe dry-run: %w", err)
			}
		}
		// Also generate nextgen prompt for dry-run
		sampleCompletedWaves := []Wave{sampleWave}
		sampleCluster := ClusterScanResult{Name: "sample", Completeness: 0.5, Issues: sampleClusters[0].Issues}
		if err := GenerateNextWavesDryRun(cfg, scanDir, sampleWave, sampleCluster, sampleCompletedWaves, nil, nil); err != nil {
			return fmt.Errorf("nextgen dry-run: %w", err)
		}
		LogOK("Dry-run complete. Check .siren/scans/ for generated prompts.")
		return nil
	}

	// Capture scan timestamp once so it stays stable across wave completions
	scanTime := time.Now()

	// Cache ScanResult for resume
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	if err := WriteScanResult(scanResultPath, scanResult); err != nil {
		LogWarn("Failed to cache scan result: %v", err)
	}

	// --- Pass 3: Wave Generate ---
	waves, err := RunWaveGenerate(ctx, cfg, scanDir, scanResult.Clusters, false)
	if err != nil {
		return fmt.Errorf("wave generate: %w", err)
	}

	LogOK("%d clusters, %d waves generated", len(scanResult.Clusters), len(waves))

	completed := BuildCompletedWaveMap(waves)
	scanner := bufio.NewScanner(input)
	adrDir := ADRDir(baseDir)
	adrCount := CountADRFiles(adrDir)

	return runInteractiveLoop(ctx, cfg, baseDir, sessionID, scanDir, scanResultPath,
		scanResult, waves, completed, adrCount, scanner, adrDir, nil, scanTime)
}

// runInteractiveLoop runs the wave selection/approval/apply loop shared by
// RunSession, RunResumeSession, and RunRescanSession.
// resumedAt controls the Navigator "Session: resumed" banner (nil hides it).
// scanTimestamp is persisted in state as LastScanned and stays stable across saves.
func runInteractiveLoop(ctx context.Context, cfg *Config, baseDir, sessionID, scanDir, scanResultPath string,
	scanResult *ScanResult, waves []Wave, completed map[string]bool, adrCount int,
	scanner *bufio.Scanner, adrDir string, resumedAt *time.Time, scanTimestamp time.Time) error {

	// --- Interactive Loop ---
	shibitoShown := false
	// sessionRejected tracks user-rejected actions per wave (keyed by WaveKey).
	// Scoped per-wave intentionally: rejected actions are only fed back to the
	// nextgen call triggered by that specific wave's completion, not accumulated
	// across the entire cluster.
	sessionRejected := make(map[string][]WaveAction)
	for {
		waves = EvaluateUnlocks(waves, completed)
		available := AvailableWaves(waves, completed)
		if len(available) == 0 {
			LogOK("All waves completed or no available waves.")
			break
		}

		// Display Link Navigator
		nav := RenderMatrixNavigator(scanResult, cfg.Linear.Project, waves, adrCount, resumedAt, string(cfg.Strictness.Default), len(scanResult.ShibitoWarnings))
		fmt.Println()
		fmt.Print(nav)

		// Display shibito warnings once (static data, does not change during session)
		if !shibitoShown {
			DisplayShibitoWarnings(os.Stdout, scanResult.ShibitoWarnings)
			shibitoShown = true
		}

		// Prompt wave selection
		selected, err := PromptWaveSelection(ctx, os.Stdout, scanner, available)
		if err == ErrQuit {
			LogInfo("Session paused. State saved.")
			break
		}
		if err == ErrGoBack {
			completedList := CompletedWaves(waves)
			if len(completedList) == 0 {
				LogInfo("No completed waves to revisit.")
				continue
			}
			revisit, backErr := PromptCompletedWaveSelection(ctx, os.Stdout, scanner, completedList)
			if backErr == ErrQuit {
				LogInfo("Session paused. State saved.")
				break
			}
			if backErr != nil {
				continue
			}
			DisplayCompletedWaveActions(os.Stdout, revisit)
			continue
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
				delete(sessionRejected, WaveKey(selected))
				applyWave = true
			case ApprovalReject:
				delete(sessionRejected, WaveKey(selected))
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
					selected = ApplyModifiedWave(selected, *result.ModifiedWave, completed)
					PropagateWaveUpdate(waves, selected)
					// Trigger Scribe to generate ADR for the modification
					// (runs even for locked waves — the decision itself is worth recording)
					if cfg.Scribe.Enabled {
						scribeResp, scribeErr := RunScribeADR(ctx, cfg, scanDir, selected, result, adrDir)
						if scribeErr != nil {
							LogWarn("Scribe failed (non-fatal): %v", scribeErr)
						} else {
							DisplayScribeResponse(os.Stdout, scribeResp)
							DisplayADRConflicts(os.Stdout, scribeResp.Conflicts)
							adrCount++
						}
					}
					if selected.Status == "locked" {
						LogWarn("Architect added unmet prerequisites — wave is now locked.")
						break
					}
				}
				continue // back to approval prompt with (possibly modified) wave
			case ApprovalSelective:
				approved, rejected, selErr := PromptSelectiveApproval(ctx, os.Stdout, scanner, selected)
				if selErr == ErrQuit {
					break
				}
				if selErr != nil {
					LogWarn("Selective approval error: %v", selErr)
					continue
				}
				if len(approved) == 0 {
					LogInfo("No actions selected. Wave skipped.")
					break
				}
				selected.Actions = approved
				// Recompute delta proportionally when actions were rejected
				totalActions := len(approved) + len(rejected)
				if totalActions > 0 && len(rejected) > 0 {
					fraction := float64(len(approved)) / float64(totalActions)
					selected.Delta.After = selected.Delta.Before + (selected.Delta.After-selected.Delta.Before)*fraction
				}
				PropagateWaveUpdate(waves, selected)
				sessionRejected[WaveKey(selected)] = rejected
				applyWave = true
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

		// Count new waves unlocked by this completion
		oldAvailable := len(AvailableWaves(waves, completed))

		if !IsWaveApplyComplete(applyResult) {
			LogWarn("Wave %s partially failed (%d errors). Not marking as completed.", WaveKey(selected), len(applyResult.Errors))
			DisplayRippleEffects(os.Stdout, applyResult.Ripples)
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
				adjustedAfter := PartialApplyDelta(applyResult, selected.Delta)
				scanResult.Clusters[i].Completeness = adjustedAfter
				// Note: per-issue completeness is NOT updated here because
				// action types vary (add_dod vs add_dependency) and we lack
				// accurate per-issue deltas. The nextgen prompt already
				// receives CompletedWaves JSON listing all applied actions,
				// which is sufficient for the LLM to avoid re-proposals.
				break
			}
		}
		scanResult.CalculateCompleteness()

		// Display rich completion summary with grouped ripple effects
		waves = EvaluateUnlocks(waves, completed)
		newAvailable := len(AvailableWaves(waves, completed))
		newCount := CalcNewlyUnlocked(oldAvailable, newAvailable)
		DisplayWaveCompletion(os.Stdout, selected, applyResult.Ripples, scanResult.Completeness, newCount)

		// --- Post-completion: Generate next waves ---
		var clusterForNextgen ClusterScanResult
		for _, c := range scanResult.Clusters {
			if c.Name == selected.ClusterName {
				clusterForNextgen = c
				break
			}
		}
		if clusterForNextgen.Name == "" {
			LogWarn("Cluster %q not found in scan results; skipping nextgen", selected.ClusterName)
		} else {
			completedWavesForCluster := completedWavesInCluster(waves, selected.ClusterName)
			existingADRs, adrErr := ReadExistingADRs(adrDir)
			if adrErr != nil {
				LogWarn("Failed to read ADRs for nextgen (non-fatal): %v", adrErr)
			}
			rejectedForWave := sessionRejected[WaveKey(selected)]
			newWaves, nextgenErr := GenerateNextWaves(ctx, cfg, scanDir, selected, clusterForNextgen, completedWavesForCluster, existingADRs, rejectedForWave)
			if nextgenErr != nil {
				LogWarn("Nextgen failed (non-fatal): %v", nextgenErr)
			} else if len(newWaves) > 0 {
				waves = append(waves, newWaves...)
				waves = EvaluateUnlocks(waves, completed)
			}
		}

		// Save state after each wave completion (crash resilience)
		if err := WriteScanResult(scanResultPath, scanResult); err != nil {
			LogWarn("Failed to update cached scan result: %v", err)
		}
		midState := BuildSessionState(cfg, sessionID, scanResult, waves, adrCount, &scanTimestamp)
		midState.ScanResultPath = scanResultPath
		if err := WriteState(baseDir, midState); err != nil {
			LogWarn("Failed to save mid-session state: %v", err)
		}
	}

	// Final consistency check
	if CheckCompletenessConsistency(scanResult.Completeness, scanResult.Clusters) {
		LogWarn("Completeness mismatch detected. Recalculating...")
		scanResult.CalculateCompleteness()
	}

	// Save state + updated scan result
	if err := WriteScanResult(scanResultPath, scanResult); err != nil {
		LogWarn("Failed to update cached scan result: %v", err)
	}
	state := BuildSessionState(cfg, sessionID, scanResult, waves, adrCount, &scanTimestamp)
	state.ScanResultPath = scanResultPath
	if err := WriteState(baseDir, state); err != nil {
		LogWarn("Failed to save state: %v", err)
	} else {
		LogOK("State saved to %s", StatePath(baseDir))
	}

	return nil
}

// CanResume checks whether a saved session state supports resumption.
// It returns false when the cached ScanResult path is empty (e.g. v0.4
// state files) or the file no longer exists on disk.
func CanResume(state *SessionState) bool {
	if state.ScanResultPath == "" {
		return false
	}
	_, err := os.Stat(state.ScanResultPath)
	return err == nil
}

// ResumeSession loads a previous session's state and cached scan result,
// restoring waves and completed map for the interactive loop.
func ResumeSession(baseDir string, state *SessionState) (*ScanResult, []Wave, map[string]bool, int, error) {
	if state.ScanResultPath == "" {
		return nil, nil, nil, 0, fmt.Errorf("no cached scan result path in state")
	}
	scanResult, err := LoadScanResult(state.ScanResultPath)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("load cached scan result: %w", err)
	}
	waves := RestoreWaves(state.Waves)
	completed := BuildCompletedWaveMap(waves)
	adrCount := CountADRFiles(ADRDir(baseDir))
	return scanResult, waves, completed, adrCount, nil
}

// RunResumeSession resumes an existing session from saved state.
func RunResumeSession(ctx context.Context, cfg *Config, baseDir string, state *SessionState, input io.Reader) error {
	if input == nil {
		return fmt.Errorf("input reader is required for interactive session")
	}
	scanResult, waves, completed, adrCount, err := ResumeSession(baseDir, state)
	if err != nil {
		return fmt.Errorf("resume: %w", err)
	}
	scanDir := ScanDir(baseDir, state.SessionID)
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	scanner := bufio.NewScanner(input)
	adrDir := ADRDir(baseDir)
	lastScanned := state.LastScanned

	LogOK("Resumed session: %d waves, %d completed", len(waves), len(completed))

	return runInteractiveLoop(ctx, cfg, baseDir, state.SessionID, scanDir, scanResultPath,
		scanResult, waves, completed, adrCount, scanner, adrDir, &lastScanned, lastScanned)
}

// RunRescanSession performs a fresh scan then merges completed status from old state.
func RunRescanSession(ctx context.Context, cfg *Config, baseDir string, oldState *SessionState, input io.Reader) error {
	if input == nil {
		return fmt.Errorf("input reader is required for interactive session")
	}
	sessionID := fmt.Sprintf("session-%d-%d", time.Now().UnixMilli(), os.Getpid())
	scanDir, err := EnsureScanDir(baseDir, sessionID)
	if err != nil {
		return err
	}
	scanResult, err := RunScan(ctx, cfg, baseDir, sessionID, false)
	if err != nil {
		return fmt.Errorf("re-scan: %w", err)
	}
	scanTime := time.Now()
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	if err := WriteScanResult(scanResultPath, scanResult); err != nil {
		LogWarn("Failed to cache scan result: %v", err)
	}
	waves, err := RunWaveGenerate(ctx, cfg, scanDir, scanResult.Clusters, false)
	if err != nil {
		return fmt.Errorf("wave generate: %w", err)
	}
	oldCompleted := BuildCompletedWaveMap(RestoreWaves(oldState.Waves))
	waves = MergeCompletedStatus(oldCompleted, waves)
	waves = EvaluateUnlocks(waves, BuildCompletedWaveMap(waves))
	completed := BuildCompletedWaveMap(waves)
	adrDir := ADRDir(baseDir)
	adrCount := CountADRFiles(adrDir)
	scanner := bufio.NewScanner(input)

	LogOK("Re-scanned: %d clusters, %d waves (%d previously completed)",
		len(scanResult.Clusters), len(waves), len(completed))

	return runInteractiveLoop(ctx, cfg, baseDir, sessionID, scanDir, scanResultPath,
		scanResult, waves, completed, adrCount, scanner, adrDir, nil, scanTime)
}

// BuildSessionState creates a SessionState from current session data.
// When lastScanned is non-nil the stored timestamp is preserved (resume);
// otherwise it defaults to time.Now() (fresh scan).
func BuildSessionState(cfg *Config, sessionID string, scanResult *ScanResult, waves []Wave, adrCount int, lastScanned *time.Time) *SessionState {
	ts := time.Now()
	if lastScanned != nil {
		ts = *lastScanned
	}
	state := &SessionState{
		Version:      "0.8",
		SessionID:    sessionID,
		Project:      cfg.Linear.Project,
		LastScanned:  ts,
		Completeness: scanResult.Completeness,
		Waves:        BuildWaveStates(waves),
		ADRCount:     adrCount,
		ShibitoCount: len(scanResult.ShibitoWarnings),
	}
	for _, c := range scanResult.Clusters {
		state.Clusters = append(state.Clusters, ClusterState{
			Name:         c.Name,
			Completeness: c.Completeness,
			IssueCount:   len(c.Issues),
		})
	}
	return state
}

// PartialApplyDelta computes the adjusted delta for a partially applied wave.
// When TotalCount is 0 (legacy result), the original delta.After is returned.
func PartialApplyDelta(result *WaveApplyResult, delta WaveDelta) float64 {
	if result.TotalCount == 0 || result.Applied >= result.TotalCount {
		return delta.After
	}
	if result.Applied == 0 {
		return delta.Before
	}
	successRate := float64(result.Applied) / float64(result.TotalCount)
	return delta.Before + (delta.After-delta.Before)*successRate
}

// IsWaveApplyComplete returns true when the apply result has no errors,
// indicating all actions were successfully applied.
func IsWaveApplyComplete(result *WaveApplyResult) bool {
	return len(result.Errors) == 0
}

// ApplyModifiedWave merges a modified wave from the architect into the original,
// preserving identity fields (ID, ClusterName) so that completion bookkeeping
// remains stable. Status is recomputed from the modified prerequisites against
// the completed map to prevent applying waves with unmet dependencies.
func ApplyModifiedWave(original, modified Wave, completed map[string]bool) Wave {
	modified.ID = original.ID
	modified.ClusterName = original.ClusterName

	// Preserve original fields when architect omits them (nil/zero from JSON).
	if modified.Actions == nil {
		modified.Actions = original.Actions
	}
	if modified.Prerequisites == nil {
		modified.Prerequisites = original.Prerequisites
	}
	if modified.Delta == (WaveDelta{}) {
		modified.Delta = original.Delta
	}

	// Normalize bare prerequisite IDs to composite "ClusterName:ID" format.
	for i, p := range modified.Prerequisites {
		if !strings.Contains(p, ":") {
			modified.Prerequisites[i] = modified.ClusterName + ":" + p
		}
	}

	// Recompute status: if any prerequisite is unmet, lock the wave.
	modified.Status = "available"
	for _, prereq := range modified.Prerequisites {
		if !completed[prereq] {
			modified.Status = "locked"
			break
		}
	}
	return modified
}

// PropagateWaveUpdate writes the updated wave back into the waves slice,
// matching by WaveKey so that subsequent AvailableWaves calls see the new state.
func PropagateWaveUpdate(waves []Wave, updated Wave) {
	key := WaveKey(updated)
	for i := range waves {
		if WaveKey(waves[i]) == key {
			waves[i] = updated
			return
		}
	}
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

// MergeCompletedStatus preserves completed status from a previous session
// when waves are regenerated after a re-scan. Waves in newWaves that match
// a key in oldCompleted are marked "completed". Waves that were in the old
// session but not in newWaves are dropped (Linear removed them).
func MergeCompletedStatus(oldCompleted map[string]bool, newWaves []Wave) []Wave {
	result := make([]Wave, len(newWaves))
	copy(result, newWaves)
	for i, w := range result {
		if oldCompleted[WaveKey(w)] {
			result[i].Status = "completed"
		}
	}
	return result
}

// RestoreWaves converts persisted WaveState list back into Wave list for session resume.
func RestoreWaves(states []WaveState) []Wave {
	waves := make([]Wave, len(states))
	for i, s := range states {
		waves[i] = Wave{
			ID:            s.ID,
			ClusterName:   s.ClusterName,
			Title:         s.Title,
			Description:   s.Description,
			Actions:       s.Actions,
			Prerequisites: s.Prerequisites,
			Delta:         s.Delta,
			Status:        s.Status,
		}
	}
	return waves
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
			Actions:       w.Actions,
			Description:   w.Description,
			Delta:         w.Delta,
		}
	}
	return states
}

// CheckCompletenessConsistency verifies that the sum of cluster completeness
// values matches the overall completeness. Returns true if mismatch detected.
// Tolerance: 5 percentage points (accounts for rounding).
func CheckCompletenessConsistency(overall float64, clusters []ClusterScanResult) bool {
	if len(clusters) == 0 {
		return false
	}
	var sum float64
	for _, c := range clusters {
		sum += c.Completeness
	}
	avg := sum / float64(len(clusters))
	diff := overall - avg
	if diff < 0 {
		diff = -diff
	}
	return diff > 0.05
}

// completedWavesInCluster returns all completed waves for the given cluster.
func completedWavesInCluster(waves []Wave, clusterName string) []Wave {
	var result []Wave
	for _, w := range waves {
		if w.ClusterName == clusterName && w.Status == "completed" {
			result = append(result, w)
		}
	}
	return result
}
