package eventsource

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	sightjack "github.com/hironow/sightjack"
)

// ProjectState replays a sequence of events to produce a SessionState.
// Unknown event types are silently skipped.
// Returns a zero-value SessionState for nil/empty input.
func ProjectState(events []sightjack.Event) *sightjack.SessionState {
	state := &sightjack.SessionState{}
	for _, e := range events {
		applyEvent(state, e)
	}
	return state
}

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
	return ProjectState(events), nil
}

// applyEvent mutates state according to the event type.
func applyEvent(state *sightjack.SessionState, e sightjack.Event) {
	switch e.Type {
	case sightjack.EventSessionStarted:
		var p sightjack.SessionStartedPayload
		if unmarshalPayload(e, &p) {
			state.Version = sightjack.StateFormatVersion
			state.SessionID = e.SessionID
			state.Project = p.Project
			state.StrictnessLevel = p.StrictnessLevel
		}

	case sightjack.EventScanCompleted:
		var p sightjack.ScanCompletedPayload
		if unmarshalPayload(e, &p) {
			state.Clusters = p.Clusters
			state.Completeness = p.Completeness
			state.ShibitoCount = p.ShibitoCount
			state.ScanResultPath = p.ScanResultPath
			state.LastScanned = p.LastScanned
		}

	case sightjack.EventWavesGenerated:
		var p sightjack.WavesGeneratedPayload
		if unmarshalPayload(e, &p) {
			state.Waves = append(state.Waves, p.Waves...)
		}

	case sightjack.EventWaveCompleted:
		var p sightjack.WaveCompletedPayload
		if unmarshalPayload(e, &p) {
			key := p.ClusterName + ":" + p.WaveID
			for i, w := range state.Waves {
				if w.ClusterName+":"+w.ID == key {
					state.Waves[i].Status = "completed"
					break
				}
			}
		}

	case sightjack.EventCompletenessUpdated:
		var p sightjack.CompletenessUpdatedPayload
		if unmarshalPayload(e, &p) {
			state.Completeness = p.OverallCompleteness
			for i, c := range state.Clusters {
				if c.Name == p.ClusterName {
					state.Clusters[i].Completeness = p.ClusterCompleteness
					break
				}
			}
		}

	case sightjack.EventWavesUnlocked:
		var p sightjack.WavesUnlockedPayload
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

	case sightjack.EventNextGenWavesAdded:
		var p sightjack.NextGenWavesAddedPayload
		if unmarshalPayload(e, &p) {
			state.Waves = append(state.Waves, p.Waves...)
		}

	case sightjack.EventWaveModified:
		var p sightjack.WaveModifiedPayload
		if unmarshalPayload(e, &p) {
			key := p.ClusterName + ":" + p.WaveID
			for i, w := range state.Waves {
				if w.ClusterName+":"+w.ID == key {
					state.Waves[i] = p.UpdatedWave
					break
				}
			}
		}

	case sightjack.EventADRGenerated:
		state.ADRCount++

	case sightjack.EventWaveApproved, sightjack.EventWaveRejected, sightjack.EventWaveApplied,
		sightjack.EventSpecificationSent, sightjack.EventReportSent,
		sightjack.EventReadyLabelsApplied, sightjack.EventSessionResumed, sightjack.EventSessionRescanned:
		// Audit-only events: no state mutation needed.

	default:
		// Unknown event type: silently skip for forward compatibility.
	}
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

// unmarshalPayload is a helper that returns false on unmarshal failure.
func unmarshalPayload(e sightjack.Event, target any) bool {
	return sightjack.UnmarshalEventPayload(e, target) == nil
}
