package session

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	sightjack "github.com/hironow/sightjack"
)

// StatusReport holds operational status information for the sightjack tool.
type StatusReport struct {
	LastScanned  time.Time `json:"last_scanned"`
	WavesTotal   int       `json:"waves_total"`
	InboxCount   int       `json:"inbox_count"`
	ArchiveCount int       `json:"archive_count"`
	SuccessRate  float64   `json:"success_rate"`
}

// Status collects current operational status from the event store and filesystem.
// baseDir is the repository root (e.g. the directory containing .siren/).
func Status(baseDir string) StatusReport {
	var report StatusReport

	// Count inbox files
	report.InboxCount = countDirFiles(sightjack.MailDir(baseDir, sightjack.InboxDir))

	// Count archive files
	report.ArchiveCount = countDirFiles(sightjack.MailDir(baseDir, sightjack.ArchiveDir))

	// Load all events across sessions for wave stats
	allEvents, err := LoadAllEvents(baseDir)
	if err != nil || len(allEvents) == 0 {
		return report
	}

	// Count wave events (applied + rejected)
	var success, total int
	for _, ev := range allEvents {
		switch ev.Type {
		case sightjack.EventWaveApplied:
			success++
			total++
		case sightjack.EventWaveRejected:
			total++
		}
	}
	report.WavesTotal = total
	report.SuccessRate = sightjack.SuccessRate(allEvents)

	// Find the most recent scan timestamp
	report.LastScanned = latestScanTime(allEvents)

	return report
}

// latestScanTime finds the most recent LastScanned from ScanCompletedPayload events.
func latestScanTime(events []sightjack.Event) time.Time {
	var latest time.Time
	for _, ev := range events {
		if ev.Type != sightjack.EventScanCompleted {
			continue
		}
		var payload sightjack.ScanCompletedPayload
		if err := sightjack.UnmarshalEventPayload(ev, &payload); err != nil {
			continue
		}
		if payload.LastScanned.After(latest) {
			latest = payload.LastScanned
		}
	}
	return latest
}

// countDirFiles returns the number of non-directory entries in the given directory.
// Returns 0 if the directory does not exist or cannot be read.
func countDirFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	return count
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
