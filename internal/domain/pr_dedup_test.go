package domain

import "testing"

func TestFilterPROpenActions_RemovesImplementationForPROpenIssues(t *testing.T) {
	// given: wave with mixed actions — some issues have PR open, some don't
	waves := []Wave{
		{
			ID:          "w1",
			ClusterName: "validation",
			Actions: []WaveAction{
				{Type: "implement", IssueID: "5", Description: "Add validation"},
				{Type: "add_dod", IssueID: "5", Description: "Add DoD to #5"},
				{Type: "implement", IssueID: "3", Description: "Fix pagination"},
			},
		},
	}
	prOpenIssues := map[string]bool{"5": true}

	// when
	filtered := FilterPROpenActions(waves, prOpenIssues)

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
	waves := []Wave{
		{
			ID:          "w1",
			ClusterName: "core",
			Actions: []WaveAction{
				{Type: "implement", IssueID: "1", Description: "Implement feature"},
				{Type: "fix", IssueID: "2", Description: "Fix bug"},
			},
		},
	}
	prOpenIssues := map[string]bool{}

	// when
	filtered := FilterPROpenActions(waves, prOpenIssues)

	// then: all actions preserved
	if len(filtered[0].Actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(filtered[0].Actions))
	}
}

func TestFilterPROpenActions_RemovesWaveWithNoRemainingActions(t *testing.T) {
	// given: wave where ALL actions are implementation for PR-open issues
	waves := []Wave{
		{
			ID:          "w1",
			ClusterName: "design",
			Actions: []WaveAction{
				{Type: "implement", IssueID: "5", Description: "Implement"},
				{Type: "fix", IssueID: "5", Description: "Fix"},
			},
		},
		{
			ID:          "w2",
			ClusterName: "core",
			Actions: []WaveAction{
				{Type: "implement", IssueID: "3", Description: "Implement"},
			},
		},
	}
	prOpenIssues := map[string]bool{"5": true}

	// when
	filtered := FilterPROpenActions(waves, prOpenIssues)

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
	clusters := []ClusterScanResult{
		{
			Name: "validation",
			Issues: []IssueDetail{
				{ID: "5", Labels: []string{"paintress:pr-open"}},
				{ID: "3", Labels: []string{"sightjack:analyzed"}},
			},
		},
		{
			Name: "infra",
			Issues: []IssueDetail{
				{ID: "21", Labels: []string{"paintress:pr-open", "sightjack:wave-done"}},
			},
		},
	}

	// when
	result := CollectPROpenIssues(clusters)

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
	waves := []Wave{
		{
			ID:          "w1",
			ClusterName: "validation",
			Actions: []WaveAction{
				{Type: "implement", IssueID: "5", Description: "Implement"},
				{Type: "add_dod", IssueID: "3", Description: "Add DoD"},
			},
		},
		{
			ID:          "w2",
			ClusterName: "infra",
			Actions: []WaveAction{
				{Type: "fix", IssueID: "21", Description: "Fix"},
			},
		},
	}
	completed := map[string]bool{"validation:w1": true} // only w1 completed

	// when
	result := CollectSpecSentIssueIDs(completed, waves)

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

func TestIssueDetail_HasPROpen(t *testing.T) {
	// given
	issue := IssueDetail{
		ID:     "5",
		Labels: []string{"sightjack:analyzed", "paintress:pr-open"},
	}

	// then
	if !issue.HasPROpen() {
		t.Error("expected HasPROpen=true for issue with paintress:pr-open label")
	}

	issueWithout := IssueDetail{
		ID:     "3",
		Labels: []string{"sightjack:analyzed"},
	}
	if issueWithout.HasPROpen() {
		t.Error("expected HasPROpen=false for issue without paintress:pr-open label")
	}
}
