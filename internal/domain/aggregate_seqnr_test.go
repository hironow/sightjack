package domain

// white-box-reason: tests unexported seqNr field increment on SessionAggregate and WaveAggregate

import (
	"testing"
	"time"
)

func TestSessionAggregate_SeqNrIncrements(t *testing.T) {
	agg := NewSessionAggregate()
	agg.SetSessionID("sess-1")
	now := time.Now()

	ev1, err := agg.Start("proj", "alert", now) // nosemgrep: adr0003-otel-span-without-defer-end — aggregate Start, not OTel [permanent]
	if err != nil {
		t.Fatal(err)
	}
	ev2, err := agg.RecordScan(ScanCompletedPayload{Completeness: 0.5}, now)
	if err != nil {
		t.Fatal(err)
	}

	if ev1.SeqNr != 1 {
		t.Errorf("ev1.SeqNr = %d, want 1", ev1.SeqNr)
	}
	if ev2.SeqNr != 2 {
		t.Errorf("ev2.SeqNr = %d, want 2", ev2.SeqNr)
	}
	if ev1.AggregateID != "sess-1" {
		t.Errorf("ev1.AggregateID = %q, want sess-1", ev1.AggregateID)
	}
	if ev1.AggregateType != AggregateTypeSession {
		t.Errorf("ev1.AggregateType = %q, want %q", ev1.AggregateType, AggregateTypeSession)
	}
	if ev1.SessionID != "sess-1" {
		t.Errorf("ev1.SessionID = %q, want sess-1 (backward compat)", ev1.SessionID)
	}
}

func TestWaveAggregate_SeqNrIncrements(t *testing.T) {
	agg := NewWaveAggregate()
	agg.SetWaves([]Wave{
		{ID: "w1", ClusterName: "auth"},
	})
	now := time.Now()

	ev1, err := agg.Approve("w1", "auth", now)
	if err != nil {
		t.Fatal(err)
	}

	if ev1.SeqNr != 1 {
		t.Errorf("ev1.SeqNr = %d, want 1", ev1.SeqNr)
	}
	if ev1.AggregateID != "auth:w1" {
		t.Errorf("ev1.AggregateID = %q, want auth:w1", ev1.AggregateID)
	}
	if ev1.AggregateType != AggregateTypeWave {
		t.Errorf("ev1.AggregateType = %q, want %q", ev1.AggregateType, AggregateTypeWave)
	}
}
