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

	expect "github.com/Netflix/go-expect"
)

func TestE2E_Run_DryRun(t *testing.T) {
	// given: a configured directory
	dir := initDir(t)

	// when: run --dry-run
	cmd := exec.Command(sightjackBin(), "run", "--dry-run", dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// then
	if err != nil {
		t.Fatalf("run --dry-run failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}

	// Verify prompt files were generated in .siren/.run/<session-id>/
	runDir := filepath.Join(dir, ".siren", ".run")
	sessions, readErr := os.ReadDir(runDir)
	if readErr != nil {
		t.Fatalf("failed to read run dir %s: %v", runDir, readErr)
	}
	if len(sessions) == 0 {
		t.Fatal("expected session directory in .siren/.run/, got none")
	}

	// Check for classify prompt inside the session directory
	sessionDir := filepath.Join(runDir, sessions[0].Name())
	entries, _ := os.ReadDir(sessionDir)
	found := false
	for _, e := range entries {
		if strings.Contains(e.Name(), "classify") {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected classify_prompt.md in session dir, got: %v", names)
	}
}

func TestE2E_Run_NewSession(t *testing.T) {
	// given: a configured directory with fake-claude in PATH
	dir := initDir(t)

	c, err := expect.NewConsole(expect.WithDefaultTimeout(15 * time.Second))
	if err != nil {
		t.Fatalf("create console: %v", err)
	}
	defer c.Close()

	cmd := exec.Command(sightjackBin(), "run", dir)
	cmd.Stdin = c.Tty()
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()

	// when: start interactive session
	if startErr := cmd.Start(); startErr != nil {
		t.Fatalf("start run: %v", startErr)
	}

	// Expect scan progress, then wave selection prompt
	// The scan produces output like "Found 1 clusters"
	c.ExpectString("clusters")

	// Expect wave selection prompt: "Select wave [1-N, b=back, q=quit]:"
	c.ExpectString("Select wave")
	c.SendLine("1")

	// Expect approval prompt: "[a] Approve all  [s] Selective  [r] Reject  [d] Discuss  [q] Back"
	c.ExpectString("Approve all")
	c.SendLine("a")

	// After apply, we get back to navigator. Quit the session.
	c.ExpectString("Select wave")
	c.SendLine("q")

	c.Tty().Close()
	c.ExpectEOF()

	if waitErr := cmd.Wait(); waitErr != nil {
		// May fail in environments without controlling terminal
		t.Skipf("run requires controlling terminal: %v", waitErr)
	}

	// then: state.json should exist
	stateFile := filepath.Join(dir, ".siren", "state.json")
	if _, statErr := os.Stat(stateFile); os.IsNotExist(statErr) {
		t.Error("state.json not created after run session")
	}
}

func TestE2E_Run_QuitImmediately(t *testing.T) {
	// given: a configured directory with fake-claude in PATH
	dir := initDir(t)

	c, err := expect.NewConsole(expect.WithDefaultTimeout(15 * time.Second))
	if err != nil {
		t.Fatalf("create console: %v", err)
	}
	defer c.Close()

	cmd := exec.Command(sightjackBin(), "run", dir)
	cmd.Stdin = c.Tty()
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()

	// when: start and immediately quit at wave selection
	if startErr := cmd.Start(); startErr != nil {
		t.Fatalf("start run: %v", startErr)
	}

	// Wait for wave selection prompt
	c.ExpectString("Select wave")
	c.SendLine("q")

	c.Tty().Close()
	c.ExpectEOF()

	if waitErr := cmd.Wait(); waitErr != nil {
		t.Skipf("run requires controlling terminal: %v", waitErr)
	}

	// then: state should still be saved (paused session)
	stateFile := filepath.Join(dir, ".siren", "state.json")
	if _, statErr := os.Stat(stateFile); os.IsNotExist(statErr) {
		t.Error("state.json not created after quitting run")
	}
}
