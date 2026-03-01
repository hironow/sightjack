package eventsource

import "path/filepath"

// EventsDir returns the path to the events directory under stateDir.
// stateDir is the tool's state directory (e.g. ".siren/"), not the repo root.
func EventsDir(stateDir string) string {
	return filepath.Join(stateDir, "events")
}

// EventStorePath returns the directory path for a given session's event store.
// stateDir is the tool's state directory (e.g. ".siren/"), not the repo root.
func EventStorePath(stateDir, sessionID string) string {
	return filepath.Join(EventsDir(stateDir), sessionID)
}
