package sightjack

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNextgenFileName(t *testing.T) {
	wave := Wave{ClusterName: "Auth", ID: "auth-w2"}
	got := nextgenFileName(wave)
	want := "nextgen_auth_auth-w2.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestClearNextgenOutput_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	wave := Wave{ClusterName: "Auth", ID: "auth-w1"}
	path := filepath.Join(dir, nextgenFileName(wave))
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	clearNextgenOutput(dir, wave)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should have been removed")
	}
}

func TestClearNextgenOutput_NoopIfMissing(t *testing.T) {
	dir := t.TempDir()
	wave := Wave{ClusterName: "Auth", ID: "auth-w1"}
	clearNextgenOutput(dir, wave) // should not panic
}

func TestParseNextGenResult_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nextgen.json")
	data := `{"cluster_name":"Auth","waves":[{"id":"auth-w3","cluster_name":"Auth","title":"Security pass","description":"desc","actions":[],"prerequisites":["auth-w2"],"delta":{"before":0.65,"after":0.80},"status":"available"}],"reasoning":"needed"}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := ParseNextGenResult(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.ClusterName != "Auth" {
		t.Errorf("cluster_name: got %q", result.ClusterName)
	}
	if len(result.Waves) != 1 {
		t.Fatalf("waves: got %d, want 1", len(result.Waves))
	}
	if result.Waves[0].ID != "auth-w3" {
		t.Errorf("wave id: got %q", result.Waves[0].ID)
	}
}

func TestParseNextGenResult_EmptyWaves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nextgen.json")
	data := `{"cluster_name":"Auth","waves":[],"reasoning":"cluster complete"}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := ParseNextGenResult(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Waves) != 0 {
		t.Errorf("expected 0 waves, got %d", len(result.Waves))
	}
}

func TestParseNextGenResult_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nextgen.json")
	if err := os.WriteFile(path, []byte("{bad json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseNextGenResult(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse nextgen result") {
		t.Errorf("error should contain 'parse nextgen result': %v", err)
	}
}

func TestParseNextGenResult_MissingFile(t *testing.T) {
	_, err := ParseNextGenResult("/nonexistent/file.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestBuildNextGenPrompt_WithDoDTemplates(t *testing.T) {
	// given: config with DoD templates matching cluster
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "scans")
	os.MkdirAll(scanDir, 0755)

	cfg := DefaultConfig()
	cfg.DoDTemplates = map[string]DoDTemplate{
		"Auth": {Must: []string{"Unit tests required"}, Should: []string{"Integration tests"}},
	}
	wave := Wave{ClusterName: "Auth", ID: "auth-w1"}
	cluster := ClusterScanResult{
		Name:         "Auth",
		Completeness: 0.65,
		Issues:       []IssueDetail{{ID: "ENG-101", Identifier: "ENG-101", Title: "Auth issue", Completeness: 0.5}},
	}

	// when
	prompt, err := buildNextGenPrompt(&cfg, scanDir, wave, cluster, nil, nil, nil, "fog", nil)

	// then
	if err != nil {
		t.Fatalf("buildNextGenPrompt: %v", err)
	}
	if !strings.Contains(prompt, "Unit tests required") {
		t.Error("expected DoD Must item in prompt")
	}
	if !strings.Contains(prompt, "Integration tests") {
		t.Error("expected DoD Should item in prompt")
	}
}

func TestBuildNextGenPrompt_WithRejectedActions(t *testing.T) {
	// given: rejected actions from a previous wave
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "scans")
	os.MkdirAll(scanDir, 0755)

	cfg := DefaultConfig()
	wave := Wave{ClusterName: "Auth", ID: "auth-w1"}
	cluster := ClusterScanResult{
		Name:         "Auth",
		Completeness: 0.65,
		Issues:       []IssueDetail{{ID: "ENG-101", Identifier: "ENG-101", Title: "Auth", Completeness: 0.5}},
	}
	rejected := []WaveAction{
		{Type: "add_dod", IssueID: "ENG-101", Description: "Rejected DoD"},
	}

	// when
	prompt, err := buildNextGenPrompt(&cfg, scanDir, wave, cluster, nil, nil, rejected, "fog", nil)

	// then
	if err != nil {
		t.Fatalf("buildNextGenPrompt: %v", err)
	}
	if !strings.Contains(prompt, "Rejected DoD") {
		t.Error("expected rejected action description in prompt")
	}
	if !strings.Contains(prompt, "ENG-101") {
		t.Error("expected rejected action issue ID in prompt")
	}
}

func TestBuildNextGenPrompt_NilOptionals(t *testing.T) {
	// given: all optional fields are nil/empty
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "scans")
	os.MkdirAll(scanDir, 0755)

	cfg := DefaultConfig()
	wave := Wave{ClusterName: "Auth", ID: "auth-w1"}
	cluster := ClusterScanResult{
		Name:         "Auth",
		Completeness: 0.5,
		Issues:       []IssueDetail{{ID: "ENG-100", Identifier: "ENG-100", Title: "Issue", Completeness: 0.5}},
	}

	// when: nil DoD, nil ADRs, nil rejected, nil completedWaves
	prompt, err := buildNextGenPrompt(&cfg, scanDir, wave, cluster, nil, nil, nil, "fog", nil)

	// then: should not panic and should produce valid prompt
	if err != nil {
		t.Fatalf("buildNextGenPrompt: %v", err)
	}
	if !strings.Contains(prompt, "Auth") {
		t.Error("expected cluster name in prompt")
	}
	if !strings.Contains(prompt, "50") {
		t.Error("expected completeness percentage in prompt")
	}
}

func TestBuildNextGenPrompt_WithExistingADRs(t *testing.T) {
	// given: existing ADRs
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "scans")
	os.MkdirAll(scanDir, 0755)

	cfg := DefaultConfig()
	wave := Wave{ClusterName: "Auth", ID: "auth-w1"}
	cluster := ClusterScanResult{
		Name:         "Auth",
		Completeness: 0.65,
		Issues:       []IssueDetail{{ID: "ENG-101", Identifier: "ENG-101", Title: "Auth", Completeness: 0.5}},
	}
	adrs := []ExistingADR{
		{Filename: "0001-use-jwt.md", Content: "We chose JWT for auth tokens."},
	}

	// when
	prompt, err := buildNextGenPrompt(&cfg, scanDir, wave, cluster, nil, adrs, nil, "fog", nil)

	// then
	if err != nil {
		t.Fatalf("buildNextGenPrompt: %v", err)
	}
	if !strings.Contains(prompt, "0001-use-jwt.md") {
		t.Error("expected ADR filename in prompt")
	}
	if !strings.Contains(prompt, "JWT for auth tokens") {
		t.Error("expected ADR content in prompt")
	}
}

func TestBuildNextGenPrompt_WithFeedback(t *testing.T) {
	// given: feedback d-mails
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "scans")
	os.MkdirAll(scanDir, 0755)

	cfg := DefaultConfig()
	wave := Wave{ClusterName: "Auth", ID: "auth-w1"}
	cluster := ClusterScanResult{
		Name:         "Auth",
		Completeness: 0.65,
		Issues:       []IssueDetail{{ID: "ENG-101", Identifier: "ENG-101", Title: "Auth", Completeness: 0.5}},
	}
	feedback := []*DMail{
		{Name: "fb-arch-001", Kind: DMailFeedback, Description: "Architecture drift in auth module", Severity: "high", Body: "Token rotation not aligned with JWT spec."},
	}

	// when
	prompt, err := buildNextGenPrompt(&cfg, scanDir, wave, cluster, nil, nil, nil, "fog", feedback)

	// then
	if err != nil {
		t.Fatalf("buildNextGenPrompt: %v", err)
	}
	if !strings.Contains(prompt, "fb-arch-001") {
		t.Error("expected feedback name in prompt")
	}
	if !strings.Contains(prompt, "Architecture drift in auth module") {
		t.Error("expected feedback description in prompt")
	}
	if !strings.Contains(prompt, "[HIGH]") {
		t.Error("expected HIGH severity marker in prompt")
	}
	if !strings.Contains(prompt, "Token rotation not aligned with JWT spec.") {
		t.Error("expected feedback body in prompt")
	}
}

func TestBuildNextGenPrompt_NilFeedback(t *testing.T) {
	// given: no feedback
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "scans")
	os.MkdirAll(scanDir, 0755)

	cfg := DefaultConfig()
	wave := Wave{ClusterName: "Auth", ID: "auth-w1"}
	cluster := ClusterScanResult{
		Name:         "Auth",
		Completeness: 0.5,
		Issues:       []IssueDetail{{ID: "ENG-100", Identifier: "ENG-100", Title: "Issue", Completeness: 0.5}},
	}

	// when
	prompt, err := buildNextGenPrompt(&cfg, scanDir, wave, cluster, nil, nil, nil, "fog", nil)

	// then
	if err != nil {
		t.Fatalf("buildNextGenPrompt: %v", err)
	}
	if strings.Contains(prompt, "受信フィードバック") || strings.Contains(prompt, "Received Feedback") {
		t.Error("feedback section should be omitted when nil")
	}
}

func TestNeedsMoreWaves_HighCompleteness_False(t *testing.T) {
	// given: cluster completeness >= 0.95
	cluster := ClusterScanResult{Name: "Auth", Completeness: 0.96}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Status: "completed"},
	}

	// when
	result := NeedsMoreWaves(cluster, waves)

	// then
	if result {
		t.Error("expected false when completeness >= 0.95")
	}
}

func TestNeedsMoreWaves_RemainingWaves_False(t *testing.T) {
	// given: available waves still exist
	cluster := ClusterScanResult{Name: "Auth", Completeness: 0.5}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Status: "completed"},
		{ID: "auth-w2", ClusterName: "Auth", Status: "available"},
	}

	// when
	result := NeedsMoreWaves(cluster, waves)

	// then
	if result {
		t.Error("expected false when available waves remain")
	}
}

func TestNeedsMoreWaves_WaveCapReached_False(t *testing.T) {
	// given: 8 waves already exist for this cluster
	cluster := ClusterScanResult{Name: "Auth", Completeness: 0.6}
	var waves []Wave
	for i := range 8 {
		waves = append(waves, Wave{
			ID:          fmt.Sprintf("auth-w%d", i+1),
			ClusterName: "Auth",
			Status:      "completed",
		})
	}

	// when
	result := NeedsMoreWaves(cluster, waves)

	// then
	if result {
		t.Error("expected false when wave cap (8) reached")
	}
}

func TestNeedsMoreWaves_LowCompleteness_NoRemaining_True(t *testing.T) {
	// given: low completeness, all waves completed, under cap
	cluster := ClusterScanResult{Name: "Auth", Completeness: 0.5}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Status: "completed"},
	}

	// when
	result := NeedsMoreWaves(cluster, waves)

	// then
	if !result {
		t.Error("expected true when completeness low, no remaining waves, under cap")
	}
}

func TestNeedsMoreWaves_IgnoresOtherClusterWaves(t *testing.T) {
	// given: waves from other clusters should not affect count
	cluster := ClusterScanResult{Name: "Auth", Completeness: 0.5}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Status: "completed"},
		{ID: "infra-w1", ClusterName: "Infra", Status: "available"},
	}

	// when
	result := NeedsMoreWaves(cluster, waves)

	// then
	if !result {
		t.Error("expected true: other cluster's waves should not count")
	}
}

func TestNeedsMoreWaves_PartialWaveIsPending(t *testing.T) {
	// given: only wave is "partial" (failed apply), no other available/locked
	cluster := ClusterScanResult{Name: "Auth", Completeness: 0.4}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Status: "partial"},
	}

	// when
	result := NeedsMoreWaves(cluster, waves)

	// then: partial wave = unfinished work, should NOT trigger nextgen
	if result {
		t.Error("expected false: partial wave should be treated as pending, not trigger nextgen")
	}
}

func TestGenerateNextWavesDryRun(t *testing.T) {
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "scans")
	if err := os.MkdirAll(scanDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	wave := Wave{ClusterName: "Auth", ID: "auth-w1"}
	cluster := ClusterScanResult{
		Name:         "Auth",
		Completeness: 0.65,
		Issues:       []IssueDetail{{ID: "ENG-101", Identifier: "ENG-101", Title: "Auth issue", Completeness: 0.5}},
	}
	completedWaves := []Wave{{ID: "auth-w1", ClusterName: "Auth", Title: "Initial setup", Status: "completed"}}

	err := GenerateNextWavesDryRun(&cfg, scanDir, wave, cluster, completedWaves, nil, nil, "fog", nil, NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}

	// Verify prompt file was created
	promptFile := filepath.Join(scanDir, "nextgen_auth_auth-w1_prompt.md")
	if _, err := os.Stat(promptFile); os.IsNotExist(err) {
		t.Error("prompt file should have been created")
	}
}
