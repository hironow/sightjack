package domain

// projectionContext tracks seen entities during replay to ensure idempotency.
type projectionContext struct {
	seenWaves map[string]bool // key = ClusterName:WaveID
	seenADRs  map[string]bool // key = ADRID
}

// ProjectState replays a sequence of events to produce a SessionState.
// Unknown event types are silently skipped.
// Returns a zero-value SessionState for nil/empty input.
func ProjectState(events []Event) *SessionState {
	state := &SessionState{}
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
func applyEvent(state *SessionState, ctx *projectionContext, e Event) {
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
			for _, w := range p.Waves {
				key := w.ClusterName + ":" + w.ID
				if !ctx.seenWaves[key] {
					ctx.seenWaves[key] = true
					state.Waves = append(state.Waves, w)
				}
			}
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
			for _, w := range p.Waves {
				key := w.ClusterName + ":" + w.ID
				if !ctx.seenWaves[key] {
					ctx.seenWaves[key] = true
					state.Waves = append(state.Waves, w)
				}
			}
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
		var p ADRGeneratedPayload
		if unmarshalPayload(e, &p) {
			if !ctx.seenADRs[p.ADRID] {
				ctx.seenADRs[p.ADRID] = true
				state.ADRCount++
			}
		}

	case EventWaveApproved, EventWaveRejected, EventWaveApplied,
		EventSpecificationSent, EventReportSent,
		EventReadyLabelsApplied, EventSessionResumed, EventSessionRescanned:
		// Audit-only events: no state mutation needed.

	default:
		// Unknown event type: silently skip for forward compatibility.
	}
}

// unmarshalPayload is a helper that returns false on unmarshal failure.
func unmarshalPayload(e Event, target any) bool {
	return UnmarshalEventPayload(e, target) == nil
}
