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
	return a.nextEvent(EventSessionStarted, SessionStartedPayload{
		Project:         project,
		StrictnessLevel: strictness,
	}, now)
}

// RecordScan produces a scan_completed event.
func (a *SessionAggregate) RecordScan(payload ScanCompletedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventScanCompleted, payload, now)
}

// UpdateCompleteness produces a completeness_updated event.
func (a *SessionAggregate) UpdateCompleteness(clusterName string, clusterCompleteness, overallCompleteness float64, now time.Time) (Event, error) {
	return a.nextEvent(EventCompletenessUpdated, CompletenessUpdatedPayload{
		ClusterName:         clusterName,
		ClusterCompleteness: clusterCompleteness,
		OverallCompleteness: overallCompleteness,
	}, now)
}

// Resume produces a session_resumed event.
func (a *SessionAggregate) Resume(originalSessionID string, now time.Time) (Event, error) {
	return a.nextEvent(EventSessionResumed, SessionResumedPayload{
		OriginalSessionID: originalSessionID,
	}, now)
}

// Rescan produces a session_rescanned event.
func (a *SessionAggregate) Rescan(originalSessionID string, now time.Time) (Event, error) {
	return a.nextEvent(EventSessionRescanned, SessionRescannedPayload{
		OriginalSessionID: originalSessionID,
	}, now)
}

// RecordWavesGenerated produces a waves_generated event.
func (a *SessionAggregate) RecordWavesGenerated(payload WavesGeneratedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventWavesGenerated, payload, now)
}

// ApproveWave produces a wave_approved event.
func (a *SessionAggregate) ApproveWave(waveID, clusterName string, now time.Time) (Event, error) {
	return a.nextEvent(EventWaveApproved, WaveIdentityPayload{
		WaveID: waveID, ClusterName: clusterName,
	}, now)
}

// RejectWave produces a wave_rejected event.
func (a *SessionAggregate) RejectWave(waveID, clusterName string, now time.Time) (Event, error) {
	return a.nextEvent(EventWaveRejected, WaveIdentityPayload{
		WaveID: waveID, ClusterName: clusterName,
	}, now)
}

// ModifyWave produces a wave_modified event.
func (a *SessionAggregate) ModifyWave(payload WaveModifiedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventWaveModified, payload, now)
}

// ApplyWave produces a wave_applied event.
func (a *SessionAggregate) ApplyWave(payload WaveAppliedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventWaveApplied, payload, now)
}

// CompleteWave produces a wave_completed event.
func (a *SessionAggregate) CompleteWave(payload WaveCompletedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventWaveCompleted, payload, now)
}

// AddNextGenWaves produces a nextgen_waves_added event.
func (a *SessionAggregate) AddNextGenWaves(payload NextGenWavesAddedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventNextGenWavesAdded, payload, now)
}

// ApplyReadyLabels produces a ready_labels_applied event.
func (a *SessionAggregate) ApplyReadyLabels(payload ReadyLabelsAppliedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventReadyLabelsApplied, payload, now)
}

// SendSpecification produces a specification_sent event.
func (a *SessionAggregate) SendSpecification(waveID, clusterName string, now time.Time) (Event, error) {
	return a.nextEvent(EventSpecificationSent, WaveIdentityPayload{
		WaveID: waveID, ClusterName: clusterName,
	}, now)
}

// SendReport produces a report_sent event.
func (a *SessionAggregate) SendReport(waveID, clusterName string, now time.Time) (Event, error) {
	return a.nextEvent(EventReportSent, WaveIdentityPayload{
		WaveID: waveID, ClusterName: clusterName,
	}, now)
}

// SendFeedback produces a feedback_sent event.
func (a *SessionAggregate) SendFeedback(waveID, clusterName string, now time.Time) (Event, error) {
	return a.nextEvent(EventFeedbackSent, WaveIdentityPayload{
		WaveID: waveID, ClusterName: clusterName,
	}, now)
}

// GenerateADR produces an adr_generated event.
func (a *SessionAggregate) GenerateADR(payload ADRGeneratedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventADRGenerated, payload, now)
}

// ReceiveFeedback produces a feedback_received event.
func (a *SessionAggregate) ReceiveFeedback(payload FeedbackReceivedPayload, now time.Time) (Event, error) {
	return a.nextEvent(EventFeedbackReceived, payload, now)
}

// UnlockWaves produces a waves_unlocked event.
func (a *SessionAggregate) UnlockWaves(unlockedIDs []string, now time.Time) (Event, error) {
	return a.nextEvent(EventWavesUnlocked, WavesUnlockedPayload{
		UnlockedWaveIDs: unlockedIDs,
	}, now)
}
