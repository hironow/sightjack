package session_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/session"
	"github.com/hironow/sightjack/internal/usecase/port"
)

func TestNopRecorder_NoOp(t *testing.T) {
	// given
	var r port.Recorder = port.NopRecorder{}
	ev, err := domain.NewEvent(domain.EventSessionStartedV2, struct{}{}, time.Now())
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}

	// when/then: should return nil without recording anything
	if err := r.Record(ev); err != nil {
		t.Errorf("NopRecorder should return nil, got: %v", err)
	}
}

// failingRecorder is a test stub that always returns an error.
type failingRecorder struct{}

func (failingRecorder) Record(domain.Event) error {
	return fmt.Errorf("disk full")
}

func TestLoggingRecorder_LogsErrorAndReturnsNil(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, true)
	recorder := session.NewLoggingRecorder(failingRecorder{}, logger)
	ev, evErr := domain.NewEvent(domain.EventSessionStartedV2, struct{}{}, time.Now())
	if evErr != nil {
		t.Fatalf("NewEvent: %v", evErr)
	}

	// when
	err := recorder.Record(ev)

	// then: error should be nil (swallowed) and warn should be logged
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	if !strings.Contains(buf.String(), "WARN") {
		t.Errorf("expected WARN in log output, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "disk full") {
		t.Errorf("expected 'disk full' in log output, got: %s", buf.String())
	}
}

func TestLoggingRecorder_PassesThroughOnSuccess(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, true)
	recorder := session.NewLoggingRecorder(port.NopRecorder{}, logger)
	ev, evErr := domain.NewEvent(domain.EventSessionStartedV2, struct{}{}, time.Now())
	if evErr != nil {
		t.Fatalf("NewEvent: %v", evErr)
	}

	// when
	err := recorder.Record(ev)

	// then: error should be nil and no warn should be logged
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	if strings.Contains(buf.String(), "WARN") {
		t.Errorf("expected no WARN in log output, got: %s", buf.String())
	}
}
