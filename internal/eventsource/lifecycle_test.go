package eventsource_test

import (
	"os"
	"path/filepath"
	"strings"
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

func TestListOversizedEventFiles_ReturnsBigFiles(t *testing.T) {
	// given
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".siren")
	eventsDir := filepath.Join(stateDir, "events")
	os.MkdirAll(eventsDir, 0o755)

	// today's file — must be excluded even if oversized
	today := time.Now().Format("2006-01-02")
	todayFile := filepath.Join(eventsDir, today+".jsonl")
	bigContent := make([]byte, eventsource.EventFileSizeThreshold+1)
	if err := os.WriteFile(todayFile, bigContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// yesterday big file — must be included
	yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
	yesterdayFile := filepath.Join(eventsDir, yesterday+".jsonl")
	if err := os.WriteFile(yesterdayFile, bigContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// small file — must be excluded
	smallFile := filepath.Join(eventsDir, "2026-01-01.jsonl")
	if err := os.WriteFile(smallFile, []byte(`{"id":"1"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// when
	files, err := eventsource.ListOversizedEventFiles(stateDir)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 oversized file, got %d: %v", len(files), files)
	}
	if files[0] != yesterday+".jsonl" {
		t.Errorf("expected %s.jsonl, got %s", yesterday, files[0])
	}
}

func TestListOversizedEventFiles_DirNotExist(t *testing.T) {
	// given
	stateDir := filepath.Join(t.TempDir(), "nonexistent")

	// when
	files, err := eventsource.ListOversizedEventFiles(stateDir)

	// then
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if files != nil {
		t.Errorf("expected nil for missing directory, got %v", files)
	}
}

func TestTruncateEventFile_KeepsLastNLines(t *testing.T) {
	// given
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".siren")
	eventsDir := filepath.Join(stateDir, "events")
	os.MkdirAll(eventsDir, 0o755)

	lines := []string{
		`{"id":"1"}`,
		`{"id":"2"}`,
		`{"id":"3"}`,
		`{"id":"4"}`,
		`{"id":"5"}`,
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	fname := "2026-01-15.jsonl"
	fpath := filepath.Join(eventsDir, fname)
	if err := os.WriteFile(fpath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// when — keep last 3 lines
	if err := eventsource.TruncateEventFile(stateDir, fname, 3); err != nil {
		t.Fatalf("TruncateEventFile: %v", err)
	}

	// then
	data, readErr := os.ReadFile(fpath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	got := string(data)
	for _, want := range []string{`{"id":"3"}`, `{"id":"4"}`, `{"id":"5"}`} {
		if !strings.Contains(got, want) {
			t.Errorf("expected line %q in result, got:\n%s", want, got)
		}
	}
	for _, notWant := range []string{`{"id":"1"}`, `{"id":"2"}`} {
		if strings.Contains(got, notWant) {
			t.Errorf("expected line %q to be removed, got:\n%s", notWant, got)
		}
	}
}
