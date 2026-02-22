package sightjack

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// FilterConvergence returns convergence-kind D-Mails from the slice.
func FilterConvergence(dmails []*DMail) []*DMail {
	var result []*DMail
	for _, m := range dmails {
		if m.Kind == DMailConvergence {
			result = append(result, m)
		}
	}
	return result
}

// RunConvergenceGate checks for convergence D-Mails and runs the
// notify + approve flow. Returns true if approved or no convergence found.
// Returns false if denied. Returns error on failure (fail-closed).
func RunConvergenceGate(ctx context.Context, dmails []*DMail, notifier Notifier, approver Approver, logger *Logger) (bool, error) {
	convergence := FilterConvergence(dmails)
	if len(convergence) == 0 {
		return true, nil
	}

	ctx, gateSpan := tracer.Start(ctx, "gate.convergence",
		trace.WithAttributes(
			attribute.Int("gate.convergence.count", len(convergence)),
		),
	)
	defer gateSpan.End()

	// Build summary message from convergence d-mails.
	var names []string
	for _, m := range convergence {
		names = append(names, m.Name)
	}
	summary := fmt.Sprintf("Convergence signal received: %s", strings.Join(names, ", "))

	// Notify (fire-and-forget — log warning on failure).
	if notifier != nil {
		_, notifySpan := tracer.Start(ctx, "notify.convergence")
		if err := notifier.Notify(ctx, "Sightjack Convergence", summary); err != nil {
			logger.Warn("Convergence notification failed (non-fatal): %v", err)
		}
		notifySpan.End()
	}

	// Approve (blocking — fail-closed on error).
	approved, err := approver.RequestApproval(ctx, summary)
	if err != nil {
		gateSpan.AddEvent("gate.convergence.error",
			trace.WithAttributes(attribute.String("error", err.Error())),
		)
		return false, fmt.Errorf("convergence approval: %w", err)
	}
	if !approved {
		gateSpan.AddEvent("gate.convergence.denied")
		return false, nil
	}
	gateSpan.AddEvent("gate.convergence.approved")
	return true, nil
}
