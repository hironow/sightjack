package usecase

import (
	"io"
	"testing"

	sightjack "github.com/hironow/sightjack"
)

func TestShowFromState_NoState(t *testing.T) {
	// given: empty temp dir with no events
	baseDir := t.TempDir()
	logger := sightjack.NewLogger(io.Discard, false)

	// when
	err := ShowFromState(io.Discard, baseDir, logger)

	// then: should fail because no previous scan exists
	if err == nil {
		t.Fatal("expected error for missing state")
	}
}
