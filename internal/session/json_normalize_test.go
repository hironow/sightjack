package session_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/session"
)

func TestNormalizeJSONFile_ConvertsUnicodeEscapes(t *testing.T) {
	// given: JSON file with \uXXXX escapes for Japanese text
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	content := `{"name":"DoD\u5168\u4ef6\u306b\u554f\u984c","value":42}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	err := session.NormalizeJSONFile(path)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	normalized := string(data)
	if strings.Contains(normalized, `\u`) {
		t.Errorf("expected no \\uXXXX escapes, got: %s", normalized)
	}
	if !strings.Contains(normalized, "DoD全件に問題") {
		t.Errorf("expected raw UTF-8 Japanese text, got: %s", normalized)
	}
}

func TestNormalizeJSONFile_PreservesValidJSON(t *testing.T) {
	// given: valid JSON that is already clean UTF-8
	dir := t.TempDir()
	path := filepath.Join(dir, "clean.json")
	content := `{"clusters":[{"name":"認証","issue_ids":["T-1"]}],"total_issues":1}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	err := session.NormalizeJSONFile(path)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "認証") {
		t.Errorf("expected preserved Japanese text, got: %s", string(data))
	}
}

func TestNormalizeJSONFile_RejectsInvalidJSON(t *testing.T) {
	// given: invalid JSON
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{not json`), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	err := session.NormalizeJSONFile(path)

	// then
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestNormalizeJSONFile_MissingFile(t *testing.T) {
	// when
	err := session.NormalizeJSONFile("/nonexistent/file.json")

	// then
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestNormalizeJSONFile_TextPrefixBeforeJSON(t *testing.T) {
	// given: Claude sometimes returns natural language before JSON
	dir := t.TempDir()
	path := filepath.Join(dir, "text_prefix.json")
	content := "Certainly, here's the analysis:\n\n" + `{"clusters":[],"total_issues":0}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	err := session.NormalizeJSONFile(path)

	// then
	if err != nil {
		t.Fatalf("expected text-prefixed JSON to normalize, got: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"total_issues"`) {
		t.Errorf("expected normalized JSON, got: %s", string(data))
	}
}

func TestNormalizeJSONFile_MarkdownCodeBlock(t *testing.T) {
	// given: Claude wraps JSON in markdown code block
	dir := t.TempDir()
	path := filepath.Join(dir, "markdown.json")
	content := "```json\n" + `{"clusters":[],"total_issues":0}` + "\n```"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	err := session.NormalizeJSONFile(path)

	// then
	if err != nil {
		t.Fatalf("expected markdown-wrapped JSON to normalize, got: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "```") {
		t.Errorf("expected markdown fences removed, got: %s", string(data))
	}
}

func TestNormalizeJSONFile_TextSuffixAfterJSON(t *testing.T) {
	// given: Claude appends text after JSON
	dir := t.TempDir()
	path := filepath.Join(dir, "suffix.json")
	content := `{"clusters":[],"total_issues":0}` + "\n\nLet me know if you need more details."
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	err := session.NormalizeJSONFile(path)

	// then
	if err != nil {
		t.Fatalf("expected JSON-with-suffix to normalize, got: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Let me know") {
		t.Errorf("expected suffix removed, got: %s", string(data))
	}
}
