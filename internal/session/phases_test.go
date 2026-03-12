package session

// white-box-reason: tests unexported approvalPhase discussion retry cap via internal stubRunner

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestApprovalPhase_CapsDiscussionFailures(t *testing.T) {
	// given: scanner that always picks "d" (discuss) + topic, far more than cap
	lines := strings.Repeat("d\nsome topic\n", 20)
	scanner := bufio.NewScanner(strings.NewReader(lines))
	out := &bytes.Buffer{}
	logger := &domain.NopLogger{}

	wave := domain.Wave{
		ID: "test-wave", ClusterName: "test", Title: "Test Wave",
		Status: "proposed",
		Actions: []domain.WaveAction{{IssueID: "TEST-1", Type: "add_dod", Description: "test"}},
	}

	// Failing discuss runner — counts invocations
	failCount := 0
	failingDiscuss := func(_ context.Context, _ *domain.Config, _ string,
		_ domain.Wave, _ string, _ string,
		_ io.Writer, _ domain.Logger) (*domain.ArchitectResponse, error) {
		failCount++
		return nil, fmt.Errorf("API down")
	}

	cfg := &domain.Config{}
	sessionRejected := make(map[string][]domain.WaveAction)
	completed := make(map[string]bool)
	adrCount := 0
	span := noop.Span{}

	// when
	_, result := approvalPhase(
		context.Background(), scanner,
		cfg, "", wave, "normal",
		nil, completed,
		sessionRejected, "", &adrCount,
		nil,
		nil, &port.NopSessionEventEmitter{},
		failingDiscuss,
		out, span, logger,
	)

	// then
	if result != approvalRejected {
		t.Errorf("expected approvalRejected after discuss cap, got %v", result)
	}
	if failCount != maxDiscussFailures {
		t.Errorf("expected %d discuss attempts, got %d", maxDiscussFailures, failCount)
	}
}
