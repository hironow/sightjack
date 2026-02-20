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
	err := RunScribeADRDryRun(cfg, scanDir, wave, architectResp, adrDir, "fog")

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

func TestReadExistingADRs_Empty(t *testing.T) {
	// given
	dir := t.TempDir()

	// when
	adrs, err := ReadExistingADRs(dir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(adrs) != 0 {
		t.Errorf("expected 0 ADRs, got %d", len(adrs))
	}
}

func TestReadExistingADRs_ReturnsContent(t *testing.T) {
	// given
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "0001-auth-decision.md"), []byte("# 0001. Auth Decision\nAccepted"), 0644)
	os.WriteFile(filepath.Join(dir, "0002-api-design.md"), []byte("# 0002. API Design\nAccepted"), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("ignore this"), 0644) // non-ADR file

	// when
	adrs, err := ReadExistingADRs(dir)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(adrs) != 2 {
		t.Fatalf("expected 2 ADRs, got %d", len(adrs))
	}
	if adrs[0].Filename != "0001-auth-decision.md" {
		t.Errorf("expected 0001-auth-decision.md, got %s", adrs[0].Filename)
	}
	if !strings.Contains(adrs[0].Content, "Auth Decision") {
		t.Error("expected ADR content to contain 'Auth Decision'")
	}
}

func TestReadExistingADRs_DirNotExist(t *testing.T) {
	// when
	adrs, err := ReadExistingADRs("/nonexistent/dir")

	// then
	if err != nil {
		t.Fatalf("unexpected error for missing dir: %v", err)
	}
	if len(adrs) != 0 {
		t.Errorf("expected 0 ADRs, got %d", len(adrs))
	}
}

func TestReadExistingADRs_UnreadableFile_ReturnsError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root user")
	}

	// given: ADR file exists but is unreadable
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "0001-readable.md"), []byte("ok"), 0644)
	unreadable := filepath.Join(dir, "0002-unreadable.md")
	os.WriteFile(unreadable, []byte("secret"), 0000)

	// when
	_, err := ReadExistingADRs(dir)

	// then: should return error, not silently skip
	if err == nil {
		t.Fatal("expected error for unreadable ADR file")
	}
	if !strings.Contains(err.Error(), "0002-unreadable.md") {
		t.Errorf("expected filename in error, got: %v", err)
	}
}

func TestRunScribeADRDryRun_IncludesExistingADRs(t *testing.T) {
	// given
	dir := t.TempDir()
	scanDir := filepath.Join(dir, "scans")
	os.MkdirAll(scanDir, 0755)
	adrDir := filepath.Join(dir, "docs", "adr")
	os.MkdirAll(adrDir, 0755)
	os.WriteFile(filepath.Join(adrDir, "0001-auth.md"), []byte("# Auth ADR"), 0644)

	cfg := &Config{
		Lang:   "en",
		Claude: ClaudeConfig{Command: "echo", TimeoutSec: 10},
	}
	wave := Wave{ID: "w1", ClusterName: "Auth", Title: "Test"}
	resp := &ArchitectResponse{Analysis: "test", Reasoning: "test"}

	// when
	err := RunScribeADRDryRun(cfg, scanDir, wave, resp, adrDir, "fog")

	// then
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	promptFiles, _ := filepath.Glob(filepath.Join(scanDir, "scribe_*_prompt.md"))
	if len(promptFiles) == 0 {
		t.Fatal("expected scribe prompt file to be created")
	}
	content, _ := os.ReadFile(promptFiles[0])
	if !strings.Contains(string(content), "0001-auth.md") {
		t.Error("expected existing ADR filename in dry-run prompt")
	}
}

func TestNormalizeScribeResult_MatchingID(t *testing.T) {
	// given: Claude returned matching adr_id
	result := &ScribeResponse{ADRID: "0003", Title: "test"}

	// when
	normalizeScribeResult(result, "0003")

	// then: no change
	if result.ADRID != "0003" {
		t.Errorf("expected 0003, got %s", result.ADRID)
	}
}

func TestNormalizeScribeResult_MismatchID(t *testing.T) {
	// given: Claude returned wrong adr_id
	result := &ScribeResponse{ADRID: "9999", Title: "test"}

	// when
	normalizeScribeResult(result, "0003")

	// then: overwritten with authoritative ID
	if result.ADRID != "0003" {
		t.Errorf("expected 0003, got %s", result.ADRID)
	}
}

func TestNormalizeScribeResult_EmptyID(t *testing.T) {
	// given: Claude returned empty adr_id
	result := &ScribeResponse{ADRID: "", Title: "test"}

	// when
	normalizeScribeResult(result, "0003")

	// then: filled with authoritative ID
	if result.ADRID != "0003" {
		t.Errorf("expected 0003, got %s", result.ADRID)
	}
}

func TestRenderADRFromDiscuss_Basic(t *testing.T) {
	// given
	dr := DiscussResult{
		WaveID:    "auth-w1",
		Analysis:  "JWT has trade-offs",
		Reasoning: "Session-based auth is simpler",
		Decision:  "Use session-based auth",
		ADRWorthy: true,
		ADRTitle:  "Session over JWT",
	}

	// when
	md := RenderADRFromDiscuss(dr, 42)

	// then
	if !strings.Contains(md, "# 0042.") {
		t.Errorf("expected ADR number in title, got:\n%s", md)
	}
	if !strings.Contains(md, "Session over JWT") {
		t.Errorf("expected ADR title, got:\n%s", md)
	}
	if !strings.Contains(md, "JWT has trade-offs") {
		t.Errorf("expected analysis in context, got:\n%s", md)
	}
	if !strings.Contains(md, "Use session-based auth") {
		t.Errorf("expected decision, got:\n%s", md)
	}
	if !strings.Contains(md, "Session-based auth is simpler") {
		t.Errorf("expected reasoning, got:\n%s", md)
	}
}

func TestRenderADRFromDiscuss_UsesWaveIDWhenNoTitle(t *testing.T) {
	// given
	dr := DiscussResult{
		WaveID:   "auth-w1",
		Analysis: "ok",
		Decision: "proceed",
	}

	// when
	md := RenderADRFromDiscuss(dr, 1)

	// then
	if !strings.Contains(md, "auth-w1") {
		t.Errorf("expected wave ID as fallback title, got:\n%s", md)
	}
}

func TestRenderADRFromDiscuss_WithModifications(t *testing.T) {
	// given
	dr := DiscussResult{
		WaveID:   "w1",
		Analysis: "changed approach",
		Decision: "use Redis",
		Modifications: []WaveModification{
			{ActionIndex: 0, Change: "Added Redis dependency"},
		},
	}

	// when
	md := RenderADRFromDiscuss(dr, 5)

	// then
	if !strings.Contains(md, "Redis dependency") {
		t.Errorf("expected modification in output, got:\n%s", md)
	}
}
