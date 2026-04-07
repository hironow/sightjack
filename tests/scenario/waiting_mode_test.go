//go:build scenario

package scenario_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestScenario_WaitingModeSpecRescan verifies that sightjack's waiting mode
// correctly handles specification D-Mail arrival:
//
//  1. Initial scan completes and session enters waiting phase
//  2. Specification D-Mails injected into .siren/inbox trigger rescan
//  3. Multiple specs in the same batch are coalesced (no selection UI)
//  4. The default waiting path (--idle-timeout > 0) is exercised
//
// This test uses --idle-timeout 45s (positive) instead of the usual -1s,
// which exercises the production default path that all other scenario tests skip.
func TestScenario_WaitingModeSpecRescan(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	ws := NewWorkspace(t, "minimal")

	// Start sightjack in background with waiting mode enabled.
	// --idle-timeout 45s ensures the session enters the waiting cycle
	// instead of exiting immediately after wave processing.
	_, output := ws.StartSightjackAsync(t, ctx, "45s")

	// Phase 1: Wait for initial scan to complete and session to enter waiting.
	// The waiting phase logs "Waiting for D-Mail" when it blocks on the inbox.
	waitForLog(t, output, "Waiting for", 60*time.Second)
	t.Log("Phase 1: waiting mode entered")

	// Phase 2: Inject specification D-Mails into inbox.
	// Two specs: one unique, one duplicate name (to test name-based dedup).
	spec1 := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "spec-waiting-001",
		"kind":                 "specification",
		"description":          "Waiting mode spec rescan test 1",
	}, "# Spec 1\n\nFirst specification for waiting mode rescan.")

	spec2 := FormatDMail(map[string]string{
		"dmail-schema-version": "1",
		"name":                 "spec-waiting-002",
		"kind":                 "specification",
		"description":          "Waiting mode spec rescan test 2",
	}, "# Spec 2\n\nSecond specification for batch coalescing.")

	ws.InjectDMail(t, ".siren", "inbox", "spec-waiting-001.md", spec1)
	ws.InjectDMail(t, ".siren", "inbox", "spec-waiting-002.md", spec2)
	t.Log("Phase 2: specification D-Mails injected")

	// Phase 3: Wait for rescan trigger log.
	// The loop.go change logs "Specification D-Mail received (N) — triggering rescan"
	waitForLog(t, output, "Specification D-Mail received", 30*time.Second)
	t.Log("Phase 3: specification rescan triggered")

	// Phase 4: Verify auto-rescan was invoked.
	// session.go logs "Auto-rescan" when loopResultRescanNeeded is returned.
	waitForLog(t, output, "Auto-rescan", 30*time.Second)
	t.Log("Phase 4: auto-rescan completed")

	// Phase 5: Verify the same batch does NOT trigger a second rescan.
	// After the first auto-rescan, the session re-enters the waiting loop.
	// The consumed batch (Snapshot advanced) must not re-trigger.
	// Wait 3s and count "Auto-rescan" occurrences — must be exactly 1.
	time.Sleep(3 * time.Second)
	rescanCount := strings.Count(output.String(), "Auto-rescan")
	if rescanCount != 1 {
		t.Errorf("expected exactly 1 Auto-rescan, got %d — batch was not consumed", rescanCount)
	}
	t.Log("Phase 5: no duplicate rescan from same batch")

	// Phase 6: Re-inject the same spec names and verify idempotency.
	// The consumed spec names (spec-waiting-001, spec-waiting-002) must not
	// trigger a second rescan even when re-delivered in a new arrival.
	waitForLog(t, output, "Waiting for", 30*time.Second) // re-entered waiting phase
	ws.InjectDMail(t, ".siren", "inbox", "spec-waiting-001-dup.md", spec1) // same name in D-Mail, different filename
	time.Sleep(3 * time.Second)
	rescanCount2 := strings.Count(output.String(), "Auto-rescan")
	if rescanCount2 != 1 {
		t.Errorf("expected still 1 Auto-rescan after re-inject of consumed spec, got %d", rescanCount2)
	}
	t.Log("Phase 6: re-injected consumed spec did not trigger duplicate rescan")
}

// waitForLog polls the output buffer for a substring, with timeout.
func waitForLog(t *testing.T, buf interface{ String() string }, substr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), substr) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for log %q in output:\n%s", substr, buf.String())
}
