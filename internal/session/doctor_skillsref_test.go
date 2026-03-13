// white-box-reason: tests unexported findSkillsRefDir function
package session

import (
	"os"
	"path/filepath"
	"testing"
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
