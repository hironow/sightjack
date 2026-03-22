package session_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/session"
)

func TestRunRescanSession_NilOldState_ReturnsError(t *testing.T) {
	// given
	ctx := context.Background()
	cfg := domain.DefaultConfig()
	logger := platform.NewLogger(io.Discard, false)

	// when
	err := session.RunRescanSession(ctx, &cfg, t.TempDir(), nil, "test-session",
		strings.NewReader(""), io.Discard, nil, logger)

	// then
	if err == nil {
		t.Fatal("expected error for nil oldState")
	}
	if !strings.Contains(err.Error(), "oldState") {
		t.Errorf("expected error to mention 'oldState', got: %v", err)
	}
}
