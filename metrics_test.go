package sightjack_test

import (
	"testing"
	"time"

	"github.com/hironow/sightjack"
)

func makeEvent(t EventType) sightjack.Event {
	return sightjack.Event{ID: "test", Type: t, Timestamp: time.Now()}
}

type EventType = sightjack.EventType

func TestSuccessRate_AllApplied(t *testing.T) {
	events := []sightjack.Event{
		makeEvent(sightjack.EventWaveApplied),
		makeEvent(sightjack.EventWaveApplied),
	}

	rate := sightjack.SuccessRate(events)

	if rate != 1.0 {
		t.Errorf("SuccessRate = %f, want 1.0", rate)
	}
}

func TestSuccessRate_AllRejected(t *testing.T) {
	events := []sightjack.Event{
		makeEvent(sightjack.EventWaveRejected),
		makeEvent(sightjack.EventWaveRejected),
	}

	rate := sightjack.SuccessRate(events)

	if rate != 0.0 {
		t.Errorf("SuccessRate = %f, want 0.0", rate)
	}
}

func TestSuccessRate_Mixed(t *testing.T) {
	events := []sightjack.Event{
		makeEvent(sightjack.EventWaveApplied),
		makeEvent(sightjack.EventWaveRejected),
		makeEvent(sightjack.EventWaveApplied),
	}

	rate := sightjack.SuccessRate(events)

	if rate < 0.66 || rate > 0.67 {
		t.Errorf("SuccessRate = %f, want ~0.666", rate)
	}
}

func TestSuccessRate_NoEvents(t *testing.T) {
	rate := sightjack.SuccessRate(nil)

	if rate != 0.0 {
		t.Errorf("SuccessRate = %f, want 0.0", rate)
	}
}

func TestSuccessRate_IgnoresOtherEvents(t *testing.T) {
	events := []sightjack.Event{
		makeEvent(sightjack.EventSessionStarted),
		makeEvent(sightjack.EventWaveApplied),
		makeEvent(sightjack.EventScanCompleted),
		makeEvent(sightjack.EventWaveRejected),
	}

	rate := sightjack.SuccessRate(events)

	if rate != 0.5 {
		t.Errorf("SuccessRate = %f, want 0.5", rate)
	}
}

func TestFormatSuccessRate_WithEvents(t *testing.T) {
	// given
	rate := 0.857142
	success := 6
	total := 7

	// when
	msg := sightjack.FormatSuccessRate(rate, success, total)

	// then
	if msg != "85.7% (6/7)" {
		t.Errorf("FormatSuccessRate = %q, want %q", msg, "85.7% (6/7)")
	}
}

func TestFormatSuccessRate_NoEvents(t *testing.T) {
	// given
	rate := 0.0
	success := 0
	total := 0

	// when
	msg := sightjack.FormatSuccessRate(rate, success, total)

	// then
	if msg != "no events" {
		t.Errorf("FormatSuccessRate = %q, want %q", msg, "no events")
	}
}
