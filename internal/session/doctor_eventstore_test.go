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
