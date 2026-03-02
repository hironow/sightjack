package eventsource_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	sightjack "github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/eventsource"
)

func TestFileEventStore_AppendAndLoadAll_RoundTrip(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir)

	e1, _ := sightjack.NewEvent(sightjack.EventSessionStarted, nil, time.Now())
	e2, _ := sightjack.NewEvent(sightjack.EventScanCompleted, nil, time.Now())

	// when
	if err := store.Append(e1, e2); err != nil {
		t.Fatalf("Append: %v", err)
	}
	events, err := store.LoadAll()

	// then
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
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

func TestFileEventStore_LoadSince_FiltersCorrectly(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir)

	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	for i := range 5 {
		ts := base.Add(time.Duration(i) * time.Minute)
		e, _ := sightjack.NewEvent(sightjack.EventSessionStarted, nil, ts)
		store.Append(e)
	}

	// when: load events after the 3rd event's timestamp (index 2 = base+2min)
	cutoff := base.Add(2 * time.Minute)
	events, err := store.LoadSince(cutoff)

	// then
	if err != nil {
		t.Fatalf("LoadSince: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events (after cutoff), got %d", len(events))
	}
}

func TestFileEventStore_LoadAll_EmptyDir(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir)

	// when
	events, err := store.LoadAll()

	// then
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestFileEventStore_LoadAll_NonExistentDir(t *testing.T) {
	// given
	store := eventsource.NewFileEventStore(filepath.Join(t.TempDir(), "does-not-exist"))

	// when
	events, err := store.LoadAll()

	// then
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events for non-existent dir, got %d", len(events))
	}
}

func TestFileEventStore_ManyEvents(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir)
	count := 50

	// when
	base := time.Now()
	for i := range count {
		e, _ := sightjack.NewEvent(sightjack.EventSessionStarted, nil, base.Add(time.Duration(i)*time.Millisecond))
		if err := store.Append(e); err != nil {
			t.Fatalf("append event %d: %v", i, err)
		}
	}

	// then
	events, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != count {
		t.Errorf("expected %d events, got %d", count, len(events))
	}
}

func TestFileEventStore_CorruptLineSkipped(t *testing.T) {
	// given: a daily JSONL file with one corrupt line
	dir := t.TempDir()

	e1, _ := sightjack.NewEvent(sightjack.EventSessionStarted, nil, time.Now())
	data1, _ := sightjack.MarshalEvent(e1)
	e2, _ := sightjack.NewEvent(sightjack.EventScanCompleted, nil, time.Now())
	data2, _ := sightjack.MarshalEvent(e2)

	content := string(data1) + "\n" + "THIS IS NOT JSON\n" + string(data2) + "\n"
	filename := time.Now().Format("2006-01-02") + ".jsonl"
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644)

	store := eventsource.NewFileEventStore(dir)

	// when
	events, err := store.LoadAll()

	// then: corrupt line is skipped, valid events remain
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events (corrupt skipped), got %d", len(events))
	}
}

func TestFileEventStore_AutoCreateDirectory(t *testing.T) {
	// given: store with non-existent directory
	dir := filepath.Join(t.TempDir(), "sub", "dir", "events")
	store := eventsource.NewFileEventStore(dir)

	e, _ := sightjack.NewEvent(sightjack.EventSessionStarted, nil, time.Now())

	// when
	err := store.Append(e)

	// then
	if err != nil {
		t.Fatalf("Append should auto-create dirs: %v", err)
	}
	events, _ := store.LoadAll()
	if len(events) != 1 {
		t.Errorf("expected 1 event after auto-create, got %d", len(events))
	}
}

func TestFileEventStore_MultipleAppendCalls(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir)

	// when: append in separate calls
	e1, _ := sightjack.NewEvent(sightjack.EventSessionStarted, nil, time.Now())
	store.Append(e1)
	e2, _ := sightjack.NewEvent(sightjack.EventScanCompleted, nil, time.Now())
	store.Append(e2)

	// then
	events, _ := store.LoadAll()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestFileEventStore_UUIDUniqueness(t *testing.T) {
	// given
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir)

	// when
	e1, _ := sightjack.NewEvent(sightjack.EventSessionStarted, nil, time.Now())
	e2, _ := sightjack.NewEvent(sightjack.EventSessionStarted, nil, time.Now())
	store.Append(e1, e2)

	// then
	events, _ := store.LoadAll()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].ID == "" || events[1].ID == "" {
		t.Error("expected non-empty UUIDs")
	}
	if events[0].ID == events[1].ID {
		t.Error("expected unique IDs for different events")
	}
}

func TestFileEventStore_DailyFileRouting(t *testing.T) {
	// given: events with different dates
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir)

	day1 := time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2025, 3, 2, 10, 0, 0, 0, time.UTC)

	e1, _ := sightjack.NewEvent(sightjack.EventSessionStarted, nil, day1)
	e2, _ := sightjack.NewEvent(sightjack.EventScanCompleted, nil, day2)

	// when
	store.Append(e1, e2)

	// then: two separate daily files created
	entries, _ := os.ReadDir(dir)
	jsonlCount := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".jsonl" {
			jsonlCount++
		}
	}
	if jsonlCount != 2 {
		t.Errorf("expected 2 daily JSONL files, got %d", jsonlCount)
	}

	// and all events are returned by LoadAll
	events, _ := store.LoadAll()
	if len(events) != 2 {
		t.Fatalf("expected 2 events from 2 files, got %d", len(events))
	}
}

func TestFileEventStore_Append_RejectsInvalidEvent(t *testing.T) {
	// given
	dir := filepath.Join(t.TempDir(), "events")
	store := eventsource.NewFileEventStore(dir)

	invalid := sightjack.Event{} // all fields empty

	// when
	err := store.Append(invalid)

	// then
	if err == nil {
		t.Fatal("expected error for invalid event")
	}
	// Directory should not have been created
	if _, statErr := os.Stat(dir); statErr == nil {
		t.Error("expected directory not to be created for rejected event")
	}
}

func TestFileEventStore_Append_AtomicValidation(t *testing.T) {
	// given: a valid event followed by an invalid event
	dir := filepath.Join(t.TempDir(), "events")
	store := eventsource.NewFileEventStore(dir)

	valid, _ := sightjack.NewEvent(sightjack.EventSessionStarted, "data", time.Now())
	invalid := sightjack.Event{SessionID: "s1"} // missing ID, Type, Timestamp, Data

	// when: batch append [valid, invalid]
	err := store.Append(valid, invalid)

	// then: entire batch rejected, valid event NOT written
	if err == nil {
		t.Fatal("expected error for batch with invalid event")
	}
	if _, statErr := os.Stat(dir); statErr == nil {
		t.Error("expected directory not to be created: valid event should not be written when batch fails")
	}
}

func TestFileEventStore_ChronologicalOrder(t *testing.T) {
	// given: events appended in reverse chronological order
	dir := t.TempDir()
	store := eventsource.NewFileEventStore(dir)

	later := time.Date(2025, 3, 1, 12, 0, 0, 0, time.UTC)
	earlier := time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC)

	e1, _ := sightjack.NewEvent(sightjack.EventScanCompleted, nil, later)
	e2, _ := sightjack.NewEvent(sightjack.EventSessionStarted, nil, earlier)

	store.Append(e1)
	store.Append(e2)

	// when
	events, _ := store.LoadAll()

	// then: events returned in chronological order
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != sightjack.EventSessionStarted {
		t.Errorf("expected first event to be session_started (earlier), got %s", events[0].Type)
	}
	if events[1].Type != sightjack.EventScanCompleted {
		t.Errorf("expected second event to be scan_completed (later), got %s", events[1].Type)
	}
}

func TestEventsDir(t *testing.T) {
	got := eventsource.EventsDir("/project/.siren")
	expected := filepath.Join("/project/.siren", "events")
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestEventStorePath(t *testing.T) {
	got := eventsource.EventStorePath("/project/.siren", "session-123")
	expected := filepath.Join("/project/.siren", "events", "session-123")
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}
