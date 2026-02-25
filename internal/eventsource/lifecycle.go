package eventsource

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ListExpiredEventFiles returns .jsonl filenames in .siren/events/ whose
// mtime exceeds the given number of days.
// Returns an empty slice (not error) when the events directory does not exist.
func ListExpiredEventFiles(baseDir string, days int) ([]string, error) {
	if days < 0 {
		return nil, fmt.Errorf("days must be non-negative, got %d", days)
	}
	dir := EventsDir(baseDir)
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
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
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

// PruneEventFiles deletes the named .jsonl files from .siren/events/.
// Files that no longer exist are silently skipped.
// Returns the list of filenames that were processed.
func PruneEventFiles(baseDir string, files []string) ([]string, error) {
	dir := EventsDir(baseDir)
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
