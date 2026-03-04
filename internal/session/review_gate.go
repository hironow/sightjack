package session

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

const (
	maxReviewGateCycles = 3
	minReviewTimeout    = 30 * time.Second
)

// RunReviewGate runs the review-fix cycle before ComposeReport.
// Returns (true, nil) if review passes or is skipped (no ReviewCmd).
// Returns (false, nil) if review fails after all cycles.
// Returns (false, err) on infrastructure errors.
func RunReviewGate(ctx context.Context, cfg *domain.Config, dir string, logger domain.Logger) (bool, error) {
	if strings.TrimSpace(cfg.Gate.ReviewCmd) == "" {
		return true, nil
	}

	if logger == nil {
		logger = &domain.NopLogger{}
	}

	budget := cfg.Gate.ReviewBudget
	if budget <= 0 {
		budget = maxReviewGateCycles
	}

	timeoutSec := cfg.Claude.TimeoutSec
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
		result, err := RunReview(reviewCtx, cfg.Gate.ReviewCmd, dir)
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

		// Last cycle — no point running fix
		if cycle == budget {
			break
		}

		// Run Claude --continue to fix review comments
		if err := runReviewFix(ctx, cfg, dir, lastComments, logger); err != nil {
			logger.Warn("Review fix failed: %v", err)
			return false, nil
		}
	}

	logger.Warn("Review gate: exhausted %d cycles, review not resolved", budget)
	return false, nil
}

// runReviewFix runs Claude --continue to fix review comments.
func runReviewFix(ctx context.Context, cfg *domain.Config, dir, comments string, logger domain.Logger) error {
	branch, err := currentBranch(ctx, dir)
	if err != nil {
		return fmt.Errorf("detect branch: %w", err)
	}

	claudeCmd := cfg.Claude.Command
	if claudeCmd == "" {
		claudeCmd = "claude"
	}
	model := cfg.Claude.Model
	if model == "" {
		model = "opus"
	}

	prompt := BuildReviewFixPrompt(branch, comments)

	fixTimeout := time.Duration(cfg.Claude.TimeoutSec) * time.Second
	if fixTimeout <= 0 {
		fixTimeout = 300 * time.Second
	}
	fixCtx, fixCancel := context.WithTimeout(ctx, fixTimeout)
	defer fixCancel()

	cmd := exec.CommandContext(fixCtx, claudeCmd,
		"--model", model,
		"--continue",
		"--dangerously-skip-permissions",
		"--print",
		"-p", prompt,
	)
	cmd.Dir = dir
	cmd.WaitDelay = 3 * time.Second

	logger.Info("Review fix: running %s --model %s --continue", claudeCmd, model)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("claude fix: %w\noutput: %s", err, domain.SummarizeReview(string(out)))
	}
	return nil
}

// currentBranch returns the current git branch name.
func currentBranch(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
