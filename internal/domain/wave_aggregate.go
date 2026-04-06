package domain

import (
	"fmt"
	"time"
)

// AggregateTypeWave is the aggregate type for wave lifecycle events.
const AggregateTypeWave = "wave"

// WaveAggregate owns wave lifecycle state and produces events for state transitions.
// It wraps domain/ pure functions with event emission.
type WaveAggregate struct {
	waves     []Wave
	completed map[string]bool // keyed by WaveKey (ClusterName:ID)
	seqNr     uint64
}

// NewWaveAggregate creates an empty WaveAggregate.
func NewWaveAggregate() *WaveAggregate {
	return &WaveAggregate{
		completed: make(map[string]bool),
	}
}

// SetWaves replaces the current wave list (used for hydration from projection).
func (a *WaveAggregate) SetWaves(waves []Wave) {
	a.waves = waves
}

// Waves returns the current wave list.
func (a *WaveAggregate) Waves() []Wave {
	return a.waves
}

// Completed returns the completed map.
func (a *WaveAggregate) Completed() map[string]bool {
	return a.completed
}

// SetCompleted replaces the completed map (used for hydration from projection).
func (a *WaveAggregate) SetCompleted(completed map[string]bool) {
	a.completed = completed
}

// MarkCompleted marks a wave as completed by its WaveKey.
func (a *WaveAggregate) MarkCompleted(waveKey string) {
	a.completed[waveKey] = true
}

// IsCompleted checks if a wave is completed by its WaveKey.
func (a *WaveAggregate) IsCompleted(waveKey string) bool {
	return a.completed[waveKey]
}

// nextEvent creates an event tagged with wave aggregate identity.
// Uses the waveKey (cluster:id) from the last operation as AggregateID.
func (a *WaveAggregate) nextEvent(eventType EventType, data any, now time.Time, waveKey string) (Event, error) {
	a.seqNr++
	ev, err := NewEvent(eventType, data, now)
	if err != nil {
		return ev, err
	}
	ev.AggregateID = waveKey
	ev.AggregateType = AggregateTypeWave
	ev.SeqNr = a.seqNr
	return ev, nil
}

// findWave returns the wave matching the given ID and cluster.
func (a *WaveAggregate) findWave(waveID, clusterName string) (Wave, bool) {
	for _, w := range a.waves {
		if w.ID == waveID && w.ClusterName == clusterName {
			return w, true
		}
	}
	return Wave{}, false
}

// Approve produces a wave_approved event.
func (a *WaveAggregate) Approve(waveID, clusterName string, now time.Time) (Event, error) {
	if _, ok := a.findWave(waveID, clusterName); !ok {
		return Event{}, fmt.Errorf("wave %s:%s not found", clusterName, waveID)
	}
	return a.nextEvent(EventWaveApprovedV2, WaveIdentityPayload{
		WaveID:      waveID,
		ClusterName: clusterName,
	}, now, clusterName+":"+waveID)
}

// Reject produces a wave_rejected event.
func (a *WaveAggregate) Reject(waveID, clusterName string, now time.Time) (Event, error) {
	if _, ok := a.findWave(waveID, clusterName); !ok {
		return Event{}, fmt.Errorf("wave %s:%s not found", clusterName, waveID)
	}
	return a.nextEvent(EventWaveRejectedV2, WaveIdentityPayload{
		WaveID:      waveID,
		ClusterName: clusterName,
	}, now, clusterName+":"+waveID)
}

// RecordApplied produces a wave_applied event.
func (a *WaveAggregate) RecordApplied(payload WaveAppliedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventWaveAppliedV2, payload, now, payload.ClusterName+":"+payload.WaveID)
}

// Complete produces a wave_completed event and marks the wave as completed.
func (a *WaveAggregate) Complete(waveID, clusterName string, applied, totalCount int, now time.Time) (Event, error) {
	if _, ok := a.findWave(waveID, clusterName); !ok {
		return Event{}, fmt.Errorf("wave %s:%s not found", clusterName, waveID)
	}
	waveKey := clusterName + ":" + waveID
	a.completed[waveKey] = true
	for i := range a.waves {
		if a.waves[i].ID == waveID && a.waves[i].ClusterName == clusterName {
			a.waves[i].Status = "completed"
			break
		}
	}
	return a.nextEvent(EventWaveCompletedV2, WaveCompletedPayload{
		WaveID:      waveID,
		ClusterName: clusterName,
		Applied:     applied,
		TotalCount:  totalCount,
	}, now, waveKey)
}

// EvaluateUnlocks checks locked waves and produces a waves_unlocked event if any are unlocked.
// Inlines unlock logic to avoid root → internal/domain circular import.
func (a *WaveAggregate) EvaluateUnlocks(now time.Time) ([]Event, error) {
	var unlockedIDs []string
	for i, w := range a.waves {
		if w.Status != "locked" {
			continue
		}
		allMet := true
		for _, prereq := range w.Prerequisites {
			if !a.completed[prereq] {
				allMet = false
				break
			}
		}
		if allMet {
			a.waves[i].Status = "available"
			unlockedIDs = append(unlockedIDs, w.ClusterName+":"+w.ID)
		}
	}

	if len(unlockedIDs) == 0 {
		return nil, nil
	}

	ev, err := a.nextEvent(EventWavesUnlockedV2, WavesUnlockedPayload{
		UnlockedWaveIDs: unlockedIDs,
	}, now, "")
	if err != nil {
		return nil, err
	}
	return []Event{ev}, nil
}

// AddNextGen produces a nextgen_waves_added event.
func (a *WaveAggregate) AddNextGen(clusterName string, waves []WaveState, now time.Time) (Event, error) {
	return a.nextEvent(EventNextGenWavesAddedV2, NextGenWavesAddedPayload{
		ClusterName: clusterName,
		Waves:       waves,
	}, now, "")
}

// WaveStatusCounts returns a map of wave status to count across all waves.
// Possible keys are the status strings present in the wave list (e.g. "available",
// "locked", "completed").
func (a *WaveAggregate) WaveStatusCounts() map[string]int {
	counts := make(map[string]int)
	for _, w := range a.waves {
		counts[w.Status]++
	}
	return counts
}

// AllWavesCompleted reports whether every wave in the aggregate has been completed.
// Returns false for an empty wave list.
func (a *WaveAggregate) AllWavesCompleted() bool {
	if len(a.waves) == 0 {
		return false
	}
	for _, w := range a.waves {
		if w.Status != "completed" {
			return false
		}
	}
	return true
}
