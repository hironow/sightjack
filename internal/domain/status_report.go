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

// FormatText returns a human-readable status report string suitable for stdout.
func (r StatusReport) FormatText() string {
	var b strings.Builder
	b.WriteString("sightjack status\n\n")

	// Last scan
	if r.LastScanned.IsZero() {
		fmt.Fprintf(&b, "  %-16s %s\n", "Last scan:", "no scans yet")
	} else {
		fmt.Fprintf(&b, "  %-16s %s\n", "Last scan:", r.LastScanned.Format(time.RFC3339))
	}

	fmt.Fprintf(&b, "  %-16s %d total\n", "Waves:", r.WavesTotal)

	// Success rate
	if r.WavesTotal == 0 {
		fmt.Fprintf(&b, "  %-16s %s\n", "Success rate:", "no events")
	} else {
		fmt.Fprintf(&b, "  %-16s %.1f%%\n", "Success rate:", r.SuccessRate*100)
	}

	fmt.Fprintf(&b, "  %-16s %d pending\n", "Inbox:", r.InboxCount)
	fmt.Fprintf(&b, "  %-16s %d processed\n", "Archive:", r.ArchiveCount)

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
