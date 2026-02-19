package sightjack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestState_WriteAndRead_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	state := SessionState{
		Version:      "0.1",
		SessionID:    "test-session-123",
		Project:      "Test Project",
		LastScanned:  time.Date(2026, 2, 16, 10, 0, 0, 0, time.UTC),
		Completeness: 0.32,
		Clusters: []ClusterState{
			{Name: "Auth", Completeness: 0.25, IssueCount: 5},
			{Name: "API", Completeness: 0.40, IssueCount: 8},
		},
	}

	err := WriteState(dir, &state)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	loaded, err := ReadState(dir)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if loaded.SessionID != "test-session-123" {
		t.Errorf("expected session ID test-session-123, got %s", loaded.SessionID)
	}
	if loaded.Completeness != 0.32 {
		t.Errorf("expected 0.32, got %f", loaded.Completeness)
	}
	if len(loaded.Clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(loaded.Clusters))
	}
	if loaded.Clusters[0].Name != "Auth" {
		t.Errorf("expected Auth, got %s", loaded.Clusters[0].Name)
	}
}

func TestState_ReadMissing_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadState(dir)
	if err == nil {
		t.Error("expected error for missing state file")
	}
}

func TestState_WriteAndRead_WithWaves(t *testing.T) {
	// given
	dir := t.TempDir()
	state := &SessionState{
		Version:      "0.2",
		SessionID:    "test-session",
		Project:      "TestProject",
		LastScanned:  time.Now().Truncate(time.Second),
		Completeness: 0.35,
		Clusters: []ClusterState{
			{Name: "Auth", Completeness: 0.25, IssueCount: 4},
		},
		Waves: []WaveState{
			{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed", ActionCount: 3},
			{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "available", Prerequisites: []string{"auth-w1"}, ActionCount: 5},
		},
	}

	// when
	if err := WriteState(dir, state); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := ReadState(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// then
	if len(loaded.Waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(loaded.Waves))
	}
	if loaded.Waves[0].ID != "auth-w1" {
		t.Errorf("expected auth-w1, got %s", loaded.Waves[0].ID)
	}
	if loaded.Waves[1].Status != "available" {
		t.Errorf("expected available, got %s", loaded.Waves[1].Status)
	}
	if loaded.Waves[1].Prerequisites[0] != "auth-w1" {
		t.Errorf("expected prerequisite auth-w1")
	}
}

func TestStatePath(t *testing.T) {
	path := StatePath("/project")
	expected := filepath.Join("/project", ".siren", "state.json")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestSessionState_ADRCount_Positive(t *testing.T) {
	// given
	state := SessionState{
		Version:  "0.4",
		ADRCount: 3,
	}

	// when
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded SessionState
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
	state := SessionState{
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

func TestWaveState_FullFieldsRoundTrip(t *testing.T) {
	// given: WaveState with all v0.5 fields populated
	state := &SessionState{
		Version:   "0.5",
		SessionID: "test-full-wave",
		Waves: []WaveState{
			{
				ID:            "auth-w1",
				ClusterName:   "Auth",
				Title:         "Dependency Ordering",
				Status:        "completed",
				Prerequisites: []string{"Auth:auth-w0"},
				ActionCount:   2,
				Actions: []WaveAction{
					{Type: "add_dependency", IssueID: "ENG-101", Description: "Add dep"},
					{Type: "add_dod", IssueID: "ENG-102", Description: "Add DoD"},
				},
				Description: "Order dependencies first",
				Delta:       WaveDelta{Before: 0.25, After: 0.50},
			},
		},
	}

	// when: round-trip through WriteState / ReadState
	dir := t.TempDir()
	if err := WriteState(dir, state); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := ReadState(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// then
	w := loaded.Waves[0]
	if len(w.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(w.Actions))
	}
	if w.Actions[0].Type != "add_dependency" {
		t.Errorf("expected add_dependency, got %s", w.Actions[0].Type)
	}
	if w.Description != "Order dependencies first" {
		t.Errorf("expected description, got %s", w.Description)
	}
	if w.Delta.Before != 0.25 || w.Delta.After != 0.50 {
		t.Errorf("expected delta {0.25, 0.50}, got {%v, %v}", w.Delta.Before, w.Delta.After)
	}
}

func TestSessionState_ADRCount_WriteAndRead(t *testing.T) {
	// given
	dir := t.TempDir()
	state := &SessionState{
		Version:      "0.4",
		SessionID:    "test-adr-count",
		Project:      "TestProject",
		LastScanned:  time.Now().Truncate(time.Second),
		Completeness: 0.50,
		Clusters:     []ClusterState{{Name: "Auth", Completeness: 0.50, IssueCount: 3}},
		ADRCount:     5,
	}

	// when
	if err := WriteState(dir, state); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := ReadState(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// then
	if loaded.ADRCount != 5 {
		t.Errorf("expected ADRCount 5, got %d", loaded.ADRCount)
	}
}

func TestWriteAndLoadScanResult_RoundTrip(t *testing.T) {
	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "scan_result.json")
	original := &ScanResult{
		Clusters: []ClusterScanResult{
			{
				Name:         "Auth",
				Completeness: 0.25,
				Issues: []IssueDetail{
					{ID: "ENG-101", Identifier: "ENG-101", Title: "Login", Completeness: 0.30},
				},
				Observations: []string{"Missing MFA"},
			},
			{
				Name:         "API",
				Completeness: 0.40,
				Issues: []IssueDetail{
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
	if err := WriteScanResult(path, original); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := LoadScanResult(path)
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
	_, err := LoadScanResult("/nonexistent/scan_result.json")

	// then
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestSessionState_ScanResultPath_RoundTrip(t *testing.T) {
	// given
	dir := t.TempDir()
	state := &SessionState{
		Version:        "0.5",
		SessionID:      "test-scan-path",
		ScanResultPath: ".siren/scans/session-123/scan_result.json",
	}

	// when
	if err := WriteState(dir, state); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := ReadState(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// then
	if loaded.ScanResultPath != ".siren/scans/session-123/scan_result.json" {
		t.Errorf("expected scan result path, got %s", loaded.ScanResultPath)
	}
}

func TestSessionState_ScanResultPath_OmittedWhenEmpty(t *testing.T) {
	// given
	state := SessionState{Version: "0.5", ScanResultPath: ""}

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

func TestLoadScanResult_PreV06Format_BackwardCompat(t *testing.T) {
	// given: JSON with Go-default field names (pre-v0.6 format, no JSON tags)
	dir := t.TempDir()
	path := filepath.Join(dir, "scan_result.json")
	os.WriteFile(path, []byte(`{
		"Clusters": [
			{"Name": "Auth", "Completeness": 0.50, "Issues": [], "Observations": ["obs1"]}
		],
		"TotalIssues": 5,
		"Completeness": 0.50,
		"Observations": ["global obs"]
	}`), 0644)

	// when
	result, err := LoadScanResult(path)

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

func TestState_OldVersionRoundTrip(t *testing.T) {
	// given: state.json written by an older version (v0.0.9 format)
	// The current code should read it without error — version is just a string field.
	dir := t.TempDir()
	oldState := SessionState{
		Version:      "0.0.9",
		SessionID:    "old-session",
		Project:      "legacy",
		LastScanned:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Completeness: 0.75,
		Clusters: []ClusterState{
			{Name: "Auth", Completeness: 0.80, IssueCount: 3},
		},
		Waves: []WaveState{
			{ID: "w1", ClusterName: "Auth", Status: "completed", ActionCount: 2},
		},
		ADRCount:    1,
		ShibitoCount: 0,
	}

	// when: write with old version and read back
	if err := WriteState(dir, &oldState); err != nil {
		t.Fatalf("WriteState: %v", err)
	}
	loaded, err := ReadState(dir)
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}

	// then: all fields preserved, version remains "0.0.9" (not auto-upgraded)
	if loaded.Version != "0.0.9" {
		t.Errorf("expected version 0.0.9, got %s", loaded.Version)
	}
	if loaded.Project != "legacy" {
		t.Errorf("expected project legacy, got %s", loaded.Project)
	}
	if loaded.Completeness != 0.75 {
		t.Errorf("expected completeness 0.75, got %f", loaded.Completeness)
	}
	if len(loaded.Waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(loaded.Waves))
	}
	if loaded.Waves[0].Status != "completed" {
		t.Errorf("expected wave completed, got %s", loaded.Waves[0].Status)
	}
}

func TestState_FutureFieldsIgnored(t *testing.T) {
	// given: state.json with extra fields not in current struct (forward compatibility)
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".siren")
	os.MkdirAll(stateDir, 0755)
	rawJSON := `{
		"version": "0.1.0",
		"session_id": "future-session",
		"project": "future",
		"completeness": 0.90,
		"unknown_field": "should be ignored",
		"nested_unknown": {"a": 1}
	}`
	os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(rawJSON), 0644)

	// when
	loaded, err := ReadState(dir)

	// then: should load without error, unknown fields silently ignored
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if loaded.Version != "0.1.0" {
		t.Errorf("expected version 0.1.0, got %s", loaded.Version)
	}
	if loaded.Completeness != 0.90 {
		t.Errorf("expected completeness 0.90, got %f", loaded.Completeness)
	}
}

func TestLoadScanResult_MalformedJSON(t *testing.T) {
	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "scan_result.json")
	os.WriteFile(path, []byte(`{invalid`), 0644)

	// when
	_, err := LoadScanResult(path)

	// then
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}
