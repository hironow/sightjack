package eventsource_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/eventsource"
)

func TestProjectState_SessionStarted(t *testing.T) {
	// given
	event := mustNewEvent(t, domain.EventSessionStarted, "s1", 1,
		domain.SessionStartedPayload{Project: "my-project", StrictnessLevel: "fog"})

	// when
	state := domain.ProjectState([]domain.Event{event})

	// then
	if state.Version != domain.StateFormatVersion {
		t.Errorf("expected version %s, got %s", domain.StateFormatVersion, state.Version)
	}
	if state.SessionID != "s1" {
		t.Errorf("expected s1, got %s", state.SessionID)
	}
	if state.Project != "my-project" {
		t.Errorf("expected my-project, got %s", state.Project)
	}
	if state.StrictnessLevel != "fog" {
		t.Errorf("expected fog, got %s", state.StrictnessLevel)
	}
}

func TestProjectState_ScanCompleted(t *testing.T) {
	// given
	events := []domain.Event{
		mustNewEvent(t, domain.EventSessionStarted, "s1", 1,
			domain.SessionStartedPayload{Project: "p1"}),
		mustNewEvent(t, domain.EventScanCompleted, "s1", 2,
			domain.ScanCompletedPayload{
				Clusters:       []domain.ClusterState{{Name: "Auth", Completeness: 0.5, IssueCount: 3}},
				Completeness:   0.5,
				ShibitoCount:   2,
				ScanResultPath: "/path/scan.json",
				LastScanned:    time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC),
			}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if state.Completeness != 0.5 {
		t.Errorf("expected 0.5, got %f", state.Completeness)
	}
	if len(state.Clusters) != 1 || state.Clusters[0].Name != "Auth" {
		t.Errorf("expected Auth cluster, got %v", state.Clusters)
	}
	if state.ShibitoCount != 2 {
		t.Errorf("expected 2, got %d", state.ShibitoCount)
	}
	if state.ScanResultPath != "/path/scan.json" {
		t.Errorf("expected path, got %s", state.ScanResultPath)
	}
}

func TestProjectState_WavesGenerated(t *testing.T) {
	// given
	events := []domain.Event{
		mustNewEvent(t, domain.EventSessionStarted, "s1", 1, nil),
		mustNewEvent(t, domain.EventWavesGenerated, "s1", 2,
			domain.WavesGeneratedPayload{
				Waves: []domain.WaveState{
					{ID: "w1", ClusterName: "Auth", Title: "First", Status: "available", ActionCount: 2},
					{ID: "w2", ClusterName: "Auth", Title: "Second", Status: "locked", ActionCount: 1},
				},
			}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if len(state.Waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(state.Waves))
	}
	if state.Waves[0].ID != "w1" {
		t.Errorf("expected w1, got %s", state.Waves[0].ID)
	}
}

func TestProjectState_WaveCompleted(t *testing.T) {
	// given
	events := []domain.Event{
		mustNewEvent(t, domain.EventSessionStarted, "s1", 1, nil),
		mustNewEvent(t, domain.EventWavesGenerated, "s1", 2,
			domain.WavesGeneratedPayload{
				Waves: []domain.WaveState{
					{ID: "w1", ClusterName: "Auth", Status: "available"},
				},
			}),
		mustNewEvent(t, domain.EventWaveCompleted, "s1", 3,
			domain.WaveCompletedPayload{WaveID: "w1", ClusterName: "Auth", Applied: 3, TotalCount: 3}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if len(state.Waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(state.Waves))
	}
	if state.Waves[0].Status != "completed" {
		t.Errorf("expected completed, got %s", state.Waves[0].Status)
	}
}

func TestProjectState_CompletenessUpdated(t *testing.T) {
	// given
	events := []domain.Event{
		mustNewEvent(t, domain.EventSessionStarted, "s1", 1, nil),
		mustNewEvent(t, domain.EventScanCompleted, "s1", 2,
			domain.ScanCompletedPayload{
				Clusters:     []domain.ClusterState{{Name: "Auth", Completeness: 0.3}},
				Completeness: 0.3,
			}),
		mustNewEvent(t, domain.EventCompletenessUpdated, "s1", 3,
			domain.CompletenessUpdatedPayload{ClusterName: "Auth", ClusterCompleteness: 0.7, OverallCompleteness: 0.7}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if state.Completeness != 0.7 {
		t.Errorf("expected 0.7, got %f", state.Completeness)
	}
	if state.Clusters[0].Completeness != 0.7 {
		t.Errorf("expected cluster 0.7, got %f", state.Clusters[0].Completeness)
	}
}

func TestProjectState_WavesUnlocked(t *testing.T) {
	// given
	events := []domain.Event{
		mustNewEvent(t, domain.EventSessionStarted, "s1", 1, nil),
		mustNewEvent(t, domain.EventWavesGenerated, "s1", 2,
			domain.WavesGeneratedPayload{
				Waves: []domain.WaveState{
					{ID: "w1", ClusterName: "Auth", Status: "completed"},
					{ID: "w2", ClusterName: "Auth", Status: "locked"},
				},
			}),
		mustNewEvent(t, domain.EventWavesUnlocked, "s1", 3,
			domain.WavesUnlockedPayload{UnlockedWaveIDs: []string{"Auth:w2"}}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if state.Waves[1].Status != "available" {
		t.Errorf("expected available after unlock, got %s", state.Waves[1].Status)
	}
}

func TestProjectState_NextGenWavesAdded(t *testing.T) {
	// given
	events := []domain.Event{
		mustNewEvent(t, domain.EventSessionStarted, "s1", 1, nil),
		mustNewEvent(t, domain.EventWavesGenerated, "s1", 2,
			domain.WavesGeneratedPayload{
				Waves: []domain.WaveState{{ID: "w1", ClusterName: "Auth"}},
			}),
		mustNewEvent(t, domain.EventNextGenWavesAdded, "s1", 3,
			domain.NextGenWavesAddedPayload{
				ClusterName: "Auth",
				Waves:       []domain.WaveState{{ID: "w2", ClusterName: "Auth"}},
			}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if len(state.Waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(state.Waves))
	}
}

func TestProjectState_WaveModified(t *testing.T) {
	// given
	events := []domain.Event{
		mustNewEvent(t, domain.EventSessionStarted, "s1", 1, nil),
		mustNewEvent(t, domain.EventWavesGenerated, "s1", 2,
			domain.WavesGeneratedPayload{
				Waves: []domain.WaveState{{ID: "w1", ClusterName: "Auth", Title: "Original"}},
			}),
		mustNewEvent(t, domain.EventWaveModified, "s1", 3,
			domain.WaveModifiedPayload{
				WaveID: "w1", ClusterName: "Auth",
				UpdatedWave: domain.WaveState{ID: "w1", ClusterName: "Auth", Title: "Modified"},
			}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if state.Waves[0].Title != "Modified" {
		t.Errorf("expected Modified, got %s", state.Waves[0].Title)
	}
}

func TestProjectState_ADRGenerated(t *testing.T) {
	// given
	events := []domain.Event{
		mustNewEvent(t, domain.EventSessionStarted, "s1", 1, nil),
		mustNewEvent(t, domain.EventADRGenerated, "s1", 2,
			domain.ADRGeneratedPayload{ADRID: "0008", Title: "Event Sourcing"}),
		mustNewEvent(t, domain.EventADRGenerated, "s1", 3,
			domain.ADRGeneratedPayload{ADRID: "0009", Title: "Another"}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if state.ADRCount != 2 {
		t.Errorf("expected 2, got %d", state.ADRCount)
	}
}

func TestProjectState_FullLifecycle(t *testing.T) {
	// given: a realistic event sequence
	events := []domain.Event{
		mustNewEvent(t, domain.EventSessionStarted, "s1", 1,
			domain.SessionStartedPayload{Project: "test-project", StrictnessLevel: "alert"}),
		mustNewEvent(t, domain.EventScanCompleted, "s1", 2,
			domain.ScanCompletedPayload{
				Clusters:       []domain.ClusterState{{Name: "Auth", Completeness: 0.3, IssueCount: 5}},
				Completeness:   0.3,
				ShibitoCount:   1,
				ScanResultPath: "/scan.json",
				LastScanned:    time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC),
			}),
		mustNewEvent(t, domain.EventWavesGenerated, "s1", 3,
			domain.WavesGeneratedPayload{
				Waves: []domain.WaveState{
					{ID: "w1", ClusterName: "Auth", Title: "First Wave", Status: "available", ActionCount: 3},
					{ID: "w2", ClusterName: "Auth", Title: "Second Wave", Status: "locked", ActionCount: 2},
				},
			}),
		mustNewEvent(t, domain.EventWaveApproved, "s1", 4,
			domain.WaveIdentityPayload{WaveID: "w1", ClusterName: "Auth"}),
		mustNewEvent(t, domain.EventWaveCompleted, "s1", 5,
			domain.WaveCompletedPayload{WaveID: "w1", ClusterName: "Auth", Applied: 3, TotalCount: 3}),
		mustNewEvent(t, domain.EventCompletenessUpdated, "s1", 6,
			domain.CompletenessUpdatedPayload{ClusterName: "Auth", ClusterCompleteness: 0.6, OverallCompleteness: 0.6}),
		mustNewEvent(t, domain.EventWavesUnlocked, "s1", 7,
			domain.WavesUnlockedPayload{UnlockedWaveIDs: []string{"Auth:w2"}}),
		mustNewEvent(t, domain.EventADRGenerated, "s1", 8,
			domain.ADRGeneratedPayload{ADRID: "0008", Title: "Event Sourcing"}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if state.Project != "test-project" {
		t.Errorf("project: got %s", state.Project)
	}
	if state.StrictnessLevel != "alert" {
		t.Errorf("strictness: got %s", state.StrictnessLevel)
	}
	if state.Completeness != 0.6 {
		t.Errorf("completeness: got %f", state.Completeness)
	}
	if state.Clusters[0].Completeness != 0.6 {
		t.Errorf("cluster completeness: got %f", state.Clusters[0].Completeness)
	}
	if len(state.Waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(state.Waves))
	}
	if state.Waves[0].Status != "completed" {
		t.Errorf("w1 status: got %s", state.Waves[0].Status)
	}
	if state.Waves[1].Status != "available" {
		t.Errorf("w2 status: got %s", state.Waves[1].Status)
	}
	if state.ADRCount != 1 {
		t.Errorf("ADRCount: got %d", state.ADRCount)
	}
	if state.ShibitoCount != 1 {
		t.Errorf("ShibitoCount: got %d", state.ShibitoCount)
	}
}

func TestProjectState_Idempotent(t *testing.T) {
	// given
	events := []domain.Event{
		mustNewEvent(t, domain.EventSessionStarted, "s1", 1,
			domain.SessionStartedPayload{Project: "p1"}),
		mustNewEvent(t, domain.EventScanCompleted, "s1", 2,
			domain.ScanCompletedPayload{Completeness: 0.5}),
	}

	// when: replay twice
	state1 := domain.ProjectState(events)
	state2 := domain.ProjectState(events)

	// then
	if state1.Completeness != state2.Completeness {
		t.Errorf("not idempotent: %f vs %f", state1.Completeness, state2.Completeness)
	}
	if state1.Project != state2.Project {
		t.Errorf("not idempotent: %s vs %s", state1.Project, state2.Project)
	}
}

func TestProjectState_UnknownEventType_Skipped(t *testing.T) {
	// given: events including an unknown type
	events := []domain.Event{
		mustNewEvent(t, domain.EventSessionStarted, "s1", 1,
			domain.SessionStartedPayload{Project: "p1"}),
		mustNewEvent(t, "future_event_type", "s1", 2, nil),
	}

	// when: should not panic
	state := domain.ProjectState(events)

	// then
	if state.Project != "p1" {
		t.Errorf("expected p1, got %s", state.Project)
	}
}

func TestProjectState_EmptyEvents(t *testing.T) {
	// when
	state := domain.ProjectState(nil)

	// then: should return zero-value state
	if state.SessionID != "" {
		t.Errorf("expected empty session ID, got %s", state.SessionID)
	}
}

func TestLoadState_RoundTrip(t *testing.T) {
	// given: store with events
	dir := t.TempDir()
	storePath := filepath.Join(dir, "events", "s1.jsonl")
	store := eventsource.NewFileEventStore(storePath, &domain.NopLogger{})
	recorder, recErr := eventsource.NewSessionRecorder(store, "s1")
	if recErr != nil {
		t.Fatalf("NewSessionRecorder: %v", recErr)
	}

	recorder.Record(mustNewEvent(t, domain.EventSessionStarted, "s1", 0,
		domain.SessionStartedPayload{Project: "test"}))
	recorder.Record(mustNewEvent(t, domain.EventScanCompleted, "s1", 0,
		domain.ScanCompletedPayload{Completeness: 0.4}))

	// when
	state, err := eventsource.LoadState(store)

	// then
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if state.Project != "test" {
		t.Errorf("expected test, got %s", state.Project)
	}
	if state.Completeness != 0.4 {
		t.Errorf("expected 0.4, got %f", state.Completeness)
	}
}

func TestLoadState_EmptyStore_ReturnsError(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(filepath.Join(dir, "empty.jsonl"), &domain.NopLogger{})

	// when
	_, err := eventsource.LoadState(store)

	// then
	if err == nil {
		t.Fatal("expected error for empty store")
	}
}

func TestLoadLatestState_FindsNewestSession(t *testing.T) {
	// given: two event files, older and newer
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".siren")
	eventsDir := eventsource.EventsDir(stateDir)
	os.MkdirAll(eventsDir, 0755)

	// Older session
	store1 := eventsource.NewFileEventStore(eventsource.EventStorePath(stateDir, "session-1000-1"), &domain.NopLogger{})
	rec1, err1 := eventsource.NewSessionRecorder(store1, "session-1000-1")
	if err1 != nil {
		t.Fatalf("NewSessionRecorder: %v", err1)
	}
	rec1.Record(mustNewEvent(t, domain.EventSessionStarted, "session-1000-1", 0,
		domain.SessionStartedPayload{Project: "old-project"}))
	rec1.Record(mustNewEvent(t, domain.EventScanCompleted, "session-1000-1", 0,
		domain.ScanCompletedPayload{Completeness: 0.3}))

	// Newer session
	store2 := eventsource.NewFileEventStore(eventsource.EventStorePath(stateDir, "session-2000-2"), &domain.NopLogger{})
	rec2, err2 := eventsource.NewSessionRecorder(store2, "session-2000-2")
	if err2 != nil {
		t.Fatalf("NewSessionRecorder: %v", err2)
	}
	rec2.Record(mustNewEvent(t, domain.EventSessionStarted, "session-2000-2", 0,
		domain.SessionStartedPayload{Project: "new-project"}))
	rec2.Record(mustNewEvent(t, domain.EventScanCompleted, "session-2000-2", 0,
		domain.ScanCompletedPayload{Completeness: 0.7}))

	// when
	state, sessionID, err := eventsource.LoadLatestState(stateDir)

	// then
	if err != nil {
		t.Fatalf("LoadLatestState: %v", err)
	}
	if sessionID != "session-2000-2" {
		t.Errorf("expected session-2000-2, got %s", sessionID)
	}
	if state.Project != "new-project" {
		t.Errorf("expected new-project, got %s", state.Project)
	}
	if state.Completeness != 0.7 {
		t.Errorf("expected 0.7, got %f", state.Completeness)
	}
}

func TestLoadLatestState_NoEventsDir(t *testing.T) {
	// given: stateDir with no events directory
	stateDir := filepath.Join(t.TempDir(), ".siren")
	os.MkdirAll(stateDir, 0755)

	// when
	_, _, err := eventsource.LoadLatestState(stateDir)

	// then
	if err == nil {
		t.Fatal("expected error for missing events dir")
	}
}

func TestLoadLatestState_EmptyEventsDir(t *testing.T) {
	// given: events directory with no files
	stateDir := filepath.Join(t.TempDir(), ".siren")
	os.MkdirAll(eventsource.EventsDir(stateDir), 0755)

	// when
	_, _, err := eventsource.LoadLatestState(stateDir)

	// then
	if err == nil {
		t.Fatal("expected error for empty events dir")
	}
}

func TestProjectState_WavesGenerated_Idempotent(t *testing.T) {
	// given: same WavesGenerated event replayed twice
	waveEvent := mustNewEvent(t, domain.EventWavesGenerated, "s1", 2,
		domain.WavesGeneratedPayload{
			Waves: []domain.WaveState{
				{ID: "w1", ClusterName: "Auth", Title: "First", Status: "available"},
				{ID: "w2", ClusterName: "Auth", Title: "Second", Status: "locked"},
			},
		})
	events := []domain.Event{
		mustNewEvent(t, domain.EventSessionStarted, "s1", 1, nil),
		waveEvent,
		waveEvent, // duplicate replay
	}

	// when
	state := domain.ProjectState(events)

	// then: waves should not be duplicated
	if len(state.Waves) != 2 {
		t.Errorf("expected 2 waves (idempotent), got %d", len(state.Waves))
	}
}

func TestProjectState_NextGenWavesAdded_Idempotent(t *testing.T) {
	// given: same NextGenWavesAdded event replayed twice
	nextgenEvent := mustNewEvent(t, domain.EventNextGenWavesAdded, "s1", 3,
		domain.NextGenWavesAddedPayload{
			ClusterName: "Auth",
			Waves:       []domain.WaveState{{ID: "w2", ClusterName: "Auth"}},
		})
	events := []domain.Event{
		mustNewEvent(t, domain.EventSessionStarted, "s1", 1, nil),
		mustNewEvent(t, domain.EventWavesGenerated, "s1", 2,
			domain.WavesGeneratedPayload{
				Waves: []domain.WaveState{{ID: "w1", ClusterName: "Auth"}},
			}),
		nextgenEvent,
		nextgenEvent, // duplicate replay
	}

	// when
	state := domain.ProjectState(events)

	// then: w2 should appear only once
	if len(state.Waves) != 2 {
		t.Errorf("expected 2 waves (w1 + w2 deduped), got %d", len(state.Waves))
	}
}

func TestProjectState_NextGenWavesAdded_DifferentWaves_Appends(t *testing.T) {
	// given: two different NextGenWavesAdded events with different waves
	events := []domain.Event{
		mustNewEvent(t, domain.EventSessionStarted, "s1", 1, nil),
		mustNewEvent(t, domain.EventWavesGenerated, "s1", 2,
			domain.WavesGeneratedPayload{
				Waves: []domain.WaveState{{ID: "w1", ClusterName: "Auth"}},
			}),
		mustNewEvent(t, domain.EventNextGenWavesAdded, "s1", 3,
			domain.NextGenWavesAddedPayload{
				ClusterName: "Auth",
				Waves:       []domain.WaveState{{ID: "w2", ClusterName: "Auth"}},
			}),
		mustNewEvent(t, domain.EventNextGenWavesAdded, "s1", 4,
			domain.NextGenWavesAddedPayload{
				ClusterName: "Auth",
				Waves:       []domain.WaveState{{ID: "w3", ClusterName: "Auth"}},
			}),
	}

	// when
	state := domain.ProjectState(events)

	// then: all three different waves should be present
	if len(state.Waves) != 3 {
		t.Errorf("expected 3 waves (w1, w2, w3), got %d", len(state.Waves))
	}
}

func TestProjectState_ADRGenerated_Idempotent(t *testing.T) {
	// given: same ADRGenerated event replayed twice
	adrEvent := mustNewEvent(t, domain.EventADRGenerated, "s1", 2,
		domain.ADRGeneratedPayload{ADRID: "0008", Title: "Event Sourcing"})
	events := []domain.Event{
		mustNewEvent(t, domain.EventSessionStarted, "s1", 1, nil),
		adrEvent,
		adrEvent, // duplicate replay
	}

	// when
	state := domain.ProjectState(events)

	// then: ADRCount should be 1, not 2
	if state.ADRCount != 1 {
		t.Errorf("expected ADRCount 1 (idempotent), got %d", state.ADRCount)
	}
}

func TestLoadAllEventsAcrossSessions_ReportsFailedSessions(t *testing.T) {
	// given: stateDir with one valid session and one empty (invalid) session dir
	stateDir := t.TempDir()
	eventsDir := filepath.Join(stateDir, "events")

	// Valid session
	validDir := filepath.Join(eventsDir, "valid-session")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatal(err)
	}
	event := mustNewEvent(t, domain.EventSessionStarted, "valid-session", 1,
		domain.SessionStartedPayload{Project: "test"})
	validStore := eventsource.NewFileEventStore(validDir, &domain.NopLogger{})
	if _, err := validStore.Append(context.Background(), event); err != nil {
		t.Fatal(err)
	}

	// Invalid session (empty directory, no events)
	invalidDir := filepath.Join(eventsDir, "invalid-session")
	if err := os.MkdirAll(invalidDir, 0755); err != nil {
		t.Fatal(err)
	}

	// when
	events, result, err := eventsource.LoadAllEventsAcrossSessions(stateDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SessionsLoaded != 1 {
		t.Errorf("SessionsLoaded = %d, want 1", result.SessionsLoaded)
	}
	if result.SessionsFailed != 1 {
		t.Errorf("SessionsFailed = %d, want 1", result.SessionsFailed)
	}
	if len(events) < 1 {
		t.Errorf("events = %d, want >= 1", len(events))
	}
}

func TestLoadAllEventsAcrossSessions_NoEventsDir_ReturnsEmpty(t *testing.T) {
	// given
	stateDir := t.TempDir()

	// when
	events, result, err := eventsource.LoadAllEventsAcrossSessions(stateDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("events = %d, want 0", len(events))
	}
	if result.SessionsLoaded != 0 {
		t.Errorf("SessionsLoaded = %d, want 0", result.SessionsLoaded)
	}
}

// mustNewEvent is a test helper that creates an event and fails on error.
// SessionID is set on the returned event. The seq parameter is ignored (kept for call-site compat).
func mustNewEvent(t *testing.T, eventType domain.EventType, sessionID string, _ int64, payload any) domain.Event {
	t.Helper()
	event, err := domain.NewEvent(eventType, payload, time.Now())
	if err != nil {
		t.Fatalf("NewEvent(%s): %v", eventType, err)
	}
	event.SessionID = sessionID
	return event
}
