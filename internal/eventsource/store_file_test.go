package eventsource_test

import (
	"os"
	"path/filepath"
	"testing"

	sightjack "github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/eventsource"
)

func TestFileEventStore_AppendAndReadAll_RoundTrip(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(filepath.Join(dir, "test.jsonl"))

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
	store := eventsource.NewFileEventStore(filepath.Join(dir, "test.jsonl"))

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
	store := eventsource.NewFileEventStore(filepath.Join(dir, "empty.jsonl"))

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
	store := eventsource.NewFileEventStore(filepath.Join(dir, "test.jsonl"))

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
	store := eventsource.NewFileEventStore(filepath.Join(dir, "empty.jsonl"))

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

func TestFileEventStore_SequentialAppend_ManyEvents(t *testing.T) {
	// given: monotonicity check enforces strictly increasing sequences
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(filepath.Join(dir, "sequential.jsonl"))
	count := 50

	// when: append in sequential order
	for i := int64(1); i <= int64(count); i++ {
		e, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", i, nil)
		if err := store.Append(e); err != nil {
			t.Fatalf("append seq %d: %v", i, err)
		}
	}

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

	store := eventsource.NewFileEventStore(path)

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
	store := eventsource.NewFileEventStore(path)

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
	store := eventsource.NewFileEventStore(filepath.Join(dir, "multi.jsonl"))

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

func TestFileEventStore_Append_SequenceMonotonicity_Success(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(filepath.Join(dir, "mono.jsonl"))

	// when: append in strictly increasing order
	for i := int64(1); i <= 3; i++ {
		e, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", i, nil)
		if err := store.Append(e); err != nil {
			t.Fatalf("append seq %d: %v", i, err)
		}
	}

	// then
	events, _ := store.ReadAll()
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

func TestFileEventStore_Append_SequenceGap_Rejected(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(filepath.Join(dir, "gap.jsonl"))

	e1, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, nil)
	store.Append(e1)

	// when: skip sequence 2, append sequence 3
	e3, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 3, nil)
	err := store.Append(e3)

	// then
	if err == nil {
		t.Fatal("expected error for sequence gap")
	}
	events, _ := store.ReadAll()
	if len(events) != 1 {
		t.Errorf("expected 1 event (only seq 1), got %d", len(events))
	}
}

func TestFileEventStore_Append_SequenceDuplicate_Rejected(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(filepath.Join(dir, "dup.jsonl"))

	e1, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, nil)
	e2, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 2, nil)
	store.Append(e1)
	store.Append(e2)

	// when: re-append sequence 2
	dup, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 2, nil)
	err := store.Append(dup)

	// then
	if err == nil {
		t.Fatal("expected error for duplicate sequence")
	}
	events, _ := store.ReadAll()
	if len(events) != 2 {
		t.Errorf("expected 2 events (seq 1,2 only), got %d", len(events))
	}
}

func TestFileEventStore_Append_RejectsInvalidEvent(t *testing.T) {
	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "events", "test.jsonl")
	store := eventsource.NewFileEventStore(path)

	invalid := sightjack.Event{} // all fields empty

	// when
	err := store.Append(invalid)

	// then
	if err == nil {
		t.Fatal("expected error for invalid event")
	}
	// File should not have been created
	if _, statErr := os.Stat(path); statErr == nil {
		t.Error("expected file not to be created for rejected event")
	}
}

func TestFileEventStore_Append_AtomicValidation(t *testing.T) {
	// given: a valid event followed by an invalid event
	dir := t.TempDir()
	path := filepath.Join(dir, "events", "atomic.jsonl")
	store := eventsource.NewFileEventStore(path)

	valid, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "s1", 1, "data")
	invalid := sightjack.Event{SessionID: "s1"} // missing Type, Sequence, etc.

	// when: batch append [valid, invalid]
	err := store.Append(valid, invalid)

	// then: entire batch rejected, valid event NOT written
	if err == nil {
		t.Fatal("expected error for batch with invalid event")
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Error("expected file not to be created: valid event should not be written when batch fails")
	}
}

func TestEventsDir(t *testing.T) {
	got := eventsource.EventsDir("/project")
	expected := filepath.Join("/project", ".siren", "events")
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestEventStorePath(t *testing.T) {
	got := eventsource.EventStorePath("/project", "session-123")
	expected := filepath.Join("/project", ".siren", "events", "session-123.jsonl")
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}
