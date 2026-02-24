package sightjack_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack"
)

func TestConfigPath(t *testing.T) {
	// when
	path := sightjack.ConfigPath("/project")

	// then
	expected := filepath.Join("/project", ".siren", "config.yaml")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestWriteGitIgnore(t *testing.T) {
	// given
	dir := t.TempDir()
	sirenDir := filepath.Join(dir, sightjack.StateDir)
	os.MkdirAll(sirenDir, 0755)

	// when
	err := sightjack.WriteGitIgnore(dir)

	// then
	if err != nil {
		t.Fatalf("WriteGitIgnore failed: %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(sirenDir, ".gitignore"))
	if readErr != nil {
		t.Fatalf("read .gitignore: %v", readErr)
	}
	content := string(data)
	if !strings.Contains(content, "events/") {
		t.Errorf("expected events/ in .gitignore, got:\n%s", content)
	}
	if !strings.Contains(content, ".run/") {
		t.Errorf("expected .run/ in .gitignore, got:\n%s", content)
	}
}

func TestWriteGitIgnore_Idempotent(t *testing.T) {
	// given
	dir := t.TempDir()
	sirenDir := filepath.Join(dir, sightjack.StateDir)
	os.MkdirAll(sirenDir, 0755)

	// when: call twice
	if err := sightjack.WriteGitIgnore(dir); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := sightjack.WriteGitIgnore(dir); err != nil {
		t.Fatalf("second call: %v", err)
	}

	// then: content should not be duplicated
	data, _ := os.ReadFile(filepath.Join(sirenDir, ".gitignore"))
	content := string(data)
	if strings.Count(content, "events/") != 1 {
		t.Errorf("expected events/ exactly once, got:\n%s", content)
	}
	if strings.Count(content, ".run/") != 1 {
		t.Errorf("expected .run/ exactly once, got:\n%s", content)
	}
}

func TestEnsureScanDir_CreatesGitIgnore(t *testing.T) {
	// given
	dir := t.TempDir()

	// when
	_, err := sightjack.EnsureScanDir(dir, "test-session")

	// then
	if err != nil {
		t.Fatalf("EnsureScanDir failed: %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(dir, sightjack.StateDir, ".gitignore"))
	if readErr != nil {
		t.Fatalf(".gitignore not created: %v", readErr)
	}
	content := string(data)
	if !strings.Contains(content, "events/") {
		t.Errorf("expected events/ in .gitignore, got:\n%s", content)
	}
}

func TestSessionState_ADRCount_Positive(t *testing.T) {
	// given
	state := sightjack.SessionState{
		Version:  "0.4",
		ADRCount: 3,
	}

	// when
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded sightjack.SessionState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if decoded.ADRCount != 3 {
		t.Errorf("expected ADRCount 3, got %d", decoded.ADRCount)
	}
}

func TestSessionState_ADRCount_ZeroOmitted(t *testing.T) {
	// given: ADRCount = 0 should be omitted from JSON (omitempty)
	state := sightjack.SessionState{
		Version:  "0.4",
		ADRCount: 0,
	}

	// when
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// then: "adr_count" should not appear in JSON
	raw := string(data)
	if json.Valid(data) && strings.Contains(raw, "adr_count") {
		t.Errorf("expected adr_count to be omitted when 0, got: %s", raw)
	}
}

func TestWriteAndLoadScanResult_RoundTrip(t *testing.T) {
	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "scan_result.json")
	original := &sightjack.ScanResult{
		Clusters: []sightjack.ClusterScanResult{
			{
				Name:         "Auth",
				Completeness: 0.25,
				Issues: []sightjack.IssueDetail{
					{ID: "ENG-101", Identifier: "ENG-101", Title: "Login", Completeness: 0.30},
				},
				Observations: []string{"Missing MFA"},
			},
			{
				Name:         "API",
				Completeness: 0.40,
				Issues: []sightjack.IssueDetail{
					{ID: "ENG-201", Identifier: "ENG-201", Title: "Rate limit", Completeness: 0.40},
				},
				Observations: []string{"No throttling"},
			},
		},
		TotalIssues:  2,
		Completeness: 0.325,
		Observations: []string{"Missing MFA", "No throttling"},
	}

	// when
	if err := sightjack.WriteScanResult(path, original); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := sightjack.LoadScanResult(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// then
	if len(loaded.Clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(loaded.Clusters))
	}
	if loaded.Clusters[0].Name != "Auth" {
		t.Errorf("expected Auth, got %s", loaded.Clusters[0].Name)
	}
	if loaded.Completeness != 0.325 {
		t.Errorf("expected 0.325, got %f", loaded.Completeness)
	}
	if loaded.TotalIssues != 2 {
		t.Errorf("expected 2 total issues, got %d", loaded.TotalIssues)
	}
	if len(loaded.Clusters[0].Issues) != 1 {
		t.Errorf("expected 1 issue in Auth, got %d", len(loaded.Clusters[0].Issues))
	}
}

func TestLoadScanResult_FileNotFound(t *testing.T) {
	// when
	_, err := sightjack.LoadScanResult("/nonexistent/scan_result.json")

	// then
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestSessionState_ScanResultPath_OmittedWhenEmpty(t *testing.T) {
	// given
	state := sightjack.SessionState{Version: "0.5", ScanResultPath: ""}

	// when
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// then
	if strings.Contains(string(data), "scan_result_path") {
		t.Error("expected scan_result_path to be omitted when empty")
	}
}

func TestLoadScanResult_SnakeCaseFormat(t *testing.T) {
	// given: JSON with snake_case field names (v0.0.12+ wire format)
	dir := t.TempDir()
	path := filepath.Join(dir, "scan_result.json")
	os.WriteFile(path, []byte(`{
		"clusters": [
			{"name": "Auth", "completeness": 0.50, "issues": [], "observations": ["obs1"]}
		],
		"total_issues": 5,
		"completeness": 0.50,
		"observations": ["global obs"]
	}`), 0644)

	// when
	result, err := sightjack.LoadScanResult(path)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(result.Clusters))
	}
	if result.Clusters[0].Name != "Auth" {
		t.Errorf("expected cluster name Auth, got %s", result.Clusters[0].Name)
	}
	if result.TotalIssues != 5 {
		t.Errorf("expected TotalIssues 5, got %d", result.TotalIssues)
	}
	if result.Completeness != 0.50 {
		t.Errorf("expected Completeness 0.50, got %f", result.Completeness)
	}
}

func TestLoadScanResult_MalformedJSON(t *testing.T) {
	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "scan_result.json")
	os.WriteFile(path, []byte(`{invalid`), 0644)

	// when
	_, err := sightjack.LoadScanResult(path)

	// then
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestWriteGitIgnore_IncludesMailDirs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, sightjack.StateDir), 0755)
	if err := sightjack.WriteGitIgnore(dir); err != nil {
		t.Fatalf("WriteGitIgnore: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, sightjack.StateDir, ".gitignore"))
	content := string(data)
	if !strings.Contains(content, "inbox/") {
		t.Error("expected inbox/ in .gitignore")
	}
	if !strings.Contains(content, "outbox/") {
		t.Error("expected outbox/ in .gitignore")
	}
	if strings.Contains(content, "archive/") {
		t.Error("archive/ should NOT be in .gitignore (git-tracked)")
	}
}
