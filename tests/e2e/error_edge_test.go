//go:build e2e

package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestE2E_Scan_NoConfig(t *testing.T) {
	// given: a directory with no .siren/config.yaml
	dir := t.TempDir()

	// when: attempt scan
	cmd := exec.Command(sightjackBin(), "scan", "--json", dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// then: should fail with meaningful error
	if err == nil {
		t.Fatal("expected error when scanning without config")
	}
	combined := stderr.String() + stdout.String()
	if len(combined) == 0 {
		t.Error("expected error output, got nothing")
	}
}

func TestE2E_Apply_InvalidJSON(t *testing.T) {
	// given: a configured directory + malformed stdin
	dir := initDir(t)

	// when: pipe invalid JSON into apply
	cmd := exec.Command(sightjackBin(), "apply", dir)
	cmd.Stdin = strings.NewReader("this is not json {{{")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// then: should fail with error, not panic
	if err == nil {
		t.Fatal("expected error for invalid JSON input")
	}
}

func TestE2E_Waves_InvalidJSON(t *testing.T) {
	// given: a configured directory + malformed stdin
	dir := initDir(t)

	// when: pipe invalid JSON into waves
	cmd := exec.Command(sightjackBin(), "waves", dir)
	cmd.Stdin = strings.NewReader("not valid json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// then: should fail with error, not panic
	if err == nil {
		t.Fatal("expected error for invalid JSON input to waves")
	}
}

func TestE2E_Verbose_Flag(t *testing.T) {
	// when: run version with --verbose
	cmd := exec.Command(sightjackBin(), "version", "--verbose")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// then: should not crash
	if err != nil {
		t.Fatalf("version --verbose failed: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "sightjack v") {
		t.Errorf("expected version string, got: %s", stdout.String())
	}
}

func TestE2E_State_Persistence(t *testing.T) {
	// given: run a full scan to generate state
	dir := initDir(t)

	cmd := exec.Command(sightjackBin(), "scan", "--json", dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("scan failed: %v\nstderr: %s", err, stderr.String())
	}

	// then: state.json should exist
	stateFile := filepath.Join(dir, ".siren", "state.json")
	if _, statErr := os.Stat(stateFile); os.IsNotExist(statErr) {
		t.Error("state.json not created after scan")
	}

	// and: show should succeed
	showCmd := exec.Command(sightjackBin(), "show", dir)
	var showOut, showErr bytes.Buffer
	showCmd.Stdout = &showOut
	showCmd.Stderr = &showErr
	if runErr := showCmd.Run(); runErr != nil {
		t.Fatalf("show failed after scan: %v\nstderr: %s", runErr, showErr.String())
	}
	if showOut.Len() == 0 && showErr.Len() == 0 {
		t.Error("show produced no output")
	}
}

func TestE2E_State_ArchivePrune_WithData(t *testing.T) {
	// given: a directory with old archived d-mail files
	dir := initDir(t)
	archiveDir := filepath.Join(dir, ".siren", "archive")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldFile := filepath.Join(archiveDir, "old-feedback.md")
	if err := os.WriteFile(oldFile, []byte("# Old feedback"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Set mtime to 60 days ago so it's expired
	oldTime := time.Now().Add(-60 * 24 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	// when: archive-prune with --days 0 --execute (prune everything)
	cmd := exec.Command(sightjackBin(), "archive-prune", "--days", "0", "--execute", dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// then: should succeed
	if err != nil {
		t.Fatalf("archive-prune failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}

	// and: old file should be removed
	if _, statErr := os.Stat(oldFile); !os.IsNotExist(statErr) {
		t.Error("old archive file should have been pruned")
	}
}
