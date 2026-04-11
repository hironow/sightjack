package session

import (
	"context"
	"io"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/eventsource"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// --- SessionRunner adapter ---

// SessionRunnerAdapter implements port.SessionRunner by delegating to session package functions.
type SessionRunnerAdapter struct {
	reviewGateRunner port.ReviewGateRunner
}

// NewSessionRunnerAdapter creates a new SessionRunnerAdapter.
func NewSessionRunnerAdapter() *SessionRunnerAdapter { return &SessionRunnerAdapter{} }

// SetReviewGateRunner injects the review gate runner (usecase-layer logic).
func (a *SessionRunnerAdapter) SetReviewGateRunner(runner port.ReviewGateRunner) {
	a.reviewGateRunner = runner
}

// ReviewGateRunner returns the injected ReviewGateRunner (nil if not set).
func (a *SessionRunnerAdapter) ReviewGateRunner() port.ReviewGateRunner {
	return a.reviewGateRunner
}

func (a *SessionRunnerAdapter) RunSession(ctx context.Context, cfg *domain.Config, baseDir, sessionID string, dryRun bool, input io.Reader, out io.Writer, emitter port.SessionEventEmitter, logger domain.Logger) error {
	return RunSession(ctx, cfg, baseDir, sessionID, dryRun, input, out, emitter, logger)
}

func (a *SessionRunnerAdapter) RunResumeSession(ctx context.Context, cfg *domain.Config, baseDir string, state *domain.SessionState, input io.Reader, out io.Writer, emitter port.SessionEventEmitter, logger domain.Logger) error {
	return RunResumeSession(ctx, cfg, baseDir, state, input, out, emitter, logger)
}

func (a *SessionRunnerAdapter) RunRescanSession(ctx context.Context, cfg *domain.Config, baseDir string, oldState *domain.SessionState, sessionID string, input io.Reader, out io.Writer, emitter port.SessionEventEmitter, logger domain.Logger) error {
	return RunRescanSession(ctx, cfg, baseDir, oldState, sessionID, input, out, emitter, logger)
}

func (a *SessionRunnerAdapter) BuildNotifier(gate domain.GateConfig) port.Notifier {
	return BuildNotifier(gate)
}

func (a *SessionRunnerAdapter) NewDispatchingRecorder(inner port.Recorder, dispatcher port.EventDispatcher, logger domain.Logger) port.Recorder {
	return NewDispatchingRecorder(inner, dispatcher, logger)
}

// --- ScanRunner adapter ---

// ScanRunnerAdapter implements port.ScanRunner by delegating to session package functions.
type ScanRunnerAdapter struct{}

// NewScanRunnerAdapter creates a new ScanRunnerAdapter.
func NewScanRunnerAdapter() *ScanRunnerAdapter { return &ScanRunnerAdapter{} }

func (a *ScanRunnerAdapter) RunScan(ctx context.Context, cfg *domain.Config, baseDir, sessionID string, dryRun bool, streamOut io.Writer, logger domain.Logger) (*domain.ScanResult, error) {
	runner, runnerStore := NewTrackedRunner(cfg, baseDir, logger)
	if runnerStore != nil {
		defer runnerStore.Close()
	}
	onceRunner, onceStore := NewOnceRunner(cfg, baseDir, logger)
	if onceStore != nil {
		defer onceStore.Close()
	}
	return RunScan(ctx, cfg, baseDir, sessionID, dryRun, streamOut, runner, onceRunner, logger)
}

func (a *ScanRunnerAdapter) RecordScanState(baseDir, sessionID string, result *domain.ScanResult, cfg *domain.Config, emitter port.SessionEventEmitter, ts time.Time, logger domain.Logger) {
	RecordScanState(baseDir, sessionID, result, cfg, emitter, ts, logger)
}

// --- RecorderFactory adapter ---

// RecorderFactoryAdapter implements port.RecorderFactory by delegating to session package functions.
type RecorderFactoryAdapter struct {
	seqCounter *eventsource.SeqCounter
}

// NewRecorderFactoryAdapter creates a new RecorderFactoryAdapter.
func NewRecorderFactoryAdapter() *RecorderFactoryAdapter { return &RecorderFactoryAdapter{} }

// SetSeqCounter injects a SeqCounter for SeqNr allocation into new recorders.
func (f *RecorderFactoryAdapter) SetSeqCounter(sc *eventsource.SeqCounter) {
	f.seqCounter = sc
}

func (f *RecorderFactoryAdapter) SessionEventsDir(baseDir, sessionID string) string {
	return SessionEventsDir(baseDir, sessionID)
}

func (f *RecorderFactoryAdapter) NewSessionRecorder(ctx context.Context, stateDir, sessionID string, logger domain.Logger) (port.Recorder, error) {
	return NewSessionRecorderWithSeqCounter(ctx, stateDir, sessionID, logger, f.seqCounter)
}

func (f *RecorderFactoryAdapter) NewEventStore(stateDir string, logger domain.Logger) port.EventStore {
	return NewEventStore(stateDir, logger)
}

// --- StateLoader adapter ---

// StateLoaderAdapter implements port.StateLoader by delegating to session package functions.
type StateLoaderAdapter struct{}

// NewStateLoaderAdapter creates a new StateLoaderAdapter.
func NewStateLoaderAdapter() *StateLoaderAdapter { return &StateLoaderAdapter{} }

func (l *StateLoaderAdapter) LoadLatestState(ctx context.Context, baseDir string) (*domain.SessionState, string, error) {
	return LoadLatestState(ctx, baseDir)
}

func (l *StateLoaderAdapter) LoadLatestResumableState(ctx context.Context, baseDir string, match func(*domain.SessionState) bool) (*domain.SessionState, string, error) {
	return LoadLatestResumableState(ctx, baseDir, match)
}

func (l *StateLoaderAdapter) CanResume(baseDir string, state *domain.SessionState) bool {
	return CanResume(baseDir, state)
}
