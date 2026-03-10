//go:build e2e

package e2e

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// sightjackBin returns the path to the sightjack binary.
// In Docker it is /usr/local/bin/sightjack; locally fall back to PATH.
// If neither is found, returns "sightjack" so exec.Command fails with a
// clear error rather than silently running the wrong binary.
func sightjackBin() string {
	if _, err := os.Stat("/usr/local/bin/sightjack"); err == nil {
		return "/usr/local/bin/sightjack"
	}
	p, err := exec.LookPath("sightjack")
	if err != nil {
		return "sightjack"
	}
	return p
}

// runCmd executes sightjack with args and returns stdout+stderr combined.
func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(sightjackBin(), args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestE2E_Version(t *testing.T) {
	// when
	out, err := runCmd(t, "version")

	// then
	if err != nil {
		t.Fatalf("version failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "sightjack v") {
		t.Errorf("expected version string, got: %s", out)
	}
}

func TestE2E_VersionJSON(t *testing.T) {
	// when
	out, err := runCmd(t, "version", "--json")

	// then
	if err != nil {
		t.Fatalf("version --json failed: %v\noutput: %s", err, out)
	}
	var v map[string]string
	if jsonErr := json.Unmarshal([]byte(out), &v); jsonErr != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", jsonErr, out)
	}
	for _, key := range []string{"version", "commit", "date", "go"} {
		if _, ok := v[key]; !ok {
			t.Errorf("missing key %q in version JSON", key)
		}
	}
}

func TestE2E_Doctor(t *testing.T) {
	// when: doctor may fail if no config exists, but should always produce output
	out, _ := runCmd(t, "doctor")

	// then
	if len(strings.TrimSpace(out)) == 0 {
		t.Error("doctor output is empty")
	}
	// Should contain health check labels regardless of pass/fail
	if !strings.Contains(out, "doctor") {
		t.Errorf("doctor output missing header: %s", out)
	}
}

func TestE2E_Help(t *testing.T) {
	// when
	out, err := runCmd(t, "--help")

	// then
	if err != nil {
		t.Fatalf("--help failed: %v\noutput: %s", err, out)
	}
	for _, sub := range []string{"scan", "waves", "select", "apply", "run", "version"} {
		if !strings.Contains(out, sub) {
			t.Errorf("--help output missing subcommand %q", sub)
		}
	}
}

func TestE2E_UnknownCommand(t *testing.T) {
	// when
	_, err := runCmd(t, "nonexistent")

	// then
	if err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestE2E_Show_NoState(t *testing.T) {
	// given: a directory with no .siren/events/
	dir := t.TempDir()

	// when
	_, err := runCmd(t, "show", dir)

	// then: should fail because no state exists
	if err == nil {
		t.Error("expected error when no state exists")
	}
}

func TestE2E_ArchivePrune_Empty(t *testing.T) {
	// given: an empty directory
	dir := t.TempDir()

	// when
	out, err := runCmd(t, "archive-prune", dir)

	// then: should succeed (nothing to prune)
	if err != nil {
		t.Fatalf("archive-prune failed: %v\noutput: %s", err, out)
	}
}

func TestE2E_Init_WithFlags(t *testing.T) {
	// given
	dir := t.TempDir()

	// when: init with flags (non-interactive, no prompts)
	out, err := runCmd(t, "init", "--team", "TestTeam", "--project", "TestProject", dir)

	// then
	if err != nil {
		t.Fatalf("init failed: %v\noutput: %s", err, out)
	}
	cfgFile := filepath.Join(dir, ".siren", "config.yaml")
	if _, statErr := os.Stat(cfgFile); errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("config file not created: %s", cfgFile)
	}
}
