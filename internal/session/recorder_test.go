package session_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func TestNopRecorder_NoOp(t *testing.T) {
	// given
	var r domain.Recorder = session.NopRecorder{}

	// when/then: should return nil without recording anything
	if err := r.Record(domain.EventSessionStarted, nil); err != nil {
		t.Errorf("NopRecorder should return nil, got: %v", err)
	}
}

// failingRecorder is a test stub that always returns an error.
type failingRecorder struct{}

func (failingRecorder) Record(domain.EventType, any) error {
	return fmt.Errorf("disk full")
}

func TestLoggingRecorder_LogsErrorAndReturnsNil(t *testing.T) {
	// given
	var buf bytes.Buffer
	logger := domain.NewLogger(&buf, true)
	recorder := session.NewLoggingRecorder(failingRecorder{}, logger)

	// when
	err := recorder.Record(domain.EventSessionStarted, nil)

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
	logger := domain.NewLogger(&buf, true)
	recorder := session.NewLoggingRecorder(session.NopRecorder{}, logger)

	// when
	err := recorder.Record(domain.EventSessionStarted, nil)

	// then: error should be nil and no warn should be logged
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	if strings.Contains(buf.String(), "WARN") {
		t.Errorf("expected no WARN in log output, got: %s", buf.String())
	}
}
