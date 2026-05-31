//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func rivalReadGolden(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(rivalTestdataDir(t), "expected", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	return data
}

// In container, testdata is located under /src/tests/e2e/testdata/rival
func rivalInputPathInContainer() string {
	return "/src/tests/e2e/testdata/rival/export_input.md"
}

func TestRivalExportReasonsE2E_StdoutMarkdownDefault(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_rival_md"
	initTestRepo(t, ctx, c, dir)

	input := rivalInputPathInContainer()
	want := rivalReadGolden(t, "export_stdout.md")

	// run in container
	stdout, _, err := runCmd(t, ctx, c, dir, "rival", "export", "reasons", "--input", input)
	if err != nil {
		t.Fatalf("binary failed: %v\noutput=%s", err, stdout)
	}

	stdoutBytes := []byte(stdout)
	if !bytes.Equal(stdoutBytes, want) {
		t.Errorf("stdout != golden\n--- got (%d bytes) ---\n%s\n--- want (%d bytes) ---\n%s",
			len(stdoutBytes), stdoutBytes, len(want), want)
	}
}

func TestRivalExportReasonsE2E_StdoutJSONFormat(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_rival_json"
	initTestRepo(t, ctx, c, dir)

	input := rivalInputPathInContainer()
	want := rivalReadGolden(t, "export_stdout.json")

	stdout, _, err := runCmd(t, ctx, c, dir, "rival", "export", "reasons", "--input", input, "--format", "json")
	if err != nil {
		t.Fatalf("binary failed: %v\noutput=%s", err, stdout)
	}

	stdoutBytes := []byte(stdout)
	if !bytes.Equal(stdoutBytes, want) {
		t.Errorf("stdout JSON != golden\n--- got (%d bytes) ---\n%s\n--- want (%d bytes) ---\n%s",
			len(stdoutBytes), stdoutBytes, len(want), want)
	}
}

func TestRivalExportReasonsE2E_FileOutput(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_rival_file"
	initTestRepo(t, ctx, c, dir)

	input := rivalInputPathInContainer()
	outPath := "/workspace/t_rival_file/canvas.md"
	want := rivalReadGolden(t, "export_stdout.md")

	stdout, _, err := runCmd(t, ctx, c, dir, "rival", "export", "reasons", "--input", input, "--output", outPath)
	if err != nil {
		t.Fatalf("binary failed: %v\noutput=%s", err, stdout)
	}

	if len(strings.TrimSpace(stdout)) != 0 {
		t.Errorf("stdout should be empty when --output is set, got: %s", stdout)
	}

	// Read output file from container
	gotStr := execInContainer(t, ctx, c, []string{"cat", outPath})
	got := []byte(gotStr)
	if !bytes.Equal(got, want) {
		t.Errorf("file content != golden\n--- got (%d bytes) ---\n%s\n--- want (%d bytes) ---\n%s",
			len(got), got, len(want), want)
	}
}

func TestRivalExportReasonsE2E_NonexistentInputExitsNonZero(t *testing.T) {
	ctx := context.Background()
	c := buildTestContainer(t, ctx)
	dir := "/workspace/t_rival_missing"
	initTestRepo(t, ctx, c, dir)

	missing := "/workspace/t_rival_missing/does-not-exist.md"

	// Using execInContainerWithExitCode to assert on non-zero exit code
	fullCmd := []string{"sh", "-c", fmt.Sprintf("cd %s && /usr/local/bin/sightjack rival export reasons --input %s", dir, missing)}
	code, stdout, stderr := execInContainerWithExitCode(t, ctx, c, fullCmd)

	if code == 0 {
		t.Fatalf("expected non-zero exit for missing input, got success\nstdout=%s\nstderr=%s",
			stdout, stderr)
	}
	if len(strings.TrimSpace(stdout)) != 0 {
		t.Errorf("stdout should be empty on error, got: %s", stdout)
	}
}
