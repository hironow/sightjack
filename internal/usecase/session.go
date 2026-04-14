package usecase

import (
	"context"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// BuildSessionEmitter creates a session event emitter with EventStore + PolicyEngine.
// Called by cmd (composition root) to wire up the session pipeline.
// In dry-run mode, dispatcher is nil (policy handlers are not invoked).
func BuildSessionEmitter(ctx context.Context, store port.EventStore, logger domain.Logger, dryRun bool, cfg *domain.Config, metrics port.PolicyMetrics, runner port.SessionRunner, sessionID string) port.SessionEventEmitter {
	agg := domain.NewSessionAggregate()
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
