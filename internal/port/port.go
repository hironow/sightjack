package port

import (
	"context"
	"errors"
	"time"

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

// PolicyMetrics records policy handler execution metrics.
type PolicyMetrics interface {
	RecordPolicyEvent(ctx context.Context, eventType string, status string)
}

// NopPolicyMetrics is a no-op metrics recorder for tests and quiet mode.
type NopPolicyMetrics struct{}

func (NopPolicyMetrics) RecordPolicyEvent(context.Context, string, string) {}

// EventStore is the append-only event persistence interface.
type EventStore interface {
	// Append persists one or more events. Validation is performed before any writes.
	Append(events ...domain.Event) error

	// LoadAll returns all events in chronological order.
	LoadAll() ([]domain.Event, error)

	// LoadSince returns events with timestamps after the given time.
	LoadSince(after time.Time) ([]domain.Event, error)
}

// OutboxStore provides transactional outbox semantics for D-Mail delivery.
// Stage records intent in a durable store; Flush materializes staged items
// to the filesystem (archive/ + outbox/) using atomic writes.
type OutboxStore interface {
	// Stage atomically records a D-Mail for delivery. Idempotent: re-staging
	// the same name is a no-op.
	Stage(name string, data []byte) error

	// Flush writes all staged-but-unflushed D-Mails to archive/ and outbox/.
	// Returns the number of items flushed.
	Flush() (int, error)

	// Close releases database resources.
	Close() error
}

// Recorder records domain events during a session.
type Recorder interface {
	Record(eventType domain.EventType, payload any) error
}

// NopRecorder is a no-op Recorder for dry-run mode and testing.
type NopRecorder struct{}

// Record always returns nil without recording anything.
func (NopRecorder) Record(domain.EventType, any) error { return nil }
