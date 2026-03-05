package session

import (
	"path/filepath"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/eventsource"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// stateDir converts a repo-root baseDir to the tool's state directory.
func stateDir(baseDir string) string {
	return filepath.Join(baseDir, domain.StateDir)
}

// SessionEventsDir returns the events directory for a specific session.
// Callers use this to compute the stateDir parameter for NewEventStore.
func SessionEventsDir(baseDir, sessionID string) string {
	return eventsource.EventStorePath(stateDir(baseDir), sessionID)
}

// NewEventStore creates an event store rooted at stateDir.
// eventsource is the interface-adapter layer for event persistence (clean-architecture).
// cmd layer should use this instead of importing eventsource directly (ADR S0008).
func NewEventStore(stateDir string, logger domain.Logger) port.EventStore {
	return eventsource.NewFileEventStore(stateDir, logger)
}

// NewSessionRecorder creates a recorder for the given session.
func NewSessionRecorder(stateDir, sessionID string, logger domain.Logger) (port.Recorder, error) {
	store := eventsource.NewFileEventStore(stateDir, logger)
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
