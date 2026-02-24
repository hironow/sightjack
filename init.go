package sightjack

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed all:templates/skills
var skillsFS embed.FS

// RenderInitConfig generates a minimal config.yaml content string.
// Only user-specified values are written; remaining fields are filled
// by DefaultConfig when LoadConfig reads the file.
func RenderInitConfig(team, project, lang, strictness string) string {
	return fmt.Sprintf(`linear:
  team: %q
  project: %q

strictness:
  default: %s

lang: %q
`, team, project, strictness, lang)
}

// InstallSkills copies embedded skill templates into baseDir/.siren/skills/.
// Existing files are overwritten (idempotent). Directories are created as needed.
func InstallSkills(baseDir string) error {
	const srcPrefix = "templates/skills"
	destRoot := filepath.Join(baseDir, StateDir, "skills")

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

		data, err := skillsFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}
		return os.WriteFile(dest, data, 0644)
	})
}
