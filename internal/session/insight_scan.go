package session

import (
	"fmt"

	"github.com/hironow/sightjack/internal/domain"
)

// WriteShibitoInsights generates insight entries from ShibitoWarnings and
// appends them to the shibito.md insight file. Skips if there are no warnings.
// Errors are logged but do not propagate — insight writing is best-effort.
func WriteShibitoInsights(w *InsightWriter, warnings []domain.ShibitoWarning, sessionID string, logger domain.Logger) {
	if len(warnings) == 0 {
		return
	}

	for _, warn := range warnings {
		entry := domain.InsightEntry{
			Title:       fmt.Sprintf("shibito-%s-%s", warn.ClosedIssueID, warn.CurrentIssueID),
			What:        warn.Description,
			Why:         fmt.Sprintf("Issue %s was closed but pattern re-emerged in %s", warn.ClosedIssueID, warn.CurrentIssueID),
			How:         "Review the original fix. Consider structural prevention",
			When:        "During scan, when closed issue patterns match current open issues",
			Who:         fmt.Sprintf("sightjack scan (%s)", sessionID),
			Constraints: fmt.Sprintf("Risk level: %s", warn.RiskLevel),
			Extra: map[string]string{
				"closed-issue-id":  warn.ClosedIssueID,
				"current-issue-id": warn.CurrentIssueID,
			},
		}

		if err := w.Append("shibito.md", "shibito", "sightjack", entry); err != nil {
			if logger != nil {
				logger.Warn("write shibito insight: %v", err)
			}
		}
	}
}

// WriteStrictnessInsights generates insight entries from cluster scan results
// that have EstimatedStrictness set. Appends to strictness.md insight file.
// Clusters without an estimate are skipped. Errors are best-effort.
func WriteStrictnessInsights(w *InsightWriter, clusters []domain.ClusterScanResult, sessionID string, logger domain.Logger) {
	for _, cluster := range clusters {
		if cluster.EstimatedStrictness == "" {
			continue
		}

		entry := domain.InsightEntry{
			Title:       fmt.Sprintf("strictness-%s", cluster.Name),
			What:        fmt.Sprintf("Cluster %s estimated strictness: %s", cluster.Name, cluster.EstimatedStrictness),
			Why:         cluster.StrictnessReasoning,
			How:         "Manual override available via config",
			When:        "During scan pass 2, per-cluster deep analysis",
			Who:         fmt.Sprintf("sightjack scan (%s)", sessionID),
			Constraints: "Estimated — may differ from manual override",
			Extra: map[string]string{
				"cluster":            cluster.Name,
				"completeness-delta": fmt.Sprintf("%.1f%%", cluster.Completeness*100),
			},
		}

		if err := w.Append("strictness.md", "strictness", "sightjack", entry); err != nil {
			if logger != nil {
				logger.Warn("write strictness insight: %v", err)
			}
		}
	}
}
