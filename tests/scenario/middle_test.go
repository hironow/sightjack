//go:build scenario

package scenario_test

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	expect "github.com/Netflix/go-expect"
)

func TestScenario_L3_Middle(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "middle")
	obs := NewObserver(ws, t)

	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// First scan
	err := ws.RunSightjackScan(t, ctx)
	if err != nil {
		t.Logf("first scan: %v", err)
	}

	ws.WaitForDMailCount(t, ".expedition", "inbox", 1, 30*time.Second)
	ws.WaitForAbsent(t, ".siren", "outbox", 15*time.Second)

	// Inject convergence D-Mail
	convergence := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "convergence-001",
		"kind":                 "convergence",
		"description":          "System convergence checkpoint",
	}, "# Convergence\n\nAll tools have stabilized.")
	ws.InjectDMail(t, ".siren", "inbox", "convergence-001.md", convergence)

	// Second scan
	err = ws.RunSightjackScan(t, ctx)
	if err != nil {
		t.Logf("second scan: %v", err)
	}

	// Verify no deadlock, all outboxes eventually empty
	ws.WaitForAbsent(t, ".siren", "outbox", 15*time.Second)
	obs.AssertAllOutboxEmpty()
}

// TestScenario_L3_Interactive tests the interactive wave selection path
// without --auto-approve. Uses go-expect (PTY-based terminal automation)
// to respond to prompts: wave selection ("1") and approval ("a").
func TestScenario_L3_Interactive(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "middle")

	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// Create a PTY console for go-expect with a generous per-expect timeout.
	c, err := expect.NewConsole(expect.WithDefaultTimeout(60 * time.Second))
	if err != nil {
		t.Fatalf("create console: %v", err)
	}
	defer c.Close()

	// Build sightjack run command WITHOUT --auto-approve.
	// Connect stdin/stdout/stderr to the PTY so go-expect can drive the
	// interactive prompts (wave selection + approval).
	cmd := exec.CommandContext(ctx, "sightjack", "run", ws.RepoPath)
	cmd.Dir = ws.RepoPath
	cmd.Env = append(os.Environ(), ws.Env...)
	cmd.Stdin = c.Tty()
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()

	if startErr := cmd.Start(); startErr != nil {
		t.Fatalf("start sightjack run: %v", startErr)
	}

	// --- Interactive sequence ---

	// 1. Wait for wave selection prompt: "Select wave [1-N, b=back, q=quit]:"
	if _, expErr := c.ExpectString("Select wave"); expErr != nil {
		t.Fatalf("expected 'Select wave' prompt: %v", expErr)
	}
	if _, sendErr := c.SendLine("1"); sendErr != nil {
		t.Fatalf("failed to send '1' for wave selection: %v", sendErr)
	}

	// 2. Wait for approval prompt: "[a] Approve all  [s] Selective ..."
	if _, expErr := c.ExpectString("Approve all"); expErr != nil {
		t.Fatalf("expected 'Approve all' prompt: %v", expErr)
	}
	if _, sendErr := c.SendLine("a"); sendErr != nil {
		t.Fatalf("failed to send 'a' for approval: %v", sendErr)
	}

	// 3. After approval, the session runs apply then enters nextgen which
	//    may take time. Close the TTY to signal EOF so the session saves
	//    state and exits gracefully. Brief pause lets apply start.
	time.Sleep(1 * time.Second)
	c.Tty().Close()
	if _, eofErr := c.ExpectEOF(); eofErr != nil {
		t.Logf("ExpectEOF: %v", eofErr)
	}

	waitErr := cmd.Wait()
	if waitErr != nil {
		// Non-zero exit after TTY close is acceptable if the session
		// was interrupted mid-apply — the test goal is to verify the
		// interactive prompts work, not that the full pipeline completes.
		t.Logf("sightjack run exited: %v (expected after TTY close)", waitErr)
	}

	// --- Verification ---
	// The specification D-Mail should have been produced and delivered
	// to .expedition/inbox via phonewave routing.
	ws.WaitForDMailCount(t, ".expedition", "inbox", 1, 30*time.Second)
	ws.WaitForAbsent(t, ".siren", "outbox", 15*time.Second)
}
