package eventsource

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
)

// loaderLogger is a package-level NopLogger for loader functions that read
// event stores internally. Corrupt-line warnings are suppressed here because
// these are batch-read paths; production code that constructs stores explicitly
// should pass a real logger via NewFileEventStore.
var loaderLogger domain.Logger = &domain.NopLogger{}

// LoadState reads all events from the store and projects them into a SessionState.
// Returns an error if the store is empty (no events to replay).
func LoadState(store *FileEventStore) (*domain.SessionState, error) {
	events, _, err := store.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("load state read events: %w", err)
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("load state: no events in store")
	}
	return domain.ProjectState(events), nil
}

// LoadLatestState finds the most recent event store in events/ and
// replays its events to produce a SessionState.
// stateDir is the tool's state directory (e.g. ".siren/"), not the repo root.
// Returns the state, the sessionID, and any error.
func LoadLatestState(stateDir string) (*domain.SessionState, string, error) {
	return loadLatestStateMatching(stateDir, nil)
}

// LoadLatestResumableState finds the most recent event store whose projected
// state satisfies the given predicate. This allows callers to skip over
// non-resumable sessions (e.g. scan-only) and find an older interactive session.
// stateDir is the tool's state directory (e.g. ".siren/"), not the repo root.
func LoadLatestResumableState(stateDir string, match func(*domain.SessionState) bool) (*domain.SessionState, string, error) {
	return loadLatestStateMatching(stateDir, match)
}

type eventCandidate struct {
	name    string
	modTime int64
}

// sortedEventCandidates returns session directories (or legacy .jsonl files)
// in eventsDir sorted by modtime descending.
func sortedEventCandidates(eventsDir string) ([]eventCandidate, error) {
	entries, err := os.ReadDir(eventsDir)
	if err != nil {
		return nil, err
	}
	var candidates []eventCandidate
	for _, e := range entries {
		// Session directories contain daily JSONL files.
		if e.IsDir() {
			info, infoErr := e.Info()
			if infoErr != nil {
				continue
			}
			candidates = append(candidates, eventCandidate{name: e.Name(), modTime: info.ModTime().UnixNano()})
			continue
		}
		// Legacy: single .jsonl files (backwards compat during migration).
		if strings.HasSuffix(e.Name(), ".jsonl") {
			info, infoErr := e.Info()
			if infoErr != nil {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".jsonl")
			candidates = append(candidates, eventCandidate{name: name, modTime: info.ModTime().UnixNano()})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime > candidates[j].modTime
	})
	return candidates, nil
}

// LoadAllResult holds statistics from loading events across sessions.
type LoadAllResult struct {
	SessionsLoaded int
	SessionsFailed int
}

// LoadAllEventsAcrossSessions aggregates events from all session stores under
// events/. stateDir is the tool's state directory (e.g. ".siren/"), not the repo root.
// Returns nil, LoadAllResult{}, nil when the events directory does not exist.
func LoadAllEventsAcrossSessions(stateDir string) ([]domain.Event, LoadAllResult, error) {
	eventsDir := EventsDir(stateDir)
	candidates, err := sortedEventCandidates(eventsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, LoadAllResult{}, nil
		}
		return nil, LoadAllResult{}, fmt.Errorf("load all events: %w", err)
	}
	var all []domain.Event
	var result LoadAllResult
	for _, c := range candidates {
		store := NewFileEventStore(EventStorePath(stateDir, c.name), loaderLogger)
		events, _, loadErr := store.LoadAll()
		if loadErr != nil || len(events) == 0 {
			result.SessionsFailed++
			continue
		}
		result.SessionsLoaded++
		all = append(all, events...)
	}
	return all, result, nil
}

// loadLatestStateMatching iterates event stores by modtime descending and
// returns the first state that satisfies match (nil match accepts any).
func loadLatestStateMatching(stateDir string, match func(*domain.SessionState) bool) (*domain.SessionState, string, error) {
	eventsDir := EventsDir(stateDir)
	candidates, err := sortedEventCandidates(eventsDir)
	if err != nil {
		return nil, "", fmt.Errorf("load latest state: %w", err)
	}
	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("load latest state: no event files in %s", eventsDir)
	}

	for _, c := range candidates {
		sessionID := c.name
		store := NewFileEventStore(EventStorePath(stateDir, sessionID), loaderLogger)
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
