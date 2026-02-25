package eventsource_test

import (
	"path/filepath"
	"testing"

	sightjack "github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/eventsource"
)

func TestSessionRecorder_Record_AutoSequence(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(filepath.Join(dir, "test.jsonl"))
	recorder, err := eventsource.NewSessionRecorder(store, "session-1")
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}

	// when
	if err := recorder.Record(sightjack.EventSessionStarted, nil); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := recorder.Record(sightjack.EventScanCompleted, nil); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// then
	events, _ := store.ReadAll()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Sequence != 1 {
		t.Errorf("expected seq 1, got %d", events[0].Sequence)
	}
	if events[1].Sequence != 2 {
		t.Errorf("expected seq 2, got %d", events[1].Sequence)
	}
	if events[0].SessionID != "session-1" {
		t.Errorf("expected session-1, got %s", events[0].SessionID)
	}
}

func TestSessionRecorder_Record_WithPayload(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(filepath.Join(dir, "test.jsonl"))
	recorder, err := eventsource.NewSessionRecorder(store, "session-1")
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}

	payload := sightjack.SessionStartedPayload{
		Project:         "my-project",
		StrictnessLevel: "fog",
	}

	// when
	if err := recorder.Record(sightjack.EventSessionStarted, payload); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// then
	events, _ := store.ReadAll()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	var decoded sightjack.SessionStartedPayload
	sightjack.UnmarshalEventPayload(events[0], &decoded)
	if decoded.Project != "my-project" {
		t.Errorf("expected my-project, got %s", decoded.Project)
	}
}

func TestSessionRecorder_CorrelationID_MatchesSessionID(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(filepath.Join(dir, "test.jsonl"))
	recorder, err := eventsource.NewSessionRecorder(store, "session-42")
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}

	// when
	recorder.Record(sightjack.EventSessionStarted, nil)
	recorder.Record(sightjack.EventScanCompleted, nil)

	// then: both events should have CorrelationID == sessionID
	events, _ := store.ReadAll()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	for i, e := range events {
		if e.CorrelationID != "session-42" {
			t.Errorf("event %d: expected CorrelationID session-42, got %s", i, e.CorrelationID)
		}
	}
}

func TestSessionRecorder_CausationID_ChainsPreviousSequence(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(filepath.Join(dir, "test.jsonl"))
	recorder, err := eventsource.NewSessionRecorder(store, "session-1")
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}

	// when
	recorder.Record(sightjack.EventSessionStarted, nil)
	recorder.Record(sightjack.EventScanCompleted, nil)
	recorder.Record(sightjack.EventWavesGenerated, nil)

	// then
	events, _ := store.ReadAll()
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	// Event 1 (seq 1): no previous event, CausationID should be empty
	if events[0].CausationID != "" {
		t.Errorf("event 1: expected empty CausationID, got %s", events[0].CausationID)
	}
	// Event 2 (seq 2): previous is seq 1
	if events[1].CausationID != "1" {
		t.Errorf("event 2: expected CausationID '1', got %s", events[1].CausationID)
	}
	// Event 3 (seq 3): previous is seq 2
	if events[2].CausationID != "2" {
		t.Errorf("event 3: expected CausationID '2', got %s", events[2].CausationID)
	}
}

func TestSessionRecorder_Resume_CausationID_ContinuesChain(t *testing.T) {
	// given: store with 3 existing events
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	store := eventsource.NewFileEventStore(path)

	for i := int64(1); i <= 3; i++ {
		e, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "session-1", i, nil)
		store.Append(e)
	}

	// when: new recorder resumes from existing store, records event 4
	recorder, err := eventsource.NewSessionRecorder(store, "session-1")
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}
	recorder.Record(sightjack.EventWavesGenerated, nil)

	// then: event 4 should have CausationID == "3" (continuing chain)
	events, _ := store.ReadAll()
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}
	if events[3].CausationID != "3" {
		t.Errorf("resumed event: expected CausationID '3', got %s", events[3].CausationID)
	}
	if events[3].CorrelationID != "session-1" {
		t.Errorf("resumed event: expected CorrelationID 'session-1', got %s", events[3].CorrelationID)
	}
}

func TestSessionRecorder_ResumeFromExistingStore(t *testing.T) {
	// given: store with 3 existing events
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	store := eventsource.NewFileEventStore(path)

	for i := int64(1); i <= 3; i++ {
		e, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "session-1", i, nil)
		store.Append(e)
	}

	// when: create new recorder from same store
	recorder, err := eventsource.NewSessionRecorder(store, "session-1")
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}
	if err := recorder.Record(sightjack.EventWavesGenerated, nil); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// then: new event should have sequence 4
	events, _ := store.ReadAll()
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}
	if events[3].Sequence != 4 {
		t.Errorf("expected seq 4 (resume from 3), got %d", events[3].Sequence)
	}
}
