//go:build scenario

package scenario_test

import (
	"context"
	"testing"
	"time"
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
