package domain_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestIsValidDotCaseEventType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		// valid dot.case
		{"session.started", true},
		{"scan.completed", true},
		{"wave.approved", true},
		{"nextgen.waves.added", true},
		{"system.cutover", true},
		{"ready.labels.applied", true},
		{"adr.generated", true},
		{"completeness.updated", true},

		// invalid: snake_case
		{"session_started", false},
		{"scan_completed", false},
		{"nextgen_waves_added", false},

		// invalid: uppercase
		{"Session.Started", false},

		// invalid: dash
		{"session-started", false},

		// invalid: empty
		{"", false},

		// invalid: leading dot
		{".session.started", false},

		// invalid: trailing dot
		{"session.started.", false},

		// invalid: consecutive dots
		{"session..started", false},

		// invalid: single segment
		{"session", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := domain.IsValidDotCaseEventType(tt.input)
			if got != tt.want {
				t.Errorf("IsValidDotCaseEventType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestAllEventConstants_AreDotCase(t *testing.T) {
	// Contract: every EventType constant MUST be dot.case (SPEC-005 / GAP-INT-007).
	constants := []domain.EventType{
		domain.EventSessionStarted,
		domain.EventScanCompleted,
		domain.EventWavesGenerated,
		domain.EventWaveApproved,
		domain.EventWaveRejected,
		domain.EventWaveModified,
		domain.EventWaveApplied,
		domain.EventWaveCompleted,
		domain.EventCompletenessUpdated,
		domain.EventWavesUnlocked,
		domain.EventNextGenWavesAdded,
		domain.EventADRGenerated,
		domain.EventReadyLabelsApplied,
		domain.EventSessionResumed,
		domain.EventSessionRescanned,
		domain.EventSpecificationSent,
		domain.EventReportSent,
		domain.EventFeedbackSent,
		domain.EventFeedbackReceived,
		domain.EventWaveStalled,
		domain.EventSystemCutover,
	}

	for _, et := range constants {
		if !domain.IsValidDotCaseEventType(string(et)) {
			t.Errorf("EventType constant %q is not dot.case", et)
		}
	}
}
