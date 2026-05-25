package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness"
	"github.com/hironow/sightjack/internal/session"
)

func TestParseClassifyResult(t *testing.T) {
	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "classify.json")
	content := `{
		"clusters": [
			{"name": "Auth", "issue_ids": ["id1", "id2"]},
			{"name": "API", "issue_ids": ["id3"]}
		],
		"total_issues": 3
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	result, err := session.ParseClassifyResult(path)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(result.Clusters))
	}
	if result.TotalIssues != 3 {
		t.Errorf("expected 3, got %d", result.TotalIssues)
	}
}

func TestParseClassifyResult_WithLabels(t *testing.T) {
	// given: classify output includes labels per cluster
	dir := t.TempDir()
	path := filepath.Join(dir, "classify.json")
	content := `{
		"clusters": [
			{"name": "Auth", "issue_ids": ["id1"], "labels": ["security", "backend"]},
			{"name": "UI", "issue_ids": ["id2"], "labels": ["frontend"]}
		],
		"total_issues": 2
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	result, err := session.ParseClassifyResult(path)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Clusters[0].Labels) != 2 {
		t.Fatalf("expected 2 labels for Auth, got %d", len(result.Clusters[0].Labels))
	}
	if result.Clusters[0].Labels[0] != "security" {
		t.Errorf("expected first label 'security', got %s", result.Clusters[0].Labels[0])
	}
	if len(result.Clusters[1].Labels) != 1 {
		t.Fatalf("expected 1 label for UI, got %d", len(result.Clusters[1].Labels))
	}
}

func TestParseClusterScanResult(t *testing.T) {
	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "cluster_auth.json")
	content := `{
		"name": "Auth",
		"completeness": 0.35,
		"issues": [
			{
				"id": "abc",
				"identifier": "AWE-50",
				"title": "Login",
				"completeness": 0.4,
				"gaps": ["DoD missing"]
			}
		],
		"observations": ["Depends on API"]
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	result, err := session.ParseClusterScanResult(path)

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
}

func TestParseClassifyResult_WithShibitoWarnings(t *testing.T) {
	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "classify.json")
	content := `{
		"clusters": [
			{"name": "Auth", "issue_ids": ["id1"]}
		],
		"total_issues": 1,
		"shibito_warnings": [
			{
				"closed_issue_id": "ENG-50",
				"current_issue_id": "ENG-120",
				"description": "Login timeout pattern re-emerging",
				"risk_level": "high"
			}
		]
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	result, err := session.ParseClassifyResult(path)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ShibitoWarnings) != 1 {
		t.Fatalf("expected 1 shibito warning, got %d", len(result.ShibitoWarnings))
	}
	if result.ShibitoWarnings[0].ClosedIssueID != "ENG-50" {
		t.Errorf("expected ENG-50, got %s", result.ShibitoWarnings[0].ClosedIssueID)
	}
	if result.ShibitoWarnings[0].RiskLevel != "high" {
		t.Errorf("expected high, got %s", result.ShibitoWarnings[0].RiskLevel)
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Auth", "auth"},
		{"API Gateway", "api_gateway"},
		{"API/Backend", "api_backend"},
		{"Front-End", "front-end"},
		{"Data & Analytics", "data___analytics"},
		{"cluster:main", "cluster_main"},
		{"日本語クラスタ", "_______"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := domain.SanitizeName(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestChunkSlice(t *testing.T) {
	tests := []struct {
		name     string
		items    []string
		size     int
		expected int // number of chunks
		lastLen  int // length of last chunk
	}{
		{"exact division", []string{"a", "b", "c", "d"}, 2, 2, 2},
		{"remainder", []string{"a", "b", "c"}, 2, 2, 1},
		{"single chunk", []string{"a", "b"}, 5, 1, 2},
		{"empty", []string{}, 2, 0, 0},
		{"one item", []string{"a"}, 3, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.ChunkSlice(tt.items, tt.size)
			if len(got) != tt.expected {
				t.Fatalf("expected %d chunks, got %d", tt.expected, len(got))
			}
			if tt.expected > 0 {
				last := got[len(got)-1]
				if len(last) != tt.lastLen {
					t.Errorf("last chunk: expected %d items, got %d", tt.lastLen, len(last))
				}
			}
		})
	}
}

func TestMergeClusterChunks(t *testing.T) {
	// given: two chunks from the same cluster
	chunks := []domain.ClusterScanResult{
		{
			Name:         "Auth",
			Completeness: 0.4,
			Issues: []domain.IssueDetail{
				{ID: "1", Completeness: 0.3},
				{ID: "2", Completeness: 0.5},
			},
			Observations: []string{"obs1"},
		},
		{
			Name:         "Auth",
			Completeness: 0.6,
			Issues: []domain.IssueDetail{
				{ID: "3", Completeness: 0.7},
			},
			Observations: []string{"obs2"},
		},
	}

	// when
	merged := harness.MergeClusterChunks("Auth", chunks)

	// then
	if merged.Name != "Auth" {
		t.Errorf("expected Auth, got %s", merged.Name)
	}
	if len(merged.Issues) != 3 {
		t.Errorf("expected 3 issues, got %d", len(merged.Issues))
	}
	if len(merged.Observations) != 2 {
		t.Errorf("expected 2 observations, got %d", len(merged.Observations))
	}
	// Completeness = (0.3 + 0.5 + 0.7) / 3 = 0.5
	if merged.Completeness != 0.5 {
		t.Errorf("expected completeness 0.5, got %f", merged.Completeness)
	}
}

func TestMergeClusterChunks_SingleChunk(t *testing.T) {
	// given: single chunk where Claude's top-level completeness differs from per-issue average
	// Claude returned 0.80 (rounded) but individual issues average to 0.75
	chunks := []domain.ClusterScanResult{
		{
			Name:         "API",
			Completeness: 0.80,
			Issues: []domain.IssueDetail{
				{ID: "1", Completeness: 0.5},
				{ID: "2", Completeness: 1.0},
			},
		},
	}

	// when
	merged := harness.MergeClusterChunks("API", chunks)

	// then: completeness must be recomputed from issues, not Claude's top-level value
	expectedCompleteness := 0.75 // (0.5 + 1.0) / 2
	if merged.Completeness != expectedCompleteness {
		t.Errorf("expected recomputed completeness %f, got %f", expectedCompleteness, merged.Completeness)
	}
	if len(merged.Issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(merged.Issues))
	}
}

func TestMergeClusterChunks_SingleChunk_CanonicalName(t *testing.T) {
	// given: Claude returned a slightly different name than pass-1 classification
	chunks := []domain.ClusterScanResult{
		{Name: "auth & login", Completeness: 0.5, Issues: make([]domain.IssueDetail, 3)},
	}

	// when: canonical name from pass-1 is "Auth"
	merged := harness.MergeClusterChunks("Auth", chunks)

	// then: canonical name must win
	if merged.Name != "Auth" {
		t.Errorf("expected canonical name 'Auth', got %q", merged.Name)
	}
}

func TestDetectFailedClusterNames(t *testing.T) {
	tests := []struct {
		name      string
		clusters  []domain.ClusterScanResult
		successes []domain.WaveGenerateResult
		want      map[string]bool
	}{
		{
			name:      "all succeed no duplicates",
			clusters:  []domain.ClusterScanResult{{Name: "Auth"}, {Name: "DB"}},
			successes: []domain.WaveGenerateResult{{ClusterName: "Auth"}, {ClusterName: "DB"}},
			want:      map[string]bool{},
		},
		{
			name:      "one fails no duplicates",
			clusters:  []domain.ClusterScanResult{{Name: "Auth"}, {Name: "DB"}},
			successes: []domain.WaveGenerateResult{{ClusterName: "Auth"}},
			want:      map[string]bool{"DB": true},
		},
		{
			name:      "duplicates all succeed",
			clusters:  []domain.ClusterScanResult{{Name: "Auth"}, {Name: "Auth"}, {Name: "DB"}},
			successes: []domain.WaveGenerateResult{{ClusterName: "Auth"}, {ClusterName: "Auth"}, {ClusterName: "DB"}},
			want:      map[string]bool{},
		},
		{
			name:      "duplicates partial failure",
			clusters:  []domain.ClusterScanResult{{Name: "Auth"}, {Name: "Auth"}, {Name: "DB"}},
			successes: []domain.WaveGenerateResult{{ClusterName: "Auth"}, {ClusterName: "DB"}},
			want:      map[string]bool{"Auth": true},
		},
		{
			name:      "all fail",
			clusters:  []domain.ClusterScanResult{{Name: "Auth"}, {Name: "DB"}},
			successes: []domain.WaveGenerateResult{},
			want:      map[string]bool{"Auth": true, "DB": true},
		},
		{
			name:      "empty input",
			clusters:  []domain.ClusterScanResult{},
			successes: []domain.WaveGenerateResult{},
			want:      map[string]bool{},
		},
		{
			name:      "empty cluster name in success is not counted",
			clusters:  []domain.ClusterScanResult{{Name: "Auth"}},
			successes: []domain.WaveGenerateResult{{ClusterName: ""}},
			want:      map[string]bool{"Auth": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := harness.DetectFailedClusterNames(tt.clusters, tt.successes)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d failed names, got %d: %v", len(tt.want), len(got), got)
			}
			for name := range tt.want {
				if !got[name] {
					t.Errorf("expected %q in failed names", name)
				}
			}
		})
	}
}
