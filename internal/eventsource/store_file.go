package eventsource

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	sightjack "github.com/hironow/sightjack"
)

// FileEventStore is a JSONL-based append-only event store.
// Each event occupies one line (compact JSON + newline).
type FileEventStore struct {
	path           string
	mu             sync.Mutex
	lastWrittenSeq int64
	seqInitialized bool
}

// NewFileEventStore creates a FileEventStore at the given path.
func NewFileEventStore(path string) *FileEventStore {
	return &FileEventStore{path: path}
}

// Append writes one or more events to the JSONL file.
// Events are serialized as compact JSON, one per line.
// The parent directory is created if it does not exist.
func (s *FileEventStore) Append(events ...sightjack.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Lazy init: read last sequence from file on first append
	if !s.seqInitialized {
		all, err := s.readAllUnlocked()
		if err != nil {
			return err
		}
		for _, e := range all {
			if e.Sequence > s.lastWrittenSeq {
				s.lastWrittenSeq = e.Sequence
			}
		}
		s.seqInitialized = true
	}

	// Validate all events before any I/O
	for _, e := range events {
		if err := sightjack.ValidateEvent(e); err != nil {
			return fmt.Errorf("event store: %w", err)
		}
	}

	// Sequence monotonicity check
	expectedSeq := s.lastWrittenSeq
	for _, e := range events {
		expectedSeq++
		if e.Sequence != expectedSeq {
			return fmt.Errorf("event store: sequence gap: expected %d, got %d", expectedSeq, e.Sequence)
		}
	}

	if err := s.appendUnlocked(events); err != nil {
		return err
	}

	// Update tracked sequence after successful write
	if len(events) > 0 {
		s.lastWrittenSeq = events[len(events)-1].Sequence
	}

	return nil
}

// appendUnlocked writes events to the JSONL file (caller must hold mu).
func (s *FileEventStore) appendUnlocked(events []sightjack.Event) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("event store mkdir: %w", err)
	}

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("event store open: %w", err)
	}
	defer f.Close()

	for _, e := range events {
		data, marshalErr := sightjack.MarshalEvent(e)
		if marshalErr != nil {
			return fmt.Errorf("event store marshal: %w", marshalErr)
		}
		data = append(data, '\n')
		if _, writeErr := f.Write(data); writeErr != nil {
			return fmt.Errorf("event store write: %w", writeErr)
		}
		if syncErr := f.Sync(); syncErr != nil {
			return fmt.Errorf("event store sync: %w", syncErr)
		}
	}
	return nil
}

// ReadAll reads all events from the JSONL file.
// Invalid lines are silently skipped.
// Returns an empty slice (not error) for a non-existent file.
func (s *FileEventStore) ReadAll() ([]sightjack.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.readAllUnlocked()
}

// readAllUnlocked reads all events without locking (caller must hold mu).
func (s *FileEventStore) readAllUnlocked() ([]sightjack.Event, error) {
	f, err := os.Open(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("event store open for read: %w", err)
	}
	defer f.Close()

	var events []sightjack.Event
	reader := bufio.NewReader(f)
	for {
		line, readErr := reader.ReadBytes('\n')
		// Trim trailing newline (and handle last line without newline)
		line = bytes.TrimRight(line, "\n")
		if len(line) > 0 {
			var e sightjack.Event
			if jsonErr := json.Unmarshal(line, &e); jsonErr == nil {
				events = append(events, e)
			}
			// Skip corrupt lines silently
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return events, fmt.Errorf("event store read: %w", readErr)
		}
	}
	return events, nil
}

// ReadSince reads events with sequence number strictly greater than afterSeq.
func (s *FileEventStore) ReadSince(afterSeq int64) ([]sightjack.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.readAllUnlocked()
	if err != nil {
		return nil, err
	}

	var filtered []sightjack.Event
	for _, e := range all {
		if e.Sequence > afterSeq {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// LastSequence returns the highest sequence number in the store.
// Returns 0 for an empty or non-existent store.
func (s *FileEventStore) LastSequence() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.readAllUnlocked()
	if err != nil {
		return 0, err
	}

	var max int64
	for _, e := range all {
		if e.Sequence > max {
			max = e.Sequence
		}
	}
	return max, nil
}
