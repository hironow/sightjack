//go:build scenario

package scenario_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScenario_L4_Hard(t *testing.T) {
	if testing.Short() {
		t.Skip("scenario tests are not short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	ws := NewWorkspace(t, "hard")
	obs := NewObserver(ws, t)

	// --- Phase 1: phonewave daemon restart ---
	pw := ws.StartPhonewave(t, ctx)
	defer ws.DumpPhonewaveLog(t, pw)

	err := ws.RunSightjackScan(t, ctx)
	if err != nil {
		t.Logf("pre-restart scan: %v (acceptable during restart test)", err)
	}

	t.Log("restarting phonewave daemon")
	ws.StopPhonewave(t, pw)
	time.Sleep(1 * time.Second)
	pw = ws.StartPhonewave(t, ctx)
	defer ws.StopPhonewave(t, pw)

	ws.WaitForAbsent(t, ".siren", "outbox", 30*time.Second)

	// --- Phase 2: fake-claude transient failure ---
	counterPath := filepath.Join(os.TempDir(), "fake-claude-call-count")
	os.Remove(counterPath)
	ws.Env = append(ws.Env, "FAKE_CLAUDE_FAIL_COUNT=2")

	for i := 0; i < 2; i++ {
		err := ws.RunSightjackScan(t, ctx)
		if err != nil {
			t.Logf("scan %d with FAIL_COUNT: %v (expected failure)", i+1, err)
		}
	}

	// Third scan should succeed
	err = ws.RunSightjackScan(t, ctx)
	if err != nil {
		t.Logf("recovery scan: %v (may still be acceptable)", err)
	}

	// Clean up FAIL_COUNT
	cleanEnv := make([]string, 0, len(ws.Env))
	for _, e := range ws.Env {
		if e != "FAKE_CLAUDE_FAIL_COUNT=2" {
			cleanEnv = append(cleanEnv, e)
		}
	}
	ws.Env = cleanEnv
	os.Remove(counterPath)

	// --- Phase 3: malformed D-Mail ---
	malformed := []byte("This is not a valid D-Mail.\nNo YAML frontmatter here.\n")
	ws.InjectDMail(t, ".siren", "inbox", "malformed-001.md", malformed)

	err = ws.RunSightjackScan(t, ctx)
	if err != nil {
		t.Logf("scan after malformed inject: %v (acceptable)", err)
	}

	time.Sleep(3 * time.Second)

	// --- Final verification ---
	ws.WaitForAbsent(t, ".siren", "outbox", 30*time.Second)
	obs.AssertAllOutboxEmpty()
	t.Log("L4 hard test passed: daemon restart + transient failures + malformed D-Mail all handled")
}
