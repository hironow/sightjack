package domain_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestWaveKey(t *testing.T) {
	t.Parallel()
	// given
	w := domain.Wave{ClusterName: "auth", ID: "w1"}

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
		waves := []domain.Wave{
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
		waves := []domain.Wave{
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
		waves := []domain.Wave{
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
	results := []domain.WaveGenerateResult{
		{ClusterName: "auth", Waves: []domain.Wave{
			{ClusterName: "auth", ID: "w1", Prerequisites: []string{"w0"}},
		}},
		{ClusterName: "billing", Waves: []domain.Wave{
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
	waves := []domain.Wave{
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
		waves := []domain.Wave{
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
		waves := []domain.Wave{
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
		waves := []domain.Wave{
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
	delta := domain.WaveDelta{Before: 0.5, After: 0.9}

	tests := []struct {
		name   string
		result *domain.WaveApplyResult
		want   float64
	}{
		{
			name:   "all_applied",
			result: &domain.WaveApplyResult{Applied: 3, TotalCount: 3},
			want:   0.9,
		},
		{
			name:   "none_applied",
			result: &domain.WaveApplyResult{Applied: 0, TotalCount: 3},
			want:   0.5,
		},
		{
			name:   "half_applied",
			result: &domain.WaveApplyResult{Applied: 1, TotalCount: 2},
			want:   0.7, // 0.5 + (0.9-0.5)*0.5
		},
		{
			name:   "zero_total_returns_after",
			result: &domain.WaveApplyResult{Applied: 0, TotalCount: 0},
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
		result := &domain.WaveApplyResult{Applied: 3, TotalCount: 3}

		// when/then
		if !domain.IsWaveApplyComplete(result) {
			t.Error("expected complete (no errors)")
		}
	})

	t.Run("incomplete_when_errors", func(t *testing.T) {
		t.Parallel()
		// given
		result := &domain.WaveApplyResult{Applied: 2, TotalCount: 3, Errors: []string{"fail"}}

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
		original := domain.Wave{
			ID: "w1", ClusterName: "auth", Title: "Original",
			Actions:       []domain.WaveAction{{Type: "fix", IssueID: "1"}},
			Prerequisites: []string{"auth:w0"},
			Delta:         domain.WaveDelta{Before: 0.3, After: 0.6},
		}
		modified := domain.Wave{
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
		original := domain.Wave{
			ID: "w1", ClusterName: "auth",
			Actions:       []domain.WaveAction{{Type: "fix", IssueID: "1"}},
			Prerequisites: []string{"auth:w0"},
			Delta:         domain.WaveDelta{Before: 0.3, After: 0.6},
		}
		modified := domain.Wave{} // all nil/zero
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
		original := domain.Wave{ID: "w2", ClusterName: "auth"}
		modified := domain.Wave{Prerequisites: []string{"w1"}}
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
		original := domain.Wave{ID: "w2", ClusterName: "auth"}
		modified := domain.Wave{Prerequisites: []string{"w1"}}
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
	waves := []domain.Wave{
		{ClusterName: "auth", ID: "w1", Title: "Old"},
		{ClusterName: "auth", ID: "w2", Title: "Other"},
	}
	updated := domain.Wave{ClusterName: "auth", ID: "w1", Title: "New"}

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
	waves := []domain.Wave{
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
		oldWaves := []domain.Wave{
			{ClusterName: "auth", ID: "w1", Title: "Old Auth Wave"},
		}
		newWaves := []domain.Wave{
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
		oldWaves := []domain.Wave{
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
		oldWaves := []domain.Wave{
			{ClusterName: "auth", ID: "w1"},
		}
		newWaves := []domain.Wave{
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
	newWaves := []domain.Wave{
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
	states := []domain.WaveState{
		{
			ID: "w1", ClusterName: "auth", Title: "Fix login",
			Description: "desc", Status: "available",
			Actions:       []domain.WaveAction{{Type: "fix", IssueID: "1"}},
			Prerequisites: []string{"auth:w0"},
			Delta:         domain.WaveDelta{Before: 0.3, After: 0.7},
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
	waves := []domain.Wave{
		{
			ID: "w1", ClusterName: "auth", Title: "Fix login",
			Description: "desc", Status: "available",
			Actions:       []domain.WaveAction{{Type: "fix", IssueID: "1"}, {Type: "fix", IssueID: "2"}},
			Prerequisites: []string{"auth:w0"},
			Delta:         domain.WaveDelta{Before: 0.3, After: 0.7},
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
		clusters []domain.ClusterScanResult
		want     bool
	}{
		{
			name:    "consistent",
			overall: 0.7,
			clusters: []domain.ClusterScanResult{
				{Completeness: 0.7}, {Completeness: 0.7},
			},
			want: false,
		},
		{
			name:    "inconsistent_beyond_tolerance",
			overall: 0.9,
			clusters: []domain.ClusterScanResult{
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
			clusters: []domain.ClusterScanResult{
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
	waves := []domain.Wave{
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

func TestToApplyResult_CreateActionPartialFailure(t *testing.T) {
	t.Parallel()

	// given: wave with mixed action types including create
	wave := domain.Wave{
		ID: "w1", ClusterName: "auth",
		Actions: []domain.WaveAction{
			{Type: "add_dod", IssueID: "ENG-1", Description: "Add DoD"},
			{Type: "create", IssueID: "ENG-1", Description: "Create sub-issue"},
			{Type: "add_label", IssueID: "ENG-2", Description: "Add label"},
		},
		Delta: domain.WaveDelta{Before: 0.3, After: 0.9},
	}
	// 2 of 3 actions succeeded; create (index 1) failed
	internal := &domain.WaveApplyResult{
		Applied:    1,
		TotalCount: 3,
		Errors:     []string{"Linear API error: duplicate issue", "timeout"},
	}

	// when
	result := domain.ToApplyResult(wave, internal)

	// then: first action succeeded
	if !result.AppliedActions[0].Success {
		t.Error("expected add_dod action to succeed")
	}
	// create action failed with specific error
	if result.AppliedActions[1].Success {
		t.Error("expected create action to fail")
	}
	if result.AppliedActions[1].Error != "Linear API error: duplicate issue" {
		t.Errorf("create error = %q, want duplicate issue error", result.AppliedActions[1].Error)
	}
	if result.AppliedActions[1].Type != "create" {
		t.Errorf("action type = %q, want create", result.AppliedActions[1].Type)
	}

	// wave is NOT complete (partial failure)
	if domain.IsWaveApplyComplete(internal) {
		t.Error("expected wave to be incomplete on partial failure")
	}
}

func TestValidWaveActionType(t *testing.T) {
	t.Parallel()
	valid := []string{"add_dod", "add_dependency", "add_label", "update_description", "create", "cancel"}
	for _, v := range valid {
		if !domain.ValidWaveActionType(v) {
			t.Errorf("expected %q to be valid", v)
		}
	}
	invalid := []string{"delete", "remove", "", "Cancel"}
	for _, v := range invalid {
		if domain.ValidWaveActionType(v) {
			t.Errorf("expected %q to be invalid", v)
		}
	}
}

func TestSelectiveApproval_CreateActionExcluded(t *testing.T) {
	t.Parallel()

	// given: wave with create and other actions
	wave := domain.Wave{
		ID: "w1", ClusterName: "auth",
		Actions: []domain.WaveAction{
			{Type: "add_dod", IssueID: "ENG-1"},
			{Type: "create", IssueID: "ENG-1"},
			{Type: "add_label", IssueID: "ENG-2"},
		},
		Delta: domain.WaveDelta{Before: 0.3, After: 0.9},
	}

	// when: simulate selective approval — only approve non-create actions
	approved := []domain.WaveAction{wave.Actions[0], wave.Actions[2]}
	rejected := []domain.WaveAction{wave.Actions[1]}
	wave.Actions = approved

	// Recompute delta proportionally (same as phases.go logic)
	totalActions := len(approved) + len(rejected)
	fraction := float64(len(approved)) / float64(totalActions)
	wave.Delta.After = wave.Delta.Before + (wave.Delta.After-wave.Delta.Before)*fraction

	// then: wave contains only approved actions (no create)
	if len(wave.Actions) != 2 {
		t.Fatalf("actions len = %d, want 2", len(wave.Actions))
	}
	for _, a := range wave.Actions {
		if a.Type == "create" {
			t.Error("create action should not be in approved list")
		}
	}

	// delta recomputed: 0.3 + (0.9-0.3) * (2/3) = 0.7
	if wave.Delta.After < 0.699 || wave.Delta.After > 0.701 {
		t.Errorf("delta.After = %f, want ~0.7", wave.Delta.After)
	}
}

func TestClustersForIssueIDs(t *testing.T) {
	t.Parallel()

	clusters := []domain.ClusterScanResult{
		{
			Name: "auth",
			Issues: []domain.IssueDetail{
				{Identifier: "MY-100"},
				{Identifier: "MY-101"},
			},
		},
		{
			Name: "billing",
			Issues: []domain.IssueDetail{
				{Identifier: "MY-200"},
			},
		},
	}

	t.Run("single issue maps to cluster", func(t *testing.T) {
		// when
		result := domain.ClustersForIssueIDs(clusters, []string{"MY-100"})

		// then
		if len(result) != 1 {
			t.Fatalf("len = %d, want 1", len(result))
		}
		if result[0].Name != "auth" {
			t.Errorf("name = %q, want auth", result[0].Name)
		}
	})

	t.Run("multiple issues same cluster deduplicates", func(t *testing.T) {
		// when
		result := domain.ClustersForIssueIDs(clusters, []string{"MY-100", "MY-101"})

		// then
		if len(result) != 1 {
			t.Fatalf("len = %d, want 1", len(result))
		}
	})

	t.Run("issues across clusters returns both", func(t *testing.T) {
		// when
		result := domain.ClustersForIssueIDs(clusters, []string{"MY-100", "MY-200"})

		// then
		if len(result) != 2 {
			t.Fatalf("len = %d, want 2", len(result))
		}
	})

	t.Run("unknown issue returns empty", func(t *testing.T) {
		// when
		result := domain.ClustersForIssueIDs(clusters, []string{"UNKNOWN-999"})

		// then
		if len(result) != 0 {
			t.Fatalf("len = %d, want 0", len(result))
		}
	})

	t.Run("empty issues returns empty", func(t *testing.T) {
		// when
		result := domain.ClustersForIssueIDs(clusters, nil)

		// then
		if len(result) != 0 {
			t.Fatalf("len = %d, want 0", len(result))
		}
	})
}

func TestRemoveSelfReferences(t *testing.T) {
	t.Parallel()

	t.Run("removes_self_referencing_prerequisite", func(t *testing.T) {
		t.Parallel()
		// given
		waves := []domain.Wave{
			{ClusterName: "auth", ID: "w1", Prerequisites: []string{"auth:w1", "auth:w0"}},
		}

		// when
		result, removed := domain.RemoveSelfReferences(waves)

		// then
		if removed != 1 {
			t.Errorf("removed = %d, want 1", removed)
		}
		if len(result[0].Prerequisites) != 1 {
			t.Fatalf("prerequisites len = %d, want 1", len(result[0].Prerequisites))
		}
		if result[0].Prerequisites[0] != "auth:w0" {
			t.Errorf("prerequisite = %q, want %q", result[0].Prerequisites[0], "auth:w0")
		}
	})

	t.Run("no_self_references_unchanged", func(t *testing.T) {
		t.Parallel()
		// given
		waves := []domain.Wave{
			{ClusterName: "auth", ID: "w2", Prerequisites: []string{"auth:w1"}},
		}

		// when
		result, removed := domain.RemoveSelfReferences(waves)

		// then
		if removed != 0 {
			t.Errorf("removed = %d, want 0", removed)
		}
		if len(result[0].Prerequisites) != 1 {
			t.Errorf("prerequisites len = %d, want 1", len(result[0].Prerequisites))
		}
	})

	t.Run("only_self_reference_results_in_empty_prerequisites", func(t *testing.T) {
		t.Parallel()
		// given
		waves := []domain.Wave{
			{ClusterName: "auth", ID: "w1", Prerequisites: []string{"auth:w1"}},
		}

		// when
		result, removed := domain.RemoveSelfReferences(waves)

		// then
		if removed != 1 {
			t.Errorf("removed = %d, want 1", removed)
		}
		if len(result[0].Prerequisites) != 0 {
			t.Errorf("prerequisites len = %d, want 0", len(result[0].Prerequisites))
		}
	})
}

func TestValidateWaveApplyResult(t *testing.T) {
	t.Parallel()

	t.Run("nil_result_returns_error", func(t *testing.T) {
		t.Parallel()
		// when
		err := domain.ValidateWaveApplyResult(nil, 3)

		// then
		if err == nil {
			t.Error("expected error for nil result")
		}
	})

	t.Run("degenerate_empty_result_returns_error", func(t *testing.T) {
		t.Parallel()
		// given
		result := &domain.WaveApplyResult{Applied: 0, TotalCount: 0}

		// when
		err := domain.ValidateWaveApplyResult(result, 3)

		// then
		if err == nil {
			t.Error("expected error for degenerate empty result with expected actions")
		}
	})

	t.Run("applied_exceeds_expected_returns_error", func(t *testing.T) {
		t.Parallel()
		// given
		result := &domain.WaveApplyResult{Applied: 5, TotalCount: 5}

		// when
		err := domain.ValidateWaveApplyResult(result, 3)

		// then
		if err == nil {
			t.Error("expected error when applied > expected actions")
		}
	})

	t.Run("valid_result_no_error", func(t *testing.T) {
		t.Parallel()
		// given
		result := &domain.WaveApplyResult{Applied: 3, TotalCount: 3}

		// when
		err := domain.ValidateWaveApplyResult(result, 3)

		// then
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("partial_apply_valid", func(t *testing.T) {
		t.Parallel()
		// given
		result := &domain.WaveApplyResult{Applied: 2, TotalCount: 3, Errors: []string{"fail"}}

		// when
		err := domain.ValidateWaveApplyResult(result, 3)

		// then
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("zero_expected_zero_result_valid", func(t *testing.T) {
		t.Parallel()
		// given: edge case where wave has 0 expected actions
		result := &domain.WaveApplyResult{Applied: 0, TotalCount: 0}

		// when
		err := domain.ValidateWaveApplyResult(result, 0)

		// then
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestClampDelta(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		delta      domain.WaveDelta
		wantBefore float64
		wantAfter  float64
	}{
		{name: "valid_unchanged", delta: domain.WaveDelta{Before: 0.3, After: 0.7}, wantBefore: 0.3, wantAfter: 0.7},
		{name: "negative_clamped", delta: domain.WaveDelta{Before: -0.1, After: 0.5}, wantBefore: 0.0, wantAfter: 0.5},
		{name: "above_one_clamped", delta: domain.WaveDelta{Before: 0.5, After: 1.5}, wantBefore: 0.5, wantAfter: 1.0},
		{name: "both_out_of_bounds", delta: domain.WaveDelta{Before: -0.5, After: 2.0}, wantBefore: 0.0, wantAfter: 1.0},
		{name: "regression_swapped", delta: domain.WaveDelta{Before: 0.8, After: 0.3}, wantBefore: 0.3, wantAfter: 0.8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// when
			got := domain.ClampDelta(tt.delta)

			// then
			if got.Before != tt.wantBefore {
				t.Errorf("Before = %f, want %f", got.Before, tt.wantBefore)
			}
			if got.After != tt.wantAfter {
				t.Errorf("After = %f, want %f", got.After, tt.wantAfter)
			}
		})
	}
}

func TestFilterEmptyWaves(t *testing.T) {
	t.Parallel()

	t.Run("removes_zero_action_waves", func(t *testing.T) {
		t.Parallel()
		// given
		waves := []domain.Wave{
			{ID: "w1", ClusterName: "auth", Actions: []domain.WaveAction{{Type: "fix", IssueID: "1"}}},
			{ID: "w2", ClusterName: "auth", Actions: nil},
			{ID: "w3", ClusterName: "auth", Actions: []domain.WaveAction{}},
		}

		// when
		filtered, removed := domain.FilterEmptyWaves(waves)

		// then
		if len(filtered) != 1 {
			t.Fatalf("filtered len = %d, want 1", len(filtered))
		}
		if removed != 2 {
			t.Errorf("removed = %d, want 2", removed)
		}
		if filtered[0].ID != "w1" {
			t.Errorf("filtered[0].ID = %q, want %q", filtered[0].ID, "w1")
		}
	})

	t.Run("all_valid_unchanged", func(t *testing.T) {
		t.Parallel()
		// given
		waves := []domain.Wave{
			{ID: "w1", Actions: []domain.WaveAction{{Type: "fix"}}},
		}

		// when
		filtered, removed := domain.FilterEmptyWaves(waves)

		// then
		if len(filtered) != 1 {
			t.Errorf("filtered len = %d, want 1", len(filtered))
		}
		if removed != 0 {
			t.Errorf("removed = %d, want 0", removed)
		}
	})

	t.Run("all_empty_returns_nil", func(t *testing.T) {
		t.Parallel()
		// given
		waves := []domain.Wave{
			{ID: "w1", Actions: nil},
		}

		// when
		filtered, removed := domain.FilterEmptyWaves(waves)

		// then
		if len(filtered) != 0 {
			t.Errorf("filtered len = %d, want 0", len(filtered))
		}
		if removed != 1 {
			t.Errorf("removed = %d, want 1", removed)
		}
	})
}

func TestLastCompletedWaveForCluster(t *testing.T) {
	t.Parallel()

	waves := []domain.Wave{
		{ClusterName: "auth", ID: "w1", Status: "completed"},
		{ClusterName: "auth", ID: "w2", Status: "completed"},
		{ClusterName: "auth", ID: "w3", Status: "available"},
		{ClusterName: "billing", ID: "w1", Status: "completed"},
	}

	t.Run("returns last completed wave", func(t *testing.T) {
		// when
		w, ok := domain.LastCompletedWaveForCluster(waves, "auth")

		// then
		if !ok {
			t.Fatal("expected ok=true")
		}
		if w.ID != "w2" {
			t.Errorf("ID = %q, want w2", w.ID)
		}
	})

	t.Run("no completed waves returns false", func(t *testing.T) {
		// when
		_, ok := domain.LastCompletedWaveForCluster(waves, "unknown")

		// then
		if ok {
			t.Error("expected ok=false for unknown cluster")
		}
	})
}
