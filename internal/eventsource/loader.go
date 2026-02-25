package eventsource

import (
	"fmt"
	"os"
	"sort"
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
	return loadLatestStateMatching(baseDir, nil)
}

// LoadLatestResumableState finds the most recent event store whose projected
// state satisfies the given predicate. This allows callers to skip over
// non-resumable sessions (e.g. scan-only) and find an older interactive session.
func LoadLatestResumableState(baseDir string, match func(*sightjack.SessionState) bool) (*sightjack.SessionState, string, error) {
	return loadLatestStateMatching(baseDir, match)
}

type eventCandidate struct {
	name    string
	modTime int64
}

// sortedEventCandidates returns .jsonl files in eventsDir sorted by modtime descending.
func sortedEventCandidates(eventsDir string) ([]eventCandidate, error) {
	entries, err := os.ReadDir(eventsDir)
	if err != nil {
		return nil, err
	}
	var candidates []eventCandidate
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			info, infoErr := e.Info()
			if infoErr != nil {
				continue
			}
			candidates = append(candidates, eventCandidate{name: e.Name(), modTime: info.ModTime().UnixNano()})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime > candidates[j].modTime
	})
	return candidates, nil
}

// loadLatestStateMatching iterates event files by modtime descending and
// returns the first state that satisfies match (nil match accepts any).
func loadLatestStateMatching(baseDir string, match func(*sightjack.SessionState) bool) (*sightjack.SessionState, string, error) {
	eventsDir := EventsDir(baseDir)
	candidates, err := sortedEventCandidates(eventsDir)
	if err != nil {
		return nil, "", fmt.Errorf("load latest state: %w", err)
	}
	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("load latest state: no event files in %s", eventsDir)
	}

	for _, c := range candidates {
		sessionID := strings.TrimSuffix(c.name, ".jsonl")
		store := NewFileEventStore(EventStorePath(baseDir, sessionID))
		state, loadErr := LoadState(store)
		if loadErr != nil {
			continue
		}
		if match == nil || match(state) {
			return state, sessionID, nil
		}
	}
	return nil, "", fmt.Errorf("load latest state: no valid event data in %s", eventsDir)
}

