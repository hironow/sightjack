package session

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/usecase/port"
	"gopkg.in/yaml.v3"
)

// InitAdapter implements port.InitRunner by orchestrating project initialization I/O.
type InitAdapter struct {
	Force bool // When true, overwrite existing config.yaml
}

// InitProject creates .siren/config.yaml and supporting files.
// Returns warnings for non-fatal errors (skill install, mail dirs).
func (a *InitAdapter) InitProject(baseDir string, opts ...port.InitOption) ([]string, error) {
	cfg := port.ApplyInitOptions(opts...)
	team, project, lang, strictness := cfg.Team, cfg.Project, cfg.Lang, cfg.Strictness
	cfgPath := domain.ConfigPath(baseDir)
	if _, err := os.Stat(cfgPath); err == nil && !a.Force {
		return nil, fmt.Errorf(".siren/config.yaml already exists in %s\nUse --force to overwrite", baseDir)
	}

	sirenDir := filepath.Join(baseDir, domain.StateDir)

	// Create standard directory structure
	if err := EnsureStateDir(sirenDir, WithMailDirs()); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}

	if err := writeConfigWithDefaults(cfgPath, team, project, lang, strictness); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	// Gitignore (append-only)
	_ = WriteGitIgnore(baseDir)

	// Skills installation (best-effort)
	var warnings []string
	if err := InstallSkills(baseDir, platform.SkillsFS, nil); err != nil {
		warnings = append(warnings, fmt.Sprintf("failed to install skills: %v", err))
	}

	return warnings, nil
}

// writeConfigWithDefaults writes config.yaml with all defaults populated.
// If an existing config.yaml exists, user values are preserved (merged over defaults).
// CLI-provided values (team, project, lang, strictness) always win.
func writeConfigWithDefaults(cfgPath, team, project, lang, strictness string) error {
	cfg := domain.DefaultConfig()

	// Apply CLI flags
	if team != "" {
		cfg.Tracker.Team = team
	}
	if project != "" {
		cfg.Tracker.Project = project
	}
	if lang != "" {
		cfg.Lang = lang
	}
	if strictness != "" {
		cfg.Strictness.Default = domain.StrictnessLevel(strictness)
	}

	// Merge existing config if present and valid YAML
	existing, readErr := os.ReadFile(cfgPath)
	if readErr == nil && len(existing) > 0 {
		var existingMap map[string]any
		if yamlErr := yaml.Unmarshal(existing, &existingMap); yamlErr == nil {
			var defaultMap map[string]any
			defaultData, marshalErr := yaml.Marshal(cfg)
			if marshalErr != nil {
				return marshalErr
			}
			if err := yaml.Unmarshal(defaultData, &defaultMap); err != nil {
				return err
			}

			// existing values override defaults
			deepMerge(defaultMap, existingMap)

			// CLI flags override everything: re-apply on top
			cliOverrides := make(map[string]any)
			trackerOverrides := make(map[string]any)
			if team != "" {
				trackerOverrides["team"] = team
			}
			if project != "" {
				trackerOverrides["project"] = project
			}
			if len(trackerOverrides) > 0 {
				cliOverrides["tracker"] = trackerOverrides
			}
			if lang != "" {
				cliOverrides["lang"] = lang
			}
			if strictness != "" {
				cliOverrides["strictness"] = map[string]any{"default": strictness}
			}
			deepMerge(defaultMap, cliOverrides)

			merged, err := yaml.Marshal(defaultMap)
			if err != nil {
				return err
			}
			// Round-trip through struct for validation
			var mergedCfg domain.Config
			if err := yaml.Unmarshal(merged, &mergedCfg); err != nil {
				return err
			}
			cfg = mergedCfg
		}
		// If YAML is invalid, fall through to write fresh defaults
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, data, 0644)
}

// deepMerge merges src into dst recursively. src values override dst values.
func deepMerge(dst, src map[string]any) {
	for k, sv := range src {
		dv, exists := dst[k]
		if !exists {
			dst[k] = sv
			continue
		}
		srcMap, srcOK := sv.(map[string]any)
		dstMap, dstOK := dv.(map[string]any)
		if srcOK && dstOK {
			deepMerge(dstMap, srcMap)
		} else {
			dst[k] = sv
		}
	}
}
