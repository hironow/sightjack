package session

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// Status collects current operational status from the event store and filesystem.
// baseDir is the repository root (e.g. the directory containing .siren/).
func Status(ctx context.Context, baseDir string) domain.StatusReport {
	var report domain.StatusReport
	applyLatestProviderMetadata(ctx, baseDir, &report)

	// Count inbox files
	report.InboxCount = countDirFiles(domain.MailDir(baseDir, domain.InboxDir))

	// Count archive files
	report.ArchiveCount = countDirFiles(domain.MailDir(baseDir, domain.ArchiveDir))

	// Load all events across sessions for wave stats
	allEvents, err := LoadAllEvents(ctx, baseDir)
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

func applyLatestProviderMetadata(ctx context.Context, baseDir string, report *domain.StatusReport) {
	dbPath := filepath.Join(baseDir, domain.StateDir, ".run", "sessions.db")
	store, err := NewSQLiteCodingSessionStore(dbPath)
	if err != nil {
		return
	}
	defer store.Close()
	records, err := store.List(ctx, port.ListSessionOpts{Limit: 1})
	if err != nil || len(records) == 0 {
		return
	}
	meta := records[0].Metadata
	report.ProviderState = meta[domain.MetadataProviderState]
	report.ProviderReason = meta[domain.MetadataProviderReason]
	if budget := meta[domain.MetadataProviderRetryBudget]; budget != "" {
		if n, err := strconv.Atoi(budget); err == nil {
			report.ProviderRetryBudget = n
		}
	}
	if resumeAt := meta[domain.MetadataProviderResumeAt]; resumeAt != "" {
		if ts, err := time.Parse(time.RFC3339, resumeAt); err == nil {
			report.ProviderResumeAt = ts
		}
	}
	report.ProviderResumeWhen = meta[domain.MetadataProviderResumeWhen]
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
