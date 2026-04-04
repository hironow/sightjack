package platform

import (
	"embed"
	"fmt"
)

//go:embed all:templates/skills
var SkillsFS embed.FS

// RenderInitConfig generates a minimal config.yaml content string.
// Only user-specified values are written; remaining fields are filled
// by DefaultConfig when LoadConfig reads the file.
func RenderInitConfig(team, project, lang, strictness string) string {
	return fmt.Sprintf(`tracker:
  team: %q
  project: %q

strictness:
  default: %s

lang: %q
`, team, project, strictness, lang)
}
