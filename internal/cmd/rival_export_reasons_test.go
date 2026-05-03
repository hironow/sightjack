// Package cmd rival_export_reasons_test.go: end-to-end tests for the
// `sightjack rival export reasons` subcommand (Phase 1.1B).
//
// Plan: refs/plans/2026-05-03-rival-contract-v1-1-extensions.md §"Phase 1.1B"
//
// The subcommand is a thin wrapper around usecase.ExportToReasonsCanvas /
// ExportToReasonsCanvasJSON. Tests cover the wiring (flags, mutually
// exclusive --input vs --wave, --output writes a file, --format=json,
// conflict resolution) without re-testing the projection's mapping
// (covered by usecase package tests).
package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/cmd"
	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// rivalSpecDMailV1 builds a fully serialized Rival Contract v1 specification
// D-Mail (frontmatter + body) suitable for writing into archive/ or to a
// stand-alone --input file.
func rivalSpecDMailV1(t *testing.T, name, contractID string, revision int, supersedes, body string) []byte {
	t.Helper()
	mail := &domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          name,
		Kind:          domain.KindSpecification,
		Description:   "Rival Contract v1 specification fixture",
		Body:          body,
		Metadata: map[string]string{
			"contract_schema":   "rival-contract-v1",
			"contract_id":       contractID,
			"contract_revision": rivalItoa(revision),
		},
	}
	if supersedes != "" {
		mail.Metadata["supersedes"] = supersedes
	}
	data, err := session.MarshalDMail(mail)
	if err != nil {
		t.Fatalf("marshal dmail %s: %v", name, err)
	}
	return data
}

// rivalItoa avoids strconv for a single use; mirrors filter test helper.
func rivalItoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	var digits [20]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		digits[i] = '-'
	}
	return string(digits[i:])
}

// readValidV1Body returns the canonical valid-v1 body text used across
// Phase 1.1A tests so the subcommand tests share fixture content.
func readValidV1Body(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "harness", "filter", "testdata", "rival", "valid-v1.md"))
	if err != nil {
		t.Fatalf("read valid-v1 body: %v", err)
	}
	return string(data)
}

// writeInputFile drops a fully-serialized D-Mail as a stand-alone .md file
// under tmp and returns its absolute path. Used for --input mode.
func writeInputFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name+".md")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write input %s: %v", path, err)
	}
	return path
}

// writeArchiveDMail drops a D-Mail into <baseDir>/.siren/archive/<name>.md
// so the --wave projection can pick it up via the archive reader.
func writeArchiveDMail(t *testing.T, baseDir, name string, data []byte) {
	t.Helper()
	if err := session.EnsureMailDirs(baseDir); err != nil {
		t.Fatalf("ensure mail dirs: %v", err)
	}
	archiveDir := domain.MailDir(baseDir, domain.ArchiveDir)
	if err := os.WriteFile(filepath.Join(archiveDir, name+".md"), data, 0o600); err != nil {
		t.Fatalf("write archive dmail %s: %v", name, err)
	}
}

func TestRivalExportReasonsCmd_StdoutDefault(t *testing.T) {
	// given a v1 spec D-Mail on disk.
	dir := t.TempDir()
	data := rivalSpecDMailV1(t, "spec-auth_aaaaaaaa", "wave-auth-expiry", 1, "", readValidV1Body(t))
	inputPath := writeInputFile(t, dir, "spec-auth_aaaaaaaa", data)

	rootCmd := cmd.NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"rival", "export", "reasons", "--input", inputPath})

	// when.
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}

	// then stdout receives the markdown projection.
	got := outBuf.String()
	for _, want := range []string{
		"# Add session expiry enforcement",
		"## Requirements",
		"## Sync",
		"Source: D-Mail spec-auth_aaaaaaaa, revision 1, supersedes none",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout missing %q\n%s", want, got)
		}
	}
}

func TestRivalExportReasonsCmd_FileOutput(t *testing.T) {
	// given a v1 spec and a target output path.
	dir := t.TempDir()
	data := rivalSpecDMailV1(t, "spec-auth_aaaaaaaa", "wave-auth-expiry", 1, "", readValidV1Body(t))
	inputPath := writeInputFile(t, dir, "spec-auth_aaaaaaaa", data)
	outPath := filepath.Join(dir, "canvas.md")

	rootCmd := cmd.NewRootCommand()
	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"rival", "export", "reasons", "--input", inputPath, "--output", outPath})

	// when.
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// then the file exists and contains the canvas markdown.
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(got), "# Add session expiry enforcement") {
		t.Errorf("output file missing canvas heading\n%s", got)
	}
	if !strings.Contains(string(got), "## Sync") {
		t.Errorf("output file missing Sync section\n%s", got)
	}
}

func TestRivalExportReasonsCmd_JSONFormat(t *testing.T) {
	dir := t.TempDir()
	data := rivalSpecDMailV1(t, "spec-auth_aaaaaaaa", "wave-auth-expiry", 1, "", readValidV1Body(t))
	inputPath := writeInputFile(t, dir, "spec-auth_aaaaaaaa", data)

	rootCmd := cmd.NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"rival", "export", "reasons", "--input", inputPath, "--format", "json"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}

	var parsed map[string]any
	if err := json.Unmarshal(outBuf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, outBuf.String())
	}
	if title, _ := parsed["title"].(string); title != "Add session expiry enforcement" {
		t.Errorf("title = %q, want %q", title, "Add session expiry enforcement")
	}
}

func TestRivalExportReasonsCmd_InputAndWaveMutuallyExclusive(t *testing.T) {
	rootCmd := cmd.NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"rival", "export", "reasons", "--input", "/tmp/nope.md", "--wave", "wave-x"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when --input and --wave are both set, got nil")
	}
	combined := strings.ToLower(errBuf.String() + " " + err.Error())
	// cobra prints "if any flags in the group [input wave] are set none of
	// the others can be" when MarkFlagsMutuallyExclusive is violated.
	if !strings.Contains(combined, "mutually exclusive") &&
		!strings.Contains(combined, "none of the others") {
		t.Errorf("expected mutually-exclusive message, got: %q / %v", errBuf.String(), err)
	}
}

func TestRivalExportReasonsCmd_RequiresInputOrWave(t *testing.T) {
	rootCmd := cmd.NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"rival", "export", "reasons"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when neither --input nor --wave provided, got nil")
	}
}

func TestRivalExportReasonsCmd_WaveSelectsCurrentRevision(t *testing.T) {
	// given two specs in archive: revision 1 (older) and revision 2 (newer).
	baseDir := t.TempDir()
	body1 := readValidV1Body(t)
	body2 := readValidV1Body(t)
	older := rivalSpecDMailV1(t, "spec-auth_aaaaaaaa", "wave-auth-expiry", 1, "", body1)
	newer := rivalSpecDMailV1(t, "spec-auth_bbbbbbbb", "wave-auth-expiry", 2, "spec-auth_aaaaaaaa", body2)
	writeArchiveDMail(t, baseDir, "spec-auth_aaaaaaaa", older)
	writeArchiveDMail(t, baseDir, "spec-auth_bbbbbbbb", newer)

	rootCmd := cmd.NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"rival", "export", "reasons", "--wave", "wave-auth-expiry", "--base-dir", baseDir})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}

	// then the Sync line points at the newer spec at revision 2.
	want := "Source: D-Mail spec-auth_bbbbbbbb, revision 2, supersedes spec-auth_aaaaaaaa"
	if got := outBuf.String(); !strings.Contains(got, want) {
		t.Errorf("output missing expected Sync line %q\n%s", want, got)
	}
}

func TestRivalExportReasonsCmd_ConflictRejectedByDefault(t *testing.T) {
	// given two specs at the SAME revision with different bodies (conflict).
	baseDir := t.TempDir()
	bodyA := strings.ReplaceAll(readValidV1Body(t), "Add session expiry enforcement", "Variant A")
	bodyB := strings.ReplaceAll(readValidV1Body(t), "Add session expiry enforcement", "Variant B")
	a := rivalSpecDMailV1(t, "spec-auth_aaaaaaaa", "wave-auth-expiry", 1, "", bodyA)
	b := rivalSpecDMailV1(t, "spec-auth_bbbbbbbb", "wave-auth-expiry", 1, "", bodyB)
	writeArchiveDMail(t, baseDir, "spec-auth_aaaaaaaa", a)
	writeArchiveDMail(t, baseDir, "spec-auth_bbbbbbbb", b)

	rootCmd := cmd.NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"rival", "export", "reasons", "--wave", "wave-auth-expiry", "--base-dir", baseDir})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error on contract conflict, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "conflict") {
		t.Errorf("error should mention conflict; got: %v", err)
	}
}

func TestRivalExportReasonsCmd_ConflictAllowedWithFlag(t *testing.T) {
	// given two specs at the SAME revision (conflict), but --allow-conflict
	// permits best-effort export of the lexicographically smaller name.
	baseDir := t.TempDir()
	bodyA := strings.ReplaceAll(readValidV1Body(t), "Add session expiry enforcement", "Variant A")
	bodyB := strings.ReplaceAll(readValidV1Body(t), "Add session expiry enforcement", "Variant B")
	a := rivalSpecDMailV1(t, "spec-auth_aaaaaaaa", "wave-auth-expiry", 1, "", bodyA)
	b := rivalSpecDMailV1(t, "spec-auth_bbbbbbbb", "wave-auth-expiry", 1, "", bodyB)
	writeArchiveDMail(t, baseDir, "spec-auth_aaaaaaaa", a)
	writeArchiveDMail(t, baseDir, "spec-auth_bbbbbbbb", b)

	rootCmd := cmd.NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"rival", "export", "reasons", "--wave", "wave-auth-expiry", "--base-dir", baseDir, "--allow-conflict"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}

	// then the lexicographically smaller D-Mail name wins.
	want := "Source: D-Mail spec-auth_aaaaaaaa, revision 1, supersedes none"
	if got := outBuf.String(); !strings.Contains(got, want) {
		t.Errorf("output missing expected Sync line for conflict-allowed mode %q\n%s", want, got)
	}
	// and stderr carries a warning so the operator knows it was best-effort.
	if !strings.Contains(strings.ToLower(errBuf.String()), "conflict") {
		t.Errorf("stderr should warn about conflict; got: %q", errBuf.String())
	}
}

func TestRivalExportReasonsCmd_WaveMissing(t *testing.T) {
	// given an empty archive (no specs at all).
	baseDir := t.TempDir()
	if err := session.EnsureMailDirs(baseDir); err != nil {
		t.Fatalf("ensure mail dirs: %v", err)
	}

	rootCmd := cmd.NewRootCommand()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"rival", "export", "reasons", "--wave", "wave-missing", "--base-dir", baseDir})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when wave id has no current contract, got nil")
	}
}

func TestRivalExportReasonsCmd_SubcommandRegistered(t *testing.T) {
	// given the root command.
	rootCmd := cmd.NewRootCommand()

	// when traversing rival -> export -> reasons.
	rivalCmd := findSubByName(rootCmd.Commands(), "rival")
	if rivalCmd == nil {
		t.Fatal("rival subcommand not registered")
	}
	exportCmd := findSubByName(rivalCmd.Commands(), "export")
	if exportCmd == nil {
		t.Fatal("rival export subcommand not registered")
	}
	reasonsCmd := findSubByName(exportCmd.Commands(), "reasons")
	if reasonsCmd == nil {
		t.Fatal("rival export reasons subcommand not registered")
	}

	// then the expected flags exist.
	for _, name := range []string{"input", "wave", "output", "format", "allow-conflict", "base-dir"} {
		if reasonsCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected flag --%s on rival export reasons", name)
		}
	}
}

// findSubByName returns the cobra subcommand with the given name, or nil.
func findSubByName(subs []*cobra.Command, name string) *cobra.Command {
	for _, c := range subs {
		if c.Name() == name {
			return c
		}
	}
	return nil
}
