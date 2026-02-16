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
