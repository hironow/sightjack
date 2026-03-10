//go:build scenario

package scenario_test

import (
	"context"
	"testing"
	"time"
)

func TestScenario_L1_Minimal(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "minimal")
	obs := NewObserver(ws, t)

	// Start phonewave daemon
	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// 1. Run sightjack → specification in .siren/outbox → phonewave → .expedition/inbox
	err := ws.RunSightjackScan(t, ctx)
	if err != nil {
		t.Fatalf("sightjack scan failed: %v", err)
	}
	specPath := ws.WaitForDMail(t, ".expedition", "inbox", 30*time.Second)
	ws.WaitForAbsent(t, ".siren", "outbox", 10*time.Second)
	obs.AssertDMailKind(specPath, "specification")

	// 2. Run paintress → report in .expedition/outbox → phonewave → .gate/inbox
	err = ws.RunPaintressExpedition(t, ctx)
	if err != nil {
		t.Fatalf("paintress expedition failed: %v", err)
	}
	reportPath := ws.WaitForDMail(t, ".gate", "inbox", 30*time.Second)
	ws.WaitForAbsent(t, ".expedition", "outbox", 10*time.Second)
	obs.AssertDMailKind(reportPath, "report")

	// 3. Start amadeus run as daemon → feedback in .gate/outbox → phonewave → .expedition/inbox
	am := ws.StartAmadeusRun(t, ctx)
	defer ws.StopAmadeusRun(t, am)

	feedbackPath := ws.WaitForDMail(t, ".expedition", "inbox", 30*time.Second)
	obs.AssertDMailKind(feedbackPath, "implementation-feedback")

	// 4. Full closed loop verified — all 3 phases completed sequentially above.
	obs.AssertAllOutboxEmpty()
}
