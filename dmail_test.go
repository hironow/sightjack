package sightjack

import (
	"strings"
	"testing"
)

func TestDMailKind_Valid(t *testing.T) {
	kinds := []DMailKind{DMailSpecification, DMailReport, DMailFeedback}
	for _, k := range kinds {
		if k == "" {
			t.Errorf("kind constant should not be empty")
		}
	}
}

func TestValidateDMail_Valid(t *testing.T) {
	mail := &DMail{
		Name:        "spec-my-42",
		Kind:        DMailSpecification,
		Description: "Issue MY-42 ready for implementation",
	}
	if err := ValidateDMail(mail); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateDMail_MissingName(t *testing.T) {
	mail := &DMail{Kind: DMailSpecification, Description: "desc"}
	if err := ValidateDMail(mail); err == nil {
		t.Error("expected error for missing name")
	}
}

func TestValidateDMail_MissingKind(t *testing.T) {
	mail := &DMail{Name: "test", Description: "desc"}
	if err := ValidateDMail(mail); err == nil {
		t.Error("expected error for missing kind")
	}
}

func TestValidateDMail_InvalidKind(t *testing.T) {
	mail := &DMail{Name: "test", Kind: "invalid", Description: "desc"}
	if err := ValidateDMail(mail); err == nil {
		t.Error("expected error for invalid kind")
	}
}

func TestValidateDMail_MissingDescription(t *testing.T) {
	mail := &DMail{Name: "test", Kind: DMailFeedback}
	if err := ValidateDMail(mail); err == nil {
		t.Error("expected error for missing description")
	}
}

func TestValidateDMail_Nil(t *testing.T) {
	if err := ValidateDMail(nil); err == nil {
		t.Error("expected error for nil mail")
	}
}

func TestDMail_Filename(t *testing.T) {
	mail := &DMail{Name: "spec-my-42"}
	if got := mail.Filename(); got != "spec-my-42.md" {
		t.Errorf("got %s, want spec-my-42.md", got)
	}
}

func TestMarshalDMail_Basic(t *testing.T) {
	mail := &DMail{
		Name:        "spec-my-42",
		Kind:        DMailSpecification,
		Description: "Issue MY-42 ready",
		Issues:      []string{"MY-42"},
		Body:        "# Rate Limiting\n\n## DoD\n- Token bucket\n",
	}
	data, err := MarshalDMail(mail)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		t.Error("expected --- prefix")
	}
	if !strings.Contains(content, "name: spec-my-42") {
		t.Error("expected name in frontmatter")
	}
	if !strings.Contains(content, "kind: specification") {
		t.Error("expected kind in frontmatter")
	}
	if !strings.Contains(content, "# Rate Limiting") {
		t.Error("expected body content")
	}
}

func TestParseDMail_RoundTrip(t *testing.T) {
	original := &DMail{
		Name:        "report-my-99",
		Kind:        DMailReport,
		Description: "PR merged for MY-99",
		Issues:      []string{"MY-99"},
		Severity:    "medium",
		Metadata:    map[string]string{"created_at": "2026-02-20T12:00:00Z"},
		Body:        "# Implementation Report\n\nPR #42 merged.\n",
	}
	data, err := MarshalDMail(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	parsed, err := ParseDMail(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Name != original.Name {
		t.Errorf("name: got %s, want %s", parsed.Name, original.Name)
	}
	if parsed.Kind != original.Kind {
		t.Errorf("kind: got %s, want %s", parsed.Kind, original.Kind)
	}
	if parsed.Severity != "medium" {
		t.Errorf("severity: got %s, want medium", parsed.Severity)
	}
	if parsed.Metadata["created_at"] != "2026-02-20T12:00:00Z" {
		t.Error("expected metadata created_at")
	}
	if parsed.Body != original.Body {
		t.Errorf("body: got %q, want %q", parsed.Body, original.Body)
	}
}

func TestParseDMail_InvalidYAML(t *testing.T) {
	data := []byte("---\ninvalid: [\n---\nbody\n")
	_, err := ParseDMail(data)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestParseDMail_NoFrontmatter(t *testing.T) {
	data := []byte("just markdown body\n")
	_, err := ParseDMail(data)
	if err == nil {
		t.Error("expected error for missing frontmatter")
	}
}
