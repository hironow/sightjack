package session

import (
	"path/filepath"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/eventsource"
)

// stateDir converts a repo-root baseDir to the tool's state directory.
func stateDir(baseDir string) string {
	return filepath.Join(baseDir, domain.StateDir)
}

// NewEventStore creates an event store for the given session.
// cmd layer should use this instead of importing eventsource directly (ADR S0008).
func NewEventStore(baseDir, sessionID string) domain.EventStore {
	return eventsource.NewFileEventStore(eventsource.EventStorePath(stateDir(baseDir), sessionID))
}

// NewSessionRecorder creates a recorder for the given session.
func NewSessionRecorder(store domain.EventStore, sessionID string) (domain.Recorder, error) {
	return eventsource.NewSessionRecorder(store, sessionID)
}

// EventStorePath returns the filesystem path for a session's event store.
func EventStorePath(baseDir, sessionID string) string {
	return eventsource.EventStorePath(stateDir(baseDir), sessionID)
}

// LoadLatestState loads the most recent session state from event data.
func LoadLatestState(baseDir string) (*domain.SessionState, string, error) {
	return eventsource.LoadLatestState(stateDir(baseDir))
}

// LoadLatestResumableState loads the latest session state that matches the predicate.
func LoadLatestResumableState(baseDir string, match func(*domain.SessionState) bool) (*domain.SessionState, string, error) {
	return eventsource.LoadLatestResumableState(stateDir(baseDir), match)
}

// LoadAllEvents aggregates events from all session stores.
func LoadAllEvents(baseDir string) ([]domain.Event, error) {
	return eventsource.LoadAllEventsAcrossSessions(stateDir(baseDir))
}

// ListExpiredEventFiles returns event files older than the retention threshold.
func ListExpiredEventFiles(baseDir string, days int) ([]string, error) {
	return eventsource.ListExpiredEventFiles(stateDir(baseDir), days)
}

// PruneEventFiles deletes the specified event files.
func PruneEventFiles(baseDir string, files []string) ([]string, error) {
	return eventsource.PruneEventFiles(stateDir(baseDir), files)
}
