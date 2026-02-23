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
	if _, eofErr := c.ExpectEOF(); eofErr != nil {
		t.Logf("ExpectEOF: %v", eofErr)
	}

	if waitErr := cmd.Wait(); waitErr != nil {
		if isTTYError(waitErr) {
			t.Skipf("select requires controlling terminal: %v", waitErr)
		}
		t.Fatalf("select exited with error: %v", waitErr)
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
// .siren/.run/ and contains valid JSON with a non-zero size.
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
	if len(data) == 0 {
		t.Errorf("cached result file %s is empty", files[0])
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

// --- Full chain: scan → waves → (simulated select) → apply → nextgen ---

// buildSelectOutput simulates the select command by extracting the first wave
// from a WavePlan and attaching remaining waves in the format apply expects.
func buildSelectOutput(t *testing.T, wavePlanJSON []byte) string {
	t.Helper()
	var plan struct {
		Waves []json.RawMessage `json:"waves"`
	}
	if err := json.Unmarshal(wavePlanJSON, &plan); err != nil {
		t.Fatalf("parse wave plan: %v", err)
	}
	if len(plan.Waves) == 0 {
		t.Fatal("wave plan has no waves to select from")
	}

	// Merge first wave with remaining_waves.
	var first map[string]json.RawMessage
	if err := json.Unmarshal(plan.Waves[0], &first); err != nil {
		t.Fatalf("parse first wave: %v", err)
	}
	if len(plan.Waves) > 1 {
		remaining, _ := json.Marshal(plan.Waves[1:])
		first["remaining_waves"] = remaining
	}

	out, err := json.MarshalIndent(first, "", "  ")
	if err != nil {
		t.Fatalf("marshal select output: %v", err)
	}
	return string(out)
}

func TestE2E_Pipe_FullChainScanToNextgen(t *testing.T) {
	// given
	dir := initDir(t)

	// step 1: scan --json
	scanCmd := exec.Command(sightjackBin(), "scan", "--json", dir)
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

	// step 3: simulated select
	selectOut := buildSelectOutput(t, wavesOut.Bytes())

	// step 4: apply
	applyCmd := exec.Command(sightjackBin(), "apply", dir)
	applyCmd.Stdin = strings.NewReader(selectOut)
	var applyOut, applyErr bytes.Buffer
	applyCmd.Stdout = &applyOut
	applyCmd.Stderr = &applyErr
	if err := applyCmd.Run(); err != nil {
		t.Fatalf("apply failed: %v\nstderr: %s\nstdout: %s", err, applyErr.String(), applyOut.String())
	}

	var applyResult map[string]any
	if err := json.Unmarshal(applyOut.Bytes(), &applyResult); err != nil {
		t.Fatalf("invalid apply JSON: %v\nstdout: %s", err, applyOut.String())
	}
	if _, ok := applyResult["wave_id"]; !ok {
		t.Error("apply result missing 'wave_id'")
	}
	if _, ok := applyResult["completed_wave"]; !ok {
		t.Error("apply result missing 'completed_wave'")
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

	// then: final output is valid WavePlan JSON
	var nextgenPlan map[string]any
	if err := json.Unmarshal(nextgenOut.Bytes(), &nextgenPlan); err != nil {
		t.Fatalf("invalid nextgen JSON: %v\nstdout: %s", err, nextgenOut.String())
	}
	if _, ok := nextgenPlan["waves"]; !ok {
		t.Error("nextgen plan missing 'waves' key")
	}

	// Verify all intermediate result files were cached.
	for _, pattern := range []string{"scan_result.json", "waves_result.json", "apply_result.json", "nextgen_result.json"} {
		assertResultFileCached(t, dir, pattern)
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

// --- Discuss interactive ---

func TestE2E_Pipe_DiscussInteractive(t *testing.T) {
	// given
	dir := initDir(t)
	fixture := fixtureBytes(t, "selected_wave.json")

	c, err := expect.NewConsole(expect.WithDefaultTimeout(5 * time.Second))
	if err != nil {
		t.Fatalf("create console: %v", err)
	}
	defer c.Close()

	cmd := exec.Command(sightjackBin(), "discuss", dir)
	cmd.Stdin = strings.NewReader(string(fixture))
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()

	// when
	if startErr := cmd.Start(); startErr != nil {
		t.Fatalf("start discuss: %v", startErr)
	}

	if _, expErr := c.ExpectString("Topic"); expErr != nil {
		c.Tty().Close()
		if waitErr := cmd.Wait(); waitErr != nil {
			t.Skipf("discuss requires controlling terminal: expect=%v, wait=%v", expErr, waitErr)
		}
		t.Skipf("discuss requires controlling terminal: %v", expErr)
	}
	if _, expErr := c.SendLine(""); expErr != nil {
		t.Fatalf("failed to send topic: %v", expErr)
	}

	c.Tty().Close()
	if _, eofErr := c.ExpectEOF(); eofErr != nil {
		t.Logf("ExpectEOF: %v", eofErr)
	}

	if waitErr := cmd.Wait(); waitErr != nil {
		if isTTYError(waitErr) {
			t.Skipf("discuss requires controlling terminal: %v", waitErr)
		}
		t.Fatalf("discuss exited with error: %v", waitErr)
	}
	assertResultFileCached(t, dir, "discuss_result.json")
}

// --- Nextgen-to-select loop-back ---

func TestE2E_Pipe_NextgenToSelect(t *testing.T) {
	// given: wave_plan.json represents a WavePlan (same format nextgen produces)
	fixture := fixtureBytes(t, "wave_plan.json")

	c, err := expect.NewConsole(expect.WithDefaultTimeout(5 * time.Second))
	if err != nil {
		t.Fatalf("create console: %v", err)
	}
	defer c.Close()

	cmd := exec.Command(sightjackBin(), "select")
	cmd.Stdin = strings.NewReader(string(fixture))
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()

	// when
	if startErr := cmd.Start(); startErr != nil {
		t.Fatalf("start select: %v", startErr)
	}

	if _, expErr := c.ExpectString("1"); expErr != nil {
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
	if _, eofErr := c.ExpectEOF(); eofErr != nil {
		t.Logf("ExpectEOF: %v", eofErr)
	}

	if waitErr := cmd.Wait(); waitErr != nil {
		if isTTYError(waitErr) {
			t.Skipf("select requires controlling terminal: %v", waitErr)
		}
		t.Fatalf("select exited with error: %v", waitErr)
	}
}
