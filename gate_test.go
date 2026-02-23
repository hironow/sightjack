package sightjack_test

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/hironow/sightjack"
)

func TestFilterConvergence_Empty(t *testing.T) {
	// given: empty slice
	result := sightjack.FilterConvergence(nil)

	// then
	if len(result) != 0 {
		t.Errorf("expected 0 convergence, got %d", len(result))
	}
}

func TestFilterConvergence_MixedKinds(t *testing.T) {
	// given: mixed d-mails
	dmails := []*sightjack.DMail{
		{Name: "fb-1", Kind: sightjack.DMailFeedback, Description: "feedback"},
		{Name: "conv-1", Kind: sightjack.DMailConvergence, Description: "convergence 1"},
		{Name: "spec-1", Kind: sightjack.DMailSpecification, Description: "spec"},
		{Name: "conv-2", Kind: sightjack.DMailConvergence, Description: "convergence 2"},
	}

	// when
	result := sightjack.FilterConvergence(dmails)

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
	dmails := []*sightjack.DMail{
		{Name: "fb-1", Kind: sightjack.DMailFeedback, Description: "feedback only"},
	}
	notifier := &sightjack.NopNotifier{}
	approver := &sightjack.AutoApprover{}
	logger := sightjack.NewLogger(io.Discard, false)

	// when
	approved, err := sightjack.RunConvergenceGate(context.Background(), dmails, notifier, approver, logger)

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
	dmails := []*sightjack.DMail{
		{Name: "conv-1", Kind: sightjack.DMailConvergence, Description: "convergence signal"},
	}
	notifier := &sightjack.NopNotifier{}
	approver := &sightjack.AutoApprover{}
	logger := sightjack.NewLogger(io.Discard, false)

	// when
	approved, err := sightjack.RunConvergenceGate(context.Background(), dmails, notifier, approver, logger)

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
	dmails := []*sightjack.DMail{
		{Name: "conv-1", Kind: sightjack.DMailConvergence, Description: "convergence signal"},
	}
	notifier := &sightjack.NopNotifier{}
	approver := &denyApprover{}
	logger := sightjack.NewLogger(io.Discard, false)

	// when
	approved, err := sightjack.RunConvergenceGate(context.Background(), dmails, notifier, approver, logger)

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
	dmails := []*sightjack.DMail{
		{Name: "conv-1", Kind: sightjack.DMailConvergence, Description: "convergence signal"},
	}
	notifier := &sightjack.NopNotifier{}
	approver := &errorApprover{err: fmt.Errorf("approval service down")}
	logger := sightjack.NewLogger(io.Discard, false)

	// when
	approved, err := sightjack.RunConvergenceGate(context.Background(), dmails, notifier, approver, logger)

	// then: fail-closed
	if err == nil {
		t.Error("expected error for failing approver")
	}
	if approved {
		t.Error("expected denial on error (fail-closed)")
	}
}

func TestConvergenceGate_ContextCancel(t *testing.T) {
	// given: convergence d-mail + cancelled context.
	// Gate should return ctx.Err(), not (false, nil).
	dmails := []*sightjack.DMail{
		{Name: "conv-1", Kind: sightjack.DMailConvergence, Description: "convergence signal"},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	notifier := &sightjack.NopNotifier{}
	approver := &sightjack.AutoApprover{}
	logger := sightjack.NewLogger(io.Discard, false)

	// when
	approved, err := sightjack.RunConvergenceGate(ctx, dmails, notifier, approver, logger)

	// then: should propagate cancellation error
	if err == nil {
		t.Fatal("expected error on context cancel, got nil")
	}
	if approved {
		t.Error("expected non-approval on cancel")
	}
}

func TestConvergenceGateWithRedrain_CatchesLateConvergence(t *testing.T) {
	// given: initial drain was empty, but convergence arrived in channel
	// between the caller's drain and this gate call (simulated by pre-loading channel).
	ch := make(chan *sightjack.DMail, 2)
	ch <- &sightjack.DMail{Name: "late-conv", Kind: sightjack.DMailConvergence, Description: "late convergence"}
	notifier := &sightjack.NopNotifier{}
	approver := &sightjack.AutoApprover{}
	logger := sightjack.NewLogger(io.Discard, false)

	// when: initial is empty, gate passes through, but re-drain catches late convergence
	var initial []*sightjack.DMail
	allDmails, approved, err := sightjack.RunConvergenceGateWithRedrain(
		context.Background(), initial, ch, notifier, approver, logger,
	)

	// then: should have re-drained and required approval for late convergence
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("expected final approval")
	}
	// The late convergence should be in the accumulated dmails
	if len(allDmails) != 1 {
		t.Fatalf("expected 1 accumulated dmail, got %d", len(allDmails))
	}
	if allDmails[0].Name != "late-conv" {
		t.Errorf("expected late-conv, got %s", allDmails[0].Name)
	}
}

func TestConvergenceGateWithRedrain_ReloopsOnMidApprovalConvergence(t *testing.T) {
	// given: initial has convergence, and more convergence arrives mid-approval.
	// injectingApprover injects a D-Mail into the channel on first call.
	ch := make(chan *sightjack.DMail, 2)
	injectApprover := &injectingApprover{
		ch:     ch,
		inject: &sightjack.DMail{Name: "late-conv", Kind: sightjack.DMailConvergence, Description: "late convergence"},
	}
	notifier := &sightjack.NopNotifier{}
	logger := sightjack.NewLogger(io.Discard, false)

	// when: initial has convergence, approval triggers inject, re-drain catches it
	initial := []*sightjack.DMail{
		{Name: "conv-1", Kind: sightjack.DMailConvergence, Description: "initial convergence"},
	}
	allDmails, approved, err := sightjack.RunConvergenceGateWithRedrain(
		context.Background(), initial, ch, notifier, injectApprover, logger,
	)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("expected final approval")
	}
	// Both initial and late convergence should be accumulated
	if len(allDmails) != 2 {
		t.Fatalf("expected 2 accumulated dmails, got %d", len(allDmails))
	}
	// Approver should have been called twice (initial convergence + re-check for late)
	if injectApprover.callCount != 2 {
		t.Errorf("expected 2 approval calls, got %d", injectApprover.callCount)
	}
}

func TestConvergenceGate_BlockingNotifierDoesNotStall(t *testing.T) {
	// given: a notifier that blocks indefinitely + convergence d-mail.
	// Gate should not hang — notification must be non-blocking.
	dmails := []*sightjack.DMail{
		{Name: "conv-1", Kind: sightjack.DMailConvergence, Description: "convergence signal"},
	}
	notifier := &blockingNotifier{ch: make(chan struct{})}
	approver := &sightjack.AutoApprover{}
	logger := sightjack.NewLogger(io.Discard, false)

	// when: run gate with a deadline
	done := make(chan struct{})
	var approved bool
	var err error
	go func() {
		approved, err = sightjack.RunConvergenceGate(context.Background(), dmails, notifier, approver, logger)
		close(done)
	}()

	// then: should complete within 2s (not block on notifier)
	select {
	case <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !approved {
			t.Error("expected approval")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunConvergenceGate blocked on slow notifier")
	}
}

// --- test helpers ---

// blockingNotifier blocks forever on Notify — simulates a hung notify command.
type blockingNotifier struct {
	ch chan struct{}
}

func (n *blockingNotifier) Notify(_ context.Context, _, _ string) error {
	<-n.ch // block forever
	return nil
}

// injectingApprover injects a D-Mail into the channel on the first call,
// then approves on subsequent calls.
type injectingApprover struct {
	ch        chan *sightjack.DMail
	inject    *sightjack.DMail
	callCount int
}

func (a *injectingApprover) RequestApproval(_ context.Context, _ string) (bool, error) {
	a.callCount++
	if a.callCount == 1 && a.inject != nil {
		a.ch <- a.inject
	}
	return true, nil
}

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
