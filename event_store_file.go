package sightjack

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileEventStore is a JSONL-based append-only event store.
// Each event occupies one line (compact JSON + newline).
type FileEventStore struct {
	path string
	mu   sync.Mutex
}

// NewFileEventStore creates a FileEventStore at the given path.
func NewFileEventStore(path string) *FileEventStore {
	return &FileEventStore{path: path}
}

// Append writes one or more events to the JSONL file.
// Events are serialized as compact JSON, one per line.
// The parent directory is created if it does not exist.
func (s *FileEventStore) Append(events ...Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("event store mkdir: %w", err)
	}

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("event store open: %w", err)
	}
	defer f.Close()

	for _, e := range events {
		data, marshalErr := MarshalEvent(e)
		if marshalErr != nil {
			return fmt.Errorf("event store marshal: %w", marshalErr)
		}
		data = append(data, '\n')
		if _, writeErr := f.Write(data); writeErr != nil {
			return fmt.Errorf("event store write: %w", writeErr)
		}
	}

	return f.Sync()
}

// ReadAll reads all events from the JSONL file.
// Invalid lines are silently skipped.
// Returns an empty slice (not error) for a non-existent file.
func (s *FileEventStore) ReadAll() ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.readAllUnlocked()
}

// readAllUnlocked reads all events without locking (caller must hold mu).
func (s *FileEventStore) readAllUnlocked() ([]Event, error) {
	f, err := os.Open(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("event store open for read: %w", err)
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			// Skip corrupt lines
			continue
		}
		events = append(events, e)
	}
	if err := scanner.Err(); err != nil {
		return events, fmt.Errorf("event store scan: %w", err)
	}
	return events, nil
}

// ReadSince reads events with sequence number strictly greater than afterSeq.
func (s *FileEventStore) ReadSince(afterSeq int64) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.readAllUnlocked()
	if err != nil {
		return nil, err
	}

	var filtered []Event
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
