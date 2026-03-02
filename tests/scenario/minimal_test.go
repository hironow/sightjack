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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "minimal")
	obs := NewObserver(ws, t)

	// Start phonewave daemon
	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// Run sightjack scan with --auto-approve
	err := ws.RunSightjackScan(t, ctx)
	if err != nil {
		t.Fatalf("sightjack scan failed: %v", err)
	}

	// Wait for specification D-Mail delivery: .siren/outbox -> phonewave -> .expedition/inbox
	specPath := ws.WaitForDMail(t, ".expedition", "inbox", 30*time.Second)

	// Verify outbox is cleaned up
	ws.WaitForAbsent(t, ".siren", "outbox", 10*time.Second)

	// Verify specification kind
	obs.AssertDMailKind(specPath, "specification")

	obs.AssertAllOutboxEmpty()
}
