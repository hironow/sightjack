package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// StatusReport holds operational status information for the sightjack tool.
type StatusReport struct {
	LastScanned  time.Time `json:"last_scanned"`
	WavesTotal   int       `json:"waves_total"`
	InboxCount   int       `json:"inbox_count"`
	ArchiveCount int       `json:"archive_count"`
	SuccessRate  float64   `json:"success_rate"`
}

// FormatText returns a human-readable status report string suitable for stderr.
func (r StatusReport) FormatText() string {
	var b strings.Builder
	b.WriteString("sightjack status:\n")

	// Last scan
	if r.LastScanned.IsZero() {
		b.WriteString("  Last scan:     no scans yet\n")
	} else {
		b.WriteString(fmt.Sprintf("  Last scan:     %s\n", r.LastScanned.Format(time.RFC3339)))
	}

	// Waves
	b.WriteString(fmt.Sprintf("  Waves:         %d total\n", r.WavesTotal))

	// Success rate
	if r.WavesTotal == 0 {
		b.WriteString("  Success rate:  no events\n")
	} else {
		b.WriteString(fmt.Sprintf("  Success rate:  %.1f%%\n", r.SuccessRate*100))
	}

	// Inbox
	b.WriteString(fmt.Sprintf("  Inbox:         %d pending\n", r.InboxCount))

	// Archive
	b.WriteString(fmt.Sprintf("  Archive:       %d processed\n", r.ArchiveCount))

	return b.String()
}

// FormatJSON returns the status report as a compact JSON string.
func (r StatusReport) FormatJSON() string {
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(data)
}
