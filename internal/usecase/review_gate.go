package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

const (
	maxReviewGateCycles = 3
	minReviewTimeout    = 30 * time.Second
)

// reviewGateRunner implements port.ReviewGateRunner using usecase logic.
type reviewGateRunner struct {
	reviewer port.ReviewExecutor
	fixer    port.ReviewFixRunner
	logger   domain.Logger
}

// NewReviewGateRunner creates a ReviewGateRunner with the given dependencies.
func NewReviewGateRunner(reviewer port.ReviewExecutor, fixer port.ReviewFixRunner, logger domain.Logger) port.ReviewGateRunner {
	return &reviewGateRunner{reviewer: reviewer, fixer: fixer, logger: logger}
}

func (r *reviewGateRunner) RunReviewGate(ctx context.Context, gate domain.GateConfig, timeoutSec int) (bool, error) {
	return RunReviewGate(ctx, gate, timeoutSec, r.reviewer, r.fixer, r.logger)
}

// RunReviewGate runs the review-fix cycle before ComposeReport.
// Returns (true, nil) if review passes or is skipped (no ReviewCmd).
// Returns (false, nil) if review fails after all cycles.
// Returns (false, err) on infrastructure errors.
//
// OTel spans are the caller's responsibility (session layer).
func RunReviewGate(
	ctx context.Context,
	gate domain.GateConfig,
	timeoutSec int,
	reviewer port.ReviewExecutor,
	fixer port.ReviewFixRunner,
	logger domain.Logger,
) (bool, error) {
	if !gate.HasReviewCmd() {
		return true, nil
	}

	if logger == nil {
		logger = &domain.NopLogger{}
	}

	budget := gate.EffectiveReviewBudget()

	if timeoutSec <= 0 {
		timeoutSec = 300
	}
	reviewTimeout := max(
		time.Duration(timeoutSec)*time.Second/time.Duration(budget),
		minReviewTimeout,
	)

	var lastComments string
	for cycle := 1; cycle <= budget; cycle++ {
		if ctx.Err() != nil {
			return false, fmt.Errorf("review gate canceled: %w", ctx.Err())
		}

		logger.Info("Review gate: cycle %d/%d", cycle, maxReviewGateCycles)

		reviewCtx, reviewCancel := context.WithTimeout(ctx, reviewTimeout)
		result, err := reviewer.RunReview(reviewCtx, gate.ReviewCmdString(), "")
		reviewCancel()
		if err != nil {
			return false, fmt.Errorf("review gate cycle %d: %w", cycle, err)
		}

		if result.Passed {
			logger.Info("Review gate: passed")
			return true, nil
		}

		lastComments = result.Comments
		logger.Warn("Review gate: comments found (cycle %d/%d)", cycle, budget)

		if cycle == budget {
			break
		}

		if err := fixer.RunReviewFix(ctx, "", "", lastComments); err != nil {
			logger.Warn("Review fix failed: %v", err)
			return false, nil
		}
	}

	logger.Warn("Review gate: exhausted %d cycles, review not resolved", budget)
	return false, nil
}
