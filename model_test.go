package sightjack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyResult_UnmarshalJSON(t *testing.T) {
	// given
	raw := `{
		"clusters": [
			{"name": "Auth", "issue_ids": ["ID1", "ID2"]},
			{"name": "API", "issue_ids": ["ID3"]}
		],
		"total_issues": 3
	}`

	// when
	var result ClassifyResult
	err := json.Unmarshal([]byte(raw), &result)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(result.Clusters))
	}
	if result.Clusters[0].Name != "Auth" {
		t.Errorf("expected Auth, got %s", result.Clusters[0].Name)
	}
	if len(result.Clusters[0].IssueIDs) != 2 {
		t.Errorf("expected 2 issue IDs, got %d", len(result.Clusters[0].IssueIDs))
	}
	if result.TotalIssues != 3 {
		t.Errorf("expected 3 total issues, got %d", result.TotalIssues)
	}
}

func TestClusterScanResult_UnmarshalJSON(t *testing.T) {
	// given
	raw := `{
		"name": "Auth",
		"completeness": 0.35,
		"issues": [
			{
				"id": "abc-123",
				"identifier": "AWE-50",
				"title": "Implement login",
				"completeness": 0.4,
				"gaps": ["DoD missing", "No dependency specified"]
			}
		],
		"observations": ["Hidden dependency on API cluster"]
	}`

	// when
	var result ClusterScanResult
	err := json.Unmarshal([]byte(raw), &result)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "Auth" {
		t.Errorf("expected Auth, got %s", result.Name)
	}
	if result.Completeness != 0.35 {
		t.Errorf("expected 0.35, got %f", result.Completeness)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}
	if result.Issues[0].Identifier != "AWE-50" {
		t.Errorf("expected AWE-50, got %s", result.Issues[0].Identifier)
	}
	if len(result.Issues[0].Gaps) != 2 {
		t.Errorf("expected 2 gaps, got %d", len(result.Issues[0].Gaps))
	}
}

func TestWave_UnmarshalJSON(t *testing.T) {
	data := `{
		"id": "auth-w1",
		"cluster_name": "Auth",
		"title": "Dependency Ordering",
		"description": "Establish issue dependencies",
		"actions": [
			{"type": "add_dependency", "issue_id": "ENG-101", "description": "Auth before token", "detail": "ENG-101 -> ENG-102"}
		],
		"prerequisites": [],
		"delta": {"before": 0.25, "after": 0.40},
		"status": "available"
	}`
	var w Wave
	if err := json.Unmarshal([]byte(data), &w); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if w.ID != "auth-w1" {
		t.Errorf("expected auth-w1, got %s", w.ID)
	}
	if w.ClusterName != "Auth" {
		t.Errorf("expected Auth, got %s", w.ClusterName)
	}
	if len(w.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(w.Actions))
	}
	if w.Actions[0].Type != "add_dependency" {
		t.Errorf("expected add_dependency, got %s", w.Actions[0].Type)
	}
	if w.Delta.Before != 0.25 || w.Delta.After != 0.40 {
		t.Errorf("unexpected delta: %+v", w.Delta)
	}
}

func TestWaveGenerateResult_UnmarshalJSON(t *testing.T) {
	data := `{
		"cluster_name": "Auth",
		"waves": [
			{"id": "auth-w1", "cluster_name": "Auth", "title": "W1", "actions": [], "prerequisites": [], "delta": {"before": 0.25, "after": 0.40}, "status": "available"},
			{"id": "auth-w2", "cluster_name": "Auth", "title": "W2", "actions": [], "prerequisites": ["auth-w1"], "delta": {"before": 0.40, "after": 0.65}, "status": "locked"}
		]
	}`
	var result WaveGenerateResult
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.ClusterName != "Auth" {
		t.Errorf("expected Auth, got %s", result.ClusterName)
	}
	if len(result.Waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(result.Waves))
	}
	if result.Waves[1].Prerequisites[0] != "auth-w1" {
		t.Errorf("expected prerequisite auth-w1, got %s", result.Waves[1].Prerequisites[0])
	}
}

func TestWaveApplyResult_UnmarshalJSON(t *testing.T) {
	data := `{
		"wave_id": "auth-w1",
		"applied": 7,
		"errors": [],
		"ripples": [
			{"cluster_name": "API", "description": "W2 unlocked"}
		]
	}`
	var result WaveApplyResult
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.WaveID != "auth-w1" {
		t.Errorf("expected auth-w1, got %s", result.WaveID)
	}
	if result.Applied != 7 {
		t.Errorf("expected 7, got %d", result.Applied)
	}
	if len(result.Ripples) != 1 {
		t.Fatalf("expected 1 ripple, got %d", len(result.Ripples))
	}
	if result.Ripples[0].ClusterName != "API" {
		t.Errorf("expected API, got %s", result.Ripples[0].ClusterName)
	}
}

func TestArchitectResponse_UnmarshalJSON(t *testing.T) {
	data := `{
		"analysis": "Looking at the cluster, splitting is unnecessary.",
		"modified_wave": {
			"id": "auth-w1",
			"cluster_name": "Auth",
			"title": "Dependency Ordering",
			"actions": [
				{"type": "add_dependency", "issue_id": "ENG-101", "description": "Auth before token", "detail": ""}
			],
			"prerequisites": [],
			"delta": {"before": 0.25, "after": 0.42},
			"status": "available"
		},
		"reasoning": "Project scale favors fewer issues"
	}`

	var resp ArchitectResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Analysis != "Looking at the cluster, splitting is unnecessary." {
		t.Errorf("unexpected analysis: %s", resp.Analysis)
	}
	if resp.ModifiedWave == nil {
		t.Fatal("expected non-nil modified_wave")
	}
	if resp.ModifiedWave.ID != "auth-w1" {
		t.Errorf("expected auth-w1, got %s", resp.ModifiedWave.ID)
	}
	if resp.Reasoning != "Project scale favors fewer issues" {
		t.Errorf("unexpected reasoning: %s", resp.Reasoning)
	}
}

func TestArchitectResponse_NilModifiedWave(t *testing.T) {
	data := `{
		"analysis": "No changes needed.",
		"modified_wave": null,
		"reasoning": "Current actions are sufficient"
	}`

	var resp ArchitectResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ModifiedWave != nil {
		t.Error("expected nil modified_wave")
	}
}

func TestScanResult_CalculateCompleteness(t *testing.T) {
	// given
	result := ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "Auth", Completeness: 0.25, Issues: make([]IssueDetail, 5)},
			{Name: "API", Completeness: 0.40, Issues: make([]IssueDetail, 5)},
		},
	}

	// when
	result.CalculateCompleteness()

	// then
	expected := 0.325
	if result.Completeness != expected {
		t.Errorf("expected %f, got %f", expected, result.Completeness)
	}
	if result.TotalIssues != 10 {
		t.Errorf("expected 10 total issues, got %d", result.TotalIssues)
	}
}

func TestArchitectResponse_MissingAnalysis(t *testing.T) {
	// given: JSON without "analysis" key — Go defaults to empty string
	data := `{
		"modified_wave": null,
		"reasoning": "ok"
	}`

	// when
	var resp ArchitectResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if resp.Analysis != "" {
		t.Errorf("expected empty string for missing analysis, got: %s", resp.Analysis)
	}
}

func TestArchitectResponse_ModifiedWaveEmptyActions(t *testing.T) {
	// given: modified_wave with "actions": [] (explicitly empty, not omitted)
	data := `{
		"analysis": "Simplified.",
		"modified_wave": {
			"id": "auth-w1",
			"cluster_name": "Auth",
			"title": "Simplified",
			"actions": [],
			"prerequisites": [],
			"delta": {"before": 0.25, "after": 0.40},
			"status": "available"
		},
		"reasoning": "Removed all actions"
	}`

	// when
	var resp ArchitectResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if resp.ModifiedWave == nil {
		t.Fatal("expected non-nil modified_wave")
	}
	if resp.ModifiedWave.Actions == nil {
		t.Error("expected non-nil (empty) Actions slice for explicit []")
	}
	if len(resp.ModifiedWave.Actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(resp.ModifiedWave.Actions))
	}
}

func TestScribeResponse_JSONRoundTrip(t *testing.T) {
	// given
	original := ScribeResponse{
		ADRID:     "0003",
		Title:     "adopt-event-sourcing",
		Content:   "# 0003. Adopt Event Sourcing\n\n**Date:** 2026-02-18\n**Status:** Accepted",
		Reasoning: "The architect discussion revealed a need for event sourcing.",
	}

	// when: marshal then unmarshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ScribeResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if decoded.ADRID != original.ADRID {
		t.Errorf("ADRID: expected %s, got %s", original.ADRID, decoded.ADRID)
	}
	if decoded.Title != original.Title {
		t.Errorf("Title: expected %s, got %s", original.Title, decoded.Title)
	}
	if decoded.Content != original.Content {
		t.Errorf("Content: expected %s, got %s", original.Content, decoded.Content)
	}
	if decoded.Reasoning != original.Reasoning {
		t.Errorf("Reasoning: expected %s, got %s", original.Reasoning, decoded.Reasoning)
	}
}

func TestParseStrictnessLevel_ValidValues(t *testing.T) {
	tests := []struct {
		input    string
		expected StrictnessLevel
	}{
		{"fog", StrictnessFog},
		{"alert", StrictnessAlert},
		{"lockdown", StrictnessLockdown},
		{"FOG", StrictnessFog},
		{"Alert", StrictnessAlert},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseStrictnessLevel(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if level != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, level)
			}
		})
	}
}

func TestParseStrictnessLevel_Invalid(t *testing.T) {
	_, err := ParseStrictnessLevel("nightmare")
	if err == nil {
		t.Fatal("expected error for invalid strictness level")
	}
}

func TestStrictnessLevel_Valid(t *testing.T) {
	valid := StrictnessFog
	invalid := StrictnessLevel("nightmare")
	if !valid.Valid() {
		t.Error("expected fog to be valid")
	}
	if invalid.Valid() {
		t.Error("expected nightmare to be invalid")
	}
}

func TestShibitoWarning_JSONRoundTrip(t *testing.T) {
	// given
	original := ShibitoWarning{
		ClosedIssueID:  "ENG-045",
		CurrentIssueID: "ENG-102",
		Description:    "Token management circular dependency re-emerging",
		RiskLevel:      "high",
	}

	// when
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ShibitoWarning
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if decoded.ClosedIssueID != "ENG-045" {
		t.Errorf("expected ENG-045, got %s", decoded.ClosedIssueID)
	}
	if decoded.RiskLevel != "high" {
		t.Errorf("expected high, got %s", decoded.RiskLevel)
	}
}

func TestScanResult_MarshalJSON_SnakeCaseKeys(t *testing.T) {
	// given
	result := ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "Auth", Completeness: 0.25},
		},
		TotalIssues:  5,
		Completeness: 0.35,
		Observations: []string{"test obs"},
	}

	// when
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(data)

	// then: all keys must be snake_case
	if !strings.Contains(s, `"clusters"`) {
		t.Errorf("expected 'clusters' key (snake_case), got: %s", s)
	}
	if !strings.Contains(s, `"total_issues"`) {
		t.Errorf("expected 'total_issues' key (snake_case), got: %s", s)
	}
	if !strings.Contains(s, `"completeness"`) {
		t.Errorf("expected 'completeness' key (snake_case), got: %s", s)
	}
	if !strings.Contains(s, `"observations"`) {
		t.Errorf("expected 'observations' key (snake_case), got: %s", s)
	}
}

func TestScanResult_UnmarshalJSON_SnakeCaseKeys(t *testing.T) {
	// given: snake_case JSON (wire format)
	raw := `{
		"clusters": [{"name": "Auth", "completeness": 0.25, "issues": [], "observations": []}],
		"total_issues": 5,
		"completeness": 0.35,
		"observations": ["global obs"],
		"shibito_warnings": [],
		"scan_warnings": ["warn1"]
	}`

	// when
	var result ScanResult
	err := json.Unmarshal([]byte(raw), &result)

	// then
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.Clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(result.Clusters))
	}
	if result.TotalIssues != 5 {
		t.Errorf("expected 5 total_issues, got %d", result.TotalIssues)
	}
	if result.Completeness != 0.35 {
		t.Errorf("expected 0.35, got %f", result.Completeness)
	}
	if len(result.Observations) != 1 || result.Observations[0] != "global obs" {
		t.Errorf("unexpected observations: %v", result.Observations)
	}
	if len(result.ScanWarnings) != 1 {
		t.Errorf("expected 1 scan_warning, got %d", len(result.ScanWarnings))
	}
}

func TestScanResult_JSONRoundTrip(t *testing.T) {
	// given
	original := ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "Auth", Completeness: 0.25, Issues: []IssueDetail{
				{ID: "abc", Identifier: "ENG-1", Title: "Login", Completeness: 0.3, Gaps: []string{"DoD"}},
			}, Observations: []string{"obs1"}, Labels: []string{"security"}},
		},
		TotalIssues:     1,
		Completeness:    0.25,
		Observations:    []string{"global"},
		ShibitoWarnings: []ShibitoWarning{{ClosedIssueID: "X", CurrentIssueID: "Y", Description: "reborn", RiskLevel: "high"}},
		ScanWarnings:    []string{"warn"},
	}

	// when: marshal then unmarshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ScanResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if len(decoded.Clusters) != 1 {
		t.Fatalf("clusters: got %d, want 1", len(decoded.Clusters))
	}
	if decoded.TotalIssues != 1 {
		t.Errorf("total_issues: got %d, want 1", decoded.TotalIssues)
	}
	if decoded.Completeness != 0.25 {
		t.Errorf("completeness: got %f, want 0.25", decoded.Completeness)
	}
	if len(decoded.Observations) != 1 {
		t.Errorf("observations: got %d, want 1", len(decoded.Observations))
	}
	if len(decoded.ShibitoWarnings) != 1 {
		t.Errorf("shibito_warnings: got %d, want 1", len(decoded.ShibitoWarnings))
	}
	if len(decoded.ScanWarnings) != 1 {
		t.Errorf("scan_warnings: got %d, want 1", len(decoded.ScanWarnings))
	}
}

func TestScanResult_UnmarshalJSON_ForwardCompatible(t *testing.T) {
	// given: JSON with unknown fields (future schema additions)
	raw := `{
		"clusters": [],
		"total_issues": 0,
		"completeness": 0.0,
		"observations": [],
		"future_field": "should be ignored",
		"another_future": 42
	}`

	// when
	var result ScanResult
	err := json.Unmarshal([]byte(raw), &result)

	// then: should not error on unknown fields
	if err != nil {
		t.Fatalf("expected forward-compatible unmarshal, got error: %v", err)
	}
}

func TestScanResult_ShibitoWarnings_OmittedWhenEmpty(t *testing.T) {
	// given
	result := ScanResult{Completeness: 0.5}

	// when
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// then
	if strings.Contains(string(data), "shibito_warnings") {
		t.Error("expected shibito_warnings to be omitted when empty")
	}
}

func TestADRConflict_JSONRoundTrip(t *testing.T) {
	// given
	original := ADRConflict{
		ExistingADRID: "0002",
		Description:   "Contradicts ADR-0002 decision on session storage",
	}

	// when
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ADRConflict
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if decoded.ExistingADRID != "0002" {
		t.Errorf("expected 0002, got %s", decoded.ExistingADRID)
	}
	if decoded.Description != "Contradicts ADR-0002 decision on session storage" {
		t.Errorf("unexpected description: %s", decoded.Description)
	}
}

func TestScribeResponse_Conflicts_OmittedWhenEmpty(t *testing.T) {
	// given
	resp := ScribeResponse{ADRID: "0001", Title: "test"}

	// when
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// then
	if strings.Contains(string(data), "conflicts") {
		t.Error("expected conflicts to be omitted when empty")
	}
}

func TestScribeResponse_Conflicts_Present(t *testing.T) {
	// given
	resp := ScribeResponse{
		ADRID: "0003",
		Title: "test",
		Conflicts: []ADRConflict{
			{ExistingADRID: "0001", Description: "contradicts auth decision"},
		},
	}

	// when
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ScribeResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if len(decoded.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(decoded.Conflicts))
	}
	if decoded.Conflicts[0].ExistingADRID != "0001" {
		t.Errorf("expected 0001, got %s", decoded.Conflicts[0].ExistingADRID)
	}
}

func TestNextGenResult_UnmarshalJSON(t *testing.T) {
	raw := `{"cluster_name":"Auth","waves":[{"id":"auth-w3","cluster_name":"Auth","title":"Security hardening","description":"Final security pass","actions":[{"type":"add_dod","issue_id":"ENG-101","description":"Add security checklist","detail":"..."}],"prerequisites":["auth-w2"],"delta":{"before":0.65,"after":0.80},"status":"available"}],"reasoning":"Auth cluster needs final security pass"}`

	var result NextGenResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.ClusterName != "Auth" {
		t.Errorf("cluster_name: got %q, want %q", result.ClusterName, "Auth")
	}
	if len(result.Waves) != 1 {
		t.Fatalf("waves: got %d, want 1", len(result.Waves))
	}
	if result.Reasoning != "Auth cluster needs final security pass" {
		t.Errorf("reasoning: got %q", result.Reasoning)
	}
}

func TestApprovalSelective_IsDistinctValue(t *testing.T) {
	choices := []ApprovalChoice{ApprovalApprove, ApprovalReject, ApprovalDiscuss, ApprovalQuit, ApprovalSelective}
	seen := make(map[ApprovalChoice]bool)
	for _, c := range choices {
		if seen[c] {
			t.Errorf("duplicate ApprovalChoice value: %d", c)
		}
		seen[c] = true
	}
	if len(seen) != 5 {
		t.Errorf("expected 5 distinct choices, got %d", len(seen))
	}
}

func TestWaveApplyResultTotalCount(t *testing.T) {
	// given
	data := `{"wave_id":"w1","applied":3,"total_count":5,"errors":["e1"]}`

	// when
	var result WaveApplyResult
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatal(err)
	}

	// then
	if result.TotalCount != 5 {
		t.Errorf("TotalCount: expected 5, got %d", result.TotalCount)
	}
}

func TestScanResult_StrictnessKeys(t *testing.T) {
	// given: scan result with labeled clusters
	result := &ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "Auth", Labels: []string{"security", "backend"}},
			{Name: "UI", Labels: []string{"frontend"}},
		},
	}

	// when/then: keys include cluster name + labels
	keys := result.StrictnessKeys("Auth")
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d: %v", len(keys), keys)
	}
	if keys[0] != "Auth" || keys[1] != "security" || keys[2] != "backend" {
		t.Errorf("unexpected keys: %v", keys)
	}

	// unknown cluster returns just the name
	keys2 := result.StrictnessKeys("Unknown")
	if len(keys2) != 1 || keys2[0] != "Unknown" {
		t.Errorf("expected [Unknown], got %v", keys2)
	}
}

func TestClusterScanResult_NumIssues_FromSlice(t *testing.T) {
	// given: cluster with populated Issues slice
	c := ClusterScanResult{
		Name:   "Auth",
		Issues: make([]IssueDetail, 5),
	}

	// when/then
	if got := c.NumIssues(); got != 5 {
		t.Errorf("NumIssues() = %d, want 5", got)
	}
}

func TestClusterScanResult_NumIssues_FromIssueCount(t *testing.T) {
	// given: cluster with IssueCount but no Issues slice (show command case)
	c := ClusterScanResult{
		Name:       "Auth",
		IssueCount: 8,
	}

	// when/then
	if got := c.NumIssues(); got != 8 {
		t.Errorf("NumIssues() = %d, want 8", got)
	}
}

func TestClusterScanResult_NumIssues_SliceTakesPrecedence(t *testing.T) {
	// given: both Issues and IssueCount set — slice wins
	c := ClusterScanResult{
		Name:       "Auth",
		Issues:     make([]IssueDetail, 3),
		IssueCount: 10,
	}

	// when/then
	if got := c.NumIssues(); got != 3 {
		t.Errorf("NumIssues() = %d, want 3 (slice takes precedence)", got)
	}
}

func TestScanResult_ClusterLabels_NilWhenNoLabels(t *testing.T) {
	// given: cluster without labels
	result := &ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "Auth"},
		},
	}

	// when/then
	labels := result.ClusterLabels("Auth")
	if labels != nil {
		t.Errorf("expected nil labels, got %v", labels)
	}
}

func TestScribeResponse_ZeroValues(t *testing.T) {
	// given: all zero-value fields
	data := `{"adr_id":"","title":"","content":"","reasoning":""}`

	// when
	var resp ScribeResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if resp.ADRID != "" {
		t.Errorf("expected empty ADRID, got %s", resp.ADRID)
	}
	if resp.Title != "" {
		t.Errorf("expected empty Title, got %s", resp.Title)
	}
}

// --- Wire format types (pipe interface) ---

func TestWavePlan_JSONRoundTrip(t *testing.T) {
	// given
	raw := `{
		"waves": [
			{
				"id": "auth-w1",
				"cluster_name": "Auth",
				"title": "Dependency Ordering",
				"description": "Order deps",
				"actions": [{"type": "add_dependency", "issue_id": "ENG-101", "description": "dep", "detail": ""}],
				"prerequisites": [],
				"delta": {"before": 0.25, "after": 0.50},
				"status": "available"
			}
		],
		"scan_result": {
			"clusters": [{"name": "Auth", "completeness": 0.25, "issues": [], "observations": []}],
			"total_issues": 5,
			"completeness": 0.25,
			"observations": []
		}
	}`

	// when
	var plan WavePlan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if len(plan.Waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(plan.Waves))
	}
	if plan.Waves[0].ID != "auth-w1" {
		t.Errorf("wave id: got %q, want %q", plan.Waves[0].ID, "auth-w1")
	}
	if plan.ScanResult == nil {
		t.Fatal("expected non-nil scan_result")
	}
	if plan.ScanResult.TotalIssues != 5 {
		t.Errorf("total_issues: got %d, want 5", plan.ScanResult.TotalIssues)
	}

	// round-trip
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded WavePlan
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if len(decoded.Waves) != 1 || decoded.ScanResult.TotalIssues != 5 {
		t.Errorf("round-trip mismatch")
	}
}

func TestWavePlan_ScanResultOmittedWhenNil(t *testing.T) {
	plan := WavePlan{Waves: []Wave{{ID: "w1", ClusterName: "X", Title: "T"}}}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "scan_result") {
		t.Error("expected scan_result to be omitted when nil")
	}
}

func TestWave_ClusterContext(t *testing.T) {
	// given: wave with cluster_context (pipe input to discuss/apply)
	raw := `{
		"id": "auth-w1",
		"cluster_name": "Auth",
		"title": "Dep Ordering",
		"description": "desc",
		"actions": [],
		"prerequisites": [],
		"delta": {"before": 0.25, "after": 0.50},
		"status": "available",
		"cluster_context": {
			"name": "Auth",
			"completeness": 0.25,
			"issues": [{"id": "abc", "identifier": "ENG-1", "title": "Login", "completeness": 0.3, "gaps": []}],
			"observations": ["obs"]
		}
	}`

	// when
	var w Wave
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if w.ClusterContext == nil {
		t.Fatal("expected non-nil cluster_context")
	}
	if w.ClusterContext.Name != "Auth" {
		t.Errorf("context name: got %q, want %q", w.ClusterContext.Name, "Auth")
	}
	if len(w.ClusterContext.Issues) != 1 {
		t.Errorf("context issues: got %d, want 1", len(w.ClusterContext.Issues))
	}
}

func TestWave_ClusterContext_OmittedWhenNil(t *testing.T) {
	w := Wave{ID: "w1", ClusterName: "X", Title: "T"}
	data, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "cluster_context") {
		t.Error("expected cluster_context to be omitted when nil")
	}
}

func TestDiscussResult_JSONRoundTrip(t *testing.T) {
	// given
	raw := `{
		"wave_id": "auth-w1",
		"analysis": "JWT has trade-offs",
		"reasoning": "Session-based is simpler",
		"decision": "Use session-based auth",
		"modifications": [
			{"action_index": 0, "change": "Updated to include Redis"}
		],
		"adr_worthy": true,
		"adr_title": "Session-based auth over JWT"
	}`

	// when
	var dr DiscussResult
	if err := json.Unmarshal([]byte(raw), &dr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if dr.WaveID != "auth-w1" {
		t.Errorf("wave_id: got %q", dr.WaveID)
	}
	if dr.Decision != "Use session-based auth" {
		t.Errorf("decision: got %q", dr.Decision)
	}
	if !dr.ADRWorthy {
		t.Error("expected adr_worthy=true")
	}
	if len(dr.Modifications) != 1 {
		t.Fatalf("modifications: got %d, want 1", len(dr.Modifications))
	}
	if dr.Modifications[0].ActionIndex != 0 {
		t.Errorf("action_index: got %d, want 0", dr.Modifications[0].ActionIndex)
	}

	// round-trip
	data, err := json.Marshal(dr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded DiscussResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if decoded.ADRTitle != "Session-based auth over JWT" {
		t.Errorf("round-trip adr_title: got %q", decoded.ADRTitle)
	}
}

func TestDiscussResult_ModificationsOmittedWhenEmpty(t *testing.T) {
	dr := DiscussResult{WaveID: "w1", Analysis: "ok", Decision: "noop"}
	data, err := json.Marshal(dr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "modifications") {
		t.Error("expected modifications to be omitted when empty")
	}
}

func TestApplyResult_JSONRoundTrip(t *testing.T) {
	// given
	raw := `{
		"wave_id": "auth-w1",
		"applied_actions": [
			{"type": "add_dependency", "issue_id": "ENG-101", "success": true}
		],
		"ripple_effects": [
			{"cluster_name": "API", "description": "W2 unlocked"}
		],
		"new_completeness": 0.50
	}`

	// when
	var ar ApplyResult
	if err := json.Unmarshal([]byte(raw), &ar); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if ar.WaveID != "auth-w1" {
		t.Errorf("wave_id: got %q", ar.WaveID)
	}
	if len(ar.AppliedActions) != 1 {
		t.Fatalf("applied_actions: got %d, want 1", len(ar.AppliedActions))
	}
	if !ar.AppliedActions[0].Success {
		t.Error("expected success=true")
	}
	if len(ar.RippleEffects) != 1 {
		t.Fatalf("ripple_effects: got %d, want 1", len(ar.RippleEffects))
	}
	if ar.NewCompleteness != 0.50 {
		t.Errorf("new_completeness: got %f, want 0.50", ar.NewCompleteness)
	}

	// round-trip
	data, err := json.Marshal(ar)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ApplyResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if decoded.NewCompleteness != 0.50 {
		t.Errorf("round-trip completeness: got %f", decoded.NewCompleteness)
	}
}

func TestApplyResult_ActionWithError(t *testing.T) {
	raw := `{
		"wave_id": "w1",
		"applied_actions": [
			{"type": "add_dod", "issue_id": "ENG-50", "success": false, "error": "permission denied"}
		],
		"new_completeness": 0.30
	}`

	var ar ApplyResult
	if err := json.Unmarshal([]byte(raw), &ar); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ar.AppliedActions[0].Success {
		t.Error("expected success=false")
	}
	if ar.AppliedActions[0].Error != "permission denied" {
		t.Errorf("error: got %q", ar.AppliedActions[0].Error)
	}
}

func TestApplyResult_RippleEffectsOmittedWhenEmpty(t *testing.T) {
	ar := ApplyResult{WaveID: "w1", NewCompleteness: 0.5}
	data, err := json.Marshal(ar)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "ripple_effects") {
		t.Error("expected ripple_effects to be omitted when empty")
	}
}

func TestToDiscussResult_Basic(t *testing.T) {
	// given
	wave := Wave{ID: "w1", ClusterName: "Auth", Title: "Setup JWT"}
	resp := &ArchitectResponse{
		Analysis:  "JWT has trade-offs in complexity",
		Reasoning: "Session-based auth is simpler",
	}
	topic := "auth approach"

	// when
	result := ToDiscussResult(wave, resp, topic)

	// then
	if result.WaveID != "w1" {
		t.Errorf("wave_id: got %q", result.WaveID)
	}
	if result.Analysis != resp.Analysis {
		t.Errorf("analysis: got %q", result.Analysis)
	}
	if result.Reasoning != resp.Reasoning {
		t.Errorf("reasoning: got %q", result.Reasoning)
	}
	if result.Decision != topic {
		t.Errorf("decision: got %q, want %q", result.Decision, topic)
	}
}

func TestToDiscussResult_WithModifiedWave(t *testing.T) {
	// given
	wave := Wave{
		ID: "w1", ClusterName: "Auth",
		Actions: []WaveAction{
			{Type: "add_dependency", IssueID: "ENG-101", Description: "original"},
			{Type: "add_dod", IssueID: "ENG-102", Description: "unchanged"},
		},
	}
	modified := Wave{
		ID: "w1", ClusterName: "Auth",
		Actions: []WaveAction{
			{Type: "add_dependency", IssueID: "ENG-101", Description: "updated with Redis"},
			{Type: "add_dod", IssueID: "ENG-102", Description: "unchanged"},
		},
	}
	resp := &ArchitectResponse{
		Analysis:     "Redis needed",
		Reasoning:    "For session store",
		ModifiedWave: &modified,
	}

	// when
	result := ToDiscussResult(wave, resp, "session store")

	// then
	if len(result.Modifications) != 1 {
		t.Fatalf("modifications: got %d, want 1", len(result.Modifications))
	}
	if result.Modifications[0].ActionIndex != 0 {
		t.Errorf("action_index: got %d, want 0", result.Modifications[0].ActionIndex)
	}
}

func TestToDiscussResult_NilModifiedWave(t *testing.T) {
	// given
	wave := Wave{ID: "w1", ClusterName: "Auth"}
	resp := &ArchitectResponse{Analysis: "ok", Reasoning: "no changes needed"}

	// when
	result := ToDiscussResult(wave, resp, "review")

	// then
	if len(result.Modifications) != 0 {
		t.Errorf("modifications: got %d, want 0", len(result.Modifications))
	}
}

func TestToDiscussResult_WithAddedActions(t *testing.T) {
	// given: modified wave has more actions than original
	wave := Wave{
		ID: "w1", ClusterName: "Auth",
		Actions: []WaveAction{
			{Type: "add_dependency", IssueID: "ENG-101", Description: "original"},
		},
	}
	modified := Wave{
		ID: "w1", ClusterName: "Auth",
		Actions: []WaveAction{
			{Type: "add_dependency", IssueID: "ENG-101", Description: "original"},
			{Type: "add_dod", IssueID: "ENG-103", Description: "new action added by architect"},
		},
	}
	resp := &ArchitectResponse{
		Analysis:     "Additional action needed",
		Reasoning:    "Discovered missing DoD",
		ModifiedWave: &modified,
	}

	// when
	result := ToDiscussResult(wave, resp, "expand scope")

	// then: added action should be reported as a modification
	if len(result.Modifications) != 1 {
		t.Fatalf("modifications: got %d, want 1 (added action)", len(result.Modifications))
	}
	if result.Modifications[0].ActionIndex != 1 {
		t.Errorf("action_index: got %d, want 1", result.Modifications[0].ActionIndex)
	}
	if result.Modifications[0].Change == "" {
		t.Error("expected non-empty change description for added action")
	}
}

func TestToDiscussResult_WithRemovedActions(t *testing.T) {
	// given: modified wave has fewer actions than original
	wave := Wave{
		ID: "w1", ClusterName: "Auth",
		Actions: []WaveAction{
			{Type: "add_dependency", IssueID: "ENG-101", Description: "keep this"},
			{Type: "add_dod", IssueID: "ENG-102", Description: "remove this"},
		},
	}
	modified := Wave{
		ID: "w1", ClusterName: "Auth",
		Actions: []WaveAction{
			{Type: "add_dependency", IssueID: "ENG-101", Description: "keep this"},
		},
	}
	resp := &ArchitectResponse{
		Analysis:     "Simplified",
		Reasoning:    "Action not needed",
		ModifiedWave: &modified,
	}

	// when
	result := ToDiscussResult(wave, resp, "simplify")

	// then: removed action should be reported
	if len(result.Modifications) != 1 {
		t.Fatalf("modifications: got %d, want 1 (removed action)", len(result.Modifications))
	}
	if result.Modifications[0].ActionIndex != 1 {
		t.Errorf("action_index: got %d, want 1", result.Modifications[0].ActionIndex)
	}
	if result.Modifications[0].Change == "" {
		t.Error("expected non-empty change description for removed action")
	}
}

func TestToApplyResult_AllSuccess(t *testing.T) {
	// given
	wave := Wave{
		ID:          "w1",
		ClusterName: "Auth",
		Actions: []WaveAction{
			{Type: "add_dependency", IssueID: "ENG-101"},
			{Type: "add_dod", IssueID: "ENG-102"},
		},
		Delta: WaveDelta{Before: 0.30, After: 0.50},
	}
	internal := &WaveApplyResult{
		WaveID:  "w1",
		Applied: 2,
		Errors:  nil,
		Ripples: []Ripple{{ClusterName: "API", Description: "W2 unlocked"}},
	}

	// when
	result := ToApplyResult(wave, internal)

	// then
	if result.WaveID != "w1" {
		t.Errorf("wave_id: got %q", result.WaveID)
	}
	if len(result.AppliedActions) != 2 {
		t.Fatalf("applied_actions: got %d, want 2", len(result.AppliedActions))
	}
	for _, a := range result.AppliedActions {
		if !a.Success {
			t.Errorf("expected success=true for %s", a.IssueID)
		}
	}
	if len(result.RippleEffects) != 1 {
		t.Fatalf("ripple_effects: got %d, want 1", len(result.RippleEffects))
	}
	if result.NewCompleteness != 0.50 {
		t.Errorf("new_completeness: got %f, want 0.50", result.NewCompleteness)
	}
}

func TestToApplyResult_WithErrors(t *testing.T) {
	// given
	wave := Wave{
		ID:          "w1",
		ClusterName: "Auth",
		Actions: []WaveAction{
			{Type: "add_dependency", IssueID: "ENG-101"},
			{Type: "add_dod", IssueID: "ENG-102"},
		},
		Delta: WaveDelta{Before: 0.30, After: 0.50},
	}
	internal := &WaveApplyResult{
		WaveID:  "w1",
		Applied: 1,
		Errors:  []string{"permission denied on ENG-102"},
		Ripples: nil,
	}

	// when
	result := ToApplyResult(wave, internal)

	// then
	if len(result.AppliedActions) != 2 {
		t.Fatalf("applied_actions: got %d, want 2", len(result.AppliedActions))
	}
	// First action should succeed, second should fail
	if !result.AppliedActions[0].Success {
		t.Error("expected first action success=true")
	}
	if result.AppliedActions[1].Success {
		t.Error("expected second action success=false")
	}
	if result.AppliedActions[1].Error == "" {
		t.Error("expected error message on failed action")
	}
	// P2: partial apply should interpolate completeness, not use Delta.After
	// 1 of 2 succeeded → Before + (After - Before) * 0.5 = 0.30 + 0.10 = 0.40
	if result.NewCompleteness != 0.40 {
		t.Errorf("new_completeness: got %f, want 0.40 (interpolated for partial apply)", result.NewCompleteness)
	}
}

func TestToApplyResult_NoActions(t *testing.T) {
	// given
	wave := Wave{ID: "w1", ClusterName: "Auth", Delta: WaveDelta{After: 0.40}}
	internal := &WaveApplyResult{WaveID: "w1", Applied: 0}

	// when
	result := ToApplyResult(wave, internal)

	// then
	if result.AppliedActions == nil {
		t.Error("expected non-nil applied_actions (empty slice)")
	}
	if len(result.AppliedActions) != 0 {
		t.Errorf("applied_actions: got %d, want 0", len(result.AppliedActions))
	}
}

func TestToApplyResult_EmbedCompletedWave(t *testing.T) {
	// given: wave with cluster context
	wave := Wave{
		ID:          "w1",
		ClusterName: "Auth",
		Title:       "Dependency Ordering",
		Actions: []WaveAction{
			{Type: "add_dependency", IssueID: "ENG-101"},
		},
		Delta: WaveDelta{Before: 0.30, After: 0.50},
		ClusterContext: &ClusterScanResult{
			Name:         "Auth",
			Completeness: 0.30,
		},
	}
	internal := &WaveApplyResult{WaveID: "w1", Applied: 1}

	// when
	result := ToApplyResult(wave, internal)

	// then: completed wave should be embedded
	if result.CompletedWave == nil {
		t.Fatal("expected CompletedWave to be embedded in ApplyResult")
	}
	if result.CompletedWave.ID != "w1" {
		t.Errorf("CompletedWave.ID: got %q, want w1", result.CompletedWave.ID)
	}
	if result.CompletedWave.ClusterName != "Auth" {
		t.Errorf("CompletedWave.ClusterName: got %q, want Auth", result.CompletedWave.ClusterName)
	}
	if result.CompletedWave.ClusterContext == nil {
		t.Error("expected CompletedWave.ClusterContext to be preserved")
	}
	// P2 follow-up: Status must be "completed" so NeedsMoreWaves and
	// CompletedWavesForCluster treat it correctly in the pipe workflow.
	if result.CompletedWave.Status != "completed" {
		t.Errorf("CompletedWave.Status: got %q, want \"completed\"", result.CompletedWave.Status)
	}
}

func TestApplyResult_RemainingWaves_RoundTrip(t *testing.T) {
	// given: ApplyResult with remaining waves
	result := ApplyResult{
		WaveID:          "w1",
		AppliedActions:  []ActionResult{{Type: "add_dependency", IssueID: "ENG-101", Success: true}},
		NewCompleteness: 0.50,
		CompletedWave: &Wave{
			ID: "w1", ClusterName: "Auth", Status: "completed",
			Actions: []WaveAction{{Type: "add_dependency", IssueID: "ENG-101"}},
			Delta:   WaveDelta{Before: 0.30, After: 0.50},
		},
		RemainingWaves: []Wave{
			{ID: "w2", ClusterName: "Auth", Status: "available", Title: "Token lifecycle"},
			{ID: "w3", ClusterName: "Auth", Status: "locked", Title: "Audit logging"},
		},
	}

	// when: marshal + unmarshal
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ApplyResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then: remaining waves survive round-trip
	if len(decoded.RemainingWaves) != 2 {
		t.Fatalf("RemainingWaves: got %d, want 2", len(decoded.RemainingWaves))
	}
	if decoded.RemainingWaves[0].ID != "w2" {
		t.Errorf("RemainingWaves[0].ID: got %q, want w2", decoded.RemainingWaves[0].ID)
	}
	if decoded.RemainingWaves[1].Status != "locked" {
		t.Errorf("RemainingWaves[1].Status: got %q, want locked", decoded.RemainingWaves[1].Status)
	}

	// Verify NeedsMoreWaves sees the full picture.
	allWaves := append([]Wave{*decoded.CompletedWave}, decoded.RemainingWaves...)
	cluster := ClusterScanResult{Name: "Auth", Completeness: 0.50}
	if NeedsMoreWaves(cluster, allWaves) {
		t.Error("NeedsMoreWaves should return false when available waves remain")
	}
}

// --- Schema example file round-trip tests ---

func TestSchemaExamples_RoundTrip(t *testing.T) {
	schemasDir := filepath.Join("docs", "schemas")

	tests := []struct {
		file   string
		target any
	}{
		{"scan_result.json", &ScanResult{}},
		{"wave_plan.json", &WavePlan{}},
		{"wave.json", &Wave{}},
		{"discuss_result.json", &DiscussResult{}},
		{"apply_result.json", &ApplyResult{}},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			path := filepath.Join(schemasDir, tt.file)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", tt.file, err)
			}

			// unmarshal
			if err := json.Unmarshal(data, tt.target); err != nil {
				t.Fatalf("unmarshal %s: %v", tt.file, err)
			}

			// re-marshal
			redata, err := json.Marshal(tt.target)
			if err != nil {
				t.Fatalf("re-marshal %s: %v", tt.file, err)
			}

			// re-unmarshal (round-trip)
			if err := json.Unmarshal(redata, tt.target); err != nil {
				t.Fatalf("round-trip unmarshal %s: %v", tt.file, err)
			}
		})
	}
}
