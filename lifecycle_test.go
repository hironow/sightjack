package sightjack_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	sightjack "github.com/hironow/sightjack"
)

// loadTestState loads the latest state from events for test verification.
func loadTestState(t *testing.T, baseDir string) *sightjack.SessionState {
	t.Helper()
	state, _, err := sightjack.LoadLatestState(baseDir)
	if err != nil {
		t.Fatalf("LoadLatestState: %v", err)
	}
	return state
}

// writeTestEvents creates an event store to simulate a pre-existing session state.
func writeTestEvents(t *testing.T, baseDir, sessionID string, state *sightjack.SessionState) {
	t.Helper()
	store := sightjack.NewFileEventStore(sightjack.EventStorePath(baseDir, sessionID))
	recorder := sightjack.NewSessionRecorder(store, sessionID)
	if err := recorder.Record(sightjack.EventSessionStarted, sightjack.SessionStartedPayload{
		Project:         state.Project,
		StrictnessLevel: state.StrictnessLevel,
	}); err != nil {
		t.Fatalf("record SessionStarted: %v", err)
	}
	if err := recorder.Record(sightjack.EventScanCompleted, sightjack.ScanCompletedPayload{
		Clusters:       state.Clusters,
		Completeness:   state.Completeness,
		ShibitoCount:   state.ShibitoCount,
		ScanResultPath: state.ScanResultPath,
		LastScanned:    state.LastScanned,
	}); err != nil {
		t.Fatalf("record ScanCompleted: %v", err)
	}
	if len(state.Waves) > 0 {
		if err := recorder.Record(sightjack.EventWavesGenerated, sightjack.WavesGeneratedPayload{
			Waves: state.Waves,
		}); err != nil {
			t.Fatalf("record WavesGenerated: %v", err)
		}
	}
	// Mark completed waves via separate events
	for _, w := range state.Waves {
		if w.Status == "completed" {
			if err := recorder.Record(sightjack.EventWaveCompleted, sightjack.WaveCompletedPayload{
				WaveID:      w.ID,
				ClusterName: w.ClusterName,
			}); err != nil {
				t.Fatalf("record WaveCompleted: %v", err)
			}
		}
	}
}

// testRecorder creates a real Recorder backed by the event store for lifecycle tests.
func testRecorder(baseDir, sessionID string) sightjack.Recorder {
	store := sightjack.NewFileEventStore(sightjack.EventStorePath(baseDir, sessionID))
	return sightjack.NewSessionRecorder(store, sessionID)
}

// testConfig returns a minimal Config for lifecycle tests.
// Labels and Scribe disabled to avoid extra Claude calls.
func testConfig() *sightjack.Config {
	return &sightjack.Config{
		Lang:       "en",
		Claude:     sightjack.ClaudeConfig{Command: "claude", TimeoutSec: 30},
		Scan:       sightjack.ScanConfig{MaxConcurrency: 1, ChunkSize: 50},
		Linear:     sightjack.LinearConfig{Team: "ENG", Project: "TestProject"},
		Scribe:     sightjack.ScribeConfig{Enabled: false},
		Strictness: sightjack.StrictnessConfig{Default: sightjack.StrictnessFog},
		Retry:      sightjack.RetryConfig{MaxAttempts: 1, BaseDelaySec: 0},
		Labels:     sightjack.LabelsConfig{Enabled: false},
	}
}

// --- Mock Dispatcher ---

// claudeMockDispatcher provides a newCmd replacement that writes canned JSON
// to the output path extracted from the Claude prompt.
type claudeMockDispatcher struct {
	t         *testing.T
	mu        sync.Mutex
	responses []mockResponse // ordered for deterministic matching
	callLog   []string       // records filenames written
}

type mockResponse struct {
	pattern string
	content string
}

func newMockDispatcher(t *testing.T) *claudeMockDispatcher {
	return &claudeMockDispatcher{t: t}
}

// Register adds a filename pattern → JSON response mapping.
// Pattern uses filepath.Match syntax (e.g., "classify.json", "cluster_*_c*.json").
func (d *claudeMockDispatcher) Register(pattern, jsonContent string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.responses = append(d.responses, mockResponse{pattern: pattern, content: jsonContent})
}

// Install replaces the global newCmd and returns a cleanup function.
func (d *claudeMockDispatcher) Install() func() {
	return sightjack.SetNewCmd(d.newCmdFunc)
}

// CallLog returns a copy of the filenames written by the mock.
func (d *claudeMockDispatcher) CallLog() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	cp := make([]string, len(d.callLog))
	copy(cp, d.callLog)
	return cp
}

func (d *claudeMockDispatcher) newCmdFunc(ctx context.Context, name string, args ...string) *exec.Cmd {
	prompt := extractPromptFromArgs(args)
	outputPath := extractOutputPath(prompt)

	if outputPath != "" {
		filename := filepath.Base(outputPath)
		d.mu.Lock()
		for _, r := range d.responses {
			matched, _ := filepath.Match(r.pattern, filename)
			if matched {
				os.MkdirAll(filepath.Dir(outputPath), 0755)
				os.WriteFile(outputPath, []byte(r.content), 0644)
				d.callLog = append(d.callLog, filename)
				break
			}
		}
		d.mu.Unlock()
	}

	return exec.Command("echo", "ok")
}

// extractPromptFromArgs finds the value of the -p flag in Claude CLI args.
func extractPromptFromArgs(args []string) string {
	for i, arg := range args {
		if arg == "-p" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// extractOutputPath finds the first absolute JSON file path in the prompt text.
var jsonPathRe = regexp.MustCompile(`(/[^\s"]+\.json)`)

func extractOutputPath(prompt string) string {
	m := jsonPathRe.FindString(prompt)
	return m
}

// --- Helper: assertFileExists ---

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", path)
	}
}

// --- Canned JSON fixtures ---

func classifySingleCluster() string {
	return `{
	"clusters": [
		{"name": "Auth", "issue_ids": ["AUTH-1", "AUTH-2"], "labels": ["security"]}
	],
	"total_issues": 2
}`
}

func classifyMultiCluster() string {
	return `{
	"clusters": [
		{"name": "Auth", "issue_ids": ["AUTH-1", "AUTH-2"], "labels": ["security"]},
		{"name": "API", "issue_ids": ["API-1"], "labels": ["backend"]}
	],
	"total_issues": 3
}`
}

func deepScanAuth() string {
	return `{
	"name": "Auth", "completeness": 0.35,
	"issues": [
		{"id": "AUTH-1", "identifier": "AUTH-1", "title": "Login flow", "completeness": 0.3, "gaps": ["DoD missing"]},
		{"id": "AUTH-2", "identifier": "AUTH-2", "title": "Token refresh", "completeness": 0.4, "gaps": ["Tests missing"]}
	],
	"observations": ["Auth depends on API"]
}`
}

func deepScanAPI() string {
	return `{
	"name": "API", "completeness": 0.50,
	"issues": [
		{"id": "API-1", "identifier": "API-1", "title": "Rate limiting", "completeness": 0.5, "gaps": ["Load test missing"]}
	],
	"observations": ["API rate limits affect Auth"]
}`
}

func waveGenAuth() string {
	return `{
	"cluster_name": "Auth",
	"waves": [{
		"id": "auth-w1", "cluster_name": "Auth", "title": "Add DoD",
		"description": "Define acceptance criteria", "status": "available",
		"actions": [{"type": "add_dod", "issue_id": "AUTH-1", "description": "Add DoD for login", "detail": ""}],
		"prerequisites": [],
		"delta": {"before": 0.35, "after": 0.65}
	}]
}`
}

func waveGenAuthTwoWaves() string {
	return `{
	"cluster_name": "Auth",
	"waves": [
		{
			"id": "auth-w1", "cluster_name": "Auth", "title": "Add DoD",
			"description": "Define criteria", "status": "available",
			"actions": [{"type": "add_dod", "issue_id": "AUTH-1", "description": "Add DoD", "detail": ""}],
			"prerequisites": [],
			"delta": {"before": 0.35, "after": 0.55}
		},
		{
			"id": "auth-w2", "cluster_name": "Auth", "title": "Add Tests",
			"description": "Write tests", "status": "available",
			"actions": [{"type": "add_test", "issue_id": "AUTH-2", "description": "Add tests", "detail": ""}],
			"prerequisites": [],
			"delta": {"before": 0.55, "after": 0.75}
		}
	]
}`
}

func waveGenAPI() string {
	return `{
	"cluster_name": "API",
	"waves": [{
		"id": "api-w1", "cluster_name": "API", "title": "Add Load Test",
		"description": "Create load test", "status": "available",
		"actions": [{"type": "add_test", "issue_id": "API-1", "description": "Add load test", "detail": ""}],
		"prerequisites": [],
		"delta": {"before": 0.50, "after": 0.80}
	}]
}`
}

func waveApplySuccess(waveID string) string {
	return `{
	"wave_id": "` + waveID + `", "applied": 1, "total_count": 1,
	"errors": [],
	"ripples": []
}`
}

func waveApplyPartialFailure(waveID string) string {
	return `{
	"wave_id": "` + waveID + `", "applied": 0, "total_count": 1,
	"errors": ["Failed to update AUTH-1: permission denied"],
	"ripples": []
}`
}

func nextgenEmpty() string {
	return `{
	"cluster_name": "Auth",
	"waves": [],
	"reasoning": "Cluster is sufficiently complete."
}`
}

// --- Tests for mock helpers ---

func TestExtractPromptFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"with -p flag", []string{"--print", "--model", "opus", "-p", "my prompt"}, "my prompt"},
		{"no -p flag", []string{"--print", "--model", "opus"}, ""},
		{"-p at end without value", []string{"-p"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPromptFromArgs(tt.args)
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestExtractOutputPath(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
		want   string
	}{
		{"classify path", "Write JSON output to /tmp/test123/.siren/.run/s1/classify.json", "/tmp/test123/.siren/.run/s1/classify.json"},
		{"cluster path", "Output: /tmp/abc/cluster_00_auth_c00.json end", "/tmp/abc/cluster_00_auth_c00.json"},
		{"no path", "Just some prompt text without a path", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractOutputPath(tt.prompt)
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

// --- Phase 2: Scan Tests ---

func TestLifecycle_RunScan_SingleCluster(t *testing.T) {
	// given
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-scan-single"

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	cleanup := d.Install()
	defer cleanup()

	ctx := context.Background()

	// when
	result, err := sightjack.RunScan(ctx, cfg, baseDir, sessionID, false, io.Discard, sightjack.NewLogger(io.Discard, false))

	// then
	if err != nil {
		t.Fatalf("RunScan failed: %v", err)
	}
	if len(result.Clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(result.Clusters))
	}
	if result.Clusters[0].Name != "Auth" {
		t.Errorf("expected cluster 'Auth', got %q", result.Clusters[0].Name)
	}
	if result.Clusters[0].Completeness != 0.35 {
		t.Errorf("expected completeness 0.35, got %f", result.Clusters[0].Completeness)
	}
	if result.TotalIssues != 2 {
		t.Errorf("expected 2 total issues, got %d", result.TotalIssues)
	}
	// Verify classify.json was written
	scanDir := sightjack.ScanDir(baseDir, sessionID)
	assertFileExists(t, filepath.Join(scanDir, "classify.json"))
}

func TestLifecycle_RunScan_StreamingGoesToOut(t *testing.T) {
	// given: RunScan streams Claude output (from mock: "echo ok") to the `out` writer.
	// When --json is used, cmd layer redirects `out` to stderr. This test verifies
	// that streaming data actually arrives in `out`, not somewhere else.
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-scan-stream"

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	cleanup := d.Install()
	defer cleanup()

	var streamBuf strings.Builder

	// when: pass &streamBuf as the streaming output writer
	result, err := sightjack.RunScan(context.Background(), cfg, baseDir, sessionID, false, &streamBuf, sightjack.NewLogger(io.Discard, false))

	// then: streaming buffer must contain mock Claude output ("ok" from echo)
	if err != nil {
		t.Fatalf("RunScan failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if streamBuf.Len() == 0 {
		t.Error("expected streaming output in out writer, got empty buffer")
	}
	// The streaming output must NOT be valid ScanResult JSON — it's raw Claude output.
	var probe sightjack.ScanResult
	if json.Unmarshal([]byte(streamBuf.String()), &probe) == nil && len(probe.Clusters) > 0 {
		t.Error("streaming output should be raw Claude output, not structured ScanResult JSON")
	}
}

func TestLifecycle_RunScan_JsonPipeStdoutClean(t *testing.T) {
	// given: simulate the --json pipe scenario.
	// stdout = JSON result only, streaming goes to a separate writer (stderr in real usage).
	// This verifies that separating `out` from the JSON writer produces clean pipe output.
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-scan-pipe"

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	cleanup := d.Install()
	defer cleanup()

	var streamBuf strings.Builder // simulates stderr (streaming)

	// when: streaming goes to streamBuf (stderr), NOT to stdout
	result, err := sightjack.RunScan(context.Background(), cfg, baseDir, sessionID, false, &streamBuf, sightjack.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("RunScan failed: %v", err)
	}

	// Simulate what scan.go does for --json: marshal result to stdout
	var stdoutBuf strings.Builder
	data, jsonErr := json.MarshalIndent(result, "", "  ")
	if jsonErr != nil {
		t.Fatalf("JSON marshal failed: %v", jsonErr)
	}
	stdoutBuf.Write(data)
	stdoutBuf.WriteByte('\n')

	// then: stdout must be valid ScanResult JSON parseable by `waves`
	var parsed sightjack.ScanResult
	if err := json.Unmarshal([]byte(stdoutBuf.String()), &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON (pipe would break): %v\nContent: %s", err, stdoutBuf.String())
	}
	if len(parsed.Clusters) != 1 {
		t.Errorf("expected 1 cluster in parsed stdout, got %d", len(parsed.Clusters))
	}

	// streaming (stderr) must have content and NOT pollute stdout
	if streamBuf.Len() == 0 {
		t.Error("expected streaming output in stderr writer")
	}
	if strings.Contains(stdoutBuf.String(), streamBuf.String()) {
		t.Error("streaming output leaked into stdout — pipe would receive non-JSON data")
	}
}

func TestLifecycle_RunScan_SavesScanResultJson(t *testing.T) {
	// given: scan command now saves scan_result.json for pipe replay
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-scan-cache"

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	cleanup := d.Install()
	defer cleanup()

	// when
	result, err := sightjack.RunScan(context.Background(), cfg, baseDir, sessionID, false, io.Discard, sightjack.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("RunScan failed: %v", err)
	}

	// Simulate what scan.go does: save scan_result.json
	scanDir := sightjack.ScanDir(baseDir, sessionID)
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	if err := sightjack.WriteScanResult(scanResultPath, result); err != nil {
		t.Fatalf("WriteScanResult failed: %v", err)
	}

	// then: scan_result.json exists and is loadable
	assertFileExists(t, scanResultPath)
	loaded, err := sightjack.LoadScanResult(scanResultPath)
	if err != nil {
		t.Fatalf("LoadScanResult failed: %v", err)
	}
	if len(loaded.Clusters) != 1 {
		t.Errorf("expected 1 cluster in cached result, got %d", len(loaded.Clusters))
	}
	if loaded.Clusters[0].Name != result.Clusters[0].Name {
		t.Errorf("cached cluster name mismatch: %q vs %q", loaded.Clusters[0].Name, result.Clusters[0].Name)
	}
}

func TestLifecycle_RunScan_MultiCluster(t *testing.T) {
	// given
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-scan-multi"

	d := newMockDispatcher(t)
	d.Register("classify.json", classifyMultiCluster())
	d.Register("cluster_00_auth_c00.json", deepScanAuth())
	d.Register("cluster_01_api_c00.json", deepScanAPI())
	cleanup := d.Install()
	defer cleanup()

	ctx := context.Background()

	// when
	result, err := sightjack.RunScan(ctx, cfg, baseDir, sessionID, false, io.Discard, sightjack.NewLogger(io.Discard, false))

	// then
	if err != nil {
		t.Fatalf("RunScan failed: %v", err)
	}
	if len(result.Clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(result.Clusters))
	}
	// Completeness should be average of Auth (0.35) and API (0.50) = 0.425
	expectedCompleteness := (0.35 + 0.50) / 2
	if result.Completeness != expectedCompleteness {
		t.Errorf("expected completeness %f, got %f", expectedCompleteness, result.Completeness)
	}
	if result.TotalIssues != 3 {
		t.Errorf("expected 3 total issues, got %d", result.TotalIssues)
	}
}

// --- Phase 3: Session Tests ---

func TestLifecycle_HappyPath(t *testing.T) {
	// given: single cluster, single wave
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-happy"

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	d.Register("wave_*_*.json", waveGenAuth())
	d.Register("apply_*_*.json", waveApplySuccess("auth-w1"))
	cleanup := d.Install()
	defer cleanup()

	ctx := context.Background()
	// stdin: select wave 1, approve, quit
	input := strings.NewReader("1\na\nq\n")

	// when
	err := sightjack.RunSession(ctx, cfg, baseDir, sessionID, false, input, io.Discard, testRecorder(baseDir, sessionID), sightjack.NewLogger(io.Discard, false))

	// then: no error
	if err != nil {
		t.Fatalf("RunSession failed: %v", err)
	}

	// events should produce reconstructable state
	state := loadTestState(t, baseDir)

	// wave should be completed
	if len(state.Waves) != 1 {
		t.Fatalf("expected 1 wave in state, got %d", len(state.Waves))
	}
	if state.Waves[0].Status != "completed" {
		t.Errorf("expected wave completed, got %q", state.Waves[0].Status)
	}

	// completeness should be updated (delta: 0.35 -> 0.65)
	if state.Completeness < 0.6 {
		t.Errorf("expected completeness >= 0.6 after apply, got %f", state.Completeness)
	}

	// scan_result.json should be cached
	scanDir := sightjack.ScanDir(baseDir, sessionID)
	assertFileExists(t, filepath.Join(scanDir, "scan_result.json"))

	// show path: RestoreWaves should reconstruct waves
	waves := sightjack.RestoreWaves(state.Waves)
	if len(waves) != 1 {
		t.Fatalf("RestoreWaves: expected 1, got %d", len(waves))
	}
	if waves[0].Status != "completed" {
		t.Errorf("RestoreWaves: expected completed, got %q", waves[0].Status)
	}
}

func TestLifecycle_RejectThenApprove(t *testing.T) {
	// given: single cluster, two waves (both available)
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-reject"

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	d.Register("wave_*_*.json", waveGenAuthTwoWaves())
	d.Register("apply_*_*.json", waveApplySuccess("auth-w1"))
	cleanup := d.Install()
	defer cleanup()

	ctx := context.Background()
	// stdin: select wave 1, reject, select wave 1 again (still first available), approve, quit
	input := strings.NewReader("1\nr\n1\na\nq\n")

	// when
	err := sightjack.RunSession(ctx, cfg, baseDir, sessionID, false, input, io.Discard, testRecorder(baseDir, sessionID), sightjack.NewLogger(io.Discard, false))

	// then
	if err != nil {
		t.Fatalf("RunSession failed: %v", err)
	}

	state := loadTestState(t, baseDir)

	// Only auth-w1 should be completed (approved on second attempt)
	completedCount := 0
	for _, w := range state.Waves {
		if w.Status == "completed" {
			completedCount++
		}
	}
	if completedCount != 1 {
		t.Errorf("expected 1 completed wave, got %d", completedCount)
	}
}

func TestLifecycle_PartialApplyNotCompleted(t *testing.T) {
	// given: apply returns errors → wave should NOT be completed
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-partial"

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	d.Register("wave_*_*.json", waveGenAuth())
	d.Register("apply_*_*.json", waveApplyPartialFailure("auth-w1"))
	cleanup := d.Install()
	defer cleanup()

	ctx := context.Background()
	// stdin: select wave 1, approve (will partially fail), quit
	input := strings.NewReader("1\na\nq\n")

	// when
	err := sightjack.RunSession(ctx, cfg, baseDir, sessionID, false, input, io.Discard, testRecorder(baseDir, sessionID), sightjack.NewLogger(io.Discard, false))

	// then
	if err != nil {
		t.Fatalf("RunSession failed: %v", err)
	}

	state := loadTestState(t, baseDir)

	// wave should still be available (not completed due to errors)
	for _, w := range state.Waves {
		if w.Status == "completed" {
			t.Errorf("expected no completed waves, but %s is completed", w.ID)
		}
	}

	// completeness should remain at initial value (0.35)
	if state.Completeness > 0.36 {
		t.Errorf("expected completeness unchanged (~0.35), got %f", state.Completeness)
	}
}

// --- Phase 4: Resume Tests ---

func TestLifecycle_ResumeFromState(t *testing.T) {
	// given: pre-existing state with wave 1 completed, wave 2 available
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-resume"

	// Set up scan directory and cache scan result
	scanDir, err := sightjack.EnsureScanDir(baseDir, sessionID)
	if err != nil {
		t.Fatalf("EnsureScanDir: %v", err)
	}
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	scanResult := &sightjack.ScanResult{
		Clusters: []sightjack.ClusterScanResult{
			{
				Name:         "Auth",
				Completeness: 0.55,
				Issues: []sightjack.IssueDetail{
					{ID: "AUTH-1", Identifier: "AUTH-1", Title: "Login flow", Completeness: 0.5, Gaps: []string{"Tests missing"}},
					{ID: "AUTH-2", Identifier: "AUTH-2", Title: "Token refresh", Completeness: 0.6, Gaps: []string{}},
				},
				Observations: []string{},
			},
		},
		TotalIssues: 2,
	}
	if err := sightjack.WriteScanResult(scanResultPath, scanResult); err != nil {
		t.Fatalf("WriteScanResult: %v", err)
	}

	// Write pre-existing state: wave 1 completed, wave 2 available
	state := &sightjack.SessionState{
		Version:      sightjack.StateFormatVersion,
		SessionID:    sessionID,
		Project:      "TestProject",
		Completeness: 0.55,
		Clusters: []sightjack.ClusterState{
			{Name: "Auth", Completeness: 0.55, IssueCount: 2},
		},
		Waves: []sightjack.WaveState{
			{
				ID:          "auth-w1",
				ClusterName: "Auth",
				Title:       "Add DoD",
				Status:      "completed",
				ActionCount: 1,
				Delta:       sightjack.WaveDelta{Before: 0.35, After: 0.55},
			},
			{
				ID:          "auth-w2",
				ClusterName: "Auth",
				Title:       "Add Tests",
				Status:      "available",
				ActionCount: 1,
				Actions:     []sightjack.WaveAction{{Type: "add_test", IssueID: "AUTH-2", Description: "Add tests"}},
				Delta:       sightjack.WaveDelta{Before: 0.55, After: 0.75},
			},
		},
		ScanResultPath: scanResultPath,
	}
	writeTestEvents(t, baseDir, sessionID, state)

	// Register mock for apply only (scan/wavegen already done)
	d := newMockDispatcher(t)
	d.Register("apply_*_*.json", waveApplySuccess("auth-w2"))
	cleanup := d.Install()
	defer cleanup()

	ctx := context.Background()
	// stdin: select wave 1 (only available wave), approve, quit
	input := strings.NewReader("1\na\nq\n")

	// when
	err = sightjack.RunResumeSession(ctx, cfg, baseDir, state, input, io.Discard, testRecorder(baseDir, state.SessionID), sightjack.NewLogger(io.Discard, false))

	// then
	if err != nil {
		t.Fatalf("RunResumeSession failed: %v", err)
	}

	updated := loadTestState(t, baseDir)

	// Both waves should be completed
	completedCount := 0
	for _, w := range updated.Waves {
		if w.Status == "completed" {
			completedCount++
		}
	}
	if completedCount != 2 {
		t.Errorf("expected 2 completed waves, got %d", completedCount)
	}

	// Completeness should have increased from 0.55
	if updated.Completeness <= 0.55 {
		t.Errorf("expected completeness > 0.55, got %f", updated.Completeness)
	}
}

func TestLifecycle_QuitAndResume(t *testing.T) {
	// Phase 1: scan + approve wave 1 + quit
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-quit-resume"

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	d.Register("wave_*_*.json", waveGenAuthTwoWaves())
	d.Register("apply_*_*.json", waveApplySuccess("auth-w1"))
	cleanup := d.Install()
	defer cleanup()

	ctx := context.Background()
	// stdin: select wave 1, approve, quit
	input := strings.NewReader("1\na\nq\n")

	err := sightjack.RunSession(ctx, cfg, baseDir, sessionID, false, input, io.Discard, testRecorder(baseDir, sessionID), sightjack.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("Phase 1 RunSession failed: %v", err)
	}

	// Verify: state persisted with wave 1 completed
	state := loadTestState(t, baseDir)
	completedPhase1 := 0
	for _, w := range state.Waves {
		if w.Status == "completed" {
			completedPhase1++
		}
	}
	if completedPhase1 != 1 {
		t.Fatalf("Phase 1: expected 1 completed wave, got %d", completedPhase1)
	}

	// Phase 2: resume + approve wave 2 + quit
	d2 := newMockDispatcher(t)
	d2.Register("apply_*_*.json", waveApplySuccess("auth-w2"))
	cleanup2 := d2.Install()
	defer cleanup2()

	input2 := strings.NewReader("1\na\nq\n")
	err = sightjack.RunResumeSession(ctx, cfg, baseDir, state, input2, io.Discard, testRecorder(baseDir, state.SessionID), sightjack.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("Phase 2 RunResumeSession failed: %v", err)
	}

	// Verify: both waves completed
	finalState := loadTestState(t, baseDir)
	completedPhase2 := 0
	for _, w := range finalState.Waves {
		if w.Status == "completed" {
			completedPhase2++
		}
	}
	if completedPhase2 != 2 {
		t.Errorf("Phase 2: expected 2 completed waves, got %d", completedPhase2)
	}

	// Completeness should be higher than Phase 1
	if finalState.Completeness <= state.Completeness {
		t.Errorf("expected completeness to increase: phase1=%f, phase2=%f",
			state.Completeness, finalState.Completeness)
	}
}

// --- Phase 5: Multi-cluster Test ---

func TestLifecycle_MultiCluster(t *testing.T) {
	// given: 2 clusters (Auth + API), each with 1 wave
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-multi"

	d := newMockDispatcher(t)
	d.Register("classify.json", classifyMultiCluster())
	d.Register("cluster_00_auth_c00.json", deepScanAuth())
	d.Register("cluster_01_api_c00.json", deepScanAPI())
	d.Register("wave_00_auth.json", waveGenAuth())
	d.Register("wave_01_api.json", waveGenAPI())
	d.Register("apply_*_*.json", waveApplySuccess("auth-w1"))
	cleanup := d.Install()
	defer cleanup()

	ctx := context.Background()
	// stdin: select wave 1 (Auth), approve, select wave 1 (API), approve, quit
	input := strings.NewReader("1\na\n1\na\nq\n")

	// when
	err := sightjack.RunSession(ctx, cfg, baseDir, sessionID, false, input, io.Discard, testRecorder(baseDir, sessionID), sightjack.NewLogger(io.Discard, false))

	// then
	if err != nil {
		t.Fatalf("RunSession failed: %v", err)
	}

	state := loadTestState(t, baseDir)

	// Both waves should be completed
	completedCount := 0
	for _, w := range state.Waves {
		if w.Status == "completed" {
			completedCount++
		}
	}
	if completedCount != 2 {
		t.Errorf("expected 2 completed waves, got %d", completedCount)
	}

	// Should have 2 clusters in state
	if len(state.Clusters) != 2 {
		t.Errorf("expected 2 clusters, got %d", len(state.Clusters))
	}

	// Overall completeness should be average of both clusters
	if state.Completeness < 0.50 {
		t.Errorf("expected completeness >= 0.50, got %f", state.Completeness)
	}
}

func TestMockDispatcher_WritesFile(t *testing.T) {
	// given
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "classify.json")
	d := newMockDispatcher(t)
	d.Register("classify.json", `{"clusters":[],"total_issues":0}`)
	cleanup := d.Install()
	defer cleanup()

	// when: simulate a Claude call with -p containing the output path
	prompt := "Write JSON to " + outputPath
	cmd := d.newCmdFunc(context.Background(), "claude", "--dangerously-skip-permissions", "--print", "-p", prompt)
	cmd.Run()

	// then
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}
	if !strings.Contains(string(data), `"total_issues"`) {
		t.Errorf("unexpected content: %s", data)
	}
	log := d.CallLog()
	if len(log) != 1 || log[0] != "classify.json" {
		t.Errorf("expected callLog [classify.json], got %v", log)
	}
}

// --- Phase 6: D-Mail Lifecycle Tests ---

func TestLifecycle_DMailFullCycle(t *testing.T) {
	// given: pre-place feedback d-mail in inbox before session starts
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-dmail-full"

	// Set up mail directories and pre-place feedback
	if err := sightjack.EnsureMailDirs(baseDir); err != nil {
		t.Fatal(err)
	}
	feedbackMail := &sightjack.DMail{
		Name:        "fb-arch-001",
		Kind:        sightjack.DMailFeedback,
		Description: "Token rotation drift detected",
		Severity:    "high",
		Body:        "JWT rotation interval misaligned with refresh window.",
	}
	feedbackData, err := sightjack.MarshalDMail(feedbackMail)
	if err != nil {
		t.Fatal(err)
	}
	inboxPath := filepath.Join(sightjack.MailDir(baseDir, sightjack.InboxDir), feedbackMail.Filename())
	if err := os.WriteFile(inboxPath, feedbackData, 0644); err != nil {
		t.Fatal(err)
	}

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	d.Register("wave_*_*.json", waveGenAuth())
	d.Register("apply_*_*.json", waveApplySuccess("auth-w1"))
	cleanup := d.Install()
	defer cleanup()

	ctx := context.Background()
	// select wave 1, approve, quit
	input := strings.NewReader("1\na\nq\n")

	// when
	err = sightjack.RunSession(ctx, cfg, baseDir, sessionID, false, input, io.Discard, testRecorder(baseDir, sessionID), sightjack.NewLogger(io.Discard, false))

	// then: session completes without error
	if err != nil {
		t.Fatalf("RunSession: %v", err)
	}

	// Feedback should be removed from inbox (received + archived)
	if _, err := os.Stat(inboxPath); !os.IsNotExist(err) {
		t.Error("feedback should have been removed from inbox")
	}

	// Feedback should exist in archive
	archiveFeedback := filepath.Join(sightjack.MailDir(baseDir, sightjack.ArchiveDir), feedbackMail.Filename())
	if _, err := os.Stat(archiveFeedback); os.IsNotExist(err) {
		t.Error("feedback should exist in archive")
	}

	// Verify archived feedback content is parseable and correct
	archivedData, err := os.ReadFile(archiveFeedback)
	if err != nil {
		t.Fatalf("read archived feedback: %v", err)
	}
	archivedFeedback, err := sightjack.ParseDMail(archivedData)
	if err != nil {
		t.Fatalf("parse archived feedback: %v", err)
	}
	if archivedFeedback.Kind != sightjack.DMailFeedback {
		t.Errorf("expected feedback kind, got %q", archivedFeedback.Kind)
	}
	if archivedFeedback.Severity != "high" {
		t.Errorf("expected high severity, got %q", archivedFeedback.Severity)
	}

	// Specification d-mail should exist in outbox and archive
	outboxFiles, _ := sightjack.ListDMail(baseDir, sightjack.OutboxDir)
	var foundSpec, foundReport bool
	for _, f := range outboxFiles {
		if strings.Contains(f, "spec") {
			foundSpec = true
			data, readErr := os.ReadFile(filepath.Join(sightjack.MailDir(baseDir, sightjack.OutboxDir), f))
			if readErr != nil {
				t.Fatalf("read spec: %v", readErr)
			}
			mail, parseErr := sightjack.ParseDMail(data)
			if parseErr != nil {
				t.Fatalf("parse spec: %v", parseErr)
			}
			if mail.Kind != sightjack.DMailSpecification {
				t.Errorf("expected specification kind, got %q", mail.Kind)
			}
			if !strings.Contains(mail.Body, "AUTH-1") {
				t.Error("spec body should reference AUTH-1")
			}
		}
		if strings.Contains(f, "report") {
			foundReport = true
			data, readErr := os.ReadFile(filepath.Join(sightjack.MailDir(baseDir, sightjack.OutboxDir), f))
			if readErr != nil {
				t.Fatalf("read report: %v", readErr)
			}
			mail, parseErr := sightjack.ParseDMail(data)
			if parseErr != nil {
				t.Fatalf("parse report: %v", parseErr)
			}
			if mail.Kind != sightjack.DMailReport {
				t.Errorf("expected report kind, got %q", mail.Kind)
			}
		}
	}
	if !foundSpec {
		t.Error("expected specification d-mail in outbox")
	}
	if !foundReport {
		t.Error("expected report d-mail in outbox")
	}

	// Both should also be in archive
	archiveFiles, _ := sightjack.ListDMail(baseDir, sightjack.ArchiveDir)
	var archiveSpec, archiveReport bool
	for _, f := range archiveFiles {
		if strings.Contains(f, "spec") {
			archiveSpec = true
		}
		if strings.Contains(f, "report") {
			archiveReport = true
		}
	}
	if !archiveSpec {
		t.Error("expected specification in archive")
	}
	if !archiveReport {
		t.Error("expected report in archive")
	}
}

func TestLifecycle_DMailResumeCycle(t *testing.T) {
	// given: pre-existing state with wave 1 completed, wave 2 available
	// Pre-place feedback in inbox before resuming
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-dmail-resume"

	// Set up scan directory and cache scan result
	scanDir, err := sightjack.EnsureScanDir(baseDir, sessionID)
	if err != nil {
		t.Fatalf("EnsureScanDir: %v", err)
	}
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	scanResult := &sightjack.ScanResult{
		Clusters: []sightjack.ClusterScanResult{
			{
				Name:         "Auth",
				Completeness: 0.55,
				Issues: []sightjack.IssueDetail{
					{ID: "AUTH-1", Identifier: "AUTH-1", Title: "Login flow", Completeness: 0.5},
					{ID: "AUTH-2", Identifier: "AUTH-2", Title: "Token refresh", Completeness: 0.6},
				},
			},
		},
		TotalIssues: 2,
	}
	if err := sightjack.WriteScanResult(scanResultPath, scanResult); err != nil {
		t.Fatalf("WriteScanResult: %v", err)
	}

	// Write pre-existing state
	state := &sightjack.SessionState{
		Version:      sightjack.StateFormatVersion,
		SessionID:    sessionID,
		Project:      "TestProject",
		Completeness: 0.55,
		Clusters:     []sightjack.ClusterState{{Name: "Auth", Completeness: 0.55, IssueCount: 2}},
		Waves: []sightjack.WaveState{
			{
				ID: "auth-w1", ClusterName: "Auth", Title: "Add DoD",
				Status: "completed", ActionCount: 1,
				Delta: sightjack.WaveDelta{Before: 0.35, After: 0.55},
			},
			{
				ID: "auth-w2", ClusterName: "Auth", Title: "Add Tests",
				Status: "available", ActionCount: 1,
				Actions: []sightjack.WaveAction{{Type: "add_test", IssueID: "AUTH-2", Description: "Add tests"}},
				Delta:   sightjack.WaveDelta{Before: 0.55, After: 0.75},
			},
		},
		ScanResultPath: scanResultPath,
	}
	writeTestEvents(t, baseDir, sessionID, state)

	// Set up mail directories and pre-place feedback
	if err := sightjack.EnsureMailDirs(baseDir); err != nil {
		t.Fatal(err)
	}
	feedbackMail := &sightjack.DMail{
		Name:        "fb-perf-001",
		Kind:        sightjack.DMailFeedback,
		Description: "Auth latency spike in token validation",
		Severity:    "high",
		Body:        "p99 latency exceeds 500ms under load.",
	}
	feedbackData, marshalErr := sightjack.MarshalDMail(feedbackMail)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	inboxPath := filepath.Join(sightjack.MailDir(baseDir, sightjack.InboxDir), feedbackMail.Filename())
	if err := os.WriteFile(inboxPath, feedbackData, 0644); err != nil {
		t.Fatal(err)
	}

	// Register mock for apply only
	d := newMockDispatcher(t)
	d.Register("apply_*_*.json", waveApplySuccess("auth-w2"))
	cleanup := d.Install()
	defer cleanup()

	ctx := context.Background()
	// select wave 1 (only available), approve, quit
	input := strings.NewReader("1\na\nq\n")

	// when
	err = sightjack.RunResumeSession(ctx, cfg, baseDir, state, input, io.Discard, testRecorder(baseDir, state.SessionID), sightjack.NewLogger(io.Discard, false))

	// then
	if err != nil {
		t.Fatalf("RunResumeSession: %v", err)
	}

	// Feedback should be archived
	if _, err := os.Stat(inboxPath); !os.IsNotExist(err) {
		t.Error("feedback should have been removed from inbox")
	}
	archiveFeedback := filepath.Join(sightjack.MailDir(baseDir, sightjack.ArchiveDir), feedbackMail.Filename())
	if _, err := os.Stat(archiveFeedback); os.IsNotExist(err) {
		t.Error("feedback should exist in archive")
	}

	// Spec and report d-mails should exist for wave auth-w2
	outboxFiles, _ := sightjack.ListDMail(baseDir, sightjack.OutboxDir)
	var specFound, reportFound bool
	for _, f := range outboxFiles {
		if strings.Contains(f, "spec") && strings.Contains(f, "auth") {
			specFound = true
			data, readErr := os.ReadFile(filepath.Join(sightjack.MailDir(baseDir, sightjack.OutboxDir), f))
			if readErr != nil {
				t.Fatalf("read spec: %v", readErr)
			}
			mail, parseErr := sightjack.ParseDMail(data)
			if parseErr != nil {
				t.Fatalf("parse spec: %v", parseErr)
			}
			if mail.Kind != sightjack.DMailSpecification {
				t.Errorf("expected specification, got %q", mail.Kind)
			}
			if len(mail.Issues) == 0 {
				t.Error("spec should have issue references")
			}
		}
		if strings.Contains(f, "report") && strings.Contains(f, "auth") {
			reportFound = true
		}
	}
	if !specFound {
		t.Error("expected specification d-mail in outbox for auth-w2")
	}
	if !reportFound {
		t.Error("expected report d-mail in outbox for auth-w2")
	}

	// Final state should have both waves completed
	finalState := loadTestState(t, baseDir)
	completedCount := 0
	for _, w := range finalState.Waves {
		if w.Status == "completed" {
			completedCount++
		}
	}
	if completedCount != 2 {
		t.Errorf("expected 2 completed waves, got %d", completedCount)
	}
}

func TestLifecycle_DMailNoFeedback_StillGeneratesSpecAndReport(t *testing.T) {
	// given: no feedback in inbox, verify spec + report are still generated
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-dmail-nofb"

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	d.Register("wave_*_*.json", waveGenAuth())
	d.Register("apply_*_*.json", waveApplySuccess("auth-w1"))
	cleanup := d.Install()
	defer cleanup()

	ctx := context.Background()
	input := strings.NewReader("1\na\nq\n")

	// when
	err := sightjack.RunSession(ctx, cfg, baseDir, sessionID, false, input, io.Discard, testRecorder(baseDir, sessionID), sightjack.NewLogger(io.Discard, false))

	// then
	if err != nil {
		t.Fatalf("RunSession: %v", err)
	}

	// Spec and report should still be generated even without feedback
	outboxFiles, listErr := sightjack.ListDMail(baseDir, sightjack.OutboxDir)
	if listErr != nil {
		t.Fatalf("list outbox: %v", listErr)
	}
	var specCount, reportCount int
	for _, f := range outboxFiles {
		if strings.Contains(f, "spec") {
			specCount++
		}
		if strings.Contains(f, "report") {
			reportCount++
		}
	}
	if specCount != 1 {
		t.Errorf("expected 1 spec d-mail, got %d", specCount)
	}
	if reportCount != 1 {
		t.Errorf("expected 1 report d-mail, got %d", reportCount)
	}

	// Inbox should be empty (no feedback was placed)
	inboxFiles, _ := sightjack.ListDMail(baseDir, sightjack.InboxDir)
	if len(inboxFiles) != 0 {
		t.Errorf("expected empty inbox, got %d files", len(inboxFiles))
	}
}

// --- Phase 7: Result Cache Round-Trip Tests ---
//
// Each pipe command caches its final result as {cmd}_result.json.
// These tests verify the full round-trip: produce → marshal → write → read → unmarshal → verify.

func architectDiscussFixture() string {
	return `{
	"analysis": "Auth module has tight coupling between login and token refresh",
	"reasoning": "Separating concerns will improve testability and maintainability",
	"decision": "approve",
	"modified_wave": null
}`
}

func TestResultCache_WavesPlan(t *testing.T) {
	// given: run scan + wave generation to get a WavePlan
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-cache-waves"

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	d.Register("wave_*_*.json", waveGenAuth())
	cleanup := d.Install()
	defer cleanup()

	scanResult, err := sightjack.RunScan(context.Background(), cfg, baseDir, sessionID, false, io.Discard, sightjack.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("RunScan failed: %v", err)
	}

	scanDir := sightjack.ScanDir(baseDir, sessionID)
	waves, _, _, err := sightjack.RunWaveGenerate(context.Background(), cfg, scanDir, scanResult.Clusters, false, sightjack.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("RunWaveGenerate failed: %v", err)
	}

	plan := sightjack.WavePlan{Waves: waves, ScanResult: scanResult}

	// when: marshal and write (same as cmd/waves.go)
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	resultPath := filepath.Join(scanDir, "waves_result.json")
	if err := os.WriteFile(resultPath, data, 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// then: round-trip read and verify
	assertFileExists(t, resultPath)
	loaded, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var parsed sightjack.WavePlan
	if err := json.Unmarshal(loaded, &parsed); err != nil {
		t.Fatalf("unmarshal as WavePlan failed (select would break): %v", err)
	}
	if len(parsed.Waves) != len(plan.Waves) {
		t.Errorf("expected %d waves, got %d", len(plan.Waves), len(parsed.Waves))
	}
	if parsed.ScanResult == nil {
		t.Error("expected ScanResult to be preserved in cached WavePlan")
	}
	if parsed.Waves[0].ClusterName != "Auth" {
		t.Errorf("expected cluster Auth, got %q", parsed.Waves[0].ClusterName)
	}
}

func TestResultCache_ApplyResult(t *testing.T) {
	// given: run scan + wave gen + wave apply to get an ApplyResult
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-cache-apply"

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	d.Register("wave_*_*.json", waveGenAuth())
	d.Register("apply_*_*.json", waveApplySuccess("auth-w1"))
	cleanup := d.Install()
	defer cleanup()

	scanResult, err := sightjack.RunScan(context.Background(), cfg, baseDir, sessionID, false, io.Discard, sightjack.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("RunScan failed: %v", err)
	}

	scanDir := sightjack.ScanDir(baseDir, sessionID)
	waves, _, _, err := sightjack.RunWaveGenerate(context.Background(), cfg, scanDir, scanResult.Clusters, false, sightjack.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("RunWaveGenerate failed: %v", err)
	}

	wave := waves[0]
	strictness := string(sightjack.ResolveStrictness(cfg.Strictness, []string{wave.ClusterName}))
	internal, err := sightjack.RunWaveApply(context.Background(), cfg, scanDir, wave, strictness, io.Discard, sightjack.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("RunWaveApply failed: %v", err)
	}

	result := sightjack.ToApplyResult(wave, internal)

	// when: marshal and write (same as cmd/apply.go)
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	resultPath := filepath.Join(scanDir, "apply_result.json")
	if err := os.WriteFile(resultPath, data, 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// then: round-trip read and verify parseable by nextgen
	assertFileExists(t, resultPath)
	loaded, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var parsed sightjack.ApplyResult
	if err := json.Unmarshal(loaded, &parsed); err != nil {
		t.Fatalf("unmarshal as ApplyResult failed (nextgen would break): %v", err)
	}
	if parsed.WaveID != "auth-w1" {
		t.Errorf("expected wave ID auth-w1, got %q", parsed.WaveID)
	}
	if len(parsed.AppliedActions) != 1 {
		t.Errorf("expected 1 applied action, got %d", len(parsed.AppliedActions))
	}
}

func TestResultCache_DiscussResult(t *testing.T) {
	// given: run scan + wave gen + architect discuss to get a DiscussResult
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-cache-discuss"

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	d.Register("wave_*_*.json", waveGenAuth())
	d.Register("architect_*_*.json", architectDiscussFixture())
	cleanup := d.Install()
	defer cleanup()

	scanResult, err := sightjack.RunScan(context.Background(), cfg, baseDir, sessionID, false, io.Discard, sightjack.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("RunScan failed: %v", err)
	}

	scanDir := sightjack.ScanDir(baseDir, sessionID)
	waves, _, _, err := sightjack.RunWaveGenerate(context.Background(), cfg, scanDir, scanResult.Clusters, false, sightjack.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("RunWaveGenerate failed: %v", err)
	}

	wave := waves[0]
	strictness := string(sightjack.ResolveStrictness(cfg.Strictness, []string{wave.ClusterName}))
	resp, err := sightjack.RunArchitectDiscuss(context.Background(), cfg, scanDir, wave, "review coupling", strictness, io.Discard, sightjack.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("RunArchitectDiscuss failed: %v", err)
	}

	result := sightjack.ToDiscussResult(wave, resp, "review coupling")

	// when: marshal and write (same as cmd/discuss.go)
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	resultPath := filepath.Join(scanDir, "discuss_result.json")
	if err := os.WriteFile(resultPath, data, 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// then: round-trip read and verify parseable by adr
	assertFileExists(t, resultPath)
	loaded, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var parsed sightjack.DiscussResult
	if err := json.Unmarshal(loaded, &parsed); err != nil {
		t.Fatalf("unmarshal as DiscussResult failed (adr would break): %v", err)
	}
	if parsed.WaveID != wave.ID {
		t.Errorf("expected wave ID %q, got %q", wave.ID, parsed.WaveID)
	}
	if parsed.Analysis == "" {
		t.Error("expected non-empty analysis")
	}
	if parsed.Decision != "approve" {
		t.Errorf("expected decision 'approve', got %q", parsed.Decision)
	}
}

func TestResultCache_NextgenPlan(t *testing.T) {
	// given: run scan + wave gen + apply + nextgen to get a WavePlan
	baseDir := t.TempDir()
	cfg := testConfig()
	sessionID := "test-cache-nextgen"

	d := newMockDispatcher(t)
	d.Register("classify.json", classifySingleCluster())
	d.Register("cluster_*_c*.json", deepScanAuth())
	d.Register("wave_*_*.json", waveGenAuth())
	d.Register("apply_*_*.json", waveApplySuccess("auth-w1"))
	d.Register("nextgen_*_*.json", nextgenEmpty())
	cleanup := d.Install()
	defer cleanup()

	scanResult, err := sightjack.RunScan(context.Background(), cfg, baseDir, sessionID, false, io.Discard, sightjack.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("RunScan failed: %v", err)
	}

	scanDir := sightjack.ScanDir(baseDir, sessionID)
	waves, _, _, err := sightjack.RunWaveGenerate(context.Background(), cfg, scanDir, scanResult.Clusters, false, sightjack.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("RunWaveGenerate failed: %v", err)
	}

	wave := waves[0]
	cluster := scanResult.Clusters[0]
	cluster.Completeness = 0.65 // post-apply completeness

	existingADRs, _ := sightjack.ReadExistingADRs(sightjack.ADRDir(baseDir))
	completedWaves := sightjack.CompletedWavesForCluster(waves, cluster.Name)
	strictness := string(sightjack.ResolveStrictness(cfg.Strictness, []string{cluster.Name}))

	newWaves, err := sightjack.GenerateNextWaves(context.Background(), cfg, scanDir, wave, cluster, completedWaves, existingADRs, nil, strictness, nil, sightjack.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("GenerateNextWaves failed: %v", err)
	}

	plan := sightjack.WavePlan{Waves: newWaves}

	// when: marshal and write (same as cmd/nextgen.go)
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	resultPath := filepath.Join(scanDir, "nextgen_result.json")
	if err := os.WriteFile(resultPath, data, 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// then: round-trip read and verify parseable by select
	assertFileExists(t, resultPath)
	loaded, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	var parsed sightjack.WavePlan
	if err := json.Unmarshal(loaded, &parsed); err != nil {
		t.Fatalf("unmarshal as WavePlan failed (select would break): %v", err)
	}
	// nextgenEmpty returns no new waves (cluster complete)
	if len(parsed.Waves) != 0 {
		t.Errorf("expected 0 waves from nextgen (cluster complete), got %d", len(parsed.Waves))
	}
}
