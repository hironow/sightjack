package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
)

// checkEventStoreIntegrity scans all event files (both legacy flat .jsonl in
// events/ root and per-session subdirectories) using the same json.Unmarshal
// judgment as the real event store replay. Load errors are surfaced, not silently
// swallowed.
func checkEventStoreIntegrity(baseDir string) domain.DoctorCheck {
	eventsRoot := filepath.Join(baseDir, domain.StateDir, "events")
	if _, err := os.Stat(eventsRoot); err != nil {
		return domain.DoctorCheck{
			Name:    "Event Store Integrity",
			Status:  domain.CheckSkip,
			Message: "no events directory",
		}
	}
	entries, err := os.ReadDir(eventsRoot)
	if err != nil {
		return domain.DoctorCheck{
			Name:    "Event Store Integrity",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("read events dir: %v", err),
		}
	}

	totalFiles := 0
	totalCorrupt := 0
	var loadErrors []string

	// 1. Scan legacy flat .jsonl files at events/ root
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		_, corrupt, fErr := countCorruptLinesInFile(filepath.Join(eventsRoot, e.Name()))
		if fErr != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", e.Name(), fErr))
			continue
		}
		totalFiles++
		totalCorrupt += corrupt
	}

	// 2. Scan per-session subdirectories (each has its own event store)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sessionDir := filepath.Join(eventsRoot, e.Name())
		store := NewEventStore(sessionDir, &domain.NopLogger{})
		_, result, loadErr := store.LoadAll()
		if loadErr != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("session %s: %v", e.Name(), loadErr))
			continue
		}
		totalFiles += result.FileCount
		totalCorrupt += result.CorruptLineCount
	}

	// Surface load errors as WARN (not silent continue)
	if len(loadErrors) > 0 {
		msg := fmt.Sprintf("%d load error(s)", len(loadErrors))
		if totalCorrupt > 0 {
			msg = fmt.Sprintf("%d corrupt line(s), %d load error(s)", totalCorrupt, len(loadErrors))
		}
		return domain.DoctorCheck{
			Name:    "Event Store Integrity",
			Status:  domain.CheckWarn,
			Message: msg,
			Hint:    "review event files in " + eventsRoot + ": " + strings.Join(loadErrors, "; "),
		}
	}

	if totalCorrupt > 0 {
		return domain.DoctorCheck{
			Name:    "Event Store Integrity",
			Status:  domain.CheckWarn,
			Message: fmt.Sprintf("%d corrupt line(s) across %d file(s)", totalCorrupt, totalFiles),
			Hint:    "corrupt lines are skipped during replay — review JSONL files in " + eventsRoot,
		}
	}
	return domain.DoctorCheck{
		Name:    "Event Store Integrity",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("event store integrity OK (%d file(s), 0 corrupt lines)", totalFiles),
	}
}

// countCorruptLinesInFile reads a .jsonl file and counts lines that fail
// json.Unmarshal into domain.Event — matching the real event store's judgment.
func countCorruptLinesInFile(path string) (validLines int, corruptLines int, err error) {
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return 0, 0, readErr
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev domain.Event
		if jsonErr := json.Unmarshal([]byte(line), &ev); jsonErr != nil {
			corruptLines++
			continue
		}
		validLines++
	}
	return validLines, corruptLines, nil
}
