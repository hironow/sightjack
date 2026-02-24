package sightjack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ParseWaveGenerateResult reads and parses a wave_{name}.json output file.
func ParseWaveGenerateResult(path string) (*WaveGenerateResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read wave result: %w", err)
	}
	var result WaveGenerateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse wave result: %w", err)
	}
	return &result, nil
}

// ParseWaveApplyResult reads and parses an apply_{wave_id}.json output file.
func ParseWaveApplyResult(path string) (*WaveApplyResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read apply result: %w", err)
	}
	var result WaveApplyResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse apply result: %w", err)
	}
	return &result, nil
}

// WaveKey returns a globally unique key for a wave: "ClusterName:ID".
func WaveKey(w Wave) string {
	return w.ClusterName + ":" + w.ID
}

// NormalizeWavePrerequisites prefixes bare prerequisite IDs with the wave's own
// cluster name so that all keys in the completed map use the composite format.
// Prerequisites that already contain ":" are left unchanged.
func NormalizeWavePrerequisites(waves []Wave) []Wave {
	result := make([]Wave, len(waves))
	copy(result, waves)
	for i, w := range result {
		normalized := make([]string, len(w.Prerequisites))
		for j, p := range w.Prerequisites {
			if strings.Contains(p, ":") {
				normalized[j] = p
			} else {
				normalized[j] = w.ClusterName + ":" + p
			}
		}
		result[i].Prerequisites = normalized
	}
	return result
}

// MergeWaveResults flattens multiple per-cluster wave results into a single wave list,
// normalizing prerequisite IDs to the composite "ClusterName:ID" format.
func MergeWaveResults(results []WaveGenerateResult) []Wave {
	var all []Wave
	for _, r := range results {
		all = append(all, r.Waves...)
	}
	return NormalizeWavePrerequisites(all)
}

// AvailableWaves returns waves that have "available" status and are not completed.
// The completed map is keyed by WaveKey (ClusterName:ID).
func AvailableWaves(waves []Wave, completed map[string]bool) []Wave {
	var available []Wave
	for _, w := range waves {
		if w.Status == "available" && !completed[WaveKey(w)] {
			available = append(available, w)
		}
	}
	return available
}

// ToApplyResult converts the internal WaveApplyResult to the pipe wire format ApplyResult.
// It builds per-action results from the wave's actions and the internal result's error list.
func ToApplyResult(wave Wave, internal *WaveApplyResult) ApplyResult {
	actions := make([]ActionResult, 0, len(wave.Actions))

	// Build per-action results: first N actions succeed (N = Applied),
	// remaining get error messages from the Errors list.
	for i, a := range wave.Actions {
		ar := ActionResult{
			Type:    a.Type,
			IssueID: a.IssueID,
			Success: i < internal.Applied,
		}
		if !ar.Success {
			errIdx := i - internal.Applied
			if errIdx >= 0 && errIdx < len(internal.Errors) {
				ar.Error = internal.Errors[errIdx]
			} else {
				ar.Error = "unknown error"
			}
		}
		actions = append(actions, ar)
	}

	// Interpolate completeness based on the ratio of successfully applied actions.
	// All success → Delta.After, all failure → Delta.Before, partial → linear interpolation.
	// Zero actions → Delta.Before (nothing accomplished).
	total := len(wave.Actions)
	var completeness float64
	if total == 0 {
		completeness = wave.Delta.Before
	} else if internal.Applied < total {
		ratio := float64(internal.Applied) / float64(total)
		completeness = wave.Delta.Before + (wave.Delta.After-wave.Delta.Before)*ratio
	} else {
		completeness = wave.Delta.After
	}

	// Only mark "completed" on full success. Partial failures get "partial"
	// so downstream logic (CompletedWavesForCluster, nextgen) does not treat
	// failed actions as done.
	if total == 0 || internal.Applied >= total {
		wave.Status = "completed"
	} else {
		wave.Status = "partial"
	}

	return ApplyResult{
		WaveID:          internal.WaveID,
		AppliedActions:  actions,
		RippleEffects:   internal.Ripples,
		NewCompleteness: completeness,
		CompletedWave:   &wave,
	}
}

// WaveApplyFileName returns the output filename for a wave apply result.
// Includes cluster name to avoid collisions when wave IDs are duplicated across clusters.
func WaveApplyFileName(wave Wave) string {
	return fmt.Sprintf("apply_%s_%s.json", SanitizeName(wave.ClusterName), SanitizeName(wave.ID))
}

// RunWaveApply executes Pass 4: apply a single approved wave via Claude Code.
// It writes the apply result to a JSON file and returns the parsed result.
func RunWaveApply(ctx context.Context, cfg *Config, scanDir string, wave Wave, strictness string, out io.Writer, logger *Logger) (*WaveApplyResult, error) {
	ctx, applySpan := tracer.Start(ctx, "wave.apply",
		trace.WithAttributes(
			attribute.String("wave.id", wave.ID),
			attribute.String("wave.cluster_name", wave.ClusterName),
			attribute.Int("wave.action_count", len(wave.Actions)),
		),
	)
	defer applySpan.End()

	applyFile := filepath.Join(scanDir, WaveApplyFileName(wave))

	actionsJSON, err := json.Marshal(wave.Actions)
	if err != nil {
		return nil, fmt.Errorf("marshal wave actions: %w", err)
	}

	dodSection := ResolveDoDSection(cfg.DoDTemplates, wave.ClusterName)

	prompt, err := RenderWaveApplyPrompt(cfg.Lang, WaveApplyPromptData{
		WaveID:          wave.ID,
		ClusterName:     wave.ClusterName,
		Title:           wave.Title,
		Actions:         string(actionsJSON),
		DoDSection:      dodSection,
		OutputPath:      applyFile,
		StrictnessLevel: strictness,
		LabelsEnabled:   cfg.Labels.Enabled,
		LabelPrefix:     cfg.Labels.Prefix,
	})
	if err != nil {
		return nil, fmt.Errorf("render apply prompt: %w", err)
	}

	// Save prompt + tee output for debugging.
	promptBase := strings.TrimSuffix(WaveApplyFileName(wave), ".json")
	if err := os.WriteFile(filepath.Join(scanDir, promptBase+"_prompt.md"), []byte(prompt), 0644); err != nil {
		logger.Warn("save apply prompt: %v", err)
	}
	applyLog, applyLogErr := os.Create(filepath.Join(scanDir, promptBase+"_output.log"))
	applyOut := out
	if applyLogErr == nil {
		defer applyLog.Close()
		applyOut = io.MultiWriter(out, applyLog)
	} else {
		logger.Warn("create apply log: %v", applyLogErr)
	}

	linearTools := WithAllowedTools(slices.Concat(BaseAllowedTools, GHAllowedTools, LinearMCPAllowedTools)...)
	logger.Scan("Applying wave: %s - %s", wave.ClusterName, wave.Title)
	if _, err := RunClaudeOnce(ctx, cfg, prompt, applyOut, logger, linearTools); err != nil {
		return nil, fmt.Errorf("wave apply %s: %w", wave.ID, err)
	}

	if normErr := NormalizeJSONFile(applyFile); normErr != nil {
		logger.Warn("normalize wave apply JSON: %v", normErr)
	}
	result, err := ParseWaveApplyResult(applyFile)
	if err != nil {
		return nil, fmt.Errorf("parse apply result %s: %w", wave.ID, err)
	}

	logger.OK("Wave %s applied: %d actions", wave.ID, result.Applied)
	return result, nil
}

// RunReadyLabel applies the ready label to issues whose all waves have completed.
// This must only be called after a successful wave apply.
func RunReadyLabel(ctx context.Context, cfg *Config, readyIssueIDs string, out io.Writer, logger *Logger) error {
	prompt, err := RenderReadyLabelPrompt(cfg.Lang, ReadyLabelPromptData{
		ReadyLabel:    cfg.Labels.ReadyLabel,
		ReadyIssueIDs: readyIssueIDs,
	})
	if err != nil {
		return fmt.Errorf("render ready label prompt: %w", err)
	}

	logger.Scan("Applying ready labels to: %s", readyIssueIDs)
	if _, err := RunClaudeOnce(ctx, cfg, prompt, out, logger, WithAllowedTools(LinearMCPAllowedTools...)); err != nil {
		return fmt.Errorf("ready label: %w", err)
	}
	return nil
}

// EvaluateUnlocks checks locked waves and unlocks them if all prerequisites are met.
// Prerequisites and the completed map both use the composite "ClusterName:ID" format.
func EvaluateUnlocks(waves []Wave, completed map[string]bool) []Wave {
	result := make([]Wave, len(waves))
	copy(result, waves)
	for i, w := range result {
		if w.Status != "locked" {
			continue
		}
		allMet := true
		for _, prereq := range w.Prerequisites {
			if !completed[prereq] {
				allMet = false
				break
			}
		}
		if allMet {
			result[i].Status = "available"
		}
	}
	return result
}

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

// PartialApplyDelta computes the adjusted delta for a partially applied wave.
// When TotalCount is 0, the original delta.After is returned.
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

// mergeOldWaves carries forward waves from clusters that failed wave
// generation but are still present in the current scan. Old waves whose
// cluster was removed from the scan (resolved issues, reorganized clusters)
// are dropped so stale work items do not persist.
//
// failedClusterNames is the set of cluster names where at least one instance
// failed generation (from detectFailedClusterNames). With duplicate cluster
// names, a name marked as failed causes ALL old waves with that name to be
// carried forward — safe over-inclusion to avoid progress loss. Old waves
// whose WaveKey already exists in newWaves are skipped to prevent duplicates.
func MergeOldWaves(oldWaves, newWaves []Wave, scannedClusters, failedClusterNames map[string]bool) []Wave {
	regenerated := make(map[string]bool, len(newWaves))
	newKeys := make(map[string]bool, len(newWaves))
	for _, w := range newWaves {
		regenerated[w.ClusterName] = true
		newKeys[WaveKey(w)] = true
	}
	merged := make([]Wave, 0, len(newWaves)+len(oldWaves))
	merged = append(merged, newWaves...)
	for _, w := range oldWaves {
		inScan := scannedClusters[w.ClusterName]
		noRegeneration := !regenerated[w.ClusterName]
		partialFailure := failedClusterNames[w.ClusterName]
		// Carry forward if cluster is still in scan AND either:
		// - no waves were regenerated for this name (complete failure), OR
		// - at least one instance with this name failed (handles duplicates)
		// Skip waves whose WaveKey already exists in newWaves to avoid duplicates.
		if inScan && (noRegeneration || partialFailure) && !newKeys[WaveKey(w)] {
			merged = append(merged, w)
		}
	}
	return merged
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

// CheckCompletenessConsistency verifies that the average of cluster completeness
// values matches the overall completeness within a tolerance. Returns true if a
// mismatch beyond the tolerance (5 percentage points) is detected.
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

// CompletedWavesForCluster returns all completed waves for the given cluster.
func CompletedWavesForCluster(waves []Wave, clusterName string) []Wave {
	var result []Wave
	for _, w := range waves {
		if w.ClusterName == clusterName && w.Status == "completed" {
			result = append(result, w)
		}
	}
	return result
}
