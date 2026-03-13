package session_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func TestCheckConfig_Fail_HasHint(t *testing.T) {
	// given
	result := session.CheckConfig("/nonexistent/config.yaml")

	// when/then
	if result.Status != domain.CheckFail {
		t.Fatalf("expected FAIL, got %v", result.Status.StatusLabel())
	}
	if result.Hint == "" {
		t.Error("expected non-empty hint for failed config check")
	}
	if !strings.Contains(result.Hint, "sightjack init") {
		t.Errorf("hint should mention sightjack init, got: %s", result.Hint)
	}
}

func TestCheckTool_NotFound_HasHint(t *testing.T) {
	// given
	result := session.CheckTool(context.Background(), "nonexistent-tool-xyz-99999")

	// when/then
	if result.Status != domain.CheckFail {
		t.Fatalf("expected FAIL, got %v", result.Status.StatusLabel())
	}
	if result.Hint == "" {
		t.Error("expected non-empty hint for missing tool")
	}
	if !strings.Contains(result.Hint, "install") {
		t.Errorf("hint should mention install, got: %s", result.Hint)
	}
}

func TestCheckStateDir_CannotCreate_HasHint(t *testing.T) {
	// given — /dev/null is not a directory, MkdirAll will fail
	result := session.CheckStateDir("/dev/null", true)

	// when/then
	if result.Status != domain.CheckFail {
		t.Fatalf("expected FAIL, got %v", result.Status.StatusLabel())
	}
	if result.Hint == "" {
		t.Error("expected non-empty hint for failed state dir check")
	}
}

func TestCheckSkills_Missing_HasHint(t *testing.T) {
	// given — temp dir has no .siren/skills/
	dir := t.TempDir()

	// when
	result := session.CheckSkills(dir)

	// then
	if result.Status != domain.CheckFail {
		t.Fatalf("expected FAIL, got %v", result.Status.StatusLabel())
	}
	if result.Hint == "" {
		t.Error("expected non-empty hint for missing skills")
	}
	if !strings.Contains(result.Hint, "sightjack init") {
		t.Errorf("hint should mention sightjack init, got: %s", result.Hint)
	}
}

func TestCheckSkills_DeprecatedFeedbackKind_Fails(t *testing.T) {
	// given — SKILL.md with deprecated "kind: feedback" (pre-split)
	dir := t.TempDir()
	for _, name := range []string{"dmail-sendable", "dmail-readable"} {
		skillDir := filepath.Join(dir, domain.StateDir, "skills", name)
		os.MkdirAll(skillDir, 0755)
		content := "---\nname: " + name + "\nmetadata:\n  dmail-schema-version: \"1\"\nconsumes:\n    - kind: feedback\n---\n"
		os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)
	}

	// when
	result := session.CheckSkills(dir)

	// then
	if result.Status != domain.CheckFail {
		t.Errorf("expected FAIL for deprecated kind, got %v: %s", result.Status.StatusLabel(), result.Message)
	}
	if !strings.Contains(result.Hint, "init --force") {
		t.Errorf("hint should mention init --force, got: %s", result.Hint)
	}
}

func TestCheckSkills_UpdatedDesignFeedbackKind_Passes(t *testing.T) {
	// given — SKILL.md with updated "kind: design-feedback" (post-split)
	dir := t.TempDir()
	for _, name := range []string{"dmail-sendable", "dmail-readable"} {
		skillDir := filepath.Join(dir, domain.StateDir, "skills", name)
		os.MkdirAll(skillDir, 0755)
		content := "---\nname: " + name + "\nmetadata:\n  dmail-schema-version: \"1\"\nconsumes:\n    - kind: design-feedback\n---\n"
		os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)
	}

	// when
	result := session.CheckSkills(dir)

	// then
	if result.Status != domain.CheckOK {
		t.Errorf("expected OK for updated kind, got %v: %s", result.Status.StatusLabel(), result.Message)
	}
}
