package sightjack

import (
	"context"
	"io"
	"os/exec"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// setupTestTracer installs an InMemoryExporter with a synchronous span processor
// so spans are immediately available for inspection. It restores the global
// TracerProvider after the test.
func setupTestTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	tracer = tp.Tracer("sightjack-test")
	t.Cleanup(func() {
		tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
		tracer = prev.Tracer("sightjack")
	})
	return exp
}

func TestInitTracer_NoopWhenEndpointUnset(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown := InitTracer("test-svc", "0.0.1")
	defer shutdown(context.Background())

	_, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	if span.IsRecording() {
		t.Error("span should NOT be recording when endpoint is unset (noop provider)")
	}
}

func TestInitTracer_ShutdownFlushesSpans(t *testing.T) {
	exp := setupTestTracer(t)

	_, span := tracer.Start(context.Background(), "flushed-span")
	span.End()

	spans := exp.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least 1 span in InMemoryExporter after span.End()")
	}
	if spans[0].Name != "flushed-span" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "flushed-span")
	}
}

func TestSpan_RunClaude_CreatesSpan(t *testing.T) {
	exp := setupTestTracer(t)

	origNewCmd := newCmd
	newCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "hello")
	}
	t.Cleanup(func() { newCmd = origNewCmd })

	cfg := &Config{
		Claude: ClaudeConfig{Command: "echo", TimeoutSec: 10},
		Retry:  RetryConfig{MaxAttempts: 1, BaseDelaySec: 1},
	}

	_, err := RunClaude(context.Background(), cfg, "test prompt", io.Discard, NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("RunClaude failed: %v", err)
	}

	spans := exp.GetSpans()
	var found bool
	for _, s := range spans {
		if s.Name == "claude.invoke" {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, len(spans))
		for i, s := range spans {
			names[i] = s.Name
		}
		t.Errorf("expected 'claude.invoke' span, got: %v", names)
	}
}

func TestSpan_RunClaude_RecordsRetryEvent(t *testing.T) {
	exp := setupTestTracer(t)

	callCount := 0
	origNewCmd := newCmd
	newCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			return exec.CommandContext(ctx, "false") // exit 1
		}
		return exec.CommandContext(ctx, "echo", "ok")
	}
	t.Cleanup(func() { newCmd = origNewCmd })

	cfg := &Config{
		Claude: ClaudeConfig{Command: "false", TimeoutSec: 30},
		Retry:  RetryConfig{MaxAttempts: 2, BaseDelaySec: 0},
	}

	// Create a parent span so retry events have a recording span to attach to.
	ctx, parentSpan := tracer.Start(context.Background(), "test-parent")
	_, _ = RunClaude(ctx, cfg, "test", io.Discard, NewLogger(io.Discard, false))
	parentSpan.End()

	spans := exp.GetSpans()
	var retryFound bool
	for _, s := range spans {
		for _, ev := range s.Events {
			if ev.Name == "claude.retry" {
				retryFound = true
			}
		}
	}
	if !retryFound {
		t.Error("expected 'claude.retry' event")
	}
}

func TestStartRootSpan_CreatesNamedSpan(t *testing.T) {
	exp := setupTestTracer(t)

	ctx := StartRootSpan(context.Background(), "scan")
	EndRootSpan(ctx)

	spans := exp.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least 1 span")
	}
	if spans[0].Name != "sightjack.scan" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "sightjack.scan")
	}
	var found bool
	for _, attr := range spans[0].Attributes {
		if string(attr.Key) == "sightjack.command" && attr.Value.AsString() == "scan" {
			found = true
		}
	}
	if !found {
		t.Error("expected sightjack.command=scan attribute on root span")
	}
}
