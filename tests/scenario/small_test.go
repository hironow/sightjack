//go:build scenario

package scenario_test

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestScenario_L2_Small(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "small")
	obs := NewObserver(ws, t)

	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// First scan
	err := ws.RunSightjackScan(t, ctx)
	if err != nil {
		t.Logf("first sightjack scan: %v", err)
	}

	// Wait for specification delivery
	ws.WaitForDMailCount(t, ".expedition", "inbox", 1, 30*time.Second)
	ws.WaitForAbsent(t, ".siren", "outbox", 15*time.Second)

	// Paintress processes specification → report delivered to .gate/inbox
	err = ws.RunPaintressExpedition(t, ctx)
	if err != nil {
		t.Fatalf("paintress expedition failed: %v", err)
	}
	ws.WaitForDMailCount(t, ".gate", "inbox", 1, 30*time.Second)

	// Amadeus processes report → feedback delivered to .siren/inbox
	err = ws.RunAmadeusCheck(t, ctx)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			t.Logf("amadeus check returned exit code 2 (drift detected) — expected")
		} else {
			t.Fatalf("amadeus check failed: %v", err)
		}
	}
	ws.WaitForDMailCount(t, ".siren", "inbox", 1, 30*time.Second)

	// Second scan (processes real feedback from amadeus)
	err = ws.RunSightjackScan(t, ctx)
	if err != nil {
		t.Logf("second sightjack scan: %v", err)
	}

	// Verify final state
	obs.AssertAllOutboxEmpty()
}
