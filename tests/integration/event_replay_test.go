//go:build integration

package integration_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/eventsource"
)

// TestEventReplay_SessionStartAndScanCompleted verifies that a JSONL event
// stream can be replayed through LoadState to produce a deterministic
// SessionState. Foundation for production-log-as-fixture testing.
func TestEventReplay_SessionStartAndScanCompleted(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})

	base := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	e1, err := domain.NewEvent(domain.EventSessionStarted, &domain.SessionStartedPayload{
		Project:         "test-project",
		StrictnessLevel: "fog",
	}, base)
	if err != nil {
		t.Fatalf("create session_started: %v", err)
	}

	clusters := []domain.ClusterState{
		{Name: "auth", IssueCount: 5},
		{Name: "api", IssueCount: 8},
		{Name: "ui", IssueCount: 2},
	}
	e2, err := domain.NewEvent(domain.EventScanCompleted, &domain.ScanCompletedPayload{
		Clusters:     clusters,
		Completeness: 0.65,
		ShibitoCount: 2,
	}, base.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("create scan_completed: %v", err)
	}

	if _, appendErr := store.Append(e1, e2); appendErr != nil {
		t.Fatalf("append: %v", appendErr)
	}

	// when
	state, loadErr := eventsource.LoadState(store)

	// then
	if loadErr != nil {
		t.Fatalf("LoadState: %v", loadErr)
	}
	if state.Project != "test-project" {
		t.Errorf("Project: got %q, want %q", state.Project, "test-project")
	}
	if state.StrictnessLevel != "fog" {
		t.Errorf("StrictnessLevel: got %q, want %q", state.StrictnessLevel, "fog")
	}
	if len(state.Clusters) != 3 {
		t.Errorf("Clusters: got %d, want 3", len(state.Clusters))
	}
	if state.ShibitoCount != 2 {
		t.Errorf("ShibitoCount: got %d, want 2", state.ShibitoCount)
	}
}

// TestEventReplay_RoundTrip verifies that events written to disk via Append,
// then loaded via LoadState, produce the same state as projecting in-memory.
func TestEventReplay_RoundTrip(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})

	base := time.Now()
	e1, _ := domain.NewEvent(domain.EventSessionStarted, &domain.SessionStartedPayload{
		Project:         "round-trip",
		StrictnessLevel: "alert",
	}, base)
	e2, _ := domain.NewEvent(domain.EventScanCompleted, &domain.ScanCompletedPayload{
		Clusters:     []domain.ClusterState{{Name: "core", IssueCount: 10}},
		Completeness: 0.80,
		ShibitoCount: 1,
	}, base.Add(time.Minute))

	// in-memory projection
	directState := domain.ProjectState([]domain.Event{e1, e2})

	// persist + reload
	store.Append(e1, e2)
	replayedState, err := eventsource.LoadState(store)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	// then
	if directState.Project != replayedState.Project {
		t.Errorf("Project: direct=%q replayed=%q", directState.Project, replayedState.Project)
	}
	if directState.Completeness != replayedState.Completeness {
		t.Errorf("Completeness: direct=%f replayed=%f", directState.Completeness, replayedState.Completeness)
	}
	if directState.ShibitoCount != replayedState.ShibitoCount {
		t.Errorf("ShibitoCount: direct=%d replayed=%d", directState.ShibitoCount, replayedState.ShibitoCount)
	}
}

// TestEventReplay_LoadLatestState picks the most recent session directory.
func TestEventReplay_LoadLatestState(t *testing.T) {
	// given
	stateDir := filepath.Join(t.TempDir(), ".siren")
	os.MkdirAll(filepath.Join(stateDir, "events"), 0o755)

	base := time.Now()

	// Older session
	store1 := eventsource.NewFileEventStore(eventsource.EventStorePath(stateDir, "session-old"), &domain.NopLogger{})
	e1, _ := domain.NewEvent(domain.EventSessionStarted, &domain.SessionStartedPayload{
		Project: "old-project", StrictnessLevel: "fog",
	}, base)
	store1.Append(e1)

	time.Sleep(10 * time.Millisecond)

	// Newer session
	store2 := eventsource.NewFileEventStore(eventsource.EventStorePath(stateDir, "session-new"), &domain.NopLogger{})
	e2, _ := domain.NewEvent(domain.EventSessionStarted, &domain.SessionStartedPayload{
		Project: "new-project", StrictnessLevel: "lockdown",
	}, base.Add(time.Hour))
	e3, _ := domain.NewEvent(domain.EventScanCompleted, &domain.ScanCompletedPayload{
		Clusters: []domain.ClusterState{{Name: "core", IssueCount: 30}},
	}, base.Add(time.Hour+time.Minute))
	store2.Append(e2, e3)

	// when
	state, sessionID, err := eventsource.LoadLatestState(stateDir)

	// then
	if err != nil {
		t.Fatalf("LoadLatestState: %v", err)
	}
	if sessionID != "session-new" {
		t.Errorf("sessionID: got %q, want %q", sessionID, "session-new")
	}
	if state.Project != "new-project" {
		t.Errorf("Project: got %q, want %q", state.Project, "new-project")
	}
	if state.StrictnessLevel != "lockdown" {
		t.Errorf("StrictnessLevel: got %q, want %q", state.StrictnessLevel, "lockdown")
	}
}
