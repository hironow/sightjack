package session

// white-box-reason: tests unexported checkEventStoreIntegrity function

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestCheckEventStoreIntegrity_CorruptLines(t *testing.T) {
	// given: base dir with corrupt event lines in a session subdir
	baseDir := t.TempDir()
	sessionDir := filepath.Join(baseDir, domain.StateDir, "events", "test-session")
	os.MkdirAll(sessionDir, 0755)

	validEvent := `{"type":"scan.completed","data":{},"timestamp":"2026-04-08T00:00:00Z","schema_version":1}`
	corruptLine := `{not valid json`
	os.WriteFile(filepath.Join(sessionDir, "2026-04-08.jsonl"),
		[]byte(validEvent+"\n"+corruptLine+"\n"+validEvent+"\n"), 0644)

	// when
	check := checkEventStoreIntegrity(baseDir)

	// then
	if check.Status != domain.CheckWarn {
		t.Errorf("expected WARN, got %s: %s", check.Status.StatusLabel(), check.Message)
	}
	if !strings.Contains(check.Message, "1 corrupt line") {
		t.Errorf("expected '1 corrupt line' in message: %q", check.Message)
	}
}

func TestCheckEventStoreIntegrity_FlatJSONL(t *testing.T) {
	// given: legacy flat .jsonl file directly in events/ root
	baseDir := t.TempDir()
	eventsDir := filepath.Join(baseDir, domain.StateDir, "events")
	os.MkdirAll(eventsDir, 0755)

	validEvent := `{"type":"scan.completed","data":{},"timestamp":"2026-04-08T00:00:00Z","schema_version":1}`
	corruptLine := `{not valid json`
	os.WriteFile(filepath.Join(eventsDir, "legacy-session.jsonl"),
		[]byte(validEvent+"\n"+corruptLine+"\n"), 0644)

	// when
	check := checkEventStoreIntegrity(baseDir)

	// then
	if check.Status != domain.CheckWarn {
		t.Errorf("expected WARN for flat .jsonl corrupt, got %s: %s", check.Status.StatusLabel(), check.Message)
	}
	if !strings.Contains(check.Message, "1 corrupt line") {
		t.Errorf("expected '1 corrupt line' in message: %q", check.Message)
	}
}

func TestCheckEventStoreIntegrity_FlatAndSessionMixed(t *testing.T) {
	// given: both flat .jsonl and session subdir with corrupt lines
	baseDir := t.TempDir()
	eventsDir := filepath.Join(baseDir, domain.StateDir, "events")
	sessionDir := filepath.Join(eventsDir, "session-1")
	os.MkdirAll(sessionDir, 0755)

	valid := `{"type":"scan.completed","data":{},"timestamp":"2026-04-08T00:00:00Z","schema_version":1}`
	corrupt := `{broken`

	// flat file with 1 corrupt line
	os.WriteFile(filepath.Join(eventsDir, "legacy.jsonl"),
		[]byte(valid+"\n"+corrupt+"\n"), 0644)
	// session dir with 1 corrupt line
	os.WriteFile(filepath.Join(sessionDir, "2026-04-08.jsonl"),
		[]byte(corrupt+"\n"+valid+"\n"), 0644)

	// when
	check := checkEventStoreIntegrity(baseDir)

	// then
	if check.Status != domain.CheckWarn {
		t.Errorf("expected WARN, got %s: %s", check.Status.StatusLabel(), check.Message)
	}
	if !strings.Contains(check.Message, "2 corrupt line") {
		t.Errorf("expected '2 corrupt line' in message: %q", check.Message)
	}
}

func TestCheckEventStoreIntegrity_StructuralCorruption(t *testing.T) {
	// given: valid JSON but invalid Event structure (timestamp not RFC3339)
	baseDir := t.TempDir()
	eventsDir := filepath.Join(baseDir, domain.StateDir, "events")
	os.MkdirAll(eventsDir, 0755)

	structuralCorrupt := `{"type":"scan.completed","data":{},"timestamp":"not-a-date"}`
	os.WriteFile(filepath.Join(eventsDir, "structural.jsonl"),
		[]byte(structuralCorrupt+"\n"), 0644)

	// when
	check := checkEventStoreIntegrity(baseDir)

	// then: json.Unmarshal into domain.Event fails on bad timestamp
	if check.Status != domain.CheckWarn {
		t.Errorf("expected WARN for structural corruption, got %s: %s", check.Status.StatusLabel(), check.Message)
	}
}

func TestCheckEventStoreIntegrity_Clean(t *testing.T) {
	// given: clean event store in session subdir
	baseDir := t.TempDir()
	sessionDir := filepath.Join(baseDir, domain.StateDir, "events", "test-session")
	os.MkdirAll(sessionDir, 0755)

	validEvent := `{"type":"scan.completed","data":{},"timestamp":"2026-04-08T00:00:00Z","schema_version":1}`
	os.WriteFile(filepath.Join(sessionDir, "2026-04-08.jsonl"),
		[]byte(validEvent+"\n"), 0644)

	// when
	check := checkEventStoreIntegrity(baseDir)

	// then
	if check.Status != domain.CheckOK {
		t.Errorf("expected OK, got %s: %s", check.Status.StatusLabel(), check.Message)
	}
}

func TestCheckEventStoreIntegrity_NoEventsDir(t *testing.T) {
	// given: base dir without events/
	baseDir := t.TempDir()

	// when
	check := checkEventStoreIntegrity(baseDir)

	// then
	if check.Status != domain.CheckSkip {
		t.Errorf("expected SKIP, got %s: %s", check.Status.StatusLabel(), check.Message)
	}
}
