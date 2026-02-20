package sightjack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderInitConfig_BasicOutput(t *testing.T) {
	// given
	team := "Engineering"
	project := "My Project"
	lang := "ja"
	strictness := "alert"

	// when
	output := RenderInitConfig(team, project, lang, strictness)

	// then
	if !strings.Contains(output, `team: "Engineering"`) {
		t.Errorf("expected team in output, got:\n%s", output)
	}
	if !strings.Contains(output, `project: "My Project"`) {
		t.Errorf("expected project in output, got:\n%s", output)
	}
	if !strings.Contains(output, "default: alert") {
		t.Errorf("expected strictness in output, got:\n%s", output)
	}
	if !strings.Contains(output, `lang: "ja"`) {
		t.Errorf("expected lang in output, got:\n%s", output)
	}
}

func TestRenderInitConfig_LoadableByLoadConfig(t *testing.T) {
	// given: rendered config written to temp file
	output := RenderInitConfig("TestTeam", "Test Project", "en", "fog")
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		t.Fatal(err)
	}

	// when: LoadConfig reads it
	cfg, err := LoadConfig(path)

	// then: values match
	if err != nil {
		t.Fatalf("LoadConfig failed on rendered config: %v", err)
	}
	if cfg.Linear.Team != "TestTeam" {
		t.Errorf("team: expected TestTeam, got %s", cfg.Linear.Team)
	}
	if cfg.Linear.Project != "Test Project" {
		t.Errorf("project: expected Test Project, got %s", cfg.Linear.Project)
	}
	if cfg.Lang != "en" {
		t.Errorf("lang: expected en, got %s", cfg.Lang)
	}
	if cfg.Strictness.Default != StrictnessFog {
		t.Errorf("strictness: expected fog, got %s", cfg.Strictness.Default)
	}
}

func TestRenderInitConfig_DefaultStrictness(t *testing.T) {
	// given: fog strictness (default)
	output := RenderInitConfig("Team", "Project", "ja", "fog")

	// when/then: strictness section present with fog
	if !strings.Contains(output, "default: fog") {
		t.Errorf("expected 'default: fog' in output, got:\n%s", output)
	}
}

func TestInstallSkills_CreatesFiles(t *testing.T) {
	// given: empty temp dir as base
	baseDir := t.TempDir()

	// when
	err := InstallSkills(baseDir)

	// then: no error
	if err != nil {
		t.Fatalf("InstallSkills: %v", err)
	}

	// then: dmail-sendable SKILL.md exists with expected content
	sendable, err := os.ReadFile(filepath.Join(baseDir, ".siren", "skills", "dmail-sendable", "SKILL.md"))
	if err != nil {
		t.Fatalf("read dmail-sendable: %v", err)
	}
	if !strings.Contains(string(sendable), "name: dmail-sendable") {
		t.Errorf("dmail-sendable missing expected content, got:\n%s", sendable)
	}

	// then: dmail-readable SKILL.md exists with expected content
	readable, err := os.ReadFile(filepath.Join(baseDir, ".siren", "skills", "dmail-readable", "SKILL.md"))
	if err != nil {
		t.Fatalf("read dmail-readable: %v", err)
	}
	if !strings.Contains(string(readable), "name: dmail-readable") {
		t.Errorf("dmail-readable missing expected content, got:\n%s", readable)
	}
}

func TestInstallSkills_Idempotent(t *testing.T) {
	// given: install once
	baseDir := t.TempDir()
	if err := InstallSkills(baseDir); err != nil {
		t.Fatalf("first install: %v", err)
	}

	// when: install again
	err := InstallSkills(baseDir)

	// then: no error, files still correct
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	sendable, _ := os.ReadFile(filepath.Join(baseDir, ".siren", "skills", "dmail-sendable", "SKILL.md"))
	if !strings.Contains(string(sendable), "name: dmail-sendable") {
		t.Errorf("content corrupted after second install")
	}
}

func TestRenderInitConfig_DefaultsApplied(t *testing.T) {
	// given: rendered config with minimal values
	output := RenderInitConfig("Team", "Project", "ja", "fog")
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(output), 0644)

	// when
	cfg, err := LoadConfig(path)

	// then: DefaultConfig values are applied for unspecified fields
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Scan.ChunkSize != 20 {
		t.Errorf("expected default ChunkSize 20, got %d", cfg.Scan.ChunkSize)
	}
	if cfg.Claude.Command != "claude" {
		t.Errorf("expected default command 'claude', got %s", cfg.Claude.Command)
	}
}
