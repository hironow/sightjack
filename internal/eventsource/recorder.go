package eventsource

import (
	"fmt"
	"sync"
	"time"

	sightjack "github.com/hironow/sightjack"
)

// SessionRecorder wraps an EventStore with automatic SessionID assignment.
// It is safe for concurrent use within a single process.
type SessionRecorder struct {
	store     sightjack.EventStore
	sessionID string
	prevID    string // ID of the previous event for CausationID chaining
	mu        sync.Mutex
}

// NewSessionRecorder creates a SessionRecorder for the given session.
func NewSessionRecorder(store sightjack.EventStore, sessionID string) (*SessionRecorder, error) {
	// Load existing events to resume CausationID chain.
	events, err := store.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("new session recorder: %w", err)
	}
	var prevID string
	if len(events) > 0 {
		prevID = events[len(events)-1].ID
	}
	return &SessionRecorder{
		store:     store,
		sessionID: sessionID,
		prevID:    prevID,
	}, nil
}

// Record creates and appends an event with a new UUID.
// SessionID and CorrelationID are set to the session ID.
// CausationID is set to the previous event's ID.
func (r *SessionRecorder) Record(eventType sightjack.EventType, payload any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	event, err := sightjack.NewEvent(eventType, payload, time.Now())
	if err != nil {
		return fmt.Errorf("recorder new event: %w", err)
	}
	event.SessionID = r.sessionID
	event.CorrelationID = r.sessionID
	if r.prevID != "" {
		event.CausationID = r.prevID
	}
	if err := r.store.Append(event); err != nil {
		return err
	}
	r.prevID = event.ID
	return nil
}
