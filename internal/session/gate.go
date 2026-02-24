package session

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	sightjack "github.com/hironow/sightjack"
)

// FilterConvergence returns convergence-kind D-Mails from the slice.
func FilterConvergence(dmails []*sightjack.DMail) []*sightjack.DMail {
	var result []*sightjack.DMail
	for _, m := range dmails {
		if m.Kind == sightjack.DMailConvergence {
			result = append(result, m)
		}
	}
	return result
}

// RunConvergenceGate checks for convergence D-Mails and runs the
// notify + approve flow. Returns true if approved or no convergence found.
// Returns false if denied. Returns error on failure (fail-closed).
func RunConvergenceGate(ctx context.Context, dmails []*sightjack.DMail, notifier sightjack.Notifier, approver sightjack.Approver, logger *sightjack.Logger) (bool, error) {
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
	// Use a detached context for the notification span so it is not tied to
	// the lifetime or cancellation of the gate context.
	if notifier != nil {
		notifySpanCtx := trace.ContextWithSpan(context.Background(), trace.SpanFromContext(ctx))
		go func(spanCtx context.Context, title, msg string) {
			_, notifySpan := tracer.Start(spanCtx, "notify.convergence")
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
func RunConvergenceGateWithRedrain(ctx context.Context, initial []*sightjack.DMail, inboxCh <-chan *sightjack.DMail,
	notifier sightjack.Notifier, approver sightjack.Approver, logger *sightjack.Logger) (dmails []*sightjack.DMail, approved bool, err error) {
	all := append([]*sightjack.DMail{}, initial...)
	for {
		ok, gateErr := RunConvergenceGate(ctx, all, notifier, approver, logger)
		if gateErr != nil {
			return nil, false, gateErr
		}
		if !ok {
			return nil, false, nil
		}
		late := sightjack.DrainInboxFeedback(inboxCh, logger)
		all = append(all, late...)
		if len(FilterConvergence(late)) == 0 {
			return all, true, nil
		}
		logger.Info("[CONVERGENCE] Late convergence detected during approval, re-checking gate")
	}
}

// BuildNotifier creates the appropriate Notifier based on config.
// If NotifyCmd is set, uses CmdNotifier. Otherwise uses LocalNotifier (OS-native).
func BuildNotifier(cfg *sightjack.Config) sightjack.Notifier {
	if cfg.Gate.NotifyCmd != "" {
		return NewCmdNotifier(cfg.Gate.NotifyCmd)
	}
	return &LocalNotifier{}
}

// BuildApprover creates the appropriate Approver based on config.
// Priority: AutoApprove → CmdApprover → StdinApprover.
func BuildApprover(cfg *sightjack.Config, input io.Reader, out io.Writer) sightjack.Approver {
	if cfg.Gate.AutoApprove {
		return &AutoApprover{}
	}
	if cfg.Gate.ApproveCmd != "" {
		return NewCmdApprover(cfg.Gate.ApproveCmd)
	}
	return NewStdinApprover(input, out)
}
