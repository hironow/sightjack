package usecase

import (
	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// ListExpiredArchive returns archived d-mail files older than the retention threshold.
func ListExpiredArchive(baseDir string, days int, logger *domain.Logger) ([]string, error) {
	return session.ListExpiredArchive(baseDir, days, logger)
}

// ListExpiredEventFiles returns event files older than the retention threshold.
func ListExpiredEventFiles(baseDir string, days int) ([]string, error) {
	return session.ListExpiredEventFiles(baseDir, days)
}

// DeleteArchiveFiles removes the specified archive files.
func DeleteArchiveFiles(baseDir string, files []string) ([]string, error) {
	return session.DeleteArchiveFiles(baseDir, files)
}

// PruneEventFiles removes the specified event files.
func PruneEventFiles(baseDir string, files []string) ([]string, error) {
	return session.PruneEventFiles(baseDir, files)
}

// PruneFlushedOutbox prunes flushed outbox rows and performs incremental vacuum.
func PruneFlushedOutbox(baseDir string) (int, error) {
	return session.PruneFlushedOutbox(baseDir)
}
