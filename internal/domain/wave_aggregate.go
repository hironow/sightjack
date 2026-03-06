package domain

import (
	"fmt"
	"time"
)

// WaveAggregate owns wave lifecycle state and produces events for state transitions.
// It wraps domain/ pure functions with event emission.
type WaveAggregate struct {
	waves     []Wave
	completed map[string]bool // keyed by WaveKey (ClusterName:ID)
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
	return NewEvent(EventWaveApproved, WaveIdentityPayload{
		WaveID:      waveID,
		ClusterName: clusterName,
	}, now)
}

// Reject produces a wave_rejected event.
func (a *WaveAggregate) Reject(waveID, clusterName string, now time.Time) (Event, error) {
	if _, ok := a.findWave(waveID, clusterName); !ok {
		return Event{}, fmt.Errorf("wave %s:%s not found", clusterName, waveID)
	}
	return NewEvent(EventWaveRejected, WaveIdentityPayload{
		WaveID:      waveID,
		ClusterName: clusterName,
	}, now)
}

// RecordApplied produces a wave_applied event.
func (a *WaveAggregate) RecordApplied(payload WaveAppliedPayload, now time.Time) (Event, error) {
	return NewEvent(EventWaveApplied, payload, now)
}

// Complete produces a wave_completed event and marks the wave as completed.
func (a *WaveAggregate) Complete(waveID, clusterName string, applied, totalCount int, now time.Time) (Event, error) {
	waveKey := clusterName + ":" + waveID
	a.completed[waveKey] = true
	return NewEvent(EventWaveCompleted, WaveCompletedPayload{
		WaveID:      waveID,
		ClusterName: clusterName,
		Applied:     applied,
		TotalCount:  totalCount,
	}, now)
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

	ev, err := NewEvent(EventWavesUnlocked, WavesUnlockedPayload{
		UnlockedWaveIDs: unlockedIDs,
	}, now)
	if err != nil {
		return nil, err
	}
	return []Event{ev}, nil
}

// AddNextGen produces a nextgen_waves_added event.
func (a *WaveAggregate) AddNextGen(clusterName string, waves []WaveState, now time.Time) (Event, error) {
	return NewEvent(EventNextGenWavesAdded, NextGenWavesAddedPayload{
		ClusterName: clusterName,
		Waves:       waves,
	}, now)
}
