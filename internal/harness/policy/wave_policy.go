// Package policy contains pure stateless functions extracted from domain/.
// These functions operate on domain types but are not part of the core domain
// model — they implement orchestration-level logic (filtering, merging,
// selection, validation) that is called from session/ and usecase/ layers.
package policy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
)

// MaxWavesPerCluster is the cap on total waves per cluster.
// Beyond this count, nextgen is skipped to prevent infinite wave growth.
const MaxWavesPerCluster = 8

// calcComplexityScore computes a complexity score for a wave based on action
// count and prerequisite count weighting. Each action contributes 1.0 and
// each prerequisite contributes 0.5.
func calcComplexityScore(w domain.Wave) float64 {
	return float64(len(w.Actions)) + float64(len(w.Prerequisites))*0.5
}

// SortWavesByComplexity returns a new slice of waves sorted by ascending
// ComplexityScore (actions + 0.5*prereqs). The sort is stable so that
// waves with equal complexity retain their original relative order.
// ComplexityScore is populated on each returned wave.
func SortWavesByComplexity(waves []domain.Wave) []domain.Wave {
	result := make([]domain.Wave, len(waves))
	copy(result, waves)
	for i := range result {
		result[i].ComplexityScore = calcComplexityScore(result[i])
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].ComplexityScore < result[j].ComplexityScore
	})
	return result
}

// NormalizeWavePrerequisites prefixes bare prerequisite IDs with the wave's own
// cluster name so that all keys in the completed map use the composite format.
// Prerequisites that already contain ":" are left unchanged.
func NormalizeWavePrerequisites(waves []domain.Wave) []domain.Wave {
	result := make([]domain.Wave, len(waves))
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

// RemoveSelfReferences removes prerequisite entries where a wave references itself.
// Returns the cleaned wave list and the count of removed self-references.
// Must be called after NormalizeWavePrerequisites (self-references are only
// detectable once bare IDs have been expanded to composite format).
func RemoveSelfReferences(waves []domain.Wave) ([]domain.Wave, int) {
	result := make([]domain.Wave, len(waves))
	copy(result, waves)
	var removed int
	for i, w := range result {
		key := domain.WaveKey(w)
		var clean []string
		for _, p := range w.Prerequisites {
			if p == key {
				removed++
			} else {
				clean = append(clean, p)
			}
		}
		result[i].Prerequisites = clean
	}
	return result, removed
}

// ClampDelta ensures Before and After are within [0, 1] and Before <= After.
// If Before > After (regression), they are swapped.
func ClampDelta(d domain.WaveDelta) domain.WaveDelta {
	if d.Before < 0 {
		d.Before = 0
	}
	if d.Before > 1 {
		d.Before = 1
	}
	if d.After < 0 {
		d.After = 0
	}
	if d.After > 1 {
		d.After = 1
	}
	if d.Before > d.After {
		d.Before, d.After = d.After, d.Before
	}
	return d
}

// MergeWaveResults flattens multiple per-cluster wave results into a single wave list,
// normalizing prerequisite IDs to the composite "ClusterName:ID" format and removing
// self-referencing prerequisites. Results are sorted by complexity score ascending.
func MergeWaveResults(results []domain.WaveGenerateResult) []domain.Wave {
	var all []domain.Wave
	for _, r := range results {
		all = append(all, r.Waves...)
	}
	normalized := NormalizeWavePrerequisites(all)
	cleaned, _ := RemoveSelfReferences(normalized)
	for i := range cleaned {
		cleaned[i].Delta = ClampDelta(cleaned[i].Delta)
	}
	return SortWavesByComplexity(cleaned)
}

// AvailableWaves returns waves that have "available" status and are not completed,
// sorted by ascending complexity score.
// The completed map is keyed by WaveKey (ClusterName:ID).
func AvailableWaves(waves []domain.Wave, completed map[string]bool) []domain.Wave {
	var available []domain.Wave
	for _, w := range waves {
		if w.Status == "available" && !completed[domain.WaveKey(w)] {
			available = append(available, w)
		}
	}
	return SortWavesByComplexity(available)
}

// EvaluateUnlocks checks locked waves and unlocks them if all prerequisites are met.
// Prerequisites and the completed map both use the composite "ClusterName:ID" format.
func EvaluateUnlocks(waves []domain.Wave, completed map[string]bool) []domain.Wave {
	result := make([]domain.Wave, len(waves))
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
func PartialApplyDelta(result *domain.WaveApplyResult, delta domain.WaveDelta) float64 {
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
func IsWaveApplyComplete(result *domain.WaveApplyResult) bool {
	return len(result.Errors) == 0
}

// ApplyModifiedWave merges a modified wave from the architect into the original,
// preserving identity fields (ID, ClusterName) so that completion bookkeeping
// remains stable. Status is recomputed from the modified prerequisites against
// the completed map to prevent applying waves with unmet dependencies.
func ApplyModifiedWave(original, modified domain.Wave, completed map[string]bool) domain.Wave {
	modified.ID = original.ID
	modified.ClusterName = original.ClusterName

	// Preserve original fields when architect omits them (nil/zero from JSON).
	if modified.Actions == nil {
		modified.Actions = original.Actions
	}
	if modified.Prerequisites == nil {
		modified.Prerequisites = original.Prerequisites
	}
	if modified.Delta == (domain.WaveDelta{}) {
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
func PropagateWaveUpdate(waves []domain.Wave, updated domain.Wave) {
	key := domain.WaveKey(updated)
	for i := range waves {
		if domain.WaveKey(waves[i]) == key {
			waves[i] = updated
			return
		}
	}
}

// DetectWaveCycles performs DFS-based cycle detection on the wave prerequisite graph.
// Returns an error describing the cycle if one is found, nil otherwise.
func DetectWaveCycles(waves []domain.Wave) error {
	if len(waves) == 0 {
		return nil
	}
	// Build adjacency map: waveKey -> prerequisites
	adj := make(map[string][]string, len(waves))
	for _, w := range waves {
		key := domain.WaveKey(w)
		adj[key] = w.Prerequisites
	}

	const (
		white = 0 // unvisited
		gray  = 1 // in current DFS path
		black = 2 // fully processed
	)
	color := make(map[string]int, len(waves))
	parent := make(map[string]string, len(waves))

	var dfs func(node string) error
	dfs = func(node string) error {
		color[node] = gray
		for _, dep := range adj[node] {
			switch color[dep] {
			case gray:
				// Back edge: cycle found. Reconstruct path.
				var path []string
				path = append(path, dep, node)
				cur := node
				for i := 0; i < len(waves) && cur != dep; i++ {
					cur = parent[cur]
					path = append(path, cur)
				}
				return fmt.Errorf("dependency cycle detected: %s", strings.Join(path, " -> "))
			case white:
				parent[dep] = node
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}
		color[node] = black
		return nil
	}

	for _, w := range waves {
		key := domain.WaveKey(w)
		if color[key] == white {
			if err := dfs(key); err != nil {
				return err
			}
		}
	}
	return nil
}

// PruneStaleWaves removes waves whose cluster is no longer in the valid cluster set.
// Completed waves are preserved regardless. Modifies state.Waves in place.
// Returns the count of pruned waves.
func PruneStaleWaves(state *domain.SessionState, validClusters []domain.ClusterState) int {
	validNames := make(map[string]bool, len(validClusters))
	for _, c := range validClusters {
		validNames[c.Name] = true
	}
	var kept []domain.WaveState
	var removed int
	for _, w := range state.Waves {
		if w.Status == "completed" || validNames[w.ClusterName] {
			kept = append(kept, w)
		} else {
			removed++
		}
	}
	state.Waves = kept
	return removed
}

// RepairLockedWaves unlocks waves whose prerequisites are all met but status is still "locked".
// Returns the repaired wave list and the count of repaired waves.
func RepairLockedWaves(waves []domain.Wave, completed map[string]bool) ([]domain.Wave, int) {
	result := make([]domain.Wave, len(waves))
	copy(result, waves)
	var repaired int
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
			repaired++
		}
	}
	return result, repaired
}

// BuildCompletedWaveMap returns a set of completed waves keyed by WaveKey (ClusterName:ID).
func BuildCompletedWaveMap(waves []domain.Wave) map[string]bool {
	completed := make(map[string]bool)
	for _, w := range waves {
		if w.Status == "completed" {
			completed[domain.WaveKey(w)] = true
		}
	}
	return completed
}

// MergeOldWaves carries forward waves from clusters that failed wave
// generation but are still present in the current scan. Old waves whose
// cluster was removed from the scan (resolved issues, reorganized clusters)
// are dropped so stale work items do not persist.
func MergeOldWaves(oldWaves, newWaves []domain.Wave, scannedClusters, failedClusterNames map[string]bool) []domain.Wave {
	regenerated := make(map[string]bool, len(newWaves))
	newKeys := make(map[string]bool, len(newWaves))
	for _, w := range newWaves {
		regenerated[w.ClusterName] = true
		newKeys[domain.WaveKey(w)] = true
	}
	merged := make([]domain.Wave, 0, len(newWaves)+len(oldWaves))
	merged = append(merged, newWaves...)
	for _, w := range oldWaves {
		inScan := scannedClusters[w.ClusterName]
		noRegeneration := !regenerated[w.ClusterName]
		partialFailure := failedClusterNames[w.ClusterName]
		if inScan && (noRegeneration || partialFailure) && !newKeys[domain.WaveKey(w)] {
			merged = append(merged, w)
		}
	}
	return merged
}

// MergeCompletedStatus preserves completed status from a previous session
// when waves are regenerated after a re-scan. Waves in newWaves that match
// a key in oldCompleted are marked "completed". Waves that were in the old
// session but not in newWaves are dropped (Linear removed them).
func MergeCompletedStatus(oldCompleted map[string]bool, newWaves []domain.Wave) []domain.Wave {
	result := make([]domain.Wave, len(newWaves))
	copy(result, newWaves)
	for i, w := range result {
		if oldCompleted[domain.WaveKey(w)] {
			result[i].Status = "completed"
		}
	}
	return result
}

// RestoreWaves converts persisted WaveState list back into Wave list for session resume.
func RestoreWaves(states []domain.WaveState) []domain.Wave {
	waves := make([]domain.Wave, len(states))
	for i, s := range states {
		waves[i] = domain.Wave{
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
func BuildWaveStates(waves []domain.Wave) []domain.WaveState {
	states := make([]domain.WaveState, len(waves))
	for i, w := range waves {
		states[i] = domain.WaveState{
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
func CheckCompletenessConsistency(overall float64, clusters []domain.ClusterScanResult) bool {
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

// ToApplyResult converts the internal WaveApplyResult to the pipe wire format ApplyResult.
// It builds per-action results from the wave's actions and the internal result's error list.
func ToApplyResult(wave domain.Wave, internal *domain.WaveApplyResult) domain.ApplyResult {
	actions := make([]domain.ActionResult, 0, len(wave.Actions))

	// Build per-action results: first N actions succeed (N = Applied),
	// remaining get error messages from the Errors list.
	for i, a := range wave.Actions {
		ar := domain.ActionResult{
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

	if total == 0 || internal.Applied >= total {
		wave.Status = "completed"
	} else {
		wave.Status = "partial"
	}

	return domain.ApplyResult{
		WaveID:          internal.WaveID,
		AppliedActions:  actions,
		RippleEffects:   internal.Ripples,
		NewCompleteness: completeness,
		CompletedWave:   &wave,
	}
}

// FilterEmptyWaves removes waves that have zero actions (nil or empty slice).
// Returns the filtered list and the count of removed waves.
func FilterEmptyWaves(waves []domain.Wave) ([]domain.Wave, int) {
	var filtered []domain.Wave
	var removed int
	for _, w := range waves {
		if len(w.Actions) == 0 {
			removed++
		} else {
			filtered = append(filtered, w)
		}
	}
	return filtered, removed
}

// AutoSelectWave selects the first available wave for auto-approve mode.
// Returns the selected wave and true if one is available, or zero Wave and false if none.
func AutoSelectWave(available []domain.Wave) (domain.Wave, bool) {
	if len(available) > 0 {
		return available[0], true
	}
	return domain.Wave{}, false
}

// CompletedWavesForCluster returns all completed waves for the given cluster.
func CompletedWavesForCluster(waves []domain.Wave, clusterName string) []domain.Wave {
	var result []domain.Wave
	for _, w := range waves {
		if w.ClusterName == clusterName && w.Status == "completed" {
			result = append(result, w)
		}
	}
	return result
}

// NeedsMoreWaves returns true when post-completion wave generation should run
// for the given cluster. It returns false (skip nextgen) when any of:
//   - cluster completeness >= 0.95 (effectively done)
//   - available (non-completed) waves still remain for the cluster
//   - total wave count for the cluster >= MaxWavesPerCluster
func NeedsMoreWaves(cluster domain.ClusterScanResult, waves []domain.Wave) bool {
	if cluster.Completeness >= 0.95 {
		return false
	}
	var clusterTotal int
	hasAvailable := false
	for _, w := range waves {
		if w.ClusterName != cluster.Name {
			continue
		}
		clusterTotal++
		if w.Status == "available" || w.Status == "locked" || w.Status == "partial" {
			hasAvailable = true
		}
	}
	if hasAvailable {
		return false
	}
	if clusterTotal >= MaxWavesPerCluster {
		return false
	}
	return true
}

// ReadyIssueIDs returns issue IDs where ALL waves targeting them are completed.
// An issue is ready when every wave containing that issue has status "completed".
// Results are sorted for deterministic output.
func ReadyIssueIDs(waves []domain.Wave) []string {
	// Track all waves per issue
	issueWaves := make(map[string][]string) // issueID -> []waveStatus
	for _, w := range waves {
		for _, a := range w.Actions {
			issueWaves[a.IssueID] = append(issueWaves[a.IssueID], w.Status)
		}
	}

	var ready []string
	for issueID, statuses := range issueWaves {
		allCompleted := true
		for _, s := range statuses {
			if s != "completed" {
				allCompleted = false
				break
			}
		}
		if allCompleted {
			ready = append(ready, issueID)
		}
	}
	sort.Strings(ready)
	return ready
}

// ClustersForIssueIDs returns the unique clusters that contain any of the given issue IDs.
// This is used to identify which clusters are affected by a report D-Mail.
func ClustersForIssueIDs(clusters []domain.ClusterScanResult, issueIDs []string) []domain.ClusterScanResult {
	if len(issueIDs) == 0 {
		return nil
	}
	// Build reverse map: issueID -> cluster index
	issueToCluster := make(map[string]int, len(clusters)*2)
	for i, c := range clusters {
		for _, issue := range c.Issues {
			issueToCluster[issue.Identifier] = i
		}
	}
	// Collect unique clusters
	seen := make(map[int]bool)
	var result []domain.ClusterScanResult
	for _, id := range issueIDs {
		if idx, ok := issueToCluster[id]; ok && !seen[idx] {
			seen[idx] = true
			result = append(result, clusters[idx])
		}
	}
	return result
}

// LastCompletedWaveForCluster returns the last completed wave for the given cluster.
// Waves are assumed to be in insertion order, so the last match wins.
// Returns false if no completed wave exists for the cluster.
func LastCompletedWaveForCluster(waves []domain.Wave, clusterName string) (domain.Wave, bool) {
	var last domain.Wave
	found := false
	for _, w := range waves {
		if w.ClusterName == clusterName && w.Status == "completed" {
			last = w
			found = true
		}
	}
	return last, found
}

// CollectSpecSentIssueIDs returns issue IDs from completed waves' actions.
// Used as session-level race condition guard: issues that have already received
// spec D-Mails should not get new implementation waves even if paintress hasn't
// applied the pr-open label yet.
func CollectSpecSentIssueIDs(completed map[string]bool, waves []domain.Wave) map[string]bool {
	result := make(map[string]bool)
	for _, w := range waves {
		key := domain.WaveKey(w)
		if !completed[key] {
			continue
		}
		for _, a := range w.Actions {
			if a.IssueID != "" && !ValidWaveActionType(a.Type) {
				result[a.IssueID] = true
			}
		}
	}
	return result
}

// CollectPROpenIssues scans clusters for issues with the paintress:pr-open label
// and returns a set of issue IDs.
func CollectPROpenIssues(clusters []domain.ClusterScanResult) map[string]bool {
	result := make(map[string]bool)
	for _, c := range clusters {
		for _, issue := range c.Issues {
			if issue.HasPROpen() {
				result[issue.ID] = true
			}
		}
	}
	return result
}

// FilterPROpenActions removes implementation-oriented actions for issues that
// already have a PR open (paintress:pr-open label). Issue-management actions
// (add_dod, add_dependency, etc.) are preserved because sightjack handles them
// directly. Waves with no remaining actions are removed entirely.
func FilterPROpenActions(waves []domain.Wave, prOpenIssues map[string]bool) []domain.Wave {
	if len(prOpenIssues) == 0 {
		return waves
	}
	result := make([]domain.Wave, 0, len(waves))
	for _, w := range waves {
		var kept []domain.WaveAction
		for _, a := range w.Actions {
			if prOpenIssues[a.IssueID] && !ValidWaveActionType(a.Type) {
				continue // implementation action for PR-open issue -> skip
			}
			kept = append(kept, a)
		}
		if len(kept) > 0 {
			w.Actions = kept
			result = append(result, w)
		}
	}
	return result
}

// validWaveActionTypes is the set of recognized wave action types.
var validWaveActionTypes = map[string]bool{
	"add_dod":            true,
	"add_dependency":     true,
	"add_label":          true,
	"update_description": true,
	"create":             true,
	"cancel":             true,
}

// ValidWaveActionType reports whether t is a recognized wave action type.
func ValidWaveActionType(t string) bool {
	return validWaveActionTypes[t]
}
