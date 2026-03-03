package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestInitProject_CreatesConfigFile(t *testing.T) {
	// given
	dir := t.TempDir()
	var buf bytes.Buffer

	// when
	err := initProject(dir, "Engineering", "Hades", "en", "alert", &buf)

	// then
	if err != nil {
		t.Fatalf("initProject failed: %v", err)
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
	if !strings.Contains(content, `lang: "en"`) {
		t.Errorf("expected lang 'en' in config, got:\n%s", content)
	}
	if !strings.Contains(content, "default: alert") {
		t.Errorf("expected strictness 'alert' in config, got:\n%s", content)
	}
}

func TestInitProject_DefaultLangAndStrictness(t *testing.T) {
	// given
	dir := t.TempDir()
	var buf bytes.Buffer

	// when
	err := initProject(dir, "Team", "Project", "", "", &buf)

	// then
	if err != nil {
		t.Fatalf("initProject failed: %v", err)
	}
	data, _ := os.ReadFile(domain.ConfigPath(dir))
	content := string(data)
	if !strings.Contains(content, `lang: "ja"`) {
		t.Errorf("expected default lang 'ja' in config, got:\n%s", content)
	}
	if !strings.Contains(content, "default: fog") {
		t.Errorf("expected default strictness 'fog' in config, got:\n%s", content)
	}
}

func TestInitProject_AlreadyExists(t *testing.T) {
	// given
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".siren"), 0755)
	os.WriteFile(domain.ConfigPath(dir), []byte("existing"), 0644)
	var buf bytes.Buffer

	// when
	err := initProject(dir, "Team", "Project", "", "", &buf)

	// then
	if err == nil {
		t.Fatal("expected error when config already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %v", err)
	}
}

func TestInitProject_CreatesGitIgnore(t *testing.T) {
	// given
	dir := t.TempDir()
	var buf bytes.Buffer

	// when
	err := initProject(dir, "Team", "Project", "", "", &buf)

	// then
	if err != nil {
		t.Fatalf("initProject failed: %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(dir, ".siren", ".gitignore"))
	if readErr != nil {
		t.Fatalf(".gitignore not created: %v", readErr)
	}
	if !strings.Contains(string(data), "events/") {
		t.Errorf("expected events/ in .gitignore, got:\n%s", data)
	}
}

func TestInitProject_InstallsSkills(t *testing.T) {
	// given
	dir := t.TempDir()
	var buf bytes.Buffer

	// when
	err := initProject(dir, "Team", "Project", "", "", &buf)

	// then
	if err != nil {
		t.Fatalf("initProject failed: %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(dir, ".siren", "skills", "dmail-sendable", "SKILL.md"))
	if readErr != nil {
		t.Fatalf("dmail-sendable SKILL.md not installed: %v", readErr)
	}
	if !strings.Contains(string(data), "name: dmail-sendable") {
		t.Errorf("unexpected dmail-sendable content: %s", data)
	}
}

func TestInitProject_CreatesMailDirs(t *testing.T) {
	// given
	dir := t.TempDir()
	var buf bytes.Buffer

	// when
	err := initProject(dir, "Team", "Project", "", "", &buf)

	// then
	if err != nil {
		t.Fatalf("initProject failed: %v", err)
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

// === P1-5: Flag-based init (no interactive prompts) ===

func TestInitCmd_FlagsOnly(t *testing.T) {
	// given — init via cobra command with flags, no stdin
	dir := t.TempDir()
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetIn(strings.NewReader("")) // empty stdin — must NOT hang
	cmd.SetArgs([]string{"init", "--team", "Engineering", "--project", "Hades", dir})

	// when
	err := cmd.Execute()

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

func TestInitCmd_MissingTeamFlag_UsesDefault(t *testing.T) {
	// given — init with no --team flag, should use default (empty or DefaultConfig value)
	dir := t.TempDir()
	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetIn(strings.NewReader("")) // empty stdin
	cmd.SetArgs([]string{"init", dir})

	// when
	err := cmd.Execute()

	// then — should succeed with defaults (no interactive prompt, no hang)
	if err != nil {
		t.Fatalf("init with defaults failed: %v", err)
	}
	cfgPath := domain.ConfigPath(dir)
	if _, readErr := os.Stat(cfgPath); readErr != nil {
		t.Fatalf("config not created: %v", readErr)
	}
}
