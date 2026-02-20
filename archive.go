package sightjack

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ListExpiredArchive returns filenames in .siren/archive/ whose mtime exceeds
// the given number of days. Only .md files are considered.
// Returns an empty slice (not error) when the archive directory does not exist.
func ListExpiredArchive(baseDir string, days int) ([]string, error) {
	dir := MailDir(baseDir, archiveDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	var expired []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			expired = append(expired, e.Name())
		}
	}
	return expired, nil
}

// PruneArchive deletes expired .md files from .siren/archive/ and returns the
// list of deleted filenames. Uses the same criteria as ListExpiredArchive.
// Returns an empty slice (not error) when the archive directory does not exist.
func PruneArchive(baseDir string, days int) ([]string, error) {
	files, err := ListExpiredArchive(baseDir, days)
	if err != nil {
		return nil, err
	}

	dir := MailDir(baseDir, archiveDir)
	var deleted []string
	for _, name := range files {
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return deleted, err
		}
		deleted = append(deleted, name)
	}
	return deleted, nil
}
