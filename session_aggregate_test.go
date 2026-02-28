package sightjack_test

import (
	"testing"
	"time"

	"github.com/hironow/sightjack"
)

func TestSessionAggregate_Start(t *testing.T) {
	// given
	agg := sightjack.NewSessionAggregate()

	// when
	ev, err := agg.Start("my-project", "standard", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != sightjack.EventSessionStarted {
		t.Fatalf("expected session_started, got %s", ev.Type)
	}
}

func TestSessionAggregate_RecordScan(t *testing.T) {
	// given
	agg := sightjack.NewSessionAggregate()
	payload := sightjack.ScanCompletedPayload{
		Clusters:       []sightjack.ClusterState{{Name: "auth", Completeness: 0.3}},
		Completeness:   0.3,
		ShibitoCount:   5,
		ScanResultPath: "scan/result.json",
		LastScanned:    time.Now().UTC(),
	}

	// when
	ev, err := agg.RecordScan(payload, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != sightjack.EventScanCompleted {
		t.Fatalf("expected scan_completed, got %s", ev.Type)
	}
}

func TestSessionAggregate_UpdateCompleteness(t *testing.T) {
	// given
	agg := sightjack.NewSessionAggregate()

	// when
	ev, err := agg.UpdateCompleteness("auth", 0.6, 0.5, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != sightjack.EventCompletenessUpdated {
		t.Fatalf("expected completeness_updated, got %s", ev.Type)
	}
}

func TestSessionAggregate_Resume(t *testing.T) {
	// given
	agg := sightjack.NewSessionAggregate()

	// when
	ev, err := agg.Resume("original-session-123", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != sightjack.EventSessionResumed {
		t.Fatalf("expected session_resumed, got %s", ev.Type)
	}
}

func TestSessionAggregate_Rescan(t *testing.T) {
	// given
	agg := sightjack.NewSessionAggregate()

	// when
	ev, err := agg.Rescan("original-session-456", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != sightjack.EventSessionRescanned {
		t.Fatalf("expected session_rescanned, got %s", ev.Type)
	}
}
