package session_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
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
	merged := domain.MergeClusterChunks("Auth", chunks)

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
	merged := domain.MergeClusterChunks("API", chunks)

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
	merged := domain.MergeClusterChunks("Auth", chunks)

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
	result0, err := session.ParseWaveGenerateResult(wave0)
	if err != nil {
		t.Fatalf("parse wave 0: %v", err)
	}
	result1, err := session.ParseWaveGenerateResult(wave1)
	if err != nil {
		t.Fatalf("parse wave 1: %v", err)
	}

	// then: merge waves
	allWaves := domain.MergeWaveResults([]domain.WaveGenerateResult{*result0, *result1})
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

func TestMergeScanResults_PropagatesShibitoWarnings(t *testing.T) {
	// given
	clusters := []domain.ClusterScanResult{
		{Name: "Auth", Completeness: 0.25, Issues: make([]domain.IssueDetail, 3)},
	}
	warnings := []domain.ShibitoWarning{
		{ClosedIssueID: "ENG-50", CurrentIssueID: "ENG-120", Description: "pattern", RiskLevel: "high"},
	}

	// when
	result := session.MergeScanResults(clusters, warnings, nil)

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
	clusters := []domain.ClusterScanResult{
		{Name: "Auth", Completeness: 0.25, Issues: make([]domain.IssueDetail, 3)},
		{Name: "API", Completeness: 0.50, Issues: make([]domain.IssueDetail, 7)},
	}

	// when
	result := session.MergeScanResults(clusters, nil, nil)

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
	clusters := []domain.ClusterScanResult{
		{Name: "Auth", Completeness: 0.5, Issues: make([]domain.IssueDetail, 3)},
	}
	scanWarnings := []string{`Cluster "Infra" scan failed: timeout`}

	// when
	result := session.MergeScanResults(clusters, nil, scanWarnings)

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
	clusters := []domain.ClusterScanResult{
		{Name: "auth", Issues: []domain.IssueDetail{{ID: "A-1"}}},
		{Name: "infra", Issues: []domain.IssueDetail{{ID: "I-1"}}},
		{Name: "frontend", Issues: []domain.IssueDetail{{ID: "F-1"}}},
	}

	dir := t.TempDir()
	cfg := domain.DefaultConfig()
	cfg.Scan.MaxConcurrency = 2

	// when
	results, warnings := session.RunParallelDeepScan(context.Background(), &cfg, dir, clusters,
		func(ctx context.Context, cfg *domain.Config, scanDir string, index int, cluster domain.ClusterScanResult) (domain.ClusterScanResult, error) {
			return domain.ClusterScanResult{Name: cluster.Name, Completeness: 0.5}, nil
		}, domain.NewLogger(io.Discard, false))

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
	clusters := []domain.ClusterScanResult{
		{Name: "auth"},
		{Name: "infra"},
	}

	dir := t.TempDir()
	cfg := domain.DefaultConfig()
	var callCount atomic.Int32

	// when
	results, warnings := session.RunParallelDeepScan(context.Background(), &cfg, dir, clusters,
		func(ctx context.Context, cfg *domain.Config, scanDir string, index int, cluster domain.ClusterScanResult) (domain.ClusterScanResult, error) {
			callCount.Add(1)
			if cluster.Name == "auth" {
				return domain.ClusterScanResult{}, fmt.Errorf("auth scan failed")
			}
			return domain.ClusterScanResult{Name: cluster.Name, Completeness: 0.7}, nil
		}, domain.NewLogger(io.Discard, false))

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
	clusters := []domain.ClusterScanResult{{Name: "only"}}
	dir := t.TempDir()
	cfg := domain.DefaultConfig()

	// when
	results, _ := session.RunParallelDeepScan(context.Background(), &cfg, dir, clusters,
		func(ctx context.Context, cfg *domain.Config, scanDir string, index int, cluster domain.ClusterScanResult) (domain.ClusterScanResult, error) {
			return domain.ClusterScanResult{Name: cluster.Name, Completeness: 1.0}, nil
		}, domain.NewLogger(io.Discard, false))

	// then
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestRunParallelDeepScan_IndexBasedLookup(t *testing.T) {
	// given: classify result with cluster names and issue IDs (simulates Pass 1 output)
	classifyClusters := []domain.ClusterClassification{
		{Name: "Auth", IssueIDs: []string{"A-1", "A-2"}},
		{Name: "Infra", IssueIDs: []string{"I-1"}},
	}

	scanClusters := make([]domain.ClusterScanResult, len(classifyClusters))
	for i, cc := range classifyClusters {
		scanClusters[i] = domain.ClusterScanResult{Name: cc.Name}
	}

	dir := t.TempDir()
	cfg := domain.DefaultConfig()
	cfg.Scan.MaxConcurrency = 2

	// when: use index-based lookup (same pattern as wired in RunScan)
	results, warnings := session.RunParallelDeepScan(context.Background(), &cfg, dir, scanClusters,
		func(ctx context.Context, cfg *domain.Config, scanDir string, index int, cluster domain.ClusterScanResult) (domain.ClusterScanResult, error) {
			cc := classifyClusters[index]
			return domain.ClusterScanResult{
				Name:         cc.Name,
				Completeness: 0.5,
				Issues:       make([]domain.IssueDetail, len(cc.IssueIDs)),
			}, nil
		}, domain.NewLogger(io.Discard, false))

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
	classifyClusters := []domain.ClusterClassification{
		{Name: "Auth", IssueIDs: []string{"A-1", "A-2"}},
		{Name: "Auth", IssueIDs: []string{"A-3"}},
	}

	scanClusters := make([]domain.ClusterScanResult, len(classifyClusters))
	for i, cc := range classifyClusters {
		scanClusters[i] = domain.ClusterScanResult{Name: cc.Name}
	}

	dir := t.TempDir()
	cfg := domain.DefaultConfig()

	// when: index-based lookup ensures each duplicate gets its own issue IDs
	results, warnings := session.RunParallelDeepScan(context.Background(), &cfg, dir, scanClusters,
		func(ctx context.Context, cfg *domain.Config, scanDir string, index int, cluster domain.ClusterScanResult) (domain.ClusterScanResult, error) {
			cc := classifyClusters[index]
			return domain.ClusterScanResult{
				Name:         cc.Name,
				Completeness: float64(len(cc.IssueIDs)) * 0.25,
				Issues:       make([]domain.IssueDetail, len(cc.IssueIDs)),
			}, nil
		}, domain.NewLogger(io.Discard, false))

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

func TestRunParallelDeepScan_ContextCancellation(t *testing.T) {
	// given: 5 clusters but context is already cancelled
	clusters := make([]domain.ClusterScanResult, 5)
	for i := range clusters {
		clusters[i] = domain.ClusterScanResult{Name: fmt.Sprintf("c%d", i)}
	}

	dir := t.TempDir()
	cfg := domain.DefaultConfig()
	cfg.Scan.MaxConcurrency = 1

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	var callCount atomic.Int32

	// when
	results, _ := session.RunParallelDeepScan(ctx, &cfg, dir, clusters,
		func(ctx context.Context, cfg *domain.Config, scanDir string, index int, cluster domain.ClusterScanResult) (domain.ClusterScanResult, error) {
			callCount.Add(1)
			return domain.ClusterScanResult{Name: cluster.Name}, nil
		}, domain.NewLogger(io.Discard, false))

	// then: no goroutines should have been launched
	if callCount.Load() != 0 {
		t.Errorf("expected 0 calls with cancelled context, got %d", callCount.Load())
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRunScan_SavesPromptAndStreamsLog(t *testing.T) {
	// given: fake claude that outputs chunks incrementally and writes classify.json
	baseDir := t.TempDir()
	sessionID := "test-stream"
	scanDir := domain.ScanDir(baseDir, sessionID)

	classifyResult := `{"clusters":[{"name":"Auth","issue_ids":["T-1"]}],"total_issues":1}`
	deepScanResult := `{"name":"Auth","completeness":0.8,"issues":[{"id":"T-1","title":"test","completeness":0.8}],"observations":["ok"]}`

	callCount := 0
	cleanup := session.SetNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		// Extract the prompt from -p argument.
		prompt := ""
		for i, a := range args {
			if a == "-p" && i+1 < len(args) {
				prompt = args[i+1]
				break
			}
		}

		// Helper: find a .json path embedded in the prompt (inside **bold**).
		findJSONPath := func(p string) string {
			for _, line := range strings.Split(p, "\n") {
				if idx := strings.Index(line, scanDir); idx >= 0 {
					sub := line[idx:]
					if end := strings.Index(sub, "**"); end > 0 {
						return sub[:end]
					}
					return strings.TrimRight(strings.TrimSpace(sub), " *`\"")
				}
			}
			return ""
		}

		if callCount == 1 {
			// Classify: stream chunks with delays, then write classify.json.
			classifyJSON := filepath.Join(scanDir, "classify.json")
			script := fmt.Sprintf(
				`printf "chunk1\n" && sleep 0.05 && printf "chunk2\n" && sleep 0.05 && printf '%s' > '%s' && printf "chunk3\n"`,
				classifyResult, classifyJSON,
			)
			_ = prompt
			return exec.CommandContext(ctx, "sh", "-c", script)
		}
		// Deep scan / wave: write result to the output path found in prompt.
		if outPath := findJSONPath(prompt); outPath != "" {
			script := fmt.Sprintf(`printf '%s' > '%s' && echo "done"`, deepScanResult, outPath)
			return exec.CommandContext(ctx, "sh", "-c", script)
		}
		return exec.CommandContext(ctx, "echo", "ok")
	})
	defer cleanup()

	cfg := domain.DefaultConfig()
	cfg.Linear.Team = "TEST"
	cfg.Linear.Project = "test-project"
	cfg.Claude.TimeoutSec = 30
	cfg.Retry.MaxAttempts = 1
	cfg.Retry.BaseDelaySec = 0
	cfg.Labels.Enabled = false // avoid RunClaudeOnce label path complexity

	// when
	result, err := session.RunScan(context.Background(), &cfg, baseDir, sessionID, false, io.Discard, domain.NewLogger(io.Discard, false))

	// then: scan should succeed
	if err != nil {
		t.Fatalf("RunScan failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// then: classify_prompt.md saved before execution
	promptData, err := os.ReadFile(filepath.Join(scanDir, "classify_prompt.md"))
	if err != nil {
		t.Fatalf("classify_prompt.md not found: %v", err)
	}
	if len(promptData) == 0 {
		t.Error("classify_prompt.md is empty")
	}
	if !strings.Contains(string(promptData), "TEST") {
		t.Error("classify_prompt.md does not contain team filter")
	}

	// then: classify_output.log contains streamed chunks
	logData, err := os.ReadFile(filepath.Join(scanDir, "classify_output.log"))
	if err != nil {
		t.Fatalf("classify_output.log not found: %v", err)
	}
	logStr := string(logData)
	for _, chunk := range []string{"chunk1", "chunk2", "chunk3"} {
		if !strings.Contains(logStr, chunk) {
			t.Errorf("classify_output.log missing %q, got: %q", chunk, logStr)
		}
	}
}

// writeRecorder counts individual Write calls to verify incremental streaming.
type writeRecorder struct {
	buf   strings.Builder
	calls int
}

func (w *writeRecorder) Write(p []byte) (int, error) {
	w.calls++
	return w.buf.Write(p)
}

func TestRunScan_StreamsIncrementally(t *testing.T) {
	// given: fake claude that outputs chunks with a deliberate delay between them.
	// Instead of polling file sizes (timing-dependent), we pass a writeRecorder
	// as the out writer. io.MultiWriter(out, logFile) calls out.Write for each
	// chunk that arrives from the subprocess pipe, so recording Write calls
	// deterministically proves incremental streaming.
	baseDir := t.TempDir()
	sessionID := "test-incremental"
	scanDir := domain.ScanDir(baseDir, sessionID)

	classifyResult := `{"clusters":[{"name":"X","issue_ids":["T-1"]}],"total_issues":1}`
	deepScanResult := `{"name":"X","completeness":1.0,"issues":[{"id":"T-1","title":"t","completeness":1.0}],"observations":[]}`

	callCount := 0
	cleanup := session.SetNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		prompt := ""
		for i, a := range args {
			if a == "-p" && i+1 < len(args) {
				prompt = args[i+1]
				break
			}
		}
		findJSONPath := func(p string) string {
			for _, line := range strings.Split(p, "\n") {
				if idx := strings.Index(line, scanDir); idx >= 0 {
					sub := line[idx:]
					if end := strings.Index(sub, "**"); end > 0 {
						return sub[:end]
					}
					return strings.TrimRight(strings.TrimSpace(sub), " *`\"")
				}
			}
			return ""
		}
		if callCount == 1 {
			// Classify: emit two chunks separated by sleep to force separate Read calls on the pipe.
			classifyJSON := filepath.Join(scanDir, "classify.json")
			script := fmt.Sprintf(
				`printf "MARKER_A\n" && sleep 0.1 && printf "MARKER_B\n" && printf '%s' > '%s'`,
				classifyResult, classifyJSON,
			)
			_ = prompt
			return exec.CommandContext(ctx, "sh", "-c", script)
		}
		if outPath := findJSONPath(prompt); outPath != "" {
			return exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf(`printf '%s' > '%s'`, deepScanResult, outPath))
		}
		return exec.CommandContext(ctx, "echo", "ok")
	})
	defer cleanup()

	cfg := domain.DefaultConfig()
	cfg.Linear.Team = "T"
	cfg.Linear.Project = "p"
	cfg.Claude.TimeoutSec = 30
	cfg.Retry.MaxAttempts = 1
	cfg.Retry.BaseDelaySec = 0
	cfg.Labels.Enabled = false

	var recorder writeRecorder

	// when
	_, err := session.RunScan(context.Background(), &cfg, baseDir, sessionID, false, &recorder, domain.NewLogger(io.Discard, false))

	// then
	if err != nil {
		t.Fatalf("RunScan failed: %v", err)
	}

	// Multiple Write calls prove the output was streamed incrementally, not buffered.
	if recorder.calls < 2 {
		t.Errorf("expected at least 2 Write calls (incremental streaming), got %d", recorder.calls)
	}

	// Verify content completeness — both markers arrived.
	output := recorder.buf.String()
	if !strings.Contains(output, "MARKER_A") {
		t.Errorf("missing MARKER_A in output: %q", output)
	}
	if !strings.Contains(output, "MARKER_B") {
		t.Errorf("missing MARKER_B in output: %q", output)
	}
}

func TestRunParallelDeepScan_CancelWhileWaitingSemaphore(t *testing.T) {
	// given: concurrency=1, cancel while second cluster waits for semaphore
	clusters := []domain.ClusterScanResult{
		{Name: "slow"},
		{Name: "should-not-run"},
	}

	dir := t.TempDir()
	cfg := domain.DefaultConfig()
	cfg.Scan.MaxConcurrency = 1

	ctx, cancel := context.WithCancel(context.Background())
	var callCount atomic.Int32

	// when: first scan runs, cancels ctx during execution; second should not start
	results, _ := session.RunParallelDeepScan(ctx, &cfg, dir, clusters,
		func(ctx context.Context, cfg *domain.Config, scanDir string, index int, cluster domain.ClusterScanResult) (domain.ClusterScanResult, error) {
			callCount.Add(1)
			if index == 0 {
				cancel() // cancel while second cluster waits for semaphore
			}
			return domain.ClusterScanResult{Name: cluster.Name, Completeness: 1.0}, nil
		}, domain.NewLogger(io.Discard, false))

	// then: only the first cluster should have been scanned
	if callCount.Load() != 1 {
		t.Errorf("expected 1 call (only first cluster), got %d", callCount.Load())
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestRunWaveGenerate_PartialFailure(t *testing.T) {
	// given: 3 clusters where the second one fails (claude exits non-zero)
	scanDir := t.TempDir()

	authResult := `{"cluster_name":"Auth","waves":[{"id":"auth-w1","cluster_name":"Auth","title":"Login","actions":[],"prerequisites":[],"delta":{"before":0.25,"after":0.40},"status":"available"}]}`
	apiResult := `{"cluster_name":"API","waves":[{"id":"api-w1","cluster_name":"API","title":"Endpoints","actions":[],"prerequisites":[],"delta":{"before":0.30,"after":0.50},"status":"available"}]}`

	cleanup := session.SetNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		prompt := ""
		for i, a := range args {
			if a == "-p" && i+1 < len(args) {
				prompt = args[i+1]
				break
			}
		}

		// Find the output JSON path embedded in the prompt.
		findJSONPath := func(p string) string {
			for _, line := range strings.Split(p, "\n") {
				if idx := strings.Index(line, scanDir); idx >= 0 {
					sub := line[idx:]
					if end := strings.Index(sub, "**"); end > 0 {
						return sub[:end]
					}
					return strings.TrimRight(strings.TrimSpace(sub), " *`\"")
				}
			}
			return ""
		}

		// Cluster "Bad" → exit 1 (simulate signal:killed / OOM).
		if strings.Contains(prompt, "Bad") {
			return exec.CommandContext(ctx, "false")
		}
		// Other clusters → write wave result.
		outPath := findJSONPath(prompt)
		if outPath != "" {
			var content string
			if strings.Contains(prompt, "Auth") {
				content = authResult
			} else {
				content = apiResult
			}
			return exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf(`printf '%s' > '%s'`, content, outPath))
		}
		return exec.CommandContext(ctx, "echo", "ok")
	})
	defer cleanup()

	clusters := []domain.ClusterScanResult{
		{Name: "Auth", Completeness: 0.25, Issues: []domain.IssueDetail{{ID: "T-1"}}},
		{Name: "Bad", Completeness: 0.10, Issues: []domain.IssueDetail{{ID: "T-2"}}},
		{Name: "API", Completeness: 0.30, Issues: []domain.IssueDetail{{ID: "T-3"}}},
	}
	cfg := domain.DefaultConfig()
	cfg.Claude.TimeoutSec = 10
	cfg.Retry.MaxAttempts = 1
	cfg.Retry.BaseDelaySec = 0
	logger := domain.NewLogger(io.Discard, false)

	// when
	waves, warnings, _, err := session.RunWaveGenerate(context.Background(), &cfg, scanDir, clusters, false, logger)

	// then: no fatal error
	if err != nil {
		t.Fatalf("expected no error for partial failure, got: %v", err)
	}

	// then: 2 waves from successful clusters
	if len(waves) != 2 {
		t.Fatalf("expected 2 waves from successful clusters, got %d", len(waves))
	}

	// then: 1 warning for the failed cluster
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "Bad") {
		t.Errorf("expected warning to mention 'Bad' cluster, got: %s", warnings[0])
	}
}

func TestRunWaveGenerate_AllFail(t *testing.T) {
	// given: all clusters fail
	scanDir := t.TempDir()

	cleanup := session.SetNewCmd(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	})
	defer cleanup()

	clusters := []domain.ClusterScanResult{
		{Name: "A", Completeness: 0.1, Issues: []domain.IssueDetail{{ID: "T-1"}}},
		{Name: "B", Completeness: 0.1, Issues: []domain.IssueDetail{{ID: "T-2"}}},
	}
	cfg := domain.DefaultConfig()
	cfg.Claude.TimeoutSec = 10
	cfg.Retry.MaxAttempts = 1
	cfg.Retry.BaseDelaySec = 0
	logger := domain.NewLogger(io.Discard, false)

	// when
	waves, warnings, _, err := session.RunWaveGenerate(context.Background(), &cfg, scanDir, clusters, false, logger)

	// then: error because ALL clusters failed
	if err == nil {
		t.Fatal("expected error when all clusters fail")
	}

	// then: no waves returned
	if len(waves) != 0 {
		t.Errorf("expected 0 waves, got %d", len(waves))
	}

	// then: warnings for each failed cluster
	if len(warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d: %v", len(warnings), warnings)
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
			got := domain.DetectFailedClusterNames(tt.clusters, tt.successes)
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

func TestRunWaveGenerate_DryRunPopulatesClusterName(t *testing.T) {
	// given: two clusters in dry-run mode
	scanDir := t.TempDir()
	cfg := domain.DefaultConfig()
	clusters := []domain.ClusterScanResult{
		{Name: "Auth", Issues: []domain.IssueDetail{{ID: "T-1"}}},
		{Name: "API", Issues: []domain.IssueDetail{{ID: "T-2"}}},
	}

	// when: dry-run wave generation via exported API
	_, _, failedNames, err := session.RunWaveGenerate(
		context.Background(), &cfg, scanDir, clusters,
		true, // dryRun
		domain.NewLogger(io.Discard, false),
	)

	// then: no error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// then: no failed clusters — proves ClusterName was correctly populated
	// (if ClusterName were empty, DetectFailedClusterNames would mark all clusters as failed)
	if len(failedNames) != 0 {
		t.Errorf("expected 0 failed clusters in dry-run, got %d: %v", len(failedNames), failedNames)
	}
}
