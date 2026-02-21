//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	expect "github.com/Netflix/go-expect"
)

// initDir creates a temp dir with .siren/config.yaml for pipe commands.
func initDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sirenDir := filepath.Join(dir, ".siren")
	if err := os.MkdirAll(sirenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `lang: en
claude:
  command: claude
  timeout_sec: 30
scan:
  max_concurrency: 1
  chunk_size: 50
linear:
  team: ENG
  project: TestProject
strictness:
  default: fog
retry:
  max_attempts: 1
  base_delay_sec: 0
labels:
  enabled: false
scribe:
  enabled: false
`
	if err := os.WriteFile(filepath.Join(sirenDir, "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// fixtureBytes reads a fixture file from tests/e2e/fixtures/.
func fixtureBytes(t *testing.T, name string) []byte {
	t.Helper()
	// Walk up from test binary location to find fixtures
	paths := []string{
		filepath.Join("tests", "e2e", "fixtures", name),
		filepath.Join("fixtures", name),
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			return data
		}
	}
	// Try absolute path from GOMOD root
	data, err := os.ReadFile(filepath.Join(srcRoot(), "tests", "e2e", "fixtures", name))
	if err != nil {
		t.Fatalf("fixture %s not found", name)
	}
	return data
}

// srcRoot returns the project root (where go.mod is).
func srcRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

func TestE2E_Pipe_ScanJSON(t *testing.T) {
	// given: a configured directory with fake-claude in PATH
	dir := initDir(t)

	// when: run scan --json (separate stdout from stderr)
	cmd := exec.Command(sightjackBin(), "scan", "--json", dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// then
	if err != nil {
		t.Fatalf("scan --json failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}
	var result map[string]any
	if jsonErr := json.Unmarshal(stdout.Bytes(), &result); jsonErr != nil {
		t.Fatalf("invalid JSON output: %v\nstdout: %s\nstderr: %s", jsonErr, stdout.String(), stderr.String())
	}
	if _, ok := result["clusters"]; !ok {
		t.Error("scan result missing 'clusters' key")
	}
}

func TestE2E_Pipe_WavesFromFixture(t *testing.T) {
	// given: a configured directory + fixture ScanResult on stdin
	dir := initDir(t)
	fixture := fixtureBytes(t, "scan_result.json")

	// when: pipe ScanResult into waves (separate stdout from stderr)
	cmd := exec.Command(sightjackBin(), "waves", dir)
	cmd.Stdin = strings.NewReader(string(fixture))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// then
	if err != nil {
		t.Fatalf("waves failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}
	var plan map[string]any
	if jsonErr := json.Unmarshal(stdout.Bytes(), &plan); jsonErr != nil {
		t.Fatalf("invalid WavePlan JSON: %v\nstdout: %s\nstderr: %s", jsonErr, stdout.String(), stderr.String())
	}
	if _, ok := plan["waves"]; !ok {
		t.Error("wave plan missing 'waves' key")
	}
}

func TestE2E_Pipe_ApplyDryRun(t *testing.T) {
	// given: a configured directory + fixture Wave on stdin
	dir := initDir(t)
	fixture := fixtureBytes(t, "selected_wave.json")

	// when: pipe Wave into apply --dry-run (separate stdout from stderr)
	cmd := exec.Command(sightjackBin(), "apply", "--dry-run", dir)
	cmd.Stdin = strings.NewReader(string(fixture))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// then
	if err != nil {
		t.Fatalf("apply --dry-run failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}
	combined := stdout.String() + stderr.String()
	if !strings.Contains(strings.ToLower(combined), "dry-run") {
		t.Errorf("expected dry-run message, got stdout: %s\nstderr: %s", stdout.String(), stderr.String())
	}
}

func TestE2E_Pipe_SelectInteractive(t *testing.T) {
	// given: fixture WavePlan + go-expect for interactive selection
	fixture := fixtureBytes(t, "wave_plan.json")

	c, err := expect.NewConsole(expect.WithDefaultTimeout(5 * time.Second))
	if err != nil {
		t.Fatalf("create console: %v", err)
	}
	defer c.Close()

	// select reads stdin for JSON and /dev/tty for interactive input.
	// We pipe JSON via stdin and use the pty for terminal interaction.
	cmd := exec.Command(sightjackBin(), "select")
	cmd.Stdin = strings.NewReader(string(fixture))
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()

	// when
	if startErr := cmd.Start(); startErr != nil {
		t.Fatalf("start select: %v", startErr)
	}

	// select opens /dev/tty directly for interactive input (separate from stdin).
	// In environments without a controlling terminal, ExpectString will timeout.
	if _, expErr := c.ExpectString("1"); expErr != nil {
		// select likely failed to open /dev/tty — skip in non-TTY environments
		c.Tty().Close()
		if waitErr := cmd.Wait(); waitErr != nil {
			t.Skipf("select requires controlling terminal: expect=%v, wait=%v", expErr, waitErr)
		}
		t.Skipf("select requires controlling terminal: %v", expErr)
	}
	if _, expErr := c.SendLine("1"); expErr != nil {
		t.Fatalf("failed to send '1': %v", expErr)
	}

	c.Tty().Close()
	c.ExpectEOF()

	if waitErr := cmd.Wait(); waitErr != nil {
		if isTTYError(waitErr) {
			t.Skipf("select requires controlling terminal: %v", waitErr)
		}
		t.Fatalf("select exited with error: %v", waitErr)
	}
}
