//go:build scenario

package scenario_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	expect "github.com/Netflix/go-expect"
)

// TestScenario_ApproveCmdPath verifies human-on-the-loop hooks in two modes:
//
//   - auto_approve: --auto-approve handles everything; --notify-cmd fires on convergence
//   - approve_cmd:  --approve-cmd (no --auto-approve) handles convergence gate via CmdApprover;
//     go-expect drives interactive wave selection/approval
func TestScenario_ApproveCmdPath(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}

	t.Run("auto_approve", testApproveCmdAutoApprove)
	t.Run("approve_cmd", testApproveCmdPure)
}

// testApproveCmdAutoApprove verifies --auto-approve + --notify-cmd (non-interactive).
// Does NOT use --approve-cmd because AutoApprover takes priority in BuildApprover(),
// making CmdApprover dead code when both flags are set.
func testApproveCmdAutoApprove(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "minimal")
	obs := NewObserver(ws, t)

	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// Create notify script
	notifyLog := filepath.Join(ws.Root, "notify.log")
	notifyScript := filepath.Join(ws.Root, "notify.sh")
	notifyContent := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %s\n", notifyLog)
	if err := os.WriteFile(notifyScript, []byte(notifyContent), 0o755); err != nil {
		t.Fatalf("write notify script: %v", err)
	}

	// Inject convergence D-Mail so convergence gate fires the notifier.
	convergence := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "convergence-auto-approve",
		"kind":                 "convergence",
		"description":          "Convergence for auto-approve test",
	}, "# Convergence\n\nAll tools stabilized.")
	ws.InjectDMail(t, ".siren", "inbox", "convergence-auto-approve.md", convergence)

	// Run sightjack with --auto-approve + --notify-cmd only.
	err := ws.RunSightjack(t, ctx, "run",
		"--auto-approve",
		"--notify-cmd", notifyScript,
		ws.RepoPath,
	)
	if err != nil {
		t.Fatalf("sightjack run failed: %v", err)
	}

	// Verify specification produced
	specPath := ws.WaitForDMail(t, ".expedition", "inbox", 30*time.Second)
	obs.AssertDMailKind(specPath, "specification")
	ws.WaitForAbsent(t, ".siren", "outbox", 10*time.Second)

	// Verify notify-cmd was invoked
	data, err := os.ReadFile(notifyLog)
	if err != nil {
		t.Fatalf("notify.log not found — notify-cmd was not invoked: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("notify.log exists but is empty — notify-cmd produced no output")
	}
	t.Logf("notify.log content:\n%s", string(data))
}

// testApproveCmdPure verifies --approve-cmd + --notify-cmd WITHOUT --auto-approve.
// This ensures CmdApprover actually handles the convergence gate (not AutoApprover).
// Uses go-expect for interactive wave selection ("1") and approval ("a").
func testApproveCmdPure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "minimal")

	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// Create approve script that logs invocations (exit 0 = approve)
	approveLog := filepath.Join(ws.Root, "approve.log")
	approveScript := filepath.Join(ws.Root, "approve.sh")
	approveContent := fmt.Sprintf("#!/bin/sh\necho \"approve: $@\" >> %s\nexit 0\n", approveLog)
	if err := os.WriteFile(approveScript, []byte(approveContent), 0o755); err != nil {
		t.Fatalf("write approve script: %v", err)
	}

	// Create notify script that logs invocations
	notifyLog := filepath.Join(ws.Root, "notify.log")
	notifyScript := filepath.Join(ws.Root, "notify.sh")
	notifyContent := fmt.Sprintf("#!/bin/sh\necho \"notify: $@\" >> %s\n", notifyLog)
	if err := os.WriteFile(notifyScript, []byte(notifyContent), 0o755); err != nil {
		t.Fatalf("write notify script: %v", err)
	}

	// Inject convergence D-Mail so convergence gate fires CmdApprover.
	convergence := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "convergence-cmd-test",
		"kind":                 "convergence",
		"description":          "Convergence for approve-cmd purity test",
	}, "# Convergence\n\nAll tools stabilized.")
	ws.InjectDMail(t, ".siren", "inbox", "convergence-cmd-test.md", convergence)

	// Create go-expect PTY console for interactive wave selection/approval.
	c, err := expect.NewConsole(expect.WithDefaultTimeout(60 * time.Second))
	if err != nil {
		t.Fatalf("create console: %v", err)
	}
	defer c.Close()

	// Start sightjack WITHOUT --auto-approve.
	// --approve-cmd handles the convergence gate; go-expect handles wave prompts.
	cmd := exec.CommandContext(ctx, "sightjack", "run",
		"--approve-cmd", approveScript,
		"--notify-cmd", notifyScript,
		ws.RepoPath,
	)
	cmd.Dir = ws.RepoPath
	cmd.Env = append(os.Environ(), ws.Env...)
	cmd.Stdin = c.Tty()
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()

	if startErr := cmd.Start(); startErr != nil {
		t.Fatalf("start sightjack run: %v", startErr)
	}

	// --- Interactive sequence (wave selection + approval) ---

	// 1. Wait for wave selection prompt
	if _, expErr := c.ExpectString("Select wave"); expErr != nil {
		t.Fatalf("expected 'Select wave' prompt: %v", expErr)
	}
	if _, sendErr := c.SendLine("1"); sendErr != nil {
		t.Fatalf("failed to send '1' for wave selection: %v", sendErr)
	}

	// 2. Wait for wave approval prompt
	if _, expErr := c.ExpectString("Approve all"); expErr != nil {
		t.Fatalf("expected 'Approve all' prompt: %v", expErr)
	}
	if _, sendErr := c.SendLine("a"); sendErr != nil {
		t.Fatalf("failed to send 'a' for approval: %v", sendErr)
	}

	// 3. Close TTY to signal EOF so the session exits gracefully.
	time.Sleep(1 * time.Second)
	c.Tty().Close()
	if _, eofErr := c.ExpectEOF(); eofErr != nil {
		t.Logf("ExpectEOF: %v", eofErr)
	}

	waitErr := cmd.Wait()
	if waitErr != nil {
		t.Logf("sightjack run exited: %v (expected after TTY close)", waitErr)
	}

	// --- Verification ---

	// Verify specification was produced and delivered
	ws.WaitForDMailCount(t, ".expedition", "inbox", 1, 30*time.Second)
	ws.WaitForAbsent(t, ".siren", "outbox", 15*time.Second)

	// Verify approve-cmd was actually invoked (CmdApprover, not AutoApprover)
	approveData, approveErr := os.ReadFile(approveLog)
	if approveErr != nil {
		t.Fatalf("approve.log not found — approve-cmd was not invoked: %v", approveErr)
	}
	if len(approveData) == 0 {
		t.Fatal("approve.log exists but is empty — approve-cmd produced no output")
	}
	t.Logf("approve.log content:\n%s", string(approveData))

	// Verify notify-cmd was invoked
	notifyData, notifyErr := os.ReadFile(notifyLog)
	if notifyErr != nil {
		t.Fatalf("notify.log not found — notify-cmd was not invoked: %v", notifyErr)
	}
	if len(notifyData) == 0 {
		t.Fatal("notify.log exists but is empty — notify-cmd produced no output")
	}
	t.Logf("notify.log content:\n%s", string(notifyData))
}
