package session

import (
	sightjack "github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/eventsource"
)

// NewEventStore creates an event store for the given session.
// cmd layer should use this instead of importing eventsource directly (ADR S0008).
func NewEventStore(baseDir, sessionID string) sightjack.EventStore {
	return eventsource.NewFileEventStore(eventsource.EventStorePath(baseDir, sessionID))
}

// NewSessionRecorder creates a recorder for the given session.
func NewSessionRecorder(store sightjack.EventStore, sessionID string) (sightjack.Recorder, error) {
	return eventsource.NewSessionRecorder(store, sessionID)
}

// EventStorePath returns the filesystem path for a session's event store.
func EventStorePath(baseDir, sessionID string) string {
	return eventsource.EventStorePath(baseDir, sessionID)
}

// LoadLatestState loads the most recent session state from event data.
func LoadLatestState(baseDir string) (*sightjack.SessionState, string, error) {
	return eventsource.LoadLatestState(baseDir)
}

// LoadLatestResumableState loads the latest session state that matches the predicate.
func LoadLatestResumableState(baseDir string, match func(*sightjack.SessionState) bool) (*sightjack.SessionState, string, error) {
	return eventsource.LoadLatestResumableState(baseDir, match)
}

// LoadAllEvents aggregates events from all session stores.
func LoadAllEvents(baseDir string) ([]sightjack.Event, error) {
	return eventsource.LoadAllEventsAcrossSessions(baseDir)
}

// ListExpiredEventFiles returns event files older than the retention threshold.
func ListExpiredEventFiles(baseDir string, days int) ([]string, error) {
	return eventsource.ListExpiredEventFiles(baseDir, days)
}

// PruneEventFiles deletes the specified event files.
func PruneEventFiles(baseDir string, files []string) ([]string, error) {
	return eventsource.PruneEventFiles(baseDir, files)
}
