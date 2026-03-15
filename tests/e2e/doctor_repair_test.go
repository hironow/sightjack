//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type doctorOutput struct {
	Checks []struct {
		Name    string `json:"name"`
		Status  string `json:"status"`
		Message string `json:"message"`
		Hint    string `json:"hint,omitempty"`
	} `json:"checks"`
}

func parseDoctorJSON(t *testing.T, output string) doctorOutput {
	t.Helper()
	idx := strings.Index(output, "{")
	if idx < 0 {
		t.Fatalf("no JSON object found in output: %s", output)
	}
	jsonStr := output[idx:]
	var result doctorOutput
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("failed to parse doctor JSON: %v\nraw: %s", err, jsonStr)
	}
	return result
}

// initTestProject creates a temp directory with git repo + sightjack init.
func initTestProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	for _, args := range [][]string{
		{"git", "init", "--initial-branch", "main", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "test"},
	} {
		cmd := exec.Command(args[0], args[1:]...) // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command — static test fixture args [permanent]
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git setup %v failed: %v\n%s", args, err, out)
		}
	}

	out, err := runCmd(t, "init", "--team", "TEST", "--project", "TestProject", dir)
	if err != nil {
		t.Fatalf("sightjack init failed: %v\n%s", err, out)
	}
	return dir
}

func TestDoctorRepair_StalePID(t *testing.T) {
	// given: initialized project with stale PID file
	dir := initTestProject(t)
	pidDir := filepath.Join(dir, ".siren")
	pidFile := filepath.Join(pidDir, "watch.pid")
	if err := os.WriteFile(pidFile, []byte("99999"), 0o644); err != nil {
		t.Fatalf("create stale PID: %v", err)
	}

	// when: run doctor --repair --json
	out, _ := runCmd(t, "doctor", "--repair", "--json", dir)

	// then: stale-pid should be fixed
	result := parseDoctorJSON(t, out)
	found := false
	for _, check := range result.Checks {
		if check.Name == "stale-pid" {
			found = true
			if check.Status != "FIX" {
				t.Errorf("stale-pid: expected status FIX, got %s", check.Status)
			}
			if !strings.Contains(check.Message, "removed stale PID") {
				t.Errorf("stale-pid: unexpected message: %s", check.Message)
			}
		}
	}
	if !found {
		t.Errorf("stale-pid check not found in results: %+v", result.Checks)
	}

	// Verify PID file was actually removed
	if _, err := os.Stat(pidFile); err == nil {
		t.Error("watch.pid should have been removed but still exists")
	}
}

func TestDoctorRepair_MissingSkillMD(t *testing.T) {
	// given: initialized project with SKILL.md deleted
	dir := initTestProject(t)
	skillPaths := []string{
		filepath.Join(dir, ".siren", "skills", "dmail-sendable", "SKILL.md"),
		filepath.Join(dir, ".siren", "skills", "dmail-readable", "SKILL.md"),
	}
	for _, p := range skillPaths {
		os.Remove(p)
	}

	// when: run doctor --repair --json
	out, _ := runCmd(t, "doctor", "--repair", "--json", dir)

	// then: Skills should be regenerated
	result := parseDoctorJSON(t, out)
	found := false
	for _, check := range result.Checks {
		if check.Name == "Skills" && check.Status == "FIX" {
			found = true
			if !strings.Contains(check.Message, "regenerated") {
				t.Errorf("Skills: expected regenerated message, got: %s", check.Message)
			}
		}
	}
	if !found {
		t.Errorf("Skills FIX check not found in results; checks: %+v", result.Checks)
	}

	// Verify SKILL.md was actually regenerated
	for _, p := range skillPaths {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to be regenerated: %v", filepath.Base(filepath.Dir(p)), err)
		}
	}
}

func TestDoctorRepair_NoRepairFlag(t *testing.T) {
	// given: stale PID file exists but --repair is NOT passed
	dir := initTestProject(t)
	pidDir := filepath.Join(dir, ".siren")
	pidFile := filepath.Join(pidDir, "watch.pid")
	if err := os.WriteFile(pidFile, []byte("99999"), 0o644); err != nil {
		t.Fatalf("create stale PID: %v", err)
	}

	// when: run doctor --json WITHOUT --repair
	out, _ := runCmd(t, "doctor", "--json", dir)

	// then: PID file should NOT be removed
	result := parseDoctorJSON(t, out)
	for _, check := range result.Checks {
		if check.Name == "stale-pid" {
			t.Errorf("stale-pid check should not appear without --repair flag")
		}
	}

	// Verify PID file still exists
	if _, err := os.Stat(pidFile); err != nil {
		t.Errorf("watch.pid should still exist without --repair: %v", err)
	}
}

// requireBinary skips the test if the named binary is not available.
func requireBinary(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("skipping: %s not found in PATH", name)
	}
}

func init() {
	// Ensure sightjack binary is available for doctor repair tests.
	_ = fmt.Sprintf("doctor_repair_test uses sightjackBin() from subcommand_test.go")
}
