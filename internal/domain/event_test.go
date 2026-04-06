package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

func TestNewEvent_SetsAllFields(t *testing.T) {
	// given
	eventType := domain.EventSessionStartedV2
	payload := map[string]string{"project": "test"}
	now := time.Now()

	// when
	event, err := domain.NewEvent(eventType, payload, now)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.ID == "" {
		t.Error("expected non-empty UUID ID")
	}
	if event.Type != domain.EventSessionStartedV2 {
		t.Errorf("expected type %s, got %s", domain.EventSessionStartedV2, event.Type)
	}
	if !event.Timestamp.Equal(now) {
		t.Errorf("expected timestamp %v, got %v", now, event.Timestamp)
	}
	if len(event.Data) == 0 {
		t.Error("expected non-empty Data")
	}
}

func TestNewEvent_GeneratesUUID(t *testing.T) {
	// given/when
	e1, _ := domain.NewEvent(domain.EventSessionStartedV2, nil, time.Now())
	e2, _ := domain.NewEvent(domain.EventSessionStartedV2, nil, time.Now())

	// then: each event gets a unique UUID
	if e1.ID == e2.ID {
		t.Errorf("expected unique IDs, both got %s", e1.ID)
	}
	if len(e1.ID) != 36 { // UUID v4 format: 8-4-4-4-12
		t.Errorf("expected UUID format (36 chars), got %d chars: %s", len(e1.ID), e1.ID)
	}
}

func TestMarshalEvent_JSONRoundTrip(t *testing.T) {
	// given
	payload := map[string]string{"project": "my-project"}
	now := time.Now()
	event, err := domain.NewEvent(domain.EventSessionStartedV2, payload, now)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	event.SessionID = "session-1"

	// when
	data, err := domain.MarshalEvent(event)
	if err != nil {
		t.Fatalf("MarshalEvent: %v", err)
	}

	// then: should be valid JSON
	if !json.Valid(data) {
		t.Fatalf("invalid JSON: %s", string(data))
	}

	// round-trip: unmarshal back
	var decoded domain.Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Type != domain.EventSessionStartedV2 {
		t.Errorf("expected type %s, got %s", domain.EventSessionStartedV2, decoded.Type)
	}
	if decoded.SessionID != "session-1" {
		t.Errorf("expected session-1, got %s", decoded.SessionID)
	}
	if decoded.ID != event.ID {
		t.Errorf("expected ID %s, got %s", event.ID, decoded.ID)
	}
}

func TestMarshalEvent_NoTrailingNewline(t *testing.T) {
	// given: JSONL format requires no trailing newline in the marshaled bytes
	event, _ := domain.NewEvent(domain.EventSessionStartedV2, nil, time.Now())

	// when
	data, err := domain.MarshalEvent(event)

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
	event, err := domain.NewEvent(domain.EventSessionStartedV2, nil, time.Now())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(event.Data) != "null" {
		t.Errorf("expected null payload, got %s", string(event.Data))
	}
}

func TestEvent_AllEventTypes_Defined(t *testing.T) {
	// Verify all 17 event types are defined and distinct
	types := []domain.EventType{
		domain.EventSessionStartedV2,
		domain.EventScanCompletedV2,
		domain.EventWavesGeneratedV2,
		domain.EventWaveApprovedV2,
		domain.EventWaveRejectedV2,
		domain.EventWaveModifiedV2,
		domain.EventWaveAppliedV2,
		domain.EventWaveCompletedV2,
		domain.EventCompletenessUpdatedV2,
		domain.EventWavesUnlockedV2,
		domain.EventNextGenWavesAddedV2,
		domain.EventADRGeneratedV2,
		domain.EventReadyLabelsAppliedV2,
		domain.EventSessionResumedV2,
		domain.EventSessionRescannedV2,
		domain.EventSpecificationSentV2,
		domain.EventReportSentV2,
	}

	seen := make(map[domain.EventType]bool)
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
	now := time.Now()
	event, _ := domain.NewEvent(domain.EventScanCompletedV2, nil, now)

	// when
	data, _ := domain.MarshalEvent(event)
	var decoded domain.Event
	json.Unmarshal(data, &decoded)

	// then: timestamps should be equal within a second (JSON time precision)
	diff := now.Sub(decoded.Timestamp)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("timestamp drift too large: %v", diff)
	}
}

func TestEvent_SchemaVersion_SetByNewEvent(t *testing.T) {
	ev, err := domain.NewEvent("test.event", map[string]string{"k": "v"}, time.Now())
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if ev.SchemaVersion != domain.CurrentEventSchemaVersion {
		t.Errorf("got %d, want %d", ev.SchemaVersion, domain.CurrentEventSchemaVersion)
	}
}

func TestEvent_SchemaVersion_ZeroIsLegacy(t *testing.T) {
	raw := `{"id":"abc","type":"test","timestamp":"2026-01-01T00:00:00Z","session_id":"s1","data":{}}`
	var ev domain.Event
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if ev.SchemaVersion != 0 {
		t.Errorf("legacy event should have SchemaVersion 0, got %d", ev.SchemaVersion)
	}
}

func TestValidateEvent_RejectsFutureSchema(t *testing.T) {
	ev, err := domain.NewEvent("test.event", map[string]string{"k": "v"}, time.Now())
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	ev.SchemaVersion = domain.CurrentEventSchemaVersion + 1
	if err := domain.ValidateEvent(ev); err == nil {
		t.Error("expected error for future schema version")
	}
}

func TestValidateEvent_Valid(t *testing.T) {
	// given
	event, _ := domain.NewEvent(domain.EventSessionStartedV2, map[string]string{"k": "v"}, time.Now())

	// when
	err := domain.ValidateEvent(event)

	// then
	if err != nil {
		t.Errorf("expected nil error for valid event, got: %v", err)
	}
}

func TestValidateEvent_EmptyType(t *testing.T) {
	// given
	event, _ := domain.NewEvent(domain.EventSessionStartedV2, "data", time.Now())
	event.Type = ""

	// when
	err := domain.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for empty type")
	}
}

func TestValidateEvent_EmptyID(t *testing.T) {
	// given
	event, _ := domain.NewEvent(domain.EventSessionStartedV2, "data", time.Now())
	event.ID = ""

	// when
	err := domain.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestValidateEvent_ZeroTimestamp(t *testing.T) {
	// given
	event, _ := domain.NewEvent(domain.EventSessionStartedV2, "data", time.Now())
	event.Timestamp = time.Time{}

	// when
	err := domain.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for zero timestamp")
	}
}

func TestValidateEvent_EmptyData(t *testing.T) {
	// given
	event, _ := domain.NewEvent(domain.EventSessionStartedV2, "data", time.Now())
	event.Data = nil

	// when
	err := domain.ValidateEvent(event)

	// then
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestEvent_UnknownType_Tolerance(t *testing.T) {
	// given: JSON with an unknown event type should still unmarshal
	raw := `{"id":"test-uuid","type":"future_event","timestamp":"2026-01-01T00:00:00Z","session_id":"s1","data":{"foo":"bar"}}`

	// when
	var event domain.Event
	err := json.Unmarshal([]byte(raw), &event)

	// then
	if err != nil {
		t.Fatalf("expected no error for unknown type, got: %v", err)
	}
	if event.Type != "future_event" {
		t.Errorf("expected future_event, got %s", event.Type)
	}
	if len(event.Data) == 0 {
		t.Error("expected preserved data")
	}
}

func TestPayload_SessionStarted_RoundTrip(t *testing.T) {
	// given
	payload := domain.SessionStartedPayload{
		Project:         "my-project",
		StrictnessLevel: "fog",
	}
	event, err := domain.NewEvent(domain.EventSessionStartedV2, payload, time.Now())
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}

	// when
	var decoded domain.SessionStartedPayload
	if err := domain.UnmarshalEventPayload(event, &decoded); err != nil {
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
	payload := domain.ScanCompletedPayload{
		Clusters: []domain.ClusterState{
			{Name: "Auth", Completeness: 0.5, IssueCount: 3},
		},
		Completeness:   0.5,
		ShibitoCount:   2,
		ScanResultPath: "/path/to/scan.json",
		LastScanned:    time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC),
	}
	event, _ := domain.NewEvent(domain.EventScanCompletedV2, payload, time.Now())

	// when
	var decoded domain.ScanCompletedPayload
	domain.UnmarshalEventPayload(event, &decoded)

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
	payload := domain.WavesGeneratedPayload{
		Waves: []domain.WaveState{
			{ID: "w1", ClusterName: "Auth", Title: "First", Status: "available", ActionCount: 2},
		},
	}
	event, _ := domain.NewEvent(domain.EventWavesGeneratedV2, payload, time.Now())

	// when
	var decoded domain.WavesGeneratedPayload
	domain.UnmarshalEventPayload(event, &decoded)

	// then
	if len(decoded.Waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(decoded.Waves))
	}
	if decoded.Waves[0].ID != "w1" {
		t.Errorf("expected w1, got %s", decoded.Waves[0].ID)
	}
}

func TestPayload_WaveApproved_RoundTrip(t *testing.T) {
	payload := domain.WaveIdentityPayload{WaveID: "w1", ClusterName: "Auth"}
	event, _ := domain.NewEvent(domain.EventWaveApprovedV2, payload, time.Now())

	var decoded domain.WaveIdentityPayload
	domain.UnmarshalEventPayload(event, &decoded)

	if decoded.WaveID != "w1" || decoded.ClusterName != "Auth" {
		t.Errorf("unexpected payload: %+v", decoded)
	}
}

func TestPayload_WaveCompleted_RoundTrip(t *testing.T) {
	payload := domain.WaveCompletedPayload{
		WaveID:      "w1",
		ClusterName: "Auth",
		Applied:     3,
		TotalCount:  3,
	}
	event, _ := domain.NewEvent(domain.EventWaveCompletedV2, payload, time.Now())

	var decoded domain.WaveCompletedPayload
	domain.UnmarshalEventPayload(event, &decoded)

	if decoded.Applied != 3 {
		t.Errorf("expected 3 applied, got %d", decoded.Applied)
	}
}

func TestPayload_CompletenessUpdated_RoundTrip(t *testing.T) {
	payload := domain.CompletenessUpdatedPayload{
		ClusterName:         "Auth",
		ClusterCompleteness: 0.75,
		OverallCompleteness: 0.60,
	}
	event, _ := domain.NewEvent(domain.EventCompletenessUpdatedV2, payload, time.Now())

	var decoded domain.CompletenessUpdatedPayload
	domain.UnmarshalEventPayload(event, &decoded)

	if decoded.ClusterCompleteness != 0.75 {
		t.Errorf("expected 0.75, got %f", decoded.ClusterCompleteness)
	}
}

func TestPayload_NextGenWavesAdded_RoundTrip(t *testing.T) {
	payload := domain.NextGenWavesAddedPayload{
		ClusterName: "Auth",
		Waves: []domain.WaveState{
			{ID: "w2", ClusterName: "Auth", Title: "Second", Status: "available"},
		},
	}
	event, _ := domain.NewEvent(domain.EventNextGenWavesAddedV2, payload, time.Now())

	var decoded domain.NextGenWavesAddedPayload
	domain.UnmarshalEventPayload(event, &decoded)

	if decoded.ClusterName != "Auth" || len(decoded.Waves) != 1 {
		t.Errorf("unexpected: %+v", decoded)
	}
}

func TestPayload_WaveModified_RoundTrip(t *testing.T) {
	payload := domain.WaveModifiedPayload{
		WaveID:      "w1",
		ClusterName: "Auth",
		UpdatedWave: domain.WaveState{
			ID: "w1", ClusterName: "Auth", Title: "Modified", Status: "available",
		},
	}
	event, _ := domain.NewEvent(domain.EventWaveModifiedV2, payload, time.Now())

	var decoded domain.WaveModifiedPayload
	domain.UnmarshalEventPayload(event, &decoded)

	if decoded.UpdatedWave.Title != "Modified" {
		t.Errorf("expected Modified, got %s", decoded.UpdatedWave.Title)
	}
}

func TestPayload_ADRGenerated_RoundTrip(t *testing.T) {
	payload := domain.ADRGeneratedPayload{ADRID: "0008", Title: "Event Sourcing"}
	event, _ := domain.NewEvent(domain.EventADRGeneratedV2, payload, time.Now())

	var decoded domain.ADRGeneratedPayload
	domain.UnmarshalEventPayload(event, &decoded)

	if decoded.ADRID != "0008" {
		t.Errorf("expected 0008, got %s", decoded.ADRID)
	}
}

func TestPayload_WavesUnlocked_RoundTrip(t *testing.T) {
	payload := domain.WavesUnlockedPayload{
		UnlockedWaveIDs: []string{"Auth:w2", "Auth:w3"},
	}
	event, _ := domain.NewEvent(domain.EventWavesUnlockedV2, payload, time.Now())

	var decoded domain.WavesUnlockedPayload
	domain.UnmarshalEventPayload(event, &decoded)

	if len(decoded.UnlockedWaveIDs) != 2 {
		t.Errorf("expected 2 unlocked, got %d", len(decoded.UnlockedWaveIDs))
	}
}

func TestPayload_ReadyLabelsApplied_RoundTrip(t *testing.T) {
	payload := domain.ReadyLabelsAppliedPayload{IssueIDs: []string{"ENG-101", "ENG-102"}}
	event, _ := domain.NewEvent(domain.EventReadyLabelsAppliedV2, payload, time.Now())

	var decoded domain.ReadyLabelsAppliedPayload
	domain.UnmarshalEventPayload(event, &decoded)

	if len(decoded.IssueIDs) != 2 {
		t.Errorf("expected 2 issues, got %d", len(decoded.IssueIDs))
	}
}

func TestPayload_WaveApplied_RoundTrip(t *testing.T) {
	payload := domain.WaveAppliedPayload{
		WaveID:      "w1",
		ClusterName: "Auth",
		Applied:     2,
		TotalCount:  3,
		Errors:      []string{"action 3 failed"},
	}
	event, _ := domain.NewEvent(domain.EventWaveAppliedV2, payload, time.Now())

	var decoded domain.WaveAppliedPayload
	domain.UnmarshalEventPayload(event, &decoded)

	if decoded.Applied != 2 || len(decoded.Errors) != 1 {
		t.Errorf("unexpected: %+v", decoded)
	}
}
