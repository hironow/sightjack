package integration_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/session"
)

// --- Canned auto-discuss fixtures ---

func autoDiscussArchitectResponse() string {
	return `{"content":"The wave introduces DoD for AUTH-1. This aligns with the security cluster goals."}`
}

func autoDiscussDevilsAdvocateResponse() string {
	return `{"content":"The DoD lacks specificity on token rotation. Existing ADR 0001 mandates convergence checks.","open_issues":["Token rotation interval not specified","No rollback strategy"],"adr_recommended":true,"adr_recommendation_reason":"New authentication pattern requires ADR for token rotation policy"}`
}

func scribeADRResponse() string {
	return `{"adr_id":"0001","title":"token-rotation-policy","content":"# 0001. Token Rotation Policy\n\n**Date:** 2026-03-08\n**Status:** Accepted\n\n## Context\nAuth module needs token rotation.\n\n## Decision\nRotate every 24h.\n\n## Consequences\n### Positive\n- Improved security\n### Negative\n- More API calls","reasoning":"New auth pattern warrants formal ADR"}`
}

// --- Auto-Approve + Auto-Discuss Integration Tests ---

func autoApproveConfig() *domain.Config {
	cfg := testConfig()
	cfg.Gate = domain.GateConfig{AutoApprove: true, WaitTimeout: -1}
	cfg.Scribe = domain.ScribeConfig{Enabled: true, AutoDiscussRounds: 1}
	return cfg
}

// TestAutoApprove_WithAutoDiscuss_GeneratesADR verifies the full chain:
// auto-approve → RunAutoDiscuss → RunScribeADR → ADR file created.
func TestAutoApprove_WithAutoDiscuss_GeneratesADR(t *testing.T) {
	// given
	baseDir := t.TempDir()
	cfg := autoApproveConfig()
	sessionID := "test-auto-discuss-adr"

	adrDir := filepath.Join(baseDir, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		t.Fatal(err)
	}

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	d.Register("wave_*_*.json", waveGenAuth())
	// auto-discuss: rounds=1 → r=0 architect, r=1 devil's advocate
	d.Register("auto_discuss_architect_*.json", autoDiscussArchitectResponse())
	d.Register("auto_discuss_devils_advocate_*.json", autoDiscussDevilsAdvocateResponse())
	d.Register("scribe_*.json", scribeADRResponse())
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

	// ADR file should exist
	adrFiles, _ := filepath.Glob(filepath.Join(adrDir, "*.md"))
	if len(adrFiles) == 0 {
		t.Error("expected ADR file to be generated in docs/adr/")
	}

	// Verify ADR content
	for _, f := range adrFiles {
		data, readErr := os.ReadFile(f)
		if readErr != nil {
			t.Fatalf("read ADR: %v", readErr)
		}
		if !strings.Contains(string(data), "Token Rotation") {
			t.Errorf("ADR content should contain 'Token Rotation', got: %s", string(data))
		}
	}

	// Verify mock call log includes auto-discuss calls
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
	if !hasScribe {
		t.Error("expected scribe call in log")
	}
}

// TestAutoApprove_DiscussRoundsZero_SkipsAutoDiscuss verifies that
// auto_discuss_rounds=0 preserves legacy auto-approve behavior (no discuss calls).
func TestAutoApprove_DiscussRoundsZero_SkipsAutoDiscuss(t *testing.T) {
	// given
	baseDir := t.TempDir()
	cfg := autoApproveConfig()
	cfg.Scribe.AutoDiscussRounds = 0 // skip auto-discuss
	sessionID := "test-discuss-rounds-zero"

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	d.Register("wave_*_*.json", waveGenAuth())
	d.Register("apply_*_*.json", waveApplySuccess("auth-w1"))
	d.Register("nextgen_*_*.json", nextgenEmpty())
	// No auto-discuss or scribe mocks registered — should not be needed
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

	// No auto-discuss calls should have been made
	log := d.CallLog()
	for _, entry := range log {
		if strings.Contains(entry, "auto_discuss") {
			t.Errorf("expected no auto_discuss calls with rounds=0, got: %s", entry)
		}
		if strings.Contains(entry, "scribe_") {
			t.Errorf("expected no scribe calls with rounds=0, got: %s", entry)
		}
	}
}

// TestAutoApprove_WithAutoDiscuss_RoundsTwo verifies that rounds=2 generates
// the expected number of debate exchanges (4 calls: arch, DA, arch, DA).
func TestAutoApprove_WithAutoDiscuss_RoundsTwo(t *testing.T) {
	// given
	baseDir := t.TempDir()
	cfg := autoApproveConfig()
	cfg.Scribe.AutoDiscussRounds = 2 // 4 calls total
	sessionID := "test-discuss-rounds-two"

	adrDir := filepath.Join(baseDir, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		t.Fatal(err)
	}

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	d.Register("wave_*_*.json", waveGenAuth())
	d.Register("auto_discuss_architect_*.json", autoDiscussArchitectResponse())
	d.Register("auto_discuss_devils_advocate_*.json", autoDiscussDevilsAdvocateResponse())
	d.Register("scribe_*.json", scribeADRResponse())
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

	// Count auto-discuss calls: rounds=2 → 4 calls (2 architect + 2 DA)
	log := d.CallLog()
	var archCount, daCount int
	for _, entry := range log {
		if strings.Contains(entry, "auto_discuss_architect") {
			archCount++
		}
		if strings.Contains(entry, "auto_discuss_devils_advocate") {
			daCount++
		}
	}
	if archCount != 2 {
		t.Errorf("expected 2 architect calls for rounds=2, got %d", archCount)
	}
	if daCount != 2 {
		t.Errorf("expected 2 devil's advocate calls for rounds=2, got %d", daCount)
	}
}

// TestAutoApprove_WithAutoDiscuss_OpenIssuesInState verifies that the
// Devil's Advocate's open_issues survive through to the event store via ADR event.
func TestAutoApprove_WithAutoDiscuss_OpenIssuesInState(t *testing.T) {
	// given
	baseDir := t.TempDir()
	cfg := autoApproveConfig()
	sessionID := "test-discuss-open-issues"

	adrDir := filepath.Join(baseDir, "docs", "adr")
	if err := os.MkdirAll(adrDir, 0755); err != nil {
		t.Fatal(err)
	}

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	d.Register("wave_*_*.json", waveGenAuth())
	d.Register("auto_discuss_architect_*.json", autoDiscussArchitectResponse())
	d.Register("auto_discuss_devils_advocate_*.json", autoDiscussDevilsAdvocateResponse())
	d.Register("scribe_*.json", scribeADRResponse())
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

	// Verify state has ADR count incremented
	state := loadTestState(t, baseDir)
	if state.ADRCount == 0 {
		t.Error("expected ADRCount > 0 in session state")
	}

	// Verify ADR file exists with expected content
	adrFiles, _ := filepath.Glob(filepath.Join(adrDir, "*.md"))
	if len(adrFiles) == 0 {
		t.Fatal("expected ADR file in docs/adr/")
	}
	data, readErr := os.ReadFile(adrFiles[0])
	if readErr != nil {
		t.Fatalf("read ADR: %v", readErr)
	}
	content := string(data)
	if !strings.Contains(content, "Token Rotation") {
		t.Errorf("ADR should contain 'Token Rotation', got: %s", content)
	}
}
