package domain

import (
	sightjack "github.com/hironow/sightjack"
)

// projectionContext tracks seen entities during replay to ensure idempotency.
type projectionContext struct {
	seenWaves map[string]bool // key = ClusterName:WaveID
	seenADRs  map[string]bool // key = ADRID
}

// ProjectState replays a sequence of events to produce a SessionState.
// Unknown event types are silently skipped.
// Returns a zero-value SessionState for nil/empty input.
func ProjectState(events []sightjack.Event) *sightjack.SessionState {
	state := &sightjack.SessionState{}
	ctx := &projectionContext{
		seenWaves: make(map[string]bool),
		seenADRs:  make(map[string]bool),
	}
	for _, e := range events {
		applyEvent(state, ctx, e)
	}
	return state
}

// applyEvent mutates state according to the event type.
// The projectionContext tracks seen entities to ensure idempotent replay.
func applyEvent(state *sightjack.SessionState, ctx *projectionContext, e sightjack.Event) {
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
			for _, w := range p.Waves {
				key := w.ClusterName + ":" + w.ID
				if !ctx.seenWaves[key] {
					ctx.seenWaves[key] = true
					state.Waves = append(state.Waves, w)
				}
			}
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
			for _, w := range p.Waves {
				key := w.ClusterName + ":" + w.ID
				if !ctx.seenWaves[key] {
					ctx.seenWaves[key] = true
					state.Waves = append(state.Waves, w)
				}
			}
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
		var p sightjack.ADRGeneratedPayload
		if unmarshalPayload(e, &p) {
			if !ctx.seenADRs[p.ADRID] {
				ctx.seenADRs[p.ADRID] = true
				state.ADRCount++
			}
		}

	case sightjack.EventWaveApproved, sightjack.EventWaveRejected, sightjack.EventWaveApplied,
		sightjack.EventSpecificationSent, sightjack.EventReportSent,
		sightjack.EventReadyLabelsApplied, sightjack.EventSessionResumed, sightjack.EventSessionRescanned:
		// Audit-only events: no state mutation needed.

	default:
		// Unknown event type: silently skip for forward compatibility.
	}
}

// unmarshalPayload is a helper that returns false on unmarshal failure.
func unmarshalPayload(e sightjack.Event, target any) bool {
	return sightjack.UnmarshalEventPayload(e, target) == nil
}
