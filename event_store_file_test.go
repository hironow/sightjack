package sightjack_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/hironow/sightjack"
)

func TestFileEventStore_AppendAndReadAll_RoundTrip(t *testing.T) {
	// given
	dir := t.TempDir()
	store := sightjack.NewFileEventStore(filepath.Join(dir, "test.jsonl"))

	e1, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, nil)
	e2, _ := sightjack.NewEvent(sightjack.EventScanCompleted, "s1", 2, nil)

	// when
	if err := store.Append(e1, e2); err != nil {
		t.Fatalf("Append: %v", err)
	}
	events, err := store.ReadAll()

	// then
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != sightjack.EventSessionStarted {
		t.Errorf("expected session_started, got %s", events[0].Type)
	}
	if events[1].Type != sightjack.EventScanCompleted {
		t.Errorf("expected scan_completed, got %s", events[1].Type)
	}
}

func TestFileEventStore_ReadSince_FiltersCorrectly(t *testing.T) {
	// given
	dir := t.TempDir()
	store := sightjack.NewFileEventStore(filepath.Join(dir, "test.jsonl"))

	for i := int64(1); i <= 5; i++ {
		e, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", i, nil)
		store.Append(e)
	}

	// when: read events after sequence 3
	events, err := store.ReadSince(3)

	// then
	if err != nil {
		t.Fatalf("ReadSince: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events (seq 4,5), got %d", len(events))
	}
	if events[0].Sequence != 4 {
		t.Errorf("expected seq 4, got %d", events[0].Sequence)
	}
	if events[1].Sequence != 5 {
		t.Errorf("expected seq 5, got %d", events[1].Sequence)
	}
}

func TestFileEventStore_ReadAll_EmptyFile(t *testing.T) {
	// given
	dir := t.TempDir()
	store := sightjack.NewFileEventStore(filepath.Join(dir, "empty.jsonl"))

	// when
	events, err := store.ReadAll()

	// then
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestFileEventStore_LastSequence(t *testing.T) {
	// given
	dir := t.TempDir()
	store := sightjack.NewFileEventStore(filepath.Join(dir, "test.jsonl"))

	e1, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, nil)
	e2, _ := sightjack.NewEvent(sightjack.EventScanCompleted, "s1", 2, nil)
	e3, _ := sightjack.NewEvent(sightjack.EventWavesGenerated, "s1", 3, nil)
	store.Append(e1, e2, e3)

	// when
	seq, err := store.LastSequence()

	// then
	if err != nil {
		t.Fatalf("LastSequence: %v", err)
	}
	if seq != 3 {
		t.Errorf("expected 3, got %d", seq)
	}
}

func TestFileEventStore_LastSequence_EmptyFile(t *testing.T) {
	// given
	dir := t.TempDir()
	store := sightjack.NewFileEventStore(filepath.Join(dir, "empty.jsonl"))

	// when
	seq, err := store.LastSequence()

	// then
	if err != nil {
		t.Fatalf("LastSequence: %v", err)
	}
	if seq != 0 {
		t.Errorf("expected 0 for empty store, got %d", seq)
	}
}

func TestFileEventStore_ConcurrentAppend(t *testing.T) {
	// given
	dir := t.TempDir()
	store := sightjack.NewFileEventStore(filepath.Join(dir, "concurrent.jsonl"))
	count := 50

	// when: append concurrently
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(seq int64) {
			defer wg.Done()
			e, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", seq, nil)
			if err := store.Append(e); err != nil {
				t.Errorf("concurrent append seq %d: %v", seq, err)
			}
		}(int64(i + 1))
	}
	wg.Wait()

	// then
	events, err := store.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(events) != count {
		t.Errorf("expected %d events, got %d", count, len(events))
	}
}

func TestFileEventStore_CorruptLineSkipped(t *testing.T) {
	// given: a JSONL file with one corrupt line
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.jsonl")

	e1, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, nil)
	data1, _ := sightjack.MarshalEvent(e1)
	e2, _ := sightjack.NewEvent(sightjack.EventScanCompleted, "s1", 2, nil)
	data2, _ := sightjack.MarshalEvent(e2)

	content := string(data1) + "\n" + "THIS IS NOT JSON\n" + string(data2) + "\n"
	os.WriteFile(path, []byte(content), 0644)

	store := sightjack.NewFileEventStore(path)

	// when
	events, err := store.ReadAll()

	// then: corrupt line is skipped, valid events remain
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events (corrupt skipped), got %d", len(events))
	}
}

func TestFileEventStore_AutoCreateDirectory(t *testing.T) {
	// given: path with non-existent parent directory
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dir", "events.jsonl")
	store := sightjack.NewFileEventStore(path)

	e, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, nil)

	// when
	err := store.Append(e)

	// then
	if err != nil {
		t.Fatalf("Append should auto-create dirs: %v", err)
	}
	events, _ := store.ReadAll()
	if len(events) != 1 {
		t.Errorf("expected 1 event after auto-create, got %d", len(events))
	}
}

func TestFileEventStore_MultipleAppendCalls(t *testing.T) {
	// given
	dir := t.TempDir()
	store := sightjack.NewFileEventStore(filepath.Join(dir, "multi.jsonl"))

	// when: append in separate calls
	e1, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, nil)
	store.Append(e1)
	e2, _ := sightjack.NewEvent(sightjack.EventScanCompleted, "s1", 2, nil)
	store.Append(e2)

	// then
	events, _ := store.ReadAll()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestEventsDir(t *testing.T) {
	got := sightjack.EventsDir("/project")
	expected := filepath.Join("/project", ".siren", "events")
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestEventStorePath(t *testing.T) {
	got := sightjack.EventStorePath("/project", "session-123")
	expected := filepath.Join("/project", ".siren", "events", "session-123.jsonl")
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}
