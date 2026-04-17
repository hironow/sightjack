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

// ClusterScanOutcome records whether wave generation succeeded for a single cluster.
type ClusterScanOutcome struct { // nosemgrep: domain-primitives.public-string-field-go -- internal scan result DTO; no validation needed [permanent]
	ClusterName string
	Succeeded   bool
}

// ScanRecoveryReport summarises which clusters succeeded and which failed
// during wave generation so that callers can present partial results and
// decide on recovery actions.
type ScanRecoveryReport struct {
	// Outcomes contains one entry per cluster in the original scan order.
	Outcomes       []ClusterScanOutcome
	SucceededCount int
	FailedCount    int
}
