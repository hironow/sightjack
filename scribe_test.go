package sightjack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNextADRNumber_EmptyDir(t *testing.T) {
	// given
	dir := t.TempDir()

	// when
	num, err := NextADRNumber(dir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if num != 1 {
		t.Errorf("expected 1 for empty dir, got %d", num)
	}
}

func TestNextADRNumber_WithGaps(t *testing.T) {
	// given: dir with 0001-foo.md and 0003-bar.md (gap at 0002)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "0001-foo.md"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "0003-bar.md"), []byte(""), 0644)

	// when
	num, err := NextADRNumber(dir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if num != 4 {
		t.Errorf("expected 4 (max+1, not count+1), got %d", num)
	}
}

func TestNextADRNumber_DirNotExist(t *testing.T) {
	// given
	dir := filepath.Join(t.TempDir(), "nonexistent")

	// when
	num, err := NextADRNumber(dir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if num != 1 {
		t.Errorf("expected 1 for non-existent dir, got %d", num)
	}
}

func TestNextADRNumber_IgnoresNonMatchingFiles(t *testing.T) {
	// given: dir with matching and non-matching files
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "0002-valid.md"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "invalid-name.md"), []byte(""), 0644)

	// when
	num, err := NextADRNumber(dir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if num != 3 {
		t.Errorf("expected 3 (only 0002 matches), got %d", num)
	}
}

func TestScribeFileName(t *testing.T) {
	// given
	wave := Wave{ID: "auth-w1", ClusterName: "Auth"}

	// when
	name := scribeFileName(wave)

	// then
	if name != "scribe_auth_auth-w1.json" {
		t.Errorf("expected scribe_auth_auth-w1.json, got %s", name)
	}
}

func TestScribeFileName_SpecialChars(t *testing.T) {
	// given
	wave := Wave{ID: "w-1", ClusterName: "UI/Frontend"}

	// when
	name := scribeFileName(wave)

	// then
	if name != "scribe_ui_frontend_w-1.json" {
		t.Errorf("expected scribe_ui_frontend_w-1.json, got %s", name)
	}
}

func TestClearScribeOutput_RemovesExisting(t *testing.T) {
	// given
	scanDir := t.TempDir()
	wave := Wave{ID: "auth-w1", ClusterName: "Auth"}
	outputFile := filepath.Join(scanDir, scribeFileName(wave))
	os.WriteFile(outputFile, []byte(`{"adr_id":"0001"}`), 0644)

	// when
	clearScribeOutput(scanDir, wave)

	// then
	if _, err := os.Stat(outputFile); !os.IsNotExist(err) {
		t.Error("expected stale output file to be removed")
	}
}

func TestClearScribeOutput_NoOpIfMissing(t *testing.T) {
	// given: no file exists
	scanDir := t.TempDir()
	wave := Wave{ID: "auth-w1", ClusterName: "Auth"}

	// when: should not panic or error
	clearScribeOutput(scanDir, wave)
}

func TestRunScribeADRDryRun(t *testing.T) {
	// given
	scanDir := t.TempDir()
	adrDir := filepath.Join(t.TempDir(), "adr")
	cfg := &Config{
		Lang:   "en",
		Claude: ClaudeConfig{Command: "claude", TimeoutSec: 60},
	}
	wave := Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Dependency Ordering",
		Actions:     []WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
	}
	architectResp := &ArchitectResponse{
		Analysis:  "Splitting recommended.",
		Reasoning: "Scale favors smaller batches.",
	}

	// when
	err := RunScribeADRDryRun(cfg, scanDir, wave, architectResp, adrDir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	promptFile := filepath.Join(scanDir, "scribe_auth_auth-w1_prompt.md")
	if _, err := os.Stat(promptFile); os.IsNotExist(err) {
		t.Error("expected scribe prompt file to be generated")
	}
}

func TestParseScribeResult_Valid(t *testing.T) {
	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "scribe_auth_auth-w1.json")
	data := `{"adr_id":"0003","title":"adopt-event-sourcing","content":"# 0003. Adopt Event Sourcing","reasoning":"Discussion revealed need"}`
	os.WriteFile(path, []byte(data), 0644)

	// when
	result, err := ParseScribeResult(path)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ADRID != "0003" {
		t.Errorf("expected 0003, got %s", result.ADRID)
	}
	if result.Title != "adopt-event-sourcing" {
		t.Errorf("expected adopt-event-sourcing, got %s", result.Title)
	}
	if result.Content != "# 0003. Adopt Event Sourcing" {
		t.Errorf("unexpected content: %s", result.Content)
	}
}

func TestParseScribeResult_MalformedJSON(t *testing.T) {
	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "scribe.json")
	os.WriteFile(path, []byte(`{"adr_id": "truncated`), 0644)

	// when
	_, err := ParseScribeResult(path)

	// then
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse scribe result") {
		t.Errorf("expected 'parse scribe result' in error, got: %v", err)
	}
}

func TestParseScribeResult_FileNotFound(t *testing.T) {
	// when
	_, err := ParseScribeResult("/nonexistent/path.json")

	// then
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestSanitizeADRTitle_Normal(t *testing.T) {
	// given
	title := "adopt-event-sourcing"

	// when
	result := sanitizeADRTitle(title)

	// then
	if result != "adopt-event-sourcing" {
		t.Errorf("expected adopt-event-sourcing, got %s", result)
	}
}

func TestSanitizeADRTitle_PathTraversal(t *testing.T) {
	// given: malicious title with path traversal
	title := "../../../etc/passwd"

	// when
	result := sanitizeADRTitle(title)

	// then: should not contain path separators or ..
	if strings.Contains(result, "/") || strings.Contains(result, "..") {
		t.Errorf("expected path separators removed, got %s", result)
	}
}

func TestSanitizeADRTitle_SpecialChars(t *testing.T) {
	// given: title with spaces and special characters
	title := "Use FastAPI for API Layer!"

	// when
	result := sanitizeADRTitle(title)

	// then: should only contain safe chars
	for _, r := range result {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			t.Errorf("unexpected character %q in sanitized title %s", r, result)
		}
	}
}

func TestSanitizeADRTitle_Empty(t *testing.T) {
	// given
	title := ""

	// when
	result := sanitizeADRTitle(title)

	// then: should return fallback
	if result != "untitled" {
		t.Errorf("expected 'untitled' for empty title, got %s", result)
	}
}

func TestCountADRFiles_EmptyDir(t *testing.T) {
	// given
	dir := t.TempDir()

	// when
	count := CountADRFiles(dir)

	// then
	if count != 0 {
		t.Errorf("expected 0 for empty dir, got %d", count)
	}
}

func TestCountADRFiles_WithMatchingAndNonMatching(t *testing.T) {
	// given
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "0001-foo.md"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "0003-bar.md"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte(""), 0644)

	// when
	count := CountADRFiles(dir)

	// then: only 2 files match NNNN-*.md pattern
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestCountADRFiles_NonexistentDir(t *testing.T) {
	// given
	dir := filepath.Join(t.TempDir(), "nonexistent")

	// when
	count := CountADRFiles(dir)

	// then
	if count != 0 {
		t.Errorf("expected 0 for non-existent dir, got %d", count)
	}
}
