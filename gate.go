package sightjack

import (
	"context"
	"fmt"
	"strings"
	"time"

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

	// Notify (fire-and-forget — non-blocking with 30s timeout).
	if notifier != nil {
		go func(title, msg string) {
			_, notifySpan := tracer.Start(ctx, "notify.convergence")
			defer notifySpan.End()
			notifyCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := notifier.Notify(notifyCtx, title, msg); err != nil {
				logger.Warn("Convergence notification failed (non-fatal): %v", err)
			}
		}("Sightjack Convergence", summary)
	}

	// Context check before approval — early exit if already cancelled.
	if ctx.Err() != nil {
		gateSpan.AddEvent("gate.convergence.cancelled")
		return false, ctx.Err()
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

// RunConvergenceGateWithRedrain runs the convergence gate in a loop,
// re-draining the inbox channel after each approval to catch late-arriving
// convergence D-Mails. Returns the accumulated D-Mails, approval status,
// and any error. The loop exits when no new convergence D-Mails arrived
// during the approval prompt.
func RunConvergenceGateWithRedrain(ctx context.Context, initial []*DMail, inboxCh <-chan *DMail,
	notifier Notifier, approver Approver, logger *Logger) (dmails []*DMail, approved bool, err error) {
	all := append([]*DMail{}, initial...)
	for {
		ok, gateErr := RunConvergenceGate(ctx, all, notifier, approver, logger)
		if gateErr != nil {
			return nil, false, gateErr
		}
		if !ok {
			return nil, false, nil
		}
		late := DrainInboxFeedback(inboxCh, logger)
		all = append(all, late...)
		if len(FilterConvergence(late)) == 0 {
			return all, true, nil
		}
		logger.Info("[CONVERGENCE] Late convergence detected during approval, re-checking gate")
	}
}
