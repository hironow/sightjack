package cmd

import (
	"testing"
)

func TestUpdateCmd_Exists(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when: look for update subcommand
	var found bool
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "update" {
			found = true
			break
		}
	}

	// then
	if !found {
		t.Error("expected 'update' subcommand to be registered")
	}
}

func TestUpdateCmd_CheckFlag(t *testing.T) {
	// given
	rootCmd := NewRootCommand()

	// when: find update command and check --check flag
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "update" {
			flag := sub.Flags().Lookup("check")
			if flag == nil {
				t.Error("expected --check flag on update command")
			}
			return
		}
	}

	t.Fatal("update command not found")
}
