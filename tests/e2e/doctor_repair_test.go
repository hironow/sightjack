//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestDoctorRepair_StalePID(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_stale_pid"

	initTestRepo(t, ctx, c, dir)
	pidFile := fmt.Sprintf("%s/.siren/watch.pid", dir)
	
	// Create stale PID file inside container
	execInContainer(t, ctx, c, []string{"sh", "-c", fmt.Sprintf("echo '99999' > %s", pidFile)})

	// when: run doctor --repair --json
	out, _, _ := runCmd(t, ctx, c, dir, "doctor", "--repair", "--json", dir)

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
	if fileExistsInContainer(t, ctx, c, pidFile) {
		t.Error("watch.pid should have been removed but still exists")
	}
}

func TestDoctorRepair_MissingSkillMD(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_missing_skill"

	initTestRepo(t, ctx, c, dir)
	skillPaths := []string{
		fmt.Sprintf("%s/.siren/skills/dmail-sendable/SKILL.md", dir),
		fmt.Sprintf("%s/.siren/skills/dmail-readable/SKILL.md", dir),
	}
	for _, p := range skillPaths {
		execInContainer(t, ctx, c, []string{"rm", "-f", p})
	}

	// when: run doctor --repair --json
	out, _, _ := runCmd(t, ctx, c, dir, "doctor", "--repair", "--json", dir)

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
		if !fileExistsInContainer(t, ctx, c, p) {
			t.Errorf("expected %s to be regenerated", p)
		}
	}
}

func TestDoctorRepair_NoRepairFlag(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_norepair"

	initTestRepo(t, ctx, c, dir)
	pidFile := fmt.Sprintf("%s/.siren/watch.pid", dir)
	
	// Create stale PID file inside container
	execInContainer(t, ctx, c, []string{"sh", "-c", fmt.Sprintf("echo '99999' > %s", pidFile)})

	// when: run doctor --json WITHOUT --repair
	out, _, _ := runCmd(t, ctx, c, dir, "doctor", "--json", dir)

	// then: PID file should NOT be removed
	result := parseDoctorJSON(t, out)
	for _, check := range result.Checks {
		if check.Name == "stale-pid" {
			t.Errorf("stale-pid check should not appear without --repair flag")
		}
	}

	// Verify PID file still exists
	if !fileExistsInContainer(t, ctx, c, pidFile) {
		t.Error("watch.pid should still exist without --repair")
	}
}
