package sightjack

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// tracer is the package-level tracer used by all instrumented code.
// When OTEL_EXPORTER_OTLP_ENDPOINT is unset, this remains a noop tracer (zero cost).
// Initialized to the global noop tracer so code works without calling InitTracer.
var tracer = otel.Tracer("sightjack")

// InitTracer sets up the OpenTelemetry TracerProvider.
// If OTEL_EXPORTER_OTLP_ENDPOINT is set, it creates an OTLP HTTP exporter
// with a BatchSpanProcessor. Otherwise, it uses the noop TracerProvider.
// Returns a shutdown function that flushes and closes the exporter.
func InitTracer(serviceName, ver string) func(context.Context) error {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		tracer = otel.Tracer(serviceName)
		return func(context.Context) error { return nil }
	}

	exp, err := otlptracehttp.New(context.Background())
	if err != nil {
		tracer = otel.Tracer(serviceName)
		return func(context.Context) error { return nil }
	}

	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(ver),
		),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	tracer = tp.Tracer(serviceName)

	return func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}
}

// StartRootSpan creates the top-level span for a sightjack subcommand and
// returns a new context carrying it. Call EndRootSpan to close the span.
func StartRootSpan(ctx context.Context, command string) context.Context {
	ctx, _ = tracer.Start(ctx, "sightjack."+command,
		trace.WithAttributes(
			attribute.String("sightjack.command", command),
		),
	)
	return ctx
}

// EndRootSpan ends the span embedded in ctx (created by StartRootSpan).
func EndRootSpan(ctx context.Context) {
	span := trace.SpanFromContext(ctx)
	span.End()
}
