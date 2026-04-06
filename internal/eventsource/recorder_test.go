package eventsource_test

import (
	"errors"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/eventsource"
)

// failOnceStore wraps a real FileEventStore and fails the first Append call,
// then delegates to the real store for subsequent calls.
type failOnceStore struct {
	real   *eventsource.FileEventStore
	failed bool
}

func (s *failOnceStore) Append(events ...domain.Event) (domain.AppendResult, error) {
	if !s.failed {
		s.failed = true
		return domain.AppendResult{}, errors.New("simulated I/O error")
	}
	return s.real.Append(events...)
}

func (s *failOnceStore) LoadAll() ([]domain.Event, domain.LoadResult, error) {
	return s.real.LoadAll()
}
func (s *failOnceStore) LoadSince(after time.Time) ([]domain.Event, domain.LoadResult, error) {
	return s.real.LoadSince(after)
}

// mustEvent is a test helper that creates a domain.Event and fails on error.
func mustEvent(t *testing.T, eventType domain.EventType, payload any) domain.Event {
	t.Helper()
	ev, err := domain.NewEvent(eventType, payload, time.Now())
	if err != nil {
		t.Fatalf("NewEvent(%s): %v", eventType, err)
	}
	return ev
}

func TestSessionRecorder_Record_AutoUUID(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	recorder, err := eventsource.NewSessionRecorder(store, "session-1")
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}

	// when
	if err := recorder.Record(mustEvent(t, domain.EventSessionStarted, struct{}{})); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := recorder.Record(mustEvent(t, domain.EventScanCompleted, struct{}{})); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// then
	events, _, _ := store.LoadAll()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].ID == "" {
		t.Error("expected non-empty UUID ID on first event")
	}
	if events[1].ID == "" {
		t.Error("expected non-empty UUID ID on second event")
	}
	if events[0].ID == events[1].ID {
		t.Error("expected unique IDs for different events")
	}
	if events[0].SessionID != "session-1" {
		t.Errorf("expected session-1, got %s", events[0].SessionID)
	}
}

func TestSessionRecorder_Record_WithPayload(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	recorder, err := eventsource.NewSessionRecorder(store, "session-1")
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}

	payload := domain.SessionStartedPayload{
		Project:         "my-project",
		StrictnessLevel: "fog",
	}

	// when
	if err := recorder.Record(mustEvent(t, domain.EventSessionStarted, payload)); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// then
	events, _, _ := store.LoadAll()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	var decoded domain.SessionStartedPayload
	domain.UnmarshalEventPayload(events[0], &decoded)
	if decoded.Project != "my-project" {
		t.Errorf("expected my-project, got %s", decoded.Project)
	}
}

func TestSessionRecorder_CorrelationID_MatchesSessionID(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	recorder, err := eventsource.NewSessionRecorder(store, "session-42")
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}

	// when
	recorder.Record(mustEvent(t, domain.EventSessionStarted, struct{}{}))
	recorder.Record(mustEvent(t, domain.EventScanCompleted, struct{}{}))

	// then: both events should have CorrelationID == sessionID
	events, _, _ := store.LoadAll()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	for i, e := range events {
		if e.CorrelationID != "session-42" {
			t.Errorf("event %d: expected CorrelationID session-42, got %s", i, e.CorrelationID)
		}
	}
}

func TestSessionRecorder_CausationID_ChainsPreviousEvent(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	recorder, err := eventsource.NewSessionRecorder(store, "session-1")
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}

	// when
	recorder.Record(mustEvent(t, domain.EventSessionStarted, struct{}{}))
	recorder.Record(mustEvent(t, domain.EventScanCompleted, struct{}{}))
	recorder.Record(mustEvent(t, domain.EventWavesGenerated, struct{}{}))

	// then
	events, _, _ := store.LoadAll()
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	// Event 1: no previous event, CausationID should be empty
	if events[0].CausationID != "" {
		t.Errorf("event 1: expected empty CausationID, got %s", events[0].CausationID)
	}
	// Event 2: previous is event 1
	if events[1].CausationID != events[0].ID {
		t.Errorf("event 2: expected CausationID %s, got %s", events[0].ID, events[1].CausationID)
	}
	// Event 3: previous is event 2
	if events[2].CausationID != events[1].ID {
		t.Errorf("event 3: expected CausationID %s, got %s", events[1].ID, events[2].CausationID)
	}
}

func TestSessionRecorder_ResumeFromExistingStore(t *testing.T) {
	// given: store with existing events
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})

	rec1, _ := eventsource.NewSessionRecorder(store, "session-1")
	rec1.Record(mustEvent(t, domain.EventSessionStarted, struct{}{}))
	rec1.Record(mustEvent(t, domain.EventScanCompleted, struct{}{}))
	rec1.Record(mustEvent(t, domain.EventWavesGenerated, struct{}{}))

	events1, _, _ := store.LoadAll()
	lastID := events1[len(events1)-1].ID

	// when: create new recorder from same store
	recorder, err := eventsource.NewSessionRecorder(store, "session-1")
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}
	if err := recorder.Record(mustEvent(t, domain.EventWaveApproved, struct{}{})); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// then: new event should chain from last existing event
	events, _, _ := store.LoadAll()
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}
	if events[3].CausationID != lastID {
		t.Errorf("resumed event: expected CausationID %s, got %s", lastID, events[3].CausationID)
	}
	if events[3].CorrelationID != "session-1" {
		t.Errorf("resumed event: expected CorrelationID 'session-1', got %s", events[3].CorrelationID)
	}
}

func TestSessionRecorder_Record_RecoverAfterAppendFailure(t *testing.T) {
	// given: a store that fails the first Append, then succeeds
	dir := t.TempDir()
	real := eventsource.NewFileEventStore(dir, &domain.NopLogger{})
	fos := &failOnceStore{real: real}
	recorder, err := eventsource.NewSessionRecorder(fos, "session-1")
	if err != nil {
		t.Fatalf("NewSessionRecorder: %v", err)
	}

	// when: first Record fails
	err1 := recorder.Record(mustEvent(t, domain.EventSessionStarted, struct{}{}))
	if err1 == nil {
		t.Fatal("expected error on first Record")
	}

	// when: second Record should succeed
	err2 := recorder.Record(mustEvent(t, domain.EventSessionStarted, struct{}{}))
	if err2 != nil {
		t.Fatalf("expected second Record to succeed, got: %v", err2)
	}

	// then: the store should have exactly 1 event
	events, _, _ := real.LoadAll()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ID == "" {
		t.Error("expected non-empty UUID ID")
	}
}
