package eventsource

import (
	"context"
	"fmt"
	"sync"

	"github.com/hironow/sightjack/internal/domain"
)

// eventStore is the package-local interface for SessionRecorder's store
// dependency. Kept unexported to avoid importing port from eventsource
// (prohibited by semgrep Rule 5). FileEventStore satisfies this via duck typing.
type eventStore interface {
	Append(ctx context.Context, events ...domain.Event) (domain.AppendResult, error)
	LoadAll(ctx context.Context) ([]domain.Event, domain.LoadResult, error)
}

// SessionRecorder wraps a FileEventStore with automatic SessionID assignment
// and optional global SeqNr allocation. Safe for concurrent use.
type SessionRecorder struct {
	store      eventStore
	seqCounter *SeqCounter // nil = no SeqNr assignment (pre-cutover)
	sessionID  string
	prevID     string
	logger     domain.Logger
	mu         sync.Mutex
}

// NewSessionRecorder creates a SessionRecorder for the given session.
// logger may be nil; when non-nil, corruption detected during LoadAll is
// surfaced as a warning rather than silently discarded.
func NewSessionRecorder(ctx context.Context, store eventStore, sessionID string, logger domain.Logger) (*SessionRecorder, error) {
	events, lr, err := store.LoadAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("new session recorder: %w", err)
	}
	if lr.CorruptLineCount > 0 && logger != nil {
		logger.Warn("event store corruption: %d corrupt lines in %d files (session %s)", lr.CorruptLineCount, lr.FileCount, sessionID)
	}
	var prevID string
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].CorrelationID == sessionID {
			prevID = events[i].ID
			break
		}
	}
	return &SessionRecorder{
		store:     store,
		sessionID: sessionID,
		prevID:    prevID,
		logger:    logger,
	}, nil
}

// SetSeqCounter attaches a SeqCounter for global SeqNr allocation.
// When set, Record() assigns a monotonic SeqNr to each event before persistence.
func (r *SessionRecorder) SetSeqCounter(sc *SeqCounter) {
	r.seqCounter = sc
}

// Record appends a pre-built event, enriching it with session metadata.
// If a SeqCounter is attached, assigns a globally monotonic SeqNr.
func (r *SessionRecorder) Record(ctx context.Context, ev domain.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ev.SessionID = r.sessionID
	ev.CorrelationID = r.sessionID
	if r.prevID != "" {
		ev.CausationID = r.prevID
	}
	if r.seqCounter != nil {
		seq, err := r.seqCounter.AllocSeqNr(ctx)
		if err != nil {
			return fmt.Errorf("alloc seq nr: %w", err)
		}
		ev.SeqNr = seq
	}
	if _, err := r.store.Append(ctx, ev); err != nil {
		return err
	}
	r.prevID = ev.ID
	return nil
}
