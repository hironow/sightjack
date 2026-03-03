package domain

import (
	"strings"
)

// SanitizeName converts a cluster name to a safe filename component.
// Only ASCII alphanumeric, hyphen, and underscore are kept; everything else becomes underscore.
func SanitizeName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// DetectFailedClusterNames compares input cluster counts to success counts
// and returns names where at least one instance failed wave generation.
// With duplicate cluster names, a name is marked failed if fewer instances
// succeeded than existed in the input.
func DetectFailedClusterNames(clusters []ClusterScanResult, successes []WaveGenerateResult) map[string]bool {
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

// ChunkSlice splits items into sub-slices of at most size elements.
func ChunkSlice(items []string, size int) [][]string {
	if len(items) == 0 {
		return nil
	}
	if size <= 0 {
		return [][]string{items}
	}
	var chunks [][]string
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks
}

// MergeClusterChunks combines multiple chunk results from the same cluster
// into a single ClusterScanResult, recalculating completeness from individual issues.
func MergeClusterChunks(name string, chunks []ClusterScanResult) ClusterScanResult {
	merged := ClusterScanResult{Name: name}
	for _, c := range chunks {
		merged.Issues = append(merged.Issues, c.Issues...)
		merged.Observations = append(merged.Observations, c.Observations...)
	}
	if len(merged.Issues) > 0 {
		total := 0.0
		for _, issue := range merged.Issues {
			total += issue.Completeness
		}
		merged.Completeness = total / float64(len(merged.Issues))
	}
	return merged
}
