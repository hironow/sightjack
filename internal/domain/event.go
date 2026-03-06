package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EventApplier applies domain events to update materialized projections.
type EventApplier interface {
	Apply(event Event) error
	Rebuild(events []Event) error
}

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
	EventFeedbackSent        EventType = "feedback_sent"
)

// Event is the immutable event envelope persisted to the event store.
type Event struct {
	ID            string          `json:"id"`
	Type          EventType       `json:"type"`
	Timestamp     time.Time       `json:"timestamp"`
	Data          json.RawMessage `json:"data"`
	SessionID     string          `json:"session_id"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	CausationID   string          `json:"causation_id,omitempty"`
}

// NewEvent creates an Event with a UUID, the given timestamp, and marshaled data payload.
func NewEvent(eventType EventType, data any, timestamp time.Time) (Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Event{}, fmt.Errorf("marshal event data: %w", err)
	}
	return Event{
		ID:        uuid.NewString(),
		Type:      eventType,
		Timestamp: timestamp,
		Data:      raw,
	}, nil
}

// ValidateEvent checks structural validity of an Event before persistence.
func ValidateEvent(e Event) error {
	var errs []string
	if e.ID == "" {
		errs = append(errs, "ID is required")
	}
	if e.Type == "" {
		errs = append(errs, "Type is required")
	}
	if e.Timestamp.IsZero() {
		errs = append(errs, "Timestamp must not be zero")
	}
	if len(e.Data) == 0 {
		errs = append(errs, "Data must not be empty")
	}
	if len(errs) > 0 {
		return errors.New("invalid event: " + strings.Join(errs, "; "))
	}
	return nil
}

// AppendResult captures metrics from an event store Append operation.
type AppendResult struct {
	BytesWritten int // total bytes written to event files
}

// LoadResult captures metrics from an event store Load operation.
type LoadResult struct {
	FileCount        int // number of .jsonl files scanned
	CorruptLineCount int // number of lines skipped due to parse errors
}

// MarshalEvent serializes an Event to compact JSON (no trailing newline).
func MarshalEvent(e Event) ([]byte, error) {
	return json.Marshal(e)
}

// UnmarshalEventPayload deserializes the Data field into the given target.
func UnmarshalEventPayload(e Event, target any) error {
	return json.Unmarshal(e.Data, target)
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
