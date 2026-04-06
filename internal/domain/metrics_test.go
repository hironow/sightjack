package domain_test

import (
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

func makeEvent(t EventType) domain.Event {
	return domain.Event{ID: "test", Type: t, Timestamp: time.Now()}
}

type EventType = domain.EventType

func TestSuccessRate_AllApplied(t *testing.T) {
	events := []domain.Event{
		makeEvent(domain.EventWaveAppliedV2),
		makeEvent(domain.EventWaveAppliedV2),
	}

	rate := domain.SuccessRate(events)

	if rate != 1.0 {
		t.Errorf("SuccessRate = %f, want 1.0", rate)
	}
}

func TestSuccessRate_AllRejected(t *testing.T) {
	events := []domain.Event{
		makeEvent(domain.EventWaveRejectedV2),
		makeEvent(domain.EventWaveRejectedV2),
	}

	rate := domain.SuccessRate(events)

	if rate != 0.0 {
		t.Errorf("SuccessRate = %f, want 0.0", rate)
	}
}

func TestSuccessRate_Mixed(t *testing.T) {
	events := []domain.Event{
		makeEvent(domain.EventWaveAppliedV2),
		makeEvent(domain.EventWaveRejectedV2),
		makeEvent(domain.EventWaveAppliedV2),
	}

	rate := domain.SuccessRate(events)

	if rate < 0.66 || rate > 0.67 {
		t.Errorf("SuccessRate = %f, want ~0.666", rate)
	}
}

func TestSuccessRate_NoEvents(t *testing.T) {
	rate := domain.SuccessRate(nil)

	if rate != 0.0 {
		t.Errorf("SuccessRate = %f, want 0.0", rate)
	}
}

func TestSuccessRate_IgnoresOtherEvents(t *testing.T) {
	events := []domain.Event{
		makeEvent(domain.EventSessionStartedV2),
		makeEvent(domain.EventWaveAppliedV2),
		makeEvent(domain.EventScanCompletedV2),
		makeEvent(domain.EventWaveRejectedV2),
	}

	rate := domain.SuccessRate(events)

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
	msg := domain.FormatSuccessRate(rate, success, total)

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
	msg := domain.FormatSuccessRate(rate, success, total)

	// then
	if msg != "no events" {
		t.Errorf("FormatSuccessRate = %q, want %q", msg, "no events")
	}
}
