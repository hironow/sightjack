package integration_test

import (
	"context"
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

	if _, appendErr := store.Append(context.Background(),e1, e2); appendErr != nil {
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
	e1, err := domain.NewEvent(domain.EventSessionStarted, &domain.SessionStartedPayload{
		Project:         "round-trip",
		StrictnessLevel: "alert",
	}, base)
	if err != nil {
		t.Fatalf("create session_started: %v", err)
	}
	e2, err := domain.NewEvent(domain.EventScanCompleted, &domain.ScanCompletedPayload{
		Clusters:     []domain.ClusterState{{Name: "core", IssueCount: 10}},
		Completeness: 0.80,
		ShibitoCount: 1,
	}, base.Add(time.Minute))
	if err != nil {
		t.Fatalf("create scan_completed: %v", err)
	}

	// in-memory projection
	directState := domain.ProjectState([]domain.Event{e1, e2})

	// persist + reload
	if _, appendErr := store.Append(context.Background(),e1, e2); appendErr != nil {
		t.Fatalf("append: %v", appendErr)
	}
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
	store1.Append(context.Background(),e1)

	// Newer session
	store2 := eventsource.NewFileEventStore(eventsource.EventStorePath(stateDir, "session-new"), &domain.NopLogger{})
	e2, _ := domain.NewEvent(domain.EventSessionStarted, &domain.SessionStartedPayload{
		Project: "new-project", StrictnessLevel: "lockdown",
	}, base.Add(time.Hour))
	e3, _ := domain.NewEvent(domain.EventScanCompleted, &domain.ScanCompletedPayload{
		Clusters: []domain.ClusterState{{Name: "core", IssueCount: 30}},
	}, base.Add(time.Hour+time.Minute))
	store2.Append(context.Background(),e2, e3)

	// Set modtimes explicitly to guarantee ordering without time.Sleep.
	oldTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	newTime := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	os.Chtimes(eventsource.EventStorePath(stateDir, "session-old"), oldTime, oldTime)
	os.Chtimes(eventsource.EventStorePath(stateDir, "session-new"), newTime, newTime)

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

// TestEventReplay_StrictnessPreserved verifies that StrictnessLevel set in
// session_started is preserved through event replay. This is the integration-
// level complement to the unit-level ResolveStrictness tests.
func TestEventReplay_StrictnessPreserved(t *testing.T) {
	levels := []string{"fog", "alert", "lockdown"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			// given
			dir := t.TempDir()
			store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})

			e, _ := domain.NewEvent(domain.EventSessionStarted, &domain.SessionStartedPayload{
				Project:         "strictness-test",
				StrictnessLevel: level,
			}, time.Now())
			store.Append(context.Background(),e)

			// when
			state, err := eventsource.LoadState(store)

			// then
			if err != nil {
				t.Fatalf("LoadState: %v", err)
			}
			if state.StrictnessLevel != level {
				t.Errorf("StrictnessLevel: got %q, want %q", state.StrictnessLevel, level)
			}
		})
	}
}

// TestEventReplay_ClusterIssueCountPreserved verifies that per-cluster
// IssueCount set in scan_completed events is preserved through JSONL
// round-trip. 7 test locations set IssueCount but none assert it after replay.
func TestEventReplay_ClusterIssueCountPreserved(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir, &domain.NopLogger{})

	base := time.Date(2025, 9, 1, 10, 0, 0, 0, time.UTC)

	e1, _ := domain.NewEvent(domain.EventSessionStarted, &domain.SessionStartedPayload{
		Project: "issue-count-test", StrictnessLevel: "fog",
	}, base)
	e2, _ := domain.NewEvent(domain.EventScanCompleted, &domain.ScanCompletedPayload{
		Clusters: []domain.ClusterState{
			{Name: "auth", IssueCount: 12},
			{Name: "api", IssueCount: 7},
			{Name: "ui", IssueCount: 3},
		},
		Completeness: 0.45,
		ShibitoCount: 1,
	}, base.Add(5*time.Minute))

	store.Append(context.Background(),e1, e2)

	// when
	state, err := eventsource.LoadState(store)

	// then
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(state.Clusters) != 3 {
		t.Fatalf("Clusters: got %d, want 3", len(state.Clusters))
	}
	// KEY ASSERTIONS: IssueCount must survive JSONL round-trip
	for _, want := range []struct {
		name  string
		count int
	}{
		{"auth", 12},
		{"api", 7},
		{"ui", 3},
	} {
		found := false
		for _, c := range state.Clusters {
			if c.Name == want.name {
				found = true
				if c.IssueCount != want.count {
					t.Errorf("Cluster %s IssueCount: got %d, want %d", want.name, c.IssueCount, want.count)
				}
				break
			}
		}
		if !found {
			t.Errorf("Cluster %q not found in replayed state", want.name)
		}
	}
}
