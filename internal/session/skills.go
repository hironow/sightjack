package session

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hironow/sightjack/internal/domain"
)

// InstallSkills copies embedded skill templates into baseDir/.siren/skills/.
// Existing files are overwritten (idempotent). Directories are created as needed.
func InstallSkills(baseDir string, skillsFS fs.FS) error {
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
		return os.WriteFile(dest, data, 0644)
	})
}
