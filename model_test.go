package sightjack

import (
	"encoding/json"
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
