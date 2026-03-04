package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/port"
)

// registerSessionPolicies registers POLICY handlers for session events.
// See ADR S0014 (POLICY pattern) and S0018 (Event Storming alignment).
func registerSessionPolicies(engine *PolicyEngine, logger domain.Logger, notifier port.Notifier, metrics port.PolicyMetrics) {
	engine.Register(domain.EventWaveApplied, func(_ context.Context, event domain.Event) error {
		logger.Debug("policy: wave applied (type=%s)", event.Type)
		return nil
	})

	engine.Register(domain.EventReportSent, func(_ context.Context, event domain.Event) error {
		logger.Debug("policy: report sent (type=%s)", event.Type)
		return nil
	})

	engine.Register(domain.EventScanCompleted, func(ctx context.Context, event domain.Event) error {
		var data domain.ScanCompletedPayload
		if err := json.Unmarshal(event.Data, &data); err != nil {
			logger.Debug("policy: scan completed parse error: %v", err)
			return nil
		}
		logger.Info("policy: scan completed (completeness=%.1f%%, clusters=%d, shibito=%d)",
			data.Completeness*100, len(data.Clusters), data.ShibitoCount)
		notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := notifier.Notify(notifyCtx, "Sightjack",
			fmt.Sprintf("Scan completed: %.1f%% (%d clusters, %d shibito)",
				data.Completeness*100, len(data.Clusters), data.ShibitoCount)); err != nil {
			logger.Debug("policy: notify error: %v", err)
		}
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
