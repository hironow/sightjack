package eventsource

import (
	"fmt"
	"sync"

	"github.com/hironow/sightjack/internal/domain"
)

// eventStore is the package-local interface for SessionRecorder's store
// dependency. Kept unexported to avoid importing port from eventsource
// (prohibited by semgrep Rule 5). FileEventStore satisfies this via duck typing.
type eventStore interface {
	Append(events ...domain.Event) error
	LoadAll() ([]domain.Event, error)
}

// SessionRecorder wraps a FileEventStore with automatic SessionID assignment.
// It is safe for concurrent use within a single process.
type SessionRecorder struct {
	store     eventStore
	sessionID string
	prevID    string // ID of the previous event for CausationID chaining
	mu        sync.Mutex
}

// NewSessionRecorder creates a SessionRecorder for the given session.
func NewSessionRecorder(store eventStore, sessionID string) (*SessionRecorder, error) {
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

// Record appends a pre-built event, enriching it with session metadata.
// SessionID and CorrelationID are set to the session ID.
// CausationID is set to the previous event's ID.
func (r *SessionRecorder) Record(ev domain.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ev.SessionID = r.sessionID
	ev.CorrelationID = r.sessionID
	if r.prevID != "" {
		ev.CausationID = r.prevID
	}
	if err := r.store.Append(ev); err != nil {
		return err
	}
	r.prevID = ev.ID
	return nil
}
