package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sightjack "github.com/hironow/sightjack"
)

func TestArchivePruneCmd_TextOutput_StdoutClean(t *testing.T) {
	// given — expired event file exists
	dir := t.TempDir()
	eventsDir := filepath.Join(dir, sightjack.StateDir, "events")
	os.MkdirAll(eventsDir, 0o755)

	oldFile := filepath.Join(eventsDir, "old-session")
	os.MkdirAll(oldFile, 0o755)
	os.WriteFile(filepath.Join(oldFile, "2025-01-01.jsonl"), []byte(`{"id":"old"}`+"\n"), 0o644)
	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	rootCmd := NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"archive-prune", dir})

	// when
	err := rootCmd.Execute()

	// then — text mode: stdout must be empty (all output to stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outBuf.Len() != 0 {
		t.Errorf("text mode should not write to stdout, got: %q", outBuf.String())
	}
	if !strings.Contains(errBuf.String(), "dry-run") {
		t.Errorf("expected dry-run message in stderr, got: %q", errBuf.String())
	}
}

func TestArchivePruneCmd_JSONOutput_DryRun(t *testing.T) {
	// given — expired event file exists
	dir := t.TempDir()
	eventsDir := filepath.Join(dir, sightjack.StateDir, "events")
	os.MkdirAll(eventsDir, 0o755)

	oldFile := filepath.Join(eventsDir, "old-session")
	os.MkdirAll(oldFile, 0o755)
	os.WriteFile(filepath.Join(oldFile, "2025-01-01.jsonl"), []byte(`{"id":"old"}`+"\n"), 0o644)
	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	rootCmd := NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"--output", "json", "archive-prune", dir})

	// when
	err := rootCmd.Execute()

	// then — JSON to stdout, file not deleted
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(oldFile); os.IsNotExist(statErr) {
		t.Error("dry-run should NOT delete the file")
	}

	var result struct {
		EventCandidates int      `json:"event_candidates"`
		EventDeleted    int      `json:"event_deleted"`
		EventFiles      []string `json:"event_files"`
	}
	if jsonErr := json.Unmarshal(outBuf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", jsonErr, outBuf.String())
	}
	if result.EventCandidates != 1 {
		t.Errorf("event_candidates = %d, want 1", result.EventCandidates)
	}
	if result.EventDeleted != 0 {
		t.Errorf("event_deleted = %d, want 0 (dry-run)", result.EventDeleted)
	}
}

func TestArchivePruneCmd_JSONOutput_Execute(t *testing.T) {
	// given — expired event file exists
	dir := t.TempDir()
	eventsDir := filepath.Join(dir, sightjack.StateDir, "events")
	os.MkdirAll(eventsDir, 0o755)

	oldFile := filepath.Join(eventsDir, "old-session")
	os.MkdirAll(oldFile, 0o755)
	os.WriteFile(filepath.Join(oldFile, "2025-01-01.jsonl"), []byte(`{"id":"old"}`+"\n"), 0o644)
	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	rootCmd := NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"--output", "json", "archive-prune", "--execute", dir})

	// when
	err := rootCmd.Execute()

	// then — JSON to stdout, file deleted
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(oldFile); !os.IsNotExist(statErr) {
		t.Error("--execute should delete the expired file")
	}

	var result struct {
		EventCandidates int `json:"event_candidates"`
		EventDeleted    int `json:"event_deleted"`
	}
	if jsonErr := json.Unmarshal(outBuf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", jsonErr, outBuf.String())
	}
	if result.EventCandidates != 1 {
		t.Errorf("event_candidates = %d, want 1", result.EventCandidates)
	}
	if result.EventDeleted != 1 {
		t.Errorf("event_deleted = %d, want 1", result.EventDeleted)
	}
}
