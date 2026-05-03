//go:build e2e

// Package e2e rival_export_e2e_test.go: end-to-end binary tests for the
// `sightjack rival export reasons` subcommand (Phase 1.2C of the Rival
// Contract v1.2 Integration & E2E plan).
//
// Plan: refs/plans/2026-05-03-rival-contract-v1-2-integration-e2e.md
// §"Phase 1.2C — `sightjack rival export reasons` binary E2E"
//
// These tests invoke the compiled `sightjack` binary as a subprocess and
// assert byte-equal output against checked-in golden fixtures. Stdout MUST
// be machine-parseable (no banner lines) per UNIX hygiene; stderr is where
// the human banner goes. The fixture is fully deterministic: the input
// D-Mail's idempotency_key is a SHA256 of (name|kind|description|body) and
// is therefore reproducible across runs.
//
// Coverage:
//
//   - TestRivalExportReasonsE2E_StdoutMarkdownDefault: default markdown to
//     stdout, byte-equal vs expected/export_stdout.md.
//   - TestRivalExportReasonsE2E_StdoutJSONFormat: --format json, byte-equal
//     vs expected/export_stdout.json.
//   - TestRivalExportReasonsE2E_FileOutput: --output <path>, file content
//     byte-equal vs the markdown golden (file output equals stdout).
//   - TestRivalExportReasonsE2E_NonexistentInputExitsNonZero: missing input
//     yields non-zero exit code and a non-empty stderr message.
package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// rivalTestdataDir resolves the absolute testdata/rival directory, walking
// the local layout (when the test binary is compiled inside tests/e2e) and
// falling back to srcRoot() for Docker / out-of-tree execution.
func rivalTestdataDir(t *testing.T) string {
	t.Helper()
	candidates := []string{
		filepath.Join("testdata", "rival"),
		filepath.Join("tests", "e2e", "testdata", "rival"),
		filepath.Join(srcRoot(), "tests", "e2e", "testdata", "rival"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			abs, absErr := filepath.Abs(c)
			if absErr == nil {
				return abs
			}
			return c
		}
	}
	t.Fatalf("rival testdata dir not found in candidates: %v", candidates)
	return ""
}

// rivalReadGolden loads a checked-in golden file. Tests assert byte-equal
// against the returned content.
func rivalReadGolden(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(rivalTestdataDir(t), "expected", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	return data
}

// rivalInputPath returns the absolute path to the input fixture D-Mail.
func rivalInputPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(rivalTestdataDir(t), "export_input.md")
}

// rivalSightjackBin resolves the sightjack binary path. SIGHTJACK_BIN env
// var takes precedence (used for local validation when /usr/local/bin holds
// a stale binary); otherwise the package-level sightjackBin() helper is
// used (Docker / PATH lookup).
func rivalSightjackBin() string {
	if env := os.Getenv("SIGHTJACK_BIN"); env != "" {
		return env
	}
	return sightjackBin()
}

// rivalRunBinary invokes the sightjack binary with the supplied args and a
// deterministic environment, capturing stdout and stderr separately. It
// returns (stdout, stderr, error). The error is the underlying *exec.Error
// or *exec.ExitError so callers can assert non-zero exits.
func rivalRunBinary(t *testing.T, args ...string) ([]byte, []byte, error) {
	t.Helper()
	cmd := exec.Command(rivalSightjackBin(), args...) // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command -- static test fixture args; rivalSightjackBin() resolves a known path and args are test-controlled flag/path literals, no user input [permanent]
	// Whitelist env: only PATH and an isolated HOME, nothing else carries
	// over from the host environment so the projection stays deterministic.
	homeDir := t.TempDir()
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + homeDir,
	}
	cmd.Dir = t.TempDir()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func TestRivalExportReasonsE2E_StdoutMarkdownDefault(t *testing.T) {
	// given: deterministic v1 spec input + checked-in markdown golden.
	input := rivalInputPath(t)
	want := rivalReadGolden(t, "export_stdout.md")

	// when: invoke the compiled binary with default --format markdown.
	stdout, stderr, err := rivalRunBinary(t, "rival", "export", "reasons", "--input", input)

	// then: zero exit, stdout byte-equal to golden, stderr non-empty (banner).
	if err != nil {
		t.Fatalf("binary failed: %v\nstderr=%s", err, stderr)
	}
	if !bytes.Equal(stdout, want) {
		t.Errorf("stdout != golden\n--- got (%d bytes) ---\n%s\n--- want (%d bytes) ---\n%s",
			len(stdout), stdout, len(want), want)
	}
}

func TestRivalExportReasonsE2E_StdoutJSONFormat(t *testing.T) {
	// given: deterministic v1 spec input + checked-in JSON golden.
	input := rivalInputPath(t)
	want := rivalReadGolden(t, "export_stdout.json")

	// when: invoke the binary with --format json.
	stdout, stderr, err := rivalRunBinary(t, "rival", "export", "reasons", "--input", input, "--format", "json")

	// then: zero exit, stdout byte-equal to JSON golden.
	if err != nil {
		t.Fatalf("binary failed: %v\nstderr=%s", err, stderr)
	}
	if !bytes.Equal(stdout, want) {
		t.Errorf("stdout JSON != golden\n--- got (%d bytes) ---\n%s\n--- want (%d bytes) ---\n%s",
			len(stdout), stdout, len(want), want)
	}
}

func TestRivalExportReasonsE2E_FileOutput(t *testing.T) {
	// given: input fixture + an output path under a temp dir.
	input := rivalInputPath(t)
	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "canvas.md")
	want := rivalReadGolden(t, "export_stdout.md")

	// when: invoke the binary with --output set; stdout must stay empty so
	// the canvas markdown ends up in the file only.
	stdout, stderr, err := rivalRunBinary(t, "rival", "export", "reasons", "--input", input, "--output", outPath)

	// then: zero exit, file exists with golden bytes, stdout is empty
	// (canvas went to file, not stdout).
	if err != nil {
		t.Fatalf("binary failed: %v\nstderr=%s", err, stderr)
	}
	if len(bytes.TrimSpace(stdout)) != 0 {
		t.Errorf("stdout should be empty when --output is set, got: %s", stdout)
	}
	got, readErr := os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("read output file: %v", readErr)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("file content != golden\n--- got (%d bytes) ---\n%s\n--- want (%d bytes) ---\n%s",
			len(got), got, len(want), want)
	}
}

func TestRivalExportReasonsE2E_NonexistentInputExitsNonZero(t *testing.T) {
	// given: a path that cannot exist.
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "does-not-exist.md")

	// when: invoke the binary against the missing path.
	stdout, stderr, err := rivalRunBinary(t, "rival", "export", "reasons", "--input", missing)

	// then: non-zero exit + non-empty stderr; stdout MUST stay empty so
	// downstream pipe consumers don't get partial canvas bytes on error.
	if err == nil {
		t.Fatalf("expected non-zero exit for missing input, got success\nstdout=%s\nstderr=%s",
			stdout, stderr)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() == 0 {
		t.Errorf("expected non-zero exit code, got 0")
	}
	if len(bytes.TrimSpace(stderr)) == 0 {
		t.Errorf("stderr should carry the error message, got empty stderr")
	}
	if len(bytes.TrimSpace(stdout)) != 0 {
		t.Errorf("stdout should be empty on error (no partial canvas), got: %s", stdout)
	}
}
