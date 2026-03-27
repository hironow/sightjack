package port

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

// ErrUnsupportedOS is returned by LocalNotifier on unsupported platforms.
var ErrUnsupportedOS = errors.New("notify: unsupported OS for local notifications")

// ReviewExecutor runs a code review command and returns the result.
// Implemented in session layer (exec.Command), injected into usecase by cmd.
type ReviewExecutor interface {
	RunReview(ctx context.Context, reviewCmd string, dir string) (*domain.ReviewResult, error)
}

// BranchResolver resolves the current git branch name.
// Implemented in session layer (exec.Command), injected into usecase by cmd.
type BranchResolver interface {
	CurrentBranch(ctx context.Context, dir string) (string, error)
}

// ReviewFixRunner runs Claude to fix review comments.
// Implemented in session layer, injected into usecase by cmd.
type ReviewFixRunner interface {
	RunReviewFix(ctx context.Context, dir, branch, comments string) error
}

// ReviewGateRunner runs the review-fix cycle.
// Implemented in usecase layer, injected into session by cmd (composition root).
type ReviewGateRunner interface {
	RunReviewGate(ctx context.Context, gate domain.GateConfig, timeoutSec int) (bool, error)
}

// InitRunner handles project initialization I/O.
// Returns warnings for non-fatal errors (e.g. skill install failures).
type InitRunner interface {
	InitProject(baseDir, team, project, lang, strictness string) (warnings []string, err error)
}

// EventDispatcher dispatches domain events to policy handlers.
// Implemented by usecase.PolicyEngine; injected into session via struct field.
type EventDispatcher interface {
	Dispatch(ctx context.Context, event domain.Event) error
}

// Approver determines whether an action should proceed.
// Implementations include StdinApprover (human prompt),
// CmdApprover (external command), and AutoApprover (always yes).
type Approver interface {
	RequestApproval(ctx context.Context, message string) (approved bool, err error)
}

// AutoApprover always approves without human interaction.
type AutoApprover struct{}

func (*AutoApprover) RequestApproval(_ context.Context, _ string) (bool, error) { return true, nil }

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
	Append(events ...domain.Event) (domain.AppendResult, error)

	// LoadAll returns all events in chronological order.
	LoadAll() ([]domain.Event, domain.LoadResult, error)

	// LoadSince returns events with timestamps after the given time.
	LoadSince(after time.Time) ([]domain.Event, domain.LoadResult, error)
}

// OutboxStore provides transactional outbox semantics for D-Mail delivery.
// Stage records intent in a durable store; Flush materializes staged items
// to the filesystem (archive/ + outbox/) using atomic writes.
type OutboxStore interface {
	// Stage atomically records a D-Mail for delivery. Idempotent: re-staging
	// the same name is a no-op.
	Stage(ctx context.Context, name string, data []byte) error

	// Flush writes all staged-but-unflushed D-Mails to archive/ and outbox/.
	// Returns the number of items flushed.
	Flush(ctx context.Context) (int, error)

	// Close releases database resources.
	Close() error
}

// Recorder records domain events during a session.
type Recorder interface {
	Record(ev domain.Event) error
}

// NopRecorder is a no-op Recorder for dry-run mode and testing.
type NopRecorder struct{}

// Record always returns nil without recording anything.
func (NopRecorder) Record(domain.Event) error { return nil }

// SessionEventEmitter wraps aggregate event production + recording.
// Implemented in usecase layer, injected into session by cmd (composition root).
// Record errors are best-effort (logged, not propagated) to preserve session continuity.
type SessionEventEmitter interface {
	EmitStart(project, strictness string, now time.Time) error
	EmitRecordScan(payload domain.ScanCompletedPayload, now time.Time) error
	EmitResume(originalSessionID string, now time.Time) error
	EmitRescan(originalSessionID string, now time.Time) error
	EmitRecordWavesGenerated(payload domain.WavesGeneratedPayload, now time.Time) error
	EmitApproveWave(waveID, clusterName string, now time.Time) error
	EmitRejectWave(waveID, clusterName string, now time.Time) error
	EmitModifyWave(payload domain.WaveModifiedPayload, now time.Time) error
	EmitApplyWave(payload domain.WaveAppliedPayload, now time.Time) error
	EmitCompleteWave(payload domain.WaveCompletedPayload, now time.Time) error
	EmitUpdateCompleteness(clusterName string, clusterC, overallC float64, now time.Time) error
	EmitUnlockWaves(unlockedIDs []string, now time.Time) error
	EmitAddNextGenWaves(payload domain.NextGenWavesAddedPayload, now time.Time) error
	EmitApplyReadyLabels(payload domain.ReadyLabelsAppliedPayload, now time.Time) error
	EmitSendSpecification(waveID, clusterName string, now time.Time) error
	EmitSendReport(waveID, clusterName string, now time.Time) error
	EmitSendFeedback(waveID, clusterName string, now time.Time) error
	EmitReceiveFeedback(payload domain.FeedbackReceivedPayload, now time.Time) error
	EmitGenerateADR(payload domain.ADRGeneratedPayload, now time.Time) error
}

// NopSessionEventEmitter is a no-op emitter for tests and dry-run mode.
type NopSessionEventEmitter struct{}

func (*NopSessionEventEmitter) EmitStart(string, string, time.Time) error { return nil }
func (*NopSessionEventEmitter) EmitRecordScan(domain.ScanCompletedPayload, time.Time) error {
	return nil
}
func (*NopSessionEventEmitter) EmitResume(string, time.Time) error { return nil }
func (*NopSessionEventEmitter) EmitRescan(string, time.Time) error { return nil }
func (*NopSessionEventEmitter) EmitRecordWavesGenerated(domain.WavesGeneratedPayload, time.Time) error {
	return nil
}
func (*NopSessionEventEmitter) EmitApproveWave(string, string, time.Time) error { return nil }
func (*NopSessionEventEmitter) EmitRejectWave(string, string, time.Time) error  { return nil }
func (*NopSessionEventEmitter) EmitModifyWave(domain.WaveModifiedPayload, time.Time) error {
	return nil
}
func (*NopSessionEventEmitter) EmitApplyWave(domain.WaveAppliedPayload, time.Time) error {
	return nil
}
func (*NopSessionEventEmitter) EmitCompleteWave(domain.WaveCompletedPayload, time.Time) error {
	return nil
}
func (*NopSessionEventEmitter) EmitUpdateCompleteness(string, float64, float64, time.Time) error {
	return nil
}
func (*NopSessionEventEmitter) EmitUnlockWaves([]string, time.Time) error { return nil }
func (*NopSessionEventEmitter) EmitAddNextGenWaves(domain.NextGenWavesAddedPayload, time.Time) error {
	return nil
}
func (*NopSessionEventEmitter) EmitApplyReadyLabels(domain.ReadyLabelsAppliedPayload, time.Time) error {
	return nil
}
func (*NopSessionEventEmitter) EmitSendSpecification(string, string, time.Time) error { return nil }
func (*NopSessionEventEmitter) EmitSendReport(string, string, time.Time) error        { return nil }
func (*NopSessionEventEmitter) EmitSendFeedback(string, string, time.Time) error { return nil }
func (*NopSessionEventEmitter) EmitReceiveFeedback(domain.FeedbackReceivedPayload, time.Time) error {
	return nil
}
func (*NopSessionEventEmitter) EmitGenerateADR(domain.ADRGeneratedPayload, time.Time) error {
	return nil
}

// SessionRunner runs interactive sightjack sessions (scan->waves->select->apply->nextgen loop).
type SessionRunner interface {
	RunSession(ctx context.Context, cfg *domain.Config, baseDir, sessionID string, dryRun bool, input io.Reader, out io.Writer, emitter SessionEventEmitter, logger domain.Logger) error
	RunResumeSession(ctx context.Context, cfg *domain.Config, baseDir string, state *domain.SessionState, input io.Reader, out io.Writer, emitter SessionEventEmitter, logger domain.Logger) error
	RunRescanSession(ctx context.Context, cfg *domain.Config, baseDir string, oldState *domain.SessionState, sessionID string, input io.Reader, out io.Writer, emitter SessionEventEmitter, logger domain.Logger) error
	SetReviewGateRunner(runner ReviewGateRunner)
	BuildNotifier(gate domain.GateConfig) Notifier
	NewDispatchingRecorder(inner Recorder, dispatcher EventDispatcher, logger domain.Logger) Recorder
}

// ScanRunner executes scans and records scan state.
type ScanRunner interface {
	RunScan(ctx context.Context, cfg *domain.Config, baseDir, sessionID string, dryRun bool, streamOut io.Writer, logger domain.Logger) (*domain.ScanResult, error)
	RecordScanState(baseDir, sessionID string, result *domain.ScanResult, cfg *domain.Config, emitter SessionEventEmitter, ts time.Time, logger domain.Logger)
}

// RecorderFactory creates session recorders and resolves event directories.
type RecorderFactory interface {
	SessionEventsDir(baseDir, sessionID string) string
	NewSessionRecorder(stateDir, sessionID string, logger domain.Logger) (Recorder, error)
	NewEventStore(stateDir string, logger domain.Logger) EventStore
}

// StateLoader loads session state from the filesystem.
type StateLoader interface {
	LoadLatestState(ctx context.Context, baseDir string) (*domain.SessionState, string, error)
	LoadLatestResumableState(ctx context.Context, baseDir string, match func(*domain.SessionState) bool) (*domain.SessionState, string, error)
	CanResume(baseDir string, state *domain.SessionState) bool
}
