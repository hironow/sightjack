package cmd

// white-box-reason: cobra command routing: tests sessions enter subcommand end-to-end
// within the cmd package, using real SQLite session store and fake provider.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// setupSessionsEnterEnv creates a temp directory tree with config + session store
// and a session record ready for "sessions enter" testing.
func setupSessionsEnterEnv(t *testing.T, providerSessionID, workDir string) (repoRoot string, recordID string) {
	t.Helper()
	repoRoot = t.TempDir()
	stateDir := filepath.Join(repoRoot, domain.StateDir)
	runDir := filepath.Join(stateDir, ".run")
	os.MkdirAll(runDir, 0755)

	// Write minimal config with echo as fake provider
	cfgContent := "team: TestTeam\nproject: TestProject\nclaude_cmd: echo\n"
	os.WriteFile(filepath.Join(stateDir, "config.yaml"), []byte(cfgContent), 0644)

	// Create session store and insert a record
	store, err := session.NewSQLiteCodingSessionStore(filepath.Join(runDir, "sessions.db"))
	if err != nil {
		t.Fatalf("create session store: %v", err)
	}
	defer store.Close()

	rec := domain.NewCodingSessionRecord(domain.ProviderClaudeCode, "test-model", workDir)
	rec.ProviderSessionID = providerSessionID
	if err := store.Save(context.Background(), rec); err != nil {
		t.Fatalf("save session record: %v", err)
	}
	return repoRoot, rec.ID
}

func TestSessionsEnter_ByRecordID(t *testing.T) {
	// given
	workDir := t.TempDir()
	repoRoot, recordID := setupSessionsEnterEnv(t, "provider-sess-001", workDir)

	var stdout bytes.Buffer
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"sessions", "enter", "--path", repoRoot, recordID})
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&bytes.Buffer{})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("sessions enter failed: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "--resume provider-sess-001") {
		t.Errorf("expected --resume provider-sess-001, got: %q", output)
	}
	if !strings.Contains(output, "--disable-slash-commands") {
		t.Errorf("expected --disable-slash-commands, got: %q", output)
	}
}

func TestSessionsEnter_ByProviderID(t *testing.T) {
	// given
	workDir := t.TempDir()
	repoRoot, _ := setupSessionsEnterEnv(t, "provider-sess-002", workDir)

	var stdout bytes.Buffer
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"sessions", "enter", "--path", repoRoot, "--provider-id", "provider-sess-002"})
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&bytes.Buffer{})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("sessions enter --provider-id failed: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "--resume provider-sess-002") {
		t.Errorf("expected --resume provider-sess-002, got: %q", output)
	}
}

func TestSessionsEnter_ConfigBaseIsRepoRoot(t *testing.T) {
	// Regression test for GAP-ARCH-037: ConfigBase must be repoRoot, not stateDir.
	// If ConfigBase were stateDir, settings would resolve to .siren/.siren/...

	// given: config + settings.json in correct location
	workDir := t.TempDir()
	repoRoot, recordID := setupSessionsEnterEnv(t, "provider-sess-037", workDir)

	// Create settings.json at the correct location (repoRoot/.siren/.claude/settings.json)
	settingsDir := filepath.Join(repoRoot, domain.StateDir, ".claude")
	os.MkdirAll(settingsDir, 0755)
	os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(`{"key":"value"}`), 0644)

	var stdout bytes.Buffer
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"sessions", "enter", "--path", repoRoot, recordID})
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&bytes.Buffer{})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("sessions enter failed: %v", err)
	}
	output := stdout.String()
	// --settings flag should be present (settings.json exists at correct path)
	if !strings.Contains(output, "--settings") {
		t.Errorf("expected --settings flag (ConfigBase=repoRoot resolves correctly), got: %q", output)
	}
}
