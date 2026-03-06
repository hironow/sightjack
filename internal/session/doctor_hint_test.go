package session

// white-box-reason: session internals: tests unexported CheckConfig/CheckTool hint generation

import (
	"context"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestCheckConfig_Fail_HasHint(t *testing.T) {
	// given
	result := CheckConfig("/nonexistent/config.yaml")

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
	result := CheckTool(context.Background(), "nonexistent-tool-xyz-99999")

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
	result := CheckStateDir("/dev/null")

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
	result := CheckSkills(dir)

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
