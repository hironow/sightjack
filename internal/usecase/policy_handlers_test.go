package usecase

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/port"
)

type notifyCall struct {
	title   string
	message string
}

type spyNotifier struct {
	calls []notifyCall
}

func (s *spyNotifier) Notify(_ context.Context, title, message string) error {
	s.calls = append(s.calls, notifyCall{title: title, message: message})
	return nil
}

func TestPolicyHandler_ScanCompleted_InfoOutput(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	engine := NewPolicyEngine(logger)
	registerSessionPolicies(engine, logger, &port.NopNotifier{})

	ev, err := domain.NewEvent(domain.EventScanCompleted, domain.ScanCompletedPayload{
		Clusters:     []domain.ClusterState{{Name: "auth", Completeness: 0.8, IssueCount: 3}},
		Completeness: 0.75,
		ShibitoCount: 2,
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then: Info-level output should contain completeness and cluster count
	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Errorf("expected INFO level output, got: %s", output)
	}
	if !strings.Contains(output, "75.0%") {
		t.Errorf("expected completeness percentage in output, got: %s", output)
	}
	if !strings.Contains(output, "clusters=1") {
		t.Errorf("expected cluster count in output, got: %s", output)
	}
}

func TestPolicyHandler_ScanCompleted_NotifiesSideEffect(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyNotifier{}
	engine := NewPolicyEngine(logger)
	registerSessionPolicies(engine, logger, spy)

	ev, err := domain.NewEvent(domain.EventScanCompleted, domain.ScanCompletedPayload{
		Clusters:     []domain.ClusterState{{Name: "auth", Completeness: 0.8, IssueCount: 3}},
		Completeness: 0.75,
		ShibitoCount: 2,
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then: Notify should have been called once
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 Notify call, got %d", len(spy.calls))
	}
	call := spy.calls[0]
	if !strings.Contains(call.title, "Sightjack") {
		t.Errorf("expected title to contain 'Sightjack', got: %s", call.title)
	}
	if !strings.Contains(call.message, "75.0%") {
		t.Errorf("expected message to contain completeness, got: %s", call.message)
	}
}

func TestPolicyHandler_WaveApplied_DebugOnly_NoInfoOutput(t *testing.T) {
	// given: Debug-only handler should NOT produce output when verbose=false
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	engine := NewPolicyEngine(logger)
	registerSessionPolicies(engine, logger, &port.NopNotifier{})

	ev, err := domain.NewEvent(domain.EventWaveApplied, map[string]string{
		"wave_id": "test",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then: no output
	output := buf.String()
	if output != "" {
		t.Errorf("expected no output for Debug-only handler with verbose=false, got: %s", output)
	}
}
