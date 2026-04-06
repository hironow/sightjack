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

func TestEventTypeMapping_AllDotCaseValid(t *testing.T) {
	// Every new dot.case constant must pass validation.
	dotCaseEvents := []domain.EventType{
		domain.EventSessionStartedV2,
		domain.EventScanCompletedV2,
		domain.EventWavesGeneratedV2,
		domain.EventWaveApprovedV2,
		domain.EventWaveRejectedV2,
		domain.EventWaveModifiedV2,
		domain.EventWaveAppliedV2,
		domain.EventWaveCompletedV2,
		domain.EventCompletenessUpdatedV2,
		domain.EventWavesUnlockedV2,
		domain.EventNextGenWavesAddedV2,
		domain.EventADRGeneratedV2,
		domain.EventReadyLabelsAppliedV2,
		domain.EventSessionResumedV2,
		domain.EventSessionRescannedV2,
		domain.EventSpecificationSentV2,
		domain.EventReportSentV2,
		domain.EventFeedbackSentV2,
		domain.EventFeedbackReceivedV2,
		domain.EventWaveStalledV2,
	}

	for _, et := range dotCaseEvents {
		if !domain.IsValidDotCaseEventType(string(et)) {
			t.Errorf("dot.case constant %q fails validation", et)
		}
	}
}

func TestLegacyEventTypeAlias_ResolvesToDotCase(t *testing.T) {
	// ResolveLegacyEventType maps snake_case → dot.case.
	// Already dot.case input is returned as-is.
	tests := []struct {
		input domain.EventType
		want  domain.EventType
	}{
		{domain.EventSessionStarted, domain.EventSessionStartedV2},
		{domain.EventScanCompleted, domain.EventScanCompletedV2},
		{domain.EventWavesGenerated, domain.EventWavesGeneratedV2},
		{domain.EventSystemCutover, domain.EventSystemCutover}, // already dot.case — unchanged
		// dot.case input returned as-is
		{domain.EventSessionStartedV2, domain.EventSessionStartedV2},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			got := domain.ResolveLegacyEventType(tt.input)
			if got != tt.want {
				t.Errorf("ResolveLegacyEventType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
