package integration_test

import (
	"context"
	"fmt"
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

func TestSpan_RunClaude_RecordsRetryEvent(t *testing.T) {
	exp := setupTestTracer(t)

	okNDJSON := `{"type":"result","subtype":"success","session_id":"mock","result":"ok","is_error":false,"num_turns":1,"duration_ms":1,"total_cost_usd":0,"usage":{"input_tokens":1,"output_tokens":1},"stop_reason":"end_turn"}`
	callCount := 0
	cleanup := session.OverrideNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			return exec.CommandContext(ctx, "sh", "-c", "cat > /dev/null && exit 1") // drain stdin, exit 1
		}
		return exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("cat > /dev/null && printf '%%s' '%s'", okNDJSON))
	})
	t.Cleanup(cleanup)

	cfg := &domain.Config{
		ClaudeCmd: "false", TimeoutSec: 30,
		Retry: domain.RetryConfig{MaxAttempts: 2, BaseDelaySec: 0},
	}

	// Create a parent span so retry events have a recording span to attach to.
	tr := otel.Tracer("sightjack-test")
	ctx, parentSpan := tr.Start(context.Background(), "test-parent") // nosemgrep: adr0003-otel-span-without-defer-end -- parentSpan.End() called explicitly after RunClaude [permanent]
	retryLogger := platform.NewLogger(io.Discard, false)
	retryAdapter := session.NewClaudeAdapter(cfg, retryLogger)
	retryRunner := session.NewRetryRunner(retryAdapter, cfg, retryLogger)
	_, _ = retryRunner.Run(ctx, "test", io.Discard)
	parentSpan.End()

	spans := exp.GetSpans()
	var retryFound bool
	for _, s := range spans {
		for _, ev := range s.Events {
			if ev.Name == "provider.retry" {
				retryFound = true
			}
		}
	}
	if !retryFound {
		t.Error("expected 'provider.retry' event")
	}
}
