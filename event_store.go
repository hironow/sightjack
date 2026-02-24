package sightjack

// EventStore is the interface for an append-only event log.
type EventStore interface {
	Append(events ...Event) error
	ReadAll() ([]Event, error)
	ReadSince(afterSeq int64) ([]Event, error)
	LastSequence() (int64, error)
}
