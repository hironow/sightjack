package sightjack

import (
	"fmt"
	"sync"
)

// SessionRecorder wraps an EventStore with automatic sequencing.
// It is safe for concurrent use within a single process.
type SessionRecorder struct {
	store     EventStore
	sessionID string
	seq       int64
	mu        sync.Mutex
}

// NewSessionRecorder creates a SessionRecorder that resumes from the store's last sequence.
func NewSessionRecorder(store EventStore, sessionID string) *SessionRecorder {
	lastSeq, _ := store.LastSequence()
	return &SessionRecorder{
		store:     store,
		sessionID: sessionID,
		seq:       lastSeq,
	}
}

// Record creates and appends an event with the next sequence number.
// If the receiver is nil, Record is a no-op (returns nil).
func (r *SessionRecorder) Record(eventType EventType, payload any) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seq++
	event, err := NewEvent(eventType, r.sessionID, r.seq, payload)
	if err != nil {
		return fmt.Errorf("recorder new event: %w", err)
	}
	return r.store.Append(event)
}
