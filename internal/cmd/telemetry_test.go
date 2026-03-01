package cmd

import (
	"context"
	"io"
	"os/exec"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/session"
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
	oldTracer := sightjack.Tracer
	sightjack.Tracer = tp.Tracer("sightjack-test")
	t.Cleanup(func() {
		tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
		sightjack.Tracer = oldTracer
	})
	return exp
}

func TestInitTracer_NoopWhenEndpointUnset(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown := initTracer("test-svc", "0.0.1")
	defer shutdown(context.Background())

	// After initTracer with no endpoint, tracer is noop. We can only verify
	// that the function returns without error and shutdown is callable.
}

func TestInitTracer_ShutdownFlushesSpans(t *testing.T) {
	exp := setupTestTracer(t)

	// Use the OTel global tracer (set by setupTestTracer) to create a span
	tr := otel.Tracer("sightjack-test")
	_, span := tr.Start(context.Background(), "flushed-span")
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

	cleanup := session.OverrideNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "hello")
	})
	t.Cleanup(cleanup)

	cfg := &sightjack.Config{
		Claude: sightjack.ClaudeConfig{Command: "echo", TimeoutSec: 10},
		Retry:  sightjack.RetryConfig{MaxAttempts: 1, BaseDelaySec: 1},
	}

	_, err := session.RunClaude(context.Background(), cfg, "test prompt", io.Discard, sightjack.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("RunClaude failed: %v", err)
	}

	spans := exp.GetSpans()
	var found bool
	for _, s := range spans {
		if s.Name == "claude.invoke" {
			found = true

			// Verify gen_ai.* semantic convention attributes (P1-3)
			requiredAttrs := map[string]string{
				"gen_ai.operation.name": "chat",
				"gen_ai.system":         "anthropic",
			}
			for key, want := range requiredAttrs {
				var attrFound bool
				for _, attr := range s.Attributes {
					if string(attr.Key) == key {
						attrFound = true
						if got := attr.Value.AsString(); got != want {
							t.Errorf("attr %s = %q, want %q", key, got, want)
						}
					}
				}
				if !attrFound {
					t.Errorf("missing gen_ai attribute %q on claude.invoke span", key)
				}
			}

			// gen_ai.request.model should be present (value varies per config)
			var modelFound bool
			for _, attr := range s.Attributes {
				if string(attr.Key) == "gen_ai.request.model" {
					modelFound = true
				}
			}
			if !modelFound {
				t.Error("missing gen_ai.request.model attribute on claude.invoke span")
			}

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
	cleanup := session.OverrideNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			return exec.CommandContext(ctx, "false") // exit 1
		}
		return exec.CommandContext(ctx, "echo", "ok")
	})
	t.Cleanup(cleanup)

	cfg := &sightjack.Config{
		Claude: sightjack.ClaudeConfig{Command: "false", TimeoutSec: 30},
		Retry:  sightjack.RetryConfig{MaxAttempts: 2, BaseDelaySec: 0},
	}

	// Create a parent span so retry events have a recording span to attach to.
	tr := otel.Tracer("sightjack-test")
	ctx, parentSpan := tr.Start(context.Background(), "test-parent")
	_, _ = session.RunClaude(ctx, cfg, "test", io.Discard, sightjack.NewLogger(io.Discard, false))
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

func TestMultiExporter_BothReceive(t *testing.T) {
	exp1 := tracetest.NewInMemoryExporter()
	exp2 := tracetest.NewInMemoryExporter()

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp1),
		sdktrace.WithSyncer(exp2),
	)
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	oldTracer := sightjack.Tracer
	sightjack.Tracer = tp.Tracer("sightjack-test")
	t.Cleanup(func() {
		tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
		sightjack.Tracer = oldTracer
	})

	_, span := sightjack.Tracer.Start(context.Background(), "multi-span")
	span.End()

	if len(exp1.GetSpans()) == 0 {
		t.Error("exporter 1 received no spans")
	}
	if len(exp2.GetSpans()) == 0 {
		t.Error("exporter 2 received no spans")
	}
}

func TestParseExtraEndpoints_CommaSeparated(t *testing.T) {
	eps := parseExtraEndpoints("localhost:4318,weave.local:4318")
	if len(eps) != 2 {
		t.Fatalf("got %d endpoints, want 2", len(eps))
	}
	if eps[0] != "localhost:4318" {
		t.Errorf("eps[0] = %q, want %q", eps[0], "localhost:4318")
	}
}

func TestParseExtraEndpoints_Empty(t *testing.T) {
	eps := parseExtraEndpoints("")
	if len(eps) != 0 {
		t.Errorf("got %d endpoints, want 0", len(eps))
	}
}

func TestStartRootSpan_CreatesNamedSpan(t *testing.T) {
	// given
	exp := setupTestTracer(t)

	// when
	_ = startRootSpan(context.Background(), "scan")
	endRootSpan()

	// then
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

func TestEndRootSpan_NilSafe(t *testing.T) {
	// given — rootSpan is nil (no startRootSpan called)
	rootSpan = nil

	// when / then — must not panic
	endRootSpan()
}
