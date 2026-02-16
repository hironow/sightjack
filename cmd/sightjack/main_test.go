package main

import (
	"slices"
	"testing"
)

func TestExtractSubcommand_BoolFlagWithValue(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantCmd   string
		wantFlags []string
		wantErr   bool
	}{
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
		{
			name:    "unknown command after subcommand rejected",
			args:    []string{"session", "garbage"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, flags, err := extractSubcommand(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got cmd=%q flags=%v", cmd, flags)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd != tt.wantCmd {
				t.Errorf("cmd: expected %q, got %q", tt.wantCmd, cmd)
			}
			if !slices.Equal(flags, tt.wantFlags) {
				t.Errorf("flags: expected %v, got %v", tt.wantFlags, flags)
			}
		})
	}
}
