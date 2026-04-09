package cmd_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/cmd"
	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"

	_ "modernc.org/sqlite"
)

// setupDeadLetterDB creates an outbox.db with the real schema and optionally
// stages items with retry_count forced to make them dead-lettered.
func setupDeadLetterDB(t *testing.T, dir string, deadLetterCount int) {
	t.Helper()
	if err := session.EnsureMailDirs(dir); err != nil {
		t.Fatalf("ensure mail dirs: %v", err)
	}
	// Create real outbox DB via session.NewOutboxStoreForDir (real schema).
	store, storeErr := session.NewOutboxStoreForDir(dir)
	if storeErr != nil {
		t.Fatalf("create outbox store: %v", storeErr)
	}
	store.Close()

	if deadLetterCount == 0 {
		return
	}

	// Open DB directly to insert dead-lettered rows.
	dbPath := filepath.Join(dir, domain.StateDir, ".run", "outbox.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	stmt, err := db.Prepare(`INSERT INTO staged (name, data, flushed, retry_count) VALUES (?, ?, 0, 5)`)
	if err != nil {
		t.Fatalf("prepare insert: %v", err)
	}
	defer stmt.Close()
	for i := 0; i < deadLetterCount; i++ {
		name := fmt.Sprintf("dead-%d.dmail", i)
		if _, err := stmt.Exec(name, []byte(`{"kind":"report"}`)); err != nil {
			t.Fatalf("insert dead letter row %d: %v", i, err)
		}
	}
}

func TestDeadLettersCmd_SubcommandRegistered(t *testing.T) {
	// given
	rootCmd := cmd.NewRootCommand()

	// when — find dead-letters subcommand
	var dlCmd *cobra.Command
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "dead-letters" {
			dlCmd = sub
			break
		}
	}

	// then
	if dlCmd == nil {
		t.Fatal("dead-letters subcommand not found")
	}

	var purgeCmd *cobra.Command
	for _, sub := range dlCmd.Commands() {
		if sub.Name() == "purge" {
			purgeCmd = sub
			break
		}
	}
	if purgeCmd == nil {
		t.Fatal("dead-letters purge subcommand not found")
	}
}

func TestDeadLettersPurgeCmd_DryRun_NoDeadLetters(t *testing.T) {
	// given — empty outbox store (no dead letters)
	dir := t.TempDir()
	setupDeadLetterDB(t, dir, 0)

	rootCmd := cmd.NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"dead-letters", "purge", dir})

	// when
	execErr := rootCmd.Execute()

	// then — no error, stdout clean
	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}
	if outBuf.Len() != 0 {
		t.Errorf("text mode should not write to stdout, got: %q", outBuf.String())
	}
	if got := errBuf.String(); !strings.Contains(got, "No dead-lettered") {
		t.Errorf("expected 'No dead-lettered' in stderr, got: %q", got)
	}
}

func TestDeadLettersPurgeCmd_DryRun_WithDeadLetters(t *testing.T) {
	// given — outbox store with 1 dead-lettered item
	dir := t.TempDir()
	setupDeadLetterDB(t, dir, 1)

	rootCmd := cmd.NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"dead-letters", "purge", dir})

	// when
	execErr := rootCmd.Execute()

	// then — dry-run message
	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}
	if outBuf.Len() != 0 {
		t.Errorf("text mode should not write to stdout, got: %q", outBuf.String())
	}
	got := errBuf.String()
	if !strings.Contains(got, "1 dead-lettered") {
		t.Errorf("expected dead letter count in stderr, got: %q", got)
	}
	if !strings.Contains(got, "dry-run") {
		t.Errorf("expected dry-run message in stderr, got: %q", got)
	}
}

func TestDeadLettersPurgeCmd_JSON_DryRun(t *testing.T) {
	// given — outbox store with 2 dead-lettered items
	dir := t.TempDir()
	setupDeadLetterDB(t, dir, 2)

	rootCmd := cmd.NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"--output", "json", "dead-letters", "purge", dir})

	// when
	execErr := rootCmd.Execute()

	// then — JSON output, nothing purged
	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}
	var result struct {
		DeadLetters int `json:"dead_letters"`
		Purged      int `json:"purged"`
	}
	if jsonErr := json.Unmarshal(outBuf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", jsonErr, outBuf.String())
	}
	if result.DeadLetters != 2 {
		t.Errorf("dead_letters = %d, want 2", result.DeadLetters)
	}
	if result.Purged != 0 {
		t.Errorf("purged = %d, want 0 (dry-run)", result.Purged)
	}
}

func TestDeadLettersPurgeCmd_JSON_Execute(t *testing.T) {
	// given — outbox store with 1 dead-lettered item
	dir := t.TempDir()
	setupDeadLetterDB(t, dir, 1)

	rootCmd := cmd.NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"--output", "json", "dead-letters", "purge", "--execute", dir})

	// when
	execErr := rootCmd.Execute()

	// then — purged in JSON
	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}
	var result struct {
		DeadLetters int `json:"dead_letters"`
		Purged      int `json:"purged"`
	}
	if jsonErr := json.Unmarshal(outBuf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", jsonErr, outBuf.String())
	}
	if result.DeadLetters != 1 {
		t.Errorf("dead_letters = %d, want 1", result.DeadLetters)
	}
	if result.Purged != 1 {
		t.Errorf("purged = %d, want 1", result.Purged)
	}
}

func TestDeadLettersPurgeCmd_Execute_MutuallyExclusiveWithDryRun(t *testing.T) {
	// given
	dir := t.TempDir()
	setupDeadLetterDB(t, dir, 1)

	rootCmd := cmd.NewRootCommand()
	outBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(outBuf)
	rootCmd.SetArgs([]string{"dead-letters", "purge", "--execute", "--dry-run", dir})

	// when
	execErr := rootCmd.Execute()

	// then — should fail
	if execErr == nil {
		t.Fatal("expected error when combining --execute with --dry-run")
	}
	if !strings.Contains(execErr.Error(), "mutually exclusive") {
		t.Errorf("error should mention 'mutually exclusive', got: %v", execErr)
	}
}

func TestDeadLettersPurgeCmd_NoDB_NoSideEffect(t *testing.T) {
	// given — directory with no outbox DB
	dir := t.TempDir()

	rootCmd := cmd.NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"dead-letters", "purge", dir})

	// when
	execErr := rootCmd.Execute()

	// then — no error, no DB created
	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}
	dbPath := filepath.Join(dir, domain.StateDir, ".run", "outbox.db")
	if _, statErr := os.Stat(dbPath); statErr == nil {
		t.Error("outbox.db should NOT be created as side-effect")
	}
	if !strings.Contains(errBuf.String(), "No dead-lettered") {
		t.Errorf("expected 'No dead-lettered' message, got: %q", errBuf.String())
	}
}
