package usecase

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
)

func TestPolicyHandler_ScanCompleted_InfoOutput(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	engine := NewPolicyEngine(logger)
	registerSessionPolicies(engine, logger)

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

func TestPolicyHandler_WaveApplied_DebugOnly_NoInfoOutput(t *testing.T) {
	// given: Debug-only handler should NOT produce output when verbose=false
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	engine := NewPolicyEngine(logger)
	registerSessionPolicies(engine, logger)

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
