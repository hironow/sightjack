package domain

import "encoding/json"

// projectionContext tracks seen entities during replay to ensure idempotency.
type projectionContext struct {
	seenWaves map[string]bool // key = ClusterName:WaveID
	seenADRs  map[string]bool // key = ADRID
}

// projectionSnapshot is the serialized form of a Projector's state.
// It includes both SessionState AND dedup maps — SessionState.ADRCount
// only stores the count (not IDs), so dedup maps cannot be reconstructed
// from SessionState alone.
type projectionSnapshot struct {
	State     SessionState    `json:"state"`
	SeenWaves map[string]bool `json:"seen_waves"`
	SeenADRs  map[string]bool `json:"seen_adrs"`
}

// Projector implements EventApplier by wrapping SessionState with dedup context.
type Projector struct {
	state SessionState
	ctx   projectionContext
}

// compile-time interface check
var _ EventApplier = (*Projector)(nil)

// NewProjector creates a Projector with zeroed state.
func NewProjector() *Projector {
	return &Projector{
		ctx: projectionContext{
			seenWaves: make(map[string]bool),
			seenADRs:  make(map[string]bool),
		},
	}
}

// Apply applies a single event to the projection.
func (p *Projector) Apply(ev Event) error {
	applyEvent(&p.state, &p.ctx, ev)
	return nil
}

// Rebuild resets and replays all events.
func (p *Projector) Rebuild(events []Event) error {
	p.state = SessionState{}
	p.ctx = projectionContext{
		seenWaves: make(map[string]bool),
		seenADRs:  make(map[string]bool),
	}
	for _, e := range events {
		applyEvent(&p.state, &p.ctx, e)
	}
	return nil
}

// Serialize returns the projection state (including dedup maps) as JSON.
func (p *Projector) Serialize() ([]byte, error) {
	snap := projectionSnapshot{
		State:     p.state,
		SeenWaves: p.ctx.seenWaves,
		SeenADRs:  p.ctx.seenADRs,
	}
	return json.Marshal(snap)
}

// Deserialize restores projection state (including dedup maps) from JSON.
func (p *Projector) Deserialize(data []byte) error {
	var snap projectionSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	p.state = snap.State
	p.ctx.seenWaves = snap.SeenWaves
	p.ctx.seenADRs = snap.SeenADRs
	if p.ctx.seenWaves == nil {
		p.ctx.seenWaves = make(map[string]bool)
	}
	if p.ctx.seenADRs == nil {
		p.ctx.seenADRs = make(map[string]bool)
	}
	return nil
}

// State returns the current materialized SessionState.
func (p *Projector) State() *SessionState {
	return &p.state
}

// ProjectState replays a sequence of events to produce a SessionState.
// Convenience wrapper around Projector for backward compatibility.
func ProjectState(events []Event) *SessionState {
	p := NewProjector()
	p.Rebuild(events)
	return p.State()
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

	case EventFeedbackSent:
		state.FeedbackCount++

	case EventFeedbackReceived:
		// Audit-only: feedback reception is logged but does not mutate state.
		// The FeedbackCount tracks outbound (sent) feedback only.

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
