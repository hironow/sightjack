//go:build scenario

package scenario_test

import (
	"context"
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

	// Inject feedback D-Mail (simulates amadeus response)
	feedback := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "feedback-retry-001",
		"kind":                 "feedback",
		"description":          "Retry feedback from amadeus",
	}, "# Feedback\n\n## Action: retry\n\nPlease rescan and update specifications.")
	ws.InjectDMail(t, ".siren", "inbox", "feedback-retry-001.md", feedback)

	// Second scan (processes feedback)
	err = ws.RunSightjackScan(t, ctx)
	if err != nil {
		t.Logf("second sightjack scan: %v", err)
	}

	// Verify final state
	obs.AssertAllOutboxEmpty()
}
