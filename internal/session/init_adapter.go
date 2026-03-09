package session

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
)

// InitAdapter implements port.InitRunner by orchestrating project initialization I/O.
type InitAdapter struct {
	Force bool // When true, overwrite existing config.yaml
}

// InitProject creates .siren/config.yaml and supporting files.
// Returns warnings for non-fatal errors (skill install, mail dirs).
func (a *InitAdapter) InitProject(baseDir, team, project, lang, strictness string) ([]string, error) {
	cfgPath := domain.ConfigPath(baseDir)
	if _, err := os.Stat(cfgPath); err == nil && !a.Force {
		return nil, fmt.Errorf(".siren/config.yaml already exists in %s\nUse --force to overwrite", baseDir)
	}

	sirenDir := filepath.Join(baseDir, domain.StateDir)
	if err := os.MkdirAll(sirenDir, 0755); err != nil {
		return nil, fmt.Errorf("create .siren dir: %w", err)
	}

	content := platform.RenderInitConfig(team, project, lang, strictness)
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	_ = WriteGitIgnore(baseDir)

	var warnings []string
	if err := InstallSkills(baseDir, platform.SkillsFS, nil); err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to install skills: %v", err))
	}
	if err := EnsureMailDirs(baseDir); err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to create mail dirs: %v", err))
	}

	return warnings, nil
}
