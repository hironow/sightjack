package integration_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/session"
)

// --- Mock Dispatcher ---

// claudeMockDispatcher provides a newCmd replacement that writes canned JSON
// to the output path extracted from the Claude prompt.
type claudeMockDispatcher struct {
	t           *testing.T
	mu          sync.Mutex
	responses   []mockResponse // ordered for deterministic matching
	callLogFile string         // path to file-based call log (written by shell scripts)
}

type mockResponse struct {
	pattern string
	content string
}

func newMockDispatcher(t *testing.T) *claudeMockDispatcher {
	return &claudeMockDispatcher{
		t:           t,
		callLogFile: filepath.Join(t.TempDir(), "dispatcher-call-log.txt"),
	}
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
	return session.OverrideNewCmd(d.newCmdFunc)
}

// CallLog returns filenames written by mock shell scripts (read from call log file).
func (d *claudeMockDispatcher) CallLog() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	logFile := d.callLogFile
	if logFile == "" {
		return nil
	}
	data, err := os.ReadFile(logFile)
	if err != nil {
		return nil
	}
	var result []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func (d *claudeMockDispatcher) newCmdFunc(ctx context.Context, name string, args ...string) *exec.Cmd {
	// Prompt is passed via stdin (not -p flag). Build a shell script that:
	// 1. Reads stdin to get prompt
	// 2. Extracts JSON output path from prompt
	// 3. Writes matching response to that path
	// 4. Emits valid stream-json NDJSON to stdout

	// Prepare response file mappings for the shell script.
	d.mu.Lock()
	defer d.mu.Unlock()
	var caseArms []string
	for _, r := range d.responses {
		escaped := strings.ReplaceAll(r.content, "'", "'\\''")
		caseArms = append(caseArms, fmt.Sprintf(
			`  %s) mkdir -p "$(dirname "$OUT_PATH")" && printf '%%s' '%s' > "$OUT_PATH" && echo "%s" >> "$CALL_LOG" ;;`,
			r.pattern, escaped, r.pattern))
	}

	callLogFile := d.callLogFile

	ndjson := `{"type":"assistant","session_id":"mock","message":{"content":[{"type":"text","text":"[mock-stream-xyzzy]"}]}}` + "\n" +
		`{"type":"result","subtype":"success","session_id":"mock","result":"[mock-stream-xyzzy]","usage":{"input_tokens":10,"output_tokens":5}}`

	script := "#!/bin/sh\nPROMPT=$(cat)\n"
	script += fmt.Sprintf("CALL_LOG='%s'\n", callLogFile)
	script += `OUT_PATH=$(echo "$PROMPT" | grep -oE '/[^ "]*\.json' | head -1)` + "\n"
	script += `FILENAME=$(basename "$OUT_PATH" 2>/dev/null)` + "\n"
	script += "if [ -n \"$OUT_PATH\" ]; then\n"
	script += "  case \"$FILENAME\" in\n"
	for _, arm := range caseArms {
		script += arm + "\n"
	}
	script += "  esac\n"
	script += "fi\n"
	script += fmt.Sprintf("printf '%%s' '%s'\n", strings.ReplaceAll(ndjson, "'", "'\\''"))

	sf := filepath.Join(d.t.TempDir(), fmt.Sprintf("mock_%d.sh", time.Now().UnixNano()))
	if err := os.WriteFile(sf, []byte(script), 0755); err != nil {
		d.t.Fatalf("write mock script: %v", err)
	}

	return exec.CommandContext(ctx, "sh", sf)
}

// extractOutputPath finds the first absolute JSON file path in the prompt text.
var jsonPathRe = regexp.MustCompile(`(/[^\s"]+\.json)`)

func extractOutputPath(prompt string) string {
	m := jsonPathRe.FindString(prompt)
	return m
}

// --- Helper: assertFileExists ---

// --- Canned JSON fixtures ---

// --- Tests for mock helpers ---

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

// --- Phase 3: Session Tests ---

// --- Phase 4: Resume Tests ---

// --- Phase 5: Multi-cluster Test ---

func TestMockDispatcher_WritesFile(t *testing.T) {
	// given
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "classify.json")
	d := newMockDispatcher(t)
	d.Register("classify.json", `{"clusters":[],"total_issues":0}`)
	cleanup := d.Install()
	defer cleanup()

	// when: simulate a Claude call — prompt via stdin (unified adapter convention)
	prompt := "Write JSON to " + outputPath
	cmd := d.newCmdFunc(context.Background(), "claude", "--dangerously-skip-permissions", "--print")
	cmd.Stdin = strings.NewReader(prompt)
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

// --- Phase 7: Result Cache Round-Trip Tests ---
//
// Each pipe command caches its final result as {cmd}_result.json.
// These tests verify the full round-trip: produce → marshal → write → read → unmarshal → verify.
