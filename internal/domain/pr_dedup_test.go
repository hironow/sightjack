package domain_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestFilterPROpenActions_RemovesImplementationForPROpenIssues(t *testing.T) {
	// given: wave with mixed actions — some issues have PR open, some don't
	waves := []domain.Wave{
		{
			ID:          "w1",
			ClusterName: "validation",
			Actions: []domain.WaveAction{
				{Type: "implement", IssueID: "5", Description: "Add validation"},
				{Type: "add_dod", IssueID: "5", Description: "Add DoD to #5"},
				{Type: "implement", IssueID: "3", Description: "Fix pagination"},
			},
		},
	}
	prOpenIssues := map[string]bool{"5": true}

	// when
	filtered := domain.FilterPROpenActions(waves, prOpenIssues)

	// then: issue #5's implementation removed, but add_dod kept; issue #3 unchanged
	if len(filtered) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(filtered))
	}
	actions := filtered[0].Actions
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions (add_dod for #5 + implement for #3), got %d", len(actions))
	}
	for _, a := range actions {
		if a.IssueID == "5" && a.Type != "add_dod" {
			t.Errorf("expected only add_dod for issue #5, got %s", a.Type)
		}
	}
}

func TestFilterPROpenActions_KeepsAllWhenNoPROpen(t *testing.T) {
	// given: no issues have PR open
	waves := []domain.Wave{
		{
			ID:          "w1",
			ClusterName: "core",
			Actions: []domain.WaveAction{
				{Type: "implement", IssueID: "1", Description: "Implement feature"},
				{Type: "fix", IssueID: "2", Description: "Fix bug"},
			},
		},
	}
	prOpenIssues := map[string]bool{}

	// when
	filtered := domain.FilterPROpenActions(waves, prOpenIssues)

	// then: all actions preserved
	if len(filtered[0].Actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(filtered[0].Actions))
	}
}

func TestFilterPROpenActions_RemovesWaveWithNoRemainingActions(t *testing.T) {
	// given: wave where ALL actions are implementation for PR-open issues
	waves := []domain.Wave{
		{
			ID:          "w1",
			ClusterName: "design",
			Actions: []domain.WaveAction{
				{Type: "implement", IssueID: "5", Description: "Implement"},
				{Type: "fix", IssueID: "5", Description: "Fix"},
			},
		},
		{
			ID:          "w2",
			ClusterName: "core",
			Actions: []domain.WaveAction{
				{Type: "implement", IssueID: "3", Description: "Implement"},
			},
		},
	}
	prOpenIssues := map[string]bool{"5": true}

	// when
	filtered := domain.FilterPROpenActions(waves, prOpenIssues)

	// then: w1 removed entirely (no remaining actions), w2 kept
	if len(filtered) != 1 {
		t.Fatalf("expected 1 wave (w2 only), got %d", len(filtered))
	}
	if filtered[0].ID != "w2" {
		t.Errorf("expected w2, got %s", filtered[0].ID)
	}
}

func TestCollectPROpenIssues_ExtractsFromClusters(t *testing.T) {
	// given
	clusters := []domain.ClusterScanResult{
		{
			Name: "validation",
			Issues: []domain.IssueDetail{
				{ID: "5", Labels: []string{"paintress:pr-open"}},
				{ID: "3", Labels: []string{"sightjack:analyzed"}},
			},
		},
		{
			Name: "infra",
			Issues: []domain.IssueDetail{
				{ID: "21", Labels: []string{"paintress:pr-open", "sightjack:wave-done"}},
			},
		},
	}

	// when
	result := domain.CollectPROpenIssues(clusters)

	// then
	if len(result) != 2 {
		t.Fatalf("expected 2 PR-open issues, got %d", len(result))
	}
	if !result["5"] || !result["21"] {
		t.Errorf("expected issues 5 and 21, got %v", result)
	}
	if result["3"] {
		t.Error("issue 3 should not be in PR-open set")
	}
}

func TestCollectSpecSentIssueIDs_FromCompletedWaves(t *testing.T) {
	// given: 2 waves, one completed with implementation actions
	waves := []domain.Wave{
		{
			ID:          "w1",
			ClusterName: "validation",
			Actions: []domain.WaveAction{
				{Type: "implement", IssueID: "5", Description: "Implement"},
				{Type: "add_dod", IssueID: "3", Description: "Add DoD"},
			},
		},
		{
			ID:          "w2",
			ClusterName: "infra",
			Actions: []domain.WaveAction{
				{Type: "fix", IssueID: "21", Description: "Fix"},
			},
		},
	}
	completed := map[string]bool{"validation:w1": true} // only w1 completed

	// when
	result := domain.CollectSpecSentIssueIDs(completed, waves)

	// then: only implementation actions from completed waves
	if !result["5"] {
		t.Error("expected issue 5 (implement in completed w1)")
	}
	if result["3"] {
		t.Error("issue 3 has add_dod (issue-management), should not be in set")
	}
	if result["21"] {
		t.Error("issue 21 is in uncompleted w2, should not be in set")
	}
}

// TestFilterPROpenActions_GoTaskboardScenario reproduces the duplicate PR problem
// discovered during go-taskboard 4-tool parallel operation (2026-03-30).
// Issue #5 had 3 PRs (#14, #17, #26) all implementing status validation.
// Issues #2, #3 had 4 PRs (#16, #19, #24, #25) all implementing pagination validation.
// Root cause: sightjack generated implementation waves for issues that already had open PRs.
func TestFilterPROpenActions_GoTaskboardScenario(t *testing.T) {
	// given: go-taskboard-like waves with mixed issue management + implementation
	waves := []domain.Wave{
		{
			ID:          "validation-w1",
			ClusterName: "status-validation",
			Actions: []domain.WaveAction{
				{Type: "add_dod", IssueID: "5", Description: "Add DoD to status validation"},
				{Type: "implement", IssueID: "5", Description: "Add ErrInvalidStatus validation"},
				{Type: "add_dod", IssueID: "10", Description: "Add DoD to error handling"},
				{Type: "implement", IssueID: "10", Description: "Replace strings.Contains with errors.Is"},
			},
		},
		{
			ID:          "pagination-w1",
			ClusterName: "pagination",
			Actions: []domain.WaveAction{
				{Type: "implement", IssueID: "2", Description: "Add offset validation"},
				{Type: "implement", IssueID: "3", Description: "Add limit validation"},
				{Type: "fix", IssueID: "1", Description: "Fix off-by-one in pagination"},
			},
		},
	}
	// Issues #5, #2, #3 already have PRs from previous session
	prOpenIssues := map[string]bool{"5": true, "2": true, "3": true}

	// when
	filtered := domain.FilterPROpenActions(waves, prOpenIssues)

	// then: validation-w1 should keep add_dod for #5, implement for #10
	if len(filtered) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(filtered))
	}

	// validation-w1: add_dod(#5) + add_dod(#10) + implement(#10) = 3 actions
	valWave := filtered[0]
	if len(valWave.Actions) != 3 {
		t.Errorf("validation-w1: expected 3 actions (add_dod #5, add_dod #10, implement #10), got %d", len(valWave.Actions))
	}
	for _, a := range valWave.Actions {
		if a.IssueID == "5" && a.Type == "implement" {
			t.Error("validation-w1: implement for issue #5 should be filtered (PR exists)")
		}
	}

	// pagination-w1: only fix(#1) remains (#2, #3 have PRs)
	pagWave := filtered[1]
	if len(pagWave.Actions) != 1 {
		t.Errorf("pagination-w1: expected 1 action (fix #1), got %d", len(pagWave.Actions))
	}
	if pagWave.Actions[0].IssueID != "1" {
		t.Errorf("pagination-w1: expected action for issue #1, got %s", pagWave.Actions[0].IssueID)
	}
}

// TestCollectSpecSentIssueIDs_PreventsRaceDuplication verifies that even without
// paintress:pr-open labels, spec-sent issue tracking prevents duplicate waves
// during rescan (covers the race window before paintress applies labels).
func TestCollectSpecSentIssueIDs_PreventsRaceDuplication(t *testing.T) {
	// given: session where wave-1 was completed (spec sent to paintress)
	// but paintress hasn't applied pr-open label yet (race window)
	waves := []domain.Wave{
		{
			ID:          "w1",
			ClusterName: "validation",
			Actions: []domain.WaveAction{
				{Type: "implement", IssueID: "5", Description: "Status validation"},
				{Type: "implement", IssueID: "2", Description: "Offset validation"},
				{Type: "add_dod", IssueID: "3", Description: "Add DoD"},
			},
		},
	}
	completed := map[string]bool{"validation:w1": true}

	// when: collect spec-sent issue IDs
	specSent := domain.CollectSpecSentIssueIDs(completed, waves)

	// then: implementation issues are tracked, issue-management issues are not
	if !specSent["5"] {
		t.Error("issue 5 should be tracked (implement action)")
	}
	if !specSent["2"] {
		t.Error("issue 2 should be tracked (implement action)")
	}
	if specSent["3"] {
		t.Error("issue 3 should NOT be tracked (add_dod is issue-management)")
	}

	// when: use spec-sent as additional filter
	newWaves := []domain.Wave{
		{
			ID:          "w2",
			ClusterName: "validation",
			Actions: []domain.WaveAction{
				{Type: "implement", IssueID: "5", Description: "DUPLICATE: same status validation"},
				{Type: "implement", IssueID: "9", Description: "New: IDGenerator interface"},
			},
		},
	}
	filtered := domain.FilterPROpenActions(newWaves, specSent)

	// then: issue #5 filtered (spec already sent), issue #9 kept (new)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(filtered))
	}
	if len(filtered[0].Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(filtered[0].Actions))
	}
	if filtered[0].Actions[0].IssueID != "9" {
		t.Errorf("expected action for issue #9, got %s", filtered[0].Actions[0].IssueID)
	}
}

func TestIssueDetail_HasPROpen(t *testing.T) {
	// given
	issue := domain.IssueDetail{
		ID:     "5",
		Labels: []string{"sightjack:analyzed", "paintress:pr-open"},
	}

	// then
	if !issue.HasPROpen() {
		t.Error("expected HasPROpen=true for issue with paintress:pr-open label")
	}

	issueWithout := domain.IssueDetail{
		ID:     "3",
		Labels: []string{"sightjack:analyzed"},
	}
	if issueWithout.HasPROpen() {
		t.Error("expected HasPROpen=false for issue without paintress:pr-open label")
	}
}
