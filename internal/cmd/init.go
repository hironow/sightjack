package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	sightjack "github.com/hironow/sightjack"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [path]",
		Short: "Create .siren/config.yaml interactively",
		Long: `Initialize a new sightjack project by creating .siren/config.yaml.

Interactively prompts for Linear team, project, language, and
strictness level. Also creates .gitignore, installs d-mail skills,
and sets up mail directories. Run 'sightjack doctor' after init
to verify the environment.`,
		Example: `  # Initialize in current directory
  sightjack init

  # Initialize in a specific directory
  sightjack init /path/to/project

  # After init, verify environment
  sightjack init && sightjack doctor`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			return runInit(baseDir, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}

func runInit(baseDir string, r io.Reader, w io.Writer) error {
	cfgPath := sightjack.ConfigPath(baseDir)
	if _, err := os.Stat(cfgPath); err == nil {
		return fmt.Errorf(".siren/config.yaml already exists in %s", baseDir)
	}

	scanner := bufio.NewScanner(r)

	fmt.Fprintln(w, "sightjack init — create .siren/config.yaml")
	fmt.Fprintln(w)

	var team string
	for team == "" {
		fmt.Fprint(w, "Linear team name: ")
		if !scanner.Scan() {
			return fmt.Errorf("unexpected end of input")
		}
		team = strings.TrimSpace(scanner.Text())
	}

	var project string
	for project == "" {
		fmt.Fprint(w, "Linear project name: ")
		if !scanner.Scan() {
			return fmt.Errorf("unexpected end of input")
		}
		project = strings.TrimSpace(scanner.Text())
	}

	lang := "ja"
	for {
		fmt.Fprint(w, "Language (ja/en) [ja]: ")
		if !scanner.Scan() {
			break
		}
		v := strings.TrimSpace(scanner.Text())
		if v == "" {
			break
		}
		if sightjack.ValidLang(v) {
			lang = v
			break
		}
		fmt.Fprintf(w, "  invalid language %q (valid: ja, en)\n", v)
	}

	strictness := "fog"
	for {
		fmt.Fprint(w, "Strictness (fog/alert/lockdown) [fog]: ")
		if !scanner.Scan() {
			break
		}
		v := strings.TrimSpace(scanner.Text())
		if v == "" {
			break
		}
		if _, err := sightjack.ParseStrictnessLevel(v); err == nil {
			strictness = strings.ToLower(v)
			break
		}
		fmt.Fprintf(w, "  invalid strictness %q (valid: fog, alert, lockdown)\n", v)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	sirenDir := filepath.Join(baseDir, ".siren")
	if err := os.MkdirAll(sirenDir, 0755); err != nil {
		return fmt.Errorf("create .siren dir: %w", err)
	}

	content := sightjack.RenderInitConfig(team, project, lang, strictness)
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	_ = sightjack.WriteGitIgnore(baseDir)

	if err := sightjack.InstallSkills(baseDir); err != nil {
		fmt.Fprintf(w, "Warning: failed to install skills: %v\n", err)
	}

	if err := sightjack.EnsureMailDirs(baseDir); err != nil {
		fmt.Fprintf(w, "Warning: failed to create mail dirs: %v\n", err)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Created .siren/config.yaml\n")
	return nil
}
