package domain_test

import (
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestRunScanCommand_Validate_Valid(t *testing.T) {
	// given
	cmd := domain.RunScanCommand{
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
	cmd := domain.RunScanCommand{
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
	cmd := domain.RunScanCommand{
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

func TestRunSessionCommand_Validate_Valid(t *testing.T) {
	// given
	cmd := domain.RunSessionCommand{
		RepoPath: "/tmp/repo",
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestRunSessionCommand_Validate_MissingRepoPath(t *testing.T) {
	// given
	cmd := domain.RunSessionCommand{}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) == 0 {
		t.Fatal("expected validation error for missing RepoPath")
	}
}

func TestResumeSessionCommand_Validate_Valid(t *testing.T) {
	// given
	cmd := domain.ResumeSessionCommand{
		RepoPath:  "/tmp/repo",
		SessionID: "session-123",
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestResumeSessionCommand_Validate_MissingSessionID(t *testing.T) {
	// given
	cmd := domain.ResumeSessionCommand{
		RepoPath: "/tmp/repo",
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) == 0 {
		t.Fatal("expected validation error for missing SessionID")
	}
}

func TestApplyWaveCommand_Validate_Valid(t *testing.T) {
	// given
	cmd := domain.ApplyWaveCommand{
		RepoPath:    "/tmp/repo",
		SessionID:   "session-123",
		ClusterName: "C1",
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestApplyWaveCommand_Validate_MissingFields(t *testing.T) {
	// given
	cmd := domain.ApplyWaveCommand{}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) < 2 {
		t.Fatalf("expected at least 2 validation errors, got %d: %v", len(errs), errs)
	}
}

func TestDiscussWaveCommand_Validate_Valid(t *testing.T) {
	// given
	cmd := domain.DiscussWaveCommand{
		RepoPath:    "/tmp/repo",
		SessionID:   "session-123",
		ClusterName: "C1",
		Topic:       "design question",
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestDiscussWaveCommand_Validate_MissingTopic(t *testing.T) {
	// given
	cmd := domain.DiscussWaveCommand{
		RepoPath:    "/tmp/repo",
		SessionID:   "session-123",
		ClusterName: "C1",
	}

	// when
	errs := cmd.Validate()

	// then
	if len(errs) == 0 {
		t.Fatal("expected validation error for missing Topic")
	}
}
