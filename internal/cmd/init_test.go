package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/cmd"
	"github.com/hironow/sightjack/internal/domain"
)

func TestInitCmd_CreatesConfigFile(t *testing.T) {
	// given
	dir := t.TempDir()
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"init", "--team", "Engineering", "--project", "Hades", "--lang", "en", "--strictness", "alert", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	data, readErr := os.ReadFile(domain.ConfigPath(dir))
	if readErr != nil {
		t.Fatalf("config not created: %v", readErr)
	}
	content := string(data)
	if !strings.Contains(content, "Engineering") {
		t.Errorf("expected team in config, got:\n%s", content)
	}
	if !strings.Contains(content, "Hades") {
		t.Errorf("expected project in config, got:\n%s", content)
	}
	if !strings.Contains(content, "lang: en") {
		t.Errorf("expected lang 'en' in config, got:\n%s", content)
	}
	if !strings.Contains(content, "default: alert") {
		t.Errorf("expected strictness 'alert' in config, got:\n%s", content)
	}
}

func TestInitCmd_DefaultLangAndStrictness(t *testing.T) {
	// given
	dir := t.TempDir()
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"init", "--team", "Team", "--project", "Project", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	data, _ := os.ReadFile(domain.ConfigPath(dir))
	content := string(data)
	if !strings.Contains(content, "lang: ja") {
		t.Errorf("expected default lang 'ja' in config, got:\n%s", content)
	}
	if !strings.Contains(content, "default: fog") {
		t.Errorf("expected default strictness 'fog' in config, got:\n%s", content)
	}
}

func TestInitCmd_AlreadyExists(t *testing.T) {
	// given
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".siren"), 0755)
	os.WriteFile(domain.ConfigPath(dir), []byte("existing"), 0644)
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"init", "--team", "Team", "--project", "Project", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error when config already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %v", err)
	}
}

func TestInitCmd_AlreadyExists_SuggestsForce(t *testing.T) {
	// given
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".siren"), 0755)
	os.WriteFile(domain.ConfigPath(dir), []byte("existing"), 0644)
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"init", "--team", "Team", "--project", "Project", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err == nil {
		t.Fatal("expected error when config already exists")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("expected '--force' hint in error, got: %v", err)
	}
}

func TestInitCmd_Force_OverwritesExisting(t *testing.T) {
	// given: existing config with old content
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".siren"), 0755)
	os.WriteFile(domain.ConfigPath(dir), []byte("old content"), 0644)

	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"init", "--force", "--team", "NewTeam", "--project", "NewProject", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("init --force failed: %v", err)
	}
	data, _ := os.ReadFile(domain.ConfigPath(dir))
	content := string(data)
	if !strings.Contains(content, "NewTeam") {
		t.Errorf("expected 'NewTeam' in overwritten config, got:\n%s", content)
	}
}

func TestInitCmd_CreatesGitIgnore(t *testing.T) {
	// given
	dir := t.TempDir()
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"init", "--team", "Team", "--project", "Project", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(dir, ".siren", ".gitignore"))
	if readErr != nil {
		t.Fatalf(".gitignore not created: %v", readErr)
	}
	if !strings.Contains(string(data), "events/") {
		t.Errorf("expected events/ in .gitignore, got:\n%s", data)
	}
}

func TestInitCmd_InstallsSkills(t *testing.T) {
	// given
	dir := t.TempDir()
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"init", "--team", "Team", "--project", "Project", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(dir, ".siren", "skills", "dmail-sendable", "SKILL.md"))
	if readErr != nil {
		t.Fatalf("dmail-sendable SKILL.md not installed: %v", readErr)
	}
	if !strings.Contains(string(data), "name: dmail-sendable") {
		t.Errorf("unexpected dmail-sendable content: %s", data)
	}
}

func TestInitCmd_CreatesMailDirs(t *testing.T) {
	// given
	dir := t.TempDir()
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"init", "--team", "Team", "--project", "Project", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	for _, sub := range []string{"inbox", "outbox", "archive"} {
		path := filepath.Join(dir, ".siren", sub)
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Errorf("%s not created: %v", sub, statErr)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", sub)
		}
	}
}

func TestInitCmd_FlagsOnly(t *testing.T) {
	// given — init via cobra command with flags, no stdin
	dir := t.TempDir()
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader("")) // empty stdin — must NOT hang
	rootCmd.SetArgs([]string{"init", "--team", "Engineering", "--project", "Hades", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("init with flags failed: %v", err)
	}
	cfgPath := domain.ConfigPath(dir)
	data, readErr := os.ReadFile(cfgPath)
	if readErr != nil {
		t.Fatalf("config not created: %v", readErr)
	}
	content := string(data)
	if !strings.Contains(content, "Engineering") {
		t.Errorf("expected team in config, got:\n%s", content)
	}
	if !strings.Contains(content, "Hades") {
		t.Errorf("expected project in config, got:\n%s", content)
	}
}

func TestInitCmd_Force_MergesExistingConfig(t *testing.T) {
	// given: existing config with user customization
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".siren"), 0755)
	os.WriteFile(domain.ConfigPath(dir), []byte("lang: en\ntracker:\n  team: OldTeam\n  project: OldProject\nretry:\n  max_attempts: 5\n"), 0644)

	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"init", "--force", "--team", "NewTeam", "--project", "NewProject", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("init --force failed: %v", err)
	}
	data, _ := os.ReadFile(domain.ConfigPath(dir))
	content := string(data)
	// CLI flags should win
	if !strings.Contains(content, "NewTeam") {
		t.Errorf("expected CLI team 'NewTeam', got:\n%s", content)
	}
	// User's retry.max_attempts=5 should be preserved
	if !strings.Contains(content, "max_attempts: 5") {
		t.Errorf("expected user's max_attempts=5 preserved, got:\n%s", content)
	}
	// New defaults should appear (e.g. labels section)
	if !strings.Contains(content, "labels:") {
		t.Errorf("expected default labels section, got:\n%s", content)
	}
}

func TestInitCmd_ConfigHasAllDefaults(t *testing.T) {
	// given: fresh init
	dir := t.TempDir()
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"init", "--team", "Team", "--project", "Project", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	data, _ := os.ReadFile(domain.ConfigPath(dir))
	content := string(data)
	// Verify comprehensive defaults are present
	for _, expected := range []string{
		"scan:", "chunk_size:", "retry:", "labels:", "gate:", "scribe:",
	} {
		if !strings.Contains(content, expected) {
			t.Errorf("expected %q in config defaults, got:\n%s", expected, content)
		}
	}
}

func TestInitCmd_MissingTeamFlag_UsesDefault(t *testing.T) {
	// given — init with no --team flag, should use default (empty or DefaultConfig value)
	dir := t.TempDir()
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader("")) // empty stdin
	rootCmd.SetArgs([]string{"init", dir})

	// when
	err := rootCmd.Execute()

	// then — should succeed with defaults (no interactive prompt, no hang)
	if err != nil {
		t.Fatalf("init with defaults failed: %v", err)
	}
	cfgPath := domain.ConfigPath(dir)
	if _, readErr := os.Stat(cfgPath); readErr != nil {
		t.Fatalf("config not created: %v", readErr)
	}
}

func TestInitCmd_OtelBackend_CreatesOtelEnv(t *testing.T) {
	// given
	dir := t.TempDir()
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"init", "--team", "TEST", "--project", "TEST", "--otel-backend", "jaeger", dir})

	// when
	err := rootCmd.Execute()

	// then
	if err != nil {
		t.Fatalf("init --otel-backend jaeger failed: %v", err)
	}
	otelPath := filepath.Join(dir, ".siren", ".otel.env")
	data, readErr := os.ReadFile(otelPath)
	if readErr != nil {
		t.Fatalf(".otel.env not created: %v", readErr)
	}
	if !strings.Contains(string(data), "OTEL_EXPORTER_OTLP_ENDPOINT") {
		t.Errorf("expected OTEL_EXPORTER_OTLP_ENDPOINT in .otel.env, got:\n%s", data)
	}
}

func TestInitCmd_Snapshot(t *testing.T) {
	// given — fresh init
	dir := t.TempDir()
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"init", dir})

	// when
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// then — walk state dir and verify exact file list
	stateDir := filepath.Join(dir, domain.StateDir)
	got := walkStateDir(t, stateDir)

	want := []string{
		".gitignore",
		".run/",
		"archive/",
		"config.yaml",
		"events/",
		"inbox/",
		"insights/",
		"outbox/",
		"skills/",
		"skills/dmail-readable/",
		"skills/dmail-readable/SKILL.md",
		"skills/dmail-sendable/",
		"skills/dmail-sendable/SKILL.md",
	}

	if !slices.Equal(want, got) {
		t.Errorf("init snapshot mismatch\nwant: %v\ngot:  %v", want, got)
	}
}

func TestInitCmd_ConfigHeader(t *testing.T) {
	// given — fresh init
	dir := t.TempDir()
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"init", dir})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// when
	data, err := os.ReadFile(domain.ConfigPath(dir))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	// then — first line is a comment
	if !strings.HasPrefix(string(data), "# sightjack configuration") {
		t.Errorf("expected config header comment, got:\n%s", string(data)[:min(len(data), 80)])
	}
}

func TestInitCmd_GitignoreComplete(t *testing.T) {
	// given — fresh init
	dir := t.TempDir()
	rootCmd := cmd.NewRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	rootCmd.SetArgs([]string{"init", dir})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// when
	data, err := os.ReadFile(filepath.Join(dir, domain.StateDir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	content := string(data)

	// then — all runtime directories must be covered
	for _, entry := range []string{"events/", ".run/", "inbox/", "outbox/", "archive/", "insights/", ".otel.env", ".mcp.json", ".claude/"} {
		if !strings.Contains(content, entry) {
			t.Errorf("expected %q in .gitignore, got:\n%s", entry, content)
		}
	}
}

// walkStateDir returns a sorted list of relative paths under stateDir.
// Directories have a trailing "/".
func walkStateDir(t *testing.T, stateDir string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(stateDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(stateDir, path)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			rel += "/"
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk state dir: %v", err)
	}
	sort.Strings(paths)
	return paths
}

func TestInitCmd_OtelFlags_Exist(t *testing.T) {
	// given
	rootCmd := cmd.NewRootCommand()

	// when — find init subcommand
	var initCmd *cobra.Command
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "init" {
			initCmd = sub
			break
		}
	}
	if initCmd == nil {
		t.Fatal("init subcommand not found")
	}

	// then — otel flags exist
	for _, flag := range []string{"otel-backend", "otel-entity", "otel-project"} {
		if initCmd.Flags().Lookup(flag) == nil {
			t.Errorf("init flag --%s not found", flag)
		}
	}
}
