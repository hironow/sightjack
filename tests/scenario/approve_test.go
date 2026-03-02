//go:build scenario

package scenario_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScenario_ApproveCmdPath(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "minimal")
	obs := NewObserver(ws, t)

	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// Create approve script (exit 0 = approve all)
	approveScript := filepath.Join(ws.Root, "approve.sh")
	if err := os.WriteFile(approveScript, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write approve script: %v", err)
	}

	// Create notify script that logs invocations for verification
	notifyLog := filepath.Join(ws.Root, "notify.log")
	notifyScript := filepath.Join(ws.Root, "notify.sh")
	notifyContent := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %s\n", notifyLog)
	if err := os.WriteFile(notifyScript, []byte(notifyContent), 0o755); err != nil {
		t.Fatalf("write notify script: %v", err)
	}

	// Inject a convergence D-Mail so sightjack's convergence gate fires the notifier.
	// Without convergence in inbox, --notify-cmd is never invoked (gate only fires on convergence).
	convergence := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "convergence-approve-test",
		"kind":                 "convergence",
		"description":          "Convergence for approve-cmd test",
	}, "# Convergence\n\nAll tools stabilized.")
	ws.InjectDMail(t, ".siren", "inbox", "convergence-approve-test.md", convergence)

	// Run sightjack with --approve-cmd and --notify-cmd flags.
	// --auto-approve is required for non-interactive wave selection/approval;
	// --approve-cmd configures the convergence gate approver;
	// --notify-cmd configures the notifier (fires on convergence signals).
	err := ws.RunSightjack(t, ctx, "run",
		"--auto-approve",
		"--approve-cmd", approveScript,
		"--notify-cmd", notifyScript,
		ws.RepoPath,
	)
	if err != nil {
		t.Fatalf("sightjack run with approve-cmd failed: %v", err)
	}

	// Verify specification was produced and delivered
	specPath := ws.WaitForDMail(t, ".expedition", "inbox", 30*time.Second)
	obs.AssertDMailKind(specPath, "specification")

	// Verify outbox was flushed
	ws.WaitForAbsent(t, ".siren", "outbox", 10*time.Second)

	// Verify notify script was invoked (convergence gate fires the notifier)
	data, err := os.ReadFile(notifyLog)
	if err != nil {
		t.Fatalf("notify.log not found — notify-cmd was not invoked: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("notify.log exists but is empty — notify-cmd produced no output")
	}
	t.Logf("notify.log content:\n%s", string(data))
}
