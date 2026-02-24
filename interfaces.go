package sightjack

import "context"

// EventStore is the interface for an append-only event log.
type EventStore interface {
	Append(events ...Event) error
	ReadAll() ([]Event, error)
	ReadSince(afterSeq int64) ([]Event, error)
	LastSequence() (int64, error)
}

// Recorder records domain events during a session.
// session.go depends only on this interface, never on concrete implementations.
type Recorder interface {
	Record(eventType EventType, payload any) error
}

// Notifier sends a notification to the user.
type Notifier interface {
	Notify(ctx context.Context, title, message string) error
}

// Approver requests user approval for a convergence gate.
type Approver interface {
	RequestApproval(ctx context.Context, message string) (approved bool, err error)
}

// Handoff defines the integration contract for downstream execution agents (v1.0).
// Implementations receive ready issue IDs and execute them via Claude Code agents.
type Handoff interface {
	// HandoffReady delivers a batch of ready issue IDs to a downstream agent
	// for autonomous execution. Returns an error if the handoff fails.
	HandoffReady(ctx context.Context, issueIDs []string) error

	// ReportIssue reports a finding (e.g. blocker, question, anomaly) back
	// to the orchestrator for a specific issue during execution.
	ReportIssue(ctx context.Context, issueID string, finding string) error
}

// HandoffResult tracks the outcome of a handoff for a single issue.
type HandoffResult struct {
	IssueID string // Linear issue identifier
	Status  string // "success", "failed", "skipped"
	Error   string // non-empty when Status is "failed"
}
