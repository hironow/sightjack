package filter_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/hironow/sightjack/internal/harness/filter"
)

func TestNewRegistry_LoadsEmbeddedPrompts(t *testing.T) {
	// when
	reg, err := filter.NewRegistry()

	// then
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	names := reg.Names()
	if len(names) == 0 {
		t.Fatal("expected at least one prompt, got none")
	}
	// Verify review_fix is loaded
	found := false
	for _, n := range names {
		if n == "review_fix" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected review_fix in names, got %v", names)
	}
}

func TestRegistry_Get_Found(t *testing.T) {
	// given
	reg, err := filter.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	// when
	def, err := reg.Get("review_fix")

	// then
	if err != nil {
		t.Fatalf("Get(review_fix): %v", err)
	}
	if def.Name != "review_fix" {
		t.Errorf("expected name review_fix, got %q", def.Name)
	}
	if def.Version != "1.0" {
		t.Errorf("expected version 1.0, got %q", def.Version)
	}
	if len(def.Variables) != 2 {
		t.Errorf("expected 2 variables, got %d", len(def.Variables))
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	// given
	reg, err := filter.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	// when
	_, err = reg.Get("nonexistent")

	// then
	if err == nil {
		t.Fatal("expected error for nonexistent prompt")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got %q", err.Error())
	}
}

func TestRegistry_Expand_ReviewFix(t *testing.T) {
	// given
	reg, err := filter.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	// when
	result, err := reg.Expand("review_fix", map[string]string{
		"branch":   "feature/auth",
		"comments": "Missing error handling in login.go",
	})

	// then
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if !strings.Contains(result, "feature/auth") {
		t.Error("expected branch name in expanded prompt")
	}
	if !strings.Contains(result, "Missing error handling in login.go") {
		t.Error("expected comments in expanded prompt")
	}
	if !strings.Contains(result, "Fix all review comments") {
		t.Error("expected instruction text in expanded prompt")
	}
}

func TestRegistry_Expand_NotFound(t *testing.T) {
	// given
	reg, err := filter.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	// when
	_, err = reg.Expand("nonexistent", nil)

	// then
	if err == nil {
		t.Fatal("expected error for nonexistent prompt")
	}
}

func TestExpandTemplate_BasicReplacement(t *testing.T) {
	tmpl := "Hello {name}, welcome to {place}."
	vars := map[string]string{
		"name":  "Alice",
		"place": "Wonderland",
	}

	result := filter.ExpandTemplate(tmpl, vars)

	if result != "Hello Alice, welcome to Wonderland." {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExpandTemplate_UnknownKeysLeftAsIs(t *testing.T) {
	tmpl := "Hello {name}, {unknown} is here."
	vars := map[string]string{"name": "Bob"}

	result := filter.ExpandTemplate(tmpl, vars)

	if result != "Hello Bob, {unknown} is here." {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExpandTemplate_NilVars(t *testing.T) {
	tmpl := "No vars {here}."

	result := filter.ExpandTemplate(tmpl, nil)

	if result != "No vars {here}." {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExpandTemplate_EmptyVars(t *testing.T) {
	tmpl := "Empty {key}."
	vars := map[string]string{"key": ""}

	result := filter.ExpandTemplate(tmpl, vars)

	if result != "Empty ." {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestRegistry_Names_Sorted(t *testing.T) {
	// given
	reg, err := filter.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	// when
	names := reg.Names()

	// then
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("names not sorted: %v", names)
			break
		}
	}
}

func TestExpandTemplate_MatchesLegacyBuildReviewFixPrompt(t *testing.T) {
	// given: the old BuildReviewFixPrompt behavior
	branch := "feature/fix-login"
	comments := "Line 42: missing nil check\nLine 88: unused variable"

	// Legacy output (from fmt.Sprintf in review.go)
	legacy := "You are on branch " + branch + ". A code review found the following issues:\n\n" +
		comments + "\n\n" +
		"Fix all review comments above. Commit and push your changes.\n" +
		"Keep fixes focused — only address the review comments, do not refactor unrelated code."

	// when: using registry
	reg, err := filter.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	result, err := reg.Expand("review_fix", map[string]string{
		"branch":   branch,
		"comments": comments,
	})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	// then: must match (trimmed to normalize trailing whitespace)
	if strings.TrimSpace(result) != strings.TrimSpace(legacy) {
		t.Errorf("regression: expanded prompt does not match legacy\n--- legacy ---\n%s\n--- expanded ---\n%s", legacy, result)
	}
}

// TestNewRegistry_DuplicateName verifies duplicate prompt names cause an error.
func TestNewRegistry_DuplicateName(t *testing.T) {
	// given: two YAML files with the same prompt name
	fsys := fstest.MapFS{
		"prompts/a.yaml": &fstest.MapFile{Data: []byte("name: dup\nversion: '1'\ntemplate: hello\n")},
		"prompts/b.yaml": &fstest.MapFile{Data: []byte("name: dup\nversion: '2'\ntemplate: world\n")},
	}

	// when
	_, err := filter.NewRegistryFromFS(fsys)

	// then
	if err == nil {
		t.Fatal("expected error for duplicate prompt name")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected 'duplicate' in error, got %q", err.Error())
	}
}

// TestNewRegistry_MissingName verifies that a YAML file without a name is rejected.
func TestNewRegistry_MissingName(t *testing.T) {
	// given
	fsys := fstest.MapFS{
		"prompts/bad.yaml": &fstest.MapFile{Data: []byte("version: '1'\ntemplate: hello\n")},
	}

	// when
	_, err := filter.NewRegistryFromFS(fsys)

	// then
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "missing name") {
		t.Errorf("expected 'missing name' in error, got %q", err.Error())
	}
}
