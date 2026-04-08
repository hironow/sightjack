package eventsource

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
)

// StoreHealth summarises the validation result of the event store.
type StoreHealth struct {
	Sessions     int      // number of session directories (or legacy flat files) containing events
	Events       int      // total number of valid event lines
	CorruptLines int      // number of lines skipped due to parse errors
	LoadErrors   []string // per-file or per-session load errors (permissions, etc.)
	NotFound     bool     // true when the events directory does not exist
	Err          error    // summary error (nil = healthy)
	ErrHint      string   // human-readable remediation hint
}

// ValidateStore walks the event store directory tree rooted at stateDir
// and validates every event file using json.Unmarshal into domain.Event —
// the same judgment used by the real event store during replay.
//
// Both legacy flat .jsonl files at the events root and per-session
// subdirectories are scanned. Load errors are accumulated (not returned
// immediately) so that one unreadable file doesn't prevent checking the rest.
//
// If the events directory does not exist, StoreHealth is returned with
// NotFound=true (not an error — the store simply has no data).
func ValidateStore(stateDir string) StoreHealth {
	eventsDir := EventsDir(stateDir)
	sessionEntries, err := os.ReadDir(eventsDir)
	if err != nil {
		return StoreHealth{NotFound: true} // directory absent — no data yet
	}

	var sessions, totalEvents, corruptLines int
	var loadErrors []string

	// Process legacy flat .jsonl files at the events root
	for _, sessionEntry := range sessionEntries {
		if sessionEntry.IsDir() || !isEventFile(sessionEntry.Name()) {
			continue
		}
		data, fErr := os.ReadFile(filepath.Join(eventsDir, sessionEntry.Name()))
		if fErr != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", sessionEntry.Name(), fErr))
			continue
		}
		hasEvent := false
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
			totalEvents++
			hasEvent = true
		}
		if hasEvent {
			sessions++
		}
	}

	// Process session directories
	for _, sessionEntry := range sessionEntries {
		if !sessionEntry.IsDir() {
			continue
		}
		sessionPath := filepath.Join(eventsDir, sessionEntry.Name())
		files, readErr := os.ReadDir(sessionPath)
		if readErr != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("session %s: %v", sessionEntry.Name(), readErr))
			continue
		}
		hasEventFile := false
		for _, f := range files {
			if f.IsDir() || !isEventFile(f.Name()) {
				continue
			}
			data, fErr := os.ReadFile(filepath.Join(sessionPath, f.Name()))
			if fErr != nil {
				loadErrors = append(loadErrors, fmt.Sprintf("session %s/%s: %v", sessionEntry.Name(), f.Name(), fErr))
				continue
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
				totalEvents++
			}
			hasEventFile = true
		}
		if hasEventFile {
			sessions++
		}
	}

	health := StoreHealth{
		Sessions:     sessions,
		Events:       totalEvents,
		CorruptLines: corruptLines,
		LoadErrors:   loadErrors,
	}
	if len(loadErrors) > 0 {
		health.Err = fmt.Errorf("%d load error(s) in event store", len(loadErrors))
		health.ErrHint = "check file permissions on the events/ directory: " + strings.Join(loadErrors, "; ")
	} else if corruptLines > 0 {
		health.Err = fmt.Errorf("%d corrupt line(s) found across event store", corruptLines)
		health.ErrHint = "corrupt lines are skipped during replay — review JSONL files in " + eventsDir
	}
	return health
}

// isEventFile reports whether a filename has the event-file suffix.
func isEventFile(name string) bool {
	return strings.HasSuffix(name, eventFileSuffix)
}

// eventFileSuffix is the canonical extension for event store files.
const eventFileSuffix = ".jsonl"
