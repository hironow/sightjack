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
	legacyContent := `{"type":"session_started","session_id":"test","timestamp":"2025-01-01T00:00:00Z","payload":{}}
{"type":"scan_completed","session_id":"test","timestamp":"2025-01-01T00:01:00Z","payload":{}}`
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
	// given: legacy flat .jsonl with a corrupt line
	stateDir := t.TempDir()
	eventsDir := filepath.Join(stateDir, ".siren", "events")
	if err := os.MkdirAll(eventsDir, 0755); err != nil {
		t.Fatal(err)
	}
	corruptContent := `{"type":"session_started"}
NOT_JSON
{"type":"scan_completed"}`
	if err := os.WriteFile(filepath.Join(eventsDir, "legacy.jsonl"), []byte(corruptContent), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	health := eventsource.ValidateStore(filepath.Join(stateDir, ".siren"))

	// then: should report corrupt JSON error
	if health.Err == nil {
		t.Fatal("expected error for corrupt JSON in legacy file")
	}
}

func TestValidateStore_CorruptLineInSessionDir_CountsAndContinues(t *testing.T) {
	// given: session dir with valid + corrupt lines
	stateDir := t.TempDir()
	sessionDir := filepath.Join(stateDir, ".siren", "events", "session-1")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `{"type":"session_started"}
CORRUPT_LINE
{"type":"scan_completed"}`
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
	content := `{"type":"session_started"}
{"type":"scan_completed"}`
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
