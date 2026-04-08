package eventsource_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/sightjack/internal/eventsource"
)

func TestValidateStore_LegacyFlatJSONL(t *testing.T) {
	// given: legacy flat .jsonl file at events root (not in session directory)
	stateDir := t.TempDir()
	eventsDir := filepath.Join(stateDir, ".siren", "events")
	if err := os.MkdirAll(eventsDir, 0755); err != nil {
		t.Fatal(err)
	}
	legacyContent := `{"type":"session_started","session_id":"test","timestamp":"2025-01-01T00:00:00Z","data":{}}
{"type":"scan_completed","session_id":"test","timestamp":"2025-01-01T00:01:00Z","data":{}}`
	if err := os.WriteFile(filepath.Join(eventsDir, "legacy-session.jsonl"), []byte(legacyContent), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	health := eventsource.ValidateStore(filepath.Join(stateDir, ".siren"))

	// then: legacy file should be counted
	if health.Err != nil {
		t.Fatalf("unexpected error: %v", health.Err)
	}
	if health.Sessions < 1 {
		t.Errorf("sessions = %d, want >= 1", health.Sessions)
	}
	if health.Events < 2 {
		t.Errorf("events = %d, want >= 2", health.Events)
	}
}

func TestValidateStore_LegacyFlatJSONL_CorruptLine(t *testing.T) {
	// given: legacy flat .jsonl with a corrupt line (counts, does not abort)
	stateDir := t.TempDir()
	eventsDir := filepath.Join(stateDir, ".siren", "events")
	if err := os.MkdirAll(eventsDir, 0755); err != nil {
		t.Fatal(err)
	}
	corruptContent := `{"type":"session_started","timestamp":"2025-01-01T00:00:00Z","data":{}}
NOT_JSON
{"type":"scan_completed","timestamp":"2025-01-01T00:01:00Z","data":{}}`
	if err := os.WriteFile(filepath.Join(eventsDir, "legacy.jsonl"), []byte(corruptContent), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	health := eventsource.ValidateStore(filepath.Join(stateDir, ".siren"))

	// then: corrupt lines counted, valid events also counted
	if health.CorruptLines != 1 {
		t.Errorf("CorruptLines = %d, want 1", health.CorruptLines)
	}
	if health.Events != 2 {
		t.Errorf("Events = %d, want 2", health.Events)
	}
	if health.Err == nil {
		t.Error("expected non-nil Err when corrupt lines exist")
	}
}

func TestValidateStore_CorruptLineInSessionDir_CountsAndContinues(t *testing.T) {
	// given: session dir with valid + corrupt lines
	stateDir := t.TempDir()
	sessionDir := filepath.Join(stateDir, ".siren", "events", "session-1")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `{"type":"session_started","timestamp":"2025-01-01T00:00:00Z","data":{}}
CORRUPT_LINE
{"type":"scan_completed","timestamp":"2025-01-01T00:01:00Z","data":{}}`
	if err := os.WriteFile(filepath.Join(sessionDir, "events.jsonl"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	health := eventsource.ValidateStore(filepath.Join(stateDir, ".siren"))

	// then: corrupt line counted but valid events also counted
	if health.CorruptLines != 1 {
		t.Errorf("CorruptLines = %d, want 1", health.CorruptLines)
	}
	if health.Events != 2 {
		t.Errorf("Events = %d, want 2", health.Events)
	}
	if health.Sessions != 1 {
		t.Errorf("Sessions = %d, want 1", health.Sessions)
	}
	if health.Err == nil {
		t.Error("expected non-nil Err when corrupt lines exist")
	}
}

func TestValidateStore_AllValid_NoCorruption(t *testing.T) {
	// given: clean session
	stateDir := t.TempDir()
	sessionDir := filepath.Join(stateDir, ".siren", "events", "session-1")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `{"type":"session_started","timestamp":"2025-01-01T00:00:00Z","data":{}}
{"type":"scan_completed","timestamp":"2025-01-01T00:01:00Z","data":{}}`
	if err := os.WriteFile(filepath.Join(sessionDir, "events.jsonl"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	health := eventsource.ValidateStore(filepath.Join(stateDir, ".siren"))

	// then
	if health.CorruptLines != 0 {
		t.Errorf("CorruptLines = %d, want 0", health.CorruptLines)
	}
	if health.Err != nil {
		t.Errorf("unexpected error: %v", health.Err)
	}
	if health.Events != 2 {
		t.Errorf("Events = %d, want 2", health.Events)
	}
}

func TestValidateStore_StructuralCorruption(t *testing.T) {
	// given: valid JSON but invalid domain.Event (bad timestamp)
	// json.Valid would accept this, but json.Unmarshal into domain.Event fails
	stateDir := t.TempDir()
	eventsDir := filepath.Join(stateDir, ".siren", "events")
	if err := os.MkdirAll(eventsDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `{"type":"session_started","timestamp":"not-a-date","data":{}}`
	if err := os.WriteFile(filepath.Join(eventsDir, "structural.jsonl"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	health := eventsource.ValidateStore(filepath.Join(stateDir, ".siren"))

	// then: structural corruption detected via json.Unmarshal
	if health.CorruptLines != 1 {
		t.Errorf("CorruptLines = %d, want 1", health.CorruptLines)
	}
	if health.Events != 0 {
		t.Errorf("Events = %d, want 0", health.Events)
	}
}

func TestValidateStore_FlatAndSessionMixed(t *testing.T) {
	// given: both flat .jsonl and session subdir with corrupt lines
	stateDir := t.TempDir()
	eventsDir := filepath.Join(stateDir, ".siren", "events")
	sessionDir := filepath.Join(eventsDir, "session-1")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}

	valid := `{"type":"scan.completed","data":{},"timestamp":"2026-04-08T00:00:00Z","schema_version":1}`
	corrupt := `{broken`

	// flat file with 1 corrupt line
	os.WriteFile(filepath.Join(eventsDir, "legacy.jsonl"),
		[]byte(valid+"\n"+corrupt+"\n"), 0644)
	// session dir with 1 corrupt line
	os.WriteFile(filepath.Join(sessionDir, "2026-04-08.jsonl"),
		[]byte(corrupt+"\n"+valid+"\n"), 0644)

	// when
	health := eventsource.ValidateStore(filepath.Join(stateDir, ".siren"))

	// then: 2 corrupt lines total
	if health.CorruptLines != 2 {
		t.Errorf("CorruptLines = %d, want 2", health.CorruptLines)
	}
	if health.Events != 2 {
		t.Errorf("Events = %d, want 2 (one valid per file)", health.Events)
	}
}

func TestValidateStore_NoEventsDir(t *testing.T) {
	// given: state dir without events/
	stateDir := t.TempDir()
	os.MkdirAll(filepath.Join(stateDir, ".siren"), 0755)

	// when
	health := eventsource.ValidateStore(filepath.Join(stateDir, ".siren"))

	// then
	if !health.NotFound {
		t.Error("expected NotFound=true")
	}
}
