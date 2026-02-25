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

func TestPayload_SessionStarted_RoundTrip(t *testing.T) {
	// given
	payload := sightjack.SessionStartedPayload{
		Project:         "my-project",
		StrictnessLevel: "fog",
	}
	event, err := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, payload)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}

	// when
	var decoded sightjack.SessionStartedPayload
	if err := sightjack.UnmarshalEventPayload(event, &decoded); err != nil {
		t.Fatalf("UnmarshalEventPayload: %v", err)
	}

	// then
	if decoded.Project != "my-project" {
		t.Errorf("expected my-project, got %s", decoded.Project)
	}
	if decoded.StrictnessLevel != "fog" {
		t.Errorf("expected fog, got %s", decoded.StrictnessLevel)
	}
}

func TestPayload_ScanCompleted_RoundTrip(t *testing.T) {
	// given
	payload := sightjack.ScanCompletedPayload{
		Clusters: []sightjack.ClusterState{
			{Name: "Auth", Completeness: 0.5, IssueCount: 3},
		},
		Completeness:   0.5,
		ShibitoCount:   2,
		ScanResultPath: "/path/to/scan.json",
		LastScanned:    time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC),
	}
	event, _ := sightjack.NewEvent(sightjack.EventScanCompleted, "s1", 2, payload)

	// when
	var decoded sightjack.ScanCompletedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	// then
	if len(decoded.Clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(decoded.Clusters))
	}
	if decoded.Clusters[0].Name != "Auth" {
		t.Errorf("expected Auth, got %s", decoded.Clusters[0].Name)
	}
	if decoded.ScanResultPath != "/path/to/scan.json" {
		t.Errorf("expected path, got %s", decoded.ScanResultPath)
	}
}

func TestPayload_WavesGenerated_RoundTrip(t *testing.T) {
	// given
	payload := sightjack.WavesGeneratedPayload{
		Waves: []sightjack.WaveState{
			{ID: "w1", ClusterName: "Auth", Title: "First", Status: "available", ActionCount: 2},
		},
	}
	event, _ := sightjack.NewEvent(sightjack.EventWavesGenerated, "s1", 3, payload)

	// when
	var decoded sightjack.WavesGeneratedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	// then
	if len(decoded.Waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(decoded.Waves))
	}
	if decoded.Waves[0].ID != "w1" {
		t.Errorf("expected w1, got %s", decoded.Waves[0].ID)
	}
}

func TestPayload_WaveApproved_RoundTrip(t *testing.T) {
	payload := sightjack.WaveIdentityPayload{WaveID: "w1", ClusterName: "Auth"}
	event, _ := sightjack.NewEvent(sightjack.EventWaveApproved, "s1", 4, payload)

	var decoded sightjack.WaveIdentityPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if decoded.WaveID != "w1" || decoded.ClusterName != "Auth" {
		t.Errorf("unexpected payload: %+v", decoded)
	}
}

func TestPayload_WaveCompleted_RoundTrip(t *testing.T) {
	payload := sightjack.WaveCompletedPayload{
		WaveID:      "w1",
		ClusterName: "Auth",
		Applied:     3,
		TotalCount:  3,
	}
	event, _ := sightjack.NewEvent(sightjack.EventWaveCompleted, "s1", 5, payload)

	var decoded sightjack.WaveCompletedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if decoded.Applied != 3 {
		t.Errorf("expected 3 applied, got %d", decoded.Applied)
	}
}

func TestPayload_CompletenessUpdated_RoundTrip(t *testing.T) {
	payload := sightjack.CompletenessUpdatedPayload{
		ClusterName:         "Auth",
		ClusterCompleteness: 0.75,
		OverallCompleteness: 0.60,
	}
	event, _ := sightjack.NewEvent(sightjack.EventCompletenessUpdated, "s1", 6, payload)

	var decoded sightjack.CompletenessUpdatedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if decoded.ClusterCompleteness != 0.75 {
		t.Errorf("expected 0.75, got %f", decoded.ClusterCompleteness)
	}
}

func TestPayload_NextGenWavesAdded_RoundTrip(t *testing.T) {
	payload := sightjack.NextGenWavesAddedPayload{
		ClusterName: "Auth",
		Waves: []sightjack.WaveState{
			{ID: "w2", ClusterName: "Auth", Title: "Second", Status: "available"},
		},
	}
	event, _ := sightjack.NewEvent(sightjack.EventNextGenWavesAdded, "s1", 7, payload)

	var decoded sightjack.NextGenWavesAddedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if decoded.ClusterName != "Auth" || len(decoded.Waves) != 1 {
		t.Errorf("unexpected: %+v", decoded)
	}
}

func TestPayload_WaveModified_RoundTrip(t *testing.T) {
	payload := sightjack.WaveModifiedPayload{
		WaveID:      "w1",
		ClusterName: "Auth",
		UpdatedWave: sightjack.WaveState{
			ID: "w1", ClusterName: "Auth", Title: "Modified", Status: "available",
		},
	}
	event, _ := sightjack.NewEvent(sightjack.EventWaveModified, "s1", 8, payload)

	var decoded sightjack.WaveModifiedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if decoded.UpdatedWave.Title != "Modified" {
		t.Errorf("expected Modified, got %s", decoded.UpdatedWave.Title)
	}
}

func TestPayload_ADRGenerated_RoundTrip(t *testing.T) {
	payload := sightjack.ADRGeneratedPayload{ADRID: "0008", Title: "Event Sourcing"}
	event, _ := sightjack.NewEvent(sightjack.EventADRGenerated, "s1", 9, payload)

	var decoded sightjack.ADRGeneratedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if decoded.ADRID != "0008" {
		t.Errorf("expected 0008, got %s", decoded.ADRID)
	}
}

func TestPayload_WavesUnlocked_RoundTrip(t *testing.T) {
	payload := sightjack.WavesUnlockedPayload{
		UnlockedWaveIDs: []string{"Auth:w2", "Auth:w3"},
	}
	event, _ := sightjack.NewEvent(sightjack.EventWavesUnlocked, "s1", 10, payload)

	var decoded sightjack.WavesUnlockedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if len(decoded.UnlockedWaveIDs) != 2 {
		t.Errorf("expected 2 unlocked, got %d", len(decoded.UnlockedWaveIDs))
	}
}

func TestPayload_ReadyLabelsApplied_RoundTrip(t *testing.T) {
	payload := sightjack.ReadyLabelsAppliedPayload{IssueIDs: []string{"ENG-101", "ENG-102"}}
	event, _ := sightjack.NewEvent(sightjack.EventReadyLabelsApplied, "s1", 11, payload)

	var decoded sightjack.ReadyLabelsAppliedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if len(decoded.IssueIDs) != 2 {
		t.Errorf("expected 2 issues, got %d", len(decoded.IssueIDs))
	}
}

func TestPayload_WaveApplied_RoundTrip(t *testing.T) {
	payload := sightjack.WaveAppliedPayload{
		WaveID:      "w1",
		ClusterName: "Auth",
		Applied:     2,
		TotalCount:  3,
		Errors:      []string{"action 3 failed"},
	}
	event, _ := sightjack.NewEvent(sightjack.EventWaveApplied, "s1", 12, payload)

	var decoded sightjack.WaveAppliedPayload
	sightjack.UnmarshalEventPayload(event, &decoded)

	if decoded.Applied != 2 || len(decoded.Errors) != 1 {
		t.Errorf("unexpected: %+v", decoded)
	}
}
