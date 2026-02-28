package sightjack

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
