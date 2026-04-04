package policy

import (
	"strings"

	"github.com/hironow/sightjack/internal/domain"
)

// DetectFailedClusterNames compares input cluster counts to success counts
// and returns names where at least one instance failed wave generation.
// With duplicate cluster names, a name is marked failed if fewer instances
// succeeded than existed in the input.
func DetectFailedClusterNames(clusters []domain.ClusterScanResult, successes []domain.WaveGenerateResult) map[string]bool {
	inputCount := make(map[string]int, len(clusters))
	for _, c := range clusters {
		inputCount[c.Name]++
	}
	successCount := make(map[string]int, len(successes))
	for _, r := range successes {
		successCount[r.ClusterName]++
	}
	failed := make(map[string]bool)
	for name, total := range inputCount {
		if successCount[name] < total {
			failed[name] = true
		}
	}
	return failed
}

// FilterEmptyClassifications removes clusters with zero issue IDs from the classification result.
// Returns the filtered list and the count of removed clusters.
func FilterEmptyClassifications(clusters []domain.ClusterClassification) ([]domain.ClusterClassification, int) {
	var filtered []domain.ClusterClassification
	var removed int
	for _, c := range clusters {
		if len(c.IssueIDs) > 0 {
			filtered = append(filtered, c)
		} else {
			removed++
		}
	}
	return filtered, removed
}

// ClampCompleteness bounds a completeness value to [0, 1].
func ClampCompleteness(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// MergeClusterChunks combines multiple chunk results from the same cluster
// into a single ClusterScanResult, recalculating completeness from individual issues.
// Individual issue completeness values are clamped to [0, 1] before averaging.
func MergeClusterChunks(name string, chunks []domain.ClusterScanResult) domain.ClusterScanResult {
	merged := domain.ClusterScanResult{Name: name}
	for _, c := range chunks {
		merged.Issues = append(merged.Issues, c.Issues...)
		merged.Observations = append(merged.Observations, c.Observations...)
	}
	if len(merged.Issues) > 0 {
		total := 0.0
		for i, issue := range merged.Issues {
			clamped := ClampCompleteness(issue.Completeness)
			merged.Issues[i].Completeness = clamped
			total += clamped
		}
		merged.Completeness = total / float64(len(merged.Issues))
	} else {
		// Preserve max completeness from source chunks when no issues remain.
		for _, c := range chunks {
			if c.Completeness > merged.Completeness {
				merged.Completeness = ClampCompleteness(c.Completeness)
			}
		}
	}
	return merged
}

// BuildScanRecoveryReport constructs a ScanRecoveryReport by comparing the
// full cluster list from the scan against the wave generation successes.
// It delegates failure detection to DetectFailedClusterNames so duplicate
// cluster names with partial failures are handled correctly.
func BuildScanRecoveryReport(clusters []domain.ClusterScanResult, successes []domain.WaveGenerateResult) domain.ScanRecoveryReport {
	failed := DetectFailedClusterNames(clusters, successes)

	// Count how many of the input clusters actually succeeded.
	// For duplicate names we track how many successes remain to allocate.
	successCount := make(map[string]int, len(successes))
	for _, r := range successes {
		successCount[r.ClusterName]++
	}

	outcomes := make([]domain.ClusterScanOutcome, 0, len(clusters))
	allocated := make(map[string]int, len(clusters))
	succeededTotal := 0
	failedTotal := 0

	for _, c := range clusters {
		ok := false
		if failed[c.Name] {
			// partial failure case: allocate successes first
			if allocated[c.Name] < successCount[c.Name] {
				ok = true
			}
		} else {
			ok = true
		}
		allocated[c.Name]++
		outcomes = append(outcomes, domain.ClusterScanOutcome{
			ClusterName: c.Name,
			Succeeded:   ok,
		})
		if ok {
			succeededTotal++
		} else {
			failedTotal++
		}
	}

	return domain.ScanRecoveryReport{
		Outcomes:       outcomes,
		SucceededCount: succeededTotal,
		FailedCount:    failedTotal,
	}
}

// --- Review helpers (merged from review_policy.go) ---

// SummarizeReview normalizes whitespace and truncates a review output string.
func SummarizeReview(comments string) string {
	normalized := strings.Join(strings.Fields(comments), " ")
	const maxLen = 500
	runes := []rune(normalized)
	if len(runes) <= maxLen {
		return normalized
	}
	return string(runes[:maxLen]) + "...(truncated)"
}
