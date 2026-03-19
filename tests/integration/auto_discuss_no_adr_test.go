package integration_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/session"
)

// --- Fixture: Devil's Advocate says "no ADR needed" ---

// autoDiscussNoADRResponse returns a Devil's Advocate response where
// adr_recommended is false and open_issues is empty.
// This simulates the DA concluding no architectural decision record is warranted.
func autoDiscussNoADRResponse() string {
	return `{"content":"The wave is straightforward. No architectural concerns identified.","open_issues":[],"adr_recommended":false,"adr_recommendation_reason":""}`
}

// scribeADRNoADRResponse returns a minimal scribe response for the no-ADR path.
// Even though adr_recommended was false, the current implementation calls RunScribeADR
// unconditionally, so the scribe mock must be registered.
func scribeADRNoADRResponse() string {
	return `{"adr_id":"0002","title":"no-op-review","content":"# 0002. No-Op Review\n\n**Date:** 2026-03-20\n**Status:** Accepted\n\n## Context\nNo architectural concerns.\n\n## Decision\nNo changes needed.\n\n## Consequences\n### Positive\n- Confirmed design\n### Negative\n- None","reasoning":"Auto-generated despite no ADR recommendation"}`
}

// TestAutoDiscuss_NoADRRecommended_ScribeStillCalled verifies that when the
// Devil's Advocate returns adr_recommended=false with empty open_issues,
// RunScribeADR is still invoked (current unconditional behavior in phases.go).
// This documents the existing behavior: the adr_recommended field is NOT used
// as a gate for scribe invocation.
func TestAutoDiscuss_NoADRRecommended_ScribeStillCalled(t *testing.T) {
	// given
	baseDir := t.TempDir()
	cfg := autoApproveConfig()
	sessionID := "test-discuss-no-adr"

	adrDir := filepath.Join(baseDir, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		t.Fatal(err)
	}

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	d.Register("wave_*_*.json", waveGenAuth())
	d.Register("auto_discuss_architect_*.json", autoDiscussArchitectResponse())
	d.Register("auto_discuss_devils_advocate_*.json", autoDiscussNoADRResponse())
	d.Register("scribe_*.json", scribeADRNoADRResponse())
	d.Register("apply_*_*.json", waveApplySuccess("auth-w1"))
	d.Register("nextgen_*_*.json", nextgenEmpty())
	cleanup := d.Install()
	defer cleanup()

	ctx := context.Background()

	// when
	err := session.RunSession(ctx, cfg, baseDir, sessionID, false,
		strings.NewReader(""), io.Discard,
		testEmitter(baseDir, sessionID),
		platform.NewLogger(io.Discard, false))

	// then
	if err != nil {
		t.Fatalf("RunSession: %v", err)
	}

	// Verify mock call log: architect, DA, AND scribe should all be called
	log := d.CallLog()
	var hasArchitect, hasDA, hasScribe bool
	for _, entry := range log {
		if strings.Contains(entry, "auto_discuss_architect") {
			hasArchitect = true
		}
		if strings.Contains(entry, "auto_discuss_devils_advocate") {
			hasDA = true
		}
		if strings.Contains(entry, "scribe_") {
			hasScribe = true
		}
	}
	if !hasArchitect {
		t.Error("expected auto_discuss_architect call in log")
	}
	if !hasDA {
		t.Error("expected auto_discuss_devils_advocate call in log")
	}
	// KEY ASSERTION: scribe is called even when adr_recommended=false.
	// This documents the current behavior where RunScribeADR is invoked
	// unconditionally after a successful RunAutoDiscuss (phases.go line ~142).
	if !hasScribe {
		t.Error("expected scribe call even with adr_recommended=false (current unconditional behavior)")
	}

	// ADR file should still be generated (scribe was called)
	adrFiles, _ := filepath.Glob(filepath.Join(adrDir, "*.md"))
	if len(adrFiles) == 0 {
		t.Error("expected ADR file to be generated even with adr_recommended=false")
	}

	// Verify ADR count in state is incremented
	state := loadTestState(t, baseDir)
	if state.ADRCount == 0 {
		t.Error("expected ADRCount > 0 in session state (scribe ran despite adr_recommended=false)")
	}
}
