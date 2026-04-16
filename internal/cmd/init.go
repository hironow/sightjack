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
	"github.com/hironow/sightjack/internal/usecase/port"
)

func newInitCommand() *cobra.Command {
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

  # Re-initialize (overwrite config, keep state)
  sightjack init --force --team Engineering --project Hades

  # Defaults only (no prompts)
  sightjack init /path/to/project`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := resolveTargetDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			team := mustString(cmd, "team")
			project := mustString(cmd, "project")
			lang := mustString(cmd, "lang")
			strictness := mustString(cmd, "strictness")

			// Apply defaults at cmd layer (previously done by WithDefaults)
			if lang == "" {
				lang = domain.DefaultConfig().Lang
			}
			if strictness == "" {
				strictness = string(domain.DefaultConfig().Strictness.Default)
			}

			force := mustBool(cmd, "force")
			adapter := &session.InitAdapter{Force: force}
			_, initErr := adapter.InitProject(
				baseDir,
				port.WithTeam(team),
				port.WithProject(project),
				port.WithLang(lang),
				port.WithStrictness(strictness),
			)
			if initErr != nil {
				return initErr
			}
			if adapter.LastResult != nil {
				session.PrintInitResult(cmd.ErrOrStderr(), adapter.LastResult)
			}

			otelBackend := mustString(cmd, "otel-backend")
			otelEntity := mustString(cmd, "otel-entity")
			otelProject := mustString(cmd, "otel-project")
			return writeOtelEnv(baseDir, otelBackend, otelEntity, otelProject, cmd.ErrOrStderr())
		},
	}
	cmd.Flags().Bool("force", false, "Overwrite existing config (preserves state directories)")
	cmd.Flags().String("team", "", "Linear team key (e.g. MY)")
	cmd.Flags().String("project", "", "Linear project name")
	cmd.Flags().String("lang", "ja", "Language (ja/en)")
	cmd.Flags().String("strictness", "fog", "Strictness level (fog/alert/lockdown)")
	cmd.Flags().String("otel-backend", "", "OTel backend: jaeger, weave")
	cmd.Flags().String("otel-entity", "", "Weave entity/team (required for weave)")
	cmd.Flags().String("otel-project", "", "Weave project (required for weave)")
	return cmd
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
