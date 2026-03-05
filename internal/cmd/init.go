package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/session"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Create .siren/config.yaml",
		Long: `Initialize a new sightjack project by creating .siren/config.yaml.

Use --team and --project flags for non-interactive mode, or omit
flags for interactive prompts. Also creates .gitignore, installs
d-mail skills, and sets up mail directories.`,
		Example: `  # Non-interactive with flags
  sightjack init --team Engineering --project Hades

  # Initialize in a specific directory
  sightjack init --team Engineering --project Hades /path/to/project

  # Defaults only (no prompts)
  sightjack init /path/to/project`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			team, _ := cmd.Flags().GetString("team")
			project, _ := cmd.Flags().GetString("project")
			lang, _ := cmd.Flags().GetString("lang")
			strictness, _ := cmd.Flags().GetString("strictness")
			if err := initProject(baseDir, team, project, lang, strictness, cmd.OutOrStdout()); err != nil {
				return err
			}
			otelBackend, _ := cmd.Flags().GetString("otel-backend")
			otelEntity, _ := cmd.Flags().GetString("otel-entity")
			otelProject, _ := cmd.Flags().GetString("otel-project")
			return writeOtelEnv(baseDir, otelBackend, otelEntity, otelProject, cmd.OutOrStdout())
		},
	}
	cmd.Flags().String("team", "", "Linear team name")
	cmd.Flags().String("project", "", "Linear project name")
	cmd.Flags().String("lang", "ja", "Language (ja/en)")
	cmd.Flags().String("strictness", "fog", "Strictness level (fog/alert/lockdown)")
	cmd.Flags().String("otel-backend", "", "OTel backend: jaeger, weave")
	cmd.Flags().String("otel-entity", "", "Weave entity/team (required for weave)")
	cmd.Flags().String("otel-project", "", "Weave project (required for weave)")
	return cmd
}

// initProject creates .siren/config.yaml and supporting files using the
// provided values directly (no interactive prompts). Empty lang or strictness
// are filled with defaults ("ja" and "fog").
func initProject(baseDir, team, project, lang, strictness string, w io.Writer) error {
	if lang == "" {
		lang = "ja"
	}
	if strictness == "" {
		strictness = "fog"
	}

	cfgPath := domain.ConfigPath(baseDir)
	if _, err := os.Stat(cfgPath); err == nil {
		return fmt.Errorf(".siren/config.yaml already exists in %s", baseDir)
	}

	sirenDir := filepath.Join(baseDir, domain.StateDir)
	if err := os.MkdirAll(sirenDir, 0755); err != nil {
		return fmt.Errorf("create .siren dir: %w", err)
	}

	content := platform.RenderInitConfig(team, project, lang, strictness)
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	_ = session.WriteGitIgnore(baseDir)

	if err := session.InstallSkills(baseDir, platform.SkillsFS); err != nil {
		fmt.Fprintf(w, "Warning: failed to install skills: %v\n", err)
	}

	if err := session.EnsureMailDirs(baseDir); err != nil {
		fmt.Fprintf(w, "Warning: failed to create mail dirs: %v\n", err)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Created .siren/config.yaml\n")
	return nil
}

// writeOtelEnv writes .otel.env to the siren state directory if backend is set.
func writeOtelEnv(baseDir, backend, entity, project string, w io.Writer) error {
	if backend == "" {
		return nil
	}
	content, err := platform.OtelEnvContent(backend, entity, project)
	if err != nil {
		return err
	}
	stateDir := filepath.Join(baseDir, domain.StateDir)
	otelPath := filepath.Join(stateDir, ".otel.env")
	if err := os.WriteFile(otelPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write .otel.env: %w", err)
	}
	fmt.Fprintf(w, "OTel backend configured: %s → %s\n", backend, otelPath)
	return nil
}
