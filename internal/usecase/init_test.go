package usecase_test

import (
	"fmt"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase"
)

type stubInitRunner struct {
	called     bool
	baseDir    string
	team       string
	project    string
	lang       string
	strictness string
	warnings   []string
	err        error
}

func (s *stubInitRunner) InitProject(baseDir, team, project, lang, strictness string) ([]string, error) {
	s.called = true
	s.baseDir = baseDir
	s.team = team
	s.project = project
	s.lang = lang
	s.strictness = strictness
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
	if runner.lang != "en" {
		t.Errorf("expected lang en, got %q", runner.lang)
	}
	if runner.strictness != "alert" {
		t.Errorf("expected strictness alert, got %q", runner.strictness)
	}
}

func TestRunInit_WithDefaults(t *testing.T) {
	runner := &stubInitRunner{}
	rp, _ := domain.NewRepoPath("/tmp/repo")
	// Defaults applied at cmd layer: lang="ja", strictness="fog"
	cmd := domain.NewInitCommand(rp, "", "", "ja", "fog")

	_, err := usecase.RunInit(cmd, runner)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if runner.lang != "ja" {
		t.Errorf("expected default lang ja, got %q", runner.lang)
	}
	if runner.strictness != "fog" {
		t.Errorf("expected default strictness fog, got %q", runner.strictness)
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

// Validation tests (empty BaseDir) are now in domain/primitives_test.go.
// WithDefaults tests removed — defaults are applied at cmd layer before construction.
