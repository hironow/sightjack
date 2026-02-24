package sightjack

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// ProjectState replays a sequence of events to produce a SessionState.
// Unknown event types are silently skipped.
// Returns a zero-value SessionState for nil/empty input.
func ProjectState(events []Event) *SessionState {
	state := &SessionState{}
	for _, e := range events {
		applyEvent(state, e)
	}
	return state
}

// LoadState reads all events from the store and projects them into a SessionState.
// Returns an error if the store is empty (no events to replay).
func LoadState(store EventStore) (*SessionState, error) {
	events, err := store.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("load state read events: %w", err)
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("load state: no events in store")
	}
	return ProjectState(events), nil
}

// applyEvent mutates state according to the event type.
func applyEvent(state *SessionState, e Event) {
	switch e.Type {
	case EventSessionStarted:
		var p SessionStartedPayload
		if unmarshalPayload(e, &p) {
			state.Version = StateFormatVersion
			state.SessionID = e.SessionID
			state.Project = p.Project
			state.StrictnessLevel = p.StrictnessLevel
		}

	case EventScanCompleted:
		var p ScanCompletedPayload
		if unmarshalPayload(e, &p) {
			state.Clusters = p.Clusters
			state.Completeness = p.Completeness
			state.ShibitoCount = p.ShibitoCount
			state.ScanResultPath = p.ScanResultPath
			state.LastScanned = p.LastScanned
		}

	case EventWavesGenerated:
		var p WavesGeneratedPayload
		if unmarshalPayload(e, &p) {
			state.Waves = append(state.Waves, p.Waves...)
		}

	case EventWaveCompleted:
		var p WaveCompletedPayload
		if unmarshalPayload(e, &p) {
			key := p.ClusterName + ":" + p.WaveID
			for i, w := range state.Waves {
				if w.ClusterName+":"+w.ID == key {
					state.Waves[i].Status = "completed"
					break
				}
			}
		}

	case EventCompletenessUpdated:
		var p CompletenessUpdatedPayload
		if unmarshalPayload(e, &p) {
			state.Completeness = p.OverallCompleteness
			for i, c := range state.Clusters {
				if c.Name == p.ClusterName {
					state.Clusters[i].Completeness = p.ClusterCompleteness
					break
				}
			}
		}

	case EventWavesUnlocked:
		var p WavesUnlockedPayload
		if unmarshalPayload(e, &p) {
			unlocked := make(map[string]bool, len(p.UnlockedWaveIDs))
			for _, id := range p.UnlockedWaveIDs {
				unlocked[id] = true
			}
			for i, w := range state.Waves {
				key := w.ClusterName + ":" + w.ID
				if unlocked[key] && w.Status == "locked" {
					state.Waves[i].Status = "available"
				}
			}
		}

	case EventNextGenWavesAdded:
		var p NextGenWavesAddedPayload
		if unmarshalPayload(e, &p) {
			state.Waves = append(state.Waves, p.Waves...)
		}

	case EventWaveModified:
		var p WaveModifiedPayload
		if unmarshalPayload(e, &p) {
			key := p.ClusterName + ":" + p.WaveID
			for i, w := range state.Waves {
				if w.ClusterName+":"+w.ID == key {
					state.Waves[i] = p.UpdatedWave
					break
				}
			}
		}

	case EventADRGenerated:
		state.ADRCount++

	case EventWaveApproved, EventWaveRejected, EventWaveApplied,
		EventSpecificationSent, EventReportSent,
		EventReadyLabelsApplied, EventSessionResumed, EventSessionRescanned:
		// Audit-only events: no state mutation needed.

	default:
		// Unknown event type: silently skip for forward compatibility.
	}
}

// LoadLatestState finds the most recent event store in .siren/events/ and
// replays its events to produce a SessionState.
// Returns the state, the sessionID, and any error.
func LoadLatestState(baseDir string) (*SessionState, string, error) {
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

// unmarshalPayload is a helper that returns false on unmarshal failure.
func unmarshalPayload(e Event, target any) bool {
	return UnmarshalEventPayload(e, target) == nil
}
