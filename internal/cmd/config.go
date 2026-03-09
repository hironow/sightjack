package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/hironow/sightjack/internal/session"
)

func newConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "View or update sightjack configuration",
		Long:  "View or update the .siren/config.yaml configuration file.",
		Example: `  sightjack config show
  sightjack config set tracker.team MY
  sightjack config set lang en`,
	}

	configCmd.AddCommand(newConfigShowCmd())
	configCmd.AddCommand(newConfigSetCmd())

	return configCmd
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [path]",
		Short: "Display effective configuration",
		Long:  "Display the effective configuration after applying defaults and clamping.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir, err := resolveBaseDir(args)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			cfg, err := loadConfig(cmd, baseDir)
			if err != nil {
				return err
			}
			out, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("marshal config: %w", err)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value> [path]",
		Short: "Update a configuration value",
		Long: `Update a configuration value in .siren/config.yaml.

Supported keys:
  tracker.team          Linear team key (e.g. MY)
  tracker.project       Linear project name
  tracker.cycle         Linear cycle name
  lang                  Language (ja or en)
  strictness.default    Default strictness level (fog, alert, lockdown)
  scan.chunk_size       Issues per scan chunk
  scan.max_concurrency  Max concurrent scan workers
  assistant.model       Claude model name
  assistant.timeout_sec Claude timeout in seconds
  gate.auto_approve     Auto-approve convergence gate (true/false)
  labels.enabled        Enable Linear label assignment (true/false)
  labels.prefix         Linear label prefix
  labels.ready_label    Linear ready label`,
		Example: `  sightjack config set tracker.team MY
  sightjack config set lang en
  sightjack config set strictness.default alert`,
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			var baseDir string
			var err error
			if len(args) == 3 {
				baseDir, err = resolveBaseDir(args[2:])
			} else {
				baseDir, err = resolveBaseDir(nil)
			}
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}

			cfgPath := resolveConfigPath(cmd, baseDir)
			if err := session.UpdateConfig(cfgPath, key, value); err != nil {
				return err
			}

			logger := loggerFrom(cmd)
			logger.Info("Updated %s = %s", key, value)
			return nil
		},
	}
}
