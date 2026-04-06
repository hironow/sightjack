package domain

import (
	"time"
)

// AggregateTypeSession is the aggregate type for session lifecycle events.
const AggregateTypeSession = "session"

// SessionAggregate owns session lifecycle state and produces events for session transitions.
type SessionAggregate struct {
	sessionID string
	seqNr     uint64
}

// NewSessionAggregate creates an empty SessionAggregate.
func NewSessionAggregate() *SessionAggregate {
	return &SessionAggregate{}
}

// SetSessionID sets the session ID (used for hydration from projection).
func (a *SessionAggregate) SetSessionID(id string) {
	a.sessionID = id
}

// nextEvent creates an event tagged with this aggregate's identity and increments SeqNr.
func (a *SessionAggregate) nextEvent(eventType EventType, data any, now time.Time) (Event, error) {
	a.seqNr++
	ev, err := NewEvent(eventType, data, now)
	if err != nil {
		return ev, err
	}
	ev.SessionID = a.sessionID // backward compat (legacy field)
	ev.AggregateID = a.sessionID
	ev.AggregateType = AggregateTypeSession
	ev.SeqNr = a.seqNr
	return ev, nil
}

// SessionID returns the current session ID.
func (a *SessionAggregate) SessionID() string {
	return a.sessionID
}

// Start produces a session_started event.
func (a *SessionAggregate) Start(project, strictness string, now time.Time) (Event, error) {
	return a.nextEvent(EventSessionStartedV2, SessionStartedPayload{
		Project:         project,
		StrictnessLevel: strictness,
	}, now)
}

// RecordScan produces a scan_completed event.
func (a *SessionAggregate) RecordScan(payload ScanCompletedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventScanCompletedV2, payload, now)
}

// UpdateCompleteness produces a completeness_updated event.
func (a *SessionAggregate) UpdateCompleteness(clusterName string, clusterCompleteness, overallCompleteness float64, now time.Time) (Event, error) {
	return a.nextEvent(EventCompletenessUpdatedV2, CompletenessUpdatedPayload{
		ClusterName:         clusterName,
		ClusterCompleteness: clusterCompleteness,
		OverallCompleteness: overallCompleteness,
	}, now)
}

// Resume produces a session_resumed event.
func (a *SessionAggregate) Resume(originalSessionID string, now time.Time) (Event, error) {
	return a.nextEvent(EventSessionResumedV2, SessionResumedPayload{
		OriginalSessionID: originalSessionID,
	}, now)
}

// Rescan produces a session_rescanned event.
func (a *SessionAggregate) Rescan(originalSessionID string, now time.Time) (Event, error) {
	return a.nextEvent(EventSessionRescannedV2, SessionRescannedPayload{
		OriginalSessionID: originalSessionID,
	}, now)
}

// RecordWavesGenerated produces a waves_generated event.
func (a *SessionAggregate) RecordWavesGenerated(payload WavesGeneratedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventWavesGeneratedV2, payload, now)
}

// ApproveWave produces a wave_approved event.
func (a *SessionAggregate) ApproveWave(waveID, clusterName string, now time.Time) (Event, error) {
	return a.nextEvent(EventWaveApprovedV2, WaveIdentityPayload{
		WaveID: waveID, ClusterName: clusterName,
	}, now)
}

// RejectWave produces a wave_rejected event.
func (a *SessionAggregate) RejectWave(waveID, clusterName string, now time.Time) (Event, error) {
	return a.nextEvent(EventWaveRejectedV2, WaveIdentityPayload{
		WaveID: waveID, ClusterName: clusterName,
	}, now)
}

// ModifyWave produces a wave_modified event.
func (a *SessionAggregate) ModifyWave(payload WaveModifiedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventWaveModifiedV2, payload, now)
}

// ApplyWave produces a wave_applied event.
func (a *SessionAggregate) ApplyWave(payload WaveAppliedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventWaveAppliedV2, payload, now)
}

// CompleteWave produces a wave_completed event.
func (a *SessionAggregate) CompleteWave(payload WaveCompletedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventWaveCompletedV2, payload, now)
}

// AddNextGenWaves produces a nextgen_waves_added event.
func (a *SessionAggregate) AddNextGenWaves(payload NextGenWavesAddedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventNextGenWavesAddedV2, payload, now)
}

// ApplyReadyLabels produces a ready_labels_applied event.
func (a *SessionAggregate) ApplyReadyLabels(payload ReadyLabelsAppliedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventReadyLabelsAppliedV2, payload, now)
}

// SendSpecification produces a specification_sent event.
func (a *SessionAggregate) SendSpecification(waveID, clusterName string, now time.Time) (Event, error) {
	return a.nextEvent(EventSpecificationSentV2, WaveIdentityPayload{
		WaveID: waveID, ClusterName: clusterName,
	}, now)
}

// SendReport produces a report_sent event.
func (a *SessionAggregate) SendReport(waveID, clusterName string, now time.Time) (Event, error) {
	return a.nextEvent(EventReportSentV2, WaveIdentityPayload{
		WaveID: waveID, ClusterName: clusterName,
	}, now)
}

// SendFeedback produces a feedback_sent event.
func (a *SessionAggregate) SendFeedback(waveID, clusterName string, now time.Time) (Event, error) {
	return a.nextEvent(EventFeedbackSentV2, WaveIdentityPayload{
		WaveID: waveID, ClusterName: clusterName,
	}, now)
}

// GenerateADR produces an adr_generated event.
func (a *SessionAggregate) GenerateADR(payload ADRGeneratedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventADRGeneratedV2, payload, now)
}

// ReceiveFeedback produces a feedback_received event.
func (a *SessionAggregate) ReceiveFeedback(payload FeedbackReceivedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventFeedbackReceivedV2, payload, now)
}

// UnlockWaves produces a waves_unlocked event.
func (a *SessionAggregate) UnlockWaves(unlockedIDs []string, now time.Time) (Event, error) {
	return a.nextEvent(EventWavesUnlockedV2, WavesUnlockedPayload{
		UnlockedWaveIDs: unlockedIDs,
	}, now)
}
