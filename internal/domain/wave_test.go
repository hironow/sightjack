package domain_test

import (
	"testing"

	sightjack "github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/domain"
)

func TestWaveKey(t *testing.T) {
	t.Parallel()
	// given
	w := sightjack.Wave{ClusterName: "auth", ID: "w1"}

	// when
	got := domain.WaveKey(w)

	// then
	if got != "auth:w1" {
		t.Errorf("WaveKey = %q, want %q", got, "auth:w1")
	}
}

func TestNormalizeWavePrerequisites(t *testing.T) {
	t.Parallel()

	t.Run("bare_id_gets_cluster_prefix", func(t *testing.T) {
		t.Parallel()
		// given
		waves := []sightjack.Wave{
			{ClusterName: "auth", ID: "w2", Prerequisites: []string{"w1"}},
		}

		// when
		result := domain.NormalizeWavePrerequisites(waves)

		// then
		if result[0].Prerequisites[0] != "auth:w1" {
			t.Errorf("prerequisite = %q, want %q", result[0].Prerequisites[0], "auth:w1")
		}
	})

	t.Run("composite_id_unchanged", func(t *testing.T) {
		t.Parallel()
		// given
		waves := []sightjack.Wave{
			{ClusterName: "auth", ID: "w2", Prerequisites: []string{"billing:w1"}},
		}

		// when
		result := domain.NormalizeWavePrerequisites(waves)

		// then
		if result[0].Prerequisites[0] != "billing:w1" {
			t.Errorf("prerequisite = %q, want %q", result[0].Prerequisites[0], "billing:w1")
		}
	})

	t.Run("does_not_mutate_input", func(t *testing.T) {
		t.Parallel()
		// given
		waves := []sightjack.Wave{
			{ClusterName: "auth", ID: "w2", Prerequisites: []string{"w1"}},
		}

		// when
		_ = domain.NormalizeWavePrerequisites(waves)

		// then — original unchanged
		if waves[0].Prerequisites[0] != "w1" {
			t.Errorf("original mutated: prerequisite = %q, want %q", waves[0].Prerequisites[0], "w1")
		}
	})
}

func TestMergeWaveResults(t *testing.T) {
	t.Parallel()
	// given
	results := []sightjack.WaveGenerateResult{
		{ClusterName: "auth", Waves: []sightjack.Wave{
			{ClusterName: "auth", ID: "w1", Prerequisites: []string{"w0"}},
		}},
		{ClusterName: "billing", Waves: []sightjack.Wave{
			{ClusterName: "billing", ID: "w1"},
		}},
	}

	// when
	merged := domain.MergeWaveResults(results)

	// then
	if len(merged) != 2 {
		t.Fatalf("merged len = %d, want 2", len(merged))
	}
	// prerequisite normalized
	if merged[0].Prerequisites[0] != "auth:w0" {
		t.Errorf("prerequisite = %q, want %q", merged[0].Prerequisites[0], "auth:w0")
	}
}

func TestAvailableWaves(t *testing.T) {
	t.Parallel()
	// given
	waves := []sightjack.Wave{
		{ClusterName: "auth", ID: "w1", Status: "available"},
		{ClusterName: "auth", ID: "w2", Status: "locked"},
		{ClusterName: "billing", ID: "w1", Status: "available"},
		{ClusterName: "billing", ID: "w2", Status: "completed"},
	}
	completed := map[string]bool{"billing:w1": true}

	// when
	avail := domain.AvailableWaves(waves, completed)

	// then — only auth:w1 (billing:w1 is in completed map)
	if len(avail) != 1 {
		t.Fatalf("available count = %d, want 1", len(avail))
	}
	if domain.WaveKey(avail[0]) != "auth:w1" {
		t.Errorf("available wave = %q, want %q", domain.WaveKey(avail[0]), "auth:w1")
	}
}

func TestEvaluateUnlocks(t *testing.T) {
	t.Parallel()

	t.Run("unlocks_when_prerequisites_met", func(t *testing.T) {
		t.Parallel()
		// given
		waves := []sightjack.Wave{
			{ClusterName: "auth", ID: "w1", Status: "completed"},
			{ClusterName: "auth", ID: "w2", Status: "locked", Prerequisites: []string{"auth:w1"}},
		}
		completed := map[string]bool{"auth:w1": true}

		// when
		result := domain.EvaluateUnlocks(waves, completed)

		// then
		if result[1].Status != "available" {
			t.Errorf("w2 status = %q, want %q", result[1].Status, "available")
		}
	})

	t.Run("stays_locked_when_prerequisite_unmet", func(t *testing.T) {
		t.Parallel()
		// given
		waves := []sightjack.Wave{
			{ClusterName: "auth", ID: "w2", Status: "locked", Prerequisites: []string{"auth:w1"}},
		}
		completed := map[string]bool{}

		// when
		result := domain.EvaluateUnlocks(waves, completed)

		// then
		if result[0].Status != "locked" {
			t.Errorf("w2 status = %q, want %q", result[0].Status, "locked")
		}
	})

	t.Run("non_locked_waves_unchanged", func(t *testing.T) {
		t.Parallel()
		// given
		waves := []sightjack.Wave{
			{ClusterName: "auth", ID: "w1", Status: "available"},
			{ClusterName: "auth", ID: "w2", Status: "completed"},
		}

		// when
		result := domain.EvaluateUnlocks(waves, map[string]bool{})

		// then
		if result[0].Status != "available" {
			t.Errorf("w1 status = %q, want %q", result[0].Status, "available")
		}
		if result[1].Status != "completed" {
			t.Errorf("w2 status = %q, want %q", result[1].Status, "completed")
		}
	})
}

func TestCalcNewlyUnlocked(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		oldAvailable int
		newAvailable int
		want         int
	}{
		{name: "two_unlocked", oldAvailable: 3, newAvailable: 4, want: 2},
		{name: "none_unlocked", oldAvailable: 2, newAvailable: 1, want: 0},
		{name: "exactly_one", oldAvailable: 1, newAvailable: 1, want: 1},
		{name: "negative_clamped", oldAvailable: 5, newAvailable: 1, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// when
			got := domain.CalcNewlyUnlocked(tt.oldAvailable, tt.newAvailable)

			// then
			if got != tt.want {
				t.Errorf("CalcNewlyUnlocked(%d, %d) = %d, want %d", tt.oldAvailable, tt.newAvailable, got, tt.want)
			}
		})
	}
}

func TestPartialApplyDelta(t *testing.T) {
	t.Parallel()
	delta := sightjack.WaveDelta{Before: 0.5, After: 0.9}

	tests := []struct {
		name   string
		result *sightjack.WaveApplyResult
		want   float64
	}{
		{
			name:   "all_applied",
			result: &sightjack.WaveApplyResult{Applied: 3, TotalCount: 3},
			want:   0.9,
		},
		{
			name:   "none_applied",
			result: &sightjack.WaveApplyResult{Applied: 0, TotalCount: 3},
			want:   0.5,
		},
		{
			name:   "half_applied",
			result: &sightjack.WaveApplyResult{Applied: 1, TotalCount: 2},
			want:   0.7, // 0.5 + (0.9-0.5)*0.5
		},
		{
			name:   "zero_total_returns_after",
			result: &sightjack.WaveApplyResult{Applied: 0, TotalCount: 0},
			want:   0.9,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// when
			got := domain.PartialApplyDelta(tt.result, delta)

			// then
			if got != tt.want {
				t.Errorf("PartialApplyDelta = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestIsWaveApplyComplete(t *testing.T) {
	t.Parallel()

	t.Run("complete_when_no_errors", func(t *testing.T) {
		t.Parallel()
		// given
		result := &sightjack.WaveApplyResult{Applied: 3, TotalCount: 3}

		// when/then
		if !domain.IsWaveApplyComplete(result) {
			t.Error("expected complete (no errors)")
		}
	})

	t.Run("incomplete_when_errors", func(t *testing.T) {
		t.Parallel()
		// given
		result := &sightjack.WaveApplyResult{Applied: 2, TotalCount: 3, Errors: []string{"fail"}}

		// when/then
		if domain.IsWaveApplyComplete(result) {
			t.Error("expected incomplete (has errors)")
		}
	})
}

func TestApplyModifiedWave(t *testing.T) {
	t.Parallel()

	t.Run("preserves_identity_fields", func(t *testing.T) {
		t.Parallel()
		// given
		original := sightjack.Wave{
			ID: "w1", ClusterName: "auth", Title: "Original",
			Actions:       []sightjack.WaveAction{{Type: "fix", IssueID: "1"}},
			Prerequisites: []string{"auth:w0"},
			Delta:         sightjack.WaveDelta{Before: 0.3, After: 0.6},
		}
		modified := sightjack.Wave{
			ID: "overwritten", ClusterName: "overwritten", Title: "Modified",
		}
		completed := map[string]bool{"auth:w0": true}

		// when
		result := domain.ApplyModifiedWave(original, modified, completed)

		// then — identity preserved
		if result.ID != "w1" {
			t.Errorf("ID = %q, want %q", result.ID, "w1")
		}
		if result.ClusterName != "auth" {
			t.Errorf("ClusterName = %q, want %q", result.ClusterName, "auth")
		}
		if result.Title != "Modified" {
			t.Errorf("Title = %q, want %q", result.Title, "Modified")
		}
	})

	t.Run("preserves_original_when_modified_nil", func(t *testing.T) {
		t.Parallel()
		// given
		original := sightjack.Wave{
			ID: "w1", ClusterName: "auth",
			Actions:       []sightjack.WaveAction{{Type: "fix", IssueID: "1"}},
			Prerequisites: []string{"auth:w0"},
			Delta:         sightjack.WaveDelta{Before: 0.3, After: 0.6},
		}
		modified := sightjack.Wave{} // all nil/zero
		completed := map[string]bool{"auth:w0": true}

		// when
		result := domain.ApplyModifiedWave(original, modified, completed)

		// then — original fields preserved
		if len(result.Actions) != 1 {
			t.Errorf("Actions len = %d, want 1", len(result.Actions))
		}
		if result.Delta.After != 0.6 {
			t.Errorf("Delta.After = %f, want 0.6", result.Delta.After)
		}
	})

	t.Run("normalizes_bare_prerequisites", func(t *testing.T) {
		t.Parallel()
		// given
		original := sightjack.Wave{ID: "w2", ClusterName: "auth"}
		modified := sightjack.Wave{Prerequisites: []string{"w1"}}
		completed := map[string]bool{"auth:w1": true}

		// when
		result := domain.ApplyModifiedWave(original, modified, completed)

		// then
		if result.Prerequisites[0] != "auth:w1" {
			t.Errorf("prerequisite = %q, want %q", result.Prerequisites[0], "auth:w1")
		}
		if result.Status != "available" {
			t.Errorf("status = %q, want %q", result.Status, "available")
		}
	})

	t.Run("locks_when_prerequisite_unmet", func(t *testing.T) {
		t.Parallel()
		// given
		original := sightjack.Wave{ID: "w2", ClusterName: "auth"}
		modified := sightjack.Wave{Prerequisites: []string{"w1"}}
		completed := map[string]bool{} // w1 not completed

		// when
		result := domain.ApplyModifiedWave(original, modified, completed)

		// then
		if result.Status != "locked" {
			t.Errorf("status = %q, want %q", result.Status, "locked")
		}
	})
}

func TestPropagateWaveUpdate(t *testing.T) {
	t.Parallel()
	// given
	waves := []sightjack.Wave{
		{ClusterName: "auth", ID: "w1", Title: "Old"},
		{ClusterName: "auth", ID: "w2", Title: "Other"},
	}
	updated := sightjack.Wave{ClusterName: "auth", ID: "w1", Title: "New"}

	// when
	domain.PropagateWaveUpdate(waves, updated)

	// then
	if waves[0].Title != "New" {
		t.Errorf("waves[0].Title = %q, want %q", waves[0].Title, "New")
	}
	if waves[1].Title != "Other" {
		t.Errorf("waves[1].Title should be unchanged, got %q", waves[1].Title)
	}
}

func TestBuildCompletedWaveMap(t *testing.T) {
	t.Parallel()
	// given
	waves := []sightjack.Wave{
		{ClusterName: "auth", ID: "w1", Status: "completed"},
		{ClusterName: "auth", ID: "w2", Status: "available"},
		{ClusterName: "billing", ID: "w1", Status: "completed"},
	}

	// when
	completed := domain.BuildCompletedWaveMap(waves)

	// then
	if !completed["auth:w1"] {
		t.Error("expected auth:w1 in completed")
	}
	if completed["auth:w2"] {
		t.Error("auth:w2 should not be in completed")
	}
	if !completed["billing:w1"] {
		t.Error("expected billing:w1 in completed")
	}
}

func TestMergeOldWaves(t *testing.T) {
	t.Parallel()

	t.Run("carries_forward_failed_cluster_waves", func(t *testing.T) {
		t.Parallel()
		// given
		oldWaves := []sightjack.Wave{
			{ClusterName: "auth", ID: "w1", Title: "Old Auth Wave"},
		}
		newWaves := []sightjack.Wave{
			{ClusterName: "billing", ID: "w1", Title: "New Billing Wave"},
		}
		scannedClusters := map[string]bool{"auth": true, "billing": true}
		failedClusters := map[string]bool{"auth": true}

		// when
		merged := domain.MergeOldWaves(oldWaves, newWaves, scannedClusters, failedClusters)

		// then — old auth wave carried forward + new billing wave
		if len(merged) != 2 {
			t.Fatalf("merged len = %d, want 2", len(merged))
		}
	})

	t.Run("drops_waves_from_removed_clusters", func(t *testing.T) {
		t.Parallel()
		// given — auth was removed from scan
		oldWaves := []sightjack.Wave{
			{ClusterName: "auth", ID: "w1"},
		}
		scannedClusters := map[string]bool{"billing": true}
		failedClusters := map[string]bool{}

		// when
		merged := domain.MergeOldWaves(oldWaves, nil, scannedClusters, failedClusters)

		// then — auth dropped
		if len(merged) != 0 {
			t.Errorf("merged len = %d, want 0", len(merged))
		}
	})

	t.Run("no_duplicate_keys", func(t *testing.T) {
		t.Parallel()
		// given — same wave in both old and new
		oldWaves := []sightjack.Wave{
			{ClusterName: "auth", ID: "w1"},
		}
		newWaves := []sightjack.Wave{
			{ClusterName: "auth", ID: "w1"},
		}
		scannedClusters := map[string]bool{"auth": true}

		// when
		merged := domain.MergeOldWaves(oldWaves, newWaves, scannedClusters, map[string]bool{})

		// then — no duplicate
		if len(merged) != 1 {
			t.Errorf("merged len = %d, want 1", len(merged))
		}
	})
}

func TestMergeCompletedStatus(t *testing.T) {
	t.Parallel()
	// given
	oldCompleted := map[string]bool{"auth:w1": true}
	newWaves := []sightjack.Wave{
		{ClusterName: "auth", ID: "w1", Status: "available"},
		{ClusterName: "auth", ID: "w2", Status: "locked"},
	}

	// when
	result := domain.MergeCompletedStatus(oldCompleted, newWaves)

	// then
	if result[0].Status != "completed" {
		t.Errorf("w1 status = %q, want %q", result[0].Status, "completed")
	}
	if result[1].Status != "locked" {
		t.Errorf("w2 status = %q, want %q", result[1].Status, "locked")
	}
	// original unchanged
	if newWaves[0].Status != "available" {
		t.Errorf("original mutated: w1 status = %q, want %q", newWaves[0].Status, "available")
	}
}

func TestRestoreWaves(t *testing.T) {
	t.Parallel()
	// given
	states := []sightjack.WaveState{
		{
			ID: "w1", ClusterName: "auth", Title: "Fix login",
			Description: "desc", Status: "available",
			Actions:       []sightjack.WaveAction{{Type: "fix", IssueID: "1"}},
			Prerequisites: []string{"auth:w0"},
			Delta:         sightjack.WaveDelta{Before: 0.3, After: 0.7},
		},
	}

	// when
	waves := domain.RestoreWaves(states)

	// then
	if len(waves) != 1 {
		t.Fatalf("waves len = %d, want 1", len(waves))
	}
	w := waves[0]
	if w.ID != "w1" || w.ClusterName != "auth" || w.Title != "Fix login" {
		t.Errorf("identity mismatch: %+v", w)
	}
	if w.Status != "available" {
		t.Errorf("status = %q, want %q", w.Status, "available")
	}
	if len(w.Actions) != 1 {
		t.Errorf("actions len = %d, want 1", len(w.Actions))
	}
	if w.Delta.After != 0.7 {
		t.Errorf("delta.After = %f, want 0.7", w.Delta.After)
	}
}

func TestBuildWaveStates(t *testing.T) {
	t.Parallel()
	// given
	waves := []sightjack.Wave{
		{
			ID: "w1", ClusterName: "auth", Title: "Fix login",
			Description: "desc", Status: "available",
			Actions:       []sightjack.WaveAction{{Type: "fix", IssueID: "1"}, {Type: "fix", IssueID: "2"}},
			Prerequisites: []string{"auth:w0"},
			Delta:         sightjack.WaveDelta{Before: 0.3, After: 0.7},
		},
	}

	// when
	states := domain.BuildWaveStates(waves)

	// then
	if len(states) != 1 {
		t.Fatalf("states len = %d, want 1", len(states))
	}
	s := states[0]
	if s.ActionCount != 2 {
		t.Errorf("ActionCount = %d, want 2", s.ActionCount)
	}
	if s.ID != "w1" || s.ClusterName != "auth" {
		t.Errorf("identity mismatch: %+v", s)
	}
}

func TestCheckCompletenessConsistency(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		overall  float64
		clusters []sightjack.ClusterScanResult
		want     bool
	}{
		{
			name:    "consistent",
			overall: 0.7,
			clusters: []sightjack.ClusterScanResult{
				{Completeness: 0.7}, {Completeness: 0.7},
			},
			want: false,
		},
		{
			name:    "inconsistent_beyond_tolerance",
			overall: 0.9,
			clusters: []sightjack.ClusterScanResult{
				{Completeness: 0.5}, {Completeness: 0.5},
			},
			want: true, // diff = 0.4 > 0.05
		},
		{
			name:     "empty_clusters",
			overall:  0.5,
			clusters: nil,
			want:     false,
		},
		{
			name:    "within_tolerance",
			overall: 0.72,
			clusters: []sightjack.ClusterScanResult{
				{Completeness: 0.7}, {Completeness: 0.7},
			},
			want: false, // diff = 0.02 < 0.05
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// when
			got := domain.CheckCompletenessConsistency(tt.overall, tt.clusters)

			// then
			if got != tt.want {
				t.Errorf("CheckCompletenessConsistency(%f, ...) = %v, want %v", tt.overall, got, tt.want)
			}
		})
	}
}

func TestCompletedWavesForCluster(t *testing.T) {
	t.Parallel()
	// given
	waves := []sightjack.Wave{
		{ClusterName: "auth", ID: "w1", Status: "completed"},
		{ClusterName: "auth", ID: "w2", Status: "available"},
		{ClusterName: "billing", ID: "w1", Status: "completed"},
	}

	// when
	result := domain.CompletedWavesForCluster(waves, "auth")

	// then
	if len(result) != 1 {
		t.Fatalf("result len = %d, want 1", len(result))
	}
	if result[0].ID != "w1" {
		t.Errorf("ID = %q, want %q", result[0].ID, "w1")
	}
}
