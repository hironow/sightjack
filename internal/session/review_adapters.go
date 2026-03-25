package session

import (
	"context"
	"io"
	"os/exec"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// reviewExecutorAdapter implements port.ReviewExecutor using exec.Command.
type reviewExecutorAdapter struct {
	dir string
}

// NewReviewExecutor creates a ReviewExecutor for the given working directory.
func NewReviewExecutor(dir string) port.ReviewExecutor {
	return &reviewExecutorAdapter{dir: dir}
}

func (a *reviewExecutorAdapter) RunReview(ctx context.Context, reviewCmd string, _ string) (*domain.ReviewResult, error) {
	return RunReview(ctx, reviewCmd, a.dir)
}

// branchResolverAdapter implements port.BranchResolver using exec.Command.
type branchResolverAdapter struct{}

// NewBranchResolver creates a BranchResolver.
func NewBranchResolver() port.BranchResolver {
	return &branchResolverAdapter{}
}

func (a *branchResolverAdapter) CurrentBranch(ctx context.Context, dir string) (string, error) {
	return currentBranch(ctx, dir)
}

// reviewFixRunnerAdapter implements port.ReviewFixRunner using ClaudeRunner.
type reviewFixRunnerAdapter struct {
	runner port.ClaudeRunner
	branch port.BranchResolver
	dir    string
	logger domain.Logger
}

// NewReviewFixRunner creates a ReviewFixRunner.
func NewReviewFixRunner(runner port.ClaudeRunner, branch port.BranchResolver, dir string, logger domain.Logger) port.ReviewFixRunner {
	return &reviewFixRunnerAdapter{
		runner: runner,
		branch: branch,
		dir:    dir,
		logger: logger,
	}
}

func (a *reviewFixRunnerAdapter) RunReviewFix(ctx context.Context, _, _, comments string) error {
	branch, err := a.branch.CurrentBranch(ctx, a.dir)
	if err != nil {
		return err
	}
	prompt := BuildReviewFixPrompt(branch, comments)
	a.logger.Info("Review fix: running claude --continue")
	out, err := a.runner.Run(ctx, prompt, io.Discard, port.WithContinue(), port.WithWorkDir(a.dir))
	if err != nil {
		return err
	}
	_ = out
	return nil
}

// currentBranch returns the current git branch name (unexported, used by adapters).
func currentBranch(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
