//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// buildTestContainer starts a sightjack test container once.
func buildTestContainer(t *testing.T, ctx context.Context) testcontainers.Container {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image: sharedImage,
		Cmd:   []string{"sleep", "infinity"},
		WaitingFor: wait.ForExec([]string{"sightjack", "--version"}).
			WithStartupTimeout(10 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("buildTestContainer: %v", err)
	}
	t.Cleanup(func() {
		if err := c.Terminate(ctx); err != nil {
			t.Errorf("terminate container: %v", err)
		}
	})
	return c
}

// execInContainer executes a command inside the test container and returns stdout.
func execInContainer(t *testing.T, ctx context.Context, c testcontainers.Container, cmd []string) string {
	t.Helper()
	code, stdout, stderr := execInContainerWithExitCode(t, ctx, c, cmd)
	if code != 0 {
		t.Fatalf("exec %v failed with code %d\nstdout: %s\nstderr: %s", cmd, code, stdout, stderr)
	}
	return stdout
}

// execInContainerWithExitCode executes a command inside the test container and returns (exitCode, stdout, stderr).
func execInContainerWithExitCode(t *testing.T, ctx context.Context, c testcontainers.Container, cmd []string) (int, string, string) {
	t.Helper()
	code, stdoutReader, err := c.Exec(ctx, cmd)
	if err != nil {
		t.Fatalf("container exec failed: %v", err)
	}
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(stdoutReader)
	return code, buf.String(), ""
}

// heredocWrite writes file content inside the container.
func heredocWrite(t *testing.T, ctx context.Context, c testcontainers.Container, path, content string) {
	t.Helper()
	cmd := []string{"sh", "-c", fmt.Sprintf("cat << 'EOF' > %s\n%s\nEOF", path, content)}
	execInContainer(t, ctx, c, cmd)
}

// runCmd executes sightjack inside the test container.
func runCmd(t *testing.T, ctx context.Context, c testcontainers.Container, dir string, args ...string) (string, string, error) {
	t.Helper()
	fullCmd := []string{"sh", "-c", fmt.Sprintf("cd %s && /usr/local/bin/sightjack %s", dir, strings.Join(args, " "))}
	code, stdout, _ := execInContainerWithExitCode(t, ctx, c, fullCmd)
	var err error
	if code != 0 {
		err = fmt.Errorf("exit code %d", code)
	}
	return stdout, "", err
}

// runCmdStdin executes sightjack inside the test container, piping data to stdin.
func runCmdStdin(t *testing.T, ctx context.Context, c testcontainers.Container, dir, stdin string, args ...string) (string, string, error) {
	t.Helper()
	fullCmd := []string{"sh", "-c", fmt.Sprintf("cat << 'EOF' | (cd %s && /usr/local/bin/sightjack %s)\n%s\nEOF", dir, strings.Join(args, " "), stdin)}
	code, stdout, _ := execInContainerWithExitCode(t, ctx, c, fullCmd)
	var err error
	if code != 0 {
		err = fmt.Errorf("exit code %d", code)
	}
	return stdout, "", err
}

// fileExistsInContainer checks if a file exists inside the container.
func fileExistsInContainer(t *testing.T, ctx context.Context, c testcontainers.Container, path string) bool {
	t.Helper()
	code, _, _ := execInContainerWithExitCode(t, ctx, c, []string{"test", "-f", path})
	return code == 0
}

// dirExistsInContainer checks if a directory exists inside the container.
func dirExistsInContainer(t *testing.T, ctx context.Context, c testcontainers.Container, path string) bool {
	t.Helper()
	code, _, _ := execInContainerWithExitCode(t, ctx, c, []string{"test", "-d", path})
	return code == 0
}

// initTestRepo creates a workspace inside the container, git init, and runs `sightjack init`.
func initTestRepo(t *testing.T, ctx context.Context, c testcontainers.Container, dir string) {
	t.Helper()
	execInContainer(t, ctx, c, []string{"mkdir", "-p", dir})
	execInContainer(t, ctx, c, []string{"sh", "-c", fmt.Sprintf("cd %s && git init --initial-branch=main", dir)})
	execInContainer(t, ctx, c, []string{"sh", "-c", fmt.Sprintf("cd %s && git config user.name 'E2E Test' && git config user.email 'e2e@test.local'", dir)})
	execInContainer(t, ctx, c, []string{"sh", "-c", fmt.Sprintf("cd %s && echo '# test' > README.md && git add . && git commit -m 'initial'", dir)})
	execInContainer(t, ctx, c, []string{"sh", "-c", fmt.Sprintf("cd %s && sightjack init --team TEST --project TestProject .", dir)})
}

// parseJSONOutput parses JSON.
func parseJSONOutput(t *testing.T, stdout string, v any) {
	t.Helper()
	start := strings.Index(stdout, "{")
	if start < 0 {
		t.Fatalf("no JSON object found: %s", stdout)
	}
	end := strings.LastIndex(stdout, "}")
	if end < 0 || end < start {
		t.Fatalf("no closing JSON brace found: %s", stdout)
	}
	jsonStr := stdout[start : end+1]
	if err := json.Unmarshal([]byte(jsonStr), v); err != nil {
		t.Fatalf("parse JSON: %v\nraw: %s", err, jsonStr)
	}
}
