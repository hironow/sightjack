package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	sightjack "github.com/hironow/sightjack"
)

func TestUsageOutput_ContainsSubcommands(t *testing.T) {
	// given: a FlagSet with our custom usage function
	var buf bytes.Buffer
	fs := flag.NewFlagSet("sightjack", flag.ContinueOnError)
	fs.SetOutput(&buf)
	fs.String("config", ".siren/config.yaml", "Config file path")
	setUsage(fs)

	// when: usage is triggered
	fs.Usage()

	// then: output contains all subcommands
	output := buf.String()
	for _, cmd := range []string{"scan", "waves", "select", "session", "show", "init", "doctor"} {
		if !strings.Contains(output, cmd) {
			t.Errorf("expected usage output to contain %q, got:\n%s", cmd, output)
		}
	}
}

func TestExtractSubcommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantCmd   string
		wantPath  string
		wantFlags []string
		wantErr   bool
	}{
		// --- existing bool-flag cases (path="" expected) ---
		{
			name:      "verbose true before session",
			args:      []string{"--verbose", "true", "session"},
			wantCmd:   "session",
			wantFlags: []string{"--verbose=true"},
		},
		{
			name:      "dry-run false before scan",
			args:      []string{"--dry-run", "false", "scan"},
			wantCmd:   "scan",
			wantFlags: []string{"--dry-run=false"},
		},
		{
			name:      "short verbose true before session",
			args:      []string{"-v", "true", "session"},
			wantCmd:   "session",
			wantFlags: []string{"-v=true"},
		},
		{
			name:      "bool flag without value still works",
			args:      []string{"--verbose", "session"},
			wantCmd:   "session",
			wantFlags: []string{"--verbose"},
		},
		{
			name:      "config with value and bool flag with value",
			args:      []string{"-c", "custom.yaml", "--verbose", "true", "session"},
			wantCmd:   "session",
			wantFlags: []string{"-c", "custom.yaml", "--verbose=true"},
		},
		{
			name:      "existing: config flag before scan",
			args:      []string{"-c", "custom.yaml", "scan"},
			wantCmd:   "scan",
			wantFlags: []string{"-c", "custom.yaml"},
		},
		{
			name:      "no subcommand defaults to scan",
			args:      []string{"--verbose"},
			wantCmd:   "scan",
			wantFlags: []string{"--verbose"},
		},
		{
			name:      "bool flag with equals syntax preserved",
			args:      []string{"--verbose=true", "session"},
			wantCmd:   "session",
			wantFlags: []string{"--verbose=true"},
		},
		{
			name:      "dry-run false is preserved for flag parser",
			args:      []string{"--dry-run", "false", "session"},
			wantCmd:   "session",
			wantFlags: []string{"--dry-run=false"},
		},
		{
			name:    "duplicate subcommands rejected",
			args:    []string{"scan", "show"},
			wantErr: true,
		},
		// --- new path extraction cases ---
		{
			name:     "subcommand with path",
			args:     []string{"scan", "/tmp/repo"},
			wantCmd:  "scan",
			wantPath: "/tmp/repo",
		},
		{
			name:     "path only defaults to scan",
			args:     []string{"/tmp/repo"},
			wantCmd:  "scan",
			wantPath: "/tmp/repo",
		},
		{
			name:    "no path returns empty",
			args:    []string{"scan"},
			wantCmd: "scan",
		},
		{
			name:    "two non-command positionals is error",
			args:    []string{"scan", "/a", "/b"},
			wantErr: true,
		},
		{
			name:     "init with path",
			args:     []string{"init", "/tmp/repo"},
			wantCmd:  "init",
			wantPath: "/tmp/repo",
		},
		{
			name:    "init without path",
			args:    []string{"init"},
			wantCmd: "init",
		},
		{
			name:     "path before subcommand",
			args:     []string{"/tmp/repo", "scan"},
			wantCmd:  "scan",
			wantPath: "/tmp/repo",
		},
		{
			name:     "relative path with subcommand",
			args:     []string{"scan", "./my-project"},
			wantCmd:  "scan",
			wantPath: "./my-project",
		},
		{
			name:      "flags and path combined",
			args:      []string{"-c", "custom.yaml", "scan", "/tmp/repo"},
			wantCmd:   "scan",
			wantPath:  "/tmp/repo",
			wantFlags: []string{"-c", "custom.yaml"},
		},
		{
			name:    "doctor without path",
			args:    []string{"doctor"},
			wantCmd: "doctor",
		},
		{
			name:     "doctor with path",
			args:     []string{"doctor", "/tmp/repo"},
			wantCmd:  "doctor",
			wantPath: "/tmp/repo",
		},
		{
			name:     "mistyped command treated as path",
			args:     []string{"scna"},
			wantCmd:  "scan",
			wantPath: "scna",
		},
		{
			name:     "unknown command treated as path",
			args:     []string{"deploy"},
			wantCmd:  "scan",
			wantPath: "deploy",
		},
		{
			name:     "bare relative path defaults to scan",
			args:     []string{"repo"},
			wantCmd:  "scan",
			wantPath: "repo",
		},
		{
			name:     "bare relative path with subcommand",
			args:     []string{"scan", "repo"},
			wantCmd:  "scan",
			wantPath: "repo",
		},
		// --- --json flag cases ---
		{
			name:      "json flag before scan",
			args:      []string{"--json", "scan"},
			wantCmd:   "scan",
			wantFlags: []string{"--json"},
		},
		{
			name:      "short json flag before scan",
			args:      []string{"-j", "scan"},
			wantCmd:   "scan",
			wantFlags: []string{"-j"},
		},
		{
			name:      "scan with json flag after",
			args:      []string{"scan", "--json"},
			wantCmd:   "scan",
			wantFlags: []string{"--json"},
		},
		{
			name:      "json true before scan",
			args:      []string{"--json", "true", "scan"},
			wantCmd:   "scan",
			wantFlags: []string{"--json=true"},
		},
		// --- waves subcommand cases ---
		{
			name:    "waves subcommand",
			args:    []string{"waves"},
			wantCmd: "waves",
		},
		{
			name:      "waves with verbose",
			args:      []string{"--verbose", "waves"},
			wantCmd:   "waves",
			wantFlags: []string{"--verbose"},
		},
		{
			name:      "waves with config",
			args:      []string{"-c", "custom.yaml", "waves"},
			wantCmd:   "waves",
			wantFlags: []string{"-c", "custom.yaml"},
		},
		// --- select subcommand cases ---
		{
			name:    "select subcommand",
			args:    []string{"select"},
			wantCmd: "select",
		},
		{
			name:      "select with verbose",
			args:      []string{"--verbose", "select"},
			wantCmd:   "select",
			wantFlags: []string{"--verbose"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, path, flags, err := extractSubcommand(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got cmd=%q path=%q flags=%v", cmd, path, flags)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd != tt.wantCmd {
				t.Errorf("cmd: expected %q, got %q", tt.wantCmd, cmd)
			}
			if path != tt.wantPath {
				t.Errorf("path: expected %q, got %q", tt.wantPath, path)
			}
			if !slices.Equal(flags, tt.wantFlags) {
				t.Errorf("flags: expected %v, got %v", tt.wantFlags, flags)
			}
		})
	}
}

func TestRunInit_CreatesConfigFile(t *testing.T) {
	// given: empty directory with stdin providing all 4 answers
	dir := t.TempDir()
	input := strings.NewReader("Engineering\nMy Project\nja\nalert\n")
	var output bytes.Buffer

	// when
	err := runInit(dir, input, &output)

	// then
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}
	cfgPath := sightjack.ConfigPath(dir)
	data, readErr := os.ReadFile(cfgPath)
	if readErr != nil {
		t.Fatalf("config not created: %v", readErr)
	}
	content := string(data)
	if !strings.Contains(content, "Engineering") {
		t.Errorf("expected team in config, got:\n%s", content)
	}
	if !strings.Contains(content, "My Project") {
		t.Errorf("expected project in config, got:\n%s", content)
	}
}

func TestRunInit_DefaultValues(t *testing.T) {
	// given: empty lines for lang and strictness (use defaults)
	dir := t.TempDir()
	input := strings.NewReader("Team\nProject\n\n\n")
	var output bytes.Buffer

	// when
	err := runInit(dir, input, &output)

	// then
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}
	data, _ := os.ReadFile(sightjack.ConfigPath(dir))
	content := string(data)
	if !strings.Contains(content, `lang: "ja"`) {
		t.Errorf("expected default lang 'ja' in config, got:\n%s", content)
	}
	if !strings.Contains(content, "default: fog") {
		t.Errorf("expected default strictness 'fog' in config, got:\n%s", content)
	}
}

func TestRunInit_ExistingConfigError(t *testing.T) {
	// given: directory with existing .siren/config.yaml
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".siren"), 0755)
	os.WriteFile(sightjack.ConfigPath(dir), []byte("existing"), 0644)
	input := strings.NewReader("Team\nProject\n\n\n")
	var output bytes.Buffer

	// when
	err := runInit(dir, input, &output)

	// then: should return error
	if err == nil {
		t.Fatal("expected error when config already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %v", err)
	}
}

func TestRunInit_InvalidLang_RepromptsUntilValid(t *testing.T) {
	// given: first answer is invalid "jp", second is valid "en"
	dir := t.TempDir()
	input := strings.NewReader("Team\nProject\njp\nen\n\n")
	var output bytes.Buffer

	// when
	err := runInit(dir, input, &output)

	// then: should succeed with the valid value
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}
	data, _ := os.ReadFile(sightjack.ConfigPath(dir))
	content := string(data)
	if !strings.Contains(content, `lang: "en"`) {
		t.Errorf("expected lang 'en' in config, got:\n%s", content)
	}
	// output should contain an error message about invalid lang
	if !strings.Contains(output.String(), "invalid") {
		t.Errorf("expected 'invalid' error in output, got:\n%s", output.String())
	}
}

func TestRunInit_InvalidStrictness_RepromptsUntilValid(t *testing.T) {
	// given: first strictness answer is invalid "strict", second is valid "alert"
	dir := t.TempDir()
	input := strings.NewReader("Team\nProject\n\nstrict\nalert\n")
	var output bytes.Buffer

	// when
	err := runInit(dir, input, &output)

	// then: should succeed with the valid value
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}
	data, _ := os.ReadFile(sightjack.ConfigPath(dir))
	content := string(data)
	if !strings.Contains(content, "default: alert") {
		t.Errorf("expected strictness 'alert' in config, got:\n%s", content)
	}
	if !strings.Contains(output.String(), "invalid") {
		t.Errorf("expected 'invalid' error in output, got:\n%s", output.String())
	}
}

func TestRunInit_EOFDuringTeam_ReturnsError(t *testing.T) {
	// given: stdin closes before team is entered
	dir := t.TempDir()
	input := strings.NewReader("")
	var output bytes.Buffer

	// when
	err := runInit(dir, input, &output)

	// then
	if err == nil {
		t.Fatal("expected error for EOF during team input")
	}
	if !strings.Contains(err.Error(), "unexpected end of input") {
		t.Errorf("expected 'unexpected end of input' error, got: %v", err)
	}
}

func TestRunInit_StrictnessCaseInsensitive(t *testing.T) {
	// given: mixed-case strictness values should be accepted
	for _, input := range []struct {
		value string
		want  string
	}{
		{"Alert", "alert"},
		{"LOCKDOWN", "lockdown"},
		{"Fog", "fog"},
	} {
		t.Run(input.value, func(t *testing.T) {
			dir := t.TempDir()
			r := strings.NewReader("Team\nProject\n\n" + input.value + "\n")
			var output bytes.Buffer

			// when
			err := runInit(dir, r, &output)

			// then
			if err != nil {
				t.Fatalf("runInit failed: %v", err)
			}
			data, _ := os.ReadFile(sightjack.ConfigPath(dir))
			content := string(data)
			if !strings.Contains(content, "default: "+input.want) {
				t.Errorf("expected strictness %q in config, got:\n%s", input.want, content)
			}
		})
	}
}

func TestConfigExplicitlySet_NotSet(t *testing.T) {
	// given: flags parsed without -c
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("config", ".siren/config.yaml", "")
	fs.String("c", ".siren/config.yaml", "")
	fs.Parse([]string{})

	// when
	result := configExplicitlySet(fs)

	// then
	if result {
		t.Error("expected false when -c not set")
	}
}

func TestConfigExplicitlySet_WithConfigFlag(t *testing.T) {
	// given: flags parsed with --config
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("config", ".siren/config.yaml", "")
	fs.String("c", ".siren/config.yaml", "")
	fs.Parse([]string{"--config", "custom.yaml"})

	// when
	result := configExplicitlySet(fs)

	// then
	if !result {
		t.Error("expected true when --config explicitly set")
	}
}

func TestConfigExplicitlySet_WithShortFlag(t *testing.T) {
	// given: flags parsed with -c
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("config", ".siren/config.yaml", "")
	fs.String("c", ".siren/config.yaml", "")
	fs.Parse([]string{"-c", "sightjack.yaml"})

	// when
	result := configExplicitlySet(fs)

	// then: even though the value matches the default, it was explicitly set
	if !result {
		t.Error("expected true when -c explicitly set to default value")
	}
}

func TestResolveConfigPath_RelativeWithBaseDir(t *testing.T) {
	// given: relative config path with a different baseDir
	configPath := ".siren/config.yaml"
	baseDir := "/repo"
	explicitlySet := true

	// when
	got := resolveConfigPath(configPath, baseDir, explicitlySet)

	// then: should resolve relative to baseDir, not cwd
	want := "/repo/.siren/config.yaml"
	if got != want {
		t.Errorf("resolveConfigPath: expected %q, got %q", want, got)
	}
}

func TestResolveConfigPath_AbsoluteUnchanged(t *testing.T) {
	// given: absolute config path
	configPath := "/custom/config.yaml"
	baseDir := "/repo"
	explicitlySet := true

	// when
	got := resolveConfigPath(configPath, baseDir, explicitlySet)

	// then: should remain unchanged
	if got != configPath {
		t.Errorf("resolveConfigPath: expected %q, got %q", configPath, got)
	}
}

func TestResolveConfigPath_NotExplicit_UsesDefault(t *testing.T) {
	// given: config not explicitly set
	configPath := ".siren/config.yaml"
	baseDir := "/repo"
	explicitlySet := false

	// when
	got := resolveConfigPath(configPath, baseDir, explicitlySet)

	// then: should use ConfigPath(baseDir)
	want := sightjack.ConfigPath(baseDir)
	if got != want {
		t.Errorf("resolveConfigPath: expected %q, got %q", want, got)
	}
}

func TestRunInit_OutputContainsPrompts(t *testing.T) {
	// given
	dir := t.TempDir()
	input := strings.NewReader("Team\nProject\n\n\n")
	var output bytes.Buffer

	// when
	runInit(dir, input, &output)

	// then: output should contain the interactive prompts
	out := output.String()
	if !strings.Contains(out, "Linear team name") {
		t.Errorf("expected 'Linear team name' prompt in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Created .siren/config.yaml") {
		t.Errorf("expected success message in output, got:\n%s", out)
	}
}

func TestRunInit_CreatesGitIgnore(t *testing.T) {
	// given
	dir := t.TempDir()
	input := strings.NewReader("Team\nProject\n\n\n")
	var output bytes.Buffer

	// when
	err := runInit(dir, input, &output)

	// then: .siren/.gitignore should exist
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(dir, ".siren", ".gitignore"))
	if readErr != nil {
		t.Fatalf(".gitignore not created: %v", readErr)
	}
	content := string(data)
	if !strings.Contains(content, "state.json") {
		t.Errorf("expected state.json in .gitignore, got:\n%s", content)
	}
}
