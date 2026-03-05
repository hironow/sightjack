package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// registerSessionPolicies registers POLICY handlers for session events.
// See ADR S0014 (POLICY pattern) and S0018 (Event Storming alignment).
func registerSessionPolicies(engine *PolicyEngine, logger domain.Logger, notifier port.Notifier, metrics port.PolicyMetrics) {
	// POLICY CONTRACT: observation-only — debug log + metrics.
	// Wave application is an intermediate session step; user sees results
	// interactively and scan.completed provides the summary notification.
	engine.Register(domain.EventWaveApplied, func(ctx context.Context, event domain.Event) error {
		logger.Debug("policy: wave applied (type=%s)", event.Type)
		metrics.RecordPolicyEvent(ctx, "wave.applied", "handled")
		return nil
	})

	// POLICY CONTRACT: observation-only — debug log + metrics.
	// Report delivery is part of the session flow; user sees it inline.
	engine.Register(domain.EventReportSent, func(ctx context.Context, event domain.Event) error {
		logger.Debug("policy: report sent (type=%s)", event.Type)
		metrics.RecordPolicyEvent(ctx, "report.sent", "handled")
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
		metrics.RecordPolicyEvent(ctx, "scan.completed", "handled")
		return nil
	})

	// POLICY: wave.completed → notify + metrics.
	// Wave completion is a milestone in the session flow.
	engine.Register(domain.EventWaveCompleted, func(ctx context.Context, event domain.Event) error {
		logger.Info("policy: wave completed (type=%s)", event.Type)
		notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := notifier.Notify(notifyCtx, "Sightjack", "Wave completed"); err != nil {
			logger.Debug("policy: notify error: %v", err)
		}
		metrics.RecordPolicyEvent(ctx, "wave.completed", "handled")
		return nil
	})

	// POLICY: specification.sent → notify + metrics.
	engine.Register(domain.EventSpecificationSent, func(ctx context.Context, event domain.Event) error {
		logger.Info("policy: specification sent (type=%s)", event.Type)
		notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := notifier.Notify(notifyCtx, "Sightjack", "Specification sent"); err != nil {
			logger.Debug("policy: notify error: %v", err)
		}
		metrics.RecordPolicyEvent(ctx, "specification.sent", "handled")
		return nil
	})
}
