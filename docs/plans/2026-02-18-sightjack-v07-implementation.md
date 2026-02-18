# Sightjack v0.7 — UX Enhancement Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement four UX features (matrix navigator, progress bar, "go back" to completed waves, rich ripple display) that bring the SIREN game feel to the CLI.

**Architecture:** Renderer-focused approach. All changes are in display/input functions (navigator.go, cli.go) with minimal session.go routing. No data model changes. Pure functions tested in isolation.

**Tech Stack:** Go, Pure ASCII rendering, bufio.Scanner input, existing session loop

---

### Task 1: RenderProgressBar

**Files:**
- Modify: `navigator.go` (add function)
- Modify: `navigator_test.go` (add tests)

**Step 1: Write the failing tests**

```go
func TestRenderProgressBar_Half(t *testing.T) {
	// given
	current := 0.50

	// when
	result := RenderProgressBar(current, 20)

	// then
	expected := "[==========..........] 50%"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestRenderProgressBar_Zero(t *testing.T) {
	// given / when
	result := RenderProgressBar(0.0, 20)

	// then
	expected := "[....................] 0%"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestRenderProgressBar_Full(t *testing.T) {
	// given / when
	result := RenderProgressBar(1.0, 20)

	// then
	expected := "[====================] 100%"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestRenderProgressBar_Partial(t *testing.T) {
	// given: 62% with width 20 -> 12.4 -> 12 filled
	result := RenderProgressBar(0.62, 20)

	// then
	expected := "[============........] 62%"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run TestRenderProgressBar -v`
Expected: FAIL — `RenderProgressBar` undefined

**Step 3: Implement RenderProgressBar**

In `navigator.go`:

```go
// RenderProgressBar produces an ASCII progress bar: [====....] NN%
func RenderProgressBar(current float64, width int) string {
	if width <= 0 {
		width = 20
	}
	filled := int(current * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("=", filled) + strings.Repeat(".", width-filled)
	return fmt.Sprintf("[%s] %d%%", bar, int(current*100))
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run TestRenderProgressBar -v`
Expected: PASS (all 4)

**Step 5: Commit**

```bash
git add navigator.go navigator_test.go
git commit -m "feat(v0.7): add RenderProgressBar utility function"
```

---

### Task 2: RenderMatrixNavigator — replace RenderNavigatorWithWaves

**Files:**
- Modify: `navigator.go` (rename + rewrite `RenderNavigatorWithWaves` to `RenderMatrixNavigator`)
- Modify: `navigator_test.go` (update all existing tests)
- Modify: `session.go` (update call site, line ~112)

**Step 1: Write the failing test for new matrix format**

Add in `navigator_test.go`:

```go
func TestRenderMatrixNavigator_GridBorders(t *testing.T) {
	// given
	result := &ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "Auth", Completeness: 0.65, Issues: make([]IssueDetail, 4)},
			{Name: "API", Completeness: 0.58, Issues: make([]IssueDetail, 6)},
		},
		TotalIssues:  10,
		Completeness: 0.615,
	}
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed"},
		{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "available"},
		{ID: "api-w1", ClusterName: "API", Title: "Split", Status: "completed"},
	}

	// when
	nav := RenderMatrixNavigator(result, "TestProject", waves, 4, nil, "fog", 0)

	// then: must contain grid border characters
	if !strings.Contains(nav, "+--") {
		t.Error("expected '+--' grid border")
	}
	if !strings.Contains(nav, "| Cluster") {
		t.Error("expected '| Cluster' header row")
	}
	if !strings.Contains(nav, "| W1") {
		t.Error("expected '| W1' column header")
	}
	if !strings.Contains(nav, "[=]") {
		t.Error("expected [=] for completed wave")
	}
	if !strings.Contains(nav, "[ ]") {
		t.Error("expected [ ] for available wave")
	}
	// Progress bar in footer
	if !strings.Contains(nav, "[=") && !strings.Contains(nav, "61%") {
		t.Error("expected progress bar in footer")
	}
}

func TestRenderMatrixNavigator_ProgressBarInFooter(t *testing.T) {
	// given
	result := &ScanResult{
		Clusters:     []ClusterScanResult{{Name: "Auth", Completeness: 0.50}},
		Completeness: 0.50,
	}
	waves := []Wave{{ID: "w1", ClusterName: "Auth", Title: "T", Status: "available"}}

	// when
	nav := RenderMatrixNavigator(result, "P", waves, 2, nil, "alert", 0)

	// then: footer line has progress bar + ADR + strictness
	if !strings.Contains(nav, "ADR: 2") {
		t.Error("expected ADR count in footer")
	}
	if !strings.Contains(nav, "Strictness: alert") {
		t.Error("expected strictness in footer")
	}
	if !strings.Contains(nav, "50%") {
		t.Error("expected 50% in progress bar")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestRenderMatrixNavigator -v`
Expected: FAIL — `RenderMatrixNavigator` undefined

**Step 3: Implement RenderMatrixNavigator**

In `navigator.go`, rename `RenderNavigatorWithWaves` to `RenderMatrixNavigator` and rewrite the body:

```go
const (
	matrixClusterCol = 20
	matrixWaveCol    = 8
	matrixCompCol    = 6
)

// RenderMatrixNavigator renders the Link Navigator as a Pure ASCII grid.
// Replaces the old RenderNavigatorWithWaves with proper table borders.
func RenderMatrixNavigator(result *ScanResult, projectName string, waves []Wave, adrCount int, lastScanned *time.Time, strictnessLevel string, shibitoCount int) string {
	wavesByCluster := make(map[string][]Wave)
	for _, w := range waves {
		wavesByCluster[w.ClusterName] = append(wavesByCluster[w.ClusterName], w)
	}

	var b strings.Builder

	// Header block (above grid)
	b.WriteString("  SIGHTJACK - Link Navigator\n")
	b.WriteString(fmt.Sprintf("  Project: %s\n", projectName))
	if lastScanned != nil {
		b.WriteString(fmt.Sprintf("  Session: resumed (last scan: %s)\n", lastScanned.Format("2006-01-02 15:04")))
	}
	if shibitoCount > 0 {
		b.WriteString(fmt.Sprintf("  Shibito: %d\n", shibitoCount))
	}
	b.WriteString("\n")

	// Grid border
	border := "+" + strings.Repeat("-", matrixClusterCol) + "+"
	for i := 0; i < waveColumns; i++ {
		border += strings.Repeat("-", matrixWaveCol) + "+"
	}
	border += strings.Repeat("-", matrixCompCol) + "+"

	// Header row
	b.WriteString(border + "\n")
	headerRow := "| " + padRight("Cluster", matrixClusterCol-1)
	for i := 1; i <= waveColumns; i++ {
		headerRow += "| " + padRight(fmt.Sprintf("W%d", i), matrixWaveCol-1)
	}
	headerRow += "| " + padRight("Comp", matrixCompCol-1) + "|"
	b.WriteString(headerRow + "\n")
	b.WriteString(border + "\n")

	// Data rows
	for _, cluster := range result.Clusters {
		pct := int(cluster.Completeness * 100)
		clusterWaves := wavesByCluster[cluster.Name]
		issueCount := len(cluster.Issues)
		nameCell := fmt.Sprintf("%s (%d)", truncate(cluster.Name, matrixClusterCol-6), issueCount)

		row := "| " + padRight(nameCell, matrixClusterCol-1)
		for i := 0; i < waveColumns; i++ {
			if i < len(clusterWaves) {
				cell := waveStatusSymbol3(clusterWaves[i].Status)
				row += "| " + padRight(cell, matrixWaveCol-1)
			} else {
				row += "|" + strings.Repeat(" ", matrixWaveCol)
			}
		}
		row += "| " + padRight(fmt.Sprintf("%d%%", pct), matrixCompCol-1) + "|"
		b.WriteString(row + "\n")
	}
	b.WriteString(border + "\n")

	// Legend
	b.WriteString("  [=] completed  [ ] available  [x] locked\n")
	b.WriteString("\n")

	// Footer: progress bar + metadata
	footer := "  " + RenderProgressBar(result.Completeness, 20)
	footer += fmt.Sprintf("  |  ADR: %d", adrCount)
	if strictnessLevel != "" {
		footer += fmt.Sprintf("  |  Strictness: %s", strictnessLevel)
	}
	b.WriteString(footer + "\n")

	return b.String()
}

// waveStatusSymbol3 returns a compact status string for matrix cells.
func waveStatusSymbol3(status string) string {
	switch status {
	case "available":
		return "[ ]"
	case "locked":
		return "[x]"
	case "completed":
		return "[=]"
	default:
		return "[?]"
	}
}
```

**Step 4: Update all call sites**

In `session.go` (line ~112), rename `RenderNavigatorWithWaves` to `RenderMatrixNavigator`.

**Step 5: Update all existing navigator tests**

All tests that call `RenderNavigatorWithWaves` must be renamed to `RenderMatrixNavigator`. The content assertions may need updating since the output format changed (grid borders instead of box borders).

Update each test:
- `TestRenderNavigatorWithWaves` -> update function name and assertions
- `TestRenderNavigatorWithWaves_CompletedWave` -> same
- `TestRenderNavigatorWithWaves_ADRCountZero` -> check footer format
- `TestRenderNavigatorWithWaves_ADRCountPositive` -> check footer format
- `TestRenderNavigatorWithWaves_ResumeInfo` -> check header format
- `TestRenderNavigatorWithWaves_NoResumeInfo` -> same
- `TestRenderNavigatorWithWaves_StrictnessBadge` -> check footer format
- `TestRenderNavigatorWithWaves_ShibitoCount` -> check header format
- `TestRenderNavigatorWithWaves_ShibitoZero_Hidden` -> same

**Step 6: Run all tests to verify they pass**

Run: `go test ./... -count=1`
Expected: PASS

**Step 7: Commit**

```bash
git add navigator.go navigator_test.go session.go
git commit -m "feat(v0.7): replace navigator with Pure ASCII matrix grid"
```

---

### Task 3: CompletedWaves helper + DisplayCompletedWaveActions

**Files:**
- Modify: `cli.go` (add 2 functions)
- Modify: `cli_test.go` (add tests)

**Step 1: Write the failing tests**

```go
func TestCompletedWaves_FiltersCompleted(t *testing.T) {
	// given
	waves := []Wave{
		{ID: "w1", ClusterName: "Auth", Title: "Deps", Status: "completed"},
		{ID: "w2", ClusterName: "Auth", Title: "DoD", Status: "available"},
		{ID: "w3", ClusterName: "API", Title: "Split", Status: "completed"},
	}

	// when
	result := CompletedWaves(waves)

	// then
	if len(result) != 2 {
		t.Fatalf("expected 2 completed, got %d", len(result))
	}
	if result[0].ID != "w1" {
		t.Errorf("expected w1, got %s", result[0].ID)
	}
	if result[1].ID != "w3" {
		t.Errorf("expected w3, got %s", result[1].ID)
	}
}

func TestCompletedWaves_NoneCompleted(t *testing.T) {
	// given
	waves := []Wave{
		{ID: "w1", Status: "available"},
		{ID: "w2", Status: "locked"},
	}

	// when
	result := CompletedWaves(waves)

	// then
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

func TestDisplayCompletedWaveActions_ShowsActions(t *testing.T) {
	// given
	var buf bytes.Buffer
	wave := Wave{
		ClusterName: "Auth",
		Title:       "DoD",
		Actions: []WaveAction{
			{Type: "add_dod", IssueID: "ENG-101", Description: "Auth flow"},
			{Type: "add_dependency", IssueID: "ENG-102", Description: "Token dep"},
		},
		Delta: WaveDelta{Before: 0.25, After: 0.40},
	}

	// when
	DisplayCompletedWaveActions(&buf, wave)

	// then
	output := buf.String()
	if !strings.Contains(output, "(completed)") {
		t.Error("expected (completed) label")
	}
	if !strings.Contains(output, "add_dod") {
		t.Error("expected action type")
	}
	if !strings.Contains(output, "ENG-101") {
		t.Error("expected issue ID")
	}
	if !strings.Contains(output, "Actions applied (2)") {
		t.Error("expected action count")
	}
}

func TestDisplayCompletedWaveActions_NoActions(t *testing.T) {
	// given
	var buf bytes.Buffer
	wave := Wave{ClusterName: "Auth", Title: "Empty"}

	// when
	DisplayCompletedWaveActions(&buf, wave)

	// then
	output := buf.String()
	if !strings.Contains(output, "Actions applied (0)") {
		t.Error("expected zero actions")
	}
}
```

Note: add `"bytes"` to imports in `cli_test.go`.

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run "TestCompletedWaves|TestDisplayCompletedWaveActions" -v`
Expected: FAIL — functions undefined

**Step 3: Implement CompletedWaves and DisplayCompletedWaveActions**

In `cli.go`:

```go
// CompletedWaves filters waves to only those with "completed" status.
func CompletedWaves(waves []Wave) []Wave {
	var result []Wave
	for _, w := range waves {
		if w.Status == "completed" {
			result = append(result, w)
		}
	}
	return result
}

// DisplayCompletedWaveActions shows the actions that were applied in a completed wave.
func DisplayCompletedWaveActions(w io.Writer, wave Wave) {
	fmt.Fprintf(w, "\n  --- %s - %s (completed) ---\n", wave.ClusterName, wave.Title)
	fmt.Fprintf(w, "  Actions applied (%d):\n", len(wave.Actions))
	for i, a := range wave.Actions {
		fmt.Fprintf(w, "    %d. [%s] %s: %s\n", i+1, a.Type, a.IssueID, a.Description)
	}
	if wave.Delta != (WaveDelta{}) {
		fmt.Fprintf(w, "\n  Result: %.0f%% -> %.0f%%\n", wave.Delta.Before*100, wave.Delta.After*100)
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run "TestCompletedWaves|TestDisplayCompletedWaveActions" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cli.go cli_test.go
git commit -m "feat(v0.7): add CompletedWaves filter and DisplayCompletedWaveActions"
```

---

### Task 4: PromptCompletedWaveSelection

**Files:**
- Modify: `cli.go` (add function)
- Modify: `cli_test.go` (add tests)

**Step 1: Write the failing tests**

```go
func TestPromptCompletedWaveSelection_ValidChoice(t *testing.T) {
	// given
	var buf bytes.Buffer
	input := strings.NewReader("2\n")
	scanner := bufio.NewScanner(input)
	completed := []Wave{
		{ID: "w1", ClusterName: "Auth", Title: "Deps", Delta: WaveDelta{Before: 0.25, After: 0.40}},
		{ID: "w3", ClusterName: "API", Title: "Split", Delta: WaveDelta{Before: 0.30, After: 0.45}},
	}

	// when
	selected, err := PromptCompletedWaveSelection(context.Background(), &buf, scanner, completed)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.ID != "w3" {
		t.Errorf("expected w3, got %s", selected.ID)
	}
}

func TestPromptCompletedWaveSelection_Quit(t *testing.T) {
	// given
	var buf bytes.Buffer
	input := strings.NewReader("q\n")
	scanner := bufio.NewScanner(input)
	completed := []Wave{{ID: "w1", ClusterName: "Auth", Title: "Deps"}}

	// when
	_, err := PromptCompletedWaveSelection(context.Background(), &buf, scanner, completed)

	// then
	if err != ErrQuit {
		t.Errorf("expected ErrQuit, got %v", err)
	}
}

func TestPromptCompletedWaveSelection_Invalid(t *testing.T) {
	// given
	var buf bytes.Buffer
	input := strings.NewReader("99\n")
	scanner := bufio.NewScanner(input)
	completed := []Wave{{ID: "w1", ClusterName: "Auth", Title: "Deps"}}

	// when
	_, err := PromptCompletedWaveSelection(context.Background(), &buf, scanner, completed)

	// then
	if err == nil || err == ErrQuit {
		t.Error("expected invalid selection error")
	}
}
```

Note: add `"bufio"`, `"context"` to cli_test.go imports if not present.

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run TestPromptCompletedWaveSelection -v`
Expected: FAIL — function undefined

**Step 3: Implement PromptCompletedWaveSelection**

In `cli.go`:

```go
// PromptCompletedWaveSelection displays completed waves and reads the user's choice.
func PromptCompletedWaveSelection(ctx context.Context, w io.Writer, s *bufio.Scanner, completed []Wave) (Wave, error) {
	fmt.Fprintln(w, "\n  Completed waves:")
	for i, wave := range completed {
		fmt.Fprintf(w, "    %d. %-6s W: %-20s (%2.0f%% -> %2.0f%%)\n",
			i+1, wave.ClusterName, wave.Title,
			wave.Delta.Before*100, wave.Delta.After*100)
	}
	fmt.Fprintf(w, "\n  Select [1-%d, q=back]: ", len(completed))

	line, err := ScanLine(ctx, s)
	if err != nil {
		return Wave{}, ErrQuit
	}
	input := strings.TrimSpace(line)
	if input == "q" {
		return Wave{}, ErrQuit
	}
	num, parseErr := strconv.Atoi(input)
	if parseErr != nil || num < 1 || num > len(completed) {
		return Wave{}, fmt.Errorf("invalid selection: %s", input)
	}
	return completed[num-1], nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run TestPromptCompletedWaveSelection -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cli.go cli_test.go
git commit -m "feat(v0.7): add PromptCompletedWaveSelection for go-back experience"
```

---

### Task 5: DisplayWaveCompletion — rich ripple display

**Files:**
- Modify: `cli.go` (add function)
- Modify: `cli_test.go` (add tests)

**Step 1: Write the failing tests**

```go
func TestDisplayWaveCompletion_Basic(t *testing.T) {
	// given
	var buf bytes.Buffer
	wave := Wave{ClusterName: "Auth", Title: "DoD", Delta: WaveDelta{Before: 0.40, After: 0.65}}
	ripples := []Ripple{
		{ClusterName: "DB", Description: "Wave 2 unlocked"},
		{ClusterName: "DB", Description: "Schema dependency added"},
		{ClusterName: "API", Description: "DoD updated: token validation"},
	}

	// when
	DisplayWaveCompletion(&buf, wave, ripples, 0.52, 2)

	// then
	output := buf.String()
	if !strings.Contains(output, "Auth") {
		t.Error("expected cluster name")
	}
	if !strings.Contains(output, "DoD") {
		t.Error("expected wave title")
	}
	if !strings.Contains(output, "40%") {
		t.Error("expected before completeness")
	}
	if !strings.Contains(output, "65%") {
		t.Error("expected after completeness")
	}
	// Grouped by cluster
	if !strings.Contains(output, "DB:") {
		t.Error("expected DB cluster group header")
	}
	if !strings.Contains(output, "API:") {
		t.Error("expected API cluster group header")
	}
	if !strings.Contains(output, "New waves available: 2") {
		t.Error("expected new waves count")
	}
	if !strings.Contains(output, "52%") {
		t.Error("expected overall completeness in progress bar")
	}
}

func TestDisplayWaveCompletion_NoRipples(t *testing.T) {
	// given
	var buf bytes.Buffer
	wave := Wave{ClusterName: "Auth", Title: "Deps", Delta: WaveDelta{Before: 0.25, After: 0.40}}

	// when
	DisplayWaveCompletion(&buf, wave, nil, 0.36, 0)

	// then
	output := buf.String()
	if strings.Contains(output, "Ripple") {
		t.Error("should not show ripple section when empty")
	}
	if strings.Contains(output, "New waves") {
		t.Error("should not show new waves when 0")
	}
}

func TestDisplayWaveCompletion_ZeroNewWaves(t *testing.T) {
	// given
	var buf bytes.Buffer
	wave := Wave{ClusterName: "Auth", Title: "DoD", Delta: WaveDelta{Before: 0.40, After: 0.65}}
	ripples := []Ripple{{ClusterName: "DB", Description: "Updated"}}

	// when
	DisplayWaveCompletion(&buf, wave, ripples, 0.50, 0)

	// then
	output := buf.String()
	if strings.Contains(output, "New waves") {
		t.Error("should not show new waves when 0")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./... -run TestDisplayWaveCompletion -v`
Expected: FAIL — function undefined

**Step 3: Implement DisplayWaveCompletion**

In `cli.go`:

```go
// DisplayWaveCompletion shows a rich wave completion summary with grouped ripple effects.
func DisplayWaveCompletion(w io.Writer, wave Wave, ripples []Ripple, overallCompleteness float64, newWavesAvailable int) {
	fmt.Fprintf(w, "\n  Wave completed: %s - %s\n", wave.ClusterName, wave.Title)
	fmt.Fprintf(w, "  Completeness: %.0f%% -> %.0f%%\n", wave.Delta.Before*100, wave.Delta.After*100)

	if len(ripples) > 0 {
		fmt.Fprintln(w, "\n  Ripple effects:")
		// Group by cluster
		grouped := make(map[string][]Ripple)
		var order []string
		for _, r := range ripples {
			if _, seen := grouped[r.ClusterName]; !seen {
				order = append(order, r.ClusterName)
			}
			grouped[r.ClusterName] = append(grouped[r.ClusterName], r)
		}
		for _, name := range order {
			fmt.Fprintf(w, "    %s:\n", name)
			for _, r := range grouped[name] {
				fmt.Fprintf(w, "      -> %s\n", r.Description)
			}
		}
	}

	if newWavesAvailable > 0 {
		fmt.Fprintf(w, "\n  New waves available: %d\n", newWavesAvailable)
	}

	fmt.Fprintf(w, "  %s\n", RenderProgressBar(overallCompleteness, 20))
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -run TestDisplayWaveCompletion -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cli.go cli_test.go
git commit -m "feat(v0.7): add DisplayWaveCompletion with grouped ripple effects"
```

---

### Task 6: PromptWaveSelection [b] option

**Files:**
- Modify: `cli.go` (update `PromptWaveSelection`)
- Modify: `cli_test.go` (add test for `b` input)

**Step 1: Write the failing test**

```go
func TestPromptWaveSelection_BackOption(t *testing.T) {
	// given
	var buf bytes.Buffer
	input := strings.NewReader("b\n")
	scanner := bufio.NewScanner(input)
	waves := []Wave{{ID: "w1", ClusterName: "Auth", Title: "Deps"}}

	// when
	_, err := PromptWaveSelection(context.Background(), &buf, scanner, waves)

	// then
	if err != ErrGoBack {
		t.Errorf("expected ErrGoBack, got %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestPromptWaveSelection_BackOption -v`
Expected: FAIL — `ErrGoBack` undefined

**Step 3: Add ErrGoBack and update PromptWaveSelection**

In `cli.go`:

```go
// ErrGoBack signals the user chose to go back to completed waves.
var ErrGoBack = errors.New("go back")
```

Update `PromptWaveSelection` prompt text from:
```go
fmt.Fprintf(w, "\nSelect wave [1-%d, q=quit]: ", len(waves))
```
to:
```go
fmt.Fprintf(w, "\nSelect wave [1-%d, b=back, q=quit]: ", len(waves))
```

Add `"b"` case before the `strconv.Atoi`:
```go
if input == "b" {
    return Wave{}, ErrGoBack
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./... -count=1`
Expected: PASS (all tests including existing PromptWaveSelection tests)

**Step 5: Commit**

```bash
git add cli.go cli_test.go
git commit -m "feat(v0.7): add [b] back option to PromptWaveSelection"
```

---

### Task 7: Wire "go back" + DisplayWaveCompletion into session.go

**Files:**
- Modify: `session.go` (update `runInteractiveLoop`)

**Step 1: Update the interactive loop in `runInteractiveLoop`**

After the `PromptWaveSelection` call (~line 123), add handling for `ErrGoBack`:

```go
// Prompt wave selection
selected, err := PromptWaveSelection(ctx, os.Stdout, scanner, available)
if err == ErrQuit {
    LogInfo("Session paused. State saved.")
    break
}
if err == ErrGoBack {
    completed := CompletedWaves(waves)
    if len(completed) == 0 {
        LogInfo("No completed waves to revisit.")
        continue
    }
    revisit, backErr := PromptCompletedWaveSelection(ctx, os.Stdout, scanner, completed)
    if backErr != nil {
        continue
    }
    DisplayCompletedWaveActions(os.Stdout, revisit)
    // Enter discuss loop for the completed wave
    // (reuses existing approval prompt logic for discuss only)
    for {
        fmt.Fprint(os.Stdout, "\n  [d] Discuss modifications  [q] Back to navigator: ")
        line, lineErr := ScanLine(ctx, scanner)
        if lineErr != nil {
            break
        }
        choice := strings.TrimSpace(strings.ToLower(line))
        if choice == "q" {
            break
        }
        if choice != "d" {
            LogWarn("Invalid input: %s", choice)
            continue
        }
        topic, topicErr := PromptDiscussTopic(ctx, os.Stdout, scanner)
        if topicErr != nil {
            continue
        }
        result, discussErr := RunArchitectDiscuss(ctx, cfg, scanDir, revisit, topic)
        if discussErr != nil {
            LogError("Architect discussion failed: %v", discussErr)
            continue
        }
        DisplayArchitectResponse(os.Stdout, result)
        if result.ModifiedWave != nil {
            revisit = ApplyModifiedWave(revisit, *result.ModifiedWave, completed_map)
            PropagateWaveUpdate(waves, revisit)
            if cfg.Scribe.Enabled {
                scribeResp, scribeErr := RunScribeADR(ctx, cfg, scanDir, revisit, result, adrDir)
                if scribeErr != nil {
                    LogWarn("Scribe failed (non-fatal): %v", scribeErr)
                } else {
                    DisplayScribeResponse(os.Stdout, scribeResp)
                    DisplayADRConflicts(os.Stdout, scribeResp.Conflicts)
                    adrCount++
                }
            }
        }
        continue
    }
    continue
}
```

Note: `completed_map` above refers to the `completed` variable in the function scope. Use `completed` directly.

**Step 2: Replace `DisplayRippleEffects` + `LogOK` at wave completion with `DisplayWaveCompletion`**

Around line ~197-233, after marking wave completed and recalculating completeness:

Replace:
```go
// Display ripple effects
DisplayRippleEffects(os.Stdout, applyResult.Ripples)
// ... (completion bookkeeping) ...
LogOK("Completeness: %.0f%%", scanResult.Completeness*100)
```

With:
```go
// Display ripple effects
// Count available waves before/after to show "new waves available"
oldAvailable := len(AvailableWaves(waves, completed))
// ... (completion bookkeeping — mark completed, update clusters, recalculate) ...
waves = EvaluateUnlocks(waves, completed)
newAvailable := len(AvailableWaves(waves, completed))
newCount := newAvailable - oldAvailable
if newCount < 0 {
    newCount = 0
}
DisplayWaveCompletion(os.Stdout, selected, applyResult.Ripples, scanResult.Completeness, newCount)
```

**Step 3: Run all tests**

Run: `go test ./... -count=1`
Expected: PASS

**Step 4: Commit**

```bash
git add session.go
git commit -m "feat(v0.7): wire go-back and DisplayWaveCompletion into session loop"
```

---

### Task 8: Version bump to 0.7.0-dev

**Files:**
- Modify: `cmd/sightjack/main.go` (version string)
- Modify: `session.go` (`BuildSessionState` version)
- Modify: `session_test.go` (version assertion)

**Step 1: Update version**

In `cmd/sightjack/main.go`:
```go
var version = "0.7.0-dev"
```

In `session.go` `BuildSessionState`:
```go
Version: "0.7",
```

In `session_test.go`, update any version checks from `"0.6"` to `"0.7"`.

**Step 2: Run all tests**

Run: `go test ./... -count=1`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/sightjack/main.go session.go session_test.go
git commit -m "feat(v0.7): bump version to 0.7.0-dev"
```

---

## Summary

| Task | Description | Est. Lines |
|------|-------------|------------|
| 1 | RenderProgressBar | ~30 |
| 2 | RenderMatrixNavigator (replace old navigator) | ~120 |
| 3 | CompletedWaves + DisplayCompletedWaveActions | ~50 |
| 4 | PromptCompletedWaveSelection | ~40 |
| 5 | DisplayWaveCompletion (rich ripple) | ~60 |
| 6 | PromptWaveSelection [b] option | ~15 |
| 7 | Wire into session.go | ~60 |
| 8 | Version bump | ~5 |
| **Total** | | **~380** |
