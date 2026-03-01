package sightjack_test

import (
	"context"
	"testing"
	"time"

	"github.com/hironow/sightjack"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
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

func TestRecordWave_IncreasesCounter(t *testing.T) {
	// given
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	origMeter := sightjack.Meter
	sightjack.Meter = mp.Meter("test")
	defer func() { sightjack.Meter = origMeter }()
	ctx := context.Background()

	// when
	sightjack.RecordWave(ctx, "applied")
	sightjack.RecordWave(ctx, "rejected")
	sightjack.RecordWave(ctx, "applied")

	// then
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
		t.Fatal(err)
	}
	total := sumCounter(t, rm, "sightjack.wave.total")
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
}

func sumCounter(t *testing.T, rm metricdata.ResourceMetrics, name string) int64 {
	t.Helper()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				sum := m.Data.(metricdata.Sum[int64])
				var total int64
				for _, dp := range sum.DataPoints {
					total += dp.Value
				}
				return total
			}
		}
	}
	t.Fatalf("metric %q not found", name)
	return 0
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
