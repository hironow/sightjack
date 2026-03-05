package domain

// HandoffResult tracks the outcome of a handoff for a single issue.
type HandoffResult struct {
	IssueID string // Linear issue identifier
	Status  string // "success", "failed", "skipped"
	Error   string // non-empty when Status is "failed"
}
