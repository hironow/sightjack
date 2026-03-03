package session

import (
	"os"
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

// Status collects current operational status from the event store and filesystem.
// baseDir is the repository root (e.g. the directory containing .siren/).
func Status(baseDir string) domain.StatusReport {
	var report domain.StatusReport

	// Count inbox files
	report.InboxCount = countDirFiles(domain.MailDir(baseDir, domain.InboxDir))

	// Count archive files
	report.ArchiveCount = countDirFiles(domain.MailDir(baseDir, domain.ArchiveDir))

	// Load all events across sessions for wave stats
	allEvents, err := LoadAllEvents(baseDir)
	if err != nil || len(allEvents) == 0 {
		return report
	}

	// Count wave events (applied + rejected)
	var success, total int
	for _, ev := range allEvents {
		switch ev.Type {
		case domain.EventWaveApplied:
			success++
			total++
		case domain.EventWaveRejected:
			total++
		}
	}
	report.WavesTotal = total
	report.SuccessRate = domain.SuccessRate(allEvents)

	// Find the most recent scan timestamp
	report.LastScanned = latestScanTime(allEvents)

	return report
}

// latestScanTime finds the most recent LastScanned from ScanCompletedPayload events.
func latestScanTime(events []domain.Event) time.Time {
	var latest time.Time
	for _, ev := range events {
		if ev.Type != domain.EventScanCompleted {
			continue
		}
		var payload domain.ScanCompletedPayload
		if err := domain.UnmarshalEventPayload(ev, &payload); err != nil {
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

