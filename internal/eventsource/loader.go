package eventsource

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	sightjack "github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/domain"
)

// LoadState reads all events from the store and projects them into a SessionState.
// Returns an error if the store is empty (no events to replay).
func LoadState(store sightjack.EventStore) (*sightjack.SessionState, error) {
	events, err := store.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("load state read events: %w", err)
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("load state: no events in store")
	}
	return domain.ProjectState(events), nil
}

// LoadLatestState finds the most recent event store in .siren/events/ and
// replays its events to produce a SessionState.
// Returns the state, the sessionID, and any error.
func LoadLatestState(baseDir string) (*sightjack.SessionState, string, error) {
	eventsDir := EventsDir(baseDir)
	entries, err := os.ReadDir(eventsDir)
	if err != nil {
		return nil, "", fmt.Errorf("load latest state: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		return nil, "", fmt.Errorf("load latest state: no event files in %s", eventsDir)
	}

	// Sort by embedded timestamp descending so newest session is tried first.
	sort.Slice(files, func(i, j int) bool {
		return eventFileTimestamp(files[i]) > eventFileTimestamp(files[j])
	})

	for _, f := range files {
		sessionID := strings.TrimSuffix(f, ".jsonl")
		store := NewFileEventStore(EventStorePath(baseDir, sessionID))
		state, loadErr := LoadState(store)
		if loadErr == nil {
			return state, sessionID, nil
		}
	}
	return nil, "", fmt.Errorf("load latest state: no valid event data in %s", eventsDir)
}

// eventFileTimestamp extracts the Unix-milli timestamp from a session JSONL filename.
// Format: "{prefix}-{unixmilli}-{pid}.jsonl". Returns 0 for unparseable names.
func eventFileTimestamp(name string) int64 {
	name = strings.TrimSuffix(name, ".jsonl")
	parts := strings.SplitN(name, "-", 3)
	if len(parts) < 2 {
		return 0
	}
	ts, _ := strconv.ParseInt(parts[1], 10, 64)
	return ts
}
