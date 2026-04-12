package usecase_test

import (
	"fmt"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase"
	"github.com/hironow/sightjack/internal/usecase/port"
)

type stubInitRunner struct {
	called   bool
	baseDir  string
	config   port.InitConfig
	warnings []string
	err      error
}

func (s *stubInitRunner) InitProject(baseDir string, opts ...port.InitOption) ([]string, error) {
	s.called = true
	s.baseDir = baseDir
	s.config = port.ApplyInitOptions(opts...)
	return s.warnings, s.err
}

func TestRunInit_ValidCommand(t *testing.T) {
	runner := &stubInitRunner{}
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewInitCommand(rp, "Eng", "Hades", "en", "alert")

	warnings, err := usecase.RunInit(cmd, runner)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	if !runner.called {
		t.Fatal("expected InitProject to be called")
	}
	if runner.config.Lang != "en" {
		t.Errorf("expected lang en, got %q", runner.config.Lang)
	}
	if runner.config.Strictness != "alert" {
		t.Errorf("expected strictness alert, got %q", runner.config.Strictness)
	}
}

func TestRunInit_WithDefaults(t *testing.T) {
	runner := &stubInitRunner{}
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewInitCommand(rp, "", "", "ja", "fog")

	_, err := usecase.RunInit(cmd, runner)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if runner.config.Lang != "ja" {
		t.Errorf("expected default lang ja, got %q", runner.config.Lang)
	}
	if runner.config.Strictness != "fog" {
		t.Errorf("expected default strictness fog, got %q", runner.config.Strictness)
	}
}

func TestRunInit_PropagatesWarnings(t *testing.T) {
	runner := &stubInitRunner{warnings: []string{"skills install failed"}}
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewInitCommand(rp, "", "", "ja", "fog")

	warnings, err := usecase.RunInit(cmd, runner)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(warnings) != 1 || warnings[0] != "skills install failed" {
		t.Errorf("expected warnings propagated, got %v", warnings)
	}
}

func TestRunInit_RunnerError(t *testing.T) {
	runner := &stubInitRunner{err: fmt.Errorf("config exists")}
	rp, _ := domain.NewRepoPath("/tmp/repo")
	cmd := domain.NewInitCommand(rp, "", "", "ja", "fog")

	_, err := usecase.RunInit(cmd, runner)

	if err == nil {
		t.Fatal("expected error from runner")
	}
}
