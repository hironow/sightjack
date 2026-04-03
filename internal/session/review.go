package session

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness"
)

// RunReview executes the review command and parses the output.
func RunReview(ctx context.Context, reviewCmd string, dir string) (*domain.ReviewResult, error) {
	if strings.TrimSpace(reviewCmd) == "" {
		return &domain.ReviewResult{Passed: true}, nil
	}

	cmd := exec.CommandContext(ctx, shellName(), shellFlag(), reviewCmd)
	cmd.Dir = dir
	cmd.WaitDelay = 1 * time.Second

	out, err := cmd.CombinedOutput()
	output := string(out)

	if ctx.Err() != nil {
		return nil, fmt.Errorf("review command canceled: %w", ctx.Err())
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if harness.IsRateLimited(output) {
				return nil, fmt.Errorf("review service rate/quota limited")
			}
			return &domain.ReviewResult{
				Passed:   false,
				Output:   output,
				Comments: output,
			}, nil
		}
		return nil, fmt.Errorf("review command failed: %w\noutput: %s", err, harness.SummarizeReview(output))
	}

	return &domain.ReviewResult{
		Passed: true,
		Output: output,
	}, nil
}

// BuildReviewFixPrompt creates a focused prompt for fixing review comments.
func BuildReviewFixPrompt(branch string, comments string) string {
	return fmt.Sprintf(`You are on branch %s. A code review found the following issues:

%s

Fix all review comments above. Commit and push your changes.
Keep fixes focused — only address the review comments, do not refactor unrelated code.`, branch, comments)
}
