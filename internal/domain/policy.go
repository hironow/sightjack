package domain

// Policy represents an implicit reactive rule: WHEN [EVENT] THEN [COMMAND].
// See ADR S0014 for the POLICY pattern reference.
type Policy struct {
	Name    string    // unique identifier for the policy
	Trigger EventType // domain event that activates this policy
	Action  string    // description of the resulting command
}

// Policies registers all known implicit policies in sightjack.
// These document the existing reactive behaviors for future automation.
var Policies = []Policy{
	{Name: "WaveAppliedComposeReport", Trigger: EventWaveApplied, Action: "ComposeReport"},
	{Name: "ReportSentDeliverToPhonewave", Trigger: EventReportSent, Action: "DeliverViaPhonewave"},
	{Name: "ScanCompletedGenerateWaves", Trigger: EventScanCompleted, Action: "GenerateWaves"},
	{Name: "WaveCompletedNextGen", Trigger: EventWaveCompleted, Action: "GenerateNextWaves"},
	{Name: "SpecificationSentDeliverToPhonewave", Trigger: EventSpecificationSent, Action: "DeliverViaPhonewave"},
}
