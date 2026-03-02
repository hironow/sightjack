package usecase

import (
	"context"

	sightjack "github.com/hironow/sightjack"
)

// registerSessionPolicies registers POLICY handlers for session events.
// See ADR S0014 (POLICY pattern) and S0018 (Event Storming alignment).
func registerSessionPolicies(engine *PolicyEngine, logger *sightjack.Logger) {
	engine.Register(sightjack.EventWaveApplied, func(_ context.Context, event sightjack.Event) error {
		logger.Debug("policy: wave applied (type=%s)", event.Type)
		return nil
	})

	engine.Register(sightjack.EventReportSent, func(_ context.Context, event sightjack.Event) error {
		logger.Debug("policy: report sent (type=%s)", event.Type)
		return nil
	})

	engine.Register(sightjack.EventScanCompleted, func(_ context.Context, event sightjack.Event) error {
		logger.Debug("policy: scan completed (type=%s)", event.Type)
		return nil
	})

	engine.Register(sightjack.EventWaveCompleted, func(_ context.Context, event sightjack.Event) error {
		logger.Debug("policy: wave completed (type=%s)", event.Type)
		return nil
	})

	engine.Register(sightjack.EventSpecificationSent, func(_ context.Context, event sightjack.Event) error {
		logger.Debug("policy: specification sent (type=%s)", event.Type)
		return nil
	})
}
