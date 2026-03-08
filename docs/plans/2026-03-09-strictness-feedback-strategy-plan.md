# Strictness × Feedback Wave Generation Strategy Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Redefine Strictness as "change tolerance for existing implementations," add scan-time estimation, cluster keys, issue status tracking, and cancel action support

**Architecture:** Extend domain types (ClusterScanResult, IssueDetail, StrictnessConfig) with new fields, update ResolveStrictness to 3-layer resolution, modify all prompt templates to use new Strictness definition, add cancel action to wave apply pipeline, write estimated strictness back to config.yaml after scan

**Tech Stack:** Go 1.26, Cobra CLI, YAML templates, Linear MCP (via Claude Code)

---

### Task 1: Domain — Add Cluster Key to ClusterScanResult

**Files:**
- Modify: `internal/domain/types.go`
- Test: `internal/domain/types_test.go` (add test)

**Step 1: Write failing test**

```go
func TestClusterScanResult_Key(t *testing.T) {
	c := ClusterScanResult{
		Name: "Auth Module",
		Key:  "auth-module",
	}
	if c.Key != "auth-module" {
		t.Errorf("expected auth-module, got %s", c.Key)
	}
}
```

**Step 2: Run to verify fails**

Run: `cd /Users/nino/tap/sightjack && go test ./internal/domain/ -run TestClusterScanResult_Key -v`

**Step 3: Implement**

Add `Key` field to `ClusterScanResult` in `internal/domain/types.go`:

```go
type ClusterScanResult struct {
	Name         string        `json:"name"`
	Key          string        `json:"key"`           // NEW: English slug for YAML key
	Completeness float64       `json:"completeness"`
	Issues       []IssueDetail `json:"issues"`
	Observations []string      `json:"observations"`
	Labels       []string      `json:"labels,omitempty"`
	IssueCount   int           `json:"-"`
}
```

Update `StrictnessKeys` to include `Key`:

```go
func (r *ScanResult) StrictnessKeys(clusterName string) []string {
	keys := []string{clusterName}
	for _, c := range r.Clusters {
		if c.Name == clusterName && c.Key != "" {
			keys = append(keys, c.Key)
			break
		}
	}
	return append(keys, r.ClusterLabels(clusterName)...)
}
```

**Step 4: Run test, verify pass**

Run: `cd /Users/nino/tap/sightjack && go test ./internal/domain/ -run TestClusterScanResult_Key -v`

**Step 5: Commit**

```bash
git -C /Users/nino/tap/sightjack add internal/domain/types.go internal/domain/types_test.go
git -C /Users/nino/tap/sightjack commit -m "feat: add Key field to ClusterScanResult for stable YAML keys"
```

---

### Task 2: Domain — Add Issue Status to IssueDetail

**Files:**
- Modify: `internal/domain/types.go`
- Test: `internal/domain/types_test.go` (add test)

**Step 1: Write failing test**

```go
func TestIssueDetail_HasStatus(t *testing.T) {
	issue := IssueDetail{
		ID:           "issue-1",
		Identifier:   "MY-123",
		Title:        "Auth flow",
		Status:       "in_progress",
		Completeness: 0.6,
		Gaps:         []string{"DoD missing"},
	}
	if issue.Status != "in_progress" {
		t.Errorf("expected in_progress, got %s", issue.Status)
	}
}
```

**Step 2: Implement**

Add `Status` field to `IssueDetail`:

```go
type IssueDetail struct {
	ID           string   `json:"id"`
	Identifier   string   `json:"identifier"`
	Title        string   `json:"title"`
	Status       string   `json:"status"`       // NEW: backlog, todo, in_progress, done, etc.
	Completeness float64  `json:"completeness"`
	Gaps         []string `json:"gaps"`
}
```

**Step 3: Run test, verify pass**

**Step 4: Commit**

```bash
git -C /Users/nino/tap/sightjack add internal/domain/types.go internal/domain/types_test.go
git -C /Users/nino/tap/sightjack commit -m "feat: add Status field to IssueDetail for cancel eligibility"
```

---

### Task 3: Domain — Add EstimatedStrictness to ClusterScanResult

**Files:**
- Modify: `internal/domain/types.go`
- Test: `internal/domain/types_test.go` (add test)

**Step 1: Write failing test**

```go
func TestClusterScanResult_EstimatedStrictness(t *testing.T) {
	c := ClusterScanResult{
		Name:                "Auth Module",
		Key:                 "auth-module",
		EstimatedStrictness: "alert",
		StrictnessReasoning: "Done 60%, In Progress 15%. Core auth has tight coupling.",
	}
	if c.EstimatedStrictness != "alert" {
		t.Errorf("expected alert, got %s", c.EstimatedStrictness)
	}
}
```

**Step 2: Implement**

Add fields to `ClusterScanResult`:

```go
type ClusterScanResult struct {
	Name                string        `json:"name"`
	Key                 string        `json:"key"`
	Completeness        float64       `json:"completeness"`
	EstimatedStrictness string        `json:"estimated_strictness,omitempty"` // NEW
	StrictnessReasoning string        `json:"strictness_reasoning,omitempty"` // NEW
	Issues              []IssueDetail `json:"issues"`
	Observations        []string      `json:"observations"`
	Labels              []string      `json:"labels,omitempty"`
	IssueCount          int           `json:"-"`
}
```

**Step 3: Run test, verify pass**

**Step 4: Commit**

```bash
git -C /Users/nino/tap/sightjack add internal/domain/types.go internal/domain/types_test.go
git -C /Users/nino/tap/sightjack commit -m "feat: add EstimatedStrictness to ClusterScanResult"
```

---

### Task 4: Domain — Add Estimated Field to StrictnessConfig + 3-Layer Resolution

**Files:**
- Modify: `internal/domain/config.go`
- Modify: `internal/domain/config_test.go`

**Step 1: Write failing tests**

```go
func TestResolveStrictness_EstimatedTakesEffect(t *testing.T) {
	cfg := StrictnessConfig{
		Default:   StrictnessFog,
		Estimated: map[string]StrictnessLevel{"auth-module": StrictnessAlert},
	}
	// Estimated alert > default fog → alert
	got := ResolveStrictness(cfg, []string{"auth-module"})
	if got != StrictnessAlert {
		t.Errorf("expected alert, got %s", got)
	}
}

func TestResolveStrictness_OverrideTrumpsEstimated(t *testing.T) {
	cfg := StrictnessConfig{
		Default:   StrictnessFog,
		Overrides: map[string]StrictnessLevel{"auth-module": StrictnessLockdown},
		Estimated: map[string]StrictnessLevel{"auth-module": StrictnessAlert},
	}
	// Override lockdown > estimated alert → lockdown
	got := ResolveStrictness(cfg, []string{"auth-module"})
	if got != StrictnessLockdown {
		t.Errorf("expected lockdown, got %s", got)
	}
}

func TestResolveStrictness_MaxOfDefaultAndEstimated(t *testing.T) {
	cfg := StrictnessConfig{
		Default:   StrictnessAlert,
		Estimated: map[string]StrictnessLevel{"auth-module": StrictnessFog},
	}
	// Default alert > estimated fog → alert
	got := ResolveStrictness(cfg, []string{"auth-module"})
	if got != StrictnessAlert {
		t.Errorf("expected alert, got %s", got)
	}
}
```

**Step 2: Run to verify fails**

Run: `cd /Users/nino/tap/sightjack && go test ./internal/domain/ -run TestResolveStrictness_Estimated -v`

**Step 3: Implement**

Add `Estimated` field to `StrictnessConfig`:

```go
type StrictnessConfig struct {
	Default   StrictnessLevel            `yaml:"default"`
	Overrides map[string]StrictnessLevel `yaml:"overrides"`
	Estimated map[string]StrictnessLevel `yaml:"estimated"` // NEW: auto-generated by scan
}
```

Update `ResolveStrictness` for 3-layer resolution:

```go
func ResolveStrictness(cfg StrictnessConfig, labels []string) StrictnessLevel {
	base := cfg.Default

	// Layer 1: Check estimated (auto-generated by scan)
	for _, label := range labels {
		lower := strings.ToLower(label)
		for key, level := range cfg.Estimated {
			if strings.ToLower(key) == lower {
				if strictnessRank(level) > strictnessRank(base) {
					base = level
				}
			}
		}
	}

	// Layer 2: Check overrides (manual, always wins if stronger)
	if len(cfg.Overrides) > 0 && len(labels) > 0 {
		for _, label := range labels {
			lower := strings.ToLower(label)
			for key, level := range cfg.Overrides {
				if strings.ToLower(key) == lower {
					if strictnessRank(level) > strictnessRank(base) {
						base = level
					}
				}
			}
		}
	}

	return base
}
```

**Step 4: Run all ResolveStrictness tests, verify pass**

Run: `cd /Users/nino/tap/sightjack && go test ./internal/domain/ -run TestResolveStrictness -v`

**Step 5: Commit**

```bash
git -C /Users/nino/tap/sightjack add internal/domain/config.go internal/domain/config_test.go
git -C /Users/nino/tap/sightjack commit -m "feat: add Estimated to StrictnessConfig with 3-layer resolution"
```

---

### Task 5: Session — Write Estimated Strictness to Config After Scan

**Files:**
- Modify: `internal/session/config.go`
- Test: `internal/session/config_test.go` (add test)

**Step 1: Write failing test**

```go
func TestWriteEstimatedStrictness(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")

	initial := domain.DefaultConfig()
	data, _ := yaml.Marshal(initial)
	os.WriteFile(cfgPath, data, 0644)

	estimated := map[string]domain.StrictnessLevel{
		"auth-module":     domain.StrictnessAlert,
		"payment-billing": domain.StrictnessLockdown,
	}

	err := WriteEstimatedStrictness(cfgPath, estimated)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Strictness.Estimated["auth-module"] != domain.StrictnessAlert {
		t.Errorf("expected alert, got %s", cfg.Strictness.Estimated["auth-module"])
	}
	if cfg.Strictness.Estimated["payment-billing"] != domain.StrictnessLockdown {
		t.Errorf("expected lockdown, got %s", cfg.Strictness.Estimated["payment-billing"])
	}
}
```

**Step 2: Implement**

Add `WriteEstimatedStrictness` function to `internal/session/config.go`:

```go
// WriteEstimatedStrictness reads the config, replaces the estimated strictness map,
// and writes back. This is called after scan to persist LLM-estimated strictness values.
func WriteEstimatedStrictness(path string, estimated map[string]domain.StrictnessLevel) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config for estimated strictness: %w", err)
	}

	cfg := domain.DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config for estimated strictness: %w", err)
	}

	cfg.Strictness.Estimated = estimated

	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, out, 0644)
}
```

Also update `LoadConfig` validation to validate estimated values:

```go
for label, level := range cfg.Strictness.Estimated {
	if !level.Valid() {
		return nil, fmt.Errorf("invalid estimated strictness for %q: %q", label, level)
	}
}
```

**Step 3: Run test, verify pass**

**Step 4: Commit**

```bash
git -C /Users/nino/tap/sightjack add internal/session/config.go internal/session/config_test.go
git -C /Users/nino/tap/sightjack commit -m "feat: add WriteEstimatedStrictness for scan-time config update"
```

---

### Task 6: Session — Call WriteEstimatedStrictness After Scan

**Files:**
- Modify: `internal/session/session.go` (and/or `session_rescan.go`)
- Look for where scan results are finalized and clusters available

**Step 1: Identify integration point**

Find where `RunScan` completes and `ClusterScanResult` is available. After scan completion, extract estimated strictness from each cluster and call `WriteEstimatedStrictness`.

**Step 2: Implement**

After scan results are merged (Pass 1 + Pass 2), extract estimated values:

```go
// After scan completion, persist estimated strictness to config
estimated := make(map[string]domain.StrictnessLevel)
for _, c := range scanResult.Clusters {
	if c.EstimatedStrictness != "" && c.Key != "" {
		level := domain.StrictnessLevel(c.EstimatedStrictness)
		if level.Valid() {
			estimated[c.Key] = level
		}
	}
}
if len(estimated) > 0 {
	cfgPath := filepath.Join(baseDir, domain.StateDir, "sightjack.yaml")
	if err := WriteEstimatedStrictness(cfgPath, estimated); err != nil {
		logger.Warn("Failed to write estimated strictness (non-fatal): %v", err)
	}
}
```

**Step 3: Verify existing tests pass**

Run: `cd /Users/nino/tap/sightjack && go test ./... -timeout 300s`

**Step 4: Commit**

```bash
git -C /Users/nino/tap/sightjack add internal/session/session.go internal/session/session_rescan.go
git -C /Users/nino/tap/sightjack commit -m "feat: persist estimated strictness to config after scan"
```

---

### Task 7: Templates — Redefine Strictness in Scan Templates

**Files:**
- Modify: `internal/platform/templates/scanner_classify_en.md.tmpl`
- Modify: `internal/platform/templates/scanner_classify_ja.md.tmpl`
- Modify: `internal/platform/templates/scanner_deepscan_en.md.tmpl`
- Modify: `internal/platform/templates/scanner_deepscan_ja.md.tmpl`

**Step 1: Update classifier templates (en + ja)**

Replace the Strictness section in `scanner_classify_en.md.tmpl`:

```markdown
## Strictness Level: {{.StrictnessLevel}}

Strictness defines change tolerance for existing implementations:
- fog: No protection for existing implementations. Free to restructure.
- alert: Respect existing implementations. Limit blast radius of changes.
- lockdown: Protect existing structure. Additive changes only.

DoD checking: ALWAYS full-depth regardless of Strictness level.
```

Add cluster key instruction to output section:

```markdown
## Output
Write the following JSON to **{{.OutputPath}}**:

    {
      "clusters": [
        {
          "name": "Display Name",
          "key": "english-slug-key",
          "issue_ids": ["issue-id-1", "issue-id-2"],
          "labels": ["label1", "label2"]
        }
      ],
      ...
    }

Each cluster must have both a "name" (display name, can be any language) and a "key" (lowercase English slug with hyphens, stable across scans).
```

**Step 2: Update deepscan templates (en + ja)**

Replace Strictness section. Add `status` and `estimated_strictness` to output:

```markdown
## Output
Write the following JSON to **{{.OutputPath}}**:

    {
      "name": "{{.ClusterName}}",
      "completeness": 0.35,
      "estimated_strictness": "fog|alert|lockdown",
      "strictness_reasoning": "Brief explanation of why this strictness level was estimated",
      "issues": [
        {
          "id": "issue-id",
          "identifier": "AWE-50",
          "title": "Issue Title",
          "status": "backlog|todo|in_progress|in_review|done|cancelled",
          "completeness": 0.4,
          "gaps": ["DoD missing", "No dependency specified"]
        }
      ],
      "observations": ["Cross-cluster findings"]
    }

For estimated_strictness, evaluate:
- Issue status distribution (many Done/In Progress → alert or lockdown)
- Code coupling and dependency complexity (tight coupling → lockdown)
- Implementation maturity (early stage → fog, production code → alert/lockdown)
```

**Step 3: Update ja templates with same changes (Japanese translations)**

**Step 4: Verify build**

Run: `cd /Users/nino/tap/sightjack && go build ./... && go vet ./...`

**Step 5: Commit**

```bash
git -C /Users/nino/tap/sightjack add internal/platform/templates/scanner_classify_en.md.tmpl internal/platform/templates/scanner_classify_ja.md.tmpl internal/platform/templates/scanner_deepscan_en.md.tmpl internal/platform/templates/scanner_deepscan_ja.md.tmpl
git -C /Users/nino/tap/sightjack commit -m "feat: redefine Strictness in scan templates + add key/status/estimation"
```

---

### Task 8: Templates — Redefine Strictness in Wave Templates

**Files:**
- Modify: `internal/platform/templates/wave_generate_en.md.tmpl`
- Modify: `internal/platform/templates/wave_generate_ja.md.tmpl`
- Modify: `internal/platform/templates/wave_nextgen_en.md.tmpl`
- Modify: `internal/platform/templates/wave_nextgen_ja.md.tmpl`

**Step 1: Update wave_generate templates (en + ja)**

Replace Strictness section + add cancel action type:

```markdown
## Strictness Level: {{.StrictnessLevel}}

Strictness defines change tolerance for existing implementations:
- fog: No protection. Cancel + rebuild freely per feedback.
- alert: Respect existing. Limit blast radius. Prefer sub-issues and dependency changes.
- lockdown: Protect structure. Additive changes only. Cancel only Backlog issues if unavoidable.

DoD checking: ALWAYS full-depth regardless of Strictness level.
```

Add `cancel` to action types:

```markdown
## Action Types
- `add_dod`: Append DoD items to the Issue description
- `add_dependency`: Set dependencies between Issues
- `add_label`: Add a label to an Issue
- `update_description`: Update an Issue description
- `create`: Create a new sub-issue
- `cancel`: Cancel an Issue (Backlog/Todo status only). Requires reason in detail field.
```

**Step 2: Update wave_nextgen templates (en + ja)**

Same Strictness redefinition. Add Feedback × Strictness guidelines:

```markdown
## Feedback × Strictness Guidelines

When feedback suggests fundamental changes:
- fog: cancel (Backlog/Todo) + create replacement issues
- alert: preserve existing issues, add sub-issues / update descriptions. Cancel Backlog/Todo as last resort
- lockdown: no cancel except Backlog. Use dependency and DoD additions to steer within existing structure

Note: cancel action requires issue status to be Backlog or Todo. Check the status field in Issue Analysis Results.
```

**Step 3: Update ja templates with Japanese translations**

**Step 4: Verify build**

**Step 5: Commit**

```bash
git -C /Users/nino/tap/sightjack add internal/platform/templates/wave_generate_en.md.tmpl internal/platform/templates/wave_generate_ja.md.tmpl internal/platform/templates/wave_nextgen_en.md.tmpl internal/platform/templates/wave_nextgen_ja.md.tmpl
git -C /Users/nino/tap/sightjack commit -m "feat: redefine Strictness in wave templates + add cancel action + feedback guidelines"
```

---

### Task 9: Templates — Redefine Strictness in Apply Templates

**Files:**
- Modify: `internal/platform/templates/wave_apply_en.md.tmpl`
- Modify: `internal/platform/templates/wave_apply_ja.md.tmpl`

**Step 1: Update apply templates**

Replace Strictness section. Add cancel action handling:

```markdown
## Application Steps
For each action:
1. `add_dod`: Append DoD items to the Issue description
2. `add_dependency`: Set Issue relationships via Linear MCP
3. `add_label`: Add a label via Linear MCP
4. `update_description`: Update the Issue description
5. `create`: Create a new sub-issue under the parent issue via Linear MCP
6. `cancel`: Cancel the Issue via Linear MCP. MUST:
   a. Verify issue status is Backlog or Todo (REJECT if In Progress or beyond)
   b. Add a comment with the cancellation reason (from action detail field)
   c. Set issue status to Cancelled
```

**Step 2: Update ja template**

**Step 3: Verify build**

**Step 4: Commit**

```bash
git -C /Users/nino/tap/sightjack add internal/platform/templates/wave_apply_en.md.tmpl internal/platform/templates/wave_apply_ja.md.tmpl
git -C /Users/nino/tap/sightjack commit -m "feat: add cancel action handling to apply templates"
```

---

### Task 10: Templates — Redefine Strictness in Discuss Templates

**Files:**
- Modify: `internal/platform/templates/auto_discuss_architect_en.md.tmpl`
- Modify: `internal/platform/templates/auto_discuss_architect_ja.md.tmpl`
- Modify: `internal/platform/templates/auto_discuss_devils_advocate_en.md.tmpl`
- Modify: `internal/platform/templates/auto_discuss_devils_advocate_ja.md.tmpl`
- Modify: `internal/platform/templates/architect_discuss_en.md.tmpl`
- Modify: `internal/platform/templates/architect_discuss_ja.md.tmpl`

**Step 1: Update all discuss templates**

Replace Strictness section with new definition (same text as Task 8). The Devil's Advocate template should additionally challenge:

```markdown
When reviewing wave proposals, also challenge:
- Does the proposed change respect the current Strictness level?
- If feedback suggests fundamental changes under alert/lockdown, are existing implementations adequately preserved?
- Are cancel actions justified and limited to Backlog/Todo issues?
```

**Step 2: Update all ja templates**

**Step 3: Verify build**

**Step 4: Commit**

```bash
git -C /Users/nino/tap/sightjack add internal/platform/templates/auto_discuss_architect_en.md.tmpl internal/platform/templates/auto_discuss_architect_ja.md.tmpl internal/platform/templates/auto_discuss_devils_advocate_en.md.tmpl internal/platform/templates/auto_discuss_devils_advocate_ja.md.tmpl internal/platform/templates/architect_discuss_en.md.tmpl internal/platform/templates/architect_discuss_ja.md.tmpl
git -C /Users/nino/tap/sightjack commit -m "feat: redefine Strictness in discuss templates + cancel challenge"
```

---

### Task 11: Domain — Add cancel to ValidActionTypes (if validation exists)

**Files:**
- Modify: `internal/domain/wave.go` or `internal/domain/types.go` (where action type validation lives)
- Test: add test for cancel action type

**Step 1: Check if action type validation exists**

Search for action type validation in domain layer. If none exists (current state: no domain validation, LLM-driven), add a `ValidWaveActionType` function:

```go
var validWaveActionTypes = map[string]bool{
	"add_dod":            true,
	"add_dependency":     true,
	"add_label":          true,
	"update_description": true,
	"create":             true,
	"cancel":             true, // NEW
}

func ValidWaveActionType(t string) bool {
	return validWaveActionTypes[t]
}
```

**Step 2: Write test**

```go
func TestValidWaveActionType(t *testing.T) {
	valid := []string{"add_dod", "add_dependency", "add_label", "update_description", "create", "cancel"}
	for _, v := range valid {
		if !ValidWaveActionType(v) {
			t.Errorf("expected %q to be valid", v)
		}
	}
	if ValidWaveActionType("delete") {
		t.Error("expected delete to be invalid")
	}
}
```

**Step 3: Commit**

```bash
git -C /Users/nino/tap/sightjack add internal/domain/wave.go internal/domain/wave_test.go
git -C /Users/nino/tap/sightjack commit -m "feat: add ValidWaveActionType with cancel support"
```

---

### Task 12: Integration Test — Estimated Strictness Resolution

**Files:**
- Create: `tests/integration/strictness_estimation_test.go`

**Step 1: Write test**

```go
func TestEstimatedStrictness_MaxWithDefault(t *testing.T) {
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessFog,
		Estimated: map[string]domain.StrictnessLevel{"auth-module": domain.StrictnessAlert},
	}
	got := domain.ResolveStrictness(cfg, []string{"auth-module"})
	if got != domain.StrictnessAlert {
		t.Errorf("expected alert, got %s", got)
	}
}

func TestEstimatedStrictness_OverrideWins(t *testing.T) {
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessFog,
		Overrides: map[string]domain.StrictnessLevel{"auth-module": domain.StrictnessLockdown},
		Estimated: map[string]domain.StrictnessLevel{"auth-module": domain.StrictnessAlert},
	}
	got := domain.ResolveStrictness(cfg, []string{"auth-module"})
	if got != domain.StrictnessLockdown {
		t.Errorf("expected lockdown, got %s", got)
	}
}

func TestEstimatedStrictness_DefaultStrongerThanEstimated(t *testing.T) {
	cfg := domain.StrictnessConfig{
		Default:   domain.StrictnessAlert,
		Estimated: map[string]domain.StrictnessLevel{"auth-module": domain.StrictnessFog},
	}
	got := domain.ResolveStrictness(cfg, []string{"auth-module"})
	if got != domain.StrictnessAlert {
		t.Errorf("expected alert (default stronger), got %s", got)
	}
}
```

**Step 2: Run tests**

Run: `cd /Users/nino/tap/sightjack && go test ./tests/integration/ -run TestEstimatedStrictness -v`

**Step 3: Commit**

```bash
git -C /Users/nino/tap/sightjack add tests/integration/strictness_estimation_test.go
git -C /Users/nino/tap/sightjack commit -m "test: add estimated strictness resolution integration tests"
```

---

### Task 13: ADR — Strictness Redefinition

**Files:**
- Create: `docs/adr/0012-strictness-redefinition.md`

Record:
- Strictness redefined from "DoD analysis depth" to "change tolerance for existing implementations"
- 3-layer resolution: max(override, estimated, default)
- Scan-time estimation via LLM (issue status distribution + code analysis)
- Estimated values persisted to config.yaml (git-tracked)
- cancel action with status-based eligibility
- Cluster key introduction for stable YAML keys

**Commit:**

```bash
git -C /Users/nino/tap/sightjack add docs/adr/0012-strictness-redefinition.md
git -C /Users/nino/tap/sightjack commit -m "docs: add ADR 0012 for Strictness redefinition"
```

---

### Task Dependency Graph

```
Task 1 (Key) ──┐
Task 2 (Status)─┼── Task 7 (Scan templates) ─── Task 8 (Wave templates) ─── Task 10 (Discuss templates)
Task 3 (Est.) ──┘                                    │
                                                     └── Task 9 (Apply templates)
Task 4 (3-layer) ── Task 5 (WriteEstimated) ── Task 6 (Integration point)
                                                     │
Task 11 (ValidActionType) ─────────────────── Task 12 (Integration tests)
                                                     │
                                                     └── Task 13 (ADR)
```
