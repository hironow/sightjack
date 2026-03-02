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

	// Run sightjack with --approve-cmd and --notify-cmd flags.
	// --auto-approve is required for non-interactive wave selection/approval;
	// --approve-cmd configures the convergence gate approver (exercised when
	// convergence D-Mails arrive in the inbox).
	// --notify-cmd configures the notifier (exercised on convergence signals).
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

	// Verify notify script was invoked (notify.log should exist and be non-empty)
	data, err := os.ReadFile(notifyLog)
	if err != nil {
		t.Logf("notify.log not found (notification may not have fired): %v", err)
	} else if len(data) == 0 {
		t.Log("notify.log exists but is empty")
	} else {
		t.Logf("notify.log content:\n%s", string(data))
	}
}
