package port

import (
	"context"
	"errors"

	"github.com/hironow/sightjack/internal/domain"
)

// ErrUnsupportedOS is returned by LocalNotifier on unsupported platforms.
var ErrUnsupportedOS = errors.New("notify: unsupported OS for local notifications")

// EventDispatcher dispatches domain events to policy handlers.
// Implemented by usecase.PolicyEngine; injected into session via struct field.
type EventDispatcher interface {
	Dispatch(ctx context.Context, event domain.Event) error
}

// Approver requests user approval for a convergence gate.
type Approver interface {
	RequestApproval(ctx context.Context, message string) (approved bool, err error)
}

// AutoApprover always approves without human interaction.
type AutoApprover struct{}

func (AutoApprover) RequestApproval(context.Context, string) (bool, error) { return true, nil }

// Notifier sends a notification to the user.
type Notifier interface {
	Notify(ctx context.Context, title, message string) error
}

// NopNotifier is a no-op notifier for tests and quiet mode.
type NopNotifier struct{}

func (NopNotifier) Notify(context.Context, string, string) error { return nil }

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
