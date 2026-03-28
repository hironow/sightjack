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
claude_cmd: claude
timeout_sec: 30
scan:
  max_concurrency: 1
  chunk_size: 50
tracker:
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
	cmd := exec.Command(sightjackBin(), "scan", "--linear", "--json", dir)
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
	assertResultFileCached(t, dir, "scan_result.json")
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
	assertResultFileCached(t, dir, "waves_result.json")
}

func TestE2E_Pipe_ApplyDryRun(t *testing.T) {
	// given: a configured directory + fixture Wave on stdin
	dir := initDir(t)
	fixture := fixtureBytes(t, "selected_wave.json")

	// when: pipe Wave into apply --dry-run (separate stdout from stderr)
	cmd := exec.Command(sightjackBin(), "apply", "--linear", "--dry-run", dir)
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

// runWithPTYRaw starts a sightjack command with a go-expect PTY for interactive
// input. Returns captured stdout and any exit error from the command.
// Use runWithPTY for success-only cases; use this when testing error exits.
func runWithPTYRaw(t *testing.T, stdinData string, interact func(*expect.Console), args ...string) (string, error) {
	t.Helper()
	c, err := expect.NewConsole(expect.WithDefaultTimeout(10 * time.Second))
	if err != nil {
		t.Fatalf("create console: %v", err)
	}
	defer c.Close()

	cmd := exec.Command(sightjackBin(), args...)
	if stdinData != "" {
		cmd.Stdin = strings.NewReader(stdinData)
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = c.Tty()
	env := os.Environ()
	ttyVal := "SIGHTJACK_TTY=" + c.Tty().Name()
	replaced := false
	for i, e := range env {
		if strings.HasPrefix(e, "SIGHTJACK_TTY=") {
			env[i] = ttyVal
			replaced = true
			break
		}
	}
	if !replaced {
		env = append(env, ttyVal)
	}
	cmd.Env = env

	if startErr := cmd.Start(); startErr != nil {
		t.Fatalf("start %v: %v", args, startErr)
	}

	interact(c)

	c.Tty().Close()
	if _, eofErr := c.ExpectEOF(); eofErr != nil {
		t.Logf("ExpectEOF: %v", eofErr)
	}

	waitErr := cmd.Wait()
	return stdout.String(), waitErr
}

// runWithPTY starts a sightjack command with a go-expect PTY for interactive
// input. JSON flows through stdin, prompts go to stderr via the PTY slave, and
// interactive input is injected through SIGHTJACK_TTY (PTY slave device path).
//
// The interact function should call ExpectString/SendLine on the Console.
// Returns captured stdout (JSON) after the command finishes.
// Fatals on non-zero exit; use runWithPTYRaw for error case tests.
func runWithPTY(t *testing.T, stdinData string, interact func(*expect.Console), args ...string) string {
	t.Helper()
	out, err := runWithPTYRaw(t, stdinData, interact, args...)
	if err != nil {
		t.Fatalf("%v failed: %v\nstdout: %s", args, err, out)
	}
	return out
}

func TestE2E_Pipe_SelectInteractive(t *testing.T) {
	tests := []struct {
		name     string
		input    string // selection input ("1" for first wave, "q" for quit)
		wantJSON bool   // true = expect valid JSON output with wantKey
		wantKey  string // top-level key expected in JSON output
		wantErr  bool   // true = expect non-zero exit
	}{
		{
			name:     "SelectFirst",
			input:    "1",
			wantJSON: true,
			wantKey:  "id",
		},
		{
			name:  "Quit",
			input: "q",
		},
		{
			name:  "GoBack",
			input: "b",
		},
		{
			name:    "InvalidNumber",
			input:   "99",
			wantErr: true,
		},
		{
			name:    "InvalidText",
			input:   "abc",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			fixture := string(fixtureBytes(t, "wave_plan.json"))

			// when
			out, err := runWithPTYRaw(t, fixture, func(c *expect.Console) {
				if _, expErr := c.ExpectString("Select wave"); expErr != nil {
					t.Fatalf("expected 'Select wave' prompt: %v", expErr)
				}
				if _, expErr := c.SendLine(tt.input); expErr != nil {
					t.Fatalf("failed to send %q: %v", tt.input, expErr)
				}
			}, "select")

			// then
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected non-zero exit")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v\nstdout: %s", err, out)
			}
			if tt.wantJSON {
				var result map[string]any
				if jsonErr := json.Unmarshal([]byte(out), &result); jsonErr != nil {
					t.Fatalf("invalid JSON: %v\nstdout: %s", jsonErr, out)
				}
				if _, ok := result[tt.wantKey]; !ok {
					t.Errorf("missing key %q in output", tt.wantKey)
				}
			}
		})
	}
}

// findResultFiles searches .siren/.run/<session>/ for files matching a glob pattern.
// Returns all matching file paths, or nil if none found.
func findResultFiles(t *testing.T, dir, pattern string) []string {
	t.Helper()
	runDir := filepath.Join(dir, ".siren", ".run")
	sessions, err := os.ReadDir(runDir)
	if err != nil {
		return nil
	}
	var matches []string
	for _, s := range sessions {
		if !s.IsDir() {
			continue
		}
		sessionDir := filepath.Join(runDir, s.Name())
		entries, _ := os.ReadDir(sessionDir)
		for _, e := range entries {
			ok, _ := filepath.Match(pattern, e.Name())
			if ok {
				matches = append(matches, filepath.Join(sessionDir, e.Name()))
			}
		}
	}
	return matches
}

// assertResultFileCached verifies at least one file matching pattern exists in
// .siren/.run/ and contains valid JSON.
func assertResultFileCached(t *testing.T, dir, pattern string) {
	t.Helper()
	files := findResultFiles(t, dir, pattern)
	if len(files) == 0 {
		t.Errorf("expected cached result file matching %q in .siren/.run/, found none", pattern)
		return
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Errorf("read cached result %s: %v", files[0], err)
		return
	}
	if !json.Valid(data) {
		t.Errorf("cached result file %s does not contain valid JSON (size=%d)", files[0], len(data))
	}
}

// --- Show as pipe destination ---

func TestE2E_Pipe_ShowFromStdin(t *testing.T) {
	tests := []struct {
		name    string
		input   func(t *testing.T) string
		wantStr string // substring expected in output; empty = just check non-empty
	}{
		{
			name:    "ScanResult",
			input:   func(t *testing.T) string { return string(fixtureBytes(t, "scan_result.json")) },
			wantStr: "Auth",
		},
		{
			name:    "WavePlan",
			input:   func(t *testing.T) string { return string(fixtureBytes(t, "wave_plan.json")) },
			wantStr: "Auth",
		},
		{
			name:  "EmptyWavePlan",
			input: func(t *testing.T) string { return `{"waves":[]}` },
		},
		{
			name:  "AmbiguousJSON",
			input: func(t *testing.T) string { return `{"clusters":[],"waves":[]}` },
			// DetectPipeType checks "clusters" first → treated as ScanResult
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			input := tt.input(t)

			// when
			cmd := exec.Command(sightjackBin(), "show")
			cmd.Stdin = strings.NewReader(input)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()

			// then
			if err != nil {
				t.Fatalf("show failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
			}
			if stdout.Len() == 0 {
				t.Error("show produced empty stdout")
			}
			if tt.wantStr != "" && !strings.Contains(stdout.String(), tt.wantStr) {
				t.Errorf("expected %q in output, got:\n%s", tt.wantStr, stdout.String())
			}
		})
	}
}

// --- Two-step pipe chains ---

func TestE2E_Pipe_TwoStepChain(t *testing.T) {
	type step struct {
		args  func(dir string) []string // command args (dir substituted at runtime)
		stdin func(t *testing.T) string // stdin content; empty = use previous stdout
	}
	tests := []struct {
		name            string
		step1           step
		step2           step
		assertJSON      bool     // true = parse step2 stdout as JSON and check for wantKey
		wantKey         string   // JSON top-level key expected in step2 stdout
		wantStr         string   // substring expected in step2 stdout (for non-JSON)
		wantResultFiles []string // glob patterns for cached result files in .siren/.run/
	}{
		{
			name: "ScanToWaves",
			step1: step{
				args: func(dir string) []string { return []string{"scan", "--json", dir} },
			},
			step2: step{
				args: func(dir string) []string { return []string{"waves", dir} },
			},
			assertJSON:      true,
			wantKey:         "waves",
			wantResultFiles: []string{"scan_result.json", "waves_result.json"},
		},
		{
			name: "ScanToShow",
			step1: step{
				args: func(dir string) []string { return []string{"scan", "--json", dir} },
			},
			step2: step{
				args: func(dir string) []string { return []string{"show"} },
			},
			wantStr:         "Auth",
			wantResultFiles: []string{"scan_result.json"},
		},
		{
			name: "WavesToShow",
			step1: step{
				args:  func(dir string) []string { return []string{"waves", dir} },
				stdin: func(t *testing.T) string { return string(fixtureBytes(t, "scan_result.json")) },
			},
			step2: step{
				args: func(dir string) []string { return []string{"show"} },
			},
			wantStr:         "Auth",
			wantResultFiles: []string{"waves_result.json"},
		},
		{
			name: "ApplyToNextgen",
			step1: step{
				args:  func(dir string) []string { return []string{"apply", dir} },
				stdin: func(t *testing.T) string { return string(fixtureBytes(t, "selected_wave.json")) },
			},
			step2: step{
				args: func(dir string) []string { return []string{"nextgen", dir} },
			},
			assertJSON:      true,
			wantKey:         "waves",
			wantResultFiles: []string{"apply_result.json", "nextgen_result.json"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			dir := initDir(t)

			// when: step 1
			cmd1 := exec.Command(sightjackBin(), tt.step1.args(dir)...)
			if tt.step1.stdin != nil {
				cmd1.Stdin = strings.NewReader(tt.step1.stdin(t))
			}
			var out1, err1 bytes.Buffer
			cmd1.Stdout = &out1
			cmd1.Stderr = &err1
			if runErr := cmd1.Run(); runErr != nil {
				t.Fatalf("step1 failed: %v\nstderr: %s\nstdout: %s", runErr, err1.String(), out1.String())
			}

			// when: step 2 (stdin = step1 stdout)
			cmd2 := exec.Command(sightjackBin(), tt.step2.args(dir)...)
			if tt.step2.stdin != nil {
				cmd2.Stdin = strings.NewReader(tt.step2.stdin(t))
			} else {
				cmd2.Stdin = strings.NewReader(out1.String())
			}
			var out2, err2 bytes.Buffer
			cmd2.Stdout = &out2
			cmd2.Stderr = &err2
			if runErr := cmd2.Run(); runErr != nil {
				t.Fatalf("step2 failed: %v\nstderr: %s\nstdout: %s", runErr, err2.String(), out2.String())
			}

			// then
			if out2.Len() == 0 {
				t.Fatal("step2 produced empty stdout")
			}
			if tt.assertJSON {
				var parsed map[string]any
				if jsonErr := json.Unmarshal(out2.Bytes(), &parsed); jsonErr != nil {
					t.Fatalf("step2 stdout is not valid JSON: %v\nstdout: %s", jsonErr, out2.String())
				}
				if _, ok := parsed[tt.wantKey]; !ok {
					t.Errorf("step2 JSON missing key %q", tt.wantKey)
				}
			}
			if tt.wantStr != "" && !strings.Contains(out2.String(), tt.wantStr) {
				t.Errorf("expected %q in step2 output, got:\n%s", tt.wantStr, out2.String())
			}
			for _, pattern := range tt.wantResultFiles {
				assertResultFileCached(t, dir, pattern)
			}
		})
	}
}

// --- ADR from DiscussResult ---

func TestE2E_Pipe_ADRFromDiscussResult(t *testing.T) {
	// given
	dir := initDir(t)
	fixture := fixtureBytes(t, "discuss_result.json")

	// when
	cmd := exec.Command(sightjackBin(), "adr", dir)
	cmd.Stdin = strings.NewReader(string(fixture))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// then
	if err != nil {
		t.Fatalf("adr failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}
	output := stdout.String()
	if len(output) == 0 {
		t.Fatal("adr produced no output")
	}
	for _, want := range []string{"# 0001.", "## Context", "## Decision", "## Consequences", "Accepted"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in ADR output, got:\n%s", want, output)
		}
	}
}

// --- Interactive pipe tests (go-expect + SIGHTJACK_TTY) ---

func TestE2E_Pipe_DiscussInteractive(t *testing.T) {
	// given
	dir := initDir(t)
	fixture := string(fixtureBytes(t, "selected_wave.json"))

	// when: discuss reads Wave JSON from stdin, prompts topic via PTY
	out := runWithPTY(t, fixture, func(c *expect.Console) {
		if _, expErr := c.ExpectString("Topic"); expErr != nil {
			t.Fatalf("expected 'Topic' prompt: %v", expErr)
		}
		if _, expErr := c.SendLine(""); expErr != nil {
			t.Fatalf("failed to send empty topic: %v", expErr)
		}
	}, "discuss", dir)

	// then: valid DiscussResult JSON
	var result map[string]any
	if jsonErr := json.Unmarshal([]byte(out), &result); jsonErr != nil {
		t.Fatalf("invalid DiscussResult JSON: %v\nstdout: %s", jsonErr, out)
	}
	if _, ok := result["decision"]; !ok {
		t.Error("DiscussResult missing 'decision' key")
	}
	assertResultFileCached(t, dir, "discuss_result.json")
}

func TestE2E_Pipe_DiscussWithTopic(t *testing.T) {
	// given
	dir := initDir(t)
	fixture := string(fixtureBytes(t, "selected_wave.json"))

	// when: discuss reads Wave JSON from stdin, sends non-empty topic via PTY
	out := runWithPTY(t, fixture, func(c *expect.Console) {
		if _, expErr := c.ExpectString("Topic"); expErr != nil {
			t.Fatalf("expected 'Topic' prompt: %v", expErr)
		}
		if _, expErr := c.SendLine("Review security implications"); expErr != nil {
			t.Fatalf("failed to send topic: %v", expErr)
		}
	}, "discuss", dir)

	// then: valid DiscussResult JSON
	var result map[string]any
	if jsonErr := json.Unmarshal([]byte(out), &result); jsonErr != nil {
		t.Fatalf("invalid DiscussResult JSON: %v\nstdout: %s", jsonErr, out)
	}
	if _, ok := result["decision"]; !ok {
		t.Error("DiscussResult missing 'decision' key")
	}
}

func TestE2E_Pipe_Select_AllLocked(t *testing.T) {
	// given: wave plan where all waves have non-available status
	lockedPlan := `{
		"waves": [{
			"id": "auth-w1", "cluster_name": "Auth", "title": "Locked Wave",
			"status": "completed", "actions": [],
			"prerequisites": [], "delta": {"before": 0.35, "after": 0.65}
		}]
	}`

	// when: select with all waves locked (error occurs after openTTY, before prompt)
	_, err := runWithPTYRaw(t, lockedPlan, func(c *expect.Console) {
		// No interaction expected — error before prompt
	}, "select")

	// then: should fail with "no available waves"
	if err == nil {
		t.Fatal("expected error for all-locked waves")
	}
}

// --- Nextgen-to-select loop-back ---

func TestE2E_Pipe_NextgenToSelect(t *testing.T) {
	// given: wave_plan.json represents a WavePlan (same format nextgen produces)
	fixture := string(fixtureBytes(t, "wave_plan.json"))

	// when: pipe WavePlan into select, pick first wave
	out := runWithPTY(t, fixture, func(c *expect.Console) {
		if _, expErr := c.ExpectString("Select wave"); expErr != nil {
			t.Fatalf("expected 'Select wave' prompt: %v", expErr)
		}
		if _, expErr := c.SendLine("1"); expErr != nil {
			t.Fatalf("failed to send '1': %v", expErr)
		}
	}, "select")

	// then: valid Wave JSON with id
	var result map[string]any
	if jsonErr := json.Unmarshal([]byte(out), &result); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\nstdout: %s", jsonErr, out)
	}
	if _, ok := result["id"]; !ok {
		t.Error("select output missing 'id' key")
	}
}

// --- Multi-step interactive chains ---

func TestE2E_Pipe_SelectToApply(t *testing.T) {
	// given
	dir := initDir(t)
	fixture := string(fixtureBytes(t, "wave_plan.json"))

	// step 1: select (interactive via PTY)
	selectOut := runWithPTY(t, fixture, func(c *expect.Console) {
		if _, expErr := c.ExpectString("Select wave"); expErr != nil {
			t.Fatalf("expected 'Select wave' prompt: %v", expErr)
		}
		if _, expErr := c.SendLine("1"); expErr != nil {
			t.Fatalf("failed to send '1': %v", expErr)
		}
	}, "select")

	// step 2: apply (non-interactive, stdin = select output)
	applyCmd := exec.Command(sightjackBin(), "apply", "--linear", dir)
	applyCmd.Stdin = strings.NewReader(selectOut)
	var applyOut, applyErr bytes.Buffer
	applyCmd.Stdout = &applyOut
	applyCmd.Stderr = &applyErr
	if err := applyCmd.Run(); err != nil {
		t.Fatalf("apply failed: %v\nstderr: %s\nstdout: %s", err, applyErr.String(), applyOut.String())
	}

	// then: valid ApplyResult JSON
	var result map[string]any
	if jsonErr := json.Unmarshal(applyOut.Bytes(), &result); jsonErr != nil {
		t.Fatalf("invalid ApplyResult JSON: %v\nstdout: %s", jsonErr, applyOut.String())
	}
	if _, ok := result["wave_id"]; !ok {
		t.Error("ApplyResult missing 'wave_id'")
	}
	assertResultFileCached(t, dir, "apply_result.json")
}

func TestE2E_Pipe_SelectToDiscussToADR(t *testing.T) {
	// given
	dir := initDir(t)
	fixture := string(fixtureBytes(t, "wave_plan.json"))

	// step 1: select (interactive via PTY)
	selectOut := runWithPTY(t, fixture, func(c *expect.Console) {
		if _, expErr := c.ExpectString("Select wave"); expErr != nil {
			t.Fatalf("expected 'Select wave' prompt: %v", expErr)
		}
		if _, expErr := c.SendLine("1"); expErr != nil {
			t.Fatalf("failed to send '1': %v", expErr)
		}
	}, "select")

	// step 2: discuss (interactive via PTY)
	discussOut := runWithPTY(t, selectOut, func(c *expect.Console) {
		if _, expErr := c.ExpectString("Topic"); expErr != nil {
			t.Fatalf("expected 'Topic' prompt: %v", expErr)
		}
		if _, expErr := c.SendLine(""); expErr != nil {
			t.Fatalf("failed to send empty topic: %v", expErr)
		}
	}, "discuss", dir)

	// step 3: adr (non-interactive)
	adrCmd := exec.Command(sightjackBin(), "adr", dir)
	adrCmd.Stdin = strings.NewReader(discussOut)
	var adrOut, adrErr bytes.Buffer
	adrCmd.Stdout = &adrOut
	adrCmd.Stderr = &adrErr
	if err := adrCmd.Run(); err != nil {
		t.Fatalf("adr failed: %v\nstderr: %s\nstdout: %s", err, adrErr.String(), adrOut.String())
	}

	// then: valid ADR markdown
	output := adrOut.String()
	for _, want := range []string{"# 0001.", "## Context", "## Decision", "Accepted"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in ADR output, got:\n%s", want, output)
		}
	}
}

func TestE2E_Pipe_FullChainWithSelect(t *testing.T) {
	// given
	dir := initDir(t)

	// step 1: scan --json
	scanCmd := exec.Command(sightjackBin(), "scan", "--linear", "--json", dir)
	var scanOut, scanErr bytes.Buffer
	scanCmd.Stdout = &scanOut
	scanCmd.Stderr = &scanErr
	if err := scanCmd.Run(); err != nil {
		t.Fatalf("scan --json failed: %v\nstderr: %s", err, scanErr.String())
	}

	// step 2: waves
	wavesCmd := exec.Command(sightjackBin(), "waves", dir)
	wavesCmd.Stdin = strings.NewReader(scanOut.String())
	var wavesOut, wavesErr bytes.Buffer
	wavesCmd.Stdout = &wavesOut
	wavesCmd.Stderr = &wavesErr
	if err := wavesCmd.Run(); err != nil {
		t.Fatalf("waves failed: %v\nstderr: %s", err, wavesErr.String())
	}

	// step 3: select (interactive via PTY)
	selectOut := runWithPTY(t, wavesOut.String(), func(c *expect.Console) {
		if _, expErr := c.ExpectString("Select wave"); expErr != nil {
			t.Fatalf("expected 'Select wave' prompt: %v", expErr)
		}
		if _, expErr := c.SendLine("1"); expErr != nil {
			t.Fatalf("failed to send '1': %v", expErr)
		}
	}, "select")

	// step 4: apply
	applyCmd := exec.Command(sightjackBin(), "apply", "--linear", dir)
	applyCmd.Stdin = strings.NewReader(selectOut)
	var applyOut, applyErr bytes.Buffer
	applyCmd.Stdout = &applyOut
	applyCmd.Stderr = &applyErr
	if err := applyCmd.Run(); err != nil {
		t.Fatalf("apply failed: %v\nstderr: %s\nstdout: %s", err, applyErr.String(), applyOut.String())
	}

	// step 5: nextgen
	nextgenCmd := exec.Command(sightjackBin(), "nextgen", dir)
	nextgenCmd.Stdin = strings.NewReader(applyOut.String())
	var nextgenOut, nextgenErr bytes.Buffer
	nextgenCmd.Stdout = &nextgenOut
	nextgenCmd.Stderr = &nextgenErr
	if err := nextgenCmd.Run(); err != nil {
		t.Fatalf("nextgen failed: %v\nstderr: %s\nstdout: %s", err, nextgenErr.String(), nextgenOut.String())
	}

	// then: valid WavePlan JSON at the end of the chain
	var plan map[string]any
	if jsonErr := json.Unmarshal(nextgenOut.Bytes(), &plan); jsonErr != nil {
		t.Fatalf("invalid WavePlan: %v\nstdout: %s", jsonErr, nextgenOut.String())
	}
	if _, ok := plan["waves"]; !ok {
		t.Error("nextgen plan missing 'waves' key")
	}
}

// --- Wave generation partial failure ---

func TestE2E_Pipe_WaveGenPartialFailure(t *testing.T) {
	tests := []struct {
		name        string
		failPattern string // FAKE_CLAUDE_FAIL_PATTERN value
		wantErr     bool   // true = expect non-zero exit from waves
		wantWarning string // substring expected in stderr (checked on success)
	}{
		{
			name:        "OneClusterFails",
			failPattern: "unstable",
			wantWarning: "Unstable",
		},
		{
			name:        "AllClustersFail",
			failPattern: "wave_",
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given: multi-cluster ScanResult + FAKE_CLAUDE_FAIL_PATTERN
			dir := initDir(t)
			fixture := fixtureBytes(t, "scan_result_multi.json")

			// when: pipe into waves with failure pattern
			cmd := exec.Command(sightjackBin(), "waves", dir)
			cmd.Stdin = strings.NewReader(string(fixture))
			env := os.Environ()
			env = append(env, "FAKE_CLAUDE_FAIL_PATTERN="+tt.failPattern)
			cmd.Env = env
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()

			// then
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected non-zero exit when all clusters fail")
				}
				return
			}
			if err != nil {
				t.Fatalf("waves should partially succeed: %v\nstderr: %s\nstdout: %s",
					err, stderr.String(), stdout.String())
			}

			// Valid WavePlan JSON with waves
			var plan map[string]any
			if jsonErr := json.Unmarshal(stdout.Bytes(), &plan); jsonErr != nil {
				t.Fatalf("invalid WavePlan JSON: %v\nstdout: %s", jsonErr, stdout.String())
			}
			if _, ok := plan["waves"]; !ok {
				t.Error("wave plan missing 'waves' key")
			}

			// Warning about failed cluster in stderr
			if tt.wantWarning != "" && !strings.Contains(stderr.String(), tt.wantWarning) {
				t.Errorf("expected warning containing %q in stderr, got:\n%s",
					tt.wantWarning, stderr.String())
			}

			// Cache file created for partial results
			assertResultFileCached(t, dir, "waves_result.json")
		})
	}
}
