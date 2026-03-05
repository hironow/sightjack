package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func TestRenderInitConfig_BasicOutput(t *testing.T) {
	// given
	team := "Engineering"
	project := "My Project"
	lang := "ja"
	strictness := "alert"

	// when
	output := domain.RenderInitConfig(team, project, lang, strictness)

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
	output := domain.RenderInitConfig("TestTeam", "Test Project", "en", "fog")
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		t.Fatal(err)
	}

	// when: LoadConfig reads it
	cfg, err := session.LoadConfig(path)

	// then: values match
	if err != nil {
		t.Fatalf("LoadConfig failed on rendered config: %v", err)
	}
	if cfg.Tracker.Team != "TestTeam" {
		t.Errorf("team: expected TestTeam, got %s", cfg.Tracker.Team)
	}
	if cfg.Tracker.Project != "Test Project" {
		t.Errorf("project: expected Test Project, got %s", cfg.Tracker.Project)
	}
	if cfg.Lang != "en" {
		t.Errorf("lang: expected en, got %s", cfg.Lang)
	}
	if cfg.Strictness.Default != domain.StrictnessFog {
		t.Errorf("strictness: expected fog, got %s", cfg.Strictness.Default)
	}
}

func TestRenderInitConfig_DefaultStrictness(t *testing.T) {
	// given: fog strictness (default)
	output := domain.RenderInitConfig("Team", "Project", "ja", "fog")

	// when/then: strictness section present with fog
	if !strings.Contains(output, "default: fog") {
		t.Errorf("expected 'default: fog' in output, got:\n%s", output)
	}
}

func TestInstallSkills_CreatesFiles(t *testing.T) {
	// given: empty temp dir as base
	baseDir := t.TempDir()

	// when
	err := session.InstallSkills(baseDir, domain.SkillsFS)

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

	// then: SKILL.md files contain metadata key (Agent Skills spec format)
	if !strings.Contains(string(sendable), "metadata:") {
		t.Errorf("dmail-sendable missing 'metadata:' key, got:\n%s", sendable)
	}
	if !strings.Contains(string(readable), "metadata:") {
		t.Errorf("dmail-readable missing 'metadata:' key, got:\n%s", readable)
	}

	// then: schema version present
	if !strings.Contains(string(sendable), "dmail-schema-version:") {
		t.Errorf("dmail-sendable missing dmail-schema-version, got:\n%s", sendable)
	}
	if !strings.Contains(string(readable), "dmail-schema-version:") {
		t.Errorf("dmail-readable missing dmail-schema-version, got:\n%s", readable)
	}

	// then: readable contains convergence kind
	if !strings.Contains(string(readable), "convergence") {
		t.Errorf("dmail-readable missing convergence kind, got:\n%s", readable)
	}
}

func TestInstallSkills_Idempotent(t *testing.T) {
	// given: install once
	baseDir := t.TempDir()
	if err := session.InstallSkills(baseDir, domain.SkillsFS); err != nil {
		t.Fatalf("first install: %v", err)
	}

	// when: install again
	err := session.InstallSkills(baseDir, domain.SkillsFS)

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
	output := domain.RenderInitConfig("Team", "Project", "ja", "fog")
	dir := t.TempDir()
	path := filepath.Join(dir, "sightjack.yaml")
	os.WriteFile(path, []byte(output), 0644)

	// when
	cfg, err := session.LoadConfig(path)

	// then: DefaultConfig values are applied for unspecified fields
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Scan.ChunkSize != 20 {
		t.Errorf("expected default ChunkSize 20, got %d", cfg.Scan.ChunkSize)
	}
	if cfg.Assistant.Command != "claude" {
		t.Errorf("expected default command 'claude', got %s", cfg.Assistant.Command)
	}
}
