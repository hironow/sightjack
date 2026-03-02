package sightjack

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// FormatSuccessRate returns a human-readable string for the given success rate.
// Returns "no events" when total is zero.
func FormatSuccessRate(rate float64, success, total int) string {
	if total == 0 {
		return "no events"
	}
	return fmt.Sprintf("%.1f%% (%d/%d)", rate*100, success, total)
}

// RecordWave increments the sightjack.wave.total OTel counter.
func RecordWave(ctx context.Context, status string) {
	c, _ := Meter.Int64Counter("sightjack.wave.total",
		metric.WithDescription("Total wave operations"),
	)
	c.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("status", status),
		),
	)
}

// SuccessRate calculates the wave success rate from a list of events.
// It counts EventWaveApplied as success and EventWaveRejected as failure.
// Returns 0.0 if there are no relevant events.
func SuccessRate(events []Event) float64 {
	var success, total int
	for _, ev := range events {
		switch ev.Type {
		case EventWaveApplied:
			success++
			total++
		case EventWaveRejected:
			total++
		}
	}
	if total == 0 {
		return 0.0
	}
	return float64(success) / float64(total)
}
