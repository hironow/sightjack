//go:build scenario

package scenario_test

import (
	"context"
	"testing"
	"time"
)

// TestScenario_FeedbackRescan verifies the closed loop:
// 1. Initial scan + wave generation → specification D-Mail
// 2. Inject design-feedback into inbox
// 3. Rescan triggered by feedback → new wave/specification
//
// This is the core value loop of sightjack: downstream feedback
// drives wave re-evaluation and new specification generation.
func TestScenario_FeedbackRescan(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "feedback-rescan")
	obs := NewObserver(ws, t)

	pw := ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)
	defer ws.DumpPhonewaveLog(t, pw)

	// Phase 1: Initial scan → specification
	err := ws.RunSightjackScan(t, ctx)
	if err != nil {
		t.Fatalf("initial sightjack scan failed: %v", err)
	}

	// Wait for spec delivery
	ws.WaitForDMailCount(t, ".expedition", "inbox", 1, 30*time.Second)
	ws.WaitForAbsent(t, ".siren", "outbox", 15*time.Second)

	// Verify initial spec was generated
	obs.AssertSpecificationDMailCount(1)

	// Phase 2: Inject design-feedback into sightjack inbox
	feedback := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "fb-arch-001",
		"kind":                 "design-feedback",
		"description":          "Architecture drift in auth module",
		"severity":             "high",
	}, "# Architecture Drift\n\nToken rotation not aligned with JWT spec.\nRescan recommended.")
	ws.InjectDMail(t, ".siren", "inbox", "fb-arch-001.md", feedback)

	// Phase 3: Run sightjack again — should incorporate feedback and rescan
	err = ws.RunSightjackScan(t, ctx)
	if err != nil {
		t.Logf("feedback-rescan sightjack: %v (may be acceptable)", err)
	}

	// Verify the feedback-rescan loop:
	// - feedback.received event should be emitted
	// - new specification D-Mails should be generated (>= 2 total in archive)
	obs.AssertFeedbackReceivedEvent()
	obs.AssertSpecificationDMailCount(2)

	// Final: all outboxes empty
	ws.WaitForAbsent(t, ".siren", "outbox", 15*time.Second)
	obs.AssertAllOutboxEmpty()
}
