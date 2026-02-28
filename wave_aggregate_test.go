package sightjack_test

import (
	"testing"
	"time"

	"github.com/hironow/sightjack"
)

func TestWaveAggregate_Approve_Available(t *testing.T) {
	// given
	agg := sightjack.NewWaveAggregate()
	agg.SetWaves([]sightjack.Wave{
		{ID: "w1", ClusterName: "auth", Status: "available"},
	})

	// when
	ev, err := agg.Approve("w1", "auth", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != sightjack.EventWaveApproved {
		t.Fatalf("expected wave_approved, got %s", ev.Type)
	}
}

func TestWaveAggregate_Approve_NotFound(t *testing.T) {
	// given
	agg := sightjack.NewWaveAggregate()

	// when
	_, err := agg.Approve("nonexistent", "auth", time.Now().UTC())

	// then
	if err == nil {
		t.Fatal("expected error for nonexistent wave")
	}
}

func TestWaveAggregate_Reject(t *testing.T) {
	// given
	agg := sightjack.NewWaveAggregate()
	agg.SetWaves([]sightjack.Wave{
		{ID: "w1", ClusterName: "auth", Status: "available"},
	})

	// when
	ev, err := agg.Reject("w1", "auth", time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != sightjack.EventWaveRejected {
		t.Fatalf("expected wave_rejected, got %s", ev.Type)
	}
}

func TestWaveAggregate_RecordApplied(t *testing.T) {
	// given
	agg := sightjack.NewWaveAggregate()
	agg.SetWaves([]sightjack.Wave{
		{ID: "w1", ClusterName: "auth", Status: "available", Actions: []sightjack.WaveAction{{Type: "fix"}}},
	})

	// when
	ev, err := agg.RecordApplied(sightjack.WaveAppliedPayload{
		WaveID: "w1", ClusterName: "auth", Applied: 1, TotalCount: 1,
	}, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != sightjack.EventWaveApplied {
		t.Fatalf("expected wave_applied, got %s", ev.Type)
	}
}

func TestWaveAggregate_Complete(t *testing.T) {
	// given
	agg := sightjack.NewWaveAggregate()
	agg.SetWaves([]sightjack.Wave{
		{ID: "w1", ClusterName: "auth", Status: "available", Actions: []sightjack.WaveAction{{Type: "fix"}}},
	})

	// when
	ev, err := agg.Complete("w1", "auth", 1, 1, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != sightjack.EventWaveCompleted {
		t.Fatalf("expected wave_completed, got %s", ev.Type)
	}
	// Completed map should be updated
	if !agg.IsCompleted("auth:w1") {
		t.Fatal("wave should be marked completed")
	}
}

func TestWaveAggregate_EvaluateUnlocks(t *testing.T) {
	// given: w2 depends on w1
	agg := sightjack.NewWaveAggregate()
	agg.SetWaves([]sightjack.Wave{
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
	if events[0].Type != sightjack.EventWavesUnlocked {
		t.Fatalf("expected waves_unlocked, got %s", events[0].Type)
	}
}

func TestWaveAggregate_EvaluateUnlocks_NothingToUnlock(t *testing.T) {
	// given: no locked waves
	agg := sightjack.NewWaveAggregate()
	agg.SetWaves([]sightjack.Wave{
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
	agg := sightjack.NewWaveAggregate()
	newWaves := []sightjack.WaveState{
		{ID: "w2", ClusterName: "auth", Title: "Next wave", Status: "available"},
	}

	// when
	ev, err := agg.AddNextGen("auth", newWaves, time.Now().UTC())

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != sightjack.EventNextGenWavesAdded {
		t.Fatalf("expected nextgen_waves_added, got %s", ev.Type)
	}
}
