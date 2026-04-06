package policy_test

import (
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness"
)

func TestWaveAggregate_Approve_Available(t *testing.T) {
	// given
	agg := domain.NewWaveAggregate()
	agg.SetWaves([]domain.Wave{
		{ID: "w1", ClusterName: "auth", Status: "available"},
	})

	// when
	ev, err := agg.Approve("w1", "auth", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventWaveApprovedV2 {
		t.Fatalf("expected wave_approved, got %s", ev.Type)
	}
}

func TestWaveAggregate_Approve_NotFound(t *testing.T) {
	// given
	agg := domain.NewWaveAggregate()

	// when
	_, err := agg.Approve("nonexistent", "auth", time.Now().UTC())

	// then
	if err == nil {
		t.Fatal("expected error for nonexistent wave")
	}
}

func TestWaveAggregate_Reject(t *testing.T) {
	// given
	agg := domain.NewWaveAggregate()
	agg.SetWaves([]domain.Wave{
		{ID: "w1", ClusterName: "auth", Status: "available"},
	})

	// when
	ev, err := agg.Reject("w1", "auth", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventWaveRejectedV2 {
		t.Fatalf("expected wave_rejected, got %s", ev.Type)
	}
}

func TestWaveAggregate_RecordApplied(t *testing.T) {
	// given
	agg := domain.NewWaveAggregate()
	agg.SetWaves([]domain.Wave{
		{ID: "w1", ClusterName: "auth", Status: "available", Actions: []domain.WaveAction{{Type: "fix"}}},
	})

	// when
	ev, err := agg.RecordApplied(domain.WaveAppliedPayload{
		WaveID: "w1", ClusterName: "auth", Applied: 1, TotalCount: 1,
	}, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventWaveAppliedV2 {
		t.Fatalf("expected wave_applied, got %s", ev.Type)
	}
}

func TestWaveAggregate_Complete(t *testing.T) {
	// given
	agg := domain.NewWaveAggregate()
	agg.SetWaves([]domain.Wave{
		{ID: "w1", ClusterName: "auth", Status: "available", Actions: []domain.WaveAction{{Type: "fix"}}},
	})

	// when
	ev, err := agg.Complete("w1", "auth", 1, 1, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventWaveCompletedV2 {
		t.Fatalf("expected wave_completed, got %s", ev.Type)
	}
	// Completed map should be updated
	if !agg.IsCompleted("auth:w1") {
		t.Fatal("wave should be marked completed")
	}
}

func TestWaveAggregate_Complete_SyncsStatusField(t *testing.T) {
	// given
	agg := domain.NewWaveAggregate()
	agg.SetWaves([]domain.Wave{
		{ID: "w1", ClusterName: "auth", Status: "available"},
	})

	// when
	_, err := agg.Complete("w1", "auth", 1, 1, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The wave's Status field should be "completed"
	waves := agg.Waves()
	if waves[0].Status != "completed" {
		t.Errorf("wave status = %q, want %q", waves[0].Status, "completed")
	}
}

func TestWaveAggregate_Complete_StatusPersistsViaRoundTrip(t *testing.T) {
	// given
	agg := domain.NewWaveAggregate()
	agg.SetWaves([]domain.Wave{
		{ID: "w1", ClusterName: "auth", Status: "available"},
	})

	// when: complete then persist/restore
	_, _ = agg.Complete("w1", "auth", 1, 1, time.Now().UTC())
	states := harness.BuildWaveStates(agg.Waves())
	restored := harness.RestoreWaves(states)
	completed := harness.BuildCompletedWaveMap(restored)

	// then: restored wave should be in completed map
	if !completed["auth:w1"] {
		t.Error("expected auth:w1 in completed map after round-trip")
	}
}

func TestWaveAggregate_Complete_RejectsUnknownWave(t *testing.T) {
	// given
	agg := domain.NewWaveAggregate()
	agg.SetWaves([]domain.Wave{
		{ID: "w1", ClusterName: "auth", Status: "available"},
	})

	// when
	_, err := agg.Complete("w999", "auth", 3, 3, time.Now().UTC())

	// then
	if err == nil {
		t.Fatal("expected error for nonexistent wave")
	}
	if agg.IsCompleted("auth:w999") {
		t.Error("phantom wave should not be in completed map")
	}
}

func TestWaveAggregate_EvaluateUnlocks(t *testing.T) {
	// given: w2 depends on w1
	agg := domain.NewWaveAggregate()
	agg.SetWaves([]domain.Wave{
		{ID: "w1", ClusterName: "auth", Status: "available"},
		{ID: "w2", ClusterName: "auth", Status: "locked", Prerequisites: []string{"auth:w1"}},
	})
	agg.MarkCompleted("auth:w1")

	// when
	events, err := agg.EvaluateUnlocks(time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != domain.EventWavesUnlockedV2 {
		t.Fatalf("expected waves_unlocked, got %s", events[0].Type)
	}
}

func TestWaveAggregate_EvaluateUnlocks_NothingToUnlock(t *testing.T) {
	// given: no locked waves
	agg := domain.NewWaveAggregate()
	agg.SetWaves([]domain.Wave{
		{ID: "w1", ClusterName: "auth", Status: "available"},
	})

	// when
	events, err := agg.EvaluateUnlocks(time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestWaveAggregate_AddNextGen(t *testing.T) {
	// given
	agg := domain.NewWaveAggregate()
	newWaves := []domain.WaveState{
		{ID: "w2", ClusterName: "auth", Title: "Next wave", Status: "available"},
	}

	// when
	ev, err := agg.AddNextGen("auth", newWaves, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != domain.EventNextGenWavesAddedV2 {
		t.Fatalf("expected nextgen_waves_added, got %s", ev.Type)
	}
}

func TestWaveAggregate_WaveStatusCounts(t *testing.T) {
	t.Parallel()
	// given
	agg := domain.NewWaveAggregate()
	agg.SetWaves([]domain.Wave{
		{ID: "w1", ClusterName: "auth", Status: "completed"},
		{ID: "w2", ClusterName: "auth", Status: "available"},
		{ID: "w3", ClusterName: "auth", Status: "available"},
		{ID: "w4", ClusterName: "auth", Status: "locked"},
	})

	// when
	counts := agg.WaveStatusCounts()

	// then
	if counts["completed"] != 1 {
		t.Errorf("completed = %d, want 1", counts["completed"])
	}
	if counts["available"] != 2 {
		t.Errorf("available = %d, want 2", counts["available"])
	}
	if counts["locked"] != 1 {
		t.Errorf("locked = %d, want 1", counts["locked"])
	}
}

func TestWaveAggregate_AllWavesCompleted_AllDone(t *testing.T) {
	t.Parallel()
	// given
	agg := domain.NewWaveAggregate()
	agg.SetWaves([]domain.Wave{
		{ID: "w1", ClusterName: "auth", Status: "completed"},
		{ID: "w2", ClusterName: "auth", Status: "completed"},
	})

	// when / then
	if !agg.AllWavesCompleted() {
		t.Error("AllWavesCompleted() = false, want true when all waves are completed")
	}
}

func TestWaveAggregate_AllWavesCompleted_NotAllDone(t *testing.T) {
	t.Parallel()
	// given
	agg := domain.NewWaveAggregate()
	agg.SetWaves([]domain.Wave{
		{ID: "w1", ClusterName: "auth", Status: "completed"},
		{ID: "w2", ClusterName: "auth", Status: "available"},
	})

	// when / then
	if agg.AllWavesCompleted() {
		t.Error("AllWavesCompleted() = true, want false when some waves are not completed")
	}
}

func TestWaveAggregate_AllWavesCompleted_EmptyReturnsFalse(t *testing.T) {
	t.Parallel()
	// given: no waves
	agg := domain.NewWaveAggregate()

	// when / then
	if agg.AllWavesCompleted() {
		t.Error("AllWavesCompleted() = true, want false for empty wave list")
	}
}
