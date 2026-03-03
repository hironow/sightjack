package usecase

import (
	"io"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestShowFromState_NoState(t *testing.T) {
	// given: empty temp dir with no events
	baseDir := t.TempDir()
	logger := domain.NewLogger(io.Discard, false)

	// when
	err := ShowFromState(io.Discard, baseDir, logger)

	// then: should fail because no previous scan exists
	if err == nil {
		t.Fatal("expected error for missing state")
	}
}
