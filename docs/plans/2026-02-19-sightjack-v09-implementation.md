# Sightjack v0.9 Implementation Plan: Production Ready + Full Integration

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Bring Sightjack to production-ready quality by implementing all remaining deferred features (DoD Templates, Error Handling/Retry, Completeness Tracking, Linear Labels, Paintress Bridge, State Recovery, Parallel Scan).

**Architecture:** Seven features wired into the existing session/scanner/prompt pipeline. Config types extend `Config` in config.go, prompt data structs extend prompt.go, templates follow the `{agent}_{action}_{lang}.md.tmpl` convention. All agent calls remain subprocess-based via RunClaude. Labels applied via Claude subprocess + Linear MCP (no direct API calls).

**Tech Stack:** Go 1.22+, `text/template` (embedded FS), `gopkg.in/yaml.v3`, `os/exec` (Claude subprocess), `sync` (semaphore)

---

## Dependencies

| Feature | Depends on |
|---------|-----------|
| A: DoD Templates | — |
| B: Error Handling | — |
| C: Completeness | — |
| D: Linear Labels | — |
| E: Paintress Bridge | D |
| F: State Recovery | — |
| G: Parallel Scan | — |

Recommended execution order: A → B → C → D → E → F → G (respects E→D dependency, groups config changes early).

---

## Task 1: Add DoDTemplate type + DoDTemplates config field

**Feature:** A (DoD Templates)

**Files:**
- Modify: `config.go`
- Modify: `config_test.go`

**Step 1: Write failing test**

In `config_test.go`, add:

```go
func TestDoDTemplatesInDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.DoDTemplates != nil {
		t.Fatalf("expected nil DoDTemplates in default config, got %v", cfg.DoDTemplates)
	}
}

func TestLoadConfigWithDoDTemplates(t *testing.T) {
	content := `
linear:
  team: test
  project: test
dod_templates:
  auth:
    must:
      - "Unit tests for all public functions"
      - "Error handling for all API calls"
    should:
      - "Integration test coverage"
  infra:
    must:
      - "Terraform plan reviewed"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.DoDTemplates) != 2 {
		t.Fatalf("expected 2 DoD templates, got %d", len(cfg.DoDTemplates))
	}
	auth := cfg.DoDTemplates["auth"]
	if len(auth.Must) != 2 {
		t.Errorf("auth.Must: expected 2, got %d", len(auth.Must))
	}
	if len(auth.Should) != 1 {
		t.Errorf("auth.Should: expected 1, got %d", len(auth.Should))
	}
	infra := cfg.DoDTemplates["infra"]
	if len(infra.Must) != 1 {
		t.Errorf("infra.Must: expected 1, got %d", len(infra.Must))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestDoDTemplates -v`
Expected: FAIL — `DoDTemplates` field not found

**Step 3: Write minimal implementation**

In `config.go`, add type and field:

```go
// DoDTemplate holds must/should Definition of Done items for a category.
type DoDTemplate struct {
	Must   []string `yaml:"must"`
	Should []string `yaml:"should"`
}
```

Add field to `Config` struct:

```go
DoDTemplates map[string]DoDTemplate `yaml:"dod_templates"`
```

No changes to `DefaultConfig()` (nil map = no templates, AI decides freely).
No changes to `LoadConfig()` validation (nil is valid).

**Step 4: Run test to verify it passes**

Run: `go test ./... -run TestDoDTemplates -v`
Expected: PASS

**Step 5: Commit**

```bash
git add config.go config_test.go
git commit -m "feat(v0.9/A): add DoDTemplate type and DoDTemplates config field"
```

---

## Task 2: Add DoDTemplates to wave prompt data + category matching

**Feature:** A (DoD Templates)

**Files:**
- Modify: `prompt.go`
- Modify: `prompt_test.go`

**Step 1: Write failing test**

In `prompt_test.go`, add:

```go
func TestMatchDoDTemplate(t *testing.T) {
	templates := map[string]DoDTemplate{
		"auth":  {Must: []string{"auth must"}, Should: []string{"auth should"}},
		"infra": {Must: []string{"infra must"}},
	}
	tests := []struct {
		clusterName string
		wantMatch   bool
		wantKey     string
	}{
		{"Auth", true, "auth"},
		{"auth-service", true, "auth"},
		{"Authentication", true, "auth"},
		{"INFRA", true, "infra"},
		{"frontend", false, ""},
	}
	for _, tt := range tests {
		matched, key := MatchDoDTemplate(templates, tt.clusterName)
		if matched != tt.wantMatch {
			t.Errorf("MatchDoDTemplate(%q): matched=%v, want %v", tt.clusterName, matched, tt.wantMatch)
		}
		if key != tt.wantKey {
			t.Errorf("MatchDoDTemplate(%q): key=%q, want %q", tt.clusterName, key, tt.wantKey)
		}
	}
}

func TestFormatDoDSection(t *testing.T) {
	tmpl := DoDTemplate{
		Must:   []string{"Unit tests", "Error handling"},
		Should: []string{"Integration tests"},
	}
	section := FormatDoDSection(tmpl)
	if !strings.Contains(section, "Unit tests") {
		t.Error("expected Must items in section")
	}
	if !strings.Contains(section, "Integration tests") {
		t.Error("expected Should items in section")
	}
}

func TestFormatDoDSectionEmpty(t *testing.T) {
	section := FormatDoDSection(DoDTemplate{})
	if section != "" {
		t.Errorf("expected empty section for empty template, got %q", section)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestMatchDoD -v && go test ./... -run TestFormatDoD -v`
Expected: FAIL — functions not found

**Step 3: Write minimal implementation**

In `prompt.go`, add:

```go
// MatchDoDTemplate finds a DoD template matching the cluster name.
// Matching uses case-insensitive prefix match. Returns (matched, key).
func MatchDoDTemplate(templates map[string]DoDTemplate, clusterName string) (bool, string) {
	lower := strings.ToLower(clusterName)
	for key := range templates {
		if strings.HasPrefix(lower, strings.ToLower(key)) {
			return true, key
		}
	}
	return false, ""
}

// FormatDoDSection formats a DoD template into a text section for prompt injection.
func FormatDoDSection(tmpl DoDTemplate) string {
	if len(tmpl.Must) == 0 && len(tmpl.Should) == 0 {
		return ""
	}
	var b strings.Builder
	if len(tmpl.Must) > 0 {
		b.WriteString("Must:\n")
		for _, item := range tmpl.Must {
			b.WriteString("- " + item + "\n")
		}
	}
	if len(tmpl.Should) > 0 {
		b.WriteString("Should:\n")
		for _, item := range tmpl.Should {
			b.WriteString("- " + item + "\n")
		}
	}
	return b.String()
}
```

Add `DoDSection` field to `WaveGeneratePromptData` and `NextGenPromptData`:

```go
type WaveGeneratePromptData struct {
	// ... existing fields ...
	DoDSection string
}

type NextGenPromptData struct {
	// ... existing fields ...
	DoDSection string
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add prompt.go prompt_test.go
git commit -m "feat(v0.9/A): add DoD template matching and prompt data fields"
```

---

## Task 3: Inject DoD into wave_generate + wave_nextgen templates

**Feature:** A (DoD Templates)

**Files:**
- Modify: `prompts/templates/wave_generate_en.md.tmpl`
- Modify: `prompts/templates/wave_generate_ja.md.tmpl`
- Modify: `prompts/templates/wave_nextgen_en.md.tmpl`
- Modify: `prompts/templates/wave_nextgen_ja.md.tmpl`
- Modify: `scanner.go` (wire DoDSection into WaveGeneratePromptData)
- Modify: `wave_generator.go` (wire DoDSection into NextGenPromptData)
- Modify: `prompt_test.go`

**Step 1: Write failing test**

In `prompt_test.go`, add:

```go
func TestRenderWaveGeneratePromptWithDoD(t *testing.T) {
	data := WaveGeneratePromptData{
		ClusterName:     "auth",
		Completeness:    "50",
		Issues:          "[]",
		Observations:    "none",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
		DoDSection:      "Must:\n- Unit tests\nShould:\n- Integration tests\n",
	}
	for _, lang := range []string{"en", "ja"} {
		result, err := RenderWaveGeneratePrompt(lang, data)
		if err != nil {
			t.Fatalf("RenderWaveGeneratePrompt(%s): %v", lang, err)
		}
		if !strings.Contains(result, "Unit tests") {
			t.Errorf("lang=%s: expected DoD section in output", lang)
		}
	}
}

func TestRenderNextGenPromptWithDoD(t *testing.T) {
	data := NextGenPromptData{
		ClusterName:     "auth",
		Completeness:    "70",
		Issues:          "[]",
		CompletedWaves:  "[]",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
		DoDSection:      "Must:\n- Terraform reviewed\n",
	}
	for _, lang := range []string{"en", "ja"} {
		result, err := RenderNextGenPrompt(lang, data)
		if err != nil {
			t.Fatalf("RenderNextGenPrompt(%s): %v", lang, err)
		}
		if !strings.Contains(result, "Terraform reviewed") {
			t.Errorf("lang=%s: expected DoD section in output", lang)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestRenderWaveGeneratePromptWithDoD -v && go test ./... -run TestRenderNextGenPromptWithDoD -v`
Expected: FAIL — DoD section not rendered by templates

**Step 3: Write minimal implementation**

Add to all four template files, after the existing content and before the output instruction:

For `wave_generate_en.md.tmpl` and `wave_generate_ja.md.tmpl`:
```
{{if .DoDSection}}
## Definition of Done Guidelines

The following DoD standards apply to this cluster:

{{.DoDSection}}
Ensure proposed wave actions align with these standards.
{{end}}
```

For `wave_nextgen_en.md.tmpl` and `wave_nextgen_ja.md.tmpl`:
```
{{if .DoDSection}}
## Definition of Done Guidelines

{{.DoDSection}}
Ensure new wave proposals align with these standards.
{{end}}
```

Wire DoD into `scanner.go` `RunWaveGenerate` — compute `DoDSection` from `cfg.DoDTemplates` before building prompt data:
```go
var dodSection string
if cfg.DoDTemplates != nil {
	if matched, key := MatchDoDTemplate(cfg.DoDTemplates, cluster.Name); matched {
		dodSection = FormatDoDSection(cfg.DoDTemplates[key])
	}
}
// ... then set DoDSection: dodSection in WaveGeneratePromptData
```

Wire DoD into `wave_generator.go` `buildNextGenPrompt` — same pattern with `cfg.DoDTemplates`.

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add prompts/templates/ scanner.go wave_generator.go prompt.go prompt_test.go
git commit -m "feat(v0.9/A): inject DoD templates into wave generation prompts"
```

---

## Task 4: Add RetryConfig type + config field

**Feature:** B (Error Handling / Retry)

**Files:**
- Modify: `config.go`
- Modify: `config_test.go`

**Step 1: Write failing test**

In `config_test.go`, add:

```go
func TestRetryConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Retry.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", cfg.Retry.MaxAttempts)
	}
	if cfg.Retry.BaseDelaySec != 2 {
		t.Errorf("expected BaseDelaySec=2, got %d", cfg.Retry.BaseDelaySec)
	}
}

func TestLoadConfigWithRetry(t *testing.T) {
	content := `
linear:
  team: test
  project: test
retry:
  max_attempts: 5
  base_delay_sec: 1
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Retry.MaxAttempts != 5 {
		t.Errorf("MaxAttempts: expected 5, got %d", cfg.Retry.MaxAttempts)
	}
	if cfg.Retry.BaseDelaySec != 1 {
		t.Errorf("BaseDelaySec: expected 1, got %d", cfg.Retry.BaseDelaySec)
	}
}

func TestLoadConfigRetryValidation(t *testing.T) {
	content := `
linear:
  team: test
  project: test
retry:
  max_attempts: 0
  base_delay_sec: -1
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Retry.MaxAttempts != 3 {
		t.Errorf("expected corrected MaxAttempts=3, got %d", cfg.Retry.MaxAttempts)
	}
	if cfg.Retry.BaseDelaySec != 2 {
		t.Errorf("expected corrected BaseDelaySec=2, got %d", cfg.Retry.BaseDelaySec)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestRetryConfig -v && go test ./... -run TestLoadConfigRetry -v`
Expected: FAIL

**Step 3: Write minimal implementation**

In `config.go`, add type:

```go
// RetryConfig holds exponential backoff retry settings for Claude subprocess calls.
type RetryConfig struct {
	MaxAttempts  int `yaml:"max_attempts"`
	BaseDelaySec int `yaml:"base_delay_sec"`
}
```

Add field to `Config`:
```go
Retry RetryConfig `yaml:"retry"`
```

Add default in `DefaultConfig()`:
```go
Retry: RetryConfig{
	MaxAttempts:  3,
	BaseDelaySec: 2,
},
```

Add validation in `LoadConfig()`:
```go
if cfg.Retry.MaxAttempts < 1 {
	cfg.Retry.MaxAttempts = 3
}
if cfg.Retry.BaseDelaySec < 1 {
	cfg.Retry.BaseDelaySec = 2
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add config.go config_test.go
git commit -m "feat(v0.9/B): add RetryConfig type and config field"
```

---

## Task 5: Implement exponential backoff retry in RunClaude

**Feature:** B (Error Handling / Retry)

**Files:**
- Modify: `claude.go`
- Modify: `claude_test.go`

**Step 1: Write failing test**

In `claude_test.go`, add:

```go
func TestRunClaudeRetriesOnFailure(t *testing.T) {
	callCount := 0
	newCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if callCount < 3 {
			return exec.Command("false") // exits non-zero
		}
		return exec.Command("echo", "success")
	}
	defer func() { newCmd = defaultNewCmd }()

	cfg := &Config{
		Claude: ClaudeConfig{Command: "claude", TimeoutSec: 10},
		Retry:  RetryConfig{MaxAttempts: 3, BaseDelaySec: 0}, // 0 for fast test
	}
	output, err := RunClaude(context.Background(), cfg, "test", io.Discard)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if !strings.Contains(output, "success") {
		t.Errorf("expected 'success' in output, got %q", output)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestRunClaudeNoRetryOnCancel(t *testing.T) {
	callCount := 0
	newCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		return exec.Command("false")
	}
	defer func() { newCmd = defaultNewCmd }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	cfg := &Config{
		Claude: ClaudeConfig{Command: "claude", TimeoutSec: 10},
		Retry:  RetryConfig{MaxAttempts: 3, BaseDelaySec: 0},
	}
	_, err := RunClaude(ctx, cfg, "test", io.Discard)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if callCount > 1 {
		t.Errorf("expected no retry on cancellation, got %d calls", callCount)
	}
}

func TestRunClaudeExhaustsRetries(t *testing.T) {
	callCount := 0
	newCmd = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		return exec.Command("false")
	}
	defer func() { newCmd = defaultNewCmd }()

	cfg := &Config{
		Claude: ClaudeConfig{Command: "claude", TimeoutSec: 10},
		Retry:  RetryConfig{MaxAttempts: 2, BaseDelaySec: 0},
	}
	_, err := RunClaude(context.Background(), cfg, "test", io.Discard)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestRunClaudeRetries -v && go test ./... -run TestRunClaudeNoRetry -v && go test ./... -run TestRunClaudeExhausts -v`
Expected: FAIL — RunClaude does not retry

**Step 3: Write minimal implementation**

Refactor `RunClaude` in `claude.go` to add retry loop:

```go
func RunClaude(ctx context.Context, cfg *Config, prompt string, w io.Writer) (string, error) {
	maxAttempts := cfg.Retry.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	baseDelay := time.Duration(cfg.Retry.BaseDelaySec) * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if attempt > 1 {
			delay := baseDelay * time.Duration(1<<(attempt-2)) // exponential: base, 2*base, 4*base...
			LogInfo("Retrying (%d/%d) after %v...", attempt, maxAttempts, delay)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}
		output, err := runClaudeOnce(ctx, cfg, prompt, w)
		if err == nil {
			return output, nil
		}
		lastErr = err
		// Don't retry on context cancellation
		if ctx.Err() != nil {
			return output, err
		}
	}
	return "", fmt.Errorf("claude failed after %d attempts: %w", maxAttempts, lastErr)
}
```

Extract current RunClaude body into `runClaudeOnce`:

```go
func runClaudeOnce(ctx context.Context, cfg *Config, prompt string, w io.Writer) (string, error) {
	timeout := time.Duration(cfg.Claude.TimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := BuildClaudeArgs(cfg, prompt)
	cmd := newCmd(ctx, cfg.Claude.Command, args...)
	// ... rest of current RunClaude body ...
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add claude.go claude_test.go
git commit -m "feat(v0.9/B): implement exponential backoff retry in RunClaude"
```

---

## Task 6: Add TotalCount to WaveApplyResult + partial apply delta

**Feature:** C (Completeness Tracking)

**Files:**
- Modify: `model.go`
- Modify: `model_test.go`
- Modify: `session.go`
- Modify: `session_test.go`

**Step 1: Write failing test**

In `model_test.go`:

```go
func TestWaveApplyResultTotalCount(t *testing.T) {
	data := `{"wave_id":"w1","applied":3,"total_count":5,"errors":["e1"]}`
	var result WaveApplyResult
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Fatal(err)
	}
	if result.TotalCount != 5 {
		t.Errorf("TotalCount: expected 5, got %d", result.TotalCount)
	}
}
```

In `session_test.go`:

```go
func TestPartialApplyDelta(t *testing.T) {
	tests := []struct {
		name       string
		applied    int
		total      int
		before     float64
		after      float64
		wantAfter  float64
	}{
		{"full success", 5, 5, 0.3, 0.6, 0.6},
		{"partial 3/5", 3, 5, 0.3, 0.6, 0.48},   // 0.3 + (0.6-0.3) * 3/5 = 0.48
		{"zero applied", 0, 5, 0.3, 0.6, 0.3},
		{"zero total", 0, 0, 0.3, 0.6, 0.6},       // TotalCount=0 means legacy, trust as-is
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &WaveApplyResult{Applied: tt.applied, TotalCount: tt.total}
			delta := WaveDelta{Before: tt.before, After: tt.after}
			got := PartialApplyDelta(result, delta)
			if fmt.Sprintf("%.4f", got) != fmt.Sprintf("%.4f", tt.wantAfter) {
				t.Errorf("PartialApplyDelta: got %.4f, want %.4f", got, tt.wantAfter)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestWaveApplyResultTotalCount -v && go test ./... -run TestPartialApplyDelta -v`
Expected: FAIL

**Step 3: Write minimal implementation**

In `model.go`, add `TotalCount` to `WaveApplyResult`:

```go
type WaveApplyResult struct {
	WaveID     string   `json:"wave_id"`
	Applied    int      `json:"applied"`
	TotalCount int      `json:"total_count,omitempty"`
	Errors     []string `json:"errors"`
	Ripples    []Ripple `json:"ripples"`
}
```

In `session.go`, add:

```go
// PartialApplyDelta computes the adjusted delta for a partially applied wave.
// When TotalCount is 0 (legacy result), the original delta.After is returned.
func PartialApplyDelta(result *WaveApplyResult, delta WaveDelta) float64 {
	if result.TotalCount == 0 || result.Applied >= result.TotalCount {
		return delta.After
	}
	if result.Applied == 0 {
		return delta.Before
	}
	successRate := float64(result.Applied) / float64(result.TotalCount)
	return delta.Before + (delta.After-delta.Before)*successRate
}
```

Wire into `runInteractiveLoop` — replace `selected.Delta.After` with `PartialApplyDelta(applyResult, selected.Delta)` when updating cluster completeness:

```go
adjustedAfter := PartialApplyDelta(applyResult, selected.Delta)
scanResult.Clusters[i].Completeness = adjustedAfter
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add model.go model_test.go session.go session_test.go
git commit -m "feat(v0.9/C): add TotalCount to WaveApplyResult and partial delta calculation"
```

---

## Task 7: Add final consistency check on session end

**Feature:** C (Completeness Tracking)

**Files:**
- Modify: `session.go`
- Modify: `session_test.go`

**Step 1: Write failing test**

In `session_test.go`:

```go
func TestCheckCompletenessConsistency(t *testing.T) {
	tests := []struct {
		name     string
		overall  float64
		clusters []ClusterScanResult
		wantWarn bool
	}{
		{"consistent", 0.5, []ClusterScanResult{
			{Name: "a", Completeness: 0.4},
			{Name: "b", Completeness: 0.6},
		}, false}, // avg = 0.5, matches
		{"inconsistent", 0.9, []ClusterScanResult{
			{Name: "a", Completeness: 0.4},
			{Name: "b", Completeness: 0.6},
		}, true}, // avg = 0.5, but overall = 0.9
		{"empty clusters", 0.0, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckCompletenessConsistency(tt.overall, tt.clusters)
			if got != tt.wantWarn {
				t.Errorf("CheckCompletenessConsistency: got %v, want %v", got, tt.wantWarn)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestCheckCompletenessConsistency -v`
Expected: FAIL

**Step 3: Write minimal implementation**

In `session.go`:

```go
// CheckCompletenessConsistency verifies that the sum of cluster completeness
// values matches the overall completeness. Returns true if mismatch detected.
// Tolerance: 5 percentage points (accounts for rounding).
func CheckCompletenessConsistency(overall float64, clusters []ClusterScanResult) bool {
	if len(clusters) == 0 {
		return false
	}
	var sum float64
	for _, c := range clusters {
		sum += c.Completeness
	}
	avg := sum / float64(len(clusters))
	diff := overall - avg
	if diff < 0 {
		diff = -diff
	}
	return diff > 0.05
}
```

Wire into `runInteractiveLoop` after the loop ends (before final state save):

```go
if CheckCompletenessConsistency(scanResult.Completeness, scanResult.Clusters) {
	LogWarn("Completeness mismatch detected. Recalculating...")
	scanResult.CalculateCompleteness()
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add session.go session_test.go
git commit -m "feat(v0.9/C): add final completeness consistency check"
```

---

## Task 8: Add LabelsConfig type + config field

**Feature:** D (Linear Label Integration)

**Files:**
- Modify: `config.go`
- Modify: `config_test.go`

**Step 1: Write failing test**

In `config_test.go`:

```go
func TestLabelsConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Labels.Enabled {
		t.Error("expected Labels.Enabled=true by default")
	}
	if cfg.Labels.Prefix != "sightjack" {
		t.Errorf("expected Prefix='sightjack', got %q", cfg.Labels.Prefix)
	}
	if cfg.Labels.ReadyLabel != "sightjack:ready" {
		t.Errorf("expected ReadyLabel='sightjack:ready', got %q", cfg.Labels.ReadyLabel)
	}
}

func TestLoadConfigWithLabels(t *testing.T) {
	content := `
linear:
  team: test
  project: test
labels:
  enabled: false
  prefix: "myprefix"
  ready_label: "myprefix:done"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Labels.Enabled {
		t.Error("expected Labels.Enabled=false")
	}
	if cfg.Labels.Prefix != "myprefix" {
		t.Errorf("Prefix: expected 'myprefix', got %q", cfg.Labels.Prefix)
	}
	if cfg.Labels.ReadyLabel != "myprefix:done" {
		t.Errorf("ReadyLabel: expected 'myprefix:done', got %q", cfg.Labels.ReadyLabel)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestLabelsConfig -v && go test ./... -run TestLoadConfigWithLabels -v`
Expected: FAIL

**Step 3: Write minimal implementation**

In `config.go`:

```go
// LabelsConfig holds Linear label assignment settings.
type LabelsConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Prefix     string `yaml:"prefix"`
	ReadyLabel string `yaml:"ready_label"`
}
```

Add field to `Config`:
```go
Labels LabelsConfig `yaml:"labels"`
```

Add default in `DefaultConfig()`:
```go
Labels: LabelsConfig{
	Enabled:    true,
	Prefix:     "sightjack",
	ReadyLabel: "sightjack:ready",
},
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add config.go config_test.go
git commit -m "feat(v0.9/D): add LabelsConfig type and config field"
```

---

## Task 9: Add LabelsEnabled to prompt data + update templates

**Feature:** D (Linear Labels)

**Files:**
- Modify: `prompt.go`
- Modify: `prompts/templates/scanner_classify_en.md.tmpl`
- Modify: `prompts/templates/scanner_classify_ja.md.tmpl`
- Modify: `prompts/templates/wave_apply_en.md.tmpl`
- Modify: `prompts/templates/wave_apply_ja.md.tmpl`
- Modify: `prompt_test.go`

**Step 1: Write failing test**

In `prompt_test.go`:

```go
func TestRenderClassifyPromptWithLabels(t *testing.T) {
	data := ClassifyPromptData{
		TeamFilter:      "test",
		ProjectFilter:   "test",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
		LabelsEnabled:   true,
		LabelPrefix:     "sightjack",
	}
	for _, lang := range []string{"en", "ja"} {
		result, err := RenderClassifyPrompt(lang, data)
		if err != nil {
			t.Fatalf("lang=%s: %v", lang, err)
		}
		if !strings.Contains(result, "sightjack:analyzed") {
			t.Errorf("lang=%s: expected label instruction in output", lang)
		}
	}
}

func TestRenderWaveApplyPromptWithLabels(t *testing.T) {
	data := WaveApplyPromptData{
		WaveID:          "w1",
		ClusterName:     "auth",
		Title:           "Wave 1",
		Actions:         "[]",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
		LabelsEnabled:   true,
		LabelPrefix:     "sightjack",
	}
	for _, lang := range []string{"en", "ja"} {
		result, err := RenderWaveApplyPrompt(lang, data)
		if err != nil {
			t.Fatalf("lang=%s: %v", lang, err)
		}
		if !strings.Contains(result, "sightjack:wave-done") {
			t.Errorf("lang=%s: expected label instruction in output", lang)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestRenderClassifyPromptWithLabels -v && go test ./... -run TestRenderWaveApplyPromptWithLabels -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Add `LabelsEnabled` and `LabelPrefix` fields to `ClassifyPromptData` and `WaveApplyPromptData`:

```go
type ClassifyPromptData struct {
	// ... existing fields ...
	LabelsEnabled bool
	LabelPrefix   string
}

type WaveApplyPromptData struct {
	// ... existing fields ...
	LabelsEnabled bool
	LabelPrefix   string
}
```

Add label sections to templates:

For `scanner_classify_{en,ja}.md.tmpl`:
```
{{if .LabelsEnabled}}
After classification, apply the label "{{.LabelPrefix}}:analyzed" to each analyzed issue using Linear MCP.
{{end}}
```

For `wave_apply_{en,ja}.md.tmpl`:
```
{{if .LabelsEnabled}}
After applying all actions, apply the label "{{.LabelPrefix}}:wave-done" to each affected issue using Linear MCP.
{{end}}
```

Wire into call sites in `scanner.go` (classify prompt) and `wave.go` (apply prompt):
- `scanner.go`: set `LabelsEnabled: cfg.Labels.Enabled, LabelPrefix: cfg.Labels.Prefix` in `ClassifyPromptData`
- `wave.go`: set `LabelsEnabled: cfg.Labels.Enabled, LabelPrefix: cfg.Labels.Prefix` in `WaveApplyPromptData`

Note: `RunWaveApply` in `wave.go` receives `cfg *Config` already, so it has access to `cfg.Labels`.

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add prompt.go prompt_test.go prompts/templates/ scanner.go wave.go
git commit -m "feat(v0.9/D): add label instructions to classify and wave_apply templates"
```

---

## Task 10: Add ReadyIssueIDs computation

**Feature:** E (Paintress Bridge) — depends on Task 8-9 (Feature D)

**Files:**
- New: `paintress.go`
- New: `paintress_test.go`

**Step 1: Write failing test**

Create `paintress_test.go`:

```go
package sightjack

import "testing"

func TestReadyIssueIDs(t *testing.T) {
	waves := []Wave{
		{ID: "w1", ClusterName: "auth", Status: "completed",
			Actions: []WaveAction{
				{IssueID: "AUTH-1"},
				{IssueID: "AUTH-2"},
			}},
		{ID: "w2", ClusterName: "auth", Status: "completed",
			Actions: []WaveAction{
				{IssueID: "AUTH-2"},
				{IssueID: "AUTH-3"},
			}},
		{ID: "w3", ClusterName: "auth", Status: "available",
			Actions: []WaveAction{
				{IssueID: "AUTH-3"},
			}},
	}

	// AUTH-1 is only in w1 (completed) -> ready
	// AUTH-2 is in w1 (completed) and w2 (completed) -> ready
	// AUTH-3 is in w2 (completed) and w3 (available) -> NOT ready
	ready := ReadyIssueIDs(waves)

	if len(ready) != 2 {
		t.Fatalf("expected 2 ready issues, got %d: %v", len(ready), ready)
	}
	readySet := make(map[string]bool)
	for _, id := range ready {
		readySet[id] = true
	}
	if !readySet["AUTH-1"] {
		t.Error("expected AUTH-1 to be ready")
	}
	if !readySet["AUTH-2"] {
		t.Error("expected AUTH-2 to be ready")
	}
	if readySet["AUTH-3"] {
		t.Error("expected AUTH-3 to NOT be ready")
	}
}

func TestReadyIssueIDsNoCompleted(t *testing.T) {
	waves := []Wave{
		{ID: "w1", Status: "available", Actions: []WaveAction{{IssueID: "A-1"}}},
	}
	ready := ReadyIssueIDs(waves)
	if len(ready) != 0 {
		t.Errorf("expected 0 ready issues, got %d", len(ready))
	}
}

func TestReadyIssueIDsAllCompleted(t *testing.T) {
	waves := []Wave{
		{ID: "w1", Status: "completed", Actions: []WaveAction{{IssueID: "A-1"}}},
		{ID: "w2", Status: "completed", Actions: []WaveAction{{IssueID: "A-1"}, {IssueID: "A-2"}}},
	}
	ready := ReadyIssueIDs(waves)
	if len(ready) != 2 {
		t.Errorf("expected 2 ready issues, got %d", len(ready))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestReadyIssueIDs -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Create `paintress.go`:

```go
package sightjack

// ReadyIssueIDs returns issue IDs where ALL waves targeting them are completed.
// An issue is ready when every wave containing that issue has status "completed".
func ReadyIssueIDs(waves []Wave) []string {
	// Track all waves per issue
	issueWaves := make(map[string][]string) // issueID -> []waveStatus
	for _, w := range waves {
		for _, a := range w.Actions {
			issueWaves[a.IssueID] = append(issueWaves[a.IssueID], w.Status)
		}
	}

	var ready []string
	for issueID, statuses := range issueWaves {
		allCompleted := true
		for _, s := range statuses {
			if s != "completed" {
				allCompleted = false
				break
			}
		}
		if allCompleted {
			ready = append(ready, issueID)
		}
	}
	return ready
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add paintress.go paintress_test.go
git commit -m "feat(v0.9/E): add ReadyIssueIDs computation for Paintress Bridge"
```

---

## Task 11: Wire ready label into wave_apply template + session flow

**Feature:** E (Paintress Bridge)

**Files:**
- Modify: `prompt.go`
- Modify: `prompts/templates/wave_apply_en.md.tmpl`
- Modify: `prompts/templates/wave_apply_ja.md.tmpl`
- Modify: `session.go`
- Modify: `prompt_test.go`

**Step 1: Write failing test**

In `prompt_test.go`:

```go
func TestRenderWaveApplyPromptWithReadyIssues(t *testing.T) {
	data := WaveApplyPromptData{
		WaveID:          "w1",
		ClusterName:     "auth",
		Title:           "Wave 1",
		Actions:         "[]",
		OutputPath:      "/tmp/out.json",
		StrictnessLevel: "fog",
		LabelsEnabled:   true,
		LabelPrefix:     "sightjack",
		ReadyLabel:      "sightjack:ready",
		ReadyIssueIDs:   "AUTH-1, AUTH-2",
	}
	for _, lang := range []string{"en", "ja"} {
		result, err := RenderWaveApplyPrompt(lang, data)
		if err != nil {
			t.Fatalf("lang=%s: %v", lang, err)
		}
		if !strings.Contains(result, "sightjack:ready") {
			t.Errorf("lang=%s: expected ready label in output", lang)
		}
		if !strings.Contains(result, "AUTH-1") {
			t.Errorf("lang=%s: expected ready issue IDs in output", lang)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestRenderWaveApplyPromptWithReadyIssues -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Add fields to `WaveApplyPromptData`:
```go
ReadyLabel    string
ReadyIssueIDs string
```

Add to `wave_apply_{en,ja}.md.tmpl` templates:
```
{{if and .LabelsEnabled .ReadyIssueIDs}}
The following issues have ALL their waves completed. Apply the "{{.ReadyLabel}}" label to them using Linear MCP:
{{.ReadyIssueIDs}}
{{end}}
```

Wire in `session.go` — before calling `RunWaveApply`, compute ready issues:
```go
// Compute ready issues for Paintress Bridge label
var readyIssueStr string
if cfg.Labels.Enabled {
	readyIDs := ReadyIssueIDs(waves)
	if len(readyIDs) > 0 {
		readyIssueStr = strings.Join(readyIDs, ", ")
	}
}
```

Pass `ReadyLabel: cfg.Labels.ReadyLabel, ReadyIssueIDs: readyIssueStr` through to `RunWaveApply`. This requires either adding the data to the `Wave` struct or modifying `RunWaveApply` to accept config. The simplest approach: pass cfg to the wave_apply prompt render site in wave.go and add the fields there.

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add prompt.go prompt_test.go prompts/templates/ session.go wave.go
git commit -m "feat(v0.9/E): wire Paintress Bridge ready label into wave apply"
```

---

## Task 12: Add RecoverStateFromScan function

**Feature:** F (State Recovery)

**Files:**
- Modify: `session.go`
- Modify: `session_test.go`

**Step 1: Write failing test**

In `session_test.go`:

```go
func TestRecoverStateFromScan(t *testing.T) {
	scanResult := &ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "auth", Completeness: 0.4, Issues: []IssueDetail{{ID: "A-1"}}},
			{Name: "infra", Completeness: 0.6, Issues: []IssueDetail{{ID: "I-1"}, {ID: "I-2"}}},
		},
		Completeness: 0.5,
	}
	waves := []Wave{
		{ID: "w1", ClusterName: "auth", Status: "completed"},
		{ID: "w2", ClusterName: "auth", Status: "available"},
	}

	dir := t.TempDir()
	adrDir := filepath.Join(dir, "docs", "adr")
	os.MkdirAll(adrDir, 0755)
	os.WriteFile(filepath.Join(adrDir, "0001-test.md"), []byte("adr"), 0644)
	os.WriteFile(filepath.Join(adrDir, "0002-test2.md"), []byte("adr2"), 0644)

	state := RecoverStateFromScan(scanResult, waves, adrDir)

	if state.Completeness != 0.5 {
		t.Errorf("Completeness: expected 0.5, got %f", state.Completeness)
	}
	if len(state.Clusters) != 2 {
		t.Errorf("Clusters: expected 2, got %d", len(state.Clusters))
	}
	if state.ADRCount != 2 {
		t.Errorf("ADRCount: expected 2, got %d", state.ADRCount)
	}
	if len(state.Waves) != 2 {
		t.Errorf("Waves: expected 2, got %d", len(state.Waves))
	}
}

func TestRecoverStateFromScanEmpty(t *testing.T) {
	scanResult := &ScanResult{}
	state := RecoverStateFromScan(scanResult, nil, "/nonexistent")

	if state.Completeness != 0 {
		t.Errorf("Completeness: expected 0, got %f", state.Completeness)
	}
	if len(state.Clusters) != 0 {
		t.Errorf("Clusters: expected 0, got %d", len(state.Clusters))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestRecoverStateFromScan -v`
Expected: FAIL

**Step 3: Write minimal implementation**

In `session.go`:

```go
// RecoverStateFromScan reconstructs a SessionState from a cached ScanResult
// and wave list. Used when state.json is missing or corrupted but scan_result.json exists.
// Unrecoverable fields (sessionRejected, exact lastScanned) are set to zero values.
func RecoverStateFromScan(scanResult *ScanResult, waves []Wave, adrDir string) *SessionState {
	state := &SessionState{
		Version:      "0.9",
		Completeness: scanResult.Completeness,
		LastScanned:  time.Now(),
		ADRCount:     CountADRFiles(adrDir),
		ShibitoCount: len(scanResult.ShibitoWarnings),
	}
	for _, c := range scanResult.Clusters {
		state.Clusters = append(state.Clusters, ClusterState{
			Name:         c.Name,
			Completeness: c.Completeness,
			IssueCount:   len(c.Issues),
		})
	}
	if len(waves) > 0 {
		state.Waves = BuildWaveStates(waves)
	}
	return state
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add session.go session_test.go
git commit -m "feat(v0.9/F): add RecoverStateFromScan for state recovery"
```

---

## Task 13: Wire recovery into session resume flow

**Feature:** F (State Recovery)

**Files:**
- Modify: `session.go`
- Modify: `cmd/sightjack/main.go`
- Modify: `session_test.go`

**Step 1: Write failing test**

In `session_test.go`:

```go
func TestRecoverFromScanResultFile(t *testing.T) {
	dir := t.TempDir()

	// Create a cached scan result without a state.json
	sessionID := "test-session"
	scanDir, _ := EnsureScanDir(dir, sessionID)
	scanResult := &ScanResult{
		Clusters:    []ClusterScanResult{{Name: "auth", Completeness: 0.5}},
		Completeness: 0.5,
	}
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	WriteScanResult(scanResultPath, scanResult)

	// Try to recover
	recovered, recErr := TryRecoverState(dir, sessionID)
	if recErr != nil {
		t.Fatalf("TryRecoverState: %v", recErr)
	}
	if recovered == nil {
		t.Fatal("expected recovered state, got nil")
	}
	if recovered.Completeness != 0.5 {
		t.Errorf("Completeness: expected 0.5, got %f", recovered.Completeness)
	}
}

func TestTryRecoverStateNoFiles(t *testing.T) {
	dir := t.TempDir()
	recovered, err := TryRecoverState(dir, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
	if recovered != nil {
		t.Error("expected nil state")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestRecoverFromScanResultFile -v && go test ./... -run TestTryRecoverStateNoFiles -v`
Expected: FAIL

**Step 3: Write minimal implementation**

In `session.go`:

```go
// TryRecoverState attempts to recover session state from cached scan result files.
// Recovery chain: 1. Try scan_result.json in scan dir → recover clusters, waves, completeness.
// Returns error if no recoverable data found.
func TryRecoverState(baseDir string, sessionID string) (*SessionState, error) {
	scanDir := ScanDir(baseDir, sessionID)
	scanResultPath := filepath.Join(scanDir, "scan_result.json")

	scanResult, err := LoadScanResult(scanResultPath)
	if err != nil {
		return nil, fmt.Errorf("no recoverable scan data: %w", err)
	}

	LogWarn("State file missing. Recovered from cached scan result.")

	adrDir := ADRDir(baseDir)
	state := RecoverStateFromScan(scanResult, nil, adrDir)
	state.SessionID = sessionID
	state.ScanResultPath = scanResultPath
	return state, nil
}
```

Wire into `cmd/sightjack/main.go` — in the `case "session"` block, add recovery attempt when `ReadState` fails:

After the existing `existingState, stateErr := sightjack.ReadState(baseDir)` check, when stateErr is non-nil, attempt:
```go
// Try recovery from cached scan results
entries, _ := os.ReadDir(filepath.Join(baseDir, ".siren", "scans"))
if len(entries) > 0 {
	lastEntry := entries[len(entries)-1]
	recovered, recErr := sightjack.TryRecoverState(baseDir, lastEntry.Name())
	if recErr == nil {
		existingState = recovered
		stateErr = nil
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add session.go session_test.go cmd/sightjack/main.go
git commit -m "feat(v0.9/F): wire state recovery into session resume flow"
```

---

## Task 14: Implement RunParallelDeepScan with semaphore

**Feature:** G (Parallel Scan)

**Files:**
- Modify: `scanner.go`
- Modify: `scanner_test.go`

**Step 1: Write failing test**

In `scanner_test.go`:

```go
func TestRunParallelDeepScan(t *testing.T) {
	// Test with mock that tracks concurrent goroutines
	clusters := []ClusterScanResult{
		{Name: "auth", Issues: []IssueDetail{{ID: "A-1"}}},
		{Name: "infra", Issues: []IssueDetail{{ID: "I-1"}}},
		{Name: "frontend", Issues: []IssueDetail{{ID: "F-1"}}},
	}

	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.Scan.MaxConcurrency = 2

	// Since this calls RunClaude, we need the scan function to be testable
	// Test the concurrency control behavior by verifying the function signature and basic flow
	results, warnings := RunParallelDeepScan(context.Background(), &cfg, dir, clusters,
		func(ctx context.Context, cfg *Config, scanDir string, cluster ClusterScanResult) (ClusterScanResult, error) {
			return ClusterScanResult{Name: cluster.Name, Completeness: 0.5}, nil
		})

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(warnings))
	}
}

func TestRunParallelDeepScanWithFailure(t *testing.T) {
	clusters := []ClusterScanResult{
		{Name: "auth"},
		{Name: "infra"},
	}

	dir := t.TempDir()
	cfg := DefaultConfig()
	callCount := 0

	results, warnings := RunParallelDeepScan(context.Background(), &cfg, dir, clusters,
		func(ctx context.Context, cfg *Config, scanDir string, cluster ClusterScanResult) (ClusterScanResult, error) {
			callCount++
			if cluster.Name == "auth" {
				return ClusterScanResult{}, fmt.Errorf("auth scan failed")
			}
			return ClusterScanResult{Name: cluster.Name, Completeness: 0.7}, nil
		})

	// Failed cluster should be skipped, remaining continue
	if len(results) != 1 {
		t.Errorf("expected 1 successful result, got %d", len(results))
	}
	if results[0].Name != "infra" {
		t.Errorf("expected 'infra', got %q", results[0].Name)
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (both attempted), got %d", callCount)
	}
}

func TestRunParallelDeepScanSingleCluster(t *testing.T) {
	clusters := []ClusterScanResult{{Name: "only"}}
	dir := t.TempDir()
	cfg := DefaultConfig()

	results, _ := RunParallelDeepScan(context.Background(), &cfg, dir, clusters,
		func(ctx context.Context, cfg *Config, scanDir string, cluster ClusterScanResult) (ClusterScanResult, error) {
			return ClusterScanResult{Name: cluster.Name, Completeness: 1.0}, nil
		})

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestRunParallelDeepScan -v`
Expected: FAIL

**Step 3: Write minimal implementation**

In `scanner.go`:

```go
// DeepScanFunc is the function signature for scanning a single cluster.
// Used by RunParallelDeepScan for testability.
type DeepScanFunc func(ctx context.Context, cfg *Config, scanDir string, cluster ClusterScanResult) (ClusterScanResult, error)

// RunParallelDeepScan executes deep scan across clusters using goroutines with
// semaphore-based concurrency control. Failed clusters get LogWarn and are skipped;
// remaining clusters continue. Returns successful results and warning messages.
func RunParallelDeepScan(ctx context.Context, cfg *Config, scanDir string,
	clusters []ClusterScanResult, scanFn DeepScanFunc) ([]ClusterScanResult, []string) {

	maxConcurrency := cfg.Scan.MaxConcurrency
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}

	type scanResult struct {
		index   int
		cluster ClusterScanResult
		err     error
	}

	sem := make(chan struct{}, maxConcurrency)
	results := make(chan scanResult, len(clusters))

	for i, cluster := range clusters {
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
			result, err := scanFn(ctx, cfg, scanDir, cluster)
			results <- scanResult{index: i, cluster: result, err: err}
		}()
	}

	var successful []ClusterScanResult
	var warnings []string
	for range clusters {
		r := <-results
		if r.err != nil {
			msg := fmt.Sprintf("Cluster %q scan failed: %v", clusters[r.index].Name, r.err)
			LogWarn("%s", msg)
			warnings = append(warnings, msg)
			continue
		}
		successful = append(successful, r.cluster)
	}

	return successful, warnings
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add scanner.go scanner_test.go
git commit -m "feat(v0.9/G): implement RunParallelDeepScan with semaphore concurrency"
```

---

## Task 15: Wire RunParallelDeepScan into scanner Pass 2

**Feature:** G (Parallel Scan)

**Files:**
- Modify: `scanner.go`
- Modify: `scanner_test.go`

**Step 1: Write failing test**

In `scanner_test.go`, ensure existing scan tests still pass and verify the parallel scan is actually wired (check that the `errgroup` import can be removed or unused):

```go
func TestRunParallelDeepScanActivation(t *testing.T) {
	// Verify: cluster count >= 2 && MaxConcurrency > 1 activates parallel
	cfg := DefaultConfig()
	cfg.Scan.MaxConcurrency = 2

	clusters := []ClusterScanResult{
		{Name: "a"},
		{Name: "b"},
	}

	callCount := 0
	results, _ := RunParallelDeepScan(context.Background(), &cfg, t.TempDir(), clusters,
		func(ctx context.Context, cfg *Config, scanDir string, cluster ClusterScanResult) (ClusterScanResult, error) {
			callCount++
			return ClusterScanResult{Name: cluster.Name, Completeness: 0.5}, nil
		})

	if callCount != 2 {
		t.Errorf("expected 2 scan calls, got %d", callCount)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}
```

**Step 2: Refactor scanner.go Pass 2**

Replace the `errgroup` based Pass 2 in `RunScan` with a call to `RunParallelDeepScan`:

- Extract the per-cluster deep scan logic (chunk + render + run + parse + merge) into a `deepScanCluster` function matching `DeepScanFunc` signature
- Replace the errgroup block with `RunParallelDeepScan(ctx, cfg, scanDir, classifiedClusters, deepScanCluster)`
- Handle warnings: log them (already done inside RunParallelDeepScan via LogWarn)
- If all clusters fail, return an error

The same refactoring applies to `RunWaveGenerate` Pass 3 if desired (optional — only Pass 2 is in scope per design).

**Step 3: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS. The `errgroup` import may become unused if both Pass 2 and Pass 3 are migrated; otherwise keep it for Pass 3.

**Step 4: Commit**

```bash
git add scanner.go scanner_test.go
git commit -m "refactor(v0.9/G): wire RunParallelDeepScan into scanner Pass 2"
```

---

## Task 16: Version bump + full test suite

**Files:**
- Modify: `cmd/sightjack/main.go` (version `0.9.0-dev`)
- Modify: `session.go` (BuildSessionState version `0.9`)
- Modify: `session_test.go` (update version assertion)

**Step 1: Version bump**

In `cmd/sightjack/main.go`:
```go
var version = "0.9.0-dev"
```

In `session.go` `BuildSessionState`:
```go
Version: "0.9",
```

In `session_test.go`, update any test asserting version `"0.8"` to `"0.9"`.

**Step 2: Full test suite**

Run: `go test ./... -v`
Expected: ALL PASS

Run: `go build ./...`
Expected: clean build

Run: `go vet ./...`
Expected: no issues

**Step 3: Commit**

```bash
git add cmd/sightjack/main.go session.go session_test.go
git commit -m "chore(v0.9): version bump to 0.9.0-dev"
```

---

## Key Patterns to Reuse

| Pattern | Source | Location |
|---------|--------|----------|
| Agent file naming | `architectDiscussFileName` | architect.go:25 |
| Stale output cleanup | `clearArchitectOutput` | architect.go:55 |
| Dry-run via `RunClaudeDryRun` | | claude.go:89 |
| Live run via `RunClaude` | | claude.go:36 |
| JSON parse pattern | `ParseArchitectResult` | architect.go:12 |
| Template render | `renderTemplate` | prompt.go:85 |
| File name sanitization | `sanitizeName` | scanner.go:240 |
| Non-fatal error pattern | `LogWarn` | session.go |
| Config defaults + validation | `DefaultConfig` / `LoadConfig` | config.go:51-99 |
| Prompt data field addition | `WaveGeneratePromptData` | prompt.go:30 |
| Wave identity | `WaveKey(w)` | wave.go:39 |
| Parallel execution | `errgroup.SetLimit` | scanner.go:96 |

## Verification Checklist

1. `go build ./...` — compiles clean
2. `go test ./... -v` — all tests pass (existing + new)
3. `go vet ./...` — no issues
4. Dry-run test: `go run ./cmd/sightjack --dry-run session` generates all prompt files
5. Config test: YAML with all new fields parses correctly
6. Manual check: version shows `0.9.0-dev`
