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
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	const srcPrefix = "templates/skills"
	destRoot := filepath.Join(baseDir, domain.StateDir, "skills")

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
