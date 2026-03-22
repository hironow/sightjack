package domain_test

import (
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

// mustEvent creates an Event with the given type, session ID, and payload.
// SessionID is set on the returned event. Panics on marshal failure.
func mustEvent(t *testing.T, eventType domain.EventType, sessionID string, _ int64, payload any) domain.Event {
	t.Helper()
	e, err := domain.NewEvent(eventType, payload, time.Now())
	if err != nil {
		t.Fatalf("mustEvent: %v", err)
	}
	e.SessionID = sessionID
	return e
}

func TestProjectState_Empty(t *testing.T) {
	t.Parallel()
	// when
	state := domain.ProjectState(nil)

	// then — zero-value state
	if state.SessionID != "" {
		t.Errorf("SessionID = %q, want empty", state.SessionID)
	}
	if len(state.Waves) != 0 {
		t.Errorf("Waves len = %d, want 0", len(state.Waves))
	}
}

func TestProjectState_SessionStarted(t *testing.T) {
	t.Parallel()
	// given
	events := []domain.Event{
		mustEvent(t, domain.EventSessionStarted, "sess-1", 1,
			domain.SessionStartedPayload{Project: "myproject", StrictnessLevel: "standard"}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if state.Version != domain.StateFormatVersion {
		t.Errorf("Version = %q, want %q", state.Version, domain.StateFormatVersion)
	}
	if state.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want %q", state.SessionID, "sess-1")
	}
	if state.Project != "myproject" {
		t.Errorf("Project = %q, want %q", state.Project, "myproject")
	}
	if state.StrictnessLevel != "standard" {
		t.Errorf("StrictnessLevel = %q, want %q", state.StrictnessLevel, "standard")
	}
}

func TestProjectState_ScanCompleted(t *testing.T) {
	t.Parallel()
	// given
	events := []domain.Event{
		mustEvent(t, domain.EventScanCompleted, "sess-1", 2,
			domain.ScanCompletedPayload{
				Clusters:     []domain.ClusterState{{Name: "auth", Completeness: 0.6}},
				Completeness: 0.6,
				ShibitoCount: 2,
			}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if len(state.Clusters) != 1 {
		t.Fatalf("Clusters len = %d, want 1", len(state.Clusters))
	}
	if state.Clusters[0].Name != "auth" {
		t.Errorf("Cluster name = %q, want %q", state.Clusters[0].Name, "auth")
	}
	if state.Completeness != 0.6 {
		t.Errorf("Completeness = %f, want 0.6", state.Completeness)
	}
	if state.ShibitoCount != 2 {
		t.Errorf("ShibitoCount = %d, want 2", state.ShibitoCount)
	}
}

func TestProjectState_WavesGenerated_Idempotent(t *testing.T) {
	t.Parallel()
	// given — same wave in two events (duplicate replay)
	wave := domain.WaveState{ID: "w1", ClusterName: "auth", Status: "available"}
	events := []domain.Event{
		mustEvent(t, domain.EventWavesGenerated, "sess-1", 3,
			domain.WavesGeneratedPayload{Waves: []domain.WaveState{wave}}),
		mustEvent(t, domain.EventWavesGenerated, "sess-1", 4,
			domain.WavesGeneratedPayload{Waves: []domain.WaveState{wave}}),
	}

	// when
	state := domain.ProjectState(events)

	// then — deduplicated
	if len(state.Waves) != 1 {
		t.Errorf("Waves len = %d, want 1 (dedup)", len(state.Waves))
	}
}

func TestProjectState_WaveCompleted(t *testing.T) {
	t.Parallel()
	// given
	events := []domain.Event{
		mustEvent(t, domain.EventWavesGenerated, "sess-1", 1,
			domain.WavesGeneratedPayload{Waves: []domain.WaveState{
				{ID: "w1", ClusterName: "auth", Status: "available"},
			}}),
		mustEvent(t, domain.EventWaveCompleted, "sess-1", 2,
			domain.WaveCompletedPayload{WaveID: "w1", ClusterName: "auth"}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if state.Waves[0].Status != "completed" {
		t.Errorf("wave status = %q, want %q", state.Waves[0].Status, "completed")
	}
}

func TestProjectState_CompletenessUpdated(t *testing.T) {
	t.Parallel()
	// given
	events := []domain.Event{
		mustEvent(t, domain.EventScanCompleted, "sess-1", 1,
			domain.ScanCompletedPayload{
				Clusters:     []domain.ClusterState{{Name: "auth", Completeness: 0.5}},
				Completeness: 0.5,
			}),
		mustEvent(t, domain.EventCompletenessUpdated, "sess-1", 2,
			domain.CompletenessUpdatedPayload{
				ClusterName:         "auth",
				ClusterCompleteness: 0.8,
				OverallCompleteness: 0.8,
			}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if state.Completeness != 0.8 {
		t.Errorf("Completeness = %f, want 0.8", state.Completeness)
	}
	if state.Clusters[0].Completeness != 0.8 {
		t.Errorf("Cluster completeness = %f, want 0.8", state.Clusters[0].Completeness)
	}
}

func TestProjectState_WavesUnlocked(t *testing.T) {
	t.Parallel()
	// given
	events := []domain.Event{
		mustEvent(t, domain.EventWavesGenerated, "sess-1", 1,
			domain.WavesGeneratedPayload{Waves: []domain.WaveState{
				{ID: "w1", ClusterName: "auth", Status: "locked"},
				{ID: "w2", ClusterName: "auth", Status: "available"},
			}}),
		mustEvent(t, domain.EventWavesUnlocked, "sess-1", 2,
			domain.WavesUnlockedPayload{UnlockedWaveIDs: []string{"auth:w1"}}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if state.Waves[0].Status != "available" {
		t.Errorf("w1 status = %q, want %q", state.Waves[0].Status, "available")
	}
	// w2 already available — unchanged
	if state.Waves[1].Status != "available" {
		t.Errorf("w2 status = %q, want %q", state.Waves[1].Status, "available")
	}
}

func TestProjectState_NextGenWavesAdded(t *testing.T) {
	t.Parallel()
	// given
	events := []domain.Event{
		mustEvent(t, domain.EventWavesGenerated, "sess-1", 1,
			domain.WavesGeneratedPayload{Waves: []domain.WaveState{
				{ID: "w1", ClusterName: "auth", Status: "available"},
			}}),
		mustEvent(t, domain.EventNextGenWavesAdded, "sess-1", 2,
			domain.NextGenWavesAddedPayload{
				ClusterName: "auth",
				Waves: []domain.WaveState{
					{ID: "w2", ClusterName: "auth", Status: "locked"},
				},
			}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if len(state.Waves) != 2 {
		t.Fatalf("Waves len = %d, want 2", len(state.Waves))
	}
	if state.Waves[1].ID != "w2" {
		t.Errorf("Waves[1].ID = %q, want %q", state.Waves[1].ID, "w2")
	}
}

func TestProjectState_WaveModified(t *testing.T) {
	t.Parallel()
	// given
	events := []domain.Event{
		mustEvent(t, domain.EventWavesGenerated, "sess-1", 1,
			domain.WavesGeneratedPayload{Waves: []domain.WaveState{
				{ID: "w1", ClusterName: "auth", Title: "Original", Status: "available"},
			}}),
		mustEvent(t, domain.EventWaveModified, "sess-1", 2,
			domain.WaveModifiedPayload{
				WaveID:      "w1",
				ClusterName: "auth",
				UpdatedWave: domain.WaveState{ID: "w1", ClusterName: "auth", Title: "Modified", Status: "available"},
			}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if state.Waves[0].Title != "Modified" {
		t.Errorf("Title = %q, want %q", state.Waves[0].Title, "Modified")
	}
}

func TestProjectState_ADRGenerated_Idempotent(t *testing.T) {
	t.Parallel()
	// given — same ADR ID twice
	events := []domain.Event{
		mustEvent(t, domain.EventADRGenerated, "sess-1", 1,
			domain.ADRGeneratedPayload{ADRID: "adr-1", Title: "Decision"}),
		mustEvent(t, domain.EventADRGenerated, "sess-1", 2,
			domain.ADRGeneratedPayload{ADRID: "adr-1", Title: "Decision"}),
	}

	// when
	state := domain.ProjectState(events)

	// then — counted once
	if state.ADRCount != 1 {
		t.Errorf("ADRCount = %d, want 1", state.ADRCount)
	}
}

func TestProjectState_AuditOnlyEventsNoMutation(t *testing.T) {
	t.Parallel()
	// given — audit-only events should not change state
	events := []domain.Event{
		mustEvent(t, domain.EventSessionStarted, "sess-1", 1,
			domain.SessionStartedPayload{Project: "proj"}),
		mustEvent(t, domain.EventWaveApproved, "sess-1", 2,
			domain.WaveIdentityPayload{WaveID: "w1", ClusterName: "auth"}),
		mustEvent(t, domain.EventSpecificationSent, "sess-1", 3,
			domain.WaveIdentityPayload{WaveID: "w1", ClusterName: "auth"}),
	}

	// when
	state := domain.ProjectState(events)

	// then — only session_started mutated state
	if state.Project != "proj" {
		t.Errorf("Project = %q, want %q", state.Project, "proj")
	}
	if len(state.Waves) != 0 {
		t.Errorf("Waves should be empty for audit-only events, got %d", len(state.Waves))
	}
}

func TestProjectState_UnknownEventSkipped(t *testing.T) {
	t.Parallel()
	// given — craft an event with unknown type
	e, err := domain.NewEvent("unknown_future_event", map[string]string{"key": "value"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	events := []domain.Event{e}

	// when
	state := domain.ProjectState(events)

	// then — no panic, zero state
	if state.SessionID != "" {
		t.Errorf("SessionID = %q, want empty", state.SessionID)
	}
}

func TestProjectState_FullLifecycle(t *testing.T) {
	t.Parallel()
	// given — a realistic event sequence
	events := []domain.Event{
		mustEvent(t, domain.EventSessionStarted, "sess-1", 1,
			domain.SessionStartedPayload{Project: "myapp", StrictnessLevel: "standard"}),
		mustEvent(t, domain.EventScanCompleted, "sess-1", 2,
			domain.ScanCompletedPayload{
				Clusters:     []domain.ClusterState{{Name: "auth", Completeness: 0.4}},
				Completeness: 0.4,
				ShibitoCount: 1,
			}),
		mustEvent(t, domain.EventWavesGenerated, "sess-1", 3,
			domain.WavesGeneratedPayload{Waves: []domain.WaveState{
				{ID: "w1", ClusterName: "auth", Status: "available"},
				{ID: "w2", ClusterName: "auth", Status: "locked"},
			}}),
		mustEvent(t, domain.EventWaveCompleted, "sess-1", 4,
			domain.WaveCompletedPayload{WaveID: "w1", ClusterName: "auth", Applied: 2, TotalCount: 2}),
		mustEvent(t, domain.EventWavesUnlocked, "sess-1", 5,
			domain.WavesUnlockedPayload{UnlockedWaveIDs: []string{"auth:w2"}}),
		mustEvent(t, domain.EventCompletenessUpdated, "sess-1", 6,
			domain.CompletenessUpdatedPayload{
				ClusterName: "auth", ClusterCompleteness: 0.7, OverallCompleteness: 0.7,
			}),
		mustEvent(t, domain.EventADRGenerated, "sess-1", 7,
			domain.ADRGeneratedPayload{ADRID: "adr-1", Title: "Decision 1"}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if state.SessionID != "sess-1" {
		t.Errorf("SessionID = %q", state.SessionID)
	}
	if state.Project != "myapp" {
		t.Errorf("Project = %q", state.Project)
	}
	if state.Completeness != 0.7 {
		t.Errorf("Completeness = %f, want 0.7", state.Completeness)
	}
	if state.Clusters[0].Completeness != 0.7 {
		t.Errorf("Cluster completeness = %f, want 0.7", state.Clusters[0].Completeness)
	}
	if state.Waves[0].Status != "completed" {
		t.Errorf("w1 status = %q, want completed", state.Waves[0].Status)
	}
	if state.Waves[1].Status != "available" {
		t.Errorf("w2 status = %q, want available", state.Waves[1].Status)
	}
	if state.ADRCount != 1 {
		t.Errorf("ADRCount = %d, want 1", state.ADRCount)
	}
	if state.ShibitoCount != 1 {
		t.Errorf("ShibitoCount = %d, want 1", state.ShibitoCount)
	}
}

func TestProjectState_FeedbackSent_IncrementsFeedbackCount(t *testing.T) {
	t.Parallel()
	// given
	events := []domain.Event{
		mustEvent(t, domain.EventSessionStarted, "sess-1", 1,
			domain.SessionStartedPayload{Project: "proj"}),
		mustEvent(t, domain.EventFeedbackSent, "sess-1", 2,
			domain.WaveIdentityPayload{WaveID: "w1", ClusterName: "auth"}),
		mustEvent(t, domain.EventFeedbackSent, "sess-1", 3,
			domain.WaveIdentityPayload{WaveID: "w2", ClusterName: "auth"}),
	}

	// when
	state := domain.ProjectState(events)

	// then
	if state.FeedbackCount != 2 {
		t.Errorf("FeedbackCount = %d, want 2", state.FeedbackCount)
	}
}
