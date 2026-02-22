package sightjack

import (
	"context"
	"fmt"
	"io"
	"testing"
)

func TestFilterConvergence_Empty(t *testing.T) {
	// given: empty slice
	result := FilterConvergence(nil)

	// then
	if len(result) != 0 {
		t.Errorf("expected 0 convergence, got %d", len(result))
	}
}

func TestFilterConvergence_MixedKinds(t *testing.T) {
	// given: mixed d-mails
	dmails := []*DMail{
		{Name: "fb-1", Kind: DMailFeedback, Description: "feedback"},
		{Name: "conv-1", Kind: DMailConvergence, Description: "convergence 1"},
		{Name: "spec-1", Kind: DMailSpecification, Description: "spec"},
		{Name: "conv-2", Kind: DMailConvergence, Description: "convergence 2"},
	}

	// when
	result := FilterConvergence(dmails)

	// then
	if len(result) != 2 {
		t.Fatalf("expected 2 convergence, got %d", len(result))
	}
	if result[0].Name != "conv-1" {
		t.Errorf("first: got %s, want conv-1", result[0].Name)
	}
	if result[1].Name != "conv-2" {
		t.Errorf("second: got %s, want conv-2", result[1].Name)
	}
}

func TestConvergenceGate_NoConvergence(t *testing.T) {
	// given: no convergence d-mails
	dmails := []*DMail{
		{Name: "fb-1", Kind: DMailFeedback, Description: "feedback only"},
	}
	notifier := &NopNotifier{}
	approver := &AutoApprover{}
	logger := NewLogger(io.Discard, false)

	// when
	approved, err := RunConvergenceGate(context.Background(), dmails, notifier, approver, logger)

	// then: pass through (no gate)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("expected approval when no convergence d-mails")
	}
}

func TestConvergenceGate_Approved(t *testing.T) {
	// given: convergence d-mail + auto-approve
	dmails := []*DMail{
		{Name: "conv-1", Kind: DMailConvergence, Description: "convergence signal"},
	}
	notifier := &NopNotifier{}
	approver := &AutoApprover{}
	logger := NewLogger(io.Discard, false)

	// when
	approved, err := RunConvergenceGate(context.Background(), dmails, notifier, approver, logger)

	// then
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("expected approval")
	}
}

func TestConvergenceGate_Denied(t *testing.T) {
	// given: convergence d-mail + denying approver
	dmails := []*DMail{
		{Name: "conv-1", Kind: DMailConvergence, Description: "convergence signal"},
	}
	notifier := &NopNotifier{}
	approver := &denyApprover{}
	logger := NewLogger(io.Discard, false)

	// when
	approved, err := RunConvergenceGate(context.Background(), dmails, notifier, approver, logger)

	// then
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if approved {
		t.Error("expected denial")
	}
}

func TestConvergenceGate_FailClosed(t *testing.T) {
	// given: convergence d-mail + failing approver
	dmails := []*DMail{
		{Name: "conv-1", Kind: DMailConvergence, Description: "convergence signal"},
	}
	notifier := &NopNotifier{}
	approver := &errorApprover{err: fmt.Errorf("approval service down")}
	logger := NewLogger(io.Discard, false)

	// when
	approved, err := RunConvergenceGate(context.Background(), dmails, notifier, approver, logger)

	// then: fail-closed
	if err == nil {
		t.Error("expected error for failing approver")
	}
	if approved {
		t.Error("expected denial on error (fail-closed)")
	}
}

// --- test helpers ---

type denyApprover struct{}

func (a *denyApprover) RequestApproval(_ context.Context, _ string) (bool, error) {
	return false, nil
}

type errorApprover struct {
	err error
}

func (a *errorApprover) RequestApproval(_ context.Context, _ string) (bool, error) {
	return false, a.err
}
