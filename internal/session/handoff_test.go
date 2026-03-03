package session_test

import (
	"context"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/port"
	"github.com/hironow/sightjack/internal/session"
)

// Compile-time check: Handoff interface exists and has expected methods.
var _ port.Handoff = (*handoffChecker)(nil)

type handoffChecker struct{}

func (p *handoffChecker) HandoffReady(_ context.Context, _ []string) error        { return nil }
func (p *handoffChecker) ReportIssue(_ context.Context, _ string, _ string) error { return nil }

func TestHandoff_InterfaceCompiles(t *testing.T) {
	// given: a type that implements Handoff
	var h port.Handoff = &handoffChecker{}

	// when / then: calling methods should not panic
	if err := h.HandoffReady(context.Background(), []string{"ENG-101"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := h.ReportIssue(context.Background(), "ENG-101", "finding"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandoffResult_ZeroValue(t *testing.T) {
	// given: zero-value HandoffResult
	var result domain.HandoffResult

	// then: fields should have zero values
	if result.IssueID != "" {
		t.Errorf("expected empty IssueID, got %s", result.IssueID)
	}
	if result.Status != "" {
		t.Errorf("expected empty Status, got %s", result.Status)
	}
}

func TestReadyIssueIDs(t *testing.T) {
	waves := []domain.Wave{
		{ID: "w1", ClusterName: "auth", Status: "completed",
			Actions: []domain.WaveAction{
				{IssueID: "AUTH-1"},
				{IssueID: "AUTH-2"},
			}},
		{ID: "w2", ClusterName: "auth", Status: "completed",
			Actions: []domain.WaveAction{
				{IssueID: "AUTH-2"},
				{IssueID: "AUTH-3"},
			}},
		{ID: "w3", ClusterName: "auth", Status: "available",
			Actions: []domain.WaveAction{
				{IssueID: "AUTH-3"},
			}},
	}

	// AUTH-1 is only in w1 (completed) -> ready
	// AUTH-2 is in w1 (completed) and w2 (completed) -> ready
	// AUTH-3 is in w2 (completed) and w3 (available) -> NOT ready
	ready := session.ReadyIssueIDs(waves)

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
	waves := []domain.Wave{
		{ID: "w1", Status: "available", Actions: []domain.WaveAction{{IssueID: "A-1"}}},
	}
	ready := session.ReadyIssueIDs(waves)
	if len(ready) != 0 {
		t.Errorf("expected 0 ready issues, got %d", len(ready))
	}
}

func TestReadyIssueIDsAllCompleted(t *testing.T) {
	waves := []domain.Wave{
		{ID: "w1", Status: "completed", Actions: []domain.WaveAction{{IssueID: "A-1"}}},
		{ID: "w2", Status: "completed", Actions: []domain.WaveAction{{IssueID: "A-1"}, {IssueID: "A-2"}}},
	}
	ready := session.ReadyIssueIDs(waves)
	if len(ready) != 2 {
		t.Errorf("expected 2 ready issues, got %d", len(ready))
	}
}

func TestReadyIssueIDs_Sorted(t *testing.T) {
	// given: multiple completed issues that would be returned in random map order
	waves := []domain.Wave{
		{ID: "w1", Status: "completed", Actions: []domain.WaveAction{
			{IssueID: "Z-1"},
			{IssueID: "A-1"},
			{IssueID: "M-1"},
		}},
	}

	// when
	ready := session.ReadyIssueIDs(waves)

	// then: results are sorted
	if len(ready) != 3 {
		t.Fatalf("expected 3 ready issues, got %d", len(ready))
	}
	if ready[0] != "A-1" || ready[1] != "M-1" || ready[2] != "Z-1" {
		t.Errorf("expected sorted [A-1, M-1, Z-1], got %v", ready)
	}
}
