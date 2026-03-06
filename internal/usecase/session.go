package usecase

import (
	"context"
	"io"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// wrapRecorder wraps a Recorder with a DispatchingRecorder if not in dry-run mode.
func wrapRecorder(recorder port.Recorder, logger domain.Logger, dryRun bool, cfg *domain.Config, metrics port.PolicyMetrics, runner port.SessionRunner) port.Recorder {
	if dryRun {
		return recorder
	}
	engine := NewPolicyEngine(logger)
	notifier := runner.BuildNotifier(cfg)
	if metrics == nil {
		metrics = port.NopPolicyMetrics{}
	}
	registerSessionPolicies(engine, logger, notifier, metrics)
	return runner.NewDispatchingRecorder(recorder, engine, logger)
}

// RunSession orchestrates the sightjack session pipeline.
// The command is always-valid by construction — no validation needed.
func RunSession(ctx context.Context, cmd domain.RunSessionCommand, cfg *domain.Config, baseDir, sessionID string, dryRun bool, input io.Reader, out io.Writer, recorder port.Recorder, logger domain.Logger, metrics port.PolicyMetrics, runner port.SessionRunner) error {
	agg := domain.NewSessionAggregate()
	emitter := NewSessionEventEmitter(agg, wrapRecorder(recorder, logger, dryRun, cfg, metrics, runner), logger)
	return runner.RunSession(ctx, cfg, baseDir, sessionID, dryRun, input, out, emitter, logger)
}

// ResumeSession orchestrates the session resume pipeline.
// The command is always-valid by construction — no validation needed.
func ResumeSession(ctx context.Context, cmd domain.ResumeSessionCommand, cfg *domain.Config, baseDir string, state *domain.SessionState, input io.Reader, out io.Writer, recorder port.Recorder, logger domain.Logger, metrics port.PolicyMetrics, runner port.SessionRunner) error {
	agg := domain.NewSessionAggregate()
	emitter := NewSessionEventEmitter(agg, wrapRecorder(recorder, logger, false, cfg, metrics, runner), logger)
	return runner.RunResumeSession(ctx, cfg, baseDir, state, input, out, emitter, logger)
}

// RescanSession orchestrates the session rescan pipeline.
// The command is always-valid by construction — no validation needed.
func RescanSession(ctx context.Context, cmd domain.RunSessionCommand, cfg *domain.Config, baseDir string, oldState *domain.SessionState, sessionID string, input io.Reader, out io.Writer, recorder port.Recorder, logger domain.Logger, metrics port.PolicyMetrics, runner port.SessionRunner) error {
	agg := domain.NewSessionAggregate()
	emitter := NewSessionEventEmitter(agg, wrapRecorder(recorder, logger, false, cfg, metrics, runner), logger)
	return runner.RunRescanSession(ctx, cfg, baseDir, oldState, sessionID, input, out, emitter, logger)
}
