package usecase
// white-box-reason: policy internals: tests unexported handler registration and spy test doubles

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/usecase/port"
)

type notifyCall struct {
	title   string
	message string
}

type spyNotifier struct {
	calls []notifyCall
}

type metricsCall struct {
	eventType string
	status    string
}

type spyPolicyMetrics struct {
	calls []metricsCall
}

func (s *spyPolicyMetrics) RecordPolicyEvent(_ context.Context, eventType, status string) {
	s.calls = append(s.calls, metricsCall{eventType: eventType, status: status})
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
	registerSessionPolicies(engine, logger, &port.NopNotifier{}, port.NopPolicyMetrics{})

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
	registerSessionPolicies(engine, logger, spy, port.NopPolicyMetrics{})

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

func TestPolicyHandler_WaveApplied_RecordsMetrics(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyPolicyMetrics{}
	engine := NewPolicyEngine(logger)
	registerSessionPolicies(engine, logger, &port.NopNotifier{}, spy)

	ev, err := domain.NewEvent(domain.EventWaveApplied, map[string]string{
		"wave_id": "test",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 RecordPolicyEvent call, got %d", len(spy.calls))
	}
	if spy.calls[0].eventType != "wave.applied" {
		t.Errorf("expected eventType 'wave.applied', got: %s", spy.calls[0].eventType)
	}
	if spy.calls[0].status != "handled" {
		t.Errorf("expected status 'handled', got: %s", spy.calls[0].status)
	}
}

func TestPolicyHandler_ReportSent_RecordsMetrics(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyPolicyMetrics{}
	engine := NewPolicyEngine(logger)
	registerSessionPolicies(engine, logger, &port.NopNotifier{}, spy)

	ev, err := domain.NewEvent(domain.EventReportSent, map[string]string{
		"target": "test",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 RecordPolicyEvent call, got %d", len(spy.calls))
	}
	if spy.calls[0].eventType != "report.sent" {
		t.Errorf("expected eventType 'report.sent', got: %s", spy.calls[0].eventType)
	}
}

func TestPolicyHandler_ScanCompleted_RecordsMetrics(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyPolicyMetrics{}
	engine := NewPolicyEngine(logger)
	registerSessionPolicies(engine, logger, &port.NopNotifier{}, spy)

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

	// then
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 RecordPolicyEvent call, got %d", len(spy.calls))
	}
	if spy.calls[0].eventType != "scan.completed" {
		t.Errorf("expected eventType 'scan.completed', got: %s", spy.calls[0].eventType)
	}
}

func TestPolicyHandler_WaveCompleted_RecordsMetrics(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyPolicyMetrics{}
	engine := NewPolicyEngine(logger)
	registerSessionPolicies(engine, logger, &port.NopNotifier{}, spy)

	ev, err := domain.NewEvent(domain.EventWaveCompleted, map[string]string{
		"wave_id": "test",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 RecordPolicyEvent call, got %d", len(spy.calls))
	}
	if spy.calls[0].eventType != "wave.completed" {
		t.Errorf("expected eventType 'wave.completed', got: %s", spy.calls[0].eventType)
	}
}

func TestPolicyHandler_SpecificationSent_RecordsMetrics(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyPolicyMetrics{}
	engine := NewPolicyEngine(logger)
	registerSessionPolicies(engine, logger, &port.NopNotifier{}, spy)

	ev, err := domain.NewEvent(domain.EventSpecificationSent, map[string]string{
		"target": "test",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 RecordPolicyEvent call, got %d", len(spy.calls))
	}
	if spy.calls[0].eventType != "specification.sent" {
		t.Errorf("expected eventType 'specification.sent', got: %s", spy.calls[0].eventType)
	}
}

func TestPolicyHandler_WaveCompleted_NotifiesSideEffect(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyNotifier{}
	engine := NewPolicyEngine(logger)
	registerSessionPolicies(engine, logger, spy, port.NopPolicyMetrics{})

	ev, err := domain.NewEvent(domain.EventWaveCompleted, map[string]string{
		"wave_id": "w-001",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 Notify call, got %d", len(spy.calls))
	}
	call := spy.calls[0]
	if !strings.Contains(call.title, "Sightjack") {
		t.Errorf("expected title to contain 'Sightjack', got: %s", call.title)
	}
	if !strings.Contains(call.message, "Wave completed") {
		t.Errorf("expected message to contain 'Wave completed', got: %s", call.message)
	}
}

func TestPolicyHandler_SpecificationSent_NotifiesSideEffect(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	spy := &spyNotifier{}
	engine := NewPolicyEngine(logger)
	registerSessionPolicies(engine, logger, spy, port.NopPolicyMetrics{})

	ev, err := domain.NewEvent(domain.EventSpecificationSent, map[string]string{
		"target": "architect",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}

	// when
	engine.Dispatch(context.Background(), ev)

	// then
	if len(spy.calls) != 1 {
		t.Fatalf("expected 1 Notify call, got %d", len(spy.calls))
	}
	call := spy.calls[0]
	if !strings.Contains(call.title, "Sightjack") {
		t.Errorf("expected title to contain 'Sightjack', got: %s", call.title)
	}
	if !strings.Contains(call.message, "Specification sent") {
		t.Errorf("expected message to contain 'Specification sent', got: %s", call.message)
	}
}
