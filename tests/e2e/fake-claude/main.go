// fake-claude is a test double for the Claude Code CLI.
//
// It mimics the subset of Claude CLI behaviour that sightjack relies on:
//   - Reads the prompt from stdin (unified adapter convention).
//   - Extracts the absolute JSON output path from the prompt text.
//   - Pattern-matches the output filename against a built-in fixture table.
//   - Writes the matching canned JSON to that path.
//   - Emits minimal stream-json NDJSON result to stdout.
//
// Install as /usr/local/bin/claude inside the E2E Docker container so that
// cfg.ClaudeCmd = "claude" resolves to this binary.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// jsonPathRe matches the first absolute .json path in the prompt text.
// Identical to the regex in lifecycle_test.go.
var jsonPathRe = regexp.MustCompile(`(/[^\s"]+\.json)`)

func main() {
	// Log env vars when FAKE_CLAUDE_ENV_LOG_DIR is set.
	if envLogDir := os.Getenv("FAKE_CLAUDE_ENV_LOG_DIR"); envLogDir != "" {
		logEnv(envLogDir)
	}

	// Handle --version flag (used by doctor's CheckTool).
	for _, a := range os.Args[1:] {
		if a == "--version" || a == "-v" {
			fmt.Println("fake-claude 0.0.0-test")
			return
		}
	}

	// Handle `mcp list` subcommand (used by doctor's auth/MCP checks).
	if len(os.Args) >= 3 && os.Args[1] == "mcp" && os.Args[2] == "list" {
		fmt.Println("  linear        ✓  connected")
		return
	}

	// Handle doctor inference probe: --max-turns is unique to doctor checks.
	// Doctor passes prompt as positional arg (not stdin).
	if hasFlag(os.Args[1:], "--max-turns") {
		prompt := ""
		for i := len(os.Args) - 1; i >= 1; i-- {
			if !strings.HasPrefix(os.Args[i], "-") && os.Args[i-1] != "--output-format" && os.Args[i-1] != "--max-turns" && os.Args[i-1] != "--model" && os.Args[i-1] != "--settings" {
				prompt = os.Args[i]
				break
			}
		}
		body := "unknown"
		if strings.Contains(prompt, "1+1") {
			body = "2"
		}
		if hasFlag(os.Args[1:], "stream-json") {
			fmt.Print(wrapStreamJSON(body))
		} else {
			fmt.Print(body)
		}
		return
	}

	// Read prompt from stdin (unified adapter convention).
	promptData, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fake-claude: read stdin: %v\n", err)
		os.Exit(1)
	}
	prompt := strings.TrimSpace(string(promptData))
	if prompt == "" {
		// Empty stdin — emit minimal stream-json result and exit.
		fmt.Print(wrapStreamJSON(""))
		return
	}

	// Log prompt when FAKE_CLAUDE_PROMPT_LOG_DIR is set.
	// Used by E2E tests to verify feedback injection into nextgen prompts.
	if logDir := os.Getenv("FAKE_CLAUDE_PROMPT_LOG_DIR"); logDir != "" {
		logPrompt(logDir, prompt)
	}

	outputPath := jsonPathRe.FindString(prompt)
	if outputPath == "" {
		// No stdout output — data exchange is file-based.
		// Printing here would leak into sightjack's stdout via RunClaude streaming.
		return
	}

	// Simulated failure for partial-failure E2E testing.
	// When FAKE_CLAUDE_FAIL_PATTERN is set and the output path contains the
	// pattern, exit 1 without writing any file — mimicking a Claude CLI crash.
	if failPattern := os.Getenv("FAKE_CLAUDE_FAIL_PATTERN"); failPattern != "" {
		if strings.Contains(outputPath, failPattern) {
			fmt.Fprintf(os.Stderr, "fake-claude: simulated failure (pattern %q matched %q)\n", failPattern, outputPath)
			os.Exit(1)
		}
	}

	filename := filepath.Base(outputPath)
	matched := false
	for _, f := range fixtures {
		ok, _ := filepath.Match(f.pattern, filename)
		if ok {
			matched = true
			if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
				fmt.Fprintf(os.Stderr, "fake-claude: mkdir: %v\n", err)
				os.Exit(1)
			}
			if err := os.WriteFile(outputPath, []byte(f.content), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "fake-claude: write: %v\n", err)
				os.Exit(1)
			}
			break
		}
	}
	if !matched {
		fmt.Fprintf(os.Stderr, "fake-claude: no fixture for %q\n", filename)
		os.Exit(2)
	}

	// Emit minimal stream-json result to stdout (adapter expects result message).
	fmt.Print(wrapStreamJSON("ok"))
}

// logPrompt appends the prompt text to a sequentially-named file in dir.
func logPrompt(dir, prompt string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	entries, _ := os.ReadDir(dir)
	seq := len(entries) + 1
	filename := fmt.Sprintf("prompt_%03d.txt", seq)
	os.WriteFile(filepath.Join(dir, filename), []byte(prompt), 0o644)
}

// logEnv writes selected environment variables and args to a JSON file.
func logEnv(dir string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	entries, _ := os.ReadDir(dir)
	seq := len(entries) + 1
	filename := fmt.Sprintf("env_%03d.json", seq)

	data := map[string]any{
		"args": os.Args[1:],
	}
	// Capture env vars that tools set via ParseShellCommand.
	for _, key := range []string{"CLAUDE_CONFIG_DIR", "CLAUDE_MODEL", "ANTHROPIC_API_KEY"} {
		if v := os.Getenv(key); v != "" {
			data[key] = v
		}
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, filename), jsonBytes, 0o644)
}

// hasFlag returns true if the given flag appears anywhere in args.
func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// wrapStreamJSON wraps a body string in a minimal stream-json result message.
func wrapStreamJSON(body string) string {
	escaped, _ := json.Marshal(body)
	return fmt.Sprintf(`{"type":"result","subtype":"success","session_id":"fake","result":%s,"is_error":false,"num_turns":1,"duration_ms":1,"total_cost_usd":0,"usage":{"input_tokens":1,"output_tokens":1},"stop_reason":"end_turn"}`, string(escaped))
}

// fixture is a filename pattern → canned JSON content pair.
type fixture struct {
	pattern string
	content string
}

// fixtures is the built-in response table.
// Patterns use filepath.Match syntax.
var fixtures = []fixture{
	{pattern: "classify.json", content: classifySingleCluster},
	{pattern: "cluster_*_c*.json", content: deepScanAuth},
	{pattern: "wave_*_*.json", content: waveGenAuth},
	{pattern: "apply_*_*.json", content: waveApplySuccess},
	{pattern: "nextgen_*_*.json", content: nextgenEmpty},
	{pattern: "architect_*_*.json", content: architectDiscussApprove},
}

// --- Canned JSON responses (ported from lifecycle_test.go) ---

var classifySingleCluster = strings.TrimSpace(`
{
  "clusters": [
    {"name": "Auth", "issue_ids": ["AUTH-1", "AUTH-2"], "labels": ["security"]}
  ],
  "total_issues": 2
}
`)

var deepScanAuth = strings.TrimSpace(`
{
  "name": "Auth", "completeness": 0.35,
  "issues": [
    {"id": "AUTH-1", "identifier": "AUTH-1", "title": "Login flow", "completeness": 0.3, "gaps": ["DoD missing"]},
    {"id": "AUTH-2", "identifier": "AUTH-2", "title": "Token refresh", "completeness": 0.4, "gaps": ["Tests missing"]}
  ],
  "observations": ["Auth depends on API"]
}
`)

var waveGenAuth = strings.TrimSpace(`
{
  "cluster_name": "Auth",
  "waves": [{
    "id": "auth-w1", "cluster_name": "Auth", "title": "Add DoD",
    "description": "Define acceptance criteria", "status": "available",
    "actions": [{"type": "add_dod", "issue_id": "AUTH-1", "description": "Add DoD for login", "detail": ""}],
    "prerequisites": [],
    "delta": {"before": 0.35, "after": 0.65}
  }]
}
`)

var waveApplySuccess = strings.TrimSpace(`
{
  "wave_id": "auth-w1", "applied": 1, "total_count": 1,
  "errors": [],
  "ripples": []
}
`)

var nextgenEmpty = strings.TrimSpace(`
{
  "cluster_name": "Auth",
  "waves": [],
  "reasoning": "Cluster is sufficiently complete."
}
`)

var architectDiscussApprove = strings.TrimSpace(`
{
  "analysis": "Auth module login flow lacks acceptance criteria",
  "reasoning": "Adding DoD first establishes clear contract",
  "decision": "approve",
  "modified_wave": null
}
`)
