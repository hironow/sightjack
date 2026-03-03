package usecase

import (
	"context"
	"encoding/json"

	"github.com/hironow/sightjack/internal/domain"
)

// registerSessionPolicies registers POLICY handlers for session events.
// See ADR S0014 (POLICY pattern) and S0018 (Event Storming alignment).
func registerSessionPolicies(engine *PolicyEngine, logger domain.Logger) {
	engine.Register(domain.EventWaveApplied, func(_ context.Context, event domain.Event) error {
		logger.Debug("policy: wave applied (type=%s)", event.Type)
		return nil
	})

	engine.Register(domain.EventReportSent, func(_ context.Context, event domain.Event) error {
		logger.Debug("policy: report sent (type=%s)", event.Type)
		return nil
	})

	engine.Register(domain.EventScanCompleted, func(_ context.Context, event domain.Event) error {
		var data domain.ScanCompletedPayload
		if err := json.Unmarshal(event.Data, &data); err != nil {
			logger.Debug("policy: scan completed parse error: %v", err)
			return nil
		}
		logger.Info("policy: scan completed (completeness=%.1f%%, clusters=%d, shibito=%d)",
			data.Completeness*100, len(data.Clusters), data.ShibitoCount)
		return nil
	})

	engine.Register(domain.EventWaveCompleted, func(_ context.Context, event domain.Event) error {
		logger.Debug("policy: wave completed (type=%s)", event.Type)
		return nil
	})

	engine.Register(domain.EventSpecificationSent, func(_ context.Context, event domain.Event) error {
		logger.Debug("policy: specification sent (type=%s)", event.Type)
		return nil
	})
}
