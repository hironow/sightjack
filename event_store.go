package sightjack

import "path/filepath"

// EventStore is the interface for an append-only event log.
type EventStore interface {
	Append(events ...Event) error
	ReadAll() ([]Event, error)
	ReadSince(afterSeq int64) ([]Event, error)
	LastSequence() (int64, error)
}

// EventsDir returns the path to the events directory under .siren/.
func EventsDir(baseDir string) string {
	return filepath.Join(baseDir, StateDir, "events")
}

// EventStorePath returns the JSONL file path for a given session.
func EventStorePath(baseDir, sessionID string) string {
	return filepath.Join(EventsDir(baseDir), sessionID+".jsonl")
}
