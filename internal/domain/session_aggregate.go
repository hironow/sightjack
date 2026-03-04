package domain

import (
	"time"
)

// SessionAggregate owns session lifecycle state and produces events for session transitions.
type SessionAggregate struct {
	sessionID string
}

// NewSessionAggregate creates an empty SessionAggregate.
func NewSessionAggregate() *SessionAggregate {
	return &SessionAggregate{}
}

// SetSessionID sets the session ID (used for hydration from projection).
func (a *SessionAggregate) SetSessionID(id string) {
	a.sessionID = id
}

// SessionID returns the current session ID.
func (a *SessionAggregate) SessionID() string {
	return a.sessionID
}

// Start produces a session_started event.
func (a *SessionAggregate) Start(project, strictness string, now time.Time) (Event, error) {
	return NewEvent(EventSessionStarted, SessionStartedPayload{
		Project:         project,
		StrictnessLevel: strictness,
	}, now)
}

// RecordScan produces a scan_completed event.
func (a *SessionAggregate) RecordScan(payload ScanCompletedPayload, now time.Time) (Event, error) {
	return NewEvent(EventScanCompleted, payload, now)
}

// UpdateCompleteness produces a completeness_updated event.
func (a *SessionAggregate) UpdateCompleteness(clusterName string, clusterCompleteness, overallCompleteness float64, now time.Time) (Event, error) {
	return NewEvent(EventCompletenessUpdated, CompletenessUpdatedPayload{
		ClusterName:         clusterName,
		ClusterCompleteness: clusterCompleteness,
		OverallCompleteness: overallCompleteness,
	}, now)
}

// Resume produces a session_resumed event.
func (a *SessionAggregate) Resume(originalSessionID string, now time.Time) (Event, error) {
	return NewEvent(EventSessionResumed, SessionResumedPayload{
		OriginalSessionID: originalSessionID,
	}, now)
}

// Rescan produces a session_rescanned event.
func (a *SessionAggregate) Rescan(originalSessionID string, now time.Time) (Event, error) {
	return NewEvent(EventSessionRescanned, SessionRescannedPayload{
		OriginalSessionID: originalSessionID,
	}, now)
}

// RecordWavesGenerated produces a waves_generated event.
func (a *SessionAggregate) RecordWavesGenerated(payload WavesGeneratedPayload, now time.Time) (Event, error) {
	return NewEvent(EventWavesGenerated, payload, now)
}

// ApproveWave produces a wave_approved event.
func (a *SessionAggregate) ApproveWave(waveID, clusterName string, now time.Time) (Event, error) {
	return NewEvent(EventWaveApproved, WaveIdentityPayload{
		WaveID: waveID, ClusterName: clusterName,
	}, now)
}

// RejectWave produces a wave_rejected event.
func (a *SessionAggregate) RejectWave(waveID, clusterName string, now time.Time) (Event, error) {
	return NewEvent(EventWaveRejected, WaveIdentityPayload{
		WaveID: waveID, ClusterName: clusterName,
	}, now)
}

// ModifyWave produces a wave_modified event.
func (a *SessionAggregate) ModifyWave(payload WaveModifiedPayload, now time.Time) (Event, error) {
	return NewEvent(EventWaveModified, payload, now)
}

// ApplyWave produces a wave_applied event.
func (a *SessionAggregate) ApplyWave(payload WaveAppliedPayload, now time.Time) (Event, error) {
	return NewEvent(EventWaveApplied, payload, now)
}

// CompleteWave produces a wave_completed event.
func (a *SessionAggregate) CompleteWave(payload WaveCompletedPayload, now time.Time) (Event, error) {
	return NewEvent(EventWaveCompleted, payload, now)
}

// AddNextGenWaves produces a nextgen_waves_added event.
func (a *SessionAggregate) AddNextGenWaves(payload NextGenWavesAddedPayload, now time.Time) (Event, error) {
	return NewEvent(EventNextGenWavesAdded, payload, now)
}

// ApplyReadyLabels produces a ready_labels_applied event.
func (a *SessionAggregate) ApplyReadyLabels(payload ReadyLabelsAppliedPayload, now time.Time) (Event, error) {
	return NewEvent(EventReadyLabelsApplied, payload, now)
}

// SendSpecification produces a specification_sent event.
func (a *SessionAggregate) SendSpecification(waveID, clusterName string, now time.Time) (Event, error) {
	return NewEvent(EventSpecificationSent, WaveIdentityPayload{
		WaveID: waveID, ClusterName: clusterName,
	}, now)
}

// SendReport produces a report_sent event.
func (a *SessionAggregate) SendReport(waveID, clusterName string, now time.Time) (Event, error) {
	return NewEvent(EventReportSent, WaveIdentityPayload{
		WaveID: waveID, ClusterName: clusterName,
	}, now)
}

// SendFeedback produces a feedback_sent event.
func (a *SessionAggregate) SendFeedback(waveID, clusterName string, now time.Time) (Event, error) {
	return NewEvent(EventFeedbackSent, WaveIdentityPayload{
		WaveID: waveID, ClusterName: clusterName,
	}, now)
}

// GenerateADR produces an adr_generated event.
func (a *SessionAggregate) GenerateADR(payload ADRGeneratedPayload, now time.Time) (Event, error) {
	return NewEvent(EventADRGenerated, payload, now)
}

// UnlockWaves produces a waves_unlocked event.
func (a *SessionAggregate) UnlockWaves(unlockedIDs []string, now time.Time) (Event, error) {
	return NewEvent(EventWavesUnlocked, WavesUnlockedPayload{
		UnlockedWaveIDs: unlockedIDs,
	}, now)
}
