package eventsource

import (
	"fmt"
	"sync"

	sightjack "github.com/hironow/sightjack"
)

// SessionRecorder wraps an EventStore with automatic sequencing.
// It is safe for concurrent use within a single process.
type SessionRecorder struct {
	store     sightjack.EventStore
	sessionID string
	seq       int64
	mu        sync.Mutex
}

// NewSessionRecorder creates a SessionRecorder that resumes from the store's last sequence.
func NewSessionRecorder(store sightjack.EventStore, sessionID string) *SessionRecorder {
	lastSeq, _ := store.LastSequence()
	return &SessionRecorder{
		store:     store,
		sessionID: sessionID,
		seq:       lastSeq,
	}
}

// Record creates and appends an event with the next sequence number.
// CorrelationID is set to the session ID to link all events in a session.
// CausationID is set to the previous event's sequence number (empty for the first event).
func (r *SessionRecorder) Record(eventType sightjack.EventType, payload any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	prevSeq := r.seq
	r.seq++
	event, err := sightjack.NewEvent(eventType, r.sessionID, r.seq, payload)
	if err != nil {
		return fmt.Errorf("recorder new event: %w", err)
	}
	event.CorrelationID = r.sessionID
	if prevSeq > 0 {
		event.CausationID = fmt.Sprintf("%d", prevSeq)
	}
	return r.store.Append(event)
}
