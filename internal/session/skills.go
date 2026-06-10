package session

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hironow/sightjack/internal/domain"
)

// InstallSkills copies embedded skill templates into baseDir/.siren/skills/.
// Existing files are overwritten when content differs (idempotent).
// Directories are created as needed. Logs to logger when a file is updated.
func InstallSkills(baseDir string, skillsFS fs.FS, logger domain.Logger) error {
	return installSkillTree(skillsFS, "templates/skills", filepath.Join(baseDir, domain.StateDir, "skills"), logger)
}

// installSkillTree copies an embedded skill tree rooted at srcPrefix
// into destRoot, creating directories and updating files only when the
// template content changed (idempotent re-install).
func installSkillTree(skillsFS fs.FS, srcPrefix, destRoot string, logger domain.Logger) error { // nosemgrep: domain-primitives.multiple-string-params-go -- srcPrefix/destRoot are orthogonal tree roles within a package-private helper [permanent]
	if logger == nil {
		logger = &domain.NopLogger{}
	}

	return fs.WalkDir(skillsFS, srcPrefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(srcPrefix, path)
		if relErr != nil {
			return fmt.Errorf("relative path for %s: %w", path, relErr)
		}
		dest := filepath.Join(destRoot, rel)

		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}

		data, err := fs.ReadFile(skillsFS, path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}
		existing, readErr := os.ReadFile(dest)
		if readErr != nil || !bytes.Equal(existing, data) {
			if readErr == nil {
				logger.Info("updated SKILL.md: %s (template changed)", rel)
			}
			return os.WriteFile(dest, data, 0644)
		}
		return nil
	})
}

// InstallClaudeSkills materializes the embedded Claude Code entry
// skills into the target project's .claude/skills/ (refs issue 0032,
// decision D5(a)): a bare `claude` session auto-discovers project
// skills there, so /sightjack-scan works without plugin machinery or
// launch flags. Idempotent: files are rewritten only when the embedded
// template changed.
func InstallClaudeSkills(baseDir string, skillsFS fs.FS, logger domain.Logger) error {
	return installSkillTree(skillsFS, "templates/claude-skills", filepath.Join(baseDir, ".claude", "skills"), logger)
}
