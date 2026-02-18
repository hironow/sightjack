# Sightjack v0.6 Implementation Plan — SIREN Mechanics

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add strictness levels (Fog/Alert/Lockdown), Shibito resurrection detection, and ADR contradiction checking to the sightjack CLI.

**Architecture:** StrictnessLevel string enum injected into all AI prompts to control analysis depth. Shibito detection via Scanner prompt extension (no new subprocess). ADR consistency check via Scribe prompt extension with existing ADR injection. All three features are non-blocking in v0.6 (warnings only).

**Tech Stack:** Go 1.22+, Go text/template, YAML config (gopkg.in/yaml.v3), Claude Code subprocess

---

## Codebase Orientation

- **model.go** — All domain types (Wave, ScanResult, SessionState, ScribeResponse, etc.)
- **config.go** — YAML config parsing, `Config` struct, `DefaultConfig()`
- **prompt.go** — PromptData structs + `Render*Prompt` functions using `text/template`
- **prompts/templates/*.tmpl** — Go templates, one per lang (en/ja) per agent
- **scribe.go** — Scribe Agent: `RunScribeADR`, `NextADRNumber`, `ParseScribeResult`
- **session.go** — Session orchestration: `RunSession`, `runInteractiveLoop`, `BuildSessionState`
- **navigator.go** — ASCII Link Navigator display: `RenderNavigator`, `RenderNavigatorWithWaves`
- **cli.go** — CLI display functions: `DisplayScribeResponse`, `PromptWaveApproval`, etc.
- **state.go** — `ReadState`, `WriteState`, `WriteScanResult`, `LoadScanResult`

**Test pattern:** `given/when/then` comments, function-based tests, no mocks for unit tests. JSON round-trip tests for model types. Template render tests check `strings.Contains` for key fields.

**Run all tests:** `go test ./... -v`
**Run single test:** `go test ./... -run TestName -v`
**Build check:** `go build ./...`
**Vet:** `go vet ./...`

---

### Task 1: Add StrictnessLevel type to model.go

**Files:**
- Modify: `model.go`
- Test: `model_test.go`

**Step 1: Write failing tests**

Add to `model_test.go`:

```go
func TestParseStrictnessLevel_ValidValues(t *testing.T) {
	tests := []struct {
		input    string
		expected StrictnessLevel
	}{
		{"fog", StrictnessFog},
		{"alert", StrictnessAlert},
		{"lockdown", StrictnessLockdown},
		{"FOG", StrictnessFog},
		{"Alert", StrictnessAlert},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// when
			level, err := ParseStrictnessLevel(tt.input)

			// then
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if level != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, level)
			}
		})
	}
}

func TestParseStrictnessLevel_Invalid(t *testing.T) {
	// when
	_, err := ParseStrictnessLevel("nightmare")

	// then
	if err == nil {
		t.Fatal("expected error for invalid strictness level")
	}
}

func TestStrictnessLevel_Valid(t *testing.T) {
	// given
	valid := StrictnessFog
	invalid := StrictnessLevel("nightmare")

	// then
	if !valid.Valid() {
		t.Error("expected fog to be valid")
	}
	if invalid.Valid() {
		t.Error("expected nightmare to be invalid")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run TestParseStrictnessLevel -v`
Expected: FAIL — `ParseStrictnessLevel` not defined

**Step 3: Write minimal implementation**

Add to `model.go` (after the existing `ResumeChoice` constants):

```go
// StrictnessLevel controls DoD analysis depth (SIREN difficulty system).
type StrictnessLevel string

const (
	StrictnessFog      StrictnessLevel = "fog"
	StrictnessAlert    StrictnessLevel = "alert"
	StrictnessLockdown StrictnessLevel = "lockdown"
)

// ParseStrictnessLevel parses a string into a StrictnessLevel.
// Case-insensitive. Returns error for unknown values.
func ParseStrictnessLevel(s string) (StrictnessLevel, error) {
	level := StrictnessLevel(strings.ToLower(s))
	if !level.Valid() {
		return "", fmt.Errorf("unknown strictness level: %q (valid: fog, alert, lockdown)", s)
	}
	return level, nil
}

// Valid returns true if the level is a known strictness value.
func (l StrictnessLevel) Valid() bool {
	switch l {
	case StrictnessFog, StrictnessAlert, StrictnessLockdown:
		return true
	}
	return false
}
```

Add `"fmt"` and `"strings"` to model.go imports (if not already present — model.go currently imports only `"time"`, so add `"fmt"` and `"strings"`).

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run "TestParseStrictnessLevel|TestStrictnessLevel" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add model.go model_test.go
git commit -m "feat(v0.6): add StrictnessLevel type with parse and validation"
```

---

### Task 2: Add StrictnessConfig to config.go

**Files:**
- Modify: `config.go`
- Test: `config_test.go`

**Step 1: Write failing tests**

Add to `config_test.go`:

```go
func TestDefaultConfig_StrictnessFog(t *testing.T) {
	// when
	cfg := DefaultConfig()

	// then
	if cfg.Strictness.Default != StrictnessFog {
		t.Errorf("expected fog, got %s", cfg.Strictness.Default)
	}
}

func TestLoadConfig_StrictnessAlert(t *testing.T) {
	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(`
linear:
  team: TEST
  project: Test
strictness:
  default: alert
`), 0644)

	// when
	cfg, err := LoadConfig(path)

	// then
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Strictness.Default != StrictnessAlert {
		t.Errorf("expected alert, got %s", cfg.Strictness.Default)
	}
}

func TestLoadConfig_StrictnessMissing_DefaultsFog(t *testing.T) {
	// given: config without strictness section
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(`
linear:
  team: TEST
  project: Test
`), 0644)

	// when
	cfg, err := LoadConfig(path)

	// then
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Strictness.Default != StrictnessFog {
		t.Errorf("expected fog default, got %s", cfg.Strictness.Default)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run "TestDefaultConfig_Strictness|TestLoadConfig_Strictness" -v`
Expected: FAIL — `cfg.Strictness` not defined

**Step 3: Write minimal implementation**

Add to `config.go`:

```go
// StrictnessConfig holds DoD strictness level settings.
type StrictnessConfig struct {
	Default StrictnessLevel `yaml:"default"`
}
```

Add to `Config` struct:

```go
Strictness StrictnessConfig `yaml:"strictness"`
```

Update `DefaultConfig()` to include:

```go
Strictness: StrictnessConfig{
    Default: StrictnessFog,
},
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run "TestDefaultConfig_Strictness|TestLoadConfig_Strictness" -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `go test ./... -v`
Expected: All PASS (no regressions)

**Step 6: Commit**

```bash
git add config.go config_test.go
git commit -m "feat(v0.6): add StrictnessConfig to Config with fog default"
```

---

### Task 3: Add ShibitoWarning type and ScanResult extension

**Files:**
- Modify: `model.go`
- Test: `model_test.go`

**Step 1: Write failing tests**

Add to `model_test.go`:

```go
func TestShibitoWarning_JSONRoundTrip(t *testing.T) {
	// given
	original := ShibitoWarning{
		ClosedIssueID:  "ENG-045",
		CurrentIssueID: "ENG-102",
		Description:    "Token management circular dependency re-emerging",
		RiskLevel:      "high",
	}

	// when
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ShibitoWarning
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if decoded.ClosedIssueID != "ENG-045" {
		t.Errorf("expected ENG-045, got %s", decoded.ClosedIssueID)
	}
	if decoded.RiskLevel != "high" {
		t.Errorf("expected high, got %s", decoded.RiskLevel)
	}
}

func TestScanResult_ShibitoWarnings_OmittedWhenEmpty(t *testing.T) {
	// given
	result := ScanResult{Completeness: 0.5}

	// when
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// then
	if strings.Contains(string(data), "shibito_warnings") {
		t.Error("expected shibito_warnings to be omitted when empty")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run "TestShibitoWarning|TestScanResult_Shibito" -v`
Expected: FAIL — `ShibitoWarning` not defined

**Step 3: Write minimal implementation**

Add to `model.go`:

```go
// ShibitoWarning represents a detected resurrection risk — a previously
// closed issue pattern re-emerging in current issues.
type ShibitoWarning struct {
	ClosedIssueID  string `json:"closed_issue_id"`
	CurrentIssueID string `json:"current_issue_id"`
	Description    string `json:"description"`
	RiskLevel      string `json:"risk_level"`
}
```

Add `ShibitoWarnings` field to `ScanResult`:

```go
ShibitoWarnings []ShibitoWarning `json:"shibito_warnings,omitempty"`
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run "TestShibitoWarning|TestScanResult_Shibito" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add model.go model_test.go
git commit -m "feat(v0.6): add ShibitoWarning type and ScanResult.ShibitoWarnings"
```

---

### Task 4: Add ADRConflict type and ScribeResponse extension

**Files:**
- Modify: `model.go`
- Test: `model_test.go`

**Step 1: Write failing tests**

Add to `model_test.go`:

```go
func TestADRConflict_JSONRoundTrip(t *testing.T) {
	// given
	original := ADRConflict{
		ExistingADRID: "0002",
		Description:   "Contradicts ADR-0002 decision on session storage",
	}

	// when
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ADRConflict
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if decoded.ExistingADRID != "0002" {
		t.Errorf("expected 0002, got %s", decoded.ExistingADRID)
	}
	if decoded.Description != "Contradicts ADR-0002 decision on session storage" {
		t.Errorf("unexpected description: %s", decoded.Description)
	}
}

func TestScribeResponse_Conflicts_OmittedWhenEmpty(t *testing.T) {
	// given
	resp := ScribeResponse{ADRID: "0001", Title: "test"}

	// when
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// then
	if strings.Contains(string(data), "conflicts") {
		t.Error("expected conflicts to be omitted when empty")
	}
}

func TestScribeResponse_Conflicts_Present(t *testing.T) {
	// given
	resp := ScribeResponse{
		ADRID: "0003",
		Title: "test",
		Conflicts: []ADRConflict{
			{ExistingADRID: "0001", Description: "contradicts auth decision"},
		},
	}

	// when
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ScribeResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// then
	if len(decoded.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(decoded.Conflicts))
	}
	if decoded.Conflicts[0].ExistingADRID != "0001" {
		t.Errorf("expected 0001, got %s", decoded.Conflicts[0].ExistingADRID)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run "TestADRConflict|TestScribeResponse_Conflicts" -v`
Expected: FAIL — `ADRConflict` not defined

**Step 3: Write minimal implementation**

Add to `model.go`:

```go
// ADRConflict represents a detected contradiction between a new ADR and an existing one.
type ADRConflict struct {
	ExistingADRID string `json:"existing_adr_id"`
	Description   string `json:"description"`
}
```

Add `Conflicts` field to `ScribeResponse`:

```go
Conflicts []ADRConflict `json:"conflicts,omitempty"`
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run "TestADRConflict|TestScribeResponse_Conflicts" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add model.go model_test.go
git commit -m "feat(v0.6): add ADRConflict type and ScribeResponse.Conflicts"
```

---

### Task 5: Add StrictnessLevel to PromptData structs

**Files:**
- Modify: `prompt.go`
- Test: `prompt_test.go`

**Step 1: Write failing test**

Add to `prompt_test.go`:

```go
func TestRenderClassifyPrompt_ContainsStrictnessLevel(t *testing.T) {
	// given
	data := ClassifyPromptData{
		TeamFilter:      "TEST",
		ProjectFilter:   "Test",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "alert",
	}

	// when
	result, err := RenderClassifyPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "alert") {
		t.Error("expected strictness level 'alert' in prompt")
	}
}

func TestRenderWaveGeneratePrompt_ContainsStrictnessLevel(t *testing.T) {
	// given
	data := WaveGeneratePromptData{
		ClusterName:     "Auth",
		Completeness:    "25",
		Issues:          "[]",
		Observations:    "none",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "lockdown",
	}

	// when
	result, err := RenderWaveGeneratePrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "lockdown") {
		t.Error("expected strictness level 'lockdown' in prompt")
	}
}

func TestRenderScribeADRPrompt_ContainsStrictnessLevel(t *testing.T) {
	// given
	data := ScribeADRPromptData{
		ClusterName:     "Auth",
		WaveTitle:       "Test",
		WaveActions:     "[]",
		Analysis:        "test",
		Reasoning:       "test",
		ADRNumber:       "0001",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
	}

	// when
	result, err := RenderScribeADRPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "fog") {
		t.Error("expected strictness level 'fog' in prompt")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run "TestRender.*ContainsStrictnessLevel" -v`
Expected: FAIL — `StrictnessLevel` field not in PromptData structs

**Step 3: Write minimal implementation**

Add `StrictnessLevel string` field to each PromptData struct in `prompt.go`:

- `ClassifyPromptData` — add `StrictnessLevel string`
- `DeepScanPromptData` — add `StrictnessLevel string`
- `WaveGeneratePromptData` — add `StrictnessLevel string`
- `WaveApplyPromptData` — add `StrictnessLevel string`
- `ScribeADRPromptData` — add `StrictnessLevel string`
- `ArchitectDiscussPromptData` — add `StrictnessLevel string`

Then add a strictness section to each template file (12 files total — 6 templates × 2 languages).

For **English templates**, add after the existing header section:

```
## Strictness Level: {{.StrictnessLevel}}
Adjust your analysis depth based on the current strictness level:
- fog: Report DoD gaps as warnings only. NFR issues are informational.
- alert: Propose sub-issues for Must-level DoD gaps. NFR gets dedicated attention.
- lockdown: Flag ALL DoD gaps. Mark incomplete issues as blocked candidates.
```

For **Japanese templates**, add:

```
## Strictness Level: {{.StrictnessLevel}}
現在の厳格度レベルに応じて分析の深さを調整してください:
- fog: DoD不足をWarningとして表示のみ。NFRは参考情報。
- alert: Must級DoD欠落にはサブIssue提案。NFRにも専用の注意を。
- lockdown: 全DoD不足をフラグ。未完了IssueをBlocked候補としてマーク。
```

Add this block to all 12 template files (6 agents × 2 langs). Place it after the first agent identification line (e.g., after "You are a Scanner Agent.").

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run "TestRender.*ContainsStrictnessLevel" -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `go test ./... -v`
Expected: All PASS (existing tests still pass — StrictnessLevel defaults to "" which renders as empty but doesn't break templates)

**Step 6: Commit**

```bash
git add prompt.go prompt_test.go prompts/templates/
git commit -m "feat(v0.6): add StrictnessLevel field to all PromptData structs and templates"
```

---

### Task 6: Add Shibito resurrection section to scanner templates

**Files:**
- Modify: `prompts/templates/scanner_classify_en.md.tmpl`
- Modify: `prompts/templates/scanner_classify_ja.md.tmpl`
- Modify: `prompts/templates/scanner_deepscan_en.md.tmpl`
- Modify: `prompts/templates/scanner_deepscan_ja.md.tmpl`
- Test: `prompt_test.go`

**Step 1: Write failing test**

Add to `prompt_test.go`:

```go
func TestRenderClassifyPrompt_ContainsShibitoSection(t *testing.T) {
	// given
	data := ClassifyPromptData{
		TeamFilter:      "TEST",
		ProjectFilter:   "Test",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "alert",
	}

	// when
	result, err := RenderClassifyPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "shibito_warnings") {
		t.Error("expected shibito_warnings in scanner prompt output schema")
	}
}

func TestRenderClassifyPrompt_Japanese_ContainsShibitoSection(t *testing.T) {
	// given
	data := ClassifyPromptData{
		TeamFilter:      "TEST",
		ProjectFilter:   "Test",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
	}

	// when
	result, err := RenderClassifyPrompt("ja", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "shibito_warnings") {
		t.Error("expected shibito_warnings in Japanese scanner prompt")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run "TestRenderClassifyPrompt_ContainsShibito" -v`
Expected: FAIL — templates don't contain shibito_warnings yet

**Step 3: Write minimal implementation**

Add to **scanner_classify_en.md.tmpl** (before the Output section):

```
## Shibito Resurrection Check
Also examine closed/cancelled issues in this project for patterns that
resemble current open issues. If you detect potential "resurrection"
(a previously resolved problem re-emerging), include it in the output.
```

Update the Output JSON schema to include:

```
  "shibito_warnings": [
    { "closed_issue_id": "ENG-045", "current_issue_id": "ENG-102", "description": "...", "risk_level": "high" }
  ]
```

Do the same for the **ja** variant with Japanese instructions, and for the **deepscan** templates (en/ja).

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run "TestRenderClassifyPrompt_ContainsShibito" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add prompts/templates/scanner_*.tmpl prompt_test.go
git commit -m "feat(v0.6): add Shibito resurrection check to scanner templates"
```

---

### Task 7: Add ReadExistingADRs to scribe.go

**Files:**
- Modify: `scribe.go`
- Test: `scribe_test.go`

**Step 1: Write failing tests**

Add to `scribe_test.go`:

```go
func TestReadExistingADRs_Empty(t *testing.T) {
	// given
	dir := t.TempDir()

	// when
	adrs, err := ReadExistingADRs(dir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(adrs) != 0 {
		t.Errorf("expected 0 ADRs, got %d", len(adrs))
	}
}

func TestReadExistingADRs_ReturnsContent(t *testing.T) {
	// given
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "0001-auth-decision.md"), []byte("# 0001. Auth Decision\nAccepted"), 0644)
	os.WriteFile(filepath.Join(dir, "0002-api-design.md"), []byte("# 0002. API Design\nAccepted"), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("ignore this"), 0644) // non-ADR file

	// when
	adrs, err := ReadExistingADRs(dir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(adrs) != 2 {
		t.Fatalf("expected 2 ADRs, got %d", len(adrs))
	}
	if adrs[0].Filename != "0001-auth-decision.md" {
		t.Errorf("expected 0001-auth-decision.md, got %s", adrs[0].Filename)
	}
	if !strings.Contains(adrs[0].Content, "Auth Decision") {
		t.Error("expected ADR content to contain 'Auth Decision'")
	}
}

func TestReadExistingADRs_DirNotExist(t *testing.T) {
	// when
	adrs, err := ReadExistingADRs("/nonexistent/dir")

	// then
	if err != nil {
		t.Fatalf("unexpected error for missing dir: %v", err)
	}
	if len(adrs) != 0 {
		t.Errorf("expected 0 ADRs, got %d", len(adrs))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run TestReadExistingADRs -v`
Expected: FAIL — `ReadExistingADRs` not defined

**Step 3: Write minimal implementation**

Add to `scribe.go`:

```go
// ExistingADR holds the filename and content of an existing ADR file.
type ExistingADR struct {
	Filename string
	Content  string
}

// ReadExistingADRs reads all NNNN-*.md files from adrDir and returns their content.
// Returns empty slice if directory doesn't exist or is empty.
func ReadExistingADRs(adrDir string) ([]ExistingADR, error) {
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var adrs []ExistingADR
	for _, e := range entries {
		if e.IsDir() || !adrPattern.MatchString(e.Name()) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(adrDir, e.Name()))
		if err != nil {
			continue // skip unreadable files
		}
		adrs = append(adrs, ExistingADR{
			Filename: e.Name(),
			Content:  string(content),
		})
	}
	return adrs, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run TestReadExistingADRs -v`
Expected: PASS

**Step 5: Commit**

```bash
git add scribe.go scribe_test.go
git commit -m "feat(v0.6): add ReadExistingADRs for ADR consistency check"
```

---

### Task 8: Add ExistingADRs to ScribeADRPromptData and update template

**Files:**
- Modify: `prompt.go`
- Modify: `prompts/templates/scribe_adr_en.md.tmpl`
- Modify: `prompts/templates/scribe_adr_ja.md.tmpl`
- Test: `prompt_test.go`

**Step 1: Write failing test**

Add to `prompt_test.go`:

```go
func TestRenderScribeADRPrompt_ContainsExistingADRs(t *testing.T) {
	// given
	data := ScribeADRPromptData{
		ClusterName:     "Auth",
		WaveTitle:       "Test",
		WaveActions:     "[]",
		Analysis:        "test",
		Reasoning:       "test",
		ADRNumber:       "0003",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "alert",
		ExistingADRs: []ExistingADR{
			{Filename: "0001-auth.md", Content: "# 0001. Auth\nAccepted"},
			{Filename: "0002-api.md", Content: "# 0002. API\nAccepted"},
		},
	}

	// when
	result, err := RenderScribeADRPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if !strings.Contains(result, "0001-auth.md") {
		t.Error("expected existing ADR filename in prompt")
	}
	if !strings.Contains(result, "# 0001. Auth") {
		t.Error("expected existing ADR content in prompt")
	}
	if !strings.Contains(result, "conflicts") {
		t.Error("expected conflicts field in output schema")
	}
}

func TestRenderScribeADRPrompt_NoExistingADRs(t *testing.T) {
	// given
	data := ScribeADRPromptData{
		ClusterName:     "Auth",
		WaveTitle:       "Test",
		WaveActions:     "[]",
		Analysis:        "test",
		Reasoning:       "test",
		ADRNumber:       "0001",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
	}

	// when
	result, err := RenderScribeADRPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	// Should render without errors even with no existing ADRs
	if !strings.Contains(result, "Scribe Agent") {
		t.Error("expected Scribe Agent in prompt")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run "TestRenderScribeADRPrompt_ContainsExistingADRs|TestRenderScribeADRPrompt_NoExistingADRs" -v`
Expected: FAIL — `ExistingADRs` field not in `ScribeADRPromptData`

**Step 3: Write minimal implementation**

Add to `ScribeADRPromptData` in `prompt.go`:

```go
ExistingADRs []ExistingADR
```

Update `scribe_adr_en.md.tmpl` — add before the Output section:

```
## ADR Consistency Check
Review existing ADRs below for potential contradictions with the new decision.
Report any conflicts found in the "conflicts" array of your output.
{{- if .ExistingADRs}}
{{range .ExistingADRs}}
### {{.Filename}}
{{.Content}}
{{end}}
{{- else}}
No existing ADRs found.
{{- end}}
```

Update the Output JSON schema to include:

```
  "conflicts": [
    { "existing_adr_id": "NNNN", "description": "..." }
  ]
```

Do the same for `scribe_adr_ja.md.tmpl` with Japanese instructions.

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run "TestRenderScribeADRPrompt_ContainsExistingADRs|TestRenderScribeADRPrompt_NoExistingADRs" -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `go test ./... -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add prompt.go prompt_test.go prompts/templates/scribe_adr_*.tmpl
git commit -m "feat(v0.6): add ExistingADRs to ScribeADRPromptData and ADR consistency template"
```

---

### Task 9: Wire ExistingADRs into RunScribeADR and RunScribeADRDryRun

**Files:**
- Modify: `scribe.go`
- Test: `scribe_test.go`

**Step 1: Write failing test**

Add to `scribe_test.go`:

```go
func TestRunScribeADRDryRun_IncludesExistingADRs(t *testing.T) {
	// given
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "scans")
	os.MkdirAll(scanDir, 0755)
	adrDir := filepath.Join(dir, "docs", "adr")
	os.MkdirAll(adrDir, 0755)
	os.WriteFile(filepath.Join(adrDir, "0001-auth.md"), []byte("# Auth ADR"), 0644)

	cfg := &Config{
		Lang: "en",
		Claude: ClaudeConfig{Command: "echo", TimeoutSec: 10},
	}
	wave := Wave{ID: "w1", ClusterName: "Auth", Title: "Test"}
	resp := &ArchitectResponse{Analysis: "test", Reasoning: "test"}

	// when
	err := RunScribeADRDryRun(cfg, scanDir, wave, resp, adrDir)

	// then
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	// Verify prompt file was created and contains existing ADR reference
	promptFiles, _ := filepath.Glob(filepath.Join(scanDir, "scribe_*_prompt.md"))
	if len(promptFiles) == 0 {
		t.Fatal("expected scribe prompt file to be created")
	}
	content, _ := os.ReadFile(promptFiles[0])
	if !strings.Contains(string(content), "0001-auth.md") {
		t.Error("expected existing ADR filename in dry-run prompt")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run TestRunScribeADRDryRun_IncludesExistingADRs -v`
Expected: FAIL — prompt doesn't include existing ADRs yet

**Step 3: Write minimal implementation**

Modify `RunScribeADRDryRun` in `scribe.go` to read existing ADRs and pass them to `RenderScribeADRPrompt`:

```go
func RunScribeADRDryRun(cfg *Config, scanDir string, wave Wave, architectResp *ArchitectResponse, adrDir string) error {
	adrNum, err := NextADRNumber(adrDir)
	if err != nil {
		return fmt.Errorf("next adr number: %w", err)
	}

	actionsJSON, err := json.Marshal(wave.Actions)
	if err != nil {
		return fmt.Errorf("marshal wave actions: %w", err)
	}

	existingADRs, err := ReadExistingADRs(adrDir)
	if err != nil {
		return fmt.Errorf("read existing ADRs: %w", err)
	}

	adrID := fmt.Sprintf("%04d", adrNum)
	outputFile := filepath.Join(scanDir, scribeFileName(wave))
	prompt, err := RenderScribeADRPrompt(cfg.Lang, ScribeADRPromptData{
		ClusterName:     wave.ClusterName,
		WaveTitle:       wave.Title,
		WaveActions:     string(actionsJSON),
		Analysis:        architectResp.Analysis,
		Reasoning:       architectResp.Reasoning,
		ADRNumber:       adrID,
		OutputPath:      outputFile,
		StrictnessLevel: string(cfg.Strictness.Default),
		ExistingADRs:    existingADRs,
	})
	if err != nil {
		return fmt.Errorf("render scribe prompt: %w", err)
	}

	dryRunName := fmt.Sprintf("scribe_%s_%s", sanitizeName(wave.ClusterName), sanitizeName(wave.ID))
	return RunClaudeDryRun(cfg, prompt, scanDir, dryRunName)
}
```

Apply the same pattern to `RunScribeADR` — add `ReadExistingADRs` call and pass `ExistingADRs` and `StrictnessLevel` to the prompt data.

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run TestRunScribeADRDryRun_IncludesExistingADRs -v`
Expected: PASS

**Step 5: Commit**

```bash
git add scribe.go scribe_test.go
git commit -m "feat(v0.6): wire ExistingADRs into RunScribeADR and dry-run"
```

---

### Task 10: Display functions — DisplayShibitoWarnings and DisplayADRConflicts

**Files:**
- Modify: `cli.go`
- Test: `cli_test.go`

**Step 1: Write failing tests**

Add to `cli_test.go`:

```go
func TestDisplayShibitoWarnings_NoWarnings(t *testing.T) {
	// given
	var buf bytes.Buffer

	// when
	DisplayShibitoWarnings(&buf, nil)

	// then
	if buf.Len() != 0 {
		t.Error("expected no output for nil warnings")
	}
}

func TestDisplayShibitoWarnings_WithWarnings(t *testing.T) {
	// given
	var buf bytes.Buffer
	warnings := []ShibitoWarning{
		{ClosedIssueID: "ENG-045", CurrentIssueID: "ENG-102", Description: "Token loop", RiskLevel: "high"},
	}

	// when
	DisplayShibitoWarnings(&buf, warnings)

	// then
	output := buf.String()
	if !strings.Contains(output, "Shibito") {
		t.Error("expected Shibito header in output")
	}
	if !strings.Contains(output, "ENG-045") {
		t.Error("expected closed issue ID in output")
	}
	if !strings.Contains(output, "ENG-102") {
		t.Error("expected current issue ID in output")
	}
	if !strings.Contains(output, "high") {
		t.Error("expected risk level in output")
	}
}

func TestDisplayADRConflicts_NoConflicts(t *testing.T) {
	// given
	var buf bytes.Buffer

	// when
	DisplayADRConflicts(&buf, nil)

	// then
	if buf.Len() != 0 {
		t.Error("expected no output for nil conflicts")
	}
}

func TestDisplayADRConflicts_WithConflicts(t *testing.T) {
	// given
	var buf bytes.Buffer
	conflicts := []ADRConflict{
		{ExistingADRID: "0002", Description: "Contradicts session storage decision"},
	}

	// when
	DisplayADRConflicts(&buf, conflicts)

	// then
	output := buf.String()
	if !strings.Contains(output, "Scribe") {
		t.Error("expected Scribe prefix in output")
	}
	if !strings.Contains(output, "0002") {
		t.Error("expected existing ADR ID in output")
	}
	if !strings.Contains(output, "Contradicts") {
		t.Error("expected conflict description in output")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run "TestDisplayShibitoWarnings|TestDisplayADRConflicts" -v`
Expected: FAIL — functions not defined

**Step 3: Write minimal implementation**

Add to `cli.go`:

```go
// DisplayShibitoWarnings shows Shibito resurrection warnings after a scan.
func DisplayShibitoWarnings(w io.Writer, warnings []ShibitoWarning) {
	if len(warnings) == 0 {
		return
	}
	fmt.Fprintf(w, "\n  [Shibito] %d resurrection warning(s):\n", len(warnings))
	for _, warn := range warnings {
		fmt.Fprintf(w, "    %s -> %s [%s]: %s\n",
			warn.ClosedIssueID, warn.CurrentIssueID, warn.RiskLevel, warn.Description)
	}
}

// DisplayADRConflicts shows ADR contradiction warnings after Scribe generates an ADR.
func DisplayADRConflicts(w io.Writer, conflicts []ADRConflict) {
	if len(conflicts) == 0 {
		return
	}
	for _, c := range conflicts {
		fmt.Fprintf(w, "  [Scribe] Warning: Potential conflict with ADR-%s: %s\n",
			c.ExistingADRID, c.Description)
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run "TestDisplayShibitoWarnings|TestDisplayADRConflicts" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cli.go cli_test.go
git commit -m "feat(v0.6): add DisplayShibitoWarnings and DisplayADRConflicts"
```

---

### Task 11: Navigator strictness badge and shibito count

**Files:**
- Modify: `navigator.go`
- Test: `navigator_test.go`

**Step 1: Write failing test**

Add to `navigator_test.go`:

```go
func TestRenderNavigatorWithWaves_StrictnessBadge(t *testing.T) {
	// given
	result := &ScanResult{
		Completeness: 0.5,
		Clusters:     []ClusterScanResult{{Name: "Auth", Completeness: 0.5}},
	}
	waves := []Wave{{ID: "w1", ClusterName: "Auth", Title: "W1", Status: "available"}}

	// when
	output := RenderNavigatorWithWaves(result, "TestProject", waves, 0, nil, "alert", 0)

	// then
	if !strings.Contains(output, "alert") {
		t.Error("expected strictness level badge in navigator")
	}
}

func TestRenderNavigatorWithWaves_ShibitoCount(t *testing.T) {
	// given
	result := &ScanResult{
		Completeness: 0.5,
		Clusters:     []ClusterScanResult{{Name: "Auth", Completeness: 0.5}},
	}
	waves := []Wave{{ID: "w1", ClusterName: "Auth", Title: "W1", Status: "available"}}

	// when
	output := RenderNavigatorWithWaves(result, "TestProject", waves, 0, nil, "fog", 3)

	// then
	if !strings.Contains(output, "Shibito: 3") {
		t.Error("expected shibito count in navigator")
	}
}

func TestRenderNavigatorWithWaves_ShibitoZero_Hidden(t *testing.T) {
	// given
	result := &ScanResult{
		Completeness: 0.5,
		Clusters:     []ClusterScanResult{{Name: "Auth", Completeness: 0.5}},
	}
	waves := []Wave{{ID: "w1", ClusterName: "Auth", Title: "W1", Status: "available"}}

	// when
	output := RenderNavigatorWithWaves(result, "TestProject", waves, 0, nil, "fog", 0)

	// then
	if strings.Contains(output, "Shibito") {
		t.Error("expected shibito count to be hidden when zero")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run "TestRenderNavigatorWithWaves_Strictness|TestRenderNavigatorWithWaves_Shibito" -v`
Expected: FAIL — function signature doesn't have strictnessLevel/shibitoCount params

**Step 3: Write minimal implementation**

Update `RenderNavigatorWithWaves` signature in `navigator.go`:

```go
func RenderNavigatorWithWaves(result *ScanResult, projectName string, waves []Wave, adrCount int, lastScanned *time.Time, strictnessLevel string, shibitoCount int) string {
```

Add after the ADR row:

```go
if strictnessLevel != "" {
    levelRow := fmt.Sprintf("  Level: %s", strictnessLevel)
    b.WriteString("|" + padRight(levelRow, navigatorWidth) + "|\n")
}
if shibitoCount > 0 {
    shibRow := fmt.Sprintf("  Shibito: %d warning(s)", shibitoCount)
    b.WriteString("|" + padRight(shibRow, navigatorWidth) + "|\n")
}
```

**IMPORTANT:** Update ALL call sites of `RenderNavigatorWithWaves`:
- `session.go:111` — add `string(cfg.Strictness.Default)` and `len(scanResult.ShibitoWarnings)` params
- Any other call sites found via `go build ./...`

**Step 4: Run tests to verify they pass**

Run: `go test ./... -v`
Expected: All PASS (including updated call sites)

**Step 5: Commit**

```bash
git add navigator.go navigator_test.go session.go
git commit -m "feat(v0.6): add strictness badge and shibito count to Navigator"
```

---

### Task 12: Wire strictness level into session.go prompt calls

**Files:**
- Modify: `session.go`
- Test: `session_test.go`

This task passes `cfg.Strictness.Default` to all prompt render calls in session.go. The exact changes depend on where prompts are rendered — most rendering happens inside `RunScan`, `RunWaveGenerate`, `RunWaveApply`, and `RunArchitectDiscuss`.

**Step 1: Identify all prompt render call sites**

Check: `scanner.go` (`RunScan` calls `RenderClassifyPrompt`, `RenderDeepScanPrompt`), `wave.go` (`RunWaveGenerate` calls `RenderWaveGeneratePrompt`; `RunWaveApply` calls `RenderWaveApplyPrompt`), `architect.go` (`RunArchitectDiscuss` calls `RenderArchitectDiscussPrompt`).

These functions need `StrictnessLevel` added to their signatures or passed through `Config`.

**Step 2: Write failing test**

Add to `session_test.go` a test that verifies the session dry-run path passes strictness through:

```go
func TestRunSession_DryRun_StrictnessInPrompts(t *testing.T) {
	// given
	dir := t.TempDir()
	cfg := &Config{
		Linear: LinearConfig{Team: "T", Project: "P"},
		Scan:   ScanConfig{ChunkSize: 20, MaxConcurrency: 1},
		Claude: ClaudeConfig{Command: "echo", TimeoutSec: 10},
		Scribe: ScribeConfig{Enabled: true},
		Strictness: StrictnessConfig{Default: StrictnessAlert},
		Lang:   "en",
	}

	// when
	err := RunSession(context.Background(), cfg, dir, "test-strict", true, nil)

	// then
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	// Check that generated prompt files contain "alert"
	prompts, _ := filepath.Glob(filepath.Join(dir, ".siren", "scans", "test-strict", "*_prompt.md"))
	foundStrictness := false
	for _, p := range prompts {
		content, _ := os.ReadFile(p)
		if strings.Contains(string(content), "alert") {
			foundStrictness = true
			break
		}
	}
	if !foundStrictness {
		t.Error("expected strictness level 'alert' in at least one prompt file")
	}
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./... -run TestRunSession_DryRun_StrictnessInPrompts -v`
Expected: FAIL — strictness not passed to prompt rendering

**Step 4: Wire strictness through**

The cleanest approach: since `Config` is already passed to `RunScan`, `RunWaveGenerate`, etc., and `Config` now has `Strictness.Default`, update the internal prompt-rendering calls within:

- `scanner.go`: `RunScan` → `RenderClassifyPrompt` and `RenderDeepScanPrompt` — add `StrictnessLevel: string(cfg.Strictness.Default)` to the PromptData
- `wave.go`: `RunWaveGenerate` → `RenderWaveGeneratePrompt` — add `StrictnessLevel: string(cfg.Strictness.Default)`
- `wave.go`: `RunWaveApply` → `RenderWaveApplyPrompt` — add `StrictnessLevel: string(cfg.Strictness.Default)`
- `architect.go`: `RunArchitectDiscuss` → `RenderArchitectDiscussPrompt` — add `StrictnessLevel: string(cfg.Strictness.Default)`
- `scribe.go`: Already done in Task 9

**Step 5: Run test to verify it passes**

Run: `go test ./... -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add scanner.go wave.go architect.go session_test.go
git commit -m "feat(v0.6): wire strictness level into all prompt render calls"
```

---

### Task 13: Display shibito warnings and ADR conflicts in session flow

**Files:**
- Modify: `session.go`

**Step 1: Add shibito warning display after scan**

In `RunSession`, after `RunScan` and before the interactive loop, display any shibito warnings:

```go
// Display Shibito warnings from scan
if len(scanResult.ShibitoWarnings) > 0 {
    DisplayShibitoWarnings(os.Stdout, scanResult.ShibitoWarnings)
}
```

**Step 2: Add ADR conflict display after Scribe**

In `runInteractiveLoop`, after `DisplayScribeResponse`, add:

```go
if len(scribeResp.Conflicts) > 0 {
    DisplayADRConflicts(os.Stdout, scribeResp.Conflicts)
}
```

**Step 3: Run full test suite**

Run: `go test ./... -v`
Expected: All PASS

**Step 4: Commit**

```bash
git add session.go
git commit -m "feat(v0.6): display shibito warnings and ADR conflicts in session flow"
```

---

### Task 14: Add ShibitoCount to SessionState

**Files:**
- Modify: `model.go`
- Modify: `session.go`
- Test: `state_test.go`

**Step 1: Write failing test**

Add to `state_test.go`:

```go
func TestSessionState_ShibitoCount_RoundTrip(t *testing.T) {
	// given
	dir := t.TempDir()
	state := &SessionState{
		Version:      "0.6",
		SessionID:    "test-shibito",
		ShibitoCount: 3,
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
	if loaded.ShibitoCount != 3 {
		t.Errorf("expected ShibitoCount 3, got %d", loaded.ShibitoCount)
	}
}

func TestSessionState_ShibitoCount_OmittedWhenZero(t *testing.T) {
	// given
	state := SessionState{Version: "0.6", ShibitoCount: 0}

	// when
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// then
	if strings.Contains(string(data), "shibito_count") {
		t.Error("expected shibito_count to be omitted when zero")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run TestSessionState_ShibitoCount -v`
Expected: FAIL — `ShibitoCount` field not in `SessionState`

**Step 3: Write minimal implementation**

Add to `SessionState` in `model.go`:

```go
ShibitoCount int `json:"shibito_count,omitempty"`
```

Update `BuildSessionState` in `session.go` to populate it:

```go
state.ShibitoCount = len(scanResult.ShibitoWarnings)
```

This requires passing `scanResult` to `BuildSessionState` (it already receives it). Add the line after `state.ADRCount = adrCount`.

**Step 4: Run tests to verify they pass**

Run: `go test ./... -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add model.go session.go state_test.go
git commit -m "feat(v0.6): add ShibitoCount to SessionState"
```

---

### Task 15: Update SessionState version and bump CLI version

**Files:**
- Modify: `session.go` — Change `Version: "0.5"` to `Version: "0.6"` in `BuildSessionState`
- Modify: `cmd/sightjack/main.go` — Change `version = "0.5.0-dev"` to `version = "0.6.0-dev"`

**Step 1: Make changes**

In `session.go:342`, change:
```go
Version: "0.5",
```
to:
```go
Version: "0.6",
```

In `cmd/sightjack/main.go:18`, change:
```go
var version = "0.5.0-dev"
```
to:
```go
var version = "0.6.0-dev"
```

**Step 2: Run full test suite**

Run: `go test ./... -v`
Expected: All PASS (update any tests that assert Version == "0.5")

**Step 3: Build verification**

Run: `go build ./... && go vet ./...`
Expected: Clean

**Step 4: Commit**

```bash
git add session.go cmd/sightjack/main.go
git commit -m "feat(v0.6): bump version to 0.6.0-dev"
```

---

## Verification Checklist

1. `go build ./...` — compiles clean
2. `go test ./... -v` — all tests pass
3. `go vet ./...` — no issues
4. Dry-run: `go run ./cmd/sightjack --dry-run` — generates prompts with strictness level
5. Navigator shows: Level badge, Shibito count (when > 0), ADR count
6. SessionState JSON contains `shibito_count` field
7. Scribe prompts contain existing ADR content for consistency check
8. All scanner prompts contain shibito resurrection check instructions
