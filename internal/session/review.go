package session

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

// ReviewResult holds the outcome of a code review execution.
type ReviewResult struct {
	Passed   bool   // true if no actionable comments were found
	Output   string // raw review output
	Comments string // extracted review comments (empty if passed)
}

// RunReview executes the review command and parses the output.
func RunReview(ctx context.Context, reviewCmd string, dir string) (*ReviewResult, error) {
	if strings.TrimSpace(reviewCmd) == "" {
		return &ReviewResult{Passed: true}, nil
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
			if domain.IsRateLimited(output) {
				return nil, fmt.Errorf("review service rate/quota limited")
			}
			return &ReviewResult{
				Passed:   false,
				Output:   output,
				Comments: output,
			}, nil
		}
		return nil, fmt.Errorf("review command failed: %w\noutput: %s", err, domain.SummarizeReview(output))
	}

	return &ReviewResult{
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
