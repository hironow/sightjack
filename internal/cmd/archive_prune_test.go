package cmd_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/cmd"
	"github.com/hironow/sightjack/internal/domain"
)

func TestArchivePruneCmd_TextOutput_StdoutClean(t *testing.T) {
	// given — expired event file exists
	dir := t.TempDir()
	eventsDir := filepath.Join(dir, domain.StateDir, "events")
	os.MkdirAll(eventsDir, 0o755)

	oldFile := filepath.Join(eventsDir, "old-session")
	os.MkdirAll(oldFile, 0o755)
	os.WriteFile(filepath.Join(oldFile, "2025-01-01.jsonl"), []byte(`{"id":"old"}`+"\n"), 0o644)
	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	rootCmd := cmd.NewRootCommand()
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
	eventsDir := filepath.Join(dir, domain.StateDir, "events")
	os.MkdirAll(eventsDir, 0o755)

	oldFile := filepath.Join(eventsDir, "old-session")
	os.MkdirAll(oldFile, 0o755)
	os.WriteFile(filepath.Join(oldFile, "2025-01-01.jsonl"), []byte(`{"id":"old"}`+"\n"), 0o644)
	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	rootCmd := cmd.NewRootCommand()
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
	if _, statErr := os.Stat(oldFile); errors.Is(statErr, fs.ErrNotExist) {
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
	eventsDir := filepath.Join(dir, domain.StateDir, "events")
	os.MkdirAll(eventsDir, 0o755)

	oldFile := filepath.Join(eventsDir, "old-session")
	os.MkdirAll(oldFile, 0o755)
	os.WriteFile(filepath.Join(oldFile, "2025-01-01.jsonl"), []byte(`{"id":"old"}`+"\n"), 0o644)
	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	rootCmd := cmd.NewRootCommand()
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
	if _, statErr := os.Stat(oldFile); !errors.Is(statErr, fs.ErrNotExist) {
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

func TestArchivePruneCmd_RebuildIndexFlag_Exists(t *testing.T) {
	// given
	rootCmd := cmd.NewRootCommand()

	// when — find archive-prune subcommand
	var apCmd *cobra.Command
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "archive-prune" {
			apCmd = sub
			break
		}
	}
	if apCmd == nil {
		t.Fatal("archive-prune subcommand not found")
	}

	// then
	f := apCmd.Flags().Lookup("rebuild-index")
	if f == nil {
		t.Fatal("--rebuild-index flag not found on archive-prune")
	}
}

func TestArchivePruneCmd_RebuildIndex_ConflictsWithExecute(t *testing.T) {
	// given
	dir := t.TempDir()
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"archive-prune", "--rebuild-index", "--execute", dir})

	// when
	err := rootCmd.Execute()

	// then — should fail
	if err == nil {
		t.Fatal("expected error when combining --rebuild-index with --execute")
	}
	if !strings.Contains(err.Error(), "rebuild-index") {
		t.Errorf("error should mention rebuild-index, got: %v", err)
	}
}

func TestArchivePruneCmd_YesFlag_Exists(t *testing.T) {
	// given
	rootCmd := cmd.NewRootCommand()

	// when — find archive-prune subcommand
	var apCmd *cobra.Command
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "archive-prune" {
			apCmd = sub
			break
		}
	}
	if apCmd == nil {
		t.Fatal("archive-prune subcommand not found")
	}

	// then
	f := apCmd.Flags().Lookup("yes")
	if f == nil {
		t.Fatal("--yes flag not found on archive-prune")
	}
	if f.DefValue != "false" {
		t.Errorf("--yes default = %q, want %q", f.DefValue, "false")
	}
	if f.Shorthand != "y" {
		t.Errorf("--yes shorthand = %q, want %q", f.Shorthand, "y")
	}
}

func TestArchivePruneCmd_RebuildIndex_CreatesIndex(t *testing.T) {
	// given — state directory with archive subdirectory
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".siren")
	archiveDir := filepath.Join(stateDir, "archive")
	os.MkdirAll(archiveDir, 0o755)
	os.WriteFile(filepath.Join(archiveDir, "2025-01-01.jsonl"), []byte(`{"id":"1","tool":"sightjack"}`+"\n"), 0o644)

	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"archive-prune", "--rebuild-index", dir})

	// when
	err := rootCmd.Execute()

	// then — should succeed and create index file
	if err != nil {
		t.Fatalf("--rebuild-index failed: %v", err)
	}
	indexPath := filepath.Join(archiveDir, "index.jsonl")
	if _, statErr := os.Stat(indexPath); os.IsNotExist(statErr) {
		t.Error("expected index.jsonl to be created by --rebuild-index")
	}
}
