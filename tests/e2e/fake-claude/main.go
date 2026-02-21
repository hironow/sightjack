// fake-claude is a test double for the Claude Code CLI.
//
// It mimics the subset of Claude CLI behaviour that sightjack relies on:
//   - Reads the prompt from the -p flag.
//   - Extracts the absolute JSON output path from the prompt text.
//   - Pattern-matches the output filename against a built-in fixture table.
//   - Writes the matching canned JSON to that path.
//   - Produces no stdout output (avoids leaking into pipe JSON).
//
// Install as /usr/local/bin/claude inside the E2E Docker container so that
// cfg.Claude.Command = "claude" resolves to this binary.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// jsonPathRe matches the first absolute .json path in the prompt text.
// Identical to the regex in lifecycle_test.go.
var jsonPathRe = regexp.MustCompile(`(/[^\s"]+\.json)`)

func main() {
	prompt := extractPrompt(os.Args[1:])
	if prompt == "" {
		// No stdout output — data exchange is file-based.
		// Printing here would leak into sightjack's stdout via RunClaude streaming.
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

// extractPrompt finds the value of the -p flag.
func extractPrompt(args []string) string {
	for i, arg := range args {
		if arg == "-p" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
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
