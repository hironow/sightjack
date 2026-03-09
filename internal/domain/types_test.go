package domain_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestClusterScanResult_Key(t *testing.T) {
	c := domain.ClusterScanResult{
		Name: "Auth Module",
		Key:  "auth-module",
	}
	if c.Key != "auth-module" {
		t.Errorf("expected auth-module, got %s", c.Key)
	}
}

func TestStrictnessKeys_IncludesClusterKey(t *testing.T) {
	r := &domain.ScanResult{
		Clusters: []domain.ClusterScanResult{
			{Name: "Auth Module", Key: "auth-module", Labels: []string{"security"}},
		},
	}
	keys := r.StrictnessKeys("Auth Module")
	// Should be: ["Auth Module", "auth-module", "security"]
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d: %v", len(keys), keys)
	}
	if keys[1] != "auth-module" {
		t.Errorf("expected auth-module at index 1, got %s", keys[1])
	}
}

func TestIssueDetail_HasStatus(t *testing.T) {
	issue := domain.IssueDetail{
		ID:           "issue-1",
		Identifier:   "MY-123",
		Title:        "Auth flow",
		Status:       "in_progress",
		Completeness: 0.6,
		Gaps:         []string{"DoD missing"},
	}
	if issue.Status != "in_progress" {
		t.Errorf("expected in_progress, got %s", issue.Status)
	}
}

func TestClusterScanResult_EstimatedStrictness(t *testing.T) {
	c := domain.ClusterScanResult{
		Name:                "Auth Module",
		Key:                 "auth-module",
		EstimatedStrictness: "alert",
		StrictnessReasoning: "Done 60%, In Progress 15%. Core auth has tight coupling.",
	}
	if c.EstimatedStrictness != "alert" {
		t.Errorf("expected alert, got %s", c.EstimatedStrictness)
	}
	if c.StrictnessReasoning == "" {
		t.Error("expected non-empty reasoning")
	}
}
