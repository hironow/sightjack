package domain

import "regexp"

// Dot-case event type constants (SPEC-005).
// These replace the legacy snake_case names for new event emission.
// Legacy constants are retained for backward-compatible reading.
const (
	EventSessionStartedV2      EventType = "session.started"
	EventScanCompletedV2       EventType = "scan.completed"
	EventWavesGeneratedV2      EventType = "waves.generated"
	EventWaveApprovedV2        EventType = "wave.approved"
	EventWaveRejectedV2        EventType = "wave.rejected"
	EventWaveModifiedV2        EventType = "wave.modified"
	EventWaveAppliedV2         EventType = "wave.applied"
	EventWaveCompletedV2       EventType = "wave.completed"
	EventCompletenessUpdatedV2 EventType = "completeness.updated"
	EventWavesUnlockedV2       EventType = "waves.unlocked"
	EventNextGenWavesAddedV2   EventType = "nextgen.waves.added"
	EventADRGeneratedV2        EventType = "adr.generated"
	EventReadyLabelsAppliedV2  EventType = "ready.labels.applied"
	EventSessionResumedV2      EventType = "session.resumed"
	EventSessionRescannedV2    EventType = "session.rescanned"
	EventSpecificationSentV2   EventType = "specification.sent"
	EventReportSentV2          EventType = "report.sent"
	EventFeedbackSentV2        EventType = "feedback.sent"
	EventFeedbackReceivedV2    EventType = "feedback.received"
	EventWaveStalledV2         EventType = "wave.stalled"
)

// dotCaseRe matches the SPEC-005 event naming convention:
// lowercase ASCII, segments separated by dots, no underscores/dashes/uppercase.
// At least two segments required. Pattern: ^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)+$
var dotCaseRe = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)+$`)

// IsValidDotCaseEventType returns true if the string conforms to SPEC-005 dot.case naming.
func IsValidDotCaseEventType(s string) bool {
	return dotCaseRe.MatchString(s)
}

// legacyToDotCase maps legacy snake_case EventType → dot.case EventType.
var legacyToDotCase = map[EventType]EventType{
	EventSessionStarted:      EventSessionStartedV2,
	EventScanCompleted:       EventScanCompletedV2,
	EventWavesGenerated:      EventWavesGeneratedV2,
	EventWaveApproved:        EventWaveApprovedV2,
	EventWaveRejected:        EventWaveRejectedV2,
	EventWaveModified:        EventWaveModifiedV2,
	EventWaveApplied:         EventWaveAppliedV2,
	EventWaveCompleted:       EventWaveCompletedV2,
	EventCompletenessUpdated: EventCompletenessUpdatedV2,
	EventWavesUnlocked:       EventWavesUnlockedV2,
	EventNextGenWavesAdded:   EventNextGenWavesAddedV2,
	EventADRGenerated:        EventADRGeneratedV2,
	EventReadyLabelsApplied:  EventReadyLabelsAppliedV2,
	EventSessionResumed:      EventSessionResumedV2,
	EventSessionRescanned:    EventSessionRescannedV2,
	EventSpecificationSent:   EventSpecificationSentV2,
	EventReportSent:          EventReportSentV2,
	EventFeedbackSent:        EventFeedbackSentV2,
	EventFeedbackReceived:    EventFeedbackReceivedV2,
	EventWaveStalled:         EventWaveStalledV2,
}

// ResolveLegacyEventType maps a legacy snake_case EventType to its dot.case equivalent.
// If the input is already dot.case or unknown, it is returned as-is.
func ResolveLegacyEventType(et EventType) EventType {
	if mapped, ok := legacyToDotCase[et]; ok {
		return mapped
	}
	return et
}
