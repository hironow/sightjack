package sightjack_test

import (
	"testing"

	sightjack "github.com/hironow/sightjack"
)

func TestRunScanCommand_Validate_Valid(t *testing.T) {
	// given
	cmd := sightjack.RunScanCommand{
		RepoPath:   "/tmp/repo",
		Lang:       "ja",
		Strictness: "fog",
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestRunScanCommand_Validate_MissingRepoPath(t *testing.T) {
	// given
	cmd := sightjack.RunScanCommand{
		Lang:       "ja",
		Strictness: "fog",
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) == 0 {
		t.Fatal("expected validation error for missing RepoPath")
	}
}

func TestRunScanCommand_Validate_InvalidLang(t *testing.T) {
	// given
	cmd := sightjack.RunScanCommand{
		RepoPath:   "/tmp/repo",
		Lang:       "jp",
		Strictness: "fog",
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) == 0 {
		t.Fatal("expected validation error for invalid lang")
	}
}
