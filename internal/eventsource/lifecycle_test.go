package eventsource_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/eventsource"
)

func TestListExpiredEventFiles_FiltersOlderThanThreshold(t *testing.T) {
	// given: 2 old files + 1 new file
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".siren")
	eventsDir := filepath.Join(stateDir, "events")
	os.MkdirAll(eventsDir, 0755)

	oldTime := time.Now().Add(-60 * 24 * time.Hour) // 60 days ago
	for _, name := range []string{"old1.jsonl", "old2.jsonl"} {
		path := filepath.Join(eventsDir, name)
		os.WriteFile(path, []byte("{}"), 0644)
		os.Chtimes(path, oldTime, oldTime)
	}
	os.WriteFile(filepath.Join(eventsDir, "new.jsonl"), []byte("{}"), 0644)

	// when
	expired, err := eventsource.ListExpiredEventFiles(stateDir, 30)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(expired) != 2 {
		t.Errorf("expected 2 expired files, got %d", len(expired))
	}
}

func TestListExpiredEventFiles_EmptyDir(t *testing.T) {
	// given: empty events directory
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".siren")
	eventsDir := filepath.Join(stateDir, "events")
	os.MkdirAll(eventsDir, 0755)

	// when
	expired, err := eventsource.ListExpiredEventFiles(stateDir, 30)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(expired) != 0 {
		t.Errorf("expected 0 expired files, got %d", len(expired))
	}
}

func TestListExpiredEventFiles_NonExistentDir(t *testing.T) {
	// given: non-existent state directory
	stateDir := filepath.Join(t.TempDir(), "nonexistent")

	// when
	expired, err := eventsource.ListExpiredEventFiles(stateDir, 30)

	// then: empty slice, not error
	if err != nil {
		t.Fatalf("expected nil error for non-existent dir, got: %v", err)
	}
	if len(expired) != 0 {
		t.Errorf("expected 0 expired files, got %d", len(expired))
	}
}

func TestPruneEventFiles_DeletesSpecifiedFiles(t *testing.T) {
	// given
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".siren")
	eventsDir := filepath.Join(stateDir, "events")
	os.MkdirAll(eventsDir, 0755)

	for _, name := range []string{"a.jsonl", "b.jsonl", "keep.jsonl"} {
		os.WriteFile(filepath.Join(eventsDir, name), []byte("{}"), 0644)
	}

	// when: delete only a.jsonl and b.jsonl
	deleted, err := eventsource.PruneEventFiles(stateDir, []string{"a.jsonl", "b.jsonl"})

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 2 {
		t.Errorf("expected 2 deleted, got %d", len(deleted))
	}

	// keep.jsonl should remain
	if _, err := os.Stat(filepath.Join(eventsDir, "keep.jsonl")); err != nil {
		t.Error("keep.jsonl should not have been deleted")
	}
	// a.jsonl and b.jsonl should be gone
	for _, name := range []string{"a.jsonl", "b.jsonl"} {
		if _, err := os.Stat(filepath.Join(eventsDir, name)); err == nil {
			t.Errorf("%s should have been deleted", name)
		}
	}
}
