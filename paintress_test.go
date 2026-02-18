package sightjack

import "testing"

func TestReadyIssueIDs(t *testing.T) {
	waves := []Wave{
		{ID: "w1", ClusterName: "auth", Status: "completed",
			Actions: []WaveAction{
				{IssueID: "AUTH-1"},
				{IssueID: "AUTH-2"},
			}},
		{ID: "w2", ClusterName: "auth", Status: "completed",
			Actions: []WaveAction{
				{IssueID: "AUTH-2"},
				{IssueID: "AUTH-3"},
			}},
		{ID: "w3", ClusterName: "auth", Status: "available",
			Actions: []WaveAction{
				{IssueID: "AUTH-3"},
			}},
	}

	// AUTH-1 is only in w1 (completed) -> ready
	// AUTH-2 is in w1 (completed) and w2 (completed) -> ready
	// AUTH-3 is in w2 (completed) and w3 (available) -> NOT ready
	ready := ReadyIssueIDs(waves)

	if len(ready) != 2 {
		t.Fatalf("expected 2 ready issues, got %d: %v", len(ready), ready)
	}
	readySet := make(map[string]bool)
	for _, id := range ready {
		readySet[id] = true
	}
	if !readySet["AUTH-1"] {
		t.Error("expected AUTH-1 to be ready")
	}
	if !readySet["AUTH-2"] {
		t.Error("expected AUTH-2 to be ready")
	}
	if readySet["AUTH-3"] {
		t.Error("expected AUTH-3 to NOT be ready")
	}
}

func TestReadyIssueIDsNoCompleted(t *testing.T) {
	waves := []Wave{
		{ID: "w1", Status: "available", Actions: []WaveAction{{IssueID: "A-1"}}},
	}
	ready := ReadyIssueIDs(waves)
	if len(ready) != 0 {
		t.Errorf("expected 0 ready issues, got %d", len(ready))
	}
}

func TestReadyIssueIDsAssuming(t *testing.T) {
	waves := []Wave{
		{ID: "w1", ClusterName: "auth", Status: "completed",
			Actions: []WaveAction{
				{IssueID: "AUTH-1"},
				{IssueID: "AUTH-2"},
			}},
		{ID: "w2", ClusterName: "auth", Status: "available",
			Actions: []WaveAction{
				{IssueID: "AUTH-2"},
				{IssueID: "AUTH-3"},
			}},
		{ID: "w3", ClusterName: "auth", Status: "available",
			Actions: []WaveAction{
				{IssueID: "AUTH-3"},
			}},
	}

	// Assuming w2 will complete:
	// AUTH-1: w1(completed) -> ready
	// AUTH-2: w1(completed) + w2(assumed completed) -> ready
	// AUTH-3: w2(assumed completed) + w3(available) -> NOT ready
	ready := ReadyIssueIDsAssuming(waves, Wave{ID: "w2", ClusterName: "auth"})

	if len(ready) != 2 {
		t.Fatalf("expected 2 ready issues, got %d: %v", len(ready), ready)
	}
	readySet := make(map[string]bool)
	for _, id := range ready {
		readySet[id] = true
	}
	if !readySet["AUTH-1"] {
		t.Error("expected AUTH-1 to be ready")
	}
	if !readySet["AUTH-2"] {
		t.Error("expected AUTH-2 to be ready")
	}
	if readySet["AUTH-3"] {
		t.Error("expected AUTH-3 to NOT be ready")
	}
}

func TestReadyIssueIDsAllCompleted(t *testing.T) {
	waves := []Wave{
		{ID: "w1", Status: "completed", Actions: []WaveAction{{IssueID: "A-1"}}},
		{ID: "w2", Status: "completed", Actions: []WaveAction{{IssueID: "A-1"}, {IssueID: "A-2"}}},
	}
	ready := ReadyIssueIDs(waves)
	if len(ready) != 2 {
		t.Errorf("expected 2 ready issues, got %d", len(ready))
	}
}
