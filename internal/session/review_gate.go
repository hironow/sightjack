package session

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/usecase/port"
)

const (
	maxReviewGateCycles = 3
	minReviewTimeout    = 30 * time.Second
)

// RunReviewGate runs the review-fix cycle with OTel spans.
// When a ReviewGateRunner is provided (variadic), delegates cycle control to it.
// Otherwise runs cycle control inline (backward compatibility for integration tests).
func RunReviewGate(ctx context.Context, gate domain.GateConfig, cfg *domain.Config, runner port.ClaudeRunner, dir string, logger domain.Logger, reviewGate ...port.ReviewGateRunner) (bool, error) {
	ctx, span := platform.Tracer.Start(ctx, "sightjack.review")
	defer span.End()

	if !gate.HasReviewCmd() {
		return true, nil
	}

	timeoutSec := cfg.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 300
	}

	// Prefer injected ReviewGateRunner (production path via cmd composition root)
	if len(reviewGate) > 0 && reviewGate[0] != nil {
		passed, err := reviewGate[0].RunReviewGate(ctx, gate, timeoutSec)
		if err != nil {
			span.RecordError(err)
			span.SetAttributes(attribute.String("error.stage", "sightjack.review"))
		}
		return passed, err
	}

	// Fallback: construct adapters and run cycle control inline
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	reviewer := NewReviewExecutor(dir)
	branch := NewBranchResolver()
	fixer := NewReviewFixRunner(runner, branch, dir, logger)
	return runCycleControl(ctx, gate, timeoutSec, reviewer, fixer, logger, span)
}

// runCycleControl implements the review-fix cycle logic.
// Used by the inline fallback path. Production path uses usecase.RunReviewGate.
func runCycleControl(ctx context.Context, gate domain.GateConfig, timeoutSec int,
	reviewer port.ReviewExecutor, fixer port.ReviewFixRunner, logger domain.Logger,
	span interface {
		SetAttributes(kv ...attribute.KeyValue)
	},
) (bool, error) {
	budget := gate.EffectiveReviewBudget()
	reviewTimeout := max(
		time.Duration(timeoutSec)*time.Second/time.Duration(budget),
		minReviewTimeout,
	)

	var lastComments string
	for cycle := 1; cycle <= budget; cycle++ {
		if ctx.Err() != nil {
			return false, fmt.Errorf("review gate canceled: %w", ctx.Err())
		}

		if span != nil {
			span.SetAttributes(attribute.Int("review.cycle", cycle))
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
