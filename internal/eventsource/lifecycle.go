package eventsource

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ListExpiredEventFiles returns session names in events/ whose
// mtime exceeds the given number of days.
// stateDir is the tool's state directory (e.g. ".siren/"), not the repo root.
// Supports both directories (new format) and .jsonl files (legacy).
// Returns an empty slice (not error) when the events directory does not exist.
func ListExpiredEventFiles(stateDir string, days int) ([]string, error) {
	if days < 0 {
		return nil, fmt.Errorf("days must be non-negative, got %d", days)
	}
	dir := EventsDir(stateDir)
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
		info, infoErr := e.Info()
		if infoErr != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			expired = append(expired, e.Name())
		}
	}
	return expired, nil
}

// PruneEventFiles deletes the named entries from events/.
// stateDir is the tool's state directory (e.g. ".siren/"), not the repo root.
// Supports both directories and files.
// Entries that no longer exist are silently skipped.
// Returns the list of names that were processed.
func PruneEventFiles(stateDir string, files []string) ([]string, error) {
	dir := EventsDir(stateDir)
	var deleted []string
	for _, name := range files {
		path := filepath.Join(dir, name)
		if err := os.RemoveAll(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return deleted, err
		}
		deleted = append(deleted, name)
	}
	return deleted, nil
}
