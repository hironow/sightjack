package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid" // nosemgrep: domain-imports-usecase-go -- uuid is stdlib-equivalent, not a usecase layer import [permanent]
)

// EventApplier applies domain events to update materialized projections. // nosemgrep: sql-in-domain-go -- comment text triggers rule but contains no SQL; domain purity enforced by architecture [permanent]
// Note: context is intentionally absent — domain types must remain pure.
// Use ContextEventApplier (usecase/port) when caller context is required.
type EventApplier interface {
	Apply(event Event) error
	Rebuild(events []Event) error
	Serialize() ([]byte, error)
	Deserialize(data []byte) error
}

// EventType identifies the kind of domain event.
type EventType string

const (
	EventSessionStarted      EventType = "session.started"
	EventScanCompleted       EventType = "scan.completed"
	EventWavesGenerated      EventType = "waves.generated"
	EventWaveApproved        EventType = "wave.approved"
	EventWaveRejected        EventType = "wave.rejected"
	EventWaveModified        EventType = "wave.modified"
	EventWaveApplied         EventType = "wave.applied"
	EventWaveCompleted       EventType = "wave.completed"
	EventCompletenessUpdated EventType = "completeness.updated"
	EventWavesUnlocked       EventType = "waves.unlocked"
	EventNextGenWavesAdded   EventType = "nextgen.waves.added"
	EventADRGenerated        EventType = "adr.generated"
	EventReadyLabelsApplied  EventType = "ready.labels.applied"
	EventSessionResumed      EventType = "session.resumed"
	EventSessionRescanned    EventType = "session.rescanned"
	EventSpecificationSent   EventType = "specification.sent"
	EventReportSent          EventType = "report.sent"
	EventFeedbackSent        EventType = "feedback.sent"
	EventFeedbackReceived    EventType = "feedback.received"
	EventWaveStalled         EventType = "wave.stalled"
	EventSystemCutover       EventType = "system.cutover"
)

// validEventTypes is the set of recognized EventType values.
var validEventTypes = map[EventType]bool{
	EventSessionStarted:      true,
	EventScanCompleted:       true,
	EventWavesGenerated:      true,
	EventWaveApproved:        true,
	EventWaveRejected:        true,
	EventWaveModified:        true,
	EventWaveApplied:         true,
	EventWaveCompleted:       true,
	EventCompletenessUpdated: true,
	EventWavesUnlocked:       true,
	EventNextGenWavesAdded:   true,
	EventADRGenerated:        true,
	EventReadyLabelsApplied:  true,
	EventSessionResumed:      true,
	EventSessionRescanned:    true,
	EventSpecificationSent:   true,
	EventReportSent:          true,
	EventFeedbackSent:        true,
	EventFeedbackReceived:    true,
	EventSystemCutover:       true,
	EventWaveStalled:         true,
}

// ValidEventType returns true if the given EventType is recognized.
func ValidEventType(t EventType) bool {
	return validEventTypes[t]
}

// AllValidEventTypes returns a copy of the canonical event type set (for testing).
func AllValidEventTypes() map[EventType]bool {
	cp := make(map[EventType]bool, len(validEventTypes))
	for k, v := range validEventTypes {
		cp[k] = v
	}
	return cp
}

// CurrentEventSchemaVersion is the schema version stamped on all new events.
// Legacy events (pre-Phase2) will have SchemaVersion 0 when deserialized.
const CurrentEventSchemaVersion uint8 = 1

// Event is the immutable event envelope persisted to the event store.
type Event struct {
	SchemaVersion uint8           `json:"schema_version,omitempty"`
	ID            string          `json:"id"`
	Type          EventType       `json:"type"`
	Timestamp     time.Time       `json:"timestamp"`
	Data          json.RawMessage `json:"data"`
	SessionID     string          `json:"session_id,omitempty"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	CausationID   string          `json:"causation_id,omitempty"`
	AggregateID   string          `json:"aggregate_id,omitempty"`
	AggregateType string          `json:"aggregate_type,omitempty"`
	SeqNr         uint64          `json:"seq_nr,omitempty"`
}

// NewEvent creates an Event with a UUID, the given timestamp, and marshaled data payload.
func NewEvent(eventType EventType, data any, timestamp time.Time) (Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Event{}, fmt.Errorf("marshal event data: %w", err)
	}
	return Event{
		SchemaVersion: CurrentEventSchemaVersion,
		ID:            uuid.NewString(),
		Type:          eventType,
		Timestamp:     timestamp,
		Data:          raw,
	}, nil
}

// ValidateEvent checks structural validity of an Event before persistence.
func ValidateEvent(e Event) error { // nosemgrep: validate-returns-error-only-go -- Event is a fully-typed struct; structural guard before persistence, parse pattern not applicable [permanent]
	var errs []string
	if e.SchemaVersion > CurrentEventSchemaVersion {
		errs = append(errs, fmt.Sprintf("SchemaVersion %d exceeds current %d", e.SchemaVersion, CurrentEventSchemaVersion))
	}
	if e.ID == "" {
		errs = append(errs, "ID is required")
	}
	if e.Type == "" {
		errs = append(errs, "Type is required")
	} else if !ValidEventType(e.Type) {
		errs = append(errs, fmt.Sprintf("Type %q is not a recognized event type", e.Type))
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
type ScanCompletedPayload struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go -- JSON wire format; FCC wrapping would break event-log compat [permanent]
	Clusters       []ClusterState `json:"clusters"`
	Completeness   float64        `json:"completeness"`
	ShibitoCount   int            `json:"shibito_count"`
	ScanResultPath string         `json:"scan_result_path"`
	LastScanned    time.Time      `json:"last_scanned"`
}

// WavesGeneratedPayload is the payload for EventWavesGenerated.
type WavesGeneratedPayload struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go -- JSON wire format; FCC wrapping would break event-log compat [permanent]
	Waves []WaveState `json:"waves"`
}

// WaveIdentityPayload is a shared payload for events that reference a single wave.
// Used by: EventWaveApproved, EventWaveRejected, EventSpecificationSent, EventReportSent.
type WaveIdentityPayload struct { // nosemgrep: domain-primitives.public-string-field-go -- JSON wire format; custom marshal would break event-log compat [permanent]
	WaveID      string `json:"wave_id"`
	ClusterName string `json:"cluster_name"`
}

// WaveModifiedPayload is the payload for EventWaveModified.
type WaveModifiedPayload struct { // nosemgrep: domain-primitives.public-string-field-go -- JSON wire format [permanent]
	WaveID      string    `json:"wave_id"`
	ClusterName string    `json:"cluster_name"`
	UpdatedWave WaveState `json:"updated_wave"`
}

// WaveAppliedPayload is the payload for EventWaveApplied.
type WaveAppliedPayload struct { // nosemgrep: domain-primitives.public-string-field-go,first-class-collection.raw-slice-field-domain-go -- JSON wire format; FCC wrapping would break event-log compat [permanent]
	WaveID      string   `json:"wave_id"`
	ClusterName string   `json:"cluster_name"`
	Applied     int      `json:"applied"`
	TotalCount  int      `json:"total_count"`
	Errors      []string `json:"errors,omitempty"`
}

// WaveCompletedPayload is the payload for EventWaveCompleted.
type WaveCompletedPayload struct { // nosemgrep: domain-primitives.public-string-field-go -- JSON wire format [permanent]
	WaveID      string `json:"wave_id"`
	ClusterName string `json:"cluster_name"`
	Applied     int    `json:"applied"`
	TotalCount  int    `json:"total_count"`
}

// CompletenessUpdatedPayload is the payload for EventCompletenessUpdated.
type CompletenessUpdatedPayload struct { // nosemgrep: domain-primitives.public-string-field-go -- JSON wire format [permanent]
	ClusterName         string  `json:"cluster_name"`
	ClusterCompleteness float64 `json:"cluster_completeness"`
	OverallCompleteness float64 `json:"overall_completeness"`
}

// WavesUnlockedPayload is the payload for EventWavesUnlocked.
type WavesUnlockedPayload struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go -- JSON wire format; FCC wrapping would break event-log compat [permanent]
	UnlockedWaveIDs []string `json:"unlocked_wave_ids"`
}

// NextGenWavesAddedPayload is the payload for EventNextGenWavesAdded.
type NextGenWavesAddedPayload struct { // nosemgrep: domain-primitives.public-string-field-go,first-class-collection.raw-slice-field-domain-go -- JSON wire format; FCC wrapping would break event-log compat [permanent]
	ClusterName string      `json:"cluster_name"`
	Waves       []WaveState `json:"waves"`
}

// ADRGeneratedPayload is the payload for EventADRGenerated.
type ADRGeneratedPayload struct {
	ADRID string `json:"adr_id"`
	Title string `json:"title"`
}

// ReadyLabelsAppliedPayload is the payload for EventReadyLabelsApplied.
type ReadyLabelsAppliedPayload struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go -- JSON wire format; FCC wrapping would break event-log compat [permanent]
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

// FeedbackReceivedPayload is the payload for EventFeedbackReceived.
type FeedbackReceivedPayload struct {
	Kind  string `json:"kind"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// --- Policies ---

// Policy represents an implicit reactive rule: WHEN [EVENT] THEN [COMMAND].
// See ADR S0014 for the POLICY pattern reference.
type Policy struct {
	Name    string    // unique identifier for the policy
	Trigger EventType // domain event that activates this policy
	Action  string    // description of the resulting command
}

// Policies registers all known implicit policies in sightjack.
// These document the existing reactive behaviors for future automation.
var Policies = []Policy{
	{Name: "WaveAppliedComposeReport", Trigger: EventWaveApplied, Action: "ComposeReport"},
	{Name: "ReportSentDeliverToPhonewave", Trigger: EventReportSent, Action: "DeliverViaPhonewave"},
	{Name: "ScanCompletedGenerateWaves", Trigger: EventScanCompleted, Action: "GenerateWaves"},
	{Name: "WaveCompletedNextGen", Trigger: EventWaveCompleted, Action: "GenerateNextWaves"},
	{Name: "SpecificationSentDeliverToPhonewave", Trigger: EventSpecificationSent, Action: "DeliverViaPhonewave"},
}
