package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestCleanCmd_NothingToClean(t *testing.T) {
	// given: empty directory with no .siren/
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--config", filepath.Join(stateDir, "config.yaml"), "clean", "--yes"})

	// when
	err := cmd.Execute()

	// then: should succeed with "nothing to clean" message
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "Nothing to clean") {
		t.Errorf("expected 'Nothing to clean' in output, got: %s", got)
	}
}

func TestCleanCmd_DeletesStateDir(t *testing.T) {
	// given: .siren/ directory with config
	dir := t.TempDir()
	stateDir := filepath.Join(dir, domain.StateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	cfgPath := filepath.Join(stateDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("team: MY\n"), 0644); err != nil {
		t.Fatalf("create config: %v", err)
	}

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--config", cfgPath, "clean", "--yes"})

	// when
	err := cmd.Execute()

	// then: should succeed and delete .siren/
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Error("expected state dir to be deleted")
	}
}
