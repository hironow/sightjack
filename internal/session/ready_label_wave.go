package session

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/hironow/sightjack/internal/domain"
)

// applyReadyLabelsWaveMode applies the ready label to GitHub issues using
// `gh issue edit --add-label` directly, bypassing Claude + Linear MCP.
// Used in wave mode where Linear MCP is unavailable.
func applyReadyLabelsWaveMode(ctx context.Context, cfg *domain.Config, issueIDs []string, logger domain.Logger) error {
	label := cfg.Labels.ReadyLabel
	for _, id := range issueIDs {
		logger.Info("Applying ready label %q to issue %s (wave mode)", label, id)
		cmd := exec.CommandContext(ctx, "gh", "issue", "edit", id, "--add-label", label)
		if out, err := cmd.CombinedOutput(); err != nil {
			logger.Warn("gh issue edit %s --add-label %s: %s", id, label, string(out))
			return fmt.Errorf("apply ready label to %s: %w", id, err)
		}
	}
	return nil
}
