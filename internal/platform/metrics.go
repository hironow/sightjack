package platform

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// RecordWave increments the sightjack.wave.total OTel counter.
func RecordWave(ctx context.Context, status string) {
	c, err := Meter.Int64Counter("sightjack.wave.total",
		metric.WithDescription("Total wave operations"),
	)
	if err != nil {
		return
	}
	c.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("status", status), // nosemgrep: otel-attribute-string-unsanitized -- status is always a string literal from callers ("applied"/"rejected") [permanent]
		),
	)
}
