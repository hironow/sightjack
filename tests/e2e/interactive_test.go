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

// isTTYError returns true if the error is related to a missing controlling
// terminal (/dev/tty). Only these errors warrant t.Skip; other failures
// should be reported as real test failures.
func isTTYError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "controlling terminal") ||
		strings.Contains(msg, "/dev/tty") ||
		strings.Contains(msg, "CONIN$") ||
		strings.Contains(msg, "device not configured")
}

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

	// Find the first directory entry (skip files like outbox.db)
	var sessionDir string
	for _, s := range sessions {
		if s.IsDir() {
			sessionDir = filepath.Join(runDir, s.Name())
			break
		}
	}
	if sessionDir == "" {
		t.Fatal("expected session directory in .siren/.run/, got none")
	}

	// Check for classify prompt inside the session directory
	entries, dirErr := os.ReadDir(sessionDir)
	if dirErr != nil {
		t.Fatalf("read session dir %s: %v", sessionDir, dirErr)
	}
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
	if _, expErr := c.ExpectString("clusters"); expErr != nil {
		t.Fatalf("expected 'clusters' output: %v", expErr)
	}

	// Expect wave selection prompt: "Select wave [1-N, b=back, q=quit]:"
	if _, expErr := c.ExpectString("Select wave"); expErr != nil {
		t.Fatalf("expected 'Select wave' prompt: %v", expErr)
	}
	if _, expErr := c.SendLine("1"); expErr != nil {
		t.Fatalf("failed to send '1': %v", expErr)
	}

	// Expect approval prompt: "[a] Approve all  [s] Selective  [r] Reject  [d] Discuss  [q] Back"
	if _, expErr := c.ExpectString("Approve all"); expErr != nil {
		t.Fatalf("expected 'Approve all' prompt: %v", expErr)
	}
	if _, expErr := c.SendLine("a"); expErr != nil {
		t.Fatalf("failed to send 'a': %v", expErr)
	}

	// After apply, the session enters nextgen which may take time.
	// Close TTY to signal EOF — the session will save state and exit.
	// We've already verified the full interactive path: scan → waves → select → approve.
	// Brief pause lets apply start before TTY close signals exit.
	time.Sleep(500 * time.Millisecond)
	c.Tty().Close()
	if _, eofErr := c.ExpectEOF(); eofErr != nil {
		t.Logf("ExpectEOF: %v", eofErr)
	}

	if waitErr := cmd.Wait(); waitErr != nil {
		if isTTYError(waitErr) {
			t.Skipf("run requires controlling terminal: %v", waitErr)
		}
		t.Fatalf("run exited with error: %v", waitErr)
	}

	// then: events should exist
	assertEventsExist(t, dir)
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
	if _, expErr := c.ExpectString("Select wave"); expErr != nil {
		t.Fatalf("expected 'Select wave' prompt: %v", expErr)
	}
	if _, expErr := c.SendLine("q"); expErr != nil {
		t.Fatalf("failed to send 'q': %v", expErr)
	}

	c.Tty().Close()
	if _, eofErr := c.ExpectEOF(); eofErr != nil {
		t.Logf("ExpectEOF: %v", eofErr)
	}

	if waitErr := cmd.Wait(); waitErr != nil {
		if isTTYError(waitErr) {
			t.Skipf("run requires controlling terminal: %v", waitErr)
		}
		t.Fatalf("run exited with error: %v", waitErr)
	}

	// then: events should still be saved (paused session)
	assertEventsExist(t, dir)
}
