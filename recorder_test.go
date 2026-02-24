package sightjack_test

import (
	"testing"

	sightjack "github.com/hironow/sightjack"
)

func TestNopRecorder_NoOp(t *testing.T) {
	// given
	var r sightjack.Recorder = sightjack.NopRecorder{}

	// when/then: should return nil without recording anything
	if err := r.Record(sightjack.EventSessionStarted, nil); err != nil {
		t.Errorf("NopRecorder should return nil, got: %v", err)
	}
}
