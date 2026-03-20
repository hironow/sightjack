package eventsource

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StoreHealth summarises the validation result of the event store.
type StoreHealth struct {
	Sessions int    // number of session directories containing event files
	Events   int    // total number of valid JSON lines
	NotFound bool   // true when the events directory does not exist
	Err      error  // first error encountered (nil = healthy)
	ErrHint  string // human-readable remediation hint
}

// ValidateStore walks the event store directory tree rooted at stateDir
// and checks that every event file contains well-formed JSON lines.
// It returns a StoreHealth summarising the result.
//
// If the events directory does not exist, StoreHealth is returned with
// Sessions=0, Events=0, Err=nil (not an error — the store simply has no data).
func ValidateStore(stateDir string) StoreHealth {
	eventsDir := EventsDir(stateDir)
	sessionEntries, err := os.ReadDir(eventsDir)
	if err != nil {
		return StoreHealth{NotFound: true} // directory absent — no data yet
	}

	var sessions, totalEvents int

	// Process legacy flat .jsonl files at the events root
	for _, sessionEntry := range sessionEntries {
		if sessionEntry.IsDir() || !isEventFile(sessionEntry.Name()) {
			continue
		}
		data, fErr := os.ReadFile(filepath.Join(eventsDir, sessionEntry.Name()))
		if fErr != nil {
			return StoreHealth{
				Err:     fmt.Errorf("read error: %w", fErr),
				ErrHint: "check file permissions on legacy flat JSONL files in events/",
			}
		}
		hasEvent := false
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if !json.Valid([]byte(line)) {
				return StoreHealth{
					Err:     fmt.Errorf("corrupt JSON in legacy file %s", sessionEntry.Name()),
					ErrHint: "check legacy flat JSONL files for corruption in events/",
				}
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
			return StoreHealth{
				Err:     fmt.Errorf("read error: %w", readErr),
				ErrHint: "check file permissions on the events/ directory",
			}
		}
		hasEventFile := false
		for _, f := range files {
			if f.IsDir() || !isEventFile(f.Name()) {
				continue
			}
			data, fErr := os.ReadFile(filepath.Join(sessionPath, f.Name()))
			if fErr != nil {
				return StoreHealth{
					Err:     fmt.Errorf("read error: %w", fErr),
					ErrHint: "check file permissions on the events/ directory",
				}
			}
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if !json.Valid([]byte(line)) {
					return StoreHealth{
						Err:     fmt.Errorf("corrupt JSON in %s/%s", sessionEntry.Name(), f.Name()),
						ErrHint: "check event files for corruption in the events/ directory",
					}
				}
				totalEvents++
			}
			hasEventFile = true
		}
		if hasEventFile {
			sessions++
		}
	}

	return StoreHealth{Sessions: sessions, Events: totalEvents}
}

// isEventFile reports whether a filename has the event-file suffix.
func isEventFile(name string) bool {
	return strings.HasSuffix(name, eventFileSuffix)
}

// eventFileSuffix is the canonical extension for event store files.
const eventFileSuffix = ".jsonl"
