package cmd

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
	"go.opentelemetry.io/otel/trace/noop"
)

var tracer trace.Tracer = noop.NewTracerProvider().Tracer("sightjack")

func initTracer(serviceName, ver string) func(context.Context) error {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "" {
		np := noop.NewTracerProvider()
		otel.SetTracerProvider(np)
		tracer = np.Tracer(serviceName)
		return func(context.Context) error { return nil }
	}

	exp, err := otlptracehttp.New(context.Background())
	if err != nil {
		np := noop.NewTracerProvider()
		otel.SetTracerProvider(np)
		tracer = np.Tracer(serviceName)
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

// startRootSpan creates the top-level span for a sightjack subcommand and
// returns a new context carrying it. Call endRootSpan to close the span.
func startRootSpan(ctx context.Context, command string) context.Context {
	ctx, _ = tracer.Start(ctx, "sightjack."+command,
		trace.WithAttributes(
			attribute.String("sightjack.command", command),
		),
	)
	return ctx
}

// endRootSpan ends the span embedded in ctx (created by startRootSpan).
func endRootSpan(ctx context.Context) {
	span := trace.SpanFromContext(ctx)
	span.End()
}
