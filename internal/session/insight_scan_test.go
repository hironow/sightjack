package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func TestWriteShibitoInsights_CreatesFileWithCorrectEntries(t *testing.T) {
	// given
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	warnings := []domain.ShibitoWarning{
		{
			ClosedIssueID:  "PROJ-100",
			CurrentIssueID: "PROJ-200",
			Description:    "Auth pattern re-emerged",
			RiskLevel:      "high",
		},
		{
			ClosedIssueID:  "PROJ-101",
			CurrentIssueID: "PROJ-201",
			Description:    "Caching issue resurfaced",
			RiskLevel:      "medium",
		},
	}

	w := session.NewInsightWriter(insightsDir, runDir)

	// when
	session.WriteShibitoInsights(w, warnings, "session-abc", nil)

	// then
	file, err := w.Read("shibito.md")
	if err != nil {
		t.Fatalf("read shibito.md: %v", err)
	}

	if len(file.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(file.Entries))
	}

	e := file.Entries[0]
	if e.Title != "shibito-PROJ-100-PROJ-200" {
		t.Errorf("unexpected title: %q", e.Title)
	}
	if e.What != "Auth pattern re-emerged" {
		t.Errorf("unexpected what: %q", e.What)
	}
	if e.Why != "Issue PROJ-100 was closed but pattern re-emerged in PROJ-200" {
		t.Errorf("unexpected why: %q", e.Why)
	}
	if e.Who != "sightjack scan (session-abc)" {
		t.Errorf("unexpected who: %q", e.Who)
	}
	if e.Constraints != "Risk level: high" {
		t.Errorf("unexpected constraints: %q", e.Constraints)
	}

	if file.Kind != "shibito" {
		t.Errorf("expected kind 'shibito', got %q", file.Kind)
	}
	if file.Tool != "sightjack" {
		t.Errorf("expected tool 'sightjack', got %q", file.Tool)
	}
}

func TestWriteShibitoInsights_SkipsWhenEmpty(t *testing.T) {
	// given
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	w := session.NewInsightWriter(insightsDir, runDir)

	// when
	session.WriteShibitoInsights(w, nil, "session-abc", nil)

	// then: no file created
	_, err := os.Stat(filepath.Join(insightsDir, "shibito.md"))
	if err == nil {
		t.Fatal("expected no file when warnings are empty")
	}
}

func TestWriteShibitoInsights_IsIdempotent(t *testing.T) {
	// given
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	warnings := []domain.ShibitoWarning{
		{ClosedIssueID: "PROJ-100", CurrentIssueID: "PROJ-200", Description: "dup", RiskLevel: "low"},
	}
	w := session.NewInsightWriter(insightsDir, runDir)

	// when: write twice
	session.WriteShibitoInsights(w, warnings, "session-abc", nil)
	session.WriteShibitoInsights(w, warnings, "session-abc", nil)

	// then: still 1 entry
	file, err := w.Read("shibito.md")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(file.Entries) != 1 {
		t.Errorf("expected 1 entry (idempotent), got %d", len(file.Entries))
	}
}

func TestWriteStrictnessInsights_CreatesFileWithCorrectEntries(t *testing.T) {
	// given
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	clusters := []domain.ClusterScanResult{
		{
			Name:                "auth-service",
			EstimatedStrictness: "lockdown",
			StrictnessReasoning: "Critical auth path with many regressions",
			Completeness:        0.85,
		},
		{
			Name:                "docs",
			EstimatedStrictness: "fog",
			StrictnessReasoning: "Low-risk documentation cluster",
			Completeness:        0.60,
		},
	}

	w := session.NewInsightWriter(insightsDir, runDir)

	// when
	session.WriteStrictnessInsights(w, clusters, "session-xyz", nil)

	// then
	file, err := w.Read("strictness.md")
	if err != nil {
		t.Fatalf("read strictness.md: %v", err)
	}

	if len(file.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(file.Entries))
	}

	e := file.Entries[0]
	if e.Title != "strictness-auth-service" {
		t.Errorf("unexpected title: %q", e.Title)
	}
	if e.What != "Cluster auth-service estimated strictness: lockdown" {
		t.Errorf("unexpected what: %q", e.What)
	}
	if e.Why != "Critical auth path with many regressions" {
		t.Errorf("unexpected why: %q", e.Why)
	}
	if e.Who != "sightjack scan (session-xyz)" {
		t.Errorf("unexpected who: %q", e.Who)
	}
	if e.Constraints != "Estimated — may differ from manual override" {
		t.Errorf("unexpected constraints: %q", e.Constraints)
	}
	if e.Extra["cluster"] != "auth-service" {
		t.Errorf("unexpected extra cluster: %q", e.Extra["cluster"])
	}
	if e.Extra["completeness-delta"] != "85.0%" {
		t.Errorf("unexpected extra completeness-delta: %q", e.Extra["completeness-delta"])
	}

	if file.Kind != "strictness" {
		t.Errorf("expected kind 'strictness', got %q", file.Kind)
	}
	if file.Tool != "sightjack" {
		t.Errorf("expected tool 'sightjack', got %q", file.Tool)
	}
}

func TestWriteStrictnessInsights_SkipsClustersWithoutEstimate(t *testing.T) {
	// given
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	clusters := []domain.ClusterScanResult{
		{Name: "no-estimate", Completeness: 0.5},
		{Name: "has-estimate", EstimatedStrictness: "alert", StrictnessReasoning: "moderate risk", Completeness: 0.7},
	}

	w := session.NewInsightWriter(insightsDir, runDir)

	// when
	session.WriteStrictnessInsights(w, clusters, "session-xyz", nil)

	// then: only 1 entry (the one with an estimate)
	file, err := w.Read("strictness.md")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(file.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(file.Entries))
	}
	if file.Entries[0].Title != "strictness-has-estimate" {
		t.Errorf("wrong entry: %q", file.Entries[0].Title)
	}
}

func TestWriteStrictnessInsights_SkipsWhenNoClustersHaveEstimate(t *testing.T) {
	// given
	dir := t.TempDir()
	insightsDir := filepath.Join(dir, "insights")
	runDir := filepath.Join(dir, ".run")
	os.MkdirAll(insightsDir, 0o755)
	os.MkdirAll(runDir, 0o755)

	clusters := []domain.ClusterScanResult{
		{Name: "plain", Completeness: 0.5},
	}

	w := session.NewInsightWriter(insightsDir, runDir)

	// when
	session.WriteStrictnessInsights(w, clusters, "session-xyz", nil)

	// then: no file
	_, err := os.Stat(filepath.Join(insightsDir, "strictness.md"))
	if err == nil {
		t.Fatal("expected no file when no clusters have strictness estimate")
	}
}
