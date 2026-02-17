# Sightjack v0.3 — Architect Agent Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `[d] Discuss` option to wave approval, enabling single-turn Architect Agent discussion that can modify wave actions before approval.

**Architecture:** Wave approval prompt gains a 4th option `[d] Discuss`. Selecting it prompts for a free-text topic, runs Claude subprocess with architect prompt template, parses JSON response, displays analysis and optional wave modifications, then loops back to approval. Same subprocess + JSON file pattern as existing Pass 1-4.

**Tech Stack:** Go, `text/template`, `encoding/json`, `bufio`, Claude CLI subprocess (`--print`)

---

### Task 1: ApprovalChoice enum and ArchitectResponse type (model.go)

**Context:** Currently `PromptWaveApproval` returns `(bool, error)`. We need a richer return type to distinguish approve/reject/discuss/quit. We also need the `ArchitectResponse` type for architect output JSON.

**Files:**
- Modify: `model.go:87-132` (after existing Wave types)
- Test: `model_test.go`

**Step 1: Write the failing test**

Add to `model_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestArchitectResponse -v`
Expected: FAIL — `ArchitectResponse` undefined

**Step 3: Write minimal implementation**

Add to `model.go` after `Ripple` type (after line 132):

```go
// ApprovalChoice represents the human's choice at the wave approval prompt.
type ApprovalChoice int

const (
	ApprovalApprove ApprovalChoice = iota
	ApprovalReject
	ApprovalDiscuss
	ApprovalQuit
)

// ArchitectResponse is the output of an architect discussion round.
type ArchitectResponse struct {
	Analysis     string `json:"analysis"`
	ModifiedWave *Wave  `json:"modified_wave"`
	Reasoning    string `json:"reasoning"`
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -run TestArchitectResponse -v`
Expected: PASS

**Step 5: Run full suite**

Run: `go test ./... -v`
Expected: ALL PASS (no existing tests break)

**Step 6: Commit**

```bash
git add model.go model_test.go
git commit -m "feat(v0.3): add ApprovalChoice enum and ArchitectResponse type"
```

---

### Task 2: Refactor PromptWaveApproval to return ApprovalChoice (cli.go)

**Context:** `PromptWaveApproval` currently returns `(bool, error)`. Change it to return `(ApprovalChoice, error)` and add `[d] Discuss` option. This is a **behavioral change** — existing callers (`session.go`, `cli_test.go`) must be updated in the same step.

**Files:**
- Modify: `cli.go:64-89` (PromptWaveApproval function)
- Modify: `cli_test.go:50-119` (existing approval tests)
- Modify: `session.go:82-93` (approval caller in interactive loop)

**Step 1: Update existing tests to use ApprovalChoice**

In `cli_test.go`, change `TestPromptWaveApproval_Approve`:

```go
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

	scanner := bufio.NewScanner(strings.NewReader("a\n"))
	var output bytes.Buffer
	ctx := context.Background()

	choice, err := PromptWaveApproval(ctx, &output, scanner, wave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != ApprovalApprove {
		t.Errorf("expected ApprovalApprove, got %d", choice)
	}
}
```

Change `TestPromptWaveApproval_Reject`:

```go
func TestPromptWaveApproval_Reject(t *testing.T) {
	wave := Wave{ID: "auth-w1", Actions: []WaveAction{}}

	scanner := bufio.NewScanner(strings.NewReader("r\n"))
	var output bytes.Buffer
	ctx := context.Background()

	choice, err := PromptWaveApproval(ctx, &output, scanner, wave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != ApprovalReject {
		t.Errorf("expected ApprovalReject, got %d", choice)
	}
}
```

Add new test `TestPromptWaveApproval_Discuss`:

```go
func TestPromptWaveApproval_Discuss(t *testing.T) {
	wave := Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Dependency Ordering",
		Actions:     []WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
		Delta:       WaveDelta{Before: 0.25, After: 0.40},
	}

	scanner := bufio.NewScanner(strings.NewReader("d\n"))
	var output bytes.Buffer
	ctx := context.Background()

	choice, err := PromptWaveApproval(ctx, &output, scanner, wave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != ApprovalDiscuss {
		t.Errorf("expected ApprovalDiscuss, got %d", choice)
	}
	if !strings.Contains(output.String(), "[d] Discuss") {
		t.Error("expected [d] Discuss in prompt output")
	}
}
```

Update `TestPromptSequence_SelectionThenApproval` to use `ApprovalChoice`:

```go
func TestPromptSequence_SelectionThenApproval(t *testing.T) {
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps",
			Actions: []WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
			Delta:   WaveDelta{Before: 0.25, After: 0.40}},
	}
	scanner := bufio.NewScanner(strings.NewReader("1\na\n"))
	var output bytes.Buffer
	ctx := context.Background()

	selected, err := PromptWaveSelection(ctx, &output, scanner, waves)
	if err != nil {
		t.Fatalf("selection: unexpected error: %v", err)
	}
	if selected.ID != "auth-w1" {
		t.Errorf("expected auth-w1, got %s", selected.ID)
	}

	choice, err := PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("approval: unexpected error: %v", err)
	}
	if choice != ApprovalApprove {
		t.Error("expected ApprovalApprove (scanner likely lost buffered input)")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run "TestPromptWaveApproval|TestPromptSequence" -v`
Expected: FAIL — `PromptWaveApproval` still returns `(bool, error)`, not `(ApprovalChoice, error)`

**Step 3: Update PromptWaveApproval implementation**

Replace `cli.go:64-89` with:

```go
// PromptWaveApproval displays a wave proposal and reads approve/reject/discuss.
func PromptWaveApproval(ctx context.Context, w io.Writer, s *bufio.Scanner, wave Wave) (ApprovalChoice, error) {
	fmt.Fprintf(w, "\n--- %s - %s ---\n", wave.ClusterName, wave.Title)
	fmt.Fprintf(w, "  Proposed actions (%d):\n", len(wave.Actions))
	for i, a := range wave.Actions {
		fmt.Fprintf(w, "    %d. [%s] %s: %s\n", i+1, a.Type, a.IssueID, a.Description)
	}
	fmt.Fprintf(w, "\n  Expected: %.0f%% -> %.0f%%\n", wave.Delta.Before*100, wave.Delta.After*100)
	fmt.Fprint(w, "\n  [a] Approve all  [r] Reject  [d] Discuss  [q] Back to navigator: ")

	line, err := ScanLine(ctx, s)
	if err != nil {
		return ApprovalQuit, ErrQuit
	}
	input := strings.TrimSpace(strings.ToLower(line))
	switch input {
	case "a":
		return ApprovalApprove, nil
	case "r":
		return ApprovalReject, nil
	case "d":
		return ApprovalDiscuss, nil
	case "q":
		return ApprovalQuit, ErrQuit
	default:
		return ApprovalQuit, fmt.Errorf("invalid input: %s", input)
	}
}
```

**Step 4: Update session.go caller**

Replace `session.go:81-93` with:

```go
		// Prompt wave approval (with discuss loop)
		var applyWave bool
		for {
			choice, err := PromptWaveApproval(ctx, os.Stdout, scanner, selected)
			if err == ErrQuit {
				break
			}
			if err != nil {
				LogWarn("Invalid input: %v", err)
				continue
			}

			switch choice {
			case ApprovalApprove:
				applyWave = true
			case ApprovalReject:
				LogInfo("Wave rejected.")
			case ApprovalDiscuss:
				LogInfo("Discussion mode not yet implemented.")
			}
			break
		}
		if !applyWave {
			continue
		}
```

Note: The `ApprovalDiscuss` case is a stub for now — it just logs and falls through to `break`, which means the wave won't be applied. Task 7 will wire it up to the actual architect flow.

**Step 5: Run tests to verify they pass**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add cli.go cli_test.go session.go
git commit -m "feat(v0.3): refactor PromptWaveApproval to return ApprovalChoice with [d] Discuss"
```

---

### Task 3: Add PromptDiscussTopic and DisplayArchitectResponse (cli.go)

**Context:** Two new CLI functions: one reads a free-text topic from the user, the other displays the architect's response including any wave modifications.

**Files:**
- Modify: `cli.go` (add after DisplayRippleEffects)
- Modify: `cli_test.go` (add new tests)

**Step 1: Write failing tests**

Add to `cli_test.go`:

```go
func TestPromptDiscussTopic(t *testing.T) {
	// given
	scanner := bufio.NewScanner(strings.NewReader("Should we split ENG-101?\n"))
	var output bytes.Buffer
	ctx := context.Background()

	// when
	topic, err := PromptDiscussTopic(ctx, &output, scanner)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if topic != "Should we split ENG-101?" {
		t.Errorf("expected topic text, got: %s", topic)
	}
	if !strings.Contains(output.String(), "Topic") {
		t.Error("expected Topic prompt in output")
	}
}

func TestPromptDiscussTopic_Quit(t *testing.T) {
	// given
	scanner := bufio.NewScanner(strings.NewReader("q\n"))
	var output bytes.Buffer
	ctx := context.Background()

	// when
	_, err := PromptDiscussTopic(ctx, &output, scanner)

	// then
	if err != ErrQuit {
		t.Errorf("expected ErrQuit, got %v", err)
	}
}

func TestPromptDiscussTopic_Empty(t *testing.T) {
	// given: empty input should error
	scanner := bufio.NewScanner(strings.NewReader("\n"))
	var output bytes.Buffer
	ctx := context.Background()

	// when
	_, err := PromptDiscussTopic(ctx, &output, scanner)

	// then
	if err == nil {
		t.Fatal("expected error for empty topic")
	}
	if err == ErrQuit {
		t.Error("expected non-quit error for empty topic")
	}
}

func TestDisplayArchitectResponse_WithModifiedWave(t *testing.T) {
	// given
	resp := &ArchitectResponse{
		Analysis: "Splitting is unnecessary for this scale.",
		ModifiedWave: &Wave{
			ID:          "auth-w1",
			ClusterName: "Auth",
			Title:       "Dependency Ordering",
			Actions: []WaveAction{
				{Type: "add_dependency", IssueID: "ENG-101", Description: "Auth before token"},
				{Type: "add_dod", IssueID: "ENG-101", Description: "Middleware interface"},
			},
			Delta: WaveDelta{Before: 0.25, After: 0.42},
		},
		Reasoning: "Project scale favors fewer issues.",
	}
	var output bytes.Buffer

	// when
	DisplayArchitectResponse(&output, resp)

	// then
	out := output.String()
	if !strings.Contains(out, "Splitting is unnecessary") {
		t.Error("expected analysis text in output")
	}
	if !strings.Contains(out, "Middleware interface") {
		t.Error("expected modified action in output")
	}
	if !strings.Contains(out, "Project scale") {
		t.Error("expected reasoning in output")
	}
}

func TestDisplayArchitectResponse_NoModifications(t *testing.T) {
	// given
	resp := &ArchitectResponse{
		Analysis:     "Current actions look good.",
		ModifiedWave: nil,
		Reasoning:    "No changes needed.",
	}
	var output bytes.Buffer

	// when
	DisplayArchitectResponse(&output, resp)

	// then
	out := output.String()
	if !strings.Contains(out, "Current actions look good") {
		t.Error("expected analysis text in output")
	}
	if strings.Contains(out, "Modified") {
		t.Error("should not show modified section when no modifications")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run "TestPromptDiscussTopic|TestDisplayArchitectResponse" -v`
Expected: FAIL — undefined functions

**Step 3: Write minimal implementation**

Add to `cli.go` after `DisplayRippleEffects`:

```go
// PromptDiscussTopic reads a free-text discussion topic from the user.
func PromptDiscussTopic(ctx context.Context, w io.Writer, s *bufio.Scanner) (string, error) {
	fmt.Fprint(w, "\n  Topic: ")

	line, err := ScanLine(ctx, s)
	if err != nil {
		return "", ErrQuit
	}
	input := strings.TrimSpace(line)
	if input == "q" {
		return "", ErrQuit
	}
	if input == "" {
		return "", fmt.Errorf("empty topic")
	}
	return input, nil
}

// DisplayArchitectResponse shows the architect's analysis and any wave modifications.
func DisplayArchitectResponse(w io.Writer, resp *ArchitectResponse) {
	fmt.Fprintf(w, "\n  [Architect] %s\n", resp.Analysis)
	if resp.Reasoning != "" {
		fmt.Fprintf(w, "\n  Reasoning: %s\n", resp.Reasoning)
	}
	if resp.ModifiedWave != nil {
		fmt.Fprintf(w, "\n  Modified actions (%d):\n", len(resp.ModifiedWave.Actions))
		for i, a := range resp.ModifiedWave.Actions {
			fmt.Fprintf(w, "    %d. [%s] %s: %s\n", i+1, a.Type, a.IssueID, a.Description)
		}
		fmt.Fprintf(w, "\n  Expected: %.0f%% -> %.0f%%\n",
			resp.ModifiedWave.Delta.Before*100, resp.ModifiedWave.Delta.After*100)
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add cli.go cli_test.go
git commit -m "feat(v0.3): add PromptDiscussTopic and DisplayArchitectResponse"
```

---

### Task 4: Architect prompt template (prompt.go + templates)

**Context:** Add `ArchitectDiscussPromptData` and `RenderArchitectDiscussPrompt` following the exact pattern of existing `RenderWaveApplyPrompt`. Create en/ja template files.

**Files:**
- Modify: `prompt.go:37-44` (add ArchitectDiscussPromptData after WaveApplyPromptData)
- Modify: `prompt.go:77-80` (add RenderArchitectDiscussPrompt after RenderWaveApplyPrompt)
- Create: `prompts/templates/architect_discuss_en.md.tmpl`
- Create: `prompts/templates/architect_discuss_ja.md.tmpl`
- Test: `prompt_test.go`

**Step 1: Write the failing test**

Add to `prompt_test.go`:

```go
func TestRenderArchitectDiscussPrompt(t *testing.T) {
	// given
	data := ArchitectDiscussPromptData{
		ClusterName: "Auth",
		WaveTitle:   "Dependency Ordering",
		WaveActions: `[{"type":"add_dependency","issue_id":"ENG-101","description":"Auth before token"}]`,
		Topic:       "Should we split ENG-101?",
		OutputPath:  "/tmp/architect_auth_auth-w1.json",
	}

	// when
	result, err := RenderArchitectDiscussPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "Auth") {
		t.Error("expected cluster name in output")
	}
	if !strings.Contains(result, "Should we split ENG-101?") {
		t.Error("expected topic in output")
	}
	if !strings.Contains(result, "/tmp/architect_auth_auth-w1.json") {
		t.Error("expected output path in output")
	}
}

func TestRenderArchitectDiscussPrompt_Japanese(t *testing.T) {
	// given
	data := ArchitectDiscussPromptData{
		ClusterName: "Auth",
		WaveTitle:   "Dependency Ordering",
		WaveActions: `[{"type":"add_dependency","issue_id":"ENG-101"}]`,
		Topic:       "ENG-101を分割すべき？",
		OutputPath:  "/tmp/out.json",
	}

	// when
	result, err := RenderArchitectDiscussPrompt("ja", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "Architect Agent") {
		t.Error("expected Architect Agent in Japanese prompt")
	}
	if !strings.Contains(result, "Auth") {
		t.Error("expected cluster name in output")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run "TestRenderArchitectDiscussPrompt" -v`
Expected: FAIL — undefined type and function

**Step 3: Create template files**

Create `prompts/templates/architect_discuss_en.md.tmpl`:

```
You are an Architect Agent for the Sightjack tool.
Analyze the human's discussion topic in the context of this cluster and wave.
You may suggest modifications to the wave actions if the discussion warrants it.

## Cluster
{{.ClusterName}}

## Current Wave
Title: {{.WaveTitle}}

### Current Actions
{{.WaveActions}}

## Discussion Topic
{{.Topic}}

## Instructions
1. Analyze the topic considering the full cluster context
2. Provide your analysis as clear text
3. If the discussion suggests changes to the wave, provide a modified wave
4. If no changes are needed, set modified_wave to null
5. Explain your reasoning

## Output
Write the following JSON to **{{.OutputPath}}**:

```json
{
  "analysis": "Your analysis text here",
  "modified_wave": null,
  "reasoning": "Why you suggest these changes (or no changes)"
}
```

If you suggest wave modifications, replace null with the full modified wave object:

```json
{
  "analysis": "Your analysis text here",
  "modified_wave": {
    "id": "wave-id",
    "cluster_name": "ClusterName",
    "title": "Wave Title",
    "description": "Wave description",
    "actions": [...],
    "prerequisites": [...],
    "delta": {"before": 0.0, "after": 0.0},
    "status": "available"
  },
  "reasoning": "Why you suggest these changes"
}
```

Important: Write output directly to the file path above. Do not write to stdout.
```

Create `prompts/templates/architect_discuss_ja.md.tmpl`:

```
あなたは Architect Agent です。
人間の議論トピックをクラスタとWaveの文脈で分析してください。
議論の結果、Waveアクションの修正が必要であれば提案してください。

## クラスタ
{{.ClusterName}}

## 現在のWave
タイトル: {{.WaveTitle}}

### 現在のアクション
{{.WaveActions}}

## 議論トピック
{{.Topic}}

## 指示
1. クラスタ全体のコンテキストを考慮してトピックを分析する
2. 分析結果を明確なテキストで提供する
3. 議論の結果Waveの変更が必要であれば、修正されたWaveを提供する
4. 変更不要であればmodified_waveをnullにする
5. 推論の根拠を説明する

## 出力
以下の JSON を **{{.OutputPath}}** に書き込んでください:

```json
{
  "analysis": "分析テキスト",
  "modified_wave": null,
  "reasoning": "変更の理由（または変更不要の理由）"
}
```

Wave修正を提案する場合は、nullを完全なWaveオブジェクトに置き換えてください:

```json
{
  "analysis": "分析テキスト",
  "modified_wave": {
    "id": "wave-id",
    "cluster_name": "クラスタ名",
    "title": "Waveタイトル",
    "description": "Wave説明",
    "actions": [...],
    "prerequisites": [...],
    "delta": {"before": 0.0, "after": 0.0},
    "status": "available"
  },
  "reasoning": "変更の理由"
}
```

重要: 出力は上記のファイルパスに直接書き込んでください。標準出力には書かないでください。
```

**Step 4: Add prompt data type and render function to prompt.go**

Add `ArchitectDiscussPromptData` after `WaveApplyPromptData` (after line 44):

```go
// ArchitectDiscussPromptData holds template data for the architect discussion prompt.
type ArchitectDiscussPromptData struct {
	ClusterName string
	WaveTitle   string
	WaveActions string
	Topic       string
	OutputPath  string
}
```

Add `RenderArchitectDiscussPrompt` after `RenderWaveApplyPrompt` (after line 80):

```go
// RenderArchitectDiscussPrompt renders the architect discussion prompt for the given language.
func RenderArchitectDiscussPrompt(lang string, data ArchitectDiscussPromptData) (string, error) {
	name := fmt.Sprintf("prompts/templates/architect_discuss_%s.md.tmpl", lang)
	return renderTemplate(name, data)
}
```

**Step 5: Run tests to verify they pass**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add prompt.go prompt_test.go prompts/templates/architect_discuss_en.md.tmpl prompts/templates/architect_discuss_ja.md.tmpl
git commit -m "feat(v0.3): add architect discussion prompt template (en/ja)"
```

---

### Task 5: Architect subprocess execution (architect.go)

**Context:** Create `architect.go` with `RunArchitectDiscuss` and `ParseArchitectResult`, following the exact same pattern as `RunWaveApply` in `wave.go`. The function renders the prompt, calls `RunClaude`, and parses the JSON output.

**Files:**
- Create: `architect.go`
- Create: `architect_test.go`

**Step 1: Write the failing tests**

Create `architect_test.go`:

```go
package sightjack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseArchitectResult(t *testing.T) {
	// given: a valid architect response JSON file
	dir := t.TempDir()
	path := filepath.Join(dir, "architect_auth_auth-w1.json")
	data := ArchitectResponse{
		Analysis: "Splitting is unnecessary.",
		ModifiedWave: &Wave{
			ID:          "auth-w1",
			ClusterName: "Auth",
			Title:       "Dependency Ordering",
			Actions: []WaveAction{
				{Type: "add_dependency", IssueID: "ENG-101", Description: "Auth before token"},
				{Type: "add_dod", IssueID: "ENG-101", Description: "Middleware interface"},
			},
			Delta:  WaveDelta{Before: 0.25, After: 0.42},
			Status: "available",
		},
		Reasoning: "Project scale favors fewer issues.",
	}
	raw, _ := json.MarshalIndent(data, "", "  ")
	os.WriteFile(path, raw, 0644)

	// when
	result, err := ParseArchitectResult(path)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Analysis != "Splitting is unnecessary." {
		t.Errorf("unexpected analysis: %s", result.Analysis)
	}
	if result.ModifiedWave == nil {
		t.Fatal("expected non-nil modified_wave")
	}
	if len(result.ModifiedWave.Actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(result.ModifiedWave.Actions))
	}
}

func TestParseArchitectResult_NilWave(t *testing.T) {
	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "architect.json")
	os.WriteFile(path, []byte(`{"analysis":"No changes.","modified_wave":null,"reasoning":"OK"}`), 0644)

	// when
	result, err := ParseArchitectResult(path)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ModifiedWave != nil {
		t.Error("expected nil modified_wave")
	}
}

func TestParseArchitectResult_FileNotFound(t *testing.T) {
	// when
	_, err := ParseArchitectResult("/nonexistent/path.json")

	// then
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestArchitectDiscussFileName(t *testing.T) {
	wave := Wave{ID: "auth-w1", ClusterName: "Auth"}
	name := architectDiscussFileName(wave)
	if name != "architect_auth_auth-w1.json" {
		t.Errorf("expected architect_auth_auth-w1.json, got %s", name)
	}
}

func TestArchitectDiscussFileName_SpecialChars(t *testing.T) {
	wave := Wave{ID: "w-1", ClusterName: "UI/Frontend"}
	name := architectDiscussFileName(wave)
	if name != "architect_ui_frontend_w-1.json" {
		t.Errorf("expected architect_ui_frontend_w-1.json, got %s", name)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run "TestParseArchitectResult|TestArchitectDiscussFileName" -v`
Expected: FAIL — undefined functions

**Step 3: Write minimal implementation**

Create `architect.go`:

```go
package sightjack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ParseArchitectResult reads and parses an architect response JSON file.
func ParseArchitectResult(path string) (*ArchitectResponse, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read architect result: %w", err)
	}
	var result ArchitectResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse architect result: %w", err)
	}
	return &result, nil
}

// architectDiscussFileName returns the output filename for an architect discussion.
func architectDiscussFileName(wave Wave) string {
	return fmt.Sprintf("architect_%s_%s.json", sanitizeName(wave.ClusterName), sanitizeName(wave.ID))
}

// RunArchitectDiscuss executes a single-turn architect discussion via Claude subprocess.
func RunArchitectDiscuss(ctx context.Context, cfg *Config, scanDir string, wave Wave, topic string) (*ArchitectResponse, error) {
	outputFile := filepath.Join(scanDir, architectDiscussFileName(wave))

	actionsJSON, err := json.Marshal(wave.Actions)
	if err != nil {
		return nil, fmt.Errorf("marshal wave actions: %w", err)
	}

	prompt, err := RenderArchitectDiscussPrompt(cfg.Lang, ArchitectDiscussPromptData{
		ClusterName: wave.ClusterName,
		WaveTitle:   wave.Title,
		WaveActions: string(actionsJSON),
		Topic:       topic,
		OutputPath:  outputFile,
	})
	if err != nil {
		return nil, fmt.Errorf("render architect prompt: %w", err)
	}

	LogScan("Architect discussing: %s - %s", wave.ClusterName, topic)
	if _, err := RunClaude(ctx, cfg, prompt, os.Stdout); err != nil {
		return nil, fmt.Errorf("architect discuss %s: %w", wave.ID, err)
	}

	result, err := ParseArchitectResult(outputFile)
	if err != nil {
		return nil, fmt.Errorf("parse architect result %s: %w", wave.ID, err)
	}

	return result, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add architect.go architect_test.go
git commit -m "feat(v0.3): add architect subprocess execution and result parsing"
```

---

### Task 6: Dry-run support for architect discussion

**Context:** Add dry-run support to `RunArchitectDiscuss` so that `session --dry-run` generates the architect prompt file as well. Follow the same pattern as `RunWaveGenerate` dry-run.

**Files:**
- Modify: `architect.go` (add dryRun parameter to RunArchitectDiscuss)
- Modify: `architect_test.go` (add dry-run test)
- Modify: `session.go:31-43` (add architect prompt to dry-run path)

**Step 1: Write the failing test**

Add to `architect_test.go`:

```go
func TestRunArchitectDiscuss_DryRun(t *testing.T) {
	// given
	scanDir := t.TempDir()
	cfg := &Config{
		Lang:   "en",
		Claude: ClaudeConfig{Command: "claude", TimeoutSec: 60},
	}
	wave := Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Dependency Ordering",
		Actions:     []WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
	}

	// when
	err := RunArchitectDiscussDryRun(cfg, scanDir, wave, "test topic")

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	promptFile := filepath.Join(scanDir, "architect_auth_auth-w1_prompt.md")
	if _, err := os.Stat(promptFile); os.IsNotExist(err) {
		t.Error("expected architect prompt file to be generated")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestRunArchitectDiscuss_DryRun -v`
Expected: FAIL — `RunArchitectDiscussDryRun` undefined

**Step 3: Write minimal implementation**

Add to `architect.go`:

```go
// RunArchitectDiscussDryRun saves the architect prompt to a file instead of executing Claude.
func RunArchitectDiscussDryRun(cfg *Config, scanDir string, wave Wave, topic string) error {
	actionsJSON, err := json.Marshal(wave.Actions)
	if err != nil {
		return fmt.Errorf("marshal wave actions: %w", err)
	}

	outputFile := filepath.Join(scanDir, architectDiscussFileName(wave))
	prompt, err := RenderArchitectDiscussPrompt(cfg.Lang, ArchitectDiscussPromptData{
		ClusterName: wave.ClusterName,
		WaveTitle:   wave.Title,
		WaveActions: string(actionsJSON),
		Topic:       topic,
		OutputPath:  outputFile,
	})
	if err != nil {
		return fmt.Errorf("render architect prompt: %w", err)
	}

	dryRunName := fmt.Sprintf("architect_%s_%s", sanitizeName(wave.ClusterName), sanitizeName(wave.ID))
	return RunClaudeDryRun(cfg, prompt, scanDir, dryRunName)
}
```

Update `session.go` dry-run block (after line 40, before `return nil`):

```go
		// Also generate architect discuss prompt for dry-run
		sampleWave := Wave{
			ID:          "sample-w1",
			ClusterName: "sample",
			Title:       "Sample Wave",
			Actions:     []WaveAction{{Type: "add_dod", IssueID: "SAMPLE-1", Description: "Sample DoD"}},
		}
		if err := RunArchitectDiscussDryRun(cfg, scanDir, sampleWave, "sample discussion topic"); err != nil {
			return fmt.Errorf("architect discuss dry-run: %w", err)
		}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 5: Update session dry-run test**

Add to `TestRunSession_DryRunGeneratesWavePrompts` in `session_test.go` (after the wave prompt check):

```go
	// then: architect discuss prompt was generated
	architectPrompt := filepath.Join(scanDir, "architect_sample_sample-w1_prompt.md")
	if _, err := os.Stat(architectPrompt); os.IsNotExist(err) {
		t.Error("architect_sample_sample-w1_prompt.md not generated — dry-run did not reach architect step")
	}
```

Run: `go test ./... -v`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add architect.go architect_test.go session.go session_test.go
git commit -m "feat(v0.3): add architect dry-run prompt generation"
```

---

### Task 7: Wire discuss flow into session interactive loop (session.go)

**Context:** Replace the stub `LogInfo("Discussion mode not yet implemented.")` from Task 2 with the actual discuss flow: prompt topic → run architect → display response → apply modified wave → loop back to approval.

**Files:**
- Modify: `session.go:81-93` (the approval loop in RunSession)

**Step 1: Write the failing test**

Add to `session_test.go`:

This is an integration-level test that verifies the discuss sequence works with piped input. Since `RunArchitectDiscuss` calls `RunClaude` which requires the Claude binary, we test the session flow using the existing dry-run pattern where applicable. For the interactive loop, we verify the discuss branch handles the `ApprovalDiscuss` choice correctly.

```go
func TestDiscussBranchReturnsToApproval(t *testing.T) {
	// This tests the session-level logic: after a discuss round,
	// the approval loop should re-prompt (not exit).
	// We verify this indirectly through PromptWaveApproval behavior:
	// input "d\n" followed by topic, then "a\n" should eventually approve.

	// given: piped input sequence: select wave 1, discuss, enter topic, then approve
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps",
			Actions: []WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
			Delta:   WaveDelta{Before: 0.25, After: 0.40}},
	}
	input := "1\nd\nShould we split?\na\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when: selection
	selected, err := PromptWaveSelection(ctx, &output, scanner, waves)
	if err != nil {
		t.Fatalf("selection error: %v", err)
	}
	if selected.ID != "auth-w1" {
		t.Fatalf("expected auth-w1, got %s", selected.ID)
	}

	// when: first approval -> discuss
	choice, err := PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("first approval error: %v", err)
	}
	if choice != ApprovalDiscuss {
		t.Fatalf("expected ApprovalDiscuss, got %d", choice)
	}

	// when: topic input
	topic, err := PromptDiscussTopic(ctx, &output, scanner)
	if err != nil {
		t.Fatalf("topic error: %v", err)
	}
	if topic != "Should we split?" {
		t.Errorf("expected topic, got: %s", topic)
	}

	// when: second approval -> approve
	choice, err = PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("second approval error: %v", err)
	}
	if choice != ApprovalApprove {
		t.Errorf("expected ApprovalApprove after discuss, got %d", choice)
	}
}
```

**Step 2: Run test to verify it passes (it should, since the CLI functions already work)**

Run: `go test ./... -run TestDiscussBranchReturnsToApproval -v`
Expected: PASS (this tests the CLI functions which are already implemented)

**Step 3: Wire up the discuss flow in session.go**

Replace the approval loop in `session.go` (the block from Task 2's stub) with the full implementation:

```go
		// Prompt wave approval (with discuss loop)
		var applyWave bool
		for {
			choice, err := PromptWaveApproval(ctx, os.Stdout, scanner, selected)
			if err == ErrQuit {
				break
			}
			if err != nil {
				LogWarn("Invalid input: %v", err)
				continue
			}

			switch choice {
			case ApprovalApprove:
				applyWave = true
			case ApprovalReject:
				LogInfo("Wave rejected.")
			case ApprovalDiscuss:
				topic, topicErr := PromptDiscussTopic(ctx, os.Stdout, scanner)
				if topicErr == ErrQuit {
					continue
				}
				if topicErr != nil {
					LogWarn("Invalid topic: %v", topicErr)
					continue
				}
				result, discussErr := RunArchitectDiscuss(ctx, cfg, scanDir, selected, topic)
				if discussErr != nil {
					LogError("Architect discussion failed: %v", discussErr)
					continue
				}
				DisplayArchitectResponse(os.Stdout, result)
				if result.ModifiedWave != nil {
					selected = *result.ModifiedWave
				}
				continue // back to approval prompt with (possibly modified) wave
			}
			break
		}
		if !applyWave {
			continue
		}
```

**Step 4: Run full suite**

Run: `go test ./... -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add session.go session_test.go
git commit -m "feat(v0.3): wire architect discuss flow into session interactive loop"
```

---

### Task 8: Update version and state version

**Context:** Bump the version to 0.3.0-dev and update SessionState version to "0.3".

**Files:**
- Modify: `cmd/sightjack/main.go:17` (version string)
- Modify: `session.go:134` (state version)

**Step 1: Update version**

In `cmd/sightjack/main.go`, change line 17:
```go
var version = "0.3.0-dev"
```

In `session.go`, change line 134:
```go
		Version:      "0.3",
```

**Step 2: Run full suite**

Run: `go test ./... -v && go build ./...`
Expected: ALL PASS, build succeeds

**Step 3: Commit**

```bash
git add cmd/sightjack/main.go session.go
git commit -m "chore(v0.3): bump version to 0.3.0-dev"
```

---

## Task Dependency Graph

```
Task 1 (model types)
  |
  v
Task 2 (refactor PromptWaveApproval) ---> Task 3 (new CLI functions)
  |                                          |
  v                                          v
Task 4 (prompt templates) ------------> Task 5 (architect.go)
                                           |
                                           v
                                        Task 6 (dry-run)
                                           |
                                           v
                                        Task 7 (wire session loop)
                                           |
                                           v
                                        Task 8 (version bump)
```

Legend:
- Task 1: Model types (ApprovalChoice, ArchitectResponse)
- Task 2: Refactor CLI approval (breaking change)
- Task 3: New CLI functions (PromptDiscussTopic, DisplayArchitectResponse)
- Task 4: Prompt templates (en/ja)
- Task 5: Architect subprocess (architect.go)
- Task 6: Dry-run support
- Task 7: Session loop wiring
- Task 8: Version bump
