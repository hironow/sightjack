# Sightjack v0.5 Session Resume — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable session resume so users can continue interrupted sessions without re-scanning Linear.

**Architecture:** Expand `WaveState` to persist full wave data (Actions, Description, Delta). Cache `ScanResult` to disk after scan. On `sightjack session`, detect existing state and prompt r/n/s (resume/new/re-scan). Save state after each wave completion for crash resilience.

**Tech Stack:** Go, JSON serialization, existing `.siren/state.json` persistence layer.

---

## Reference Files

| File | Purpose |
|------|---------|
| `model.go` | All types: `SessionState`, `WaveState`, `Wave`, `ScanResult`, etc. |
| `state.go` | `ReadState`, `WriteState`, `StatePath`, `EnsureScanDir` |
| `session.go` | `RunSession`, `BuildWaveStates`, `BuildCompletedWaveMap` |
| `navigator.go` | `RenderNavigatorWithWaves` |
| `cli.go` | `PromptWaveSelection`, `PromptWaveApproval`, `ScanLine` |
| `wave.go` | `WaveKey`, `EvaluateUnlocks`, `AvailableWaves` |
| `scanner.go` | `RunScan`, `RunWaveGenerate`, `MergeScanResults` |
| `config.go` | `Config`, `DefaultConfig`, `LoadConfig` |
| `cmd/sightjack/main.go` | CLI entry point for `session` subcommand |

---

## Task 1: Expand WaveState with Actions, Description, Delta

**Files:**
- Modify: `model.go:80-87`
- Test: `state_test.go`

**Step 1: Write the failing test**

Add to `state_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestWaveState_FullFieldsRoundTrip -v`
Expected: FAIL — `WaveState` has no `Actions`, `Description`, `Delta` fields.

**Step 3: Write minimal implementation**

Update `WaveState` in `model.go:80-87`:

```go
type WaveState struct {
	ID            string       `json:"id"`
	ClusterName   string       `json:"cluster_name"`
	Title         string       `json:"title"`
	Status        string       `json:"status"`
	Prerequisites []string     `json:"prerequisites,omitempty"`
	ActionCount   int          `json:"action_count"`
	Actions       []WaveAction `json:"actions,omitempty"`
	Description   string       `json:"description,omitempty"`
	Delta         WaveDelta    `json:"delta,omitempty"`
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -run TestWaveState_FullFieldsRoundTrip -v`
Expected: PASS

**Step 5: Commit**

```bash
git add model.go state_test.go
git commit -m "feat(v0.5): expand WaveState with Actions, Description, Delta"
```

---

## Task 2: Update BuildWaveStates to persist full fields

**Files:**
- Modify: `session.go:294-307` (`BuildWaveStates` function)
- Test: `session_test.go`

**Step 1: Write the failing test**

Add to `session_test.go`:

```go
func TestBuildWaveStates_IncludesFullFields(t *testing.T) {
	// given
	waves := []Wave{
		{
			ID:            "auth-w1",
			ClusterName:   "Auth",
			Title:         "Deps",
			Status:        "completed",
			Prerequisites: []string{"Auth:auth-w0"},
			Actions: []WaveAction{
				{Type: "add_dependency", IssueID: "ENG-101", Description: "dep"},
				{Type: "add_dod", IssueID: "ENG-102", Description: "dod"},
			},
			Description: "Order dependencies first",
			Delta:       WaveDelta{Before: 0.20, After: 0.40},
		},
	}

	// when
	states := BuildWaveStates(waves)

	// then
	s := states[0]
	if len(s.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(s.Actions))
	}
	if s.Description != "Order dependencies first" {
		t.Errorf("expected description, got %s", s.Description)
	}
	if s.Delta.Before != 0.20 || s.Delta.After != 0.40 {
		t.Errorf("expected delta {0.20, 0.40}, got {%v, %v}", s.Delta.Before, s.Delta.After)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestBuildWaveStates_IncludesFullFields -v`
Expected: FAIL — current `BuildWaveStates` does not copy Actions, Description, Delta.

**Step 3: Write minimal implementation**

Update `BuildWaveStates` in `session.go`:

```go
func BuildWaveStates(waves []Wave) []WaveState {
	states := make([]WaveState, len(waves))
	for i, w := range waves {
		states[i] = WaveState{
			ID:            w.ID,
			ClusterName:   w.ClusterName,
			Title:         w.Title,
			Status:        w.Status,
			Prerequisites: w.Prerequisites,
			ActionCount:   len(w.Actions),
			Actions:       w.Actions,
			Description:   w.Description,
			Delta:         w.Delta,
		}
	}
	return states
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: ALL PASS (existing tests unaffected — they only check fields that were already persisted)

**Step 5: Commit**

```bash
git add session.go session_test.go
git commit -m "feat(v0.5): persist full wave data in BuildWaveStates"
```

---

## Task 3: Add RestoreWaves function

**Files:**
- Modify: `session.go`
- Test: `session_test.go`

**Step 1: Write the failing test**

Add to `session_test.go`:

```go
func TestRestoreWaves_ConvertsWaveStatesToWaves(t *testing.T) {
	// given
	states := []WaveState{
		{
			ID:            "auth-w1",
			ClusterName:   "Auth",
			Title:         "Deps",
			Status:        "completed",
			Prerequisites: []string{"Auth:auth-w0"},
			ActionCount:   2,
			Actions: []WaveAction{
				{Type: "add_dependency", IssueID: "ENG-101", Description: "dep"},
				{Type: "add_dod", IssueID: "ENG-102", Description: "dod"},
			},
			Description: "Order dependencies first",
			Delta:       WaveDelta{Before: 0.20, After: 0.40},
		},
		{
			ID:          "auth-w2",
			ClusterName: "Auth",
			Title:       "DoD",
			Status:      "available",
			ActionCount: 1,
			Actions:     []WaveAction{{Type: "add_dod", IssueID: "ENG-103", Description: "dod2"}},
			Delta:       WaveDelta{Before: 0.40, After: 0.60},
		},
	}

	// when
	waves := RestoreWaves(states)

	// then
	if len(waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(waves))
	}
	w := waves[0]
	if w.ID != "auth-w1" {
		t.Errorf("expected auth-w1, got %s", w.ID)
	}
	if w.ClusterName != "Auth" {
		t.Errorf("expected Auth, got %s", w.ClusterName)
	}
	if w.Status != "completed" {
		t.Errorf("expected completed, got %s", w.Status)
	}
	if len(w.Actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(w.Actions))
	}
	if w.Description != "Order dependencies first" {
		t.Errorf("expected description, got %s", w.Description)
	}
	if w.Delta.Before != 0.20 {
		t.Errorf("expected delta before 0.20, got %v", w.Delta.Before)
	}
}

func TestRestoreWaves_EmptyInput(t *testing.T) {
	// given
	var states []WaveState

	// when
	waves := RestoreWaves(states)

	// then
	if waves == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(waves) != 0 {
		t.Errorf("expected empty slice, got %d", len(waves))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestRestoreWaves -v`
Expected: FAIL — `RestoreWaves` undefined.

**Step 3: Write minimal implementation**

Add to `session.go`:

```go
// RestoreWaves converts persisted WaveState list back into Wave list for session resume.
func RestoreWaves(states []WaveState) []Wave {
	waves := make([]Wave, len(states))
	for i, s := range states {
		waves[i] = Wave{
			ID:            s.ID,
			ClusterName:   s.ClusterName,
			Title:         s.Title,
			Description:   s.Description,
			Actions:       s.Actions,
			Prerequisites: s.Prerequisites,
			Delta:         s.Delta,
			Status:        s.Status,
		}
	}
	return waves
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add session.go session_test.go
git commit -m "feat(v0.5): add RestoreWaves to convert WaveState[] to Wave[]"
```

---

## Task 4: Add ScanResult caching (WriteScanResult / LoadScanResult)

**Files:**
- Modify: `state.go`
- Test: `state_test.go`

**Step 1: Write the failing test**

Add to `state_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run "TestWriteAndLoadScanResult|TestLoadScanResult" -v`
Expected: FAIL — `WriteScanResult`, `LoadScanResult` undefined.

**Step 3: Write minimal implementation**

Add to `state.go`:

```go
// WriteScanResult serializes a ScanResult to a JSON file for session resume caching.
func WriteScanResult(path string, result *ScanResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal scan result: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write scan result: %w", err)
	}
	return nil
}

// LoadScanResult reads a cached ScanResult from a JSON file.
func LoadScanResult(path string) (*ScanResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scan result: %w", err)
	}
	var result ScanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse scan result: %w", err)
	}
	return &result, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add state.go state_test.go
git commit -m "feat(v0.5): add WriteScanResult/LoadScanResult for ScanResult caching"
```

---

## Task 5: Add ScanResultPath to SessionState

**Files:**
- Modify: `model.go:61-70`
- Test: `state_test.go`

**Step 1: Write the failing test**

Add to `state_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestSessionState_ScanResultPath -v`
Expected: FAIL — `ScanResultPath` field undefined.

**Step 3: Write minimal implementation**

Add field to `SessionState` in `model.go`:

```go
type SessionState struct {
	Version        string         `json:"version"`
	SessionID      string         `json:"session_id"`
	Project        string         `json:"project"`
	LastScanned    time.Time      `json:"last_scanned"`
	Completeness   float64        `json:"completeness"`
	Clusters       []ClusterState `json:"clusters"`
	Waves          []WaveState    `json:"waves,omitempty"`
	ADRCount       int            `json:"adr_count,omitempty"`
	ScanResultPath string         `json:"scan_result_path,omitempty"`
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add model.go state_test.go
git commit -m "feat(v0.5): add ScanResultPath to SessionState"
```

---

## Task 6: Cache ScanResult after scan in RunSession

**Files:**
- Modify: `session.go:14-64` (inside `RunSession`, after `RunScan` succeeds)

**Step 1: Write the failing test**

Add to `session_test.go`:

```go
func TestRunSession_DryRunDoesNotCacheScanResult(t *testing.T) {
	// given: dry-run should NOT write scan_result.json (no real scan happened)
	baseDir := t.TempDir()
	cfg := &Config{
		Lang:   "en",
		Claude: ClaudeConfig{Command: "claude", TimeoutSec: 60},
		Scan:   ScanConfig{MaxConcurrency: 1, ChunkSize: 50},
		Linear: LinearConfig{Team: "ENG", Project: "Test"},
		Scribe: ScribeConfig{Enabled: true},
	}
	sessionID := "test-no-cache"
	ctx := context.Background()

	// when
	err := RunSession(ctx, cfg, baseDir, sessionID, true, nil)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	scanDir := ScanDir(baseDir, sessionID)
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	if _, err := os.Stat(scanResultPath); !os.IsNotExist(err) {
		t.Error("scan_result.json should not exist in dry-run mode")
	}
}
```

**Step 2: Run test to verify it passes (green baseline)**

Run: `go test ./... -run TestRunSession_DryRunDoesNotCacheScanResult -v`
Expected: PASS (dry-run returns before writing anything — confirms no regression)

This test establishes the dry-run boundary. The actual caching code runs only in non-dry-run, which cannot be tested without a live Claude subprocess. The caching logic will be verified through integration with Task 4's `WriteScanResult` tests.

**Step 3: Add caching to RunSession**

In `session.go`, after `RunScan` returns successfully (line ~29), before `RunWaveGenerate`:

```go
// Cache ScanResult for resume
scanResultPath := filepath.Join(scanDir, "scan_result.json")
if err := WriteScanResult(scanResultPath, scanResult); err != nil {
	LogWarn("Failed to cache scan result: %v", err)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add session.go session_test.go
git commit -m "feat(v0.5): cache ScanResult to scan_result.json after scan"
```

---

## Task 7: Add PromptResume CLI function

**Files:**
- Modify: `cli.go`
- Test: `cli_test.go`

**Step 1: Write the failing tests**

Add to `cli_test.go`:

```go
func TestPromptResume_ChooseResume(t *testing.T) {
	// given
	state := &SessionState{
		Completeness: 0.62,
		ADRCount:     4,
		LastScanned:  time.Date(2026, 2, 17, 15, 30, 0, 0, time.UTC),
	}
	input := "r\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when
	choice, err := PromptResume(ctx, &output, scanner, state)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != ResumeChoiceResume {
		t.Errorf("expected ResumeChoiceResume, got %d", choice)
	}
	if !strings.Contains(output.String(), "62%") {
		t.Error("expected completeness in prompt")
	}
	if !strings.Contains(output.String(), "4 ADRs") {
		t.Error("expected ADR count in prompt")
	}
}

func TestPromptResume_ChooseNew(t *testing.T) {
	// given
	state := &SessionState{Completeness: 0.30, ADRCount: 1, LastScanned: time.Now()}
	input := "n\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when
	choice, err := PromptResume(ctx, &output, scanner, state)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != ResumeChoiceNew {
		t.Errorf("expected ResumeChoiceNew, got %d", choice)
	}
}

func TestPromptResume_ChooseRescan(t *testing.T) {
	// given
	state := &SessionState{Completeness: 0.50, ADRCount: 2, LastScanned: time.Now()}
	input := "s\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when
	choice, err := PromptResume(ctx, &output, scanner, state)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != ResumeChoiceRescan {
		t.Errorf("expected ResumeChoiceRescan, got %d", choice)
	}
}

func TestPromptResume_ChooseQuit(t *testing.T) {
	// given
	state := &SessionState{Completeness: 0.50, LastScanned: time.Now()}
	input := "q\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when
	_, err := PromptResume(ctx, &output, scanner, state)

	// then
	if err != ErrQuit {
		t.Errorf("expected ErrQuit, got %v", err)
	}
}

func TestPromptResume_InvalidInput(t *testing.T) {
	// given
	state := &SessionState{Completeness: 0.50, LastScanned: time.Now()}
	input := "x\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when
	_, err := PromptResume(ctx, &output, scanner, state)

	// then
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
	if err == ErrQuit {
		t.Error("should not be ErrQuit for invalid input")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestPromptResume -v`
Expected: FAIL — `PromptResume`, `ResumeChoiceResume`, etc. undefined.

**Step 3: Write minimal implementation**

Add to `model.go` (after `ApprovalChoice`):

```go
type ResumeChoice int

const (
	ResumeChoiceResume ResumeChoice = iota
	ResumeChoiceNew
	ResumeChoiceRescan
)
```

Add to `cli.go`:

```go
// PromptResume displays previous session info and asks the user to resume, start new, or re-scan.
func PromptResume(ctx context.Context, w io.Writer, s *bufio.Scanner, state *SessionState) (ResumeChoice, error) {
	completePct := int(state.Completeness * 100)
	fmt.Fprintf(w, "\n  Previous session found (%d%% complete, %d ADRs)\n", completePct, state.ADRCount)
	fmt.Fprintf(w, "  Last scan: %s\n\n", state.LastScanned.Format("2006-01-02 15:04"))
	fmt.Fprintln(w, "  [r] Resume session")
	fmt.Fprintln(w, "  [n] Start new session")
	fmt.Fprintln(w, "  [s] Re-scan Linear and resume")
	fmt.Fprint(w, "\n  Choice: ")

	line, err := ScanLine(ctx, s)
	if err != nil {
		return ResumeChoiceResume, ErrQuit
	}
	input := strings.TrimSpace(strings.ToLower(line))
	switch input {
	case "r":
		return ResumeChoiceResume, nil
	case "n":
		return ResumeChoiceNew, nil
	case "s":
		return ResumeChoiceRescan, nil
	case "q":
		return ResumeChoiceResume, ErrQuit
	default:
		return ResumeChoiceResume, fmt.Errorf("invalid input: %s", input)
	}
}
```

Note: `cli_test.go` needs imports: `"bufio"`, `"bytes"`, `"context"`, `"strings"`, `"time"`.

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add model.go cli.go cli_test.go
git commit -m "feat(v0.5): add PromptResume with r/n/s/q choices"
```

---

## Task 8: Add MergeCompletedStatus for re-scan merge

**Files:**
- Modify: `session.go`
- Test: `session_test.go`

**Step 1: Write the failing tests**

Add to `session_test.go`:

```go
func TestMergeCompletedStatus_PreservesCompleted(t *testing.T) {
	// given: old completed waves
	oldCompleted := map[string]bool{
		"Auth:auth-w1": true,
		"API:api-w1":   true,
	}
	// given: new waves from re-scan (auth-w1 still exists, api-w2 is new)
	newWaves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "available"},
		{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "locked"},
		{ID: "api-w2", ClusterName: "API", Title: "New Wave", Status: "available"},
	}

	// when
	merged := MergeCompletedStatus(oldCompleted, newWaves)

	// then: auth-w1 should be completed (was in old)
	for _, w := range merged {
		if WaveKey(w) == "Auth:auth-w1" && w.Status != "completed" {
			t.Errorf("expected Auth:auth-w1 completed, got %s", w.Status)
		}
	}
	// then: api-w1 not in new waves (dropped from Linear) — not present at all
	for _, w := range merged {
		if WaveKey(w) == "API:api-w1" {
			t.Error("API:api-w1 should not appear in merged result")
		}
	}
	// then: auth-w2 and api-w2 keep original status
	for _, w := range merged {
		if WaveKey(w) == "Auth:auth-w2" && w.Status != "locked" {
			t.Errorf("expected Auth:auth-w2 locked, got %s", w.Status)
		}
		if WaveKey(w) == "API:api-w2" && w.Status != "available" {
			t.Errorf("expected API:api-w2 available, got %s", w.Status)
		}
	}
}

func TestMergeCompletedStatus_EmptyOld(t *testing.T) {
	// given: no old completed waves
	oldCompleted := map[string]bool{}
	newWaves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Status: "available"},
	}

	// when
	merged := MergeCompletedStatus(oldCompleted, newWaves)

	// then: all waves keep original status
	if len(merged) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(merged))
	}
	if merged[0].Status != "available" {
		t.Errorf("expected available, got %s", merged[0].Status)
	}
}

func TestMergeCompletedStatus_EmptyNew(t *testing.T) {
	// given: old waves completed but new scan returns nothing
	oldCompleted := map[string]bool{"Auth:auth-w1": true}
	var newWaves []Wave

	// when
	merged := MergeCompletedStatus(oldCompleted, newWaves)

	// then
	if len(merged) != 0 {
		t.Errorf("expected 0 waves, got %d", len(merged))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestMergeCompletedStatus -v`
Expected: FAIL — `MergeCompletedStatus` undefined.

**Step 3: Write minimal implementation**

Add to `session.go`:

```go
// MergeCompletedStatus preserves completed status from a previous session
// when waves are regenerated after a re-scan. Waves in newWaves that match
// a key in oldCompleted are marked "completed". Waves that were in the old
// session but not in newWaves are dropped (Linear removed them).
func MergeCompletedStatus(oldCompleted map[string]bool, newWaves []Wave) []Wave {
	result := make([]Wave, len(newWaves))
	copy(result, newWaves)
	for i, w := range result {
		if oldCompleted[WaveKey(w)] {
			result[i].Status = "completed"
		}
	}
	return result
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add session.go session_test.go
git commit -m "feat(v0.5): add MergeCompletedStatus for re-scan merge"
```

---

## Task 9: Add Navigator resume display line

**Files:**
- Modify: `navigator.go:63-131` (`RenderNavigatorWithWaves`)
- Test: `navigator_test.go`

**Step 1: Write the failing test**

Add to `navigator_test.go`:

```go
func TestRenderNavigatorWithWaves_ResumeInfo(t *testing.T) {
	// given
	result := &ScanResult{
		Clusters:     []ClusterScanResult{{Name: "Auth", Completeness: 0.62}},
		TotalIssues:  5,
		Completeness: 0.62,
	}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed"},
	}
	lastScanned := time.Date(2026, 2, 17, 15, 30, 0, 0, time.UTC)

	// when
	nav := RenderNavigatorWithWaves(result, "TestProject", waves, 3, &lastScanned)

	// then
	if !strings.Contains(nav, "Session: resumed") {
		t.Error("expected 'Session: resumed' in navigator")
	}
	if !strings.Contains(nav, "2026-02-17 15:30") {
		t.Error("expected last scan timestamp in navigator")
	}
}

func TestRenderNavigatorWithWaves_NoResumeInfo(t *testing.T) {
	// given: nil lastScanned means fresh session
	result := &ScanResult{
		Clusters:     []ClusterScanResult{{Name: "Auth", Completeness: 0.25}},
		TotalIssues:  3,
		Completeness: 0.25,
	}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "available"},
	}

	// when
	nav := RenderNavigatorWithWaves(result, "TestProject", waves, 0, nil)

	// then: no resume line
	if strings.Contains(nav, "Session:") {
		t.Error("should not contain 'Session:' for fresh session")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run "TestRenderNavigatorWithWaves_ResumeInfo|TestRenderNavigatorWithWaves_NoResumeInfo" -v`
Expected: FAIL — `RenderNavigatorWithWaves` has 4 args, tests pass 5.

**Step 3: Write minimal implementation**

Update `RenderNavigatorWithWaves` signature in `navigator.go`:

```go
func RenderNavigatorWithWaves(result *ScanResult, projectName string, waves []Wave, adrCount int, lastScanned *time.Time) string {
```

Add after the ADR row (line ~81) and before the border:

```go
if lastScanned != nil {
	sessionRow := fmt.Sprintf("  Session: resumed (last scan: %s)", lastScanned.Format("2006-01-02 15:04"))
	b.WriteString("|" + padRight(sessionRow, navigatorWidth) + "|\n")
}
```

Update all call sites to pass `nil` for fresh sessions:
- `session.go:89` — `RenderNavigatorWithWaves(scanResult, cfg.Linear.Project, waves, adrCount, nil)`
- `cmd/sightjack/main.go` — not used there (only `RenderNavigator` is used)

Update all existing tests in `navigator_test.go` to pass `nil` as 5th arg.

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add navigator.go navigator_test.go session.go
git commit -m "feat(v0.5): add session resume line to Navigator display"
```

---

## Task 10: Save state after each wave completion

**Files:**
- Modify: `session.go:80-198` (interactive loop)

Currently state is saved only at the end of session (lines 200-222). Design requires saving after each wave completion for crash resilience.

**Step 1: Extract state save into a helper**

First, write a test for the helper:

Add to `session_test.go`:

```go
func TestBuildSessionState(t *testing.T) {
	// given
	scanResult := &ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "Auth", Completeness: 0.50, Issues: make([]IssueDetail, 3)},
		},
		Completeness: 0.50,
	}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed",
			Actions: []WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "d"}},
			Delta:   WaveDelta{Before: 0.25, After: 0.50}},
	}
	cfg := &Config{Linear: LinearConfig{Project: "TestProject"}}
	sessionID := "test-123"
	adrCount := 2

	// when
	state := BuildSessionState(cfg, sessionID, scanResult, waves, adrCount)

	// then
	if state.Version != "0.5" {
		t.Errorf("expected version 0.5, got %s", state.Version)
	}
	if state.SessionID != "test-123" {
		t.Errorf("expected test-123, got %s", state.SessionID)
	}
	if state.Completeness != 0.50 {
		t.Errorf("expected 0.50, got %f", state.Completeness)
	}
	if state.ADRCount != 2 {
		t.Errorf("expected 2, got %d", state.ADRCount)
	}
	if len(state.Clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(state.Clusters))
	}
	if len(state.Waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(state.Waves))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestBuildSessionState -v`
Expected: FAIL — `BuildSessionState` undefined.

**Step 3: Write minimal implementation**

Add to `session.go`:

```go
// BuildSessionState creates a SessionState from current session data.
func BuildSessionState(cfg *Config, sessionID string, scanResult *ScanResult, waves []Wave, adrCount int) *SessionState {
	state := &SessionState{
		Version:      "0.5",
		SessionID:    sessionID,
		Project:      cfg.Linear.Project,
		LastScanned:  time.Now(),
		Completeness: scanResult.Completeness,
		Waves:        BuildWaveStates(waves),
		ADRCount:     adrCount,
	}
	for _, c := range scanResult.Clusters {
		state.Clusters = append(state.Clusters, ClusterState{
			Name:         c.Name,
			Completeness: c.Completeness,
			IssueCount:   len(c.Issues),
		})
	}
	return state
}
```

Then refactor `RunSession` to:
1. Use `BuildSessionState` for both the end-of-session save and the per-wave save
2. Add `saveState(baseDir, cfg, sessionID, scanResult, waves, adrCount, scanResultPath)` call after marking each wave completed (after line ~186)

Replace the existing state save block (lines 200-222) with:

```go
state := BuildSessionState(cfg, sessionID, scanResult, waves, adrCount)
state.ScanResultPath = scanResultPath
```

Add state save after wave completion (after line ~186, before `LogOK`):

```go
// Save state after each wave completion (crash resilience)
midState := BuildSessionState(cfg, sessionID, scanResult, waves, adrCount)
midState.ScanResultPath = scanResultPath
if err := WriteState(baseDir, midState); err != nil {
	LogWarn("Failed to save mid-session state: %v", err)
}
```

Note: `scanResultPath` needs to be declared at the top of the non-dry-run path (as a variable accessible in the loop).

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add session.go session_test.go
git commit -m "feat(v0.5): save state after each wave completion + extract BuildSessionState"
```

---

## Task 11: Wire resume flow into RunSession and main.go

**Files:**
- Modify: `session.go` (major change — add resume/new/re-scan paths)
- Modify: `cmd/sightjack/main.go` (detect state and call PromptResume)

This is the integration task. It connects Tasks 1-10.

**Step 1: Write the failing test**

Add to `session_test.go`:

```go
func TestResumeSession_RestoresWavesFromState(t *testing.T) {
	// given: a saved state with completed and available waves + cached scan result
	baseDir := t.TempDir()

	// Create scan result cache
	scanDir := ScanDir(baseDir, "old-session")
	os.MkdirAll(scanDir, 0755)
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	scanResult := &ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "Auth", Completeness: 0.50, Issues: []IssueDetail{
				{ID: "ENG-101", Identifier: "ENG-101", Title: "Login", Completeness: 0.50},
			}},
		},
		TotalIssues:  1,
		Completeness: 0.50,
	}
	if err := WriteScanResult(scanResultPath, scanResult); err != nil {
		t.Fatalf("write scan result: %v", err)
	}

	// Create state pointing to that scan result
	state := &SessionState{
		Version:        "0.5",
		SessionID:      "old-session",
		Project:        "TestProject",
		LastScanned:    time.Now(),
		Completeness:   0.50,
		ScanResultPath: scanResultPath,
		Clusters: []ClusterState{
			{Name: "Auth", Completeness: 0.50, IssueCount: 1},
		},
		Waves: []WaveState{
			{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed",
				ActionCount: 1,
				Actions:     []WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "d"}},
				Delta:       WaveDelta{Before: 0.25, After: 0.50}},
			{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "available",
				ActionCount: 1,
				Actions:     []WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "d2"}},
				Delta:       WaveDelta{Before: 0.50, After: 0.75}},
		},
		ADRCount: 2,
	}
	if err := WriteState(baseDir, state); err != nil {
		t.Fatalf("write state: %v", err)
	}

	// when: ResumeSession loads state and returns waves + scan result
	resumedScanResult, waves, completed, adrCount, err := ResumeSession(baseDir, state)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(waves))
	}
	if waves[0].Status != "completed" {
		t.Errorf("expected auth-w1 completed, got %s", waves[0].Status)
	}
	if !completed["Auth:auth-w1"] {
		t.Error("expected Auth:auth-w1 in completed map")
	}
	if resumedScanResult.Completeness != 0.50 {
		t.Errorf("expected completeness 0.50, got %f", resumedScanResult.Completeness)
	}
	if adrCount != 2 {
		t.Errorf("expected adrCount 2, got %d", adrCount)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestResumeSession -v`
Expected: FAIL — `ResumeSession` undefined.

**Step 3: Write minimal implementation**

Add to `session.go`:

```go
// ResumeSession loads a previous session's state and cached scan result,
// restoring waves and completed map for the interactive loop.
func ResumeSession(baseDir string, state *SessionState) (*ScanResult, []Wave, map[string]bool, int, error) {
	// Load cached scan result
	if state.ScanResultPath == "" {
		return nil, nil, nil, 0, fmt.Errorf("no cached scan result path in state")
	}
	scanResult, err := LoadScanResult(state.ScanResultPath)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("load cached scan result: %w", err)
	}

	// Restore waves from state
	waves := RestoreWaves(state.Waves)
	completed := BuildCompletedWaveMap(waves)

	return scanResult, waves, completed, state.ADRCount, nil
}
```

Then update `cmd/sightjack/main.go` in the `session` case to:
1. Try `ReadState(baseDir)` before calling `RunSession`
2. If state exists and not dry-run, call `PromptResume`
3. Based on choice: call `RunSession` (for new), `RunResumeSession` (for resume), or re-scan+merge

The exact wiring in `main.go`:

```go
case "session":
	// ... existing config loading ...

	// Check for existing state (resume detection)
	if !dryRun {
		existingState, stateErr := sightjack.ReadState(baseDir)
		if stateErr == nil {
			scanner := bufio.NewScanner(os.Stdin)
			choice, promptErr := sightjack.PromptResume(ctx, os.Stdout, scanner, existingState)
			if promptErr == sightjack.ErrQuit {
				return
			}
			if promptErr != nil {
				sightjack.LogWarn("Invalid input: %v", promptErr)
				// Fall through to fresh session
			} else {
				switch choice {
				case sightjack.ResumeChoiceResume:
					if err := sightjack.RunResumeSession(ctx, cfg, baseDir, existingState, os.Stdin); err != nil {
						sightjack.LogError("Resume failed: %v", err)
						os.Exit(1)
					}
					return
				case sightjack.ResumeChoiceRescan:
					if err := sightjack.RunRescanSession(ctx, cfg, baseDir, existingState, os.Stdin); err != nil {
						sightjack.LogError("Re-scan resume failed: %v", err)
						os.Exit(1)
					}
					return
				case sightjack.ResumeChoiceNew:
					// Fall through to fresh session below
				}
			}
		}
	}

	// Fresh session (existing code)
	sessionID := fmt.Sprintf("session-%d-%d", time.Now().UnixMilli(), os.Getpid())
	// ... rest of existing session code ...
```

**Step 4: Implement RunResumeSession and RunRescanSession**

Add to `session.go`:

```go
// RunResumeSession resumes an existing session from saved state.
func RunResumeSession(ctx context.Context, cfg *Config, baseDir string, state *SessionState, input io.Reader) error {
	if input == nil {
		return fmt.Errorf("input reader is required for interactive session")
	}

	scanResult, waves, completed, adrCount, err := ResumeSession(baseDir, state)
	if err != nil {
		return fmt.Errorf("resume: %w", err)
	}

	scanDir := ScanDir(baseDir, state.SessionID)
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	scanner := bufio.NewScanner(input)
	adrDir := ADRDir(baseDir)
	lastScanned := state.LastScanned

	LogOK("Resumed session: %d waves, %d completed", len(waves), len(completed))

	return runInteractiveLoop(ctx, cfg, baseDir, state.SessionID, scanDir, scanResultPath, scanResult, waves, completed, adrCount, scanner, adrDir, &lastScanned)
}

// RunRescanSession performs a fresh scan then merges completed status from old state.
func RunRescanSession(ctx context.Context, cfg *Config, baseDir string, oldState *SessionState, input io.Reader) error {
	if input == nil {
		return fmt.Errorf("input reader is required for interactive session")
	}

	sessionID := fmt.Sprintf("session-%d-%d", time.Now().UnixMilli(), os.Getpid())
	scanDir, err := EnsureScanDir(baseDir, sessionID)
	if err != nil {
		return err
	}

	// Fresh scan
	scanResult, err := RunScan(ctx, cfg, baseDir, sessionID, false)
	if err != nil {
		return fmt.Errorf("re-scan: %w", err)
	}

	// Cache scan result
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	if err := WriteScanResult(scanResultPath, scanResult); err != nil {
		LogWarn("Failed to cache scan result: %v", err)
	}

	// Generate waves
	waves, err := RunWaveGenerate(ctx, cfg, scanDir, scanResult.Clusters, false)
	if err != nil {
		return fmt.Errorf("wave generate: %w", err)
	}

	// Merge completed status from old session
	oldCompleted := BuildCompletedWaveMap(RestoreWaves(oldState.Waves))
	waves = MergeCompletedStatus(oldCompleted, waves)
	waves = EvaluateUnlocks(waves, BuildCompletedWaveMap(waves))

	completed := BuildCompletedWaveMap(waves)
	adrCount := oldState.ADRCount
	scanner := bufio.NewScanner(input)
	adrDir := ADRDir(baseDir)

	LogOK("Re-scanned: %d clusters, %d waves (%d previously completed)",
		len(scanResult.Clusters), len(waves), len(completed))

	return runInteractiveLoop(ctx, cfg, baseDir, sessionID, scanDir, scanResultPath, scanResult, waves, completed, adrCount, scanner, adrDir, nil)
}
```

**Step 5: Extract interactive loop from RunSession**

Extract the interactive loop (current lines 80-198) into:

```go
func runInteractiveLoop(ctx context.Context, cfg *Config, baseDir, sessionID, scanDir, scanResultPath string,
	scanResult *ScanResult, waves []Wave, completed map[string]bool, adrCount int,
	scanner *bufio.Scanner, adrDir string, lastScanned *time.Time) error {
	// ... existing interactive loop code, using the params instead of local vars ...
	// Navigator call uses lastScanned param
	// State save uses BuildSessionState + scanResultPath
}
```

This refactoring keeps `RunSession` as the fresh-session path while reusing the loop.

**Step 4 (continued): Run test to verify it passes**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add session.go session_test.go cmd/sightjack/main.go
git commit -m "feat(v0.5): wire resume/new/re-scan flow into session and main"
```

---

## Task 12: Update version string

**Files:**
- Modify: `cmd/sightjack/main.go:17`

**Step 1: Update version**

Change: `var version = "0.3.0-dev"` to `var version = "0.5.0-dev"`

**Step 2: Update state version in BuildSessionState**

Already done in Task 10 — `Version: "0.5"`.

Also update `RunSession` end-of-session save to use `BuildSessionState` (Task 10 refactoring ensures this).

**Step 3: Run tests**

Run: `go test ./... -v && go build ./...`
Expected: ALL PASS, builds clean.

**Step 4: Commit**

```bash
git add cmd/sightjack/main.go
git commit -m "chore: bump version to 0.5.0-dev"
```

---

## Task 13: Full integration verification

**Step 1: Run all tests**

```bash
go test ./... -v
```

Expected: ALL PASS

**Step 2: Run vet**

```bash
go vet ./...
```

Expected: No issues

**Step 3: Build**

```bash
go build ./...
```

Expected: Clean compile

**Step 4: Dry-run test**

```bash
go run ./cmd/sightjack session --dry-run
```

Expected: Generates all prompts in `.siren/scans/`

---

## Dependency Graph

```
Task 1  --> Task 2  --> Task 3
                    \
Task 4  --> Task 5   \
                      +--> Task 10 --> Task 11
Task 7  ---------------/
Task 8  ---------------/
Task 9  ---------------/
Task 6 (standalone)

Task 12 (after Task 11)
Task 13 (after all)
```

Legend:
- Task 1-3: WaveState expansion + RestoreWaves
- Task 4-5: ScanResult caching
- Task 6: cache in RunSession
- Task 7: PromptResume CLI
- Task 8: MergeCompletedStatus
- Task 9: Navigator resume display
- Task 10: BuildSessionState + per-wave save
- Task 11: Integration (wires everything together)
- Task 12: Version bump
- Task 13: Full verification
