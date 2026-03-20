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
