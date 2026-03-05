package session

import (
	"context"
	"io"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// --- SessionRunner adapter ---

// SessionRunnerAdapter implements port.SessionRunner by delegating to session package functions.
type SessionRunnerAdapter struct{}

// NewSessionRunnerAdapter creates a new SessionRunnerAdapter.
func NewSessionRunnerAdapter() *SessionRunnerAdapter { return &SessionRunnerAdapter{} }

func (a *SessionRunnerAdapter) RunSession(ctx context.Context, cfg *domain.Config, baseDir, sessionID string, dryRun bool, input io.Reader, out io.Writer, recorder port.Recorder, agg *domain.SessionAggregate, logger domain.Logger) error {
	return RunSession(ctx, cfg, baseDir, sessionID, dryRun, input, out, recorder, agg, logger)
}

func (a *SessionRunnerAdapter) RunResumeSession(ctx context.Context, cfg *domain.Config, baseDir string, state *domain.SessionState, input io.Reader, out io.Writer, recorder port.Recorder, agg *domain.SessionAggregate, logger domain.Logger) error {
	return RunResumeSession(ctx, cfg, baseDir, state, input, out, recorder, agg, logger)
}

func (a *SessionRunnerAdapter) RunRescanSession(ctx context.Context, cfg *domain.Config, baseDir string, oldState *domain.SessionState, sessionID string, input io.Reader, out io.Writer, recorder port.Recorder, agg *domain.SessionAggregate, logger domain.Logger) error {
	return RunRescanSession(ctx, cfg, baseDir, oldState, sessionID, input, out, recorder, agg, logger)
}

func (a *SessionRunnerAdapter) BuildNotifier(cfg *domain.Config) port.Notifier {
	return BuildNotifier(cfg)
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
	return RunScan(ctx, cfg, baseDir, sessionID, dryRun, streamOut, logger)
}

func (a *ScanRunnerAdapter) RecordScanState(baseDir, sessionID string, result *domain.ScanResult, cfg *domain.Config, recorder port.Recorder, agg *domain.SessionAggregate, ts time.Time, logger domain.Logger) {
	RecordScanState(baseDir, sessionID, result, cfg, recorder, agg, ts, logger)
}

// --- RecorderFactory adapter ---

// RecorderFactoryAdapter implements port.RecorderFactory by delegating to session package functions.
type RecorderFactoryAdapter struct{}

// NewRecorderFactoryAdapter creates a new RecorderFactoryAdapter.
func NewRecorderFactoryAdapter() *RecorderFactoryAdapter { return &RecorderFactoryAdapter{} }

func (f *RecorderFactoryAdapter) SessionEventsDir(baseDir, sessionID string) string {
	return SessionEventsDir(baseDir, sessionID)
}

func (f *RecorderFactoryAdapter) NewSessionRecorder(stateDir, sessionID string, logger domain.Logger) (port.Recorder, error) {
	return NewSessionRecorder(stateDir, sessionID, logger)
}

func (f *RecorderFactoryAdapter) NewEventStore(stateDir string, logger domain.Logger) port.EventStore {
	return NewEventStore(stateDir, logger)
}

// --- StateLoader adapter ---

// StateLoaderAdapter implements port.StateLoader by delegating to session package functions.
type StateLoaderAdapter struct{}

// NewStateLoaderAdapter creates a new StateLoaderAdapter.
func NewStateLoaderAdapter() *StateLoaderAdapter { return &StateLoaderAdapter{} }

func (l *StateLoaderAdapter) LoadLatestState(baseDir string) (*domain.SessionState, string, error) {
	return LoadLatestState(baseDir)
}

func (l *StateLoaderAdapter) LoadLatestResumableState(baseDir string, match func(*domain.SessionState) bool) (*domain.SessionState, string, error) {
	return LoadLatestResumableState(baseDir, match)
}

func (l *StateLoaderAdapter) CanResume(baseDir string, state *domain.SessionState) bool {
	return CanResume(baseDir, state)
}
