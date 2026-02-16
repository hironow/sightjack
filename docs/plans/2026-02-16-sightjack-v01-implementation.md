# Sightjack v0.1 Skeleton Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the minimal sightjack skeleton that scans Linear Issues via Claude Code, classifies them into Clusters, and displays a Link Navigator matrix in the terminal.

**Architecture:** Go orchestrator launches Claude Code subprocesses that interact with Linear via MCP. File-based communication: Claude writes JSON results to specified paths, Go reads and renders. Two-pass scanning (classify then deep-scan per cluster).

**Tech Stack:** Go 1.26, `gopkg.in/yaml.v3` for config, standard library for everything else. Claude Code CLI as subprocess. Linear MCP Server for Issue access.

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `justfile`
- Modify: `.gitignore`

**Step 1: Initialize Go module**

```bash
cd /Users/nino/sightjack && go mod init github.com/hironow/sightjack
```

**Step 2: Add yaml dependency**

```bash
cd /Users/nino/sightjack && go get gopkg.in/yaml.v3
```

**Step 3: Create justfile**

Create `justfile`:

```just
# sightjack task runner

# Run all tests
test:
    go test ./...

# Run tests with verbose output
test-v:
    go test -v ./...

# Run tests with coverage
test-cov:
    go test -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out

# Build binary
build:
    go build -o sightjack ./cmd/sightjack

# Run linter
lint:
    go vet ./...

# Format code
fmt:
    gofmt -w .

# Clean build artifacts
clean:
    rm -f sightjack coverage.out
```

**Step 4: Update .gitignore**

Append to `.gitignore`:

```
# sightjack runtime state
.sightjack/
```

**Step 5: Commit**

```bash
git add go.mod go.sum justfile .gitignore
git commit -m "feat: initialize Go module and project scaffolding"
```

---

### Task 2: Domain Model (model.go)

**Files:**
- Create: `model.go`
- Create: `model_test.go`

**Step 1: Write failing test for ClassifyResult JSON unmarshaling**

Create `model_test.go`:

```go
package sightjack

import (
	"encoding/json"
	"testing"
)

func TestClassifyResult_UnmarshalJSON(t *testing.T) {
	// given
	raw := `{
		"clusters": [
			{"name": "Auth", "issue_ids": ["ID1", "ID2"]},
			{"name": "API", "issue_ids": ["ID3"]}
		],
		"total_issues": 3
	}`

	// when
	var result ClassifyResult
	err := json.Unmarshal([]byte(raw), &result)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(result.Clusters))
	}
	if result.Clusters[0].Name != "Auth" {
		t.Errorf("expected Auth, got %s", result.Clusters[0].Name)
	}
	if len(result.Clusters[0].IssueIDs) != 2 {
		t.Errorf("expected 2 issue IDs, got %d", len(result.Clusters[0].IssueIDs))
	}
	if result.TotalIssues != 3 {
		t.Errorf("expected 3 total issues, got %d", result.TotalIssues)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/nino/sightjack && go test -run TestClassifyResult -v
```

Expected: FAIL — `ClassifyResult` not defined.

**Step 3: Write minimal model.go**

Create `model.go`:

```go
package sightjack

import "time"

// ClassifyResult is the output of Pass 1 (cluster classification).
// Written by Claude Code to classify.json.
type ClassifyResult struct {
	Clusters    []ClusterClassification `json:"clusters"`
	TotalIssues int                     `json:"total_issues"`
}

// ClusterClassification holds a cluster name and its issue IDs from Pass 1.
type ClusterClassification struct {
	Name     string   `json:"name"`
	IssueIDs []string `json:"issue_ids"`
}

// ClusterScanResult is the output of Pass 2 (per-cluster deep scan).
// Written by Claude Code to cluster_{name}.json.
type ClusterScanResult struct {
	Name         string      `json:"name"`
	Completeness float64     `json:"completeness"`
	Issues       []IssueDetail `json:"issues"`
	Observations []string    `json:"observations"`
}

// IssueDetail holds the deep scan analysis of a single issue.
type IssueDetail struct {
	ID           string   `json:"id"`
	Identifier   string   `json:"identifier"`
	Title        string   `json:"title"`
	Completeness float64  `json:"completeness"`
	Gaps         []string `json:"gaps"`
}

// ScanResult is the merged result of Pass 1 + Pass 2.
type ScanResult struct {
	Clusters     []ClusterScanResult
	TotalIssues  int
	Completeness float64
	Observations []string
}

// SessionState is the thin state file persisted to .sightjack/state.json.
type SessionState struct {
	Version      string         `json:"version"`
	SessionID    string         `json:"session_id"`
	Project      string         `json:"project"`
	LastScanned  time.Time      `json:"last_scanned"`
	Completeness float64        `json:"completeness"`
	Clusters     []ClusterState `json:"clusters"`
}

// ClusterState is the per-cluster state within SessionState.
type ClusterState struct {
	Name         string  `json:"name"`
	Completeness float64 `json:"completeness"`
	IssueCount   int     `json:"issue_count"`
}
```

**Step 4: Run test to verify it passes**

```bash
cd /Users/nino/sightjack && go test -run TestClassifyResult -v
```

Expected: PASS

**Step 5: Write failing test for ClusterScanResult**

Add to `model_test.go`:

```go
func TestClusterScanResult_UnmarshalJSON(t *testing.T) {
	// given
	raw := `{
		"name": "Auth",
		"completeness": 0.35,
		"issues": [
			{
				"id": "abc-123",
				"identifier": "AWE-50",
				"title": "Implement login",
				"completeness": 0.4,
				"gaps": ["DoD missing", "No dependency specified"]
			}
		],
		"observations": ["Hidden dependency on API cluster"]
	}`

	// when
	var result ClusterScanResult
	err := json.Unmarshal([]byte(raw), &result)

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
	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}
	if result.Issues[0].Identifier != "AWE-50" {
		t.Errorf("expected AWE-50, got %s", result.Issues[0].Identifier)
	}
	if len(result.Issues[0].Gaps) != 2 {
		t.Errorf("expected 2 gaps, got %d", len(result.Issues[0].Gaps))
	}
}
```

**Step 6: Run test to verify it passes** (types already defined)

```bash
cd /Users/nino/sightjack && go test -run TestClusterScanResult -v
```

Expected: PASS

**Step 7: Write failing test for ScanResult.CalculateCompleteness**

Add to `model_test.go`:

```go
func TestScanResult_CalculateCompleteness(t *testing.T) {
	// given
	result := ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "Auth", Completeness: 0.25, Issues: make([]IssueDetail, 5)},
			{Name: "API", Completeness: 0.40, Issues: make([]IssueDetail, 5)},
		},
	}

	// when
	result.CalculateCompleteness()

	// then
	expected := 0.325 // (0.25 + 0.40) / 2
	if result.Completeness != expected {
		t.Errorf("expected %f, got %f", expected, result.Completeness)
	}
	if result.TotalIssues != 10 {
		t.Errorf("expected 10 total issues, got %d", result.TotalIssues)
	}
}
```

**Step 8: Run test to verify it fails**

```bash
cd /Users/nino/sightjack && go test -run TestScanResult_CalculateCompleteness -v
```

Expected: FAIL — `CalculateCompleteness` not defined.

**Step 9: Implement CalculateCompleteness**

Add to `model.go`:

```go
// CalculateCompleteness computes overall completeness as the average of cluster completeness values,
// and tallies total issues across all clusters.
func (r *ScanResult) CalculateCompleteness() {
	if len(r.Clusters) == 0 {
		return
	}
	var sum float64
	var total int
	for _, c := range r.Clusters {
		sum += c.Completeness
		total += len(c.Issues)
	}
	r.Completeness = sum / float64(len(r.Clusters))
	r.TotalIssues = total
}
```

**Step 10: Run all tests**

```bash
cd /Users/nino/sightjack && just test-v
```

Expected: All PASS

**Step 11: Commit**

```bash
git add model.go model_test.go
git commit -m "feat: add domain model types with JSON unmarshaling and completeness calculation"
```

---

### Task 3: Configuration (config.go)

**Files:**
- Create: `config.go`
- Create: `config_test.go`

**Step 1: Write failing test for config parsing**

Create `config_test.go`:

```go
package sightjack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// given
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
linear:
  team: "TEST-TEAM"
  project: "Test Project"
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// when
	cfg, err := LoadConfig(cfgPath)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Linear.Team != "TEST-TEAM" {
		t.Errorf("expected TEST-TEAM, got %s", cfg.Linear.Team)
	}
	if cfg.Linear.Project != "Test Project" {
		t.Errorf("expected Test Project, got %s", cfg.Linear.Project)
	}
	// defaults
	if cfg.Scan.ChunkSize != 20 {
		t.Errorf("expected default chunk_size 20, got %d", cfg.Scan.ChunkSize)
	}
	if cfg.Scan.MaxConcurrency != 3 {
		t.Errorf("expected default max_concurrency 3, got %d", cfg.Scan.MaxConcurrency)
	}
	if cfg.Claude.Command != "claude" {
		t.Errorf("expected default command 'claude', got %s", cfg.Claude.Command)
	}
	if cfg.Claude.TimeoutSec != 300 {
		t.Errorf("expected default timeout 300, got %d", cfg.Claude.TimeoutSec)
	}
	if cfg.Lang != "ja" {
		t.Errorf("expected default lang 'ja', got %s", cfg.Lang)
	}
}

func TestLoadConfig_FullOverride(t *testing.T) {
	// given
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sightjack.yaml")
	err := os.WriteFile(cfgPath, []byte(`
linear:
  team: "MY-TEAM"
  project: "My Project"
  cycle: "Sprint 5"
scan:
  chunk_size: 50
  max_concurrency: 5
claude:
  command: "cc-p"
  model: "sonnet"
  timeout_sec: 600
lang: "en"
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// when
	cfg, err := LoadConfig(cfgPath)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Scan.ChunkSize != 50 {
		t.Errorf("expected 50, got %d", cfg.Scan.ChunkSize)
	}
	if cfg.Claude.Model != "sonnet" {
		t.Errorf("expected sonnet, got %s", cfg.Claude.Model)
	}
	if cfg.Lang != "en" {
		t.Errorf("expected en, got %s", cfg.Lang)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	// when
	_, err := LoadConfig("/nonexistent/path.yaml")

	// then
	if err == nil {
		t.Error("expected error for missing config file")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/nino/sightjack && go test -run TestLoadConfig -v
```

Expected: FAIL — `LoadConfig` not defined.

**Step 3: Implement config.go**

Create `config.go`:

```go
package sightjack

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all sightjack configuration from sightjack.yaml.
type Config struct {
	Linear LinearConfig `yaml:"linear"`
	Scan   ScanConfig   `yaml:"scan"`
	Claude ClaudeConfig `yaml:"claude"`
	Lang   string       `yaml:"lang"`
}

// LinearConfig holds Linear API filter settings.
type LinearConfig struct {
	Team    string `yaml:"team"`
	Project string `yaml:"project"`
	Cycle   string `yaml:"cycle"`
}

// ScanConfig holds scanning behavior settings.
type ScanConfig struct {
	ChunkSize      int `yaml:"chunk_size"`
	MaxConcurrency int `yaml:"max_concurrency"`
}

// ClaudeConfig holds Claude Code subprocess settings.
type ClaudeConfig struct {
	Command    string `yaml:"command"`
	Model      string `yaml:"model"`
	TimeoutSec int    `yaml:"timeout_sec"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Scan: ScanConfig{
			ChunkSize:      20,
			MaxConcurrency: 3,
		},
		Claude: ClaudeConfig{
			Command:    "claude",
			Model:      "opus",
			TimeoutSec: 300,
		},
		Lang: "ja",
	}
}

// LoadConfig reads and parses a sightjack.yaml file.
// Missing fields are filled with defaults.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}
```

**Step 4: Run tests**

```bash
cd /Users/nino/sightjack && go test -run TestLoadConfig -v
```

Expected: All PASS

**Step 5: Commit**

```bash
git add config.go config_test.go
git commit -m "feat: add YAML config loading with defaults"
```

---

### Task 4: Logger (logger.go)

**Files:**
- Create: `logger.go`
- Create: `logger_test.go`

**Step 1: Write failing test for log formatting**

Create `logger_test.go`:

```go
package sightjack

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogLine_Format(t *testing.T) {
	// given
	var buf bytes.Buffer

	// when
	formatLogLine(&buf, "INFO", "", "hello %s", "world")

	// then
	line := buf.String()
	if !strings.Contains(line, "INFO") {
		t.Errorf("expected INFO prefix, got: %s", line)
	}
	if !strings.Contains(line, "hello world") {
		t.Errorf("expected 'hello world', got: %s", line)
	}
	// timestamp format [HH:MM:SS]
	if line[0] != '[' {
		t.Errorf("expected timestamp prefix, got: %s", line)
	}
}

func TestLogLine_WithColor(t *testing.T) {
	// given
	var buf bytes.Buffer

	// when
	formatLogLine(&buf, " OK ", colorGreen, "success")

	// then
	line := buf.String()
	if !strings.Contains(line, colorGreen) {
		t.Errorf("expected green color code in output")
	}
	if !strings.Contains(line, colorReset) {
		t.Errorf("expected color reset in output")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/nino/sightjack && go test -run TestLogLine -v
```

Expected: FAIL — `formatLogLine` not defined.

**Step 3: Implement logger.go**

Create `logger.go`:

```go
package sightjack

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
)

var (
	logMu   sync.Mutex
	logFile *os.File
)

// InitLogFile opens a log file for dual output (console + file).
func InitLogFile(path string) error {
	logMu.Lock()
	defer logMu.Unlock()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	logFile = f
	return nil
}

// CloseLogFile closes the log file.
func CloseLogFile() {
	logMu.Lock()
	defer logMu.Unlock()
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

// formatLogLine writes a formatted log line to the given writer.
func formatLogLine(w io.Writer, prefix, color string, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format("15:04:05")
	if color != "" {
		fmt.Fprintf(w, "[%s] %s%s%s %s\n", ts, color, prefix, colorReset, msg)
	} else {
		fmt.Fprintf(w, "[%s] %s %s\n", ts, prefix, msg)
	}
}

func logLine(prefix, color string, format string, args ...any) {
	// Console output (with color)
	formatLogLine(os.Stdout, prefix, color, format, args...)

	// File output (without color)
	logMu.Lock()
	defer logMu.Unlock()
	if logFile != nil {
		msg := fmt.Sprintf(format, args...)
		ts := time.Now().Format("15:04:05")
		fmt.Fprintf(logFile, "[%s] %s %s\n", ts, prefix, msg)
	}
}

// LogInfo logs an informational message in cyan.
func LogInfo(format string, args ...any) {
	logLine("INFO", colorCyan, format, args...)
}

// LogOK logs a success message in green.
func LogOK(format string, args ...any) {
	logLine(" OK ", colorGreen, format, args...)
}

// LogWarn logs a warning message in yellow.
func LogWarn(format string, args ...any) {
	logLine("WARN", colorYellow, format, args...)
}

// LogError logs an error message in red.
func LogError(format string, args ...any) {
	logLine(" ERR", colorRed, format, args...)
}

// LogScan logs a scan-related message in blue.
func LogScan(format string, args ...any) {
	logLine("SCAN", colorBlue, format, args...)
}

// LogNav logs a navigator-related message in purple.
func LogNav(format string, args ...any) {
	logLine(" NAV", colorPurple, format, args...)
}
```

**Step 4: Run tests**

```bash
cd /Users/nino/sightjack && go test -run TestLogLine -v
```

Expected: All PASS

**Step 5: Commit**

```bash
git add logger.go logger_test.go
git commit -m "feat: add colored dual-output logger"
```

---

### Task 5: State Management (state.go)

**Files:**
- Create: `state.go`
- Create: `state_test.go`

**Step 1: Write failing test for state write/read round-trip**

Create `state_test.go`:

```go
package sightjack

import (
	"path/filepath"
	"testing"
	"time"
)

func TestState_WriteAndRead_RoundTrip(t *testing.T) {
	// given
	dir := t.TempDir()
	state := SessionState{
		Version:      "0.1",
		SessionID:    "test-session-123",
		Project:      "Test Project",
		LastScanned:  time.Date(2026, 2, 16, 10, 0, 0, 0, time.UTC),
		Completeness: 0.32,
		Clusters: []ClusterState{
			{Name: "Auth", Completeness: 0.25, IssueCount: 5},
			{Name: "API", Completeness: 0.40, IssueCount: 8},
		},
	}

	// when
	err := WriteState(dir, &state)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	loaded, err := ReadState(dir)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	// then
	if loaded.SessionID != "test-session-123" {
		t.Errorf("expected session ID test-session-123, got %s", loaded.SessionID)
	}
	if loaded.Completeness != 0.32 {
		t.Errorf("expected 0.32, got %f", loaded.Completeness)
	}
	if len(loaded.Clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(loaded.Clusters))
	}
	if loaded.Clusters[0].Name != "Auth" {
		t.Errorf("expected Auth, got %s", loaded.Clusters[0].Name)
	}
}

func TestState_ReadMissing_ReturnsError(t *testing.T) {
	// given
	dir := t.TempDir()

	// when
	_, err := ReadState(dir)

	// then
	if err == nil {
		t.Error("expected error for missing state file")
	}
}

func TestStatePath(t *testing.T) {
	// when
	path := StatePath("/project")

	// then
	expected := filepath.Join("/project", ".sightjack", "state.json")
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/nino/sightjack && go test -run TestState -v
```

Expected: FAIL

**Step 3: Implement state.go**

Create `state.go`:

```go
package sightjack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const stateDir = ".sightjack"
const stateFile = "state.json"

// StatePath returns the full path to the state file.
func StatePath(baseDir string) string {
	return filepath.Join(baseDir, stateDir, stateFile)
}

// ScanDir returns the scan output directory for a given session.
func ScanDir(baseDir, sessionID string) string {
	return filepath.Join(baseDir, stateDir, "scans", sessionID)
}

// WriteState persists a SessionState to .sightjack/state.json.
func WriteState(baseDir string, state *SessionState) error {
	dir := filepath.Join(baseDir, stateDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	path := StatePath(baseDir)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

// ReadState loads a SessionState from .sightjack/state.json.
func ReadState(baseDir string) (*SessionState, error) {
	data, err := os.ReadFile(StatePath(baseDir))
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	return &state, nil
}

// EnsureScanDir creates the scan output directory for a session.
func EnsureScanDir(baseDir, sessionID string) (string, error) {
	dir := ScanDir(baseDir, sessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create scan dir: %w", err)
	}
	return dir, nil
}
```

**Step 4: Run tests**

```bash
cd /Users/nino/sightjack && go test -run TestState -v
```

Expected: All PASS

**Step 5: Commit**

```bash
git add state.go state_test.go
git commit -m "feat: add state file persistence with JSON round-trip"
```

---

### Task 6: Claude Code Subprocess (claude.go)

**Files:**
- Create: `claude.go`
- Create: `claude_test.go`

**Step 1: Write failing test for BuildClaudeArgs**

Create `claude_test.go`:

```go
package sightjack

import (
	"testing"
)

func TestBuildClaudeArgs(t *testing.T) {
	// given
	cfg := &Config{
		Claude: ClaudeConfig{
			Command: "claude",
			Model:   "opus",
		},
	}
	prompt := "Analyze these issues"

	// when
	args := BuildClaudeArgs(cfg, prompt)

	// then
	expected := []string{"--print", "--model", "opus", "-p", "Analyze these issues"}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, e := range expected {
		if args[i] != e {
			t.Errorf("arg[%d]: expected %q, got %q", i, e, args[i])
		}
	}
}

func TestBuildClaudeArgs_NoModel(t *testing.T) {
	// given
	cfg := &Config{
		Claude: ClaudeConfig{
			Command: "claude",
			Model:   "",
		},
	}
	prompt := "test prompt"

	// when
	args := BuildClaudeArgs(cfg, prompt)

	// then
	for _, a := range args {
		if a == "--model" {
			t.Error("--model should not be present when model is empty")
		}
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/nino/sightjack && go test -run TestBuildClaudeArgs -v
```

Expected: FAIL

**Step 3: Implement claude.go**

Create `claude.go`:

```go
package sightjack

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// newCmd is a variable to allow test injection.
var newCmd = defaultNewCmd

func defaultNewCmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// BuildClaudeArgs constructs the argument list for Claude Code CLI.
func BuildClaudeArgs(cfg *Config, prompt string) []string {
	args := []string{"--print"}
	if cfg.Claude.Model != "" {
		args = append(args, "--model", cfg.Claude.Model)
	}
	args = append(args, "-p", prompt)
	return args
}

// RunClaude executes a Claude Code subprocess and streams output to stdout.
// The subprocess writes its structured result to outputPath (specified in prompt).
// This function returns the raw stdout output for logging purposes.
func RunClaude(ctx context.Context, cfg *Config, prompt string) (string, error) {
	timeout := time.Duration(cfg.Claude.TimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := BuildClaudeArgs(cfg, prompt)
	cmd := newCmd(ctx, cfg.Claude.Command, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("claude start: %w", err)
	}

	// Streaming goroutine: tee output to terminal + buffer
	var output strings.Builder
	done := make(chan struct{})

	go func() {
		defer close(done)
		reader := bufio.NewReader(stdout)
		buf := make([]byte, 4096)
		for {
			n, readErr := reader.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				os.Stdout.Write(chunk)
				output.Write(chunk)
			}
			if readErr != nil {
				if readErr != io.EOF {
					LogWarn("stdout read: %v", readErr)
				}
				break
			}
		}
	}()

	<-done

	if err := cmd.Wait(); err != nil {
		return output.String(), fmt.Errorf("claude exit: %w", err)
	}

	return output.String(), nil
}

// RunClaudeDryRun generates the prompt without executing Claude Code.
// Writes the prompt to the specified path for inspection.
func RunClaudeDryRun(cfg *Config, prompt, outputPath string) error {
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("create dry-run dir: %w", err)
	}
	promptFile := outputPath + "/prompt.md"
	if err := os.WriteFile(promptFile, []byte(prompt), 0644); err != nil {
		return fmt.Errorf("write prompt: %w", err)
	}
	LogInfo("Dry-run: prompt saved to %s", promptFile)
	return nil
}
```

**Step 4: Run tests**

```bash
cd /Users/nino/sightjack && go test -run TestBuildClaudeArgs -v
```

Expected: All PASS

**Step 5: Write failing test for RunClaudeDryRun**

Add to `claude_test.go`:

```go
func TestRunClaudeDryRun(t *testing.T) {
	// given
	dir := t.TempDir()
	cfg := &Config{Claude: ClaudeConfig{Command: "claude"}}
	prompt := "test prompt content"
	outDir := dir + "/dryrun"

	// when
	err := RunClaudeDryRun(cfg, prompt, outDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outDir + "/prompt.md")
	if err != nil {
		t.Fatalf("failed to read prompt file: %v", err)
	}
	if string(data) != prompt {
		t.Errorf("expected %q, got %q", prompt, string(data))
	}
}
```

**Step 6: Run tests**

```bash
cd /Users/nino/sightjack && go test -run TestRunClaudeDryRun -v
```

Expected: PASS

**Step 7: Commit**

```bash
git add claude.go claude_test.go
git commit -m "feat: add Claude Code subprocess management with streaming and dry-run"
```

---

### Task 7: Prompt Templates (prompt.go)

**Files:**
- Create: `prompt.go`
- Create: `prompt_test.go`
- Create: `prompts/templates/scanner_classify_ja.md.tmpl`
- Create: `prompts/templates/scanner_classify_en.md.tmpl`
- Create: `prompts/templates/scanner_deepscan_ja.md.tmpl`
- Create: `prompts/templates/scanner_deepscan_en.md.tmpl`

**Step 1: Write failing test for RenderClassifyPrompt**

Create `prompt_test.go`:

```go
package sightjack

import (
	"strings"
	"testing"
)

func TestRenderClassifyPrompt(t *testing.T) {
	// given
	data := ClassifyPromptData{
		TeamFilter:    "MY-TEAM",
		ProjectFilter: "My Project",
		OutputPath:    "/tmp/classify.json",
	}

	// when
	result, err := RenderClassifyPrompt("ja", data)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "MY-TEAM") {
		t.Error("expected team filter in prompt")
	}
	if !strings.Contains(result, "/tmp/classify.json") {
		t.Error("expected output path in prompt")
	}
}

func TestRenderDeepScanPrompt(t *testing.T) {
	// given
	data := DeepScanPromptData{
		ClusterName: "Auth",
		IssueIDs:    "ID1, ID2, ID3",
		OutputPath:  "/tmp/cluster_auth.json",
	}

	// when
	result, err := RenderDeepScanPrompt("ja", data)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Auth") {
		t.Error("expected cluster name in prompt")
	}
	if !strings.Contains(result, "/tmp/cluster_auth.json") {
		t.Error("expected output path in prompt")
	}
}

func TestRenderClassifyPrompt_English(t *testing.T) {
	// given
	data := ClassifyPromptData{
		TeamFilter:    "TEST",
		ProjectFilter: "Test",
		OutputPath:    "/tmp/out.json",
	}

	// when
	result, err := RenderClassifyPrompt("en", data)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Scanner Agent") {
		t.Error("expected Scanner Agent in English prompt")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/nino/sightjack && go test -run TestRender -v
```

Expected: FAIL

**Step 3: Create prompt templates**

Create `prompts/templates/scanner_classify_ja.md.tmpl`:

```
あなたは Scanner Agent です。
Linear MCP Server を使って指定プロジェクトの Issue を取得し、
論理的なクラスタ（機能グループ）に分類してください。

## フィルター条件
- チーム: {{.TeamFilter}}
- プロジェクト: {{.ProjectFilter}}
{{- if .CycleFilter}}
- サイクル: {{.CycleFilter}}
{{- end}}

## 手順
1. Linear MCP で Issue 一覧を取得してください
2. Issue のタイトル・ラベル・説明から論理的なクラスタに分類してください
3. 各クラスタの名前と所属する Issue ID のリストを作成してください

## 出力
以下の JSON を **{{.OutputPath}}** に書き込んでください:

```json
{
  "clusters": [
    { "name": "クラスタ名", "issue_ids": ["issue-id-1", "issue-id-2"] }
  ],
  "total_issues": 数値
}
```

重要: 出力は上記のファイルパスに直接書き込んでください。標準出力には書かないでください。
```

Create `prompts/templates/scanner_classify_en.md.tmpl`:

```
You are a Scanner Agent.
Use the Linear MCP Server to fetch Issues from the specified project
and classify them into logical clusters (functional groups).

## Filter Criteria
- Team: {{.TeamFilter}}
- Project: {{.ProjectFilter}}
{{- if .CycleFilter}}
- Cycle: {{.CycleFilter}}
{{- end}}

## Steps
1. Fetch the Issue list via Linear MCP
2. Classify Issues into logical clusters based on title, labels, and description
3. Create a list of cluster names with their associated Issue IDs

## Output
Write the following JSON to **{{.OutputPath}}**:

```json
{
  "clusters": [
    { "name": "ClusterName", "issue_ids": ["issue-id-1", "issue-id-2"] }
  ],
  "total_issues": number
}
```

Important: Write output directly to the file path above. Do not write to stdout.
```

Create `prompts/templates/scanner_deepscan_ja.md.tmpl`:

```
あなたは Scanner Agent です。
以下のクラスタに属する Issue を詳細に分析し、完成度を評価してください。

## 対象クラスタ
- クラスタ名: {{.ClusterName}}
- Issue IDs: {{.IssueIDs}}

## 分析項目
各 Issue について以下を評価してください:
- DoD (Definition of Done) の有無と品質
- 依存関係の明示度
- 実装に必要な技術的判断の有無
- 見積もり可能な粒度か
- 不足している情報（gaps）

## 完成度の目安
- 0.0-0.2: タイトルのみ、詳細なし
- 0.2-0.4: 概要あり、DoD/依存関係なし
- 0.4-0.6: DoD 部分的、依存関係一部明示
- 0.6-0.8: DoD 完備、依存関係明示、技術判断記録あり
- 0.8-1.0: 実装可能（Paintress で処理できるレベル）

## 出力
以下の JSON を **{{.OutputPath}}** に書き込んでください:

```json
{
  "name": "{{.ClusterName}}",
  "completeness": 0.35,
  "issues": [
    {
      "id": "issue-id",
      "identifier": "AWE-50",
      "title": "Issue Title",
      "completeness": 0.4,
      "gaps": ["DoD missing", "No dependency specified"]
    }
  ],
  "observations": ["クラスタ横断の発見事項"]
}
```

重要: 出力は上記のファイルパスに直接書き込んでください。標準出力には書かないでください。
```

Create `prompts/templates/scanner_deepscan_en.md.tmpl`:

```
You are a Scanner Agent.
Analyze the Issues belonging to the following cluster in detail
and evaluate their completeness.

## Target Cluster
- Cluster Name: {{.ClusterName}}
- Issue IDs: {{.IssueIDs}}

## Analysis Criteria
Evaluate each Issue on:
- Presence and quality of DoD (Definition of Done)
- Clarity of dependencies
- Whether technical decisions are needed
- Whether it is estimable at its current granularity
- Missing information (gaps)

## Completeness Scale
- 0.0-0.2: Title only, no details
- 0.2-0.4: Overview exists, no DoD/dependencies
- 0.4-0.6: Partial DoD, some dependencies noted
- 0.6-0.8: Full DoD, clear dependencies, technical decisions recorded
- 0.8-1.0: Implementation-ready (processable by Paintress)

## Output
Write the following JSON to **{{.OutputPath}}**:

```json
{
  "name": "{{.ClusterName}}",
  "completeness": 0.35,
  "issues": [
    {
      "id": "issue-id",
      "identifier": "AWE-50",
      "title": "Issue Title",
      "completeness": 0.4,
      "gaps": ["DoD missing", "No dependency specified"]
    }
  ],
  "observations": ["Cross-cluster findings"]
}
```

Important: Write output directly to the file path above. Do not write to stdout.
```

**Step 4: Implement prompt.go**

Create `prompt.go`:

```go
package sightjack

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed prompts/templates/*.tmpl
var promptFS embed.FS

// ClassifyPromptData holds template data for the classify prompt.
type ClassifyPromptData struct {
	TeamFilter    string
	ProjectFilter string
	CycleFilter   string
	OutputPath    string
}

// DeepScanPromptData holds template data for the deep scan prompt.
type DeepScanPromptData struct {
	ClusterName string
	IssueIDs    string
	OutputPath  string
}

func renderTemplate(name string, data any) (string, error) {
	tmpl, err := template.ParseFS(promptFS, name)
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.String(), nil
}

// RenderClassifyPrompt renders the cluster classification prompt for the given language.
func RenderClassifyPrompt(lang string, data ClassifyPromptData) (string, error) {
	name := fmt.Sprintf("prompts/templates/scanner_classify_%s.md.tmpl", lang)
	return renderTemplate(name, data)
}

// RenderDeepScanPrompt renders the deep scan prompt for the given language.
func RenderDeepScanPrompt(lang string, data DeepScanPromptData) (string, error) {
	name := fmt.Sprintf("prompts/templates/scanner_deepscan_%s.md.tmpl", lang)
	return renderTemplate(name, data)
}
```

**Step 5: Run tests**

```bash
cd /Users/nino/sightjack && go test -run TestRender -v
```

Expected: All PASS

**Step 6: Commit**

```bash
git add prompt.go prompt_test.go prompts/
git commit -m "feat: add prompt template system with classify and deepscan templates"
```

---

### Task 8: Scanner Orchestration (scanner.go)

**Files:**
- Create: `scanner.go`
- Create: `scanner_test.go`

**Step 1: Write failing test for ParseClassifyResult**

Create `scanner_test.go`:

```go
package sightjack

import (
	"os"
	"path/filepath"
	"testing"
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
	result, err := ParseClassifyResult(path)

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
	result, err := ParseClusterScanResult(path)

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

func TestMergeScanResults(t *testing.T) {
	// given
	clusters := []ClusterScanResult{
		{Name: "Auth", Completeness: 0.25, Issues: make([]IssueDetail, 3)},
		{Name: "API", Completeness: 0.50, Issues: make([]IssueDetail, 7)},
	}

	// when
	result := MergeScanResults(clusters)

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
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/nino/sightjack && go test -run "TestParse|TestMerge" -v
```

Expected: FAIL

**Step 3: Implement scanner.go**

Create `scanner.go`:

```go
package sightjack

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

// ParseClassifyResult reads and parses the classify.json output file.
func ParseClassifyResult(path string) (*ClassifyResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read classify result: %w", err)
	}
	var result ClassifyResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse classify result: %w", err)
	}
	return &result, nil
}

// ParseClusterScanResult reads and parses a cluster_{name}.json output file.
func ParseClusterScanResult(path string) (*ClusterScanResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cluster result: %w", err)
	}
	var result ClusterScanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse cluster result: %w", err)
	}
	return &result, nil
}

// MergeScanResults combines per-cluster deep scan results into a single ScanResult.
func MergeScanResults(clusters []ClusterScanResult) ScanResult {
	result := ScanResult{Clusters: clusters}
	result.CalculateCompleteness()

	for _, c := range clusters {
		result.Observations = append(result.Observations, c.Observations...)
	}
	return result
}

// RunScan executes the full two-pass scan.
// Pass 1: Classify all issues into clusters.
// Pass 2: Deep scan each cluster in parallel.
func RunScan(ctx context.Context, cfg *Config, baseDir string, dryRun bool) (*ScanResult, error) {
	sessionID := fmt.Sprintf("scan-%d", os.Getpid())
	scanDir, err := EnsureScanDir(baseDir, sessionID)
	if err != nil {
		return nil, err
	}

	// --- Pass 1: Classify ---
	LogScan("Pass 1: Classifying issues...")
	classifyOutput := filepath.Join(scanDir, "classify.json")

	classifyPrompt, err := RenderClassifyPrompt(cfg.Lang, ClassifyPromptData{
		TeamFilter:    cfg.Linear.Team,
		ProjectFilter: cfg.Linear.Project,
		CycleFilter:   cfg.Linear.Cycle,
		OutputPath:    classifyOutput,
	})
	if err != nil {
		return nil, fmt.Errorf("render classify prompt: %w", err)
	}

	if dryRun {
		return nil, RunClaudeDryRun(cfg, classifyPrompt, scanDir)
	}

	if _, err := RunClaude(ctx, cfg, classifyPrompt); err != nil {
		return nil, fmt.Errorf("classify scan: %w", err)
	}

	classify, err := ParseClassifyResult(classifyOutput)
	if err != nil {
		return nil, err
	}
	LogOK("Found %d clusters with %d total issues", len(classify.Clusters), classify.TotalIssues)

	// --- Pass 2: Deep scan per cluster (parallel) ---
	LogScan("Pass 2: Deep scanning %d clusters...", len(classify.Clusters))

	var (
		mu       sync.Mutex
		clusters []ClusterScanResult
	)

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.Scan.MaxConcurrency)

	for _, cc := range classify.Clusters {
		g.Go(func() error {
			clusterFile := filepath.Join(scanDir, fmt.Sprintf("cluster_%s.json", sanitizeName(cc.Name)))
			prompt, renderErr := RenderDeepScanPrompt(cfg.Lang, DeepScanPromptData{
				ClusterName: cc.Name,
				IssueIDs:    strings.Join(cc.IssueIDs, ", "),
				OutputPath:  clusterFile,
			})
			if renderErr != nil {
				return fmt.Errorf("render deepscan prompt for %s: %w", cc.Name, renderErr)
			}

			LogScan("Scanning cluster: %s (%d issues)", cc.Name, len(cc.IssueIDs))
			if _, runErr := RunClaude(gCtx, cfg, prompt); runErr != nil {
				return fmt.Errorf("deepscan %s: %w", cc.Name, runErr)
			}

			result, parseErr := ParseClusterScanResult(clusterFile)
			if parseErr != nil {
				return fmt.Errorf("parse %s: %w", cc.Name, parseErr)
			}

			mu.Lock()
			clusters = append(clusters, *result)
			mu.Unlock()

			LogOK("Cluster %s: %.0f%% complete", cc.Name, result.Completeness*100)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	merged := MergeScanResults(clusters)
	return &merged, nil
}

// sanitizeName converts a cluster name to a safe filename component.
func sanitizeName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", "_"))
}
```

**Step 4: Run tests**

```bash
cd /Users/nino/sightjack && go test -run "TestParse|TestMerge" -v
```

Expected: All PASS

**Step 5: Get errgroup dependency**

```bash
cd /Users/nino/sightjack && go get golang.org/x/sync/errgroup
```

**Step 6: Run all tests**

```bash
cd /Users/nino/sightjack && just test
```

Expected: All PASS

**Step 7: Commit**

```bash
git add scanner.go scanner_test.go go.mod go.sum
git commit -m "feat: add two-pass scanner with parallel deep scan and file-based parsing"
```

---

### Task 9: Link Navigator (navigator.go)

**Files:**
- Create: `navigator.go`
- Create: `navigator_test.go`

**Step 1: Write failing test for RenderNavigator**

Create `navigator_test.go`:

```go
package sightjack

import (
	"strings"
	"testing"
)

func TestRenderNavigator_Basic(t *testing.T) {
	// given
	result := &ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "Auth", Completeness: 0.25, Issues: make([]IssueDetail, 5)},
			{Name: "API", Completeness: 0.40, Issues: make([]IssueDetail, 8)},
		},
		TotalIssues:  13,
		Completeness: 0.325,
	}

	// when
	output := RenderNavigator(result, "My Project")

	// then
	if !strings.Contains(output, "SIGHTJACK") {
		t.Error("expected SIGHTJACK header")
	}
	if !strings.Contains(output, "My Project") {
		t.Error("expected project name")
	}
	if !strings.Contains(output, "Auth") {
		t.Error("expected Auth cluster")
	}
	if !strings.Contains(output, "API") {
		t.Error("expected API cluster")
	}
	if !strings.Contains(output, "25%") {
		t.Error("expected Auth completeness 25%")
	}
	if !strings.Contains(output, "40%") {
		t.Error("expected API completeness 40%")
	}
	if !strings.Contains(output, "32%") {
		t.Error("expected overall completeness ~32%")
	}
}

func TestRenderNavigator_Empty(t *testing.T) {
	// given
	result := &ScanResult{}

	// when
	output := RenderNavigator(result, "Empty Project")

	// then
	if !strings.Contains(output, "SIGHTJACK") {
		t.Error("expected SIGHTJACK header even with no clusters")
	}
	if !strings.Contains(output, "0%") {
		t.Error("expected 0% completeness")
	}
}

func TestRenderNavigator_LongClusterName(t *testing.T) {
	// given
	result := &ScanResult{
		Clusters: []ClusterScanResult{
			{Name: "Authentication & Authorization", Completeness: 0.5},
		},
		Completeness: 0.5,
	}

	// when
	output := RenderNavigator(result, "Test")

	// then
	// Should truncate or handle gracefully
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if len(line) > 80 {
			// Allow some overflow but check it doesn't break
			t.Logf("Long line (%d chars): %s", len(line), line)
		}
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/nino/sightjack && go test -run TestRenderNavigator -v
```

Expected: FAIL

**Step 3: Implement navigator.go**

Create `navigator.go`:

```go
package sightjack

import (
	"fmt"
	"strings"
)

const (
	navigatorWidth  = 60
	maxClusterName  = 20
	waveColumns     = 4
)

// RenderNavigator produces the ASCII Link Navigator matrix.
// In v0.1, all wave cells show [] (not generated).
func RenderNavigator(result *ScanResult, projectName string) string {
	var b strings.Builder

	completePct := int(result.Completeness * 100)

	// Header
	border := strings.Repeat("=", navigatorWidth)
	b.WriteString(fmt.Sprintf("+%s+\n", border))
	b.WriteString(fmt.Sprintf("|%s|\n", center("SIGHTJACK - Link Navigator", navigatorWidth)))
	b.WriteString(fmt.Sprintf("|  Project: %-20s  |  Completeness: %3d%%  |\n", truncate(projectName, 20), completePct))
	b.WriteString(fmt.Sprintf("+%s+\n", border))

	// Column headers
	b.WriteString(fmt.Sprintf("|%s|\n", strings.Repeat(" ", navigatorWidth)))
	header := fmt.Sprintf("  %-*s", maxClusterName, "Cluster")
	for i := 1; i <= waveColumns; i++ {
		header += fmt.Sprintf("  W%d  ", i)
	}
	header += "  Comp."
	b.WriteString(fmt.Sprintf("| %-*s|\n", navigatorWidth-1, header))

	separator := "  " + strings.Repeat("-", navigatorWidth-4)
	b.WriteString(fmt.Sprintf("| %-*s|\n", navigatorWidth-1, separator))

	// Cluster rows
	for _, cluster := range result.Clusters {
		pct := int(cluster.Completeness * 100)
		name := truncate(cluster.Name, maxClusterName)
		row := fmt.Sprintf("  %-*s", maxClusterName, name)
		for range waveColumns {
			row += "  []  "
		}
		row += fmt.Sprintf("  %3d%%", pct)
		b.WriteString(fmt.Sprintf("| %-*s|\n", navigatorWidth-1, row))
	}

	// Footer
	b.WriteString(fmt.Sprintf("|%s|\n", strings.Repeat(" ", navigatorWidth)))
	b.WriteString(fmt.Sprintf("+%s+\n", border))
	b.WriteString(fmt.Sprintf("| %-*s|\n", navigatorWidth-1, " [] not generated  [=] available  [#] complete"))
	b.WriteString(fmt.Sprintf("| %-*s|\n", navigatorWidth-1, " [x] locked (dependency)"))
	b.WriteString(fmt.Sprintf("+%s+\n", border))

	return b.String()
}

func center(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	pad := (width - len(s)) / 2
	return strings.Repeat(" ", pad) + s + strings.Repeat(" ", width-len(s)-pad)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "~"
}
```

**Step 4: Run tests**

```bash
cd /Users/nino/sightjack && go test -run TestRenderNavigator -v
```

Expected: All PASS

**Step 5: Commit**

```bash
git add navigator.go navigator_test.go
git commit -m "feat: add Link Navigator ASCII matrix renderer"
```

---

### Task 10: CLI Entry Point (cmd/sightjack/main.go)

**Files:**
- Create: `cmd/sightjack/main.go`

**Step 1: Create the CLI entry point**

Create `cmd/sightjack/main.go`:

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	sightjack "github.com/hironow/sightjack"
)

var version = "0.1.0-dev"

func main() {
	var (
		configPath string
		lang       string
		verbose    bool
		dryRun     bool
		showVer    bool
	)

	flag.StringVar(&configPath, "config", "sightjack.yaml", "Config file path")
	flag.StringVar(&configPath, "c", "sightjack.yaml", "Config file path (shorthand)")
	flag.StringVar(&lang, "lang", "", "Language override (ja/en)")
	flag.StringVar(&lang, "l", "", "Language override (shorthand)")
	flag.BoolVar(&verbose, "verbose", false, "Verbose logging")
	flag.BoolVar(&verbose, "v", false, "Verbose logging (shorthand)")
	flag.BoolVar(&dryRun, "dry-run", false, "Generate prompts without executing Claude")
	flag.BoolVar(&showVer, "version", false, "Show version")
	flag.Parse()

	if showVer {
		fmt.Printf("sightjack %s\n", version)
		os.Exit(0)
	}

	subcmd := flag.Arg(0)
	if subcmd == "" {
		subcmd = "scan"
	}

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

	switch subcmd {
	case "scan":
		runScan(ctx, cfg, dryRun)
	case "show":
		runShow(cfg)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\nUsage: sightjack [scan|show]\n", subcmd)
		os.Exit(1)
	}
}

func runScan(ctx context.Context, cfg *sightjack.Config, dryRun bool) {
	baseDir, err := os.Getwd()
	if err != nil {
		sightjack.LogError("Failed to get working directory: %v", err)
		os.Exit(1)
	}

	sightjack.LogInfo("Starting sightjack scan...")
	sightjack.LogInfo("Team: %s | Project: %s | Lang: %s", cfg.Linear.Team, cfg.Linear.Project, cfg.Lang)

	result, err := sightjack.RunScan(ctx, cfg, baseDir, dryRun)
	if err != nil {
		sightjack.LogError("Scan failed: %v", err)
		os.Exit(1)
	}

	if dryRun {
		sightjack.LogOK("Dry-run complete. Check .sightjack/scans/ for generated prompts.")
		return
	}

	// Display Link Navigator
	nav := sightjack.RenderNavigator(result, cfg.Linear.Project)
	fmt.Println()
	fmt.Print(nav)

	// Save state
	state := &sightjack.SessionState{
		Version:      "0.1",
		SessionID:    fmt.Sprintf("scan-%d", os.Getpid()),
		Project:      cfg.Linear.Project,
		Completeness: result.Completeness,
	}
	for _, c := range result.Clusters {
		state.Clusters = append(state.Clusters, sightjack.ClusterState{
			Name:         c.Name,
			Completeness: c.Completeness,
			IssueCount:   len(c.Issues),
		})
	}

	if err := sightjack.WriteState(baseDir, state); err != nil {
		sightjack.LogWarn("Failed to save state: %v", err)
	} else {
		sightjack.LogOK("State saved to %s", sightjack.StatePath(baseDir))
	}

	sightjack.LogOK("Scan complete. Overall completeness: %.0f%%", result.Completeness*100)
}

func runShow(cfg *sightjack.Config) {
	baseDir, err := os.Getwd()
	if err != nil {
		sightjack.LogError("Failed to get working directory: %v", err)
		os.Exit(1)
	}

	state, err := sightjack.ReadState(baseDir)
	if err != nil {
		sightjack.LogError("No previous scan found: %v", err)
		sightjack.LogInfo("Run 'sightjack scan' first.")
		os.Exit(1)
	}

	// Reconstruct minimal ScanResult from state
	result := &sightjack.ScanResult{
		Completeness: state.Completeness,
	}
	for _, c := range state.Clusters {
		result.Clusters = append(result.Clusters, sightjack.ClusterScanResult{
			Name:         c.Name,
			Completeness: c.Completeness,
			Issues:       make([]sightjack.IssueDetail, c.IssueCount),
		})
		result.TotalIssues += c.IssueCount
	}

	nav := sightjack.RenderNavigator(result, state.Project)
	fmt.Println()
	fmt.Print(nav)
	sightjack.LogInfo("Last scanned: %s", state.LastScanned.Format("2006-01-02 15:04:05"))
}
```

**Step 2: Build and verify**

```bash
cd /Users/nino/sightjack && go build ./cmd/sightjack
```

Expected: Build succeeds, produces `sightjack` binary.

**Step 3: Test version flag**

```bash
cd /Users/nino/sightjack && ./sightjack --version
```

Expected: `sightjack 0.1.0-dev`

**Step 4: Test dry-run (will fail due to missing config, but shows error handling)**

```bash
cd /Users/nino/sightjack && ./sightjack --dry-run scan
```

Expected: Error about missing config file (expected behavior).

**Step 5: Run all tests**

```bash
cd /Users/nino/sightjack && just test
```

Expected: All PASS

**Step 6: Clean up binary**

```bash
cd /Users/nino/sightjack && rm -f sightjack
```

**Step 7: Commit**

```bash
git add cmd/sightjack/main.go
git commit -m "feat: add CLI entry point with scan and show commands"
```

---

### Task 11: Default Config and Integration Smoke Test

**Files:**
- Create: `sightjack.yaml` (example config)
- Modify: `.gitignore`

**Step 1: Create example sightjack.yaml**

Create `sightjack.yaml`:

```yaml
# sightjack configuration
# See docs/plans/2026-02-16-sightjack-v01-design.md for details

linear:
  team: ""        # Linear team key (e.g. "MY-TEAM")
  project: ""     # Linear project name
  cycle: ""       # Optional: cycle filter

scan:
  chunk_size: 20          # Max issues per Claude invocation
  max_concurrency: 3      # Parallel Claude processes for deep scan

claude:
  command: "claude"       # Claude CLI command
  model: "opus"           # Model to use
  timeout_sec: 300        # Per-invocation timeout (seconds)

lang: "ja"                # Prompt language: ja / en
```

**Step 2: Run full test suite**

```bash
cd /Users/nino/sightjack && just test-v
```

Expected: All tests pass.

**Step 3: Run lint**

```bash
cd /Users/nino/sightjack && just lint
```

Expected: No issues.

**Step 4: Verify build**

```bash
cd /Users/nino/sightjack && just build && ./sightjack --version && rm sightjack
```

Expected: Version prints correctly.

**Step 5: Commit**

```bash
git add sightjack.yaml
git commit -m "feat: add example sightjack.yaml configuration"
```

---

## Summary

| Task | Component | Key Files | Commits |
|------|-----------|-----------|---------|
| 1 | Scaffolding | go.mod, justfile, .gitignore | 1 |
| 2 | Domain Model | model.go, model_test.go | 1 |
| 3 | Configuration | config.go, config_test.go | 1 |
| 4 | Logger | logger.go, logger_test.go | 1 |
| 5 | State | state.go, state_test.go | 1 |
| 6 | Claude Subprocess | claude.go, claude_test.go | 1 |
| 7 | Prompt Templates | prompt.go, prompt_test.go, templates/ | 1 |
| 8 | Scanner | scanner.go, scanner_test.go | 1 |
| 9 | Link Navigator | navigator.go, navigator_test.go | 1 |
| 10 | CLI Entry Point | cmd/sightjack/main.go | 1 |
| 11 | Integration | sightjack.yaml | 1 |

**Total: 11 tasks, 11 commits, ~20 source files**
