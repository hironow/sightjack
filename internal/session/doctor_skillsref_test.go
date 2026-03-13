// white-box-reason: tests unexported findSkillsRefDir and checkSkillsRefToolchain functions
package session

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestFindSkillsRefDir_UsesBaseDir_NotCWD(t *testing.T) {
	// given: a baseDir containing ../skills-ref relative to itself
	baseDir := t.TempDir()
	skillsRefDir := filepath.Join(baseDir, "..", "skills-ref")
	if err := os.MkdirAll(skillsRefDir, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(skillsRefDir)

	// CWD is NOT baseDir — it's wherever the test runner is.
	// The function should find skills-ref relative to baseDir, not CWD.

	// when
	result := findSkillsRefDir(baseDir)

	// then: should find the skills-ref directory relative to baseDir
	if result == "" {
		t.Error("findSkillsRefDir(baseDir) returned empty; want non-empty path relative to baseDir")
	}
}

func TestFindSkillsRefDir_NotFoundWhenAbsent(t *testing.T) {
	// given: a baseDir with no skills-ref nearby
	baseDir := t.TempDir()

	// when
	result := findSkillsRefDir(baseDir)

	// then
	if result != "" {
		t.Errorf("findSkillsRefDir returned %q for dir with no skills-ref; want empty", result)
	}
}

func TestCheckSkillsRefToolchain_SubDirExists_NotOnPath_ReturnsWarn(t *testing.T) {
	// given: skills-ref is NOT on PATH, but a sibling checkout directory exists
	baseDir := t.TempDir()

	// Override lookPath so skills-ref is never found (uv IS found)
	restoreLookPath := OverrideLookPath(func(cmd string) (string, error) {
		if cmd == "uv" {
			return "/usr/bin/uv", nil
		}
		return "", fmt.Errorf("not found: %s", cmd)
	})
	defer restoreLookPath()

	// Override findSkillsRefDirFn to pretend a checkout exists
	restoreFindDir := OverrideFindSkillsRefDir(func(_ string) string {
		return filepath.Join(baseDir, "..", "skills-ref")
	})
	defer restoreFindDir()

	// when: repair=false
	results := checkSkillsRefToolchain(baseDir, false)

	// then: should be WARN, not OK — a directory checkout is not the same as having the executable
	if len(results) == 0 {
		t.Fatal("expected at least one check result")
	}
	if results[0].Status != domain.CheckWarn {
		t.Errorf("expected WARN when subDir exists but skills-ref not on PATH, got %v: %s",
			results[0].Status.StatusLabel(), results[0].Message)
	}
}

func TestCheckSkillsRefToolchain_Repair_VerifiesPathAfterInstall(t *testing.T) {
	// given: skills-ref is NOT on PATH, uv IS found, repair=true
	baseDir := t.TempDir()
	installCalled := false

	restoreLookPath := OverrideLookPath(func(cmd string) (string, error) {
		if cmd == "uv" {
			return "/usr/bin/uv", nil
		}
		// Even after install, skills-ref is not on PATH
		return "", fmt.Errorf("not found: %s", cmd)
	})
	defer restoreLookPath()

	restoreFindDir := OverrideFindSkillsRefDir(func(_ string) string { return "" })
	defer restoreFindDir()

	restoreInstall := OverrideInstallSkillsRef(func() error {
		installCalled = true
		return nil
	})
	defer restoreInstall()

	// when: repair=true but skills-ref still not on PATH after install
	results := checkSkillsRefToolchain(baseDir, true)

	// then: install was called but result should be WARN because skills-ref still not on PATH
	if !installCalled {
		t.Error("expected installSkillsRefFn to be called during repair")
	}
	if len(results) == 0 {
		t.Fatal("expected at least one check result")
	}
	if results[0].Status != domain.CheckWarn {
		t.Errorf("expected WARN when install succeeds but skills-ref still not on PATH, got %v: %s",
			results[0].Status.StatusLabel(), results[0].Message)
	}
}
