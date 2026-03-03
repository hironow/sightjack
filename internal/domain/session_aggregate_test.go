package domain_test

import (
	"testing"
	"time"

	sightjack "github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/domain"
)

func TestSessionAggregate_Start(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.Start("my-project", "standard", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventSessionStarted {
		t.Fatalf("expected session_started, got %s", ev.Type)
	}
}

func TestSessionAggregate_RecordScan(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()
	payload := domain.ScanCompletedPayload{
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
	if ev.Type != domain.EventScanCompleted {
		t.Fatalf("expected scan_completed, got %s", ev.Type)
	}
}

func TestSessionAggregate_UpdateCompleteness(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.UpdateCompleteness("auth", 0.6, 0.5, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventCompletenessUpdated {
		t.Fatalf("expected completeness_updated, got %s", ev.Type)
	}
}

func TestSessionAggregate_Resume(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.Resume("original-session-123", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventSessionResumed {
		t.Fatalf("expected session_resumed, got %s", ev.Type)
	}
}

func TestSessionAggregate_Rescan(t *testing.T) {
	// given
	agg := domain.NewSessionAggregate()

	// when
	ev, err := agg.Rescan("original-session-456", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventSessionRescanned {
		t.Fatalf("expected session_rescanned, got %s", ev.Type)
	}
}
