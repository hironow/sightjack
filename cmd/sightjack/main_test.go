package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestUsageOutput_ContainsSubcommands(t *testing.T) {
	// given: a FlagSet with our custom usage function
	var buf bytes.Buffer
	fs := flag.NewFlagSet("sightjack", flag.ContinueOnError)
	fs.SetOutput(&buf)
	fs.String("config", "sightjack.yaml", "Config file path")
	setUsage(fs)

	// when: usage is triggered
	fs.Usage()

	// then: output contains all subcommands
	output := buf.String()
	for _, cmd := range []string{"scan", "session", "show", "init"} {
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
	cfgPath := filepath.Join(dir, "sightjack.yaml")
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
	data, _ := os.ReadFile(filepath.Join(dir, "sightjack.yaml"))
	content := string(data)
	if !strings.Contains(content, `lang: "ja"`) {
		t.Errorf("expected default lang 'ja' in config, got:\n%s", content)
	}
	if !strings.Contains(content, "default: fog") {
		t.Errorf("expected default strictness 'fog' in config, got:\n%s", content)
	}
}

func TestRunInit_ExistingConfigError(t *testing.T) {
	// given: directory with existing sightjack.yaml
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "sightjack.yaml"), []byte("existing"), 0644)
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
	if !strings.Contains(out, "Created sightjack.yaml") {
		t.Errorf("expected success message in output, got:\n%s", out)
	}
}
