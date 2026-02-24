package sightjack_test

import (
	"path/filepath"
	"testing"

	"github.com/hironow/sightjack"
)

func TestSessionRecorder_Record_AutoSequence(t *testing.T) {
	// given
	dir := t.TempDir()
	store := sightjack.NewFileEventStore(filepath.Join(dir, "test.jsonl"))
	recorder := sightjack.NewSessionRecorder(store, "session-1")

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
	store := sightjack.NewFileEventStore(filepath.Join(dir, "test.jsonl"))
	recorder := sightjack.NewSessionRecorder(store, "session-1")

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

func TestSessionRecorder_ResumeFromExistingStore(t *testing.T) {
	// given: store with 3 existing events
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	store := sightjack.NewFileEventStore(path)

	for i := int64(1); i <= 3; i++ {
		e, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "session-1", i, nil)
		store.Append(e)
	}

	// when: create new recorder from same store
	recorder := sightjack.NewSessionRecorder(store, "session-1")
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

func TestSessionRecorder_NilRecorder_NoOp(t *testing.T) {
	// given: nil recorder (used when recorder is optional)
	var recorder *sightjack.SessionRecorder

	// when/then: should not panic
	err := recorder.Record(sightjack.EventSessionStarted, nil)
	if err != nil {
		t.Errorf("expected nil error from nil recorder, got: %v", err)
	}
}
