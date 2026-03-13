//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func requireDocker(t *testing.T) {
	t.Helper()
	dockerPath, err := exec.LookPath("docker")
	if err != nil || dockerPath == "" {
		t.Skip("skipping: docker not found in PATH")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("skipping: docker daemon not available")
	}
}

func buildDoctorContainer(t *testing.T, ctx context.Context) testcontainers.Container {
	t.Helper()
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:       "../..",
				Dockerfile:    "tests/e2e/testdata/Dockerfile.test",
				PrintBuildLog: true,
			},
			WaitingFor: wait.ForExec([]string{"sightjack", "version"}).
				WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	}
	container, err := testcontainers.GenericContainer(ctx, req)
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}
	t.Cleanup(func() {
		if cErr := container.Terminate(ctx); cErr != nil {
			t.Logf("failed to terminate container: %v", cErr)
		}
	})
	return container
}

func execCmd(t *testing.T, ctx context.Context, c testcontainers.Container, cmd []string) string {
	t.Helper()
	code, output := execCmdNoFail(t, ctx, c, cmd)
	if code != 0 {
		t.Fatalf("command %v failed (exit %d): %s", cmd, code, output)
	}
	return output
}

func execCmdNoFail(t *testing.T, ctx context.Context, c testcontainers.Container, cmd []string) (int, string) {
	t.Helper()
	code, reader, err := c.Exec(ctx, cmd)
	if err != nil {
		t.Fatalf("exec error: %v", err)
	}
	buf := new(strings.Builder)
	if reader != nil {
		b := make([]byte, 4096)
		for {
			n, readErr := reader.Read(b)
			if n > 0 {
				buf.Write(b[:n])
			}
			if readErr != nil {
				break
			}
		}
	}
	return code, buf.String()
}

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
	// The output may contain Docker multiplexing header bytes; find the JSON object
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

func initProject(t *testing.T, ctx context.Context, c testcontainers.Container, workDir string) {
	t.Helper()
	// Initialize git repo (required for sightjack init)
	execCmd(t, ctx, c, []string{"git", "init", workDir})
	execCmd(t, ctx, c, []string{"git", "-C", workDir, "config", "user.email", "test@test.com"})
	execCmd(t, ctx, c, []string{"git", "-C", workDir, "config", "user.name", "test"})
	// Initialize sightjack project
	execCmd(t, ctx, c, []string{
		"sightjack", "init",
		"--team", "TEST",
		"--project", "TestProject",
		workDir,
	})
}

func TestDoctorRepair_StalePID(t *testing.T) {
	requireDocker(t)
	// given: a stale watch.pid with a dead PID
	ctx := context.Background()
	c := buildDoctorContainer(t, ctx)
	workDir := "/workspace/test-stale-pid"

	initProject(t, ctx, c, workDir)

	// Create stale PID file with a dead process ID
	pidDir := fmt.Sprintf("%s/.siren", workDir)
	execCmd(t, ctx, c, []string{"mkdir", "-p", pidDir})
	execCmd(t, ctx, c, []string{"sh", "-c", fmt.Sprintf("echo 99999 > %s/watch.pid", pidDir)})

	// Verify PID file exists
	execCmd(t, ctx, c, []string{"test", "-f", fmt.Sprintf("%s/watch.pid", pidDir)})

	// when: run doctor --repair --json
	code, output := execCmdNoFail(t, ctx, c, []string{
		"sightjack", "doctor", "--repair", "--json", workDir,
	})

	// then: PID file should be removed and output should contain "fixed"/"FIX"
	t.Logf("doctor --repair output (exit %d): %s", code, output)

	result := parseDoctorJSON(t, output)
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
		t.Errorf("stale-pid check not found in results")
	}

	// Verify PID file was actually removed
	exitCode, _ := execCmdNoFail(t, ctx, c, []string{"test", "-f", fmt.Sprintf("%s/watch.pid", pidDir)})
	if exitCode == 0 {
		t.Error("watch.pid should have been removed but still exists")
	}
}

func TestDoctorRepair_MissingSkillMD(t *testing.T) {
	requireDocker(t)
	// given: initialized project with SKILL.md deleted
	ctx := context.Background()
	c := buildDoctorContainer(t, ctx)
	workDir := "/workspace/test-missing-skill"

	initProject(t, ctx, c, workDir)

	// Delete SKILL.md files
	execCmd(t, ctx, c, []string{"sh", "-c", fmt.Sprintf("rm -f %s/.siren/skills/dmail-sendable/SKILL.md %s/.siren/skills/dmail-readable/SKILL.md", workDir, workDir)})

	// when: run doctor --repair --json
	code, output := execCmdNoFail(t, ctx, c, []string{
		"sightjack", "doctor", "--repair", "--json", workDir,
	})

	// then: SKILL.md should be regenerated
	t.Logf("doctor --repair output (exit %d): %s", code, output)

	result := parseDoctorJSON(t, output)
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
	execCmd(t, ctx, c, []string{"test", "-f", fmt.Sprintf("%s/.siren/skills/dmail-sendable/SKILL.md", workDir)})
	execCmd(t, ctx, c, []string{"test", "-f", fmt.Sprintf("%s/.siren/skills/dmail-readable/SKILL.md", workDir)})
}

func TestDoctorRepair_NoRepairFlag(t *testing.T) {
	requireDocker(t)
	// given: stale PID file exists but --repair is NOT passed
	ctx := context.Background()
	c := buildDoctorContainer(t, ctx)
	workDir := "/workspace/test-no-repair"

	initProject(t, ctx, c, workDir)

	// Create stale PID file
	pidDir := fmt.Sprintf("%s/.siren", workDir)
	execCmd(t, ctx, c, []string{"sh", "-c", fmt.Sprintf("echo 99999 > %s/watch.pid", pidDir)})

	// when: run doctor --json WITHOUT --repair
	_, output := execCmdNoFail(t, ctx, c, []string{
		"sightjack", "doctor", "--json", workDir,
	})

	// then: PID file should NOT be removed (no stale-pid check in output)
	result := parseDoctorJSON(t, output)
	for _, check := range result.Checks {
		if check.Name == "stale-pid" {
			t.Errorf("stale-pid check should not appear without --repair flag")
		}
	}

	// Verify PID file still exists
	execCmd(t, ctx, c, []string{"test", "-f", fmt.Sprintf("%s/watch.pid", pidDir)})
}
