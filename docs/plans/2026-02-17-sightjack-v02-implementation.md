# Sightjack v0.2 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the SIREN experience loop — Wave generation, interactive selection, approve/apply to Linear, ripple effects display.

**Architecture:** 4-Pass Session extending v0.1's 2-Pass. Pass 1-2 reused. Pass 3 generates Waves per cluster. Pass 4 applies approved Waves via Claude Code + Linear MCP. Interactive CLI loop between Pass 3 and 4. All Claude Code communication is file-based JSON.

**Tech Stack:** Go 1.22+, `golang.org/x/sync/errgroup`, `text/template`, `embed.FS`, `bufio.Scanner` for stdin, `encoding/json`

**Design doc:** `docs/plans/2026-02-17-sightjack-v02-design.md`

---

### Task 1: Wave Data Model

Add Wave-related types to model.go. These are the foundation for everything else.

**Files:**
- Modify: `model.go`
- Test: `model_test.go`

**Step 1: Write failing tests for Wave JSON unmarshalling**

Add to `model_test.go`:

```go
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
```

**Step 2: Run tests to verify they fail**

Run: `go test -run "TestWave_UnmarshalJSON|TestWaveGenerateResult_UnmarshalJSON|TestWaveApplyResult_UnmarshalJSON" -v ./...`
Expected: FAIL (types not defined)

**Step 3: Add Wave types to model.go**

Add after the existing types in `model.go`:

```go
// Wave is a unit of work proposed by AI for a cluster.
type Wave struct {
	ID            string       `json:"id"`
	ClusterName   string       `json:"cluster_name"`
	Title         string       `json:"title"`
	Description   string       `json:"description"`
	Actions       []WaveAction `json:"actions"`
	Prerequisites []string     `json:"prerequisites"`
	Delta         WaveDelta    `json:"delta"`
	Status        string       `json:"status"`
}

// WaveAction is a single change proposed within a Wave.
type WaveAction struct {
	Type        string `json:"type"`
	IssueID     string `json:"issue_id"`
	Description string `json:"description"`
	Detail      string `json:"detail"`
}

// WaveDelta holds expected completeness change.
type WaveDelta struct {
	Before float64 `json:"before"`
	After  float64 `json:"after"`
}

// WaveGenerateResult is the Pass 3 output per cluster.
type WaveGenerateResult struct {
	ClusterName string `json:"cluster_name"`
	Waves       []Wave `json:"waves"`
}

// WaveApplyResult is the Pass 4 output per wave.
type WaveApplyResult struct {
	WaveID  string   `json:"wave_id"`
	Applied int      `json:"applied"`
	Errors  []string `json:"errors"`
	Ripples []Ripple `json:"ripples"`
}

// Ripple is a cross-cluster effect from applying a wave.
type Ripple struct {
	ClusterName string `json:"cluster_name"`
	Description string `json:"description"`
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -run "TestWave_UnmarshalJSON|TestWaveGenerateResult_UnmarshalJSON|TestWaveApplyResult_UnmarshalJSON" -v ./...`
Expected: PASS

**Step 5: Run all tests**

Run: `go test ./...`
Expected: All pass (no regressions)

**Step 6: Commit**

```bash
git add model.go model_test.go
git commit -m "feat(v0.2): add Wave data model types"
```

---

### Task 2: Wave Parsing and Unlock Logic

Create `wave.go` with file parsing functions and unlock evaluation.

**Files:**
- Create: `wave.go`
- Create: `wave_test.go`

**Step 1: Write failing tests**

Create `wave_test.go`:

```go
package sightjack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseWaveGenerateResult(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wave_auth.json")
	content := `{
		"cluster_name": "Auth",
		"waves": [
			{"id": "auth-w1", "cluster_name": "Auth", "title": "Deps", "actions": [], "prerequisites": [], "delta": {"before": 0.25, "after": 0.40}, "status": "available"}
		]
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ParseWaveGenerateResult(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ClusterName != "Auth" {
		t.Errorf("expected Auth, got %s", result.ClusterName)
	}
	if len(result.Waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(result.Waves))
	}
}

func TestParseWaveApplyResult(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "apply_auth-w1.json")
	content := `{
		"wave_id": "auth-w1",
		"applied": 5,
		"errors": [],
		"ripples": [{"cluster_name": "API", "description": "W2 unlocked"}]
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ParseWaveApplyResult(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Applied != 5 {
		t.Errorf("expected 5, got %d", result.Applied)
	}
}

func TestAvailableWaves(t *testing.T) {
	waves := []Wave{
		{ID: "auth-w1", Status: "available", Prerequisites: nil},
		{ID: "auth-w2", Status: "locked", Prerequisites: []string{"auth-w1"}},
		{ID: "api-w1", Status: "available", Prerequisites: nil},
		{ID: "api-w2", Status: "locked", Prerequisites: []string{"api-w1", "auth-w1"}},
	}
	completed := map[string]bool{}

	available := AvailableWaves(waves, completed)
	if len(available) != 2 {
		t.Fatalf("expected 2 available, got %d", len(available))
	}

	// After completing auth-w1
	completed["auth-w1"] = true
	waves = EvaluateUnlocks(waves, completed)
	available = AvailableWaves(waves, completed)

	// auth-w2 should be unlocked now (prereq auth-w1 met)
	// api-w2 still locked (needs api-w1 too)
	if len(available) != 2 {
		t.Fatalf("expected 2 available after auth-w1 complete, got %d", len(available))
	}
	// Find auth-w2 in available
	found := false
	for _, w := range available {
		if w.ID == "auth-w2" {
			found = true
		}
	}
	if !found {
		t.Error("expected auth-w2 to be available")
	}
}

func TestEvaluateUnlocks(t *testing.T) {
	waves := []Wave{
		{ID: "a-w1", Status: "completed"},
		{ID: "a-w2", Status: "locked", Prerequisites: []string{"a-w1"}},
		{ID: "b-w1", Status: "locked", Prerequisites: []string{"a-w1", "a-w2"}},
	}
	completed := map[string]bool{"a-w1": true}

	updated := EvaluateUnlocks(waves, completed)

	if updated[1].Status != "available" {
		t.Errorf("expected a-w2 available, got %s", updated[1].Status)
	}
	if updated[2].Status != "locked" {
		t.Errorf("expected b-w1 still locked, got %s", updated[2].Status)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -run "TestParseWaveGenerateResult|TestParseWaveApplyResult|TestAvailableWaves|TestEvaluateUnlocks" -v ./...`
Expected: FAIL (functions not defined)

**Step 3: Implement wave.go**

Create `wave.go`:

```go
package sightjack

import (
	"encoding/json"
	"fmt"
	"os"
)

// ParseWaveGenerateResult reads and parses a wave_{name}.json output file.
func ParseWaveGenerateResult(path string) (*WaveGenerateResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read wave result: %w", err)
	}
	var result WaveGenerateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse wave result: %w", err)
	}
	return &result, nil
}

// ParseWaveApplyResult reads and parses an apply_{wave_id}.json output file.
func ParseWaveApplyResult(path string) (*WaveApplyResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read apply result: %w", err)
	}
	var result WaveApplyResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse apply result: %w", err)
	}
	return &result, nil
}

// AvailableWaves returns waves that have "available" status and are not completed.
func AvailableWaves(waves []Wave, completed map[string]bool) []Wave {
	var available []Wave
	for _, w := range waves {
		if w.Status == "available" && !completed[w.ID] {
			available = append(available, w)
		}
	}
	return available
}

// EvaluateUnlocks checks locked waves and unlocks them if all prerequisites are met.
func EvaluateUnlocks(waves []Wave, completed map[string]bool) []Wave {
	result := make([]Wave, len(waves))
	copy(result, waves)
	for i, w := range result {
		if w.Status != "locked" {
			continue
		}
		allMet := true
		for _, prereq := range w.Prerequisites {
			if !completed[prereq] {
				allMet = false
				break
			}
		}
		if allMet {
			result[i].Status = "available"
		}
	}
	return result
}
```

**Step 4: Run tests**

Run: `go test -run "TestParseWaveGenerateResult|TestParseWaveApplyResult|TestAvailableWaves|TestEvaluateUnlocks" -v ./...`
Expected: PASS

**Step 5: Run all tests**

Run: `go test ./...`
Expected: All pass

**Step 6: Commit**

```bash
git add wave.go wave_test.go
git commit -m "feat(v0.2): add wave parsing and unlock logic"
```

---

### Task 3: State Extension with WaveState

Extend SessionState to include wave data so sessions can be resumed.

**Files:**
- Modify: `model.go`
- Modify: `state.go`
- Modify: `state_test.go`

**Step 1: Write failing test for state with waves**

Add to `state_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test -run "TestState_WriteAndRead_WithWaves" -v ./...`
Expected: FAIL (WaveState type not defined, Waves field missing)

**Step 3: Add WaveState to model.go and Waves field to SessionState**

Add `WaveState` type to `model.go`:

```go
// WaveState is the per-wave state within SessionState.
type WaveState struct {
	ID            string   `json:"id"`
	ClusterName   string   `json:"cluster_name"`
	Title         string   `json:"title"`
	Status        string   `json:"status"`
	Prerequisites []string `json:"prerequisites,omitempty"`
	ActionCount   int      `json:"action_count"`
}
```

Add `Waves` field to `SessionState`:

```go
type SessionState struct {
	Version      string         `json:"version"`
	SessionID    string         `json:"session_id"`
	Project      string         `json:"project"`
	LastScanned  time.Time      `json:"last_scanned"`
	Completeness float64        `json:"completeness"`
	Clusters     []ClusterState `json:"clusters"`
	Waves        []WaveState    `json:"waves,omitempty"`
}
```

**Step 4: Run tests**

Run: `go test -run "TestState_WriteAndRead_WithWaves" -v ./...`
Expected: PASS

**Step 5: Run all tests (check v0.1 state tests still pass)**

Run: `go test ./...`
Expected: All pass (existing state tests must not break)

**Step 6: Commit**

```bash
git add model.go state_test.go
git commit -m "feat(v0.2): extend SessionState with WaveState"
```

---

### Task 4: Wave Prompt Templates and Rendering

Add prompt templates for Pass 3 (Wave Generate) and Pass 4 (Wave Apply), and their rendering functions.

**Files:**
- Create: `prompts/templates/wave_generate_ja.md.tmpl`
- Create: `prompts/templates/wave_generate_en.md.tmpl`
- Create: `prompts/templates/wave_apply_ja.md.tmpl`
- Create: `prompts/templates/wave_apply_en.md.tmpl`
- Modify: `prompt.go`
- Modify: `prompt_test.go`

**Step 1: Write failing tests**

Add to `prompt_test.go`:

```go
func TestRenderWaveGeneratePrompt(t *testing.T) {
	data := WaveGeneratePromptData{
		ClusterName:  "Auth",
		Completeness: 0.25,
		Issues:       `[{"id":"ENG-101","title":"Login","completeness":0.3,"gaps":["No DoD"]}]`,
		Observations: "Cross-cluster dependency detected",
		OutputPath:   "/tmp/wave_auth.json",
	}
	result, err := RenderWaveGeneratePrompt("ja", data)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "Auth") {
		t.Error("expected cluster name in output")
	}
	if !strings.Contains(result, "/tmp/wave_auth.json") {
		t.Error("expected output path in output")
	}
}

func TestRenderWaveApplyPrompt(t *testing.T) {
	data := WaveApplyPromptData{
		WaveID:      "auth-w1",
		ClusterName: "Auth",
		Title:       "Dependency Ordering",
		Actions:     `[{"type":"add_dependency","issue_id":"ENG-101","description":"Auth before token","detail":"ENG-101 -> ENG-102"}]`,
		OutputPath:  "/tmp/apply_auth-w1.json",
	}
	result, err := RenderWaveApplyPrompt("ja", data)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "auth-w1") {
		t.Error("expected wave ID in output")
	}
	if !strings.Contains(result, "/tmp/apply_auth-w1.json") {
		t.Error("expected output path in output")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -run "TestRenderWaveGeneratePrompt|TestRenderWaveApplyPrompt" -v ./...`
Expected: FAIL

**Step 3: Create template files**

`prompts/templates/wave_generate_ja.md.tmpl`:

```
あなたは Scanner Agent です。
以下のクラスタの分析結果に基づいて、Issue群の完成度を向上させるための Wave（作業塊）を提案してください。

## 対象クラスタ
- クラスタ名: {{.ClusterName}}
- 現在の完成度: {{printf "%.0f" (mul .Completeness 100)}}%

## Issue分析結果
{{.Issues}}

## 観察事項
{{.Observations}}

## Wave提案の指針
- 各Waveは「1回の作業セッション」で完了できる粒度
- Wave 1は前提条件なし（即座に着手可能）
- Wave 2以降はWave 1の完了を前提にしてよい
- 各Waveには具体的なアクション（DoD追記、依存関係追加、ラベル付与等）を含める

## アクションタイプ
- `add_dod`: IssueのDescriptionにDoD項目を追記
- `add_dependency`: Issue間の依存関係を設定
- `add_label`: Issueにラベルを付与
- `update_description`: Issueの説明を更新

## 出力
以下の JSON を **{{.OutputPath}}** に書き込んでください:

` + "```" + `json
{
  "cluster_name": "{{.ClusterName}}",
  "waves": [
    {
      "id": "cluster-w1",
      "cluster_name": "{{.ClusterName}}",
      "title": "Wave名",
      "description": "このWaveで何をやるか",
      "actions": [
        {"type": "add_dod", "issue_id": "issue-id", "description": "人間向け説明", "detail": "追記する具体的な内容"}
      ],
      "prerequisites": [],
      "delta": {"before": 0.25, "after": 0.40},
      "status": "available"
    }
  ]
}
` + "```" + `

重要: 出力は上記のファイルパスに直接書き込んでください。標準出力には書かないでください。
```

`prompts/templates/wave_generate_en.md.tmpl`: Same content in English.

`prompts/templates/wave_apply_ja.md.tmpl`:

```
あなたは Scanner Agent です。
承認されたWaveのアクションをLinear MCP Server経由でIssueに適用してください。

## Wave情報
- Wave ID: {{.WaveID}}
- クラスタ: {{.ClusterName}}
- タイトル: {{.Title}}

## 適用するアクション
{{.Actions}}

## 適用手順
各アクションについて:
1. `add_dod`: IssueのDescriptionにDoD項目を追記する
2. `add_dependency`: Linear MCPでIssue間の関連を設定する
3. `add_label`: Linear MCPでラベルを付与する
4. `update_description`: IssueのDescriptionを更新する

適用後、他のクラスタへの波及効果（ripples）があれば記録してください。

## 出力
以下の JSON を **{{.OutputPath}}** に書き込んでください:

` + "```" + `json
{
  "wave_id": "{{.WaveID}}",
  "applied": 7,
  "errors": [],
  "ripples": [
    {"cluster_name": "API", "description": "W2の前提条件が満たされた"}
  ]
}
` + "```" + `

重要: 出力は上記のファイルパスに直接書き込んでください。標準出力には書かないでください。
```

`prompts/templates/wave_apply_en.md.tmpl`: Same content in English.

**Step 4: Add prompt data types and render functions to prompt.go**

```go
// WaveGeneratePromptData holds template data for the wave generation prompt.
type WaveGeneratePromptData struct {
	ClusterName  string
	Completeness float64
	Issues       string
	Observations string
	OutputPath   string
}

// WaveApplyPromptData holds template data for the wave apply prompt.
type WaveApplyPromptData struct {
	WaveID      string
	ClusterName string
	Title       string
	Actions     string
	OutputPath  string
}

// RenderWaveGeneratePrompt renders the wave generation prompt for the given language.
func RenderWaveGeneratePrompt(lang string, data WaveGeneratePromptData) (string, error) {
	name := fmt.Sprintf("prompts/templates/wave_generate_%s.md.tmpl", lang)
	return renderTemplate(name, data)
}

// RenderWaveApplyPrompt renders the wave apply prompt for the given language.
func RenderWaveApplyPrompt(lang string, data WaveApplyPromptData) (string, error) {
	name := fmt.Sprintf("prompts/templates/wave_apply_%s.md.tmpl", lang)
	return renderTemplate(name, data)
}
```

**Note on template syntax:** The `{{printf "%.0f" (mul .Completeness 100)}}` requires a `mul` template function. If Go templates don't support this natively, use `fmt.Sprintf("%.0f", data.Completeness*100)` in the Go code and pass it as a string field `CompletenessPercent` instead. Choose the simpler approach.

**Step 5: Run tests**

Run: `go test -run "TestRenderWaveGeneratePrompt|TestRenderWaveApplyPrompt" -v ./...`
Expected: PASS

**Step 6: Run all tests**

Run: `go test ./...`
Expected: All pass

**Step 7: Commit**

```bash
git add prompt.go prompt_test.go prompts/templates/wave_*.tmpl
git commit -m "feat(v0.2): add wave generation and apply prompt templates"
```

---

### Task 5: Navigator with Wave Data

Modify `RenderNavigator` to accept wave data and display actual wave states instead of placeholder `[]`.

**Files:**
- Modify: `navigator.go`
- Modify: `navigator_test.go`

**Step 1: Write failing test**

Add to `navigator_test.go`:

```go
func TestRenderNavigator_WithWaves(t *testing.T) {
	result := &ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "Auth", Completeness: 0.25},
			{Name: "API", Completeness: 0.30},
		},
		TotalIssues:  10,
		Completeness: 0.275,
	}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "available"},
		{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "locked"},
		{ID: "api-w1", ClusterName: "API", Title: "Split", Status: "available"},
	}

	nav := RenderNavigatorWithWaves(result, "TestProject", waves)

	if !strings.Contains(nav, "[ ]") {
		t.Error("expected [ ] for available wave")
	}
	if !strings.Contains(nav, "[x]") {
		t.Error("expected [x] for locked wave")
	}
	if !strings.Contains(nav, "Deps") {
		t.Error("expected wave title 'Deps' in output")
	}
}

func TestRenderNavigator_WithCompletedWave(t *testing.T) {
	result := &ScanResult{
		Clusters:     []ClusterScanResult{{Name: "Auth", Completeness: 0.40}},
		TotalIssues:  4,
		Completeness: 0.40,
	}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed"},
		{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "available"},
	}

	nav := RenderNavigatorWithWaves(result, "TestProject", waves)

	if !strings.Contains(nav, "[=]") {
		t.Error("expected [=] for completed wave")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -run "TestRenderNavigator_WithWaves|TestRenderNavigator_WithCompletedWave" -v ./...`
Expected: FAIL

**Step 3: Add RenderNavigatorWithWaves to navigator.go**

Add a new function `RenderNavigatorWithWaves` that groups waves by cluster and renders the appropriate status symbol:

```go
// RenderNavigatorWithWaves renders the Link Navigator with actual wave data.
// Wave status symbols: [ ] available  [x] locked  [=] completed
func RenderNavigatorWithWaves(result *ScanResult, projectName string, waves []Wave) string {
	// Group waves by cluster name
	wavesByCluster := make(map[string][]Wave)
	for _, w := range waves {
		wavesByCluster[w.ClusterName] = append(wavesByCluster[w.ClusterName], w)
	}

	var b strings.Builder
	// ... header (same as RenderNavigator)
	// ... cluster rows use wavesByCluster[cluster.Name] to render wave cells
	// Each cell: status symbol + truncated title
	// "[ ] Deps" for available, "[x] DoD" for locked, "[=] Done" for completed
	// ... footer with updated legend
}
```

Keep `RenderNavigator` (v0.1) unchanged for backward compatibility. The new function is additive.

**Step 4: Run tests**

Run: `go test -run "TestRenderNavigator_WithWaves|TestRenderNavigator_WithCompletedWave" -v ./...`
Expected: PASS

**Step 5: Run all tests (v0.1 navigator tests must still pass)**

Run: `go test ./...`
Expected: All pass

**Step 6: Commit**

```bash
git add navigator.go navigator_test.go
git commit -m "feat(v0.2): add wave-aware Link Navigator rendering"
```

---

### Task 6: CLI Input/Output

Create `cli.go` with functions for interactive prompt display and stdin reading.

**Files:**
- Create: `cli.go`
- Create: `cli_test.go`

**Step 1: Write failing tests**

Create `cli_test.go`:

```go
package sightjack

import (
	"bytes"
	"strings"
	"testing"
)

func TestPromptWaveSelection(t *testing.T) {
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Delta: WaveDelta{Before: 0.25, After: 0.40}},
		{ID: "api-w1", ClusterName: "API", Title: "Split", Delta: WaveDelta{Before: 0.30, After: 0.45}},
	}

	input := strings.NewReader("1\n")
	var output bytes.Buffer

	selected, err := PromptWaveSelection(&output, input, waves)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.ID != "auth-w1" {
		t.Errorf("expected auth-w1, got %s", selected.ID)
	}
	if !strings.Contains(output.String(), "Auth") {
		t.Error("expected Auth in output")
	}
}

func TestPromptWaveSelection_Quit(t *testing.T) {
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps"},
	}

	input := strings.NewReader("q\n")
	var output bytes.Buffer

	_, err := PromptWaveSelection(&output, input, waves)
	if err != ErrQuit {
		t.Errorf("expected ErrQuit, got %v", err)
	}
}

func TestPromptWaveApproval_Approve(t *testing.T) {
	wave := Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Dependency Ordering",
		Actions: []WaveAction{
			{Type: "add_dependency", IssueID: "ENG-101", Description: "Auth before token"},
		},
		Delta: WaveDelta{Before: 0.25, After: 0.40},
	}

	input := strings.NewReader("a\n")
	var output bytes.Buffer

	approved, err := PromptWaveApproval(&output, input, wave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("expected approval")
	}
}

func TestPromptWaveApproval_Reject(t *testing.T) {
	wave := Wave{ID: "auth-w1", Actions: []WaveAction{}}

	input := strings.NewReader("r\n")
	var output bytes.Buffer

	approved, err := PromptWaveApproval(&output, input, wave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if approved {
		t.Error("expected rejection")
	}
}

func TestDisplayRippleEffects(t *testing.T) {
	ripples := []Ripple{
		{ClusterName: "API", Description: "W2 unlocked"},
		{ClusterName: "DB", Description: "New dependency added"},
	}

	var output bytes.Buffer
	DisplayRippleEffects(&output, ripples)

	out := output.String()
	if !strings.Contains(out, "API") {
		t.Error("expected API in ripple output")
	}
	if !strings.Contains(out, "W2 unlocked") {
		t.Error("expected ripple description in output")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -run "TestPromptWaveSelection|TestPromptWaveApproval|TestDisplayRippleEffects" -v ./...`
Expected: FAIL

**Step 3: Implement cli.go**

Create `cli.go`:

```go
package sightjack

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ErrQuit signals the user chose to quit.
var ErrQuit = errors.New("user quit")

// PromptWaveSelection displays available waves and reads the user's choice.
func PromptWaveSelection(w io.Writer, r io.Reader, waves []Wave) (Wave, error) {
	fmt.Fprintln(w, "\nAvailable waves:")
	for i, wave := range waves {
		fmt.Fprintf(w, "  %d. %-6s W: %-20s (%2.0f%% -> %2.0f%%)\n",
			i+1, wave.ClusterName, wave.Title,
			wave.Delta.Before*100, wave.Delta.After*100)
	}
	fmt.Fprintf(w, "\nSelect wave [1-%d, q=quit]: ", len(waves))

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return Wave{}, ErrQuit
	}
	input := strings.TrimSpace(scanner.Text())
	if input == "q" {
		return Wave{}, ErrQuit
	}
	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > len(waves) {
		return Wave{}, fmt.Errorf("invalid selection: %s", input)
	}
	return waves[num-1], nil
}

// PromptWaveApproval displays a wave proposal and reads approve/reject.
func PromptWaveApproval(w io.Writer, r io.Reader, wave Wave) (bool, error) {
	fmt.Fprintf(w, "\n--- %s - %s ---\n", wave.ClusterName, wave.Title)
	fmt.Fprintf(w, "  Proposed actions (%d):\n", len(wave.Actions))
	for i, a := range wave.Actions {
		fmt.Fprintf(w, "    %d. [%s] %s: %s\n", i+1, a.Type, a.IssueID, a.Description)
	}
	fmt.Fprintf(w, "\n  Expected: %.0f%% -> %.0f%%\n", wave.Delta.Before*100, wave.Delta.After*100)
	fmt.Fprint(w, "\n  [a] Approve all  [r] Reject  [q] Back to navigator: ")

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return false, ErrQuit
	}
	input := strings.TrimSpace(strings.ToLower(scanner.Text()))
	switch input {
	case "a":
		return true, nil
	case "r":
		return false, nil
	case "q":
		return false, ErrQuit
	default:
		return false, fmt.Errorf("invalid input: %s", input)
	}
}

// DisplayRippleEffects shows cross-cluster effects after a wave is applied.
func DisplayRippleEffects(w io.Writer, ripples []Ripple) {
	if len(ripples) == 0 {
		return
	}
	fmt.Fprintln(w, "\n  Ripple effects:")
	for _, r := range ripples {
		fmt.Fprintf(w, "    -> %s: %s\n", r.ClusterName, r.Description)
	}
}
```

**Step 4: Run tests**

Run: `go test -run "TestPromptWaveSelection|TestPromptWaveApproval|TestDisplayRippleEffects" -v ./...`
Expected: PASS

**Step 5: Run all tests**

Run: `go test ./...`
Expected: All pass

**Step 6: Commit**

```bash
git add cli.go cli_test.go
git commit -m "feat(v0.2): add interactive CLI input/output for wave selection"
```

---

### Task 7: Pass 3 — Wave Generation in Scanner

Add Wave generation (Pass 3) to scanner.go, running in parallel per cluster after Pass 2.

**Files:**
- Modify: `scanner.go`
- Modify: `scanner_test.go`

**Step 1: Write failing test for wave generation result parsing**

Add to `scanner_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test -run "TestRunWaveGenerate_ParsesResults" -v ./...`
Expected: FAIL (MergeWaveResults not defined)

**Step 3: Add MergeWaveResults to wave.go**

```go
// MergeWaveResults flattens multiple per-cluster wave results into a single wave list.
func MergeWaveResults(results []WaveGenerateResult) []Wave {
	var all []Wave
	for _, r := range results {
		all = append(all, r.Waves...)
	}
	return all
}
```

**Step 4: Run tests**

Run: `go test -run "TestRunWaveGenerate_ParsesResults" -v ./...`
Expected: PASS

**Step 5: Add RunWaveGenerate function to scanner.go**

This function mirrors the Pass 2 pattern — parallel Claude Code invocations per cluster, writing to wave_{name}.json files.

```go
// RunWaveGenerate executes Pass 3: generate waves for each cluster in parallel.
func RunWaveGenerate(ctx context.Context, cfg *Config, scanDir string, clusters []ClusterScanResult, dryRun bool) ([]Wave, error) {
	LogScan("Pass 3: Generating waves for %d clusters...", len(clusters))

	waveResults := make([]WaveGenerateResult, len(clusters))

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.Scan.MaxConcurrency)

	for i, cluster := range clusters {
		g.Go(func() error {
			waveFile := filepath.Join(scanDir, fmt.Sprintf("wave_%02d_%s.json", i, sanitizeName(cluster.Name)))

			issuesJSON, err := json.Marshal(cluster.Issues)
			if err != nil {
				return fmt.Errorf("marshal issues for %s: %w", cluster.Name, err)
			}

			prompt, err := RenderWaveGeneratePrompt(cfg.Lang, WaveGeneratePromptData{
				ClusterName:  cluster.Name,
				Completeness: cluster.Completeness,
				Issues:       string(issuesJSON),
				Observations: strings.Join(cluster.Observations, "\n"),
				OutputPath:   waveFile,
			})
			if err != nil {
				return fmt.Errorf("render wave prompt for %s: %w", cluster.Name, err)
			}

			if dryRun {
				return RunClaudeDryRun(cfg, prompt, scanDir)
			}

			LogScan("Generating waves: %s", cluster.Name)
			if _, err := RunClaude(gCtx, cfg, prompt, io.Discard); err != nil {
				return fmt.Errorf("wave generate %s: %w", cluster.Name, err)
			}

			result, err := ParseWaveGenerateResult(waveFile)
			if err != nil {
				return fmt.Errorf("parse waves %s: %w", cluster.Name, err)
			}
			waveResults[i] = *result
			LogOK("Cluster %s: %d waves generated", cluster.Name, len(result.Waves))
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return MergeWaveResults(waveResults), nil
}
```

**Step 6: Run all tests**

Run: `go test ./...`
Expected: All pass

**Step 7: Commit**

```bash
git add scanner.go scanner_test.go wave.go wave_test.go
git commit -m "feat(v0.2): add Pass 3 wave generation"
```

---

### Task 8: Pass 4 — Wave Apply

Add Wave apply function that executes a single approved wave via Claude Code.

**Files:**
- Modify: `wave.go`
- Modify: `wave_test.go`

**Step 1: Write failing test**

Add to `wave_test.go`:

```go
func TestMergeWaveResults(t *testing.T) {
	results := []WaveGenerateResult{
		{ClusterName: "Auth", Waves: []Wave{{ID: "auth-w1"}, {ID: "auth-w2"}}},
		{ClusterName: "API", Waves: []Wave{{ID: "api-w1"}}},
	}

	merged := MergeWaveResults(results)
	if len(merged) != 3 {
		t.Fatalf("expected 3 waves, got %d", len(merged))
	}
}

func TestMergeWaveResults_Empty(t *testing.T) {
	merged := MergeWaveResults(nil)
	if len(merged) != 0 {
		t.Errorf("expected 0 waves, got %d", len(merged))
	}
}
```

**Step 2: Run tests**

Run: `go test -run "TestMergeWaveResults" -v ./...`
Expected: PASS (MergeWaveResults was already added in Task 7)

**Step 3: Add RunWaveApply to wave.go**

```go
// RunWaveApply executes Pass 4: apply a single approved wave via Claude Code.
func RunWaveApply(ctx context.Context, cfg *Config, scanDir string, wave Wave) (*WaveApplyResult, error) {
	applyFile := filepath.Join(scanDir, fmt.Sprintf("apply_%s.json", sanitizeName(wave.ID)))

	actionsJSON, err := json.Marshal(wave.Actions)
	if err != nil {
		return nil, fmt.Errorf("marshal wave actions: %w", err)
	}

	prompt, err := RenderWaveApplyPrompt(cfg.Lang, WaveApplyPromptData{
		WaveID:      wave.ID,
		ClusterName: wave.ClusterName,
		Title:       wave.Title,
		Actions:     string(actionsJSON),
		OutputPath:  applyFile,
	})
	if err != nil {
		return nil, fmt.Errorf("render apply prompt: %w", err)
	}

	LogScan("Applying wave: %s - %s", wave.ClusterName, wave.Title)
	if _, err := RunClaude(ctx, cfg, prompt, os.Stdout); err != nil {
		return nil, fmt.Errorf("wave apply %s: %w", wave.ID, err)
	}

	result, err := ParseWaveApplyResult(applyFile)
	if err != nil {
		return nil, fmt.Errorf("parse apply result %s: %w", wave.ID, err)
	}

	LogOK("Wave %s applied: %d actions", wave.ID, result.Applied)
	return result, nil
}
```

**Note:** `RunWaveApply` uses `os.Stdout` (not `io.Discard`) because it's called one at a time in the interactive loop, so streaming output is fine.

**Step 4: Run all tests**

Run: `go test ./...`
Expected: All pass

**Step 5: Commit**

```bash
git add wave.go wave_test.go
git commit -m "feat(v0.2): add Pass 4 wave apply"
```

---

### Task 9: Session Loop Orchestration

Create `session.go` that ties everything together: Pass 1-3 auto-run, then interactive wave loop.

**Files:**
- Create: `session.go`
- Create: `session_test.go`

**Step 1: Write failing test for session setup**

Create `session_test.go`:

```go
package sightjack

import (
	"testing"
)

func TestBuildCompletedWaveMap(t *testing.T) {
	waves := []Wave{
		{ID: "auth-w1", Status: "completed"},
		{ID: "auth-w2", Status: "available"},
		{ID: "api-w1", Status: "completed"},
	}

	completed := BuildCompletedWaveMap(waves)
	if len(completed) != 2 {
		t.Fatalf("expected 2 completed, got %d", len(completed))
	}
	if !completed["auth-w1"] {
		t.Error("expected auth-w1 completed")
	}
	if completed["auth-w2"] {
		t.Error("auth-w2 should not be completed")
	}
}

func TestBuildWaveStates(t *testing.T) {
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed", Prerequisites: nil, Actions: make([]WaveAction, 3)},
		{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "available", Prerequisites: []string{"auth-w1"}, Actions: make([]WaveAction, 5)},
	}

	states := BuildWaveStates(waves)
	if len(states) != 2 {
		t.Fatalf("expected 2, got %d", len(states))
	}
	if states[0].ActionCount != 3 {
		t.Errorf("expected 3 actions, got %d", states[0].ActionCount)
	}
	if states[1].Prerequisites[0] != "auth-w1" {
		t.Errorf("expected prerequisite auth-w1")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test -run "TestBuildCompletedWaveMap|TestBuildWaveStates" -v ./...`
Expected: FAIL

**Step 3: Implement session.go**

Create `session.go`:

```go
package sightjack

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

// RunSession runs the full session: Pass 1-3 (auto), then interactive wave loop.
func RunSession(ctx context.Context, cfg *Config, baseDir string, sessionID string, input io.Reader) error {
	scanDir, err := EnsureScanDir(baseDir, sessionID)
	if err != nil {
		return err
	}

	// --- Pass 1+2: Scan (reuse v0.1 RunScan) ---
	scanResult, err := RunScan(ctx, cfg, baseDir, sessionID, false)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	// --- Pass 3: Wave Generate ---
	waves, err := RunWaveGenerate(ctx, cfg, scanDir, scanResult.Clusters, false)
	if err != nil {
		return fmt.Errorf("wave generate: %w", err)
	}

	LogOK("%d clusters, %d waves generated", len(scanResult.Clusters), len(waves))

	completed := BuildCompletedWaveMap(waves)

	// --- Interactive Loop ---
	for {
		waves = EvaluateUnlocks(waves, completed)
		available := AvailableWaves(waves, completed)
		if len(available) == 0 {
			LogOK("All waves completed or no available waves.")
			break
		}

		// Display Link Navigator
		nav := RenderNavigatorWithWaves(scanResult, cfg.Linear.Project, waves)
		fmt.Println()
		fmt.Print(nav)

		// Prompt wave selection
		selected, err := PromptWaveSelection(os.Stdout, input, available)
		if err == ErrQuit {
			LogInfo("Session paused. State saved.")
			break
		}
		if err != nil {
			LogWarn("Invalid selection: %v", err)
			continue
		}

		// Prompt wave approval
		approved, err := PromptWaveApproval(os.Stdout, input, selected)
		if err == ErrQuit {
			continue
		}
		if err != nil {
			LogWarn("Invalid input: %v", err)
			continue
		}
		if !approved {
			LogInfo("Wave rejected.")
			continue
		}

		// --- Pass 4: Wave Apply ---
		applyResult, err := RunWaveApply(ctx, cfg, scanDir, selected)
		if err != nil {
			LogError("Apply failed: %v", err)
			continue
		}

		// Display ripple effects
		DisplayRippleEffects(os.Stdout, applyResult.Ripples)

		// Mark wave completed
		completed[selected.ID] = true
		for i, w := range waves {
			if w.ID == selected.ID {
				waves[i].Status = "completed"
				break
			}
		}

		// Update completeness from delta
		scanResult.Completeness = selected.Delta.After
		for i, c := range scanResult.Clusters {
			if c.Name == selected.ClusterName {
				scanResult.Clusters[i].Completeness = selected.Delta.After
				break
			}
		}

		LogOK("Completeness: %.0f%%", scanResult.Completeness*100)
	}

	// Save state
	state := &SessionState{
		Version:      "0.2",
		SessionID:    sessionID,
		Project:      cfg.Linear.Project,
		LastScanned:  time.Now(),
		Completeness: scanResult.Completeness,
		Waves:        BuildWaveStates(waves),
	}
	for _, c := range scanResult.Clusters {
		state.Clusters = append(state.Clusters, ClusterState{
			Name:         c.Name,
			Completeness: c.Completeness,
			IssueCount:   len(c.Issues),
		})
	}

	if err := WriteState(baseDir, state); err != nil {
		LogWarn("Failed to save state: %v", err)
	} else {
		LogOK("State saved to %s", StatePath(baseDir))
	}

	return nil
}

// BuildCompletedWaveMap returns a set of completed wave IDs.
func BuildCompletedWaveMap(waves []Wave) map[string]bool {
	completed := make(map[string]bool)
	for _, w := range waves {
		if w.Status == "completed" {
			completed[w.ID] = true
		}
	}
	return completed
}

// BuildWaveStates converts Wave list to WaveState list for persistence.
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
		}
	}
	return states
}
```

**Step 4: Run tests**

Run: `go test -run "TestBuildCompletedWaveMap|TestBuildWaveStates" -v ./...`
Expected: PASS

**Step 5: Run all tests**

Run: `go test ./...`
Expected: All pass

**Step 6: Commit**

```bash
git add session.go session_test.go
git commit -m "feat(v0.2): add session loop orchestration"
```

---

### Task 10: Session Subcommand in main.go

Add `session` to known subcommands and wire it up.

**Files:**
- Modify: `cmd/sightjack/main.go`

**Step 1: Add "session" to extractSubcommand knownCmds**

In `extractSubcommand`, change:

```go
knownCmds := map[string]bool{"scan": true, "show": true}
```

to:

```go
knownCmds := map[string]bool{"scan": true, "show": true, "session": true}
```

**Step 2: Add session case to main switch**

Add after the `show` case:

```go
case "session":
	cfg, err := sightjack.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	if lang != "" {
		cfg.Lang = lang
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	sessionID := fmt.Sprintf("session-%d-%d", time.Now().UnixMilli(), os.Getpid())

	if err := sightjack.RunSession(ctx, cfg, baseDir(), sessionID, os.Stdin); err != nil {
		sightjack.LogError("Session failed: %v", err)
		os.Exit(1)
	}
```

**Note:** `baseDir()` should use `os.Getwd()` — extract to a helper or inline. Follow the same pattern as the existing `scan` case.

**Step 3: Update default subcommand (if no subcommand given)**

Change the default from `"scan"` to keep `"scan"` as default (no change needed here, `session` must be explicitly invoked).

**Step 4: Update usage string**

Change:
```go
"Usage: sightjack [scan|show] [flags]"
```
to:
```go
"Usage: sightjack [scan|show|session] [flags]"
```

**Step 5: Build and verify**

Run: `go build ./cmd/sightjack/`
Expected: Build succeeds

**Step 6: Run all tests**

Run: `go test ./...`
Expected: All pass

**Step 7: Commit**

```bash
git add cmd/sightjack/main.go
git commit -m "feat(v0.2): add session subcommand"
```

---

### Task 11: Integration Test — Dry Run Session

Add an integration-level test that verifies the full session can initialize in dry-run mode.

**Files:**
- Modify: `scanner.go` (add dry-run support to RunWaveGenerate)
- Modify: `session.go` (add dry-run parameter to RunSession)

**Step 1: Add dryRun parameter to RunSession**

Modify `RunSession` signature:

```go
func RunSession(ctx context.Context, cfg *Config, baseDir string, sessionID string, dryRun bool, input io.Reader) error
```

When `dryRun`:
- Pass `true` to `RunScan` (already supported)
- Pass `true` to `RunWaveGenerate`
- Skip the interactive loop
- Log dry-run completion

**Step 2: Wire `--dry-run` flag in main.go session case**

```go
case "session":
	// ... same as before ...
	if dryRun {
		if err := sightjack.RunSession(ctx, cfg, baseDir(), sessionID, true, nil); err != nil {
			sightjack.LogError("Session dry-run failed: %v", err)
			os.Exit(1)
		}
		sightjack.LogOK("Dry-run complete. Check .siren/scans/ for generated prompts.")
	} else {
		if err := sightjack.RunSession(ctx, cfg, baseDir(), sessionID, false, os.Stdin); err != nil {
			sightjack.LogError("Session failed: %v", err)
			os.Exit(1)
		}
	}
```

**Step 3: Run all tests**

Run: `go test ./...`
Expected: All pass

**Step 4: Manual verification**

Run: `go build ./cmd/sightjack/ && ./sightjack session --dry-run`
Expected: Dry-run outputs prompt files to `.siren/scans/`

**Step 5: Commit**

```bash
git add session.go scanner.go cmd/sightjack/main.go
git commit -m "feat(v0.2): add dry-run support for session command"
```

---

## Summary

| Task | Component | Files | Key Functions |
|------|-----------|-------|---------------|
| 1 | Wave Data Model | model.go | Wave, WaveAction, WaveDelta, WaveApplyResult, Ripple |
| 2 | Wave Parsing + Unlock | wave.go | ParseWaveGenerateResult, AvailableWaves, EvaluateUnlocks |
| 3 | State Extension | model.go, state.go | WaveState, SessionState.Waves |
| 4 | Prompt Templates | prompt.go, templates/ | RenderWaveGeneratePrompt, RenderWaveApplyPrompt |
| 5 | Navigator + Waves | navigator.go | RenderNavigatorWithWaves |
| 6 | CLI Input/Output | cli.go | PromptWaveSelection, PromptWaveApproval, DisplayRippleEffects |
| 7 | Pass 3 Wave Gen | scanner.go, wave.go | RunWaveGenerate, MergeWaveResults |
| 8 | Pass 4 Wave Apply | wave.go | RunWaveApply |
| 9 | Session Loop | session.go | RunSession, BuildCompletedWaveMap, BuildWaveStates |
| 10 | Session Subcommand | main.go | "session" case in main switch |
| 11 | Integration Test | session.go, main.go | dry-run support |
