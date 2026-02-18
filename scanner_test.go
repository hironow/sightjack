package sightjack

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
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
	result, err := ParseClassifyResult(path)

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
	result, err := ParseClusterScanResult(path)

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
			got := sanitizeName(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestClusterFileName_UniqueForCollisions(t *testing.T) {
	// given: two names that sanitize to the same string
	name1 := clusterFileName(0, "API Backend")
	name2 := clusterFileName(1, "API/Backend")

	// then: filenames must differ despite identical sanitized names
	if name1 == name2 {
		t.Errorf("expected unique filenames, both got %q", name1)
	}

	// and: filenames contain the sanitized name for readability
	if name1 != "cluster_00_api_backend.json" {
		t.Errorf("unexpected filename format: %s", name1)
	}
	if name2 != "cluster_01_api_backend.json" {
		t.Errorf("unexpected filename format: %s", name2)
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
			got := chunkSlice(tt.items, tt.size)
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
	chunks := []ClusterScanResult{
		{
			Name:         "Auth",
			Completeness: 0.4,
			Issues: []IssueDetail{
				{ID: "1", Completeness: 0.3},
				{ID: "2", Completeness: 0.5},
			},
			Observations: []string{"obs1"},
		},
		{
			Name:         "Auth",
			Completeness: 0.6,
			Issues: []IssueDetail{
				{ID: "3", Completeness: 0.7},
			},
			Observations: []string{"obs2"},
		},
	}

	// when
	merged := mergeClusterChunks("Auth", chunks)

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
	chunks := []ClusterScanResult{
		{
			Name:         "API",
			Completeness: 0.80,
			Issues: []IssueDetail{
				{ID: "1", Completeness: 0.5},
				{ID: "2", Completeness: 1.0},
			},
		},
	}

	// when
	merged := mergeClusterChunks("API", chunks)

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
	chunks := []ClusterScanResult{
		{Name: "auth & login", Completeness: 0.5, Issues: make([]IssueDetail, 3)},
	}

	// when: canonical name from pass-1 is "Auth"
	merged := mergeClusterChunks("Auth", chunks)

	// then: canonical name must win
	if merged.Name != "Auth" {
		t.Errorf("expected canonical name 'Auth', got %q", merged.Name)
	}
}

func TestRunWaveGenerate_ParsesResults(t *testing.T) {
	// given: mock wave generation output files
	dir := t.TempDir()
	wave0 := filepath.Join(dir, "wave_00_auth.json")
	wave1 := filepath.Join(dir, "wave_01_api.json")

	os.WriteFile(wave0, []byte(`{
		"cluster_name": "Auth",
		"waves": [
			{"id": "auth-w1", "cluster_name": "Auth", "title": "Deps", "actions": [], "prerequisites": [], "delta": {"before": 0.25, "after": 0.40}, "status": "available"}
		]
	}`), 0644)
	os.WriteFile(wave1, []byte(`{
		"cluster_name": "API",
		"waves": [
			{"id": "api-w1", "cluster_name": "API", "title": "Split", "actions": [], "prerequisites": [], "delta": {"before": 0.30, "after": 0.45}, "status": "available"}
		]
	}`), 0644)

	// when: parse both files
	result0, err := ParseWaveGenerateResult(wave0)
	if err != nil {
		t.Fatalf("parse wave 0: %v", err)
	}
	result1, err := ParseWaveGenerateResult(wave1)
	if err != nil {
		t.Fatalf("parse wave 1: %v", err)
	}

	// then: merge waves
	allWaves := MergeWaveResults([]WaveGenerateResult{*result0, *result1})
	if len(allWaves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(allWaves))
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
	result, err := ParseClassifyResult(path)

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

func TestMergeScanResults_PropagatesShibitoWarnings(t *testing.T) {
	// given
	clusters := []ClusterScanResult{
		{Name: "Auth", Completeness: 0.25, Issues: make([]IssueDetail, 3)},
	}
	warnings := []ShibitoWarning{
		{ClosedIssueID: "ENG-50", CurrentIssueID: "ENG-120", Description: "pattern", RiskLevel: "high"},
	}

	// when
	result := MergeScanResults(clusters, warnings, nil)

	// then
	if len(result.ShibitoWarnings) != 1 {
		t.Fatalf("expected 1 shibito warning, got %d", len(result.ShibitoWarnings))
	}
	if result.ShibitoWarnings[0].ClosedIssueID != "ENG-50" {
		t.Errorf("expected ENG-50, got %s", result.ShibitoWarnings[0].ClosedIssueID)
	}
}

func TestMergeScanResults(t *testing.T) {
	// given
	clusters := []ClusterScanResult{
		{Name: "Auth", Completeness: 0.25, Issues: make([]IssueDetail, 3)},
		{Name: "API", Completeness: 0.50, Issues: make([]IssueDetail, 7)},
	}

	// when
	result := MergeScanResults(clusters, nil, nil)

	// then
	if result.TotalIssues != 10 {
		t.Errorf("expected 10, got %d", result.TotalIssues)
	}
	if result.Completeness != 0.375 {
		t.Errorf("expected 0.375, got %f", result.Completeness)
	}
	if len(result.Clusters) != 2 {
		t.Errorf("expected 2 clusters, got %d", len(result.Clusters))
	}
}

func TestMergeScanResults_WithScanWarnings(t *testing.T) {
	// given: partial scan success — some clusters failed
	clusters := []ClusterScanResult{
		{Name: "Auth", Completeness: 0.5, Issues: make([]IssueDetail, 3)},
	}
	scanWarnings := []string{`Cluster "Infra" scan failed: timeout`}

	// when
	result := MergeScanResults(clusters, nil, scanWarnings)

	// then
	if len(result.ScanWarnings) != 1 {
		t.Fatalf("expected 1 scan warning, got %d", len(result.ScanWarnings))
	}
	if result.ScanWarnings[0] != scanWarnings[0] {
		t.Errorf("expected %q, got %q", scanWarnings[0], result.ScanWarnings[0])
	}
}

func TestRunParallelDeepScan(t *testing.T) {
	// given
	clusters := []ClusterScanResult{
		{Name: "auth", Issues: []IssueDetail{{ID: "A-1"}}},
		{Name: "infra", Issues: []IssueDetail{{ID: "I-1"}}},
		{Name: "frontend", Issues: []IssueDetail{{ID: "F-1"}}},
	}

	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Scan.MaxConcurrency = 2

	// when
	results, warnings := RunParallelDeepScan(context.Background(), &cfg, dir, clusters,
		func(ctx context.Context, cfg *Config, scanDir string, index int, cluster ClusterScanResult) (ClusterScanResult, error) {
			return ClusterScanResult{Name: cluster.Name, Completeness: 0.5}, nil
		})

	// then
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(warnings))
	}
}

func TestRunParallelDeepScanWithFailure(t *testing.T) {
	// given
	clusters := []ClusterScanResult{
		{Name: "auth"},
		{Name: "infra"},
	}

	dir := t.TempDir()
	cfg := DefaultConfig()
	var callCount atomic.Int32

	// when
	results, warnings := RunParallelDeepScan(context.Background(), &cfg, dir, clusters,
		func(ctx context.Context, cfg *Config, scanDir string, index int, cluster ClusterScanResult) (ClusterScanResult, error) {
			callCount.Add(1)
			if cluster.Name == "auth" {
				return ClusterScanResult{}, fmt.Errorf("auth scan failed")
			}
			return ClusterScanResult{Name: cluster.Name, Completeness: 0.7}, nil
		})

	// then
	if len(results) != 1 {
		t.Errorf("expected 1 successful result, got %d", len(results))
	}
	if len(results) > 0 && results[0].Name != "infra" {
		t.Errorf("expected 'infra', got %q", results[0].Name)
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}
	if callCount.Load() != 2 {
		t.Errorf("expected 2 calls, got %d", callCount.Load())
	}
}

func TestRunParallelDeepScanSingleCluster(t *testing.T) {
	// given
	clusters := []ClusterScanResult{{Name: "only"}}
	dir := t.TempDir()
	cfg := DefaultConfig()

	// when
	results, _ := RunParallelDeepScan(context.Background(), &cfg, dir, clusters,
		func(ctx context.Context, cfg *Config, scanDir string, index int, cluster ClusterScanResult) (ClusterScanResult, error) {
			return ClusterScanResult{Name: cluster.Name, Completeness: 1.0}, nil
		})

	// then
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestRunParallelDeepScan_IndexBasedLookup(t *testing.T) {
	// given: classify result with cluster names and issue IDs (simulates Pass 1 output)
	classifyClusters := []ClusterClassification{
		{Name: "Auth", IssueIDs: []string{"A-1", "A-2"}},
		{Name: "Infra", IssueIDs: []string{"I-1"}},
	}

	scanClusters := make([]ClusterScanResult, len(classifyClusters))
	for i, cc := range classifyClusters {
		scanClusters[i] = ClusterScanResult{Name: cc.Name}
	}

	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Scan.MaxConcurrency = 2

	// when: use index-based lookup (same pattern as wired in RunScan)
	results, warnings := RunParallelDeepScan(context.Background(), &cfg, dir, scanClusters,
		func(ctx context.Context, cfg *Config, scanDir string, index int, cluster ClusterScanResult) (ClusterScanResult, error) {
			cc := classifyClusters[index]
			return ClusterScanResult{
				Name:         cc.Name,
				Completeness: 0.5,
				Issues:       make([]IssueDetail, len(cc.IssueIDs)),
			}, nil
		})

	// then
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d: %v", len(warnings), warnings)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	nameSet := make(map[string]bool)
	for _, r := range results {
		nameSet[r.Name] = true
	}
	if !nameSet["Auth"] {
		t.Error("expected Auth cluster in results")
	}
	if !nameSet["Infra"] {
		t.Error("expected Infra cluster in results")
	}
}

func TestRunParallelDeepScan_DuplicateClusterNames(t *testing.T) {
	// given: classifier returns duplicate cluster names with different issue IDs
	classifyClusters := []ClusterClassification{
		{Name: "Auth", IssueIDs: []string{"A-1", "A-2"}},
		{Name: "Auth", IssueIDs: []string{"A-3"}},
	}

	scanClusters := make([]ClusterScanResult, len(classifyClusters))
	for i, cc := range classifyClusters {
		scanClusters[i] = ClusterScanResult{Name: cc.Name}
	}

	dir := t.TempDir()
	cfg := DefaultConfig()

	// when: index-based lookup ensures each duplicate gets its own issue IDs
	results, warnings := RunParallelDeepScan(context.Background(), &cfg, dir, scanClusters,
		func(ctx context.Context, cfg *Config, scanDir string, index int, cluster ClusterScanResult) (ClusterScanResult, error) {
			cc := classifyClusters[index]
			return ClusterScanResult{
				Name:         cc.Name,
				Completeness: float64(len(cc.IssueIDs)) * 0.25,
				Issues:       make([]IssueDetail, len(cc.IssueIDs)),
			}, nil
		})

	// then: both clusters scanned with correct issue counts (order is non-deterministic)
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(warnings))
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Collect completeness values (order may vary)
	completenessSet := make(map[float64]int)
	issueCountSet := make(map[int]int)
	for _, r := range results {
		completenessSet[r.Completeness]++
		issueCountSet[len(r.Issues)]++
	}
	// One Auth has 2 issues (completeness 0.5), the other has 1 issue (completeness 0.25)
	if completenessSet[0.5] != 1 {
		t.Errorf("expected one result with completeness 0.5, got %d", completenessSet[0.5])
	}
	if completenessSet[0.25] != 1 {
		t.Errorf("expected one result with completeness 0.25, got %d", completenessSet[0.25])
	}
	if issueCountSet[2] != 1 {
		t.Errorf("expected one result with 2 issues, got %d", issueCountSet[2])
	}
	if issueCountSet[1] != 1 {
		t.Errorf("expected one result with 1 issue, got %d", issueCountSet[1])
	}
}
