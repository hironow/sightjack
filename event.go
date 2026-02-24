package sightjack

import (
	"encoding/json"
	"fmt"
	"time"
)

// EventType identifies the kind of domain event.
type EventType string

const (
	EventSessionStarted      EventType = "session_started"
	EventScanCompleted       EventType = "scan_completed"
	EventWavesGenerated      EventType = "waves_generated"
	EventWaveApproved        EventType = "wave_approved"
	EventWaveRejected        EventType = "wave_rejected"
	EventWaveModified        EventType = "wave_modified"
	EventWaveApplied         EventType = "wave_applied"
	EventWaveCompleted       EventType = "wave_completed"
	EventCompletenessUpdated EventType = "completeness_updated"
	EventWavesUnlocked       EventType = "waves_unlocked"
	EventNextGenWavesAdded   EventType = "nextgen_waves_added"
	EventADRGenerated        EventType = "adr_generated"
	EventReadyLabelsApplied  EventType = "ready_labels_applied"
	EventSessionResumed      EventType = "session_resumed"
	EventSessionRescanned    EventType = "session_rescanned"
	EventSpecificationSent   EventType = "specification_sent"
	EventReportSent          EventType = "report_sent"
)

// EventSchemaVersion is the schema version stamped into every event envelope.
const EventSchemaVersion = "2"

// Event is the immutable event envelope persisted to the event store.
// PayloadRaw holds the JSON-encoded payload; the Payload field is transient.
type Event struct {
	SchemaVersion string          `json:"schema_version"`
	Type          EventType       `json:"type"`
	Timestamp     time.Time       `json:"timestamp"`
	SessionID     string          `json:"session_id"`
	Sequence      int64           `json:"sequence"`
	CorrelationID string          `json:"correlation_id"`
	CausationID   string          `json:"causation_id"`
	PayloadRaw    json.RawMessage `json:"payload"`
}

// NewEvent creates an Event with the given type, session, sequence, and payload.
// The payload is immediately marshaled to PayloadRaw.
func NewEvent(eventType EventType, sessionID string, seq int64, payload any) (Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("marshal event payload: %w", err)
	}
	return Event{
		SchemaVersion: EventSchemaVersion,
		Type:          eventType,
		Timestamp:     time.Now(),
		SessionID:     sessionID,
		Sequence:      seq,
		PayloadRaw:    raw,
	}, nil
}

// ValidateEvent checks structural validity of an Event before persistence.
// It returns an error if any required field is missing or invalid.
func ValidateEvent(e Event) error {
	if e.Type == "" {
		return fmt.Errorf("validate event: type is empty")
	}
	if e.SessionID == "" {
		return fmt.Errorf("validate event: session_id is empty")
	}
	if e.Sequence < 1 {
		return fmt.Errorf("validate event: sequence must be >= 1, got %d", e.Sequence)
	}
	if e.SchemaVersion == "" {
		return fmt.Errorf("validate event: schema_version is empty")
	}
	if e.Timestamp.IsZero() {
		return fmt.Errorf("validate event: timestamp is zero")
	}
	if len(e.PayloadRaw) == 0 {
		return fmt.Errorf("validate event: payload is empty")
	}
	return nil
}

// MarshalEvent serializes an Event to compact JSON (no trailing newline).
func MarshalEvent(e Event) ([]byte, error) {
	return json.Marshal(e)
}

// UnmarshalEventPayload deserializes the PayloadRaw field into the given target.
func UnmarshalEventPayload(e Event, target any) error {
	return json.Unmarshal(e.PayloadRaw, target)
}

// SessionStartedPayload is the payload for EventSessionStarted.
type SessionStartedPayload struct {
	Project         string `json:"project"`
	StrictnessLevel string `json:"strictness_level"`
}

// ScanCompletedPayload is the payload for EventScanCompleted.
type ScanCompletedPayload struct {
	Clusters       []ClusterState `json:"clusters"`
	Completeness   float64        `json:"completeness"`
	ShibitoCount   int            `json:"shibito_count"`
	ScanResultPath string         `json:"scan_result_path"`
	LastScanned    time.Time      `json:"last_scanned"`
}

// WavesGeneratedPayload is the payload for EventWavesGenerated.
type WavesGeneratedPayload struct {
	Waves []WaveState `json:"waves"`
}

// WaveIdentityPayload is a shared payload for events that reference a single wave.
// Used by: EventWaveApproved, EventWaveRejected, EventSpecificationSent, EventReportSent.
type WaveIdentityPayload struct {
	WaveID      string `json:"wave_id"`
	ClusterName string `json:"cluster_name"`
}

// WaveModifiedPayload is the payload for EventWaveModified.
type WaveModifiedPayload struct {
	WaveID      string    `json:"wave_id"`
	ClusterName string    `json:"cluster_name"`
	UpdatedWave WaveState `json:"updated_wave"`
}

// WaveAppliedPayload is the payload for EventWaveApplied.
type WaveAppliedPayload struct {
	WaveID      string   `json:"wave_id"`
	ClusterName string   `json:"cluster_name"`
	Applied     int      `json:"applied"`
	TotalCount  int      `json:"total_count"`
	Errors      []string `json:"errors,omitempty"`
}

// WaveCompletedPayload is the payload for EventWaveCompleted.
type WaveCompletedPayload struct {
	WaveID      string `json:"wave_id"`
	ClusterName string `json:"cluster_name"`
	Applied     int    `json:"applied"`
	TotalCount  int    `json:"total_count"`
}

// CompletenessUpdatedPayload is the payload for EventCompletenessUpdated.
type CompletenessUpdatedPayload struct {
	ClusterName         string  `json:"cluster_name"`
	ClusterCompleteness float64 `json:"cluster_completeness"`
	OverallCompleteness float64 `json:"overall_completeness"`
}

// WavesUnlockedPayload is the payload for EventWavesUnlocked.
type WavesUnlockedPayload struct {
	UnlockedWaveIDs []string `json:"unlocked_wave_ids"`
}

// NextGenWavesAddedPayload is the payload for EventNextGenWavesAdded.
type NextGenWavesAddedPayload struct {
	ClusterName string      `json:"cluster_name"`
	Waves       []WaveState `json:"waves"`
}

// ADRGeneratedPayload is the payload for EventADRGenerated.
type ADRGeneratedPayload struct {
	ADRID string `json:"adr_id"`
	Title string `json:"title"`
}

// ReadyLabelsAppliedPayload is the payload for EventReadyLabelsApplied.
type ReadyLabelsAppliedPayload struct {
	IssueIDs []string `json:"issue_ids"`
}

// SessionResumedPayload is the payload for EventSessionResumed.
type SessionResumedPayload struct {
	OriginalSessionID string `json:"original_session_id"`
}

// SessionRescannedPayload is the payload for EventSessionRescanned.
type SessionRescannedPayload struct {
	OriginalSessionID string `json:"original_session_id"`
}
