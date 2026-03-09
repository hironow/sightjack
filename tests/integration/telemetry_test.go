package integration_test

import (
	"context"
	"io"
	"os/exec"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/session"
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
	oldTracer := platform.Tracer
	platform.Tracer = tp.Tracer("sightjack-test")
	t.Cleanup(func() {
		tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
		platform.Tracer = oldTracer
	})
	return exp
}

func TestSpan_RunClaude_CreatesSpan(t *testing.T) {
	exp := setupTestTracer(t)

	cleanup := session.OverrideNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "hello")
	})
	t.Cleanup(cleanup)

	cfg := &domain.Config{
		ClaudeCmd: "echo", TimeoutSec: 10,
		Retry: domain.RetryConfig{MaxAttempts: 1, BaseDelaySec: 1},
	}

	_, err := session.RunClaude(context.Background(), cfg, "test prompt", io.Discard, platform.NewLogger(io.Discard, false))
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

			// Cross-tool conformance: claude.model and claude.timeout_sec must be present
			conformanceAttrs := []string{"claude.model", "claude.timeout_sec"}
			for _, key := range conformanceAttrs {
				var attrFound bool
				for _, attr := range s.Attributes {
					if string(attr.Key) == key {
						attrFound = true
					}
				}
				if !attrFound {
					t.Errorf("missing cross-tool conformance attribute %q on claude.invoke span", key)
				}
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

	cfg := &domain.Config{
		ClaudeCmd: "false", TimeoutSec: 30,
		Retry: domain.RetryConfig{MaxAttempts: 2, BaseDelaySec: 0},
	}

	// Create a parent span so retry events have a recording span to attach to.
	tr := otel.Tracer("sightjack-test")
	ctx, parentSpan := tr.Start(context.Background(), "test-parent") // nosemgrep: adr0003-otel-span-without-defer-end -- parentSpan.End() called explicitly after RunClaude [permanent]
	_, _ = session.RunClaude(ctx, cfg, "test", io.Discard, platform.NewLogger(io.Discard, false))
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
