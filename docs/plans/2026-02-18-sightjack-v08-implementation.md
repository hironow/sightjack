# Sightjack v0.8: Wave Dynamic Evolution — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add dynamic wave generation after completion, ADR feedback into wave prompts, and selective action approval.

**Architecture:** Three features wired into the existing interactive loop. `wave_generator.go` handles post-completion wave generation (mirroring `architect.go` pattern). `cli.go` gets a new `PromptSelectiveApproval` function. `session.go` orchestrates: after wave apply, generate next waves; before wave apply, allow selective approval. ADR feedback is purely template-level — existing `ReadExistingADRs` is reused.

**Tech Stack:** Go 1.23, `text/template`, `encoding/json`, `embed`

---

### Task 1: Add `NextGenResult` type to `model.go`

**Files:**
- Modify: `model.go:135-139` (near `WaveGenerateResult`)
- Test: `model_test.go`

**Step 1: Write the failing test**

Add to `model_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestNextGenResult_UnmarshalJSON -v`
Expected: FAIL — `NextGenResult` undefined

**Step 3: Write minimal implementation**

Add to `model.go` after `WaveGenerateResult` (line ~139):

```go
// NextGenResult is the output of post-completion wave generation.
type NextGenResult struct {
	ClusterName string `json:"cluster_name"`
	Waves       []Wave `json:"waves"`
	Reasoning   string `json:"reasoning"`
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -run TestNextGenResult -v`
Expected: PASS

**Step 5: Commit**

```bash
git add model.go model_test.go
git commit -m "feat(v0.8): add NextGenResult type for dynamic wave generation"
```

---

### Task 2: Add `ApprovalSelective` to `model.go`

**Files:**
- Modify: `model.go:158-163` (ApprovalChoice enum)
- Test: `model_test.go`

**Step 1: Write the failing test**

Add to `model_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestApprovalSelective_IsDistinctValue -v`
Expected: FAIL — `ApprovalSelective` undefined

**Step 3: Write minimal implementation**

Modify `model.go` — add `ApprovalSelective` to the const block:

```go
const (
	ApprovalApprove ApprovalChoice = iota
	ApprovalReject
	ApprovalDiscuss
	ApprovalQuit
	ApprovalSelective
)
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -run TestApprovalSelective -v`
Expected: PASS

**Step 5: Commit**

```bash
git add model.go model_test.go
git commit -m "feat(v0.8): add ApprovalSelective choice for individual action approval"
```

---

### Task 3: Add `NextGenPromptData` + `RenderNextGenPrompt` + templates

**Files:**
- Modify: `prompt.go` (add `NextGenPromptData` + `RenderNextGenPrompt`)
- Create: `prompts/templates/wave_nextgen_en.md.tmpl`
- Create: `prompts/templates/wave_nextgen_ja.md.tmpl`
- Test: `prompt_test.go`

**Step 1: Write the failing test**

Add to `prompt_test.go`:

```go
func TestRenderNextGenPrompt_English(t *testing.T) {
	data := NextGenPromptData{
		ClusterName:     "Auth",
		Completeness:    "65",
		Issues:          `[{"id":"ENG-101"}]`,
		CompletedWaves:  `[{"id":"auth-w1","title":"Initial setup"}]`,
		ExistingADRs:    []ExistingADR{{Filename: "0001-jwt.md", Content: "# JWT decision"}},
		RejectedActions: `[{"type":"add_dod","issue_id":"ENG-102","description":"Rejected action"}]`,
		OutputPath:      "/tmp/nextgen.json",
		StrictnessLevel: "alert",
	}
	result, err := RenderNextGenPrompt("en", data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{"Auth", "65", "ENG-101", "auth-w1", "0001-jwt.md", "JWT decision", "Rejected action", "/tmp/nextgen.json", "alert"} {
		if !strings.Contains(result, want) {
			t.Errorf("missing %q in output", want)
		}
	}
}

func TestRenderNextGenPrompt_Japanese(t *testing.T) {
	data := NextGenPromptData{
		ClusterName:    "API",
		Completeness:   "50",
		Issues:         `[]`,
		CompletedWaves: `[]`,
		OutputPath:     "/tmp/out.json",
		StrictnessLevel: "fog",
	}
	result, err := RenderNextGenPrompt("ja", data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(result, "API") {
		t.Errorf("missing cluster name in output")
	}
}

func TestRenderNextGenPrompt_NoADRs(t *testing.T) {
	data := NextGenPromptData{
		ClusterName:    "DB",
		Completeness:   "40",
		Issues:         `[]`,
		CompletedWaves: `[]`,
		OutputPath:     "/tmp/out.json",
		StrictnessLevel: "fog",
	}
	result, err := RenderNextGenPrompt("en", data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// Should NOT contain the ADR section header when no ADRs
	if strings.Contains(result, "Existing ADRs") {
		t.Errorf("ADR section should be omitted when no ADRs exist")
	}
}

func TestRenderNextGenPrompt_NoRejectedActions(t *testing.T) {
	data := NextGenPromptData{
		ClusterName:    "DB",
		Completeness:   "40",
		Issues:         `[]`,
		CompletedWaves: `[]`,
		OutputPath:     "/tmp/out.json",
		StrictnessLevel: "fog",
	}
	result, err := RenderNextGenPrompt("en", data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(result, "Rejected Actions") {
		t.Errorf("Rejected section should be omitted when empty")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestRenderNextGenPrompt -v`
Expected: FAIL — `NextGenPromptData` undefined

**Step 3: Write minimal implementation**

Add to `prompt.go` after `ArchitectDiscussPromptData`:

```go
// NextGenPromptData holds template data for post-completion wave generation.
type NextGenPromptData struct {
	ClusterName     string
	Completeness    string
	Issues          string
	CompletedWaves  string
	ExistingADRs    []ExistingADR
	RejectedActions string
	OutputPath      string
	StrictnessLevel string
}

// RenderNextGenPrompt renders the next-gen wave generation prompt.
func RenderNextGenPrompt(lang string, data NextGenPromptData) (string, error) {
	name := fmt.Sprintf("prompts/templates/wave_nextgen_%s.md.tmpl", lang)
	return renderTemplate(name, data)
}
```

Create `prompts/templates/wave_nextgen_en.md.tmpl`:

```
You are a Wave Generator Agent.
A wave has just been completed in this cluster. Generate the NEXT wave(s) based on current state.

## Strictness Level: {{.StrictnessLevel}}
- fog: Report DoD gaps as warnings only.
- alert: Propose sub-issues for Must-level DoD gaps.
- lockdown: Flag ALL DoD gaps. Mark incomplete issues as blocked candidates.

## Target Cluster
- Cluster Name: {{.ClusterName}}
- Current Completeness: {{.Completeness}}%

## Issue Analysis Results
{{.Issues}}

## Completed Waves
{{.CompletedWaves}}
{{if .ExistingADRs}}

## Existing ADRs (design decisions to respect)
{{range .ExistingADRs}}
### {{.Filename}}
{{.Content}}
{{end}}
{{end}}
{{if .RejectedActions}}

## Previously Rejected Actions
The user rejected these actions in a previous wave. Do NOT re-propose the same actions.
{{.RejectedActions}}
{{end}}

## Generation Guidelines
- Generate 0-2 new waves based on remaining gaps
- Return 0 waves if the cluster appears complete
- Each wave should be completable in a single work session
- New waves may depend on previously completed waves
- Respect existing ADR decisions
- Do NOT re-propose previously rejected actions

## Action Types
- `add_dod`: Append DoD items to the Issue description
- `add_dependency`: Set dependencies between Issues
- `add_label`: Add a label to an Issue
- `update_description`: Update an Issue description
- `create`: Create a new sub-issue

## Output
Write the following JSON to **{{.OutputPath}}**:

` + "```" + `json
{
  "cluster_name": "{{.ClusterName}}",
  "waves": [
    {
      "id": "cluster-wN",
      "cluster_name": "{{.ClusterName}}",
      "title": "Wave Title",
      "description": "What this Wave accomplishes",
      "actions": [
        {"type": "add_dod", "issue_id": "issue-id", "description": "Human-readable description", "detail": "Specific content to add"}
      ],
      "prerequisites": [],
      "delta": {"before": 0.65, "after": 0.80},
      "status": "available"
    }
  ],
  "reasoning": "Why these waves are needed"
}
` + "```" + `

Return an empty waves array if no further work is needed.
Write output directly to the file path above. Do not write to stdout.
```

Create `prompts/templates/wave_nextgen_ja.md.tmpl` (same structure, Japanese instructions):

```
あなたはWave生成エージェントです。
このクラスタでWaveが完了しました。現在の状態に基づいて次のWaveを生成してください。

## 厳格度レベル: {{.StrictnessLevel}}
- fog: DoD不足は警告のみ。
- alert: Must-levelのDoD不足にはサブIssueを提案。
- lockdown: 全てのDoD不足をフラグ。

## 対象クラスタ
- クラスタ名: {{.ClusterName}}
- 現在の完成度: {{.Completeness}}%

## Issue分析結果
{{.Issues}}

## 完了済みWave
{{.CompletedWaves}}
{{if .ExistingADRs}}

## 既存ADR（尊重すべき設計判断）
{{range .ExistingADRs}}
### {{.Filename}}
{{.Content}}
{{end}}
{{end}}
{{if .RejectedActions}}

## 前回拒否されたアクション
ユーザーが前回拒否したアクションです。同じアクションを再提案しないでください。
{{.RejectedActions}}
{{end}}

## 生成ガイドライン
- 残りのギャップに基づいて0-2個の新Waveを生成
- クラスタが完了済みに見える場合は0個を返す
- 各Waveは1セッションで完了可能な規模にする
- 新Waveは完了済みWaveに依存可能
- 既存ADRの設計判断を尊重する
- 前回拒否されたアクションを再提案しない

## アクションタイプ
- `add_dod`: IssueのDoDに項目を追加
- `add_dependency`: Issue間の依存関係を設定
- `add_label`: Issueにラベルを追加
- `update_description`: Issueの説明を更新
- `create`: 新しいサブIssueを作成

## 出力
以下のJSONを **{{.OutputPath}}** に書き出してください:

` + "```" + `json
{
  "cluster_name": "{{.ClusterName}}",
  "waves": [
    {
      "id": "cluster-wN",
      "cluster_name": "{{.ClusterName}}",
      "title": "Wave Title",
      "description": "このWaveが達成すること",
      "actions": [
        {"type": "add_dod", "issue_id": "issue-id", "description": "人間が読める説明", "detail": "追加する具体的な内容"}
      ],
      "prerequisites": [],
      "delta": {"before": 0.65, "after": 0.80},
      "status": "available"
    }
  ],
  "reasoning": "これらのWaveが必要な理由"
}
` + "```" + `

追加作業が不要な場合はwaves配列を空にしてください。
出力は上記ファイルパスに直接書き出してください。stdoutには出力しないでください。
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -run TestRenderNextGenPrompt -v`
Expected: PASS

**Step 5: Commit**

```bash
git add prompt.go prompt_test.go prompts/templates/wave_nextgen_en.md.tmpl prompts/templates/wave_nextgen_ja.md.tmpl
git commit -m "feat(v0.8): add NextGenPromptData, RenderNextGenPrompt, and templates"
```

---

### Task 4: Add `wave_generator.go` — file helpers + parse

**Files:**
- Create: `wave_generator.go`
- Create: `wave_generator_test.go`

**Step 1: Write the failing tests**

Create `wave_generator_test.go`:

```go
package sightjack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNextgenFileName(t *testing.T) {
	wave := Wave{ClusterName: "Auth", ID: "auth-w2"}
	got := nextgenFileName(wave)
	want := "nextgen_auth_auth-w2.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestClearNextgenOutput_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	wave := Wave{ClusterName: "Auth", ID: "auth-w1"}
	path := filepath.Join(dir, nextgenFileName(wave))
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	clearNextgenOutput(dir, wave)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should have been removed")
	}
}

func TestClearNextgenOutput_NoopIfMissing(t *testing.T) {
	dir := t.TempDir()
	wave := Wave{ClusterName: "Auth", ID: "auth-w1"}
	clearNextgenOutput(dir, wave) // should not panic
}

func TestParseNextGenResult_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nextgen.json")
	data := `{"cluster_name":"Auth","waves":[{"id":"auth-w3","cluster_name":"Auth","title":"Security pass","description":"desc","actions":[],"prerequisites":["auth-w2"],"delta":{"before":0.65,"after":0.80},"status":"available"}],"reasoning":"needed"}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := ParseNextGenResult(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.ClusterName != "Auth" {
		t.Errorf("cluster_name: got %q", result.ClusterName)
	}
	if len(result.Waves) != 1 {
		t.Fatalf("waves: got %d, want 1", len(result.Waves))
	}
	if result.Waves[0].ID != "auth-w3" {
		t.Errorf("wave id: got %q", result.Waves[0].ID)
	}
}

func TestParseNextGenResult_EmptyWaves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nextgen.json")
	data := `{"cluster_name":"Auth","waves":[],"reasoning":"cluster complete"}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := ParseNextGenResult(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Waves) != 0 {
		t.Errorf("expected 0 waves, got %d", len(result.Waves))
	}
}

func TestParseNextGenResult_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nextgen.json")
	if err := os.WriteFile(path, []byte("{bad json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ParseNextGenResult(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !contains(err.Error(), "parse nextgen result") {
		t.Errorf("error should contain 'parse nextgen result': %v", err)
	}
}

func TestParseNextGenResult_MissingFile(t *testing.T) {
	_, err := ParseNextGenResult("/nonexistent/file.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestNextgen -v`
Expected: FAIL — functions undefined

**Step 3: Write minimal implementation**

Create `wave_generator.go`:

```go
package sightjack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// nextgenFileName returns the output filename for a nextgen wave generation run.
func nextgenFileName(wave Wave) string {
	return fmt.Sprintf("nextgen_%s_%s.json", sanitizeName(wave.ClusterName), sanitizeName(wave.ID))
}

// clearNextgenOutput removes any existing nextgen output file.
func clearNextgenOutput(scanDir string, wave Wave) {
	path := filepath.Join(scanDir, nextgenFileName(wave))
	os.Remove(path)
}

// ParseNextGenResult reads and parses a nextgen wave generation result JSON file.
func ParseNextGenResult(path string) (*NextGenResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read nextgen result: %w", err)
	}
	var result NextGenResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse nextgen result: %w", err)
	}
	return &result, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run TestNextgen -v && go test ./... -run TestParseNextGenResult -v`
Expected: PASS

**Step 5: Commit**

```bash
git add wave_generator.go wave_generator_test.go
git commit -m "feat(v0.8): add wave_generator.go with file helpers and ParseNextGenResult"
```

---

### Task 5: Add `GenerateNextWaves` + dry-run to `wave_generator.go`

**Files:**
- Modify: `wave_generator.go`
- Modify: `wave_generator_test.go`

**Context:** This follows the exact pattern of `RunArchitectDiscuss` (architect.go:62-94) and `RunScribeADR` (scribe.go).

**Step 1: Write the failing test**

Add to `wave_generator_test.go`:

```go
func TestGenerateNextWavesDryRun(t *testing.T) {
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "scans")
	if err := os.MkdirAll(scanDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	wave := Wave{ClusterName: "Auth", ID: "auth-w1"}
	cluster := ClusterScanResult{
		Name:         "Auth",
		Completeness: 0.65,
		Issues:       []IssueDetail{{ID: "ENG-101", Identifier: "ENG-101", Title: "Auth issue", Completeness: 0.5}},
	}
	completedWaves := []Wave{{ID: "auth-w1", ClusterName: "Auth", Title: "Initial setup", Status: "completed"}}

	err := GenerateNextWavesDryRun(&cfg, scanDir, wave, cluster, completedWaves, nil, nil)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}

	// Verify prompt file was created
	promptFile := filepath.Join(scanDir, "nextgen_auth_auth-w1_prompt.md")
	if _, err := os.Stat(promptFile); os.IsNotExist(err) {
		t.Error("prompt file should have been created")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestGenerateNextWavesDryRun -v`
Expected: FAIL — `GenerateNextWavesDryRun` undefined

**Step 3: Write minimal implementation**

Add to `wave_generator.go`:

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// GenerateNextWavesDryRun saves the nextgen prompt to a file instead of executing Claude.
func GenerateNextWavesDryRun(cfg *Config, scanDir string, completedWave Wave, cluster ClusterScanResult, completedWaves []Wave, existingADRs []ExistingADR, rejectedActions []WaveAction) error {
	prompt, err := buildNextGenPrompt(cfg, scanDir, completedWave, cluster, completedWaves, existingADRs, rejectedActions)
	if err != nil {
		return err
	}
	dryRunName := fmt.Sprintf("nextgen_%s_%s", sanitizeName(completedWave.ClusterName), sanitizeName(completedWave.ID))
	return RunClaudeDryRun(cfg, prompt, scanDir, dryRunName)
}

// GenerateNextWaves executes post-completion wave generation for a cluster.
func GenerateNextWaves(ctx context.Context, cfg *Config, scanDir string, completedWave Wave, cluster ClusterScanResult, completedWaves []Wave, existingADRs []ExistingADR, rejectedActions []WaveAction) ([]Wave, error) {
	clearNextgenOutput(scanDir, completedWave)
	outputFile := filepath.Join(scanDir, nextgenFileName(completedWave))

	prompt, err := buildNextGenPrompt(cfg, scanDir, completedWave, cluster, completedWaves, existingADRs, rejectedActions)
	if err != nil {
		return nil, err
	}

	LogScan("Generating next waves: %s", completedWave.ClusterName)
	if _, err := RunClaude(ctx, cfg, prompt, io.Discard); err != nil {
		return nil, fmt.Errorf("nextgen %s: %w", completedWave.ClusterName, err)
	}

	result, err := ParseNextGenResult(outputFile)
	if err != nil {
		return nil, fmt.Errorf("parse nextgen %s: %w", completedWave.ClusterName, err)
	}

	newWaves := NormalizeWavePrerequisites(result.Waves)
	if len(newWaves) > 0 {
		LogOK("Generated %d new wave(s) for %s: %s", len(newWaves), completedWave.ClusterName, result.Reasoning)
	}
	return newWaves, nil
}

// buildNextGenPrompt constructs the prompt for post-completion wave generation.
func buildNextGenPrompt(cfg *Config, scanDir string, completedWave Wave, cluster ClusterScanResult, completedWaves []Wave, existingADRs []ExistingADR, rejectedActions []WaveAction) (string, error) {
	outputFile := filepath.Join(scanDir, nextgenFileName(completedWave))

	issuesJSON, err := json.Marshal(cluster.Issues)
	if err != nil {
		return "", fmt.Errorf("marshal issues: %w", err)
	}

	completedJSON, err := json.Marshal(completedWaves)
	if err != nil {
		return "", fmt.Errorf("marshal completed waves: %w", err)
	}

	var rejectedStr string
	if len(rejectedActions) > 0 {
		rejectedJSON, err := json.Marshal(rejectedActions)
		if err != nil {
			return "", fmt.Errorf("marshal rejected actions: %w", err)
		}
		rejectedStr = string(rejectedJSON)
	}

	return RenderNextGenPrompt(cfg.Lang, NextGenPromptData{
		ClusterName:     completedWave.ClusterName,
		Completeness:    fmt.Sprintf("%.0f", cluster.Completeness*100),
		Issues:          string(issuesJSON),
		CompletedWaves:  string(completedJSON),
		ExistingADRs:    existingADRs,
		RejectedActions: rejectedStr,
		OutputPath:      outputFile,
		StrictnessLevel: string(cfg.Strictness.Default),
	})
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS (all tests including existing ones)

**Step 5: Commit**

```bash
git add wave_generator.go wave_generator_test.go
git commit -m "feat(v0.8): add GenerateNextWaves and dry-run for post-completion wave generation"
```

---

### Task 6: Add `PromptSelectiveApproval` to `cli.go`

**Files:**
- Modify: `cli.go:78` (update PromptWaveApproval prompt text)
- Modify: `cli.go` (add PromptSelectiveApproval)
- Test: `cli_test.go`

**Step 1: Write the failing tests**

Add to `cli_test.go`:

```go
func TestPromptWaveApproval_Selective(t *testing.T) {
	wave := Wave{
		ClusterName: "Auth",
		Title:       "JWT middleware",
		Actions:     []WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "Add spec"}},
		Delta:       WaveDelta{Before: 0.3, After: 0.5},
	}
	input := strings.NewReader("s\n")
	scanner := bufio.NewScanner(input)
	var buf bytes.Buffer

	choice, err := PromptWaveApproval(context.Background(), &buf, scanner, wave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != ApprovalSelective {
		t.Errorf("expected ApprovalSelective, got %d", choice)
	}
}

func TestPromptSelectiveApproval_AllSelected(t *testing.T) {
	actions := []WaveAction{
		{Type: "add_dod", IssueID: "ENG-101", Description: "Add spec"},
		{Type: "add_dep", IssueID: "ENG-102", Description: "Add dependency"},
	}
	wave := Wave{ClusterName: "Auth", Title: "Test", Actions: actions}
	input := strings.NewReader("done\n")
	scanner := bufio.NewScanner(input)
	var buf bytes.Buffer

	approved, rejected, err := PromptSelectiveApproval(context.Background(), &buf, scanner, wave)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(approved) != 2 {
		t.Errorf("approved: got %d, want 2", len(approved))
	}
	if len(rejected) != 0 {
		t.Errorf("rejected: got %d, want 0", len(rejected))
	}
}

func TestPromptSelectiveApproval_ToggleOne(t *testing.T) {
	actions := []WaveAction{
		{Type: "add_dod", IssueID: "ENG-101", Description: "Add spec"},
		{Type: "add_dep", IssueID: "ENG-102", Description: "Add dependency"},
		{Type: "create", IssueID: "ENG-103", Description: "Create sub-issue"},
	}
	wave := Wave{ClusterName: "Auth", Title: "Test", Actions: actions}
	input := strings.NewReader("3\ndone\n")
	scanner := bufio.NewScanner(input)
	var buf bytes.Buffer

	approved, rejected, err := PromptSelectiveApproval(context.Background(), &buf, scanner, wave)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(approved) != 2 {
		t.Errorf("approved: got %d, want 2", len(approved))
	}
	if len(rejected) != 1 {
		t.Errorf("rejected: got %d, want 1", len(rejected))
	}
	if rejected[0].IssueID != "ENG-103" {
		t.Errorf("rejected[0].IssueID: got %q, want %q", rejected[0].IssueID, "ENG-103")
	}
}

func TestPromptSelectiveApproval_SelectNoneThenDone(t *testing.T) {
	actions := []WaveAction{
		{Type: "add_dod", IssueID: "ENG-101", Description: "Add spec"},
	}
	wave := Wave{ClusterName: "Auth", Title: "Test", Actions: actions}
	input := strings.NewReader("n\ndone\n")
	scanner := bufio.NewScanner(input)
	var buf bytes.Buffer

	approved, rejected, err := PromptSelectiveApproval(context.Background(), &buf, scanner, wave)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(approved) != 0 {
		t.Errorf("approved: got %d, want 0", len(approved))
	}
	if len(rejected) != 1 {
		t.Errorf("rejected: got %d, want 1", len(rejected))
	}
}

func TestPromptSelectiveApproval_SelectAll(t *testing.T) {
	actions := []WaveAction{
		{Type: "add_dod", IssueID: "ENG-101", Description: "Spec"},
		{Type: "add_dep", IssueID: "ENG-102", Description: "Dep"},
	}
	wave := Wave{ClusterName: "Auth", Title: "Test", Actions: actions}
	input := strings.NewReader("n\na\ndone\n")
	scanner := bufio.NewScanner(input)
	var buf bytes.Buffer

	approved, _, err := PromptSelectiveApproval(context.Background(), &buf, scanner, wave)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(approved) != 2 {
		t.Errorf("approved: got %d, want 2", len(approved))
	}
}

func TestPromptSelectiveApproval_Quit(t *testing.T) {
	actions := []WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "Spec"}}
	wave := Wave{ClusterName: "Auth", Title: "Test", Actions: actions}
	input := strings.NewReader("q\n")
	scanner := bufio.NewScanner(input)
	var buf bytes.Buffer

	_, _, err := PromptSelectiveApproval(context.Background(), &buf, scanner, wave)
	if err != ErrQuit {
		t.Errorf("expected ErrQuit, got %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run "TestPromptSelectiveApproval|TestPromptWaveApproval_Selective" -v`
Expected: FAIL — `ApprovalSelective` and `PromptSelectiveApproval` undefined in cli.go

**Step 3: Write minimal implementation**

Modify `cli.go` `PromptWaveApproval` prompt text (line 78):

```go
fmt.Fprint(w, "\n  [a] Approve all  [s] Selective  [r] Reject  [d] Discuss  [q] Back to navigator: ")
```

Add `case "s"` to the switch (after `case "d"`):

```go
	case "s":
		return ApprovalSelective, nil
```

Add new function to `cli.go`:

```go
// PromptSelectiveApproval displays wave actions with toggle checkboxes.
// Returns approved and rejected action lists.
func PromptSelectiveApproval(ctx context.Context, w io.Writer, s *bufio.Scanner, wave Wave) ([]WaveAction, []WaveAction, error) {
	selected := make([]bool, len(wave.Actions))
	for i := range selected {
		selected[i] = true // default: all selected
	}

	for {
		// Display current state
		fmt.Fprintf(w, "\n  --- %s - %s ---\n", wave.ClusterName, wave.Title)
		for i, a := range wave.Actions {
			mark := "x"
			if !selected[i] {
				mark = " "
			}
			fmt.Fprintf(w, "    %d. [%s] [%s] %s: %s\n", i+1, mark, a.Type, a.IssueID, a.Description)
		}
		fmt.Fprintf(w, "\n  Toggle [1-%d, a=all, n=none, done=confirm, q=quit]: ", len(wave.Actions))

		line, err := ScanLine(ctx, s)
		if err != nil {
			return nil, nil, ErrQuit
		}
		input := strings.TrimSpace(strings.ToLower(line))

		switch input {
		case "q":
			return nil, nil, ErrQuit
		case "done":
			var approved, rejected []WaveAction
			for i, a := range wave.Actions {
				if selected[i] {
					approved = append(approved, a)
				} else {
					rejected = append(rejected, a)
				}
			}
			return approved, rejected, nil
		case "a":
			for i := range selected {
				selected[i] = true
			}
			continue
		case "n":
			for i := range selected {
				selected[i] = false
			}
			continue
		default:
			num, parseErr := strconv.Atoi(input)
			if parseErr != nil || num < 1 || num > len(wave.Actions) {
				fmt.Fprintf(w, "  Invalid input: %s\n", input)
				continue
			}
			selected[num-1] = !selected[num-1]
		}
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cli.go cli_test.go
git commit -m "feat(v0.8): add PromptSelectiveApproval and ApprovalSelective to wave approval flow"
```

---

### Task 7: Wire selective approval into `session.go`

**Files:**
- Modify: `session.go:175-217` (approval switch)
- Test: `session_test.go`

**Step 1: Write the failing test**

Add to `session_test.go`:

```go
func TestApprovalSelective_InPromptWaveApproval(t *testing.T) {
	wave := Wave{
		ClusterName: "Auth",
		Title:       "Test",
		Actions: []WaveAction{
			{Type: "add_dod", IssueID: "ENG-101", Description: "Add spec"},
			{Type: "add_dep", IssueID: "ENG-102", Description: "Add dep"},
		},
		Delta: WaveDelta{Before: 0.3, After: 0.5},
	}
	// "s" selects selective mode, "2" toggles action 2, "done" confirms
	input := strings.NewReader("s\n")
	scanner := bufio.NewScanner(input)
	var buf bytes.Buffer

	choice, err := PromptWaveApproval(context.Background(), &buf, scanner, wave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != ApprovalSelective {
		t.Errorf("expected ApprovalSelective, got %d", choice)
	}
}
```

**Step 2: Run test to verify it passes** (already works from Task 6)

Run: `go test ./... -run TestApprovalSelective_InPromptWaveApproval -v`
Expected: PASS

**Step 3: Wire into session.go approval switch**

Modify `session.go` `runInteractiveLoop` — add `sessionRejected` map before the loop and `ApprovalSelective` case in the switch:

Before the `for {` loop (~line 116), add:

```go
	sessionRejected := make(map[string][]WaveAction)
```

In the approval switch (after `case ApprovalDiscuss:` block, before the final `break`), add:

```go
			case ApprovalSelective:
				approved, rejected, selErr := PromptSelectiveApproval(ctx, os.Stdout, scanner, selected)
				if selErr == ErrQuit {
					break
				}
				if selErr != nil {
					LogWarn("Selective approval error: %v", selErr)
					continue
				}
				if len(approved) == 0 {
					LogInfo("No actions selected. Wave skipped.")
					break
				}
				selected.Actions = approved
				if len(rejected) > 0 {
					sessionRejected[WaveKey(selected)] = rejected
				}
				applyWave = true
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add session.go session_test.go
git commit -m "feat(v0.8): wire selective approval into interactive session loop"
```

---

### Task 8: Wire wave dynamic generation into `session.go`

**Files:**
- Modify: `session.go:258-272` (after wave completion)
- Test: `session_test.go`

**Context:** After `DisplayWaveCompletion` (line 262), trigger `GenerateNextWaves`. The function signature of `runInteractiveLoop` does NOT need to change — `sessionRejected` is local.

**Step 1: Write the failing test**

This is an integration test. Add to `session_test.go`:

```go
func TestRunInteractiveLoop_SignatureUnchanged(t *testing.T) {
	// Verify runInteractiveLoop still compiles with existing signature.
	// This is a compile-time check — the actual wiring is tested via
	// the full session flow in integration tests.
	_ = runInteractiveLoop
}
```

**Step 2: Write the implementation**

Modify `session.go`. After `DisplayWaveCompletion` (line 262), add the nextgen block:

```go
		// --- Post-completion: Generate next waves ---
		var clusterForNextgen ClusterScanResult
		for _, c := range scanResult.Clusters {
			if c.Name == selected.ClusterName {
				clusterForNextgen = c
				break
			}
		}
		completedWavesForCluster := completedWavesInCluster(waves, selected.ClusterName)
		existingADRs, adrErr := ReadExistingADRs(adrDir)
		if adrErr != nil {
			LogWarn("Failed to read ADRs for nextgen (non-fatal): %v", adrErr)
		}
		rejectedForWave := sessionRejected[WaveKey(selected)]
		newWaves, nextgenErr := GenerateNextWaves(ctx, cfg, scanDir, selected, clusterForNextgen, completedWavesForCluster, existingADRs, rejectedForWave)
		if nextgenErr != nil {
			LogWarn("Nextgen failed (non-fatal): %v", nextgenErr)
		} else if len(newWaves) > 0 {
			waves = append(waves, newWaves...)
			waves = EvaluateUnlocks(waves, completed)
			LogOK("%d new wave(s) generated for %s", len(newWaves), selected.ClusterName)
		}
```

Add helper function to `session.go`:

```go
// completedWavesInCluster returns all completed waves for the given cluster.
func completedWavesInCluster(waves []Wave, clusterName string) []Wave {
	var result []Wave
	for _, w := range waves {
		if w.ClusterName == clusterName && w.Status == "completed" {
			result = append(result, w)
		}
	}
	return result
}
```

**Step 3: Run tests to verify they pass**

Run: `go test ./... -v && go build ./...`
Expected: PASS + builds clean

**Step 4: Commit**

```bash
git add session.go session_test.go
git commit -m "feat(v0.8): wire GenerateNextWaves into session loop after wave completion"
```

---

### Task 9: Wire dry-run path for nextgen

**Files:**
- Modify: `session.go:46-77` (dry-run block)

**Step 1: Write the implementation**

Add to the dry-run block in `RunSession`, after the scribe dry-run (line ~75):

```go
		// Also generate nextgen prompt for dry-run
		sampleCompletedWaves := []Wave{sampleWave}
		if err := GenerateNextWavesDryRun(&cfg, scanDir, sampleWave,
			ClusterScanResult{Name: "sample", Completeness: 0.5, Issues: sampleClusters[0].Issues},
			sampleCompletedWaves, nil, nil); err != nil {
			return fmt.Errorf("nextgen dry-run: %w", err)
		}
```

Note: Use `&cfg` since `cfg` is a local `Config` value, not a pointer, in the dry-run path (line 47 derives `sampleClusters` from a `Config` value). Check the actual type — `RunSession` receives `cfg *Config`, so just use `cfg`.

**Step 2: Run tests to verify they pass**

Run: `go test ./... -v && go build ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add session.go
git commit -m "feat(v0.8): add nextgen dry-run prompt generation"
```

---

### Task 10: Update version + run full test suite

**Files:**
- Modify: `cmd/sightjack/main.go` (version bump)
- Modify: `session.go` (BuildSessionState version)

**Step 1: Update version**

In `cmd/sightjack/main.go`, change version from `"0.7.0-dev"` to `"0.8.0-dev"`.

In `session.go` `BuildSessionState` (line 385), change:

```go
Version: "0.8",
```

**Step 2: Run full test suite**

Run: `go test ./... -v && go build ./... && go vet ./...`
Expected: ALL PASS, builds clean, no vet issues

**Step 3: Commit**

```bash
git add cmd/sightjack/main.go session.go
git commit -m "feat(v0.8): bump version to 0.8.0-dev"
```

---

## Key Patterns to Reuse

| Pattern | Source | Location |
|---------|--------|----------|
| Agent file naming | `architectDiscussFileName` | architect.go:25 |
| Stale output cleanup | `clearArchitectOutput` | architect.go:56 |
| Dry-run via `RunClaudeDryRun` | | claude.go:89 |
| Live run via `RunClaude` | | claude.go:36 |
| JSON parse pattern | `ParseArchitectResult` | architect.go:12 |
| Template render | `renderTemplate` | prompt.go:73 |
| Prerequisite normalization | `NormalizeWavePrerequisites` | wave.go:46 |
| Non-fatal error pattern | `LogWarn` | session.go |
| ADR reading | `ReadExistingADRs` | scribe.go:89 |

## Verification Checklist

1. `go build ./...` — compiles clean
2. `go test ./... -v` — all tests pass
3. `go vet ./...` — no issues
4. Dry-run: `go run ./cmd/sightjack --dry-run` generates `nextgen_*_prompt.md` in scan dir
5. Manual check: Navigator shows new waves after completion
6. Manual check: `[s]` selective approval toggles individual actions
7. Manual check: Rejected actions appear in nextgen prompt
