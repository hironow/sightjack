package usecase

import (
	"context"
	"io"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// buildSessionEmitter creates a session event emitter with EventStore + PolicyEngine.
// In dry-run mode, store is nil (events are silently discarded).
func buildSessionEmitter(ctx context.Context, agg *domain.SessionAggregate, store port.EventStore, logger domain.Logger, dryRun bool, cfg *domain.Config, metrics port.PolicyMetrics, runner port.SessionRunner, sessionID string) port.SessionEventEmitter {
	var dispatcher port.EventDispatcher
	if !dryRun {
		engine := NewPolicyEngine(logger)
		notifier := runner.BuildNotifier(cfg.Gate)
		if metrics == nil {
			metrics = port.NopPolicyMetrics{}
		}
		registerSessionPolicies(engine, logger, notifier, metrics)
		dispatcher = engine
	}
	return NewSessionEventEmitter(ctx, agg, store, dispatcher, logger, sessionID)
}

// RunSession orchestrates the sightjack session pipeline.
// The command is always-valid by construction — no validation needed.
func RunSession(ctx context.Context, cmd domain.RunSessionCommand, cfg *domain.Config, baseDir, sessionID string, dryRun bool, input io.Reader, out io.Writer, store port.EventStore, logger domain.Logger, metrics port.PolicyMetrics, runner port.SessionRunner) error {
	agg := domain.NewSessionAggregate()
	emitter := buildSessionEmitter(ctx, agg, store, logger, dryRun, cfg, metrics, runner, sessionID)
	return runner.RunSession(ctx, cfg, baseDir, sessionID, dryRun, input, out, emitter, logger)
}

// ResumeSession orchestrates the session resume pipeline.
// The command is always-valid by construction — no validation needed.
func ResumeSession(ctx context.Context, cmd domain.ResumeSessionCommand, cfg *domain.Config, baseDir string, state *domain.SessionState, input io.Reader, out io.Writer, store port.EventStore, logger domain.Logger, metrics port.PolicyMetrics, runner port.SessionRunner) error {
	agg := domain.NewSessionAggregate()
	emitter := buildSessionEmitter(ctx, agg, store, logger, false, cfg, metrics, runner, state.SessionID)
	return runner.RunResumeSession(ctx, cfg, baseDir, state, input, out, emitter, logger)
}

// RescanSession orchestrates the session rescan pipeline.
// The command is always-valid by construction — no validation needed.
func RescanSession(ctx context.Context, cmd domain.RunSessionCommand, cfg *domain.Config, baseDir string, oldState *domain.SessionState, sessionID string, input io.Reader, out io.Writer, store port.EventStore, logger domain.Logger, metrics port.PolicyMetrics, runner port.SessionRunner) error {
	agg := domain.NewSessionAggregate()
	emitter := buildSessionEmitter(ctx, agg, store, logger, false, cfg, metrics, runner, sessionID)
	return runner.RunRescanSession(ctx, cfg, baseDir, oldState, sessionID, input, out, emitter, logger)
}
