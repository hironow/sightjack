package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/usecase/port"
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
func RunConvergenceGate(ctx context.Context, dmails []*DMail, notifier port.Notifier, approver port.Approver, logger domain.Logger) (bool, error) {
	convergence := FilterConvergence(dmails)
	if len(convergence) == 0 {
		return true, nil
	}

	ctx, gateSpan := platform.Tracer.Start(ctx, "gate.convergence",
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
	summary := fmt.Sprintf("[CONVERGENCE] Convergence signal received: %s", strings.Join(names, ", "))

	// Notify (fire-and-forget — non-blocking with 30s timeout).
	// Use a detached context for the notification span so it is not tied to
	// the lifetime or cancellation of the gate context.
	if notifier != nil {
		notifySpanCtx := trace.ContextWithSpan(context.Background(), trace.SpanFromContext(ctx))
		go func(spanCtx context.Context, title, msg string) {
			_, notifySpan := platform.Tracer.Start(spanCtx, "notify.convergence")
			defer notifySpan.End()
			notifyCtx, cancel := context.WithTimeout(spanCtx, 30*time.Second)
			defer cancel()
			if err := notifier.Notify(notifyCtx, title, msg); err != nil {
				logger.Warn("Convergence notification failed (non-fatal): %v", err)
			}
		}(notifySpanCtx, "Sightjack Convergence", summary)
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
			trace.WithAttributes(attribute.String("error", platform.SanitizeUTF8(err.Error()))),
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

// maxRedrainCycles caps how many times the convergence gate re-drains
// the inbox. Prevents infinite looping when convergence D-Mails arrive
// faster than the approval cycle.
const maxRedrainCycles = 3

// RunConvergenceGateWithRedrain runs the convergence gate in a loop,
// re-draining the inbox channel after each approval to catch late-arriving
// convergence D-Mails. Returns the accumulated D-Mails, approval status,
// and any error. The loop exits when no new convergence D-Mails arrived
// during the approval prompt, or when maxRedrainCycles is reached
// (fail-closed: approved=false).
func RunConvergenceGateWithRedrain(ctx context.Context, initial []*DMail, inboxCh <-chan *DMail,
	notifier port.Notifier, approver port.Approver, logger domain.Logger) (dmails []*DMail, approved bool, err error) {
	all := append([]*DMail{}, initial...)
	for cycle := 0; cycle < maxRedrainCycles; cycle++ {
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
		logger.Info("[CONVERGENCE] Late convergence detected during approval, re-checking gate (%d/%d)", cycle+1, maxRedrainCycles)
	}
	// Fail-closed: convergence not confirmed after maxRedrainCycles.
	logger.Warn("[CONVERGENCE] Redrain cap reached (%d cycles) — fail-closed, convergence unconfirmed", maxRedrainCycles)
	return all, false, nil
}

// BuildNotifier creates the appropriate Notifier based on config.
// If NotifyCmd is set, uses CmdNotifier. Otherwise uses LocalNotifier (OS-native).
func BuildNotifier(gate domain.GateConfig) port.Notifier {
	if gate.HasNotifyCmd() {
		return NewCmdNotifier(gate.NotifyCmdString())
	}
	return &LocalNotifier{}
}

