package eventsource

import (
	"path/filepath"

	sightjack "github.com/hironow/sightjack"
)

// EventsDir returns the path to the events directory under .siren/.
func EventsDir(baseDir string) string {
	return filepath.Join(baseDir, sightjack.StateDir, "events")
}

// EventStorePath returns the JSONL file path for a given session.
func EventStorePath(baseDir, sessionID string) string {
	return filepath.Join(EventsDir(baseDir), sessionID+".jsonl")
}
