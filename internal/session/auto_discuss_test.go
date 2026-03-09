package session

// white-box-reason: auto-discuss orchestration: tests unexported helpers and round logic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
)

func TestSpeakerForRound(t *testing.T) {
	tests := []struct {
		round int
		want  string
	}{
		{0, "architect"},
		{1, "devils_advocate"},
		{2, "architect"},
		{3, "devils_advocate"},
		{4, "architect"},
	}
	for _, tt := range tests {
		if got := speakerForRound(tt.round); got != tt.want {
			t.Errorf("speakerForRound(%d) = %q, want %q", tt.round, got, tt.want)
		}
	}
}

func TestAutoDiscussOutputFileName(t *testing.T) {
	// given
	wave := domain.Wave{ClusterName: "auth", ID: "w1"}

	// when
	name := autoDiscussOutputFileName("architect", wave, 0)

	// then
	if name == "" {
		t.Fatal("expected non-empty filename")
	}
	if !strings.Contains(name, "architect") {
		t.Error("expected 'architect' in filename")
	}
	if !strings.Contains(name, "r0") {
		t.Error("expected round number in filename")
	}
	if !strings.Contains(name, "auth") {
		t.Error("expected cluster name in filename")
	}
}

func TestAutoDiscussOutputFileName_DevilsAdvocate(t *testing.T) {
	wave := domain.Wave{ClusterName: "auth", ID: "w1"}
	name := autoDiscussOutputFileName("devils_advocate", wave, 3)
	if !strings.Contains(name, "devils_advocate") {
		t.Error("expected 'devils_advocate' in filename")
	}
	if !strings.Contains(name, "r3") {
		t.Error("expected 'r3' in filename")
	}
}

func TestReadCLAUDEMD_NotFound(t *testing.T) {
	dir := t.TempDir()
	content := ReadCLAUDEMD(dir)
	if content != "" {
		t.Errorf("expected empty content for missing CLAUDE.md, got: %q", content)
	}
}

func TestReadCLAUDEMD_FoundInCurrentDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Project Rules"), 0644); err != nil {
		t.Fatal(err)
	}
	content := ReadCLAUDEMD(dir)
	if content != "# Project Rules" {
		t.Errorf("expected '# Project Rules', got: %q", content)
	}
}

func TestReadCLAUDEMD_FoundInParentDir(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "subdir")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "CLAUDE.md"), []byte("# Parent Rules"), 0644); err != nil {
		t.Fatal(err)
	}
	content := ReadCLAUDEMD(child)
	if content != "# Parent Rules" {
		t.Errorf("expected '# Parent Rules', got: %q", content)
	}
}

func TestReadRoundContent(t *testing.T) {
	dir := t.TempDir()
	outputFile := filepath.Join(dir, "test_output.json")
	if err := os.WriteFile(outputFile, []byte(`{"content":"Hello from architect"}`), 0644); err != nil {
		t.Fatal(err)
	}
	content, err := readRoundContent(outputFile)
	if err != nil {
		t.Fatalf("readRoundContent: %v", err)
	}
	if content != "Hello from architect" {
		t.Errorf("expected 'Hello from architect', got: %q", content)
	}
}

func TestReadRoundContent_MissingFile(t *testing.T) {
	_, err := readRoundContent("/nonexistent/file.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestBuildSummaryFromRounds(t *testing.T) {
	rounds := []domain.AutoDiscussRound{
		{Round: 0, Speaker: "architect", Content: "Design is sound."},
		{Round: 1, Speaker: "devils_advocate", Content: "But what about X?"},
	}
	summary := buildSummaryFromRounds(rounds)
	if !strings.Contains(summary, "architect") {
		t.Error("expected architect in summary")
	}
	if !strings.Contains(summary, "devils_advocate") {
		t.Error("expected devils_advocate in summary")
	}
}

func TestBuildSummaryFromRounds_Empty(t *testing.T) {
	summary := buildSummaryFromRounds(nil)
	if summary != "" {
		t.Errorf("expected empty summary for nil rounds, got: %q", summary)
	}
}

func TestParseFinalRound_WithOpenIssues(t *testing.T) {
	dir := t.TempDir()
	wave := domain.Wave{ClusterName: "auth", ID: "w1"}
	outputFile := filepath.Join(dir, autoDiscussOutputFileName("devils_advocate", wave, 3))
	data := `{"content":"Final assessment","open_issues":["Issue A","Issue B"],"adr_recommended":true,"adr_recommendation_reason":"New auth pattern"}`
	if err := os.WriteFile(outputFile, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	issues, summary := parseFinalRound(dir, wave, 3)
	if len(issues) != 2 {
		t.Errorf("expected 2 open issues, got %d", len(issues))
	}
	if summary != "New auth pattern" {
		t.Errorf("expected 'New auth pattern', got: %q", summary)
	}
}

func TestParseFinalRound_MissingFile(t *testing.T) {
	issues, summary := parseFinalRound(t.TempDir(), domain.Wave{ClusterName: "x", ID: "y"}, 0)
	if len(issues) != 0 {
		t.Errorf("expected 0 issues for missing file, got %d", len(issues))
	}
	if summary != "" {
		t.Errorf("expected empty summary for missing file, got: %q", summary)
	}
}
