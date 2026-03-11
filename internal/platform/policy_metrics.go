package platform

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// OTelPolicyMetrics implements port.PolicyMetrics using OTel counters.
type OTelPolicyMetrics struct{}

func (*OTelPolicyMetrics) RecordPolicyEvent(ctx context.Context, eventType, status string) {
	c, _ := Meter.Int64Counter("sightjack.policy.event.total",
		metric.WithDescription("Policy handler execution count"),
	)
	c.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("event_type", eventType), // nosemgrep: otel-attribute-string-unsanitized -- eventType is always a string literal from policy_handlers.go [permanent]
			attribute.String("status", status),         // nosemgrep: otel-attribute-string-unsanitized -- status is always a string literal from policy_handlers.go [permanent]
		),
	)
}
