package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

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

func TestUpdateCmd_IsUpToDate(t *testing.T) {
	cases := []struct {
		name     string
		current  string
		latest   string
		upToDate bool
	}{
		{name: "same version", current: "1.0.0", latest: "1.0.0", upToDate: true},
		{name: "current newer", current: "2.0.0", latest: "1.0.0", upToDate: true},
		{name: "current older", current: "1.0.0", latest: "2.0.0", upToDate: false},
		{name: "dev version", current: "dev", latest: "1.0.0", upToDate: false},
		{name: "v-prefixed", current: "v1.0.0", latest: "1.0.0", upToDate: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			got := isUpToDate(tc.current, tc.latest)

			// then
			if got != tc.upToDate {
				t.Errorf("isUpToDate(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.upToDate)
			}
		})
	}
}
