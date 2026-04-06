package integration_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/eventsource"
	"github.com/hironow/sightjack/internal/session"
)

func TestAggregateRecorder_ScanEvents_HaveSessionID(t *testing.T) {
	// given: aggregate + recorder wired to a temp event store
	baseDir := t.TempDir()
	sessionID := "test-scan-001"
	store := session.NewEventStore(session.SessionEventsDir(baseDir, sessionID), &domain.NopLogger{})
	recorder, err := eventsource.NewSessionRecorder(store, sessionID)
	if err != nil {
		t.Fatalf("new recorder: %v", err)
	}
	agg := domain.NewSessionAggregate()
	now := time.Now()

	// when: aggregate generates events and recorder persists them
	startEvt, err := agg.Start("my-project", "balanced", now) // nosemgrep: adr0003-otel-span-without-defer-end -- not an OTel span; domain aggregate method [permanent]
	if err != nil {
		t.Fatalf("aggregate start: %v", err)
	}
	if err := recorder.Record(startEvt); err != nil {
		t.Fatalf("record start: %v", err)
	}

	scanPayload := domain.ScanCompletedPayload{
		Clusters: []domain.ClusterState{
			{Name: "Auth", Completeness: 0.5, IssueCount: 3},
		},
		Completeness:   0.5,
		ShibitoCount:   1,
		ScanResultPath: ".siren/.run/test/scan_result.json",
		LastScanned:    now,
	}
	scanEvt, err := agg.RecordScan(scanPayload, now)
	if err != nil {
		t.Fatalf("aggregate scan: %v", err)
	}
	if err := recorder.Record(scanEvt); err != nil {
		t.Fatalf("record scan: %v", err)
	}

	// then: events are stored with SessionID, CorrelationID, and CausationID chain
	events, _, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	// Verify session_started event
	e0 := events[0]
	if e0.Type != domain.EventSessionStartedV2 {
		t.Errorf("event[0] type: got %s, want %s", e0.Type, domain.EventSessionStartedV2)
	}
	if e0.SessionID != sessionID {
		t.Errorf("event[0] SessionID: got %q, want %q", e0.SessionID, sessionID)
	}
	if e0.CorrelationID != sessionID {
		t.Errorf("event[0] CorrelationID: got %q, want %q", e0.CorrelationID, sessionID)
	}

	// Verify scan_completed event
	e1 := events[1]
	if e1.Type != domain.EventScanCompletedV2 {
		t.Errorf("event[1] type: got %s, want %s", e1.Type, domain.EventScanCompletedV2)
	}
	if e1.SessionID != sessionID {
		t.Errorf("event[1] SessionID: got %q, want %q", e1.SessionID, sessionID)
	}
	if e1.CausationID != e0.ID {
		t.Errorf("event[1] CausationID: got %q, want %q (event[0].ID)", e1.CausationID, e0.ID)
	}

	// Verify payload content is preserved through json.RawMessage pass-through
	var startPayload domain.SessionStartedPayload
	if err := json.Unmarshal(e0.Data, &startPayload); err != nil {
		t.Fatalf("unmarshal start payload: %v", err)
	}
	if startPayload.Project != "my-project" {
		t.Errorf("start payload project: got %q, want %q", startPayload.Project, "my-project")
	}

	var scanResultPayload domain.ScanCompletedPayload
	if err := json.Unmarshal(e1.Data, &scanResultPayload); err != nil {
		t.Fatalf("unmarshal scan payload: %v", err)
	}
	if scanResultPayload.Completeness != 0.5 {
		t.Errorf("scan payload completeness: got %f, want %f", scanResultPayload.Completeness, 0.5)
	}
	if len(scanResultPayload.Clusters) != 1 {
		t.Errorf("scan payload clusters: got %d, want 1", len(scanResultPayload.Clusters))
	}
}
