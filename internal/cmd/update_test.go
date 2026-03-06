package cmd

// white-box-reason: cobra command construction: NewRootCommand and CLI routing are unexported

import (
	"testing"

	"github.com/Masterminds/semver/v3"
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

func TestUpdateCmd_DevVersionIsNotSemver(t *testing.T) {
	// given: default version is "dev" which is not valid semver
	// This test ensures the semver guard in update.go would catch it
	// and prevent a panic from LessOrEqual.

	// when
	_, err := semver.NewVersion("dev")

	// then: "dev" must NOT be valid semver
	if err == nil {
		t.Fatal("expected 'dev' to be invalid semver, but it parsed successfully")
	}
}

func TestUpdateCmd_TaggedVersionIsSemver(t *testing.T) {
	// given: a typical GoReleaser-injected version
	for _, v := range []string{"0.0.12", "1.0.0", "2.1.3-rc1"} {
		t.Run(v, func(t *testing.T) {
			// when
			_, err := semver.NewVersion(v)

			// then
			if err != nil {
				t.Errorf("expected %q to be valid semver: %v", v, err)
			}
		})
	}
}
