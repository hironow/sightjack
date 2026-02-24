package sightjack_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hironow/sightjack"
)

func TestNewEvent_SetsAllFields(t *testing.T) {
	// given
	sessionID := "session-123"
	eventType := sightjack.EventSessionStarted
	payload := map[string]string{"project": "test"}

	// when
	event, err := sightjack.NewEvent(eventType, sessionID, 1, payload)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.SchemaVersion != sightjack.EventSchemaVersion {
		t.Errorf("expected schema version %s, got %s", sightjack.EventSchemaVersion, event.SchemaVersion)
	}
	if event.Type != sightjack.EventSessionStarted {
		t.Errorf("expected type %s, got %s", sightjack.EventSessionStarted, event.Type)
	}
	if event.SessionID != sessionID {
		t.Errorf("expected session ID %s, got %s", sessionID, event.SessionID)
	}
	if event.Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", event.Sequence)
	}
	if event.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if len(event.PayloadRaw) == 0 {
		t.Error("expected non-empty PayloadRaw")
	}
}

func TestMarshalEvent_JSONRoundTrip(t *testing.T) {
	// given
	payload := map[string]string{"project": "my-project"}
	event, err := sightjack.NewEvent(sightjack.EventSessionStarted, "session-1", 1, payload)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}

	// when
	data, err := sightjack.MarshalEvent(event)
	if err != nil {
		t.Fatalf("MarshalEvent: %v", err)
	}

	// then: should be valid JSON
	if !json.Valid(data) {
		t.Fatalf("invalid JSON: %s", string(data))
	}

	// round-trip: unmarshal back
	var decoded sightjack.Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Type != sightjack.EventSessionStarted {
		t.Errorf("expected type %s, got %s", sightjack.EventSessionStarted, decoded.Type)
	}
	if decoded.SessionID != "session-1" {
		t.Errorf("expected session-1, got %s", decoded.SessionID)
	}
	if decoded.Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", decoded.Sequence)
	}

	// Verify CorrelationID and CausationID fields are present in JSON
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}
	if _, ok := raw["correlation_id"]; !ok {
		t.Error("expected correlation_id field in JSON")
	}
	if _, ok := raw["causation_id"]; !ok {
		t.Error("expected causation_id field in JSON")
	}
}

func TestMarshalEvent_NoTrailingNewline(t *testing.T) {
	// given: JSONL format requires no trailing newline in the marshaled bytes
	event, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, nil)

	// when
	data, err := sightjack.MarshalEvent(event)

	// then
	if err != nil {
		t.Fatalf("MarshalEvent: %v", err)
	}
	if len(data) > 0 && data[len(data)-1] == '\n' {
		t.Error("MarshalEvent should not include trailing newline")
	}
}

func TestNewEvent_NilPayload(t *testing.T) {
	// given/when
	event, err := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, nil)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(event.PayloadRaw) != "null" {
		t.Errorf("expected null payload, got %s", string(event.PayloadRaw))
	}
}

func TestEvent_AllEventTypes_Defined(t *testing.T) {
	// Verify all 17 event types are defined and distinct
	types := []sightjack.EventType{
		sightjack.EventSessionStarted,
		sightjack.EventScanCompleted,
		sightjack.EventWavesGenerated,
		sightjack.EventWaveApproved,
		sightjack.EventWaveRejected,
		sightjack.EventWaveModified,
		sightjack.EventWaveApplied,
		sightjack.EventWaveCompleted,
		sightjack.EventCompletenessUpdated,
		sightjack.EventWavesUnlocked,
		sightjack.EventNextGenWavesAdded,
		sightjack.EventADRGenerated,
		sightjack.EventReadyLabelsApplied,
		sightjack.EventSessionResumed,
		sightjack.EventSessionRescanned,
		sightjack.EventSpecificationSent,
		sightjack.EventReportSent,
	}

	seen := make(map[sightjack.EventType]bool)
	for _, et := range types {
		if et == "" {
			t.Error("found empty EventType")
		}
		if seen[et] {
			t.Errorf("duplicate EventType: %s", et)
		}
		seen[et] = true
	}

	if len(types) != 17 {
		t.Errorf("expected 17 event types, got %d", len(types))
	}
}

func TestEvent_TimestampPreservedInJSON(t *testing.T) {
	// given
	event, _ := sightjack.NewEvent(sightjack.EventScanCompleted, "s1", 2, nil)
	originalTime := event.Timestamp

	// when
	data, _ := sightjack.MarshalEvent(event)
	var decoded sightjack.Event
	json.Unmarshal(data, &decoded)

	// then: timestamps should be equal within a second (JSON time precision)
	diff := originalTime.Sub(decoded.Timestamp)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("timestamp drift too large: %v", diff)
	}
}

func TestValidateEvent_Valid(t *testing.T) {
	// given
	event, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, map[string]string{"k": "v"})

	// when
	err := sightjack.ValidateEvent(event)

	// then
	if err != nil {
		t.Errorf("expected nil error for valid event, got: %v", err)
	}
}

func TestValidateEvent_EmptyType(t *testing.T) {
	// given
	event, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, "data")
	event.Type = ""

	// when
	err := sightjack.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for empty type")
	}
}

func TestValidateEvent_EmptySessionID(t *testing.T) {
	// given
	event, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, "data")
	event.SessionID = ""

	// when
	err := sightjack.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for empty session_id")
	}
}

func TestValidateEvent_ZeroSequence(t *testing.T) {
	// given
	event, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, "data")
	event.Sequence = 0

	// when
	err := sightjack.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for zero sequence")
	}
}

func TestValidateEvent_ZeroTimestamp(t *testing.T) {
	// given
	event, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, "data")
	event.Timestamp = time.Time{}

	// when
	err := sightjack.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for zero timestamp")
	}
}

func TestValidateEvent_EmptyPayload(t *testing.T) {
	// given
	event, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, "data")
	event.PayloadRaw = nil

	// when
	err := sightjack.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for empty payload")
	}
}

func TestEvent_UnknownType_Tolerance(t *testing.T) {
	// given: JSON with an unknown event type should still unmarshal
	raw := `{"schema_version":"1","type":"future_event","timestamp":"2026-01-01T00:00:00Z","session_id":"s1","sequence":1,"payload":{"foo":"bar"}}`

	// when
	var event sightjack.Event
	err := json.Unmarshal([]byte(raw), &event)

	// then
	if err != nil {
		t.Fatalf("expected no error for unknown type, got: %v", err)
	}
	if event.Type != "future_event" {
		t.Errorf("expected future_event, got %s", event.Type)
	}
	if len(event.PayloadRaw) == 0 {
		t.Error("expected preserved payload")
	}
}
