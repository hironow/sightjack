# D-Mail Library Layer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add D-Mail protocol library layer to Sightjack — types, parse/compose/receive functions, and directory management for inter-tool communication.

**Architecture:** Single file `dmail.go` with YAML frontmatter + Markdown body format. `gopkg.in/yaml.v3` (already in go.mod) for YAML parsing. Manual `---` delimiter splitting for frontmatter extraction. Follows existing `state.go` patterns for file I/O and directory management.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, `t.TempDir()` for test isolation

---

### Task 1: DMail Type + DMailKind Constants + ValidateDMail

**Files:**
- Create: `dmail.go`
- Create: `dmail_test.go`

**Step 1: Write failing tests**

```go
// dmail_test.go
package sightjack

import "testing"

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
```

**Step 2: Run tests — expect FAIL (types not defined)**

Run: `go test ./... -run TestDMailKind -count=1`

**Step 3: Implement types + ValidateDMail**

```go
// dmail.go
package sightjack

import "fmt"

// DMail represents a d-mail message: YAML frontmatter + Markdown body.
type DMail struct {
	Name        string            `yaml:"name"`
	Kind        DMailKind         `yaml:"kind"`
	Description string            `yaml:"description"`
	Issues      []string          `yaml:"issues,omitempty"`
	Severity    string            `yaml:"severity,omitempty"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
	Body        string            `yaml:"-"`
}

// DMailKind is the message type for d-mails.
type DMailKind string

const (
	DMailSpecification DMailKind = "specification"
	DMailReport        DMailKind = "report"
	DMailFeedback      DMailKind = "feedback"
)

// Filename returns the canonical filename: "<name>.md".
func (d *DMail) Filename() string {
	return d.Name + ".md"
}

// ValidateDMail checks required fields and kind validity.
func ValidateDMail(mail *DMail) error {
	if mail.Name == "" {
		return fmt.Errorf("dmail: name is required")
	}
	if mail.Description == "" {
		return fmt.Errorf("dmail: description is required")
	}
	switch mail.Kind {
	case DMailSpecification, DMailReport, DMailFeedback:
		// valid
	default:
		return fmt.Errorf("dmail: invalid kind %q (valid: specification, report, feedback)", mail.Kind)
	}
	return nil
}
```

**Step 4: Run tests — expect PASS**

Run: `go test ./... -run "TestDMailKind|TestValidateDMail" -count=1`

**Step 5: Commit**

```
feat: add DMail type, DMailKind constants, and ValidateDMail
```

---

### Task 2: MarshalDMail + ParseDMail

**Files:**
- Modify: `dmail.go`
- Modify: `dmail_test.go`

**Step 1: Write failing tests**

```go
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
```

**Step 2: Run tests — expect FAIL**

Run: `go test ./... -run "TestMarshalDMail|TestParseDMail" -count=1`

**Step 3: Implement MarshalDMail + ParseDMail**

```go
import (
	"bytes"
	"fmt"
	"strings"
	"gopkg.in/yaml.v3"
)

const frontmatterDelim = "---"

// MarshalDMail serializes a DMail to YAML frontmatter + Markdown body.
func MarshalDMail(mail *DMail) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(frontmatterDelim + "\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(mail); err != nil {
		return nil, fmt.Errorf("dmail marshal frontmatter: %w", err)
	}
	enc.Close()
	buf.WriteString(frontmatterDelim + "\n")
	if mail.Body != "" {
		buf.WriteString("\n")
		buf.WriteString(mail.Body)
	}
	return buf.Bytes(), nil
}

// ParseDMail parses YAML frontmatter + Markdown body from bytes.
func ParseDMail(data []byte) (*DMail, error) {
	content := string(data)
	if !strings.HasPrefix(content, frontmatterDelim+"\n") {
		return nil, fmt.Errorf("dmail: missing frontmatter delimiter")
	}
	rest := content[len(frontmatterDelim)+1:]
	idx := strings.Index(rest, "\n"+frontmatterDelim+"\n")
	if idx < 0 {
		return nil, fmt.Errorf("dmail: missing closing frontmatter delimiter")
	}
	yamlPart := rest[:idx]
	bodyPart := rest[idx+len("\n"+frontmatterDelim+"\n"):]

	var mail DMail
	if err := yaml.Unmarshal([]byte(yamlPart), &mail); err != nil {
		return nil, fmt.Errorf("dmail parse frontmatter: %w", err)
	}
	mail.Body = strings.TrimPrefix(bodyPart, "\n")
	return &mail, nil
}
```

**Step 4: Run tests — expect PASS**

Run: `go test ./... -run "TestMarshalDMail|TestParseDMail" -count=1`

**Step 5: Commit**

```
feat: add MarshalDMail and ParseDMail (YAML frontmatter + Markdown body)
```

---

### Task 3: MailDir + EnsureMailDirs + WriteGitIgnore Update

**Files:**
- Modify: `dmail.go`
- Modify: `dmail_test.go`
- Modify: `state.go:27-31` (WriteGitIgnore content)

**Step 1: Write failing tests**

```go
func TestMailDir(t *testing.T) {
	got := MailDir("/project", "inbox")
	want := filepath.Join("/project", ".siren", "inbox")
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestEnsureMailDirs_CreatesAll(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureMailDirs(dir); err != nil {
		t.Fatalf("EnsureMailDirs: %v", err)
	}
	for _, sub := range []string{"inbox", "outbox", "archive"} {
		path := MailDir(dir, sub)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("%s not created: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", sub)
		}
	}
}

func TestEnsureMailDirs_Idempotent(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureMailDirs(dir); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := EnsureMailDirs(dir); err != nil {
		t.Fatalf("second: %v", err)
	}
}
```

Also update `TestWriteGitIgnore`:

```go
func TestWriteGitIgnore_IncludesMailDirs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, stateDir), 0755)
	if err := WriteGitIgnore(dir); err != nil {
		t.Fatalf("WriteGitIgnore: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, stateDir, ".gitignore"))
	content := string(data)
	if !strings.Contains(content, "inbox/") {
		t.Error("expected inbox/ in .gitignore")
	}
	if !strings.Contains(content, "outbox/") {
		t.Error("expected outbox/ in .gitignore")
	}
	if strings.Contains(content, "archive/") {
		t.Error("archive/ should NOT be in .gitignore (git-tracked)")
	}
}
```

**Step 2: Run tests — expect FAIL**

Run: `go test ./... -run "TestMailDir|TestEnsureMailDirs|TestWriteGitIgnore_IncludesMailDirs" -count=1`

**Step 3: Implement**

In `dmail.go`:
```go
const (
	inboxDir   = "inbox"
	outboxDir  = "outbox"
	archiveDir = "archive"
)

func MailDir(baseDir, sub string) string {
	return filepath.Join(baseDir, stateDir, sub)
}

func EnsureMailDirs(baseDir string) error {
	for _, sub := range []string{inboxDir, outboxDir, archiveDir} {
		if err := os.MkdirAll(MailDir(baseDir, sub), 0755); err != nil {
			return fmt.Errorf("create %s dir: %w", sub, err)
		}
	}
	return nil
}
```

In `state.go:28`, update WriteGitIgnore content:
```go
content := "state.json\n.run/\ninbox/\noutbox/\n"
```

**Step 4: Run ALL tests (gitignore test update may break existing)**

Run: `go test ./... -count=1`

**Step 5: Commit**

```
feat: add MailDir, EnsureMailDirs, and update WriteGitIgnore for d-mail dirs
```

---

### Task 4: ComposeDMail

**Files:**
- Modify: `dmail.go`
- Modify: `dmail_test.go`

**Step 1: Write failing tests**

```go
func TestComposeDMail_WritesToOutboxAndArchive(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureMailDirs(dir); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	mail := &DMail{
		Name:        "spec-my-42",
		Kind:        DMailSpecification,
		Description: "Ready for impl",
		Body:        "# DoD\n- item 1\n",
	}
	if err := ComposeDMail(dir, mail); err != nil {
		t.Fatalf("compose: %v", err)
	}

	// outbox file exists
	outboxPath := filepath.Join(MailDir(dir, "outbox"), "spec-my-42.md")
	if _, err := os.Stat(outboxPath); err != nil {
		t.Errorf("outbox file missing: %v", err)
	}

	// archive file exists
	archivePath := filepath.Join(MailDir(dir, "archive"), "spec-my-42.md")
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("archive file missing: %v", err)
	}

	// content is parseable
	data, _ := os.ReadFile(outboxPath)
	parsed, err := ParseDMail(data)
	if err != nil {
		t.Fatalf("parse outbox: %v", err)
	}
	if parsed.Name != "spec-my-42" {
		t.Errorf("name: got %s", parsed.Name)
	}
}

func TestComposeDMail_ValidationError(t *testing.T) {
	dir := t.TempDir()
	EnsureMailDirs(dir)
	mail := &DMail{Name: "", Kind: DMailSpecification, Description: "bad"}
	if err := ComposeDMail(dir, mail); err == nil {
		t.Error("expected validation error for empty name")
	}
}
```

**Step 2: Run tests — expect FAIL**

Run: `go test ./... -run TestComposeDMail -count=1`

**Step 3: Implement**

```go
func ComposeDMail(baseDir string, mail *DMail) error {
	if err := ValidateDMail(mail); err != nil {
		return err
	}
	data, err := MarshalDMail(mail)
	if err != nil {
		return err
	}
	filename := mail.Filename()
	for _, sub := range []string{outboxDir, archiveDir} {
		path := filepath.Join(MailDir(baseDir, sub), filename)
		if err := os.WriteFile(path, data, 0644); err != nil {
			return fmt.Errorf("dmail compose to %s: %w", sub, err)
		}
	}
	return nil
}
```

**Step 4: Run tests — expect PASS**

Run: `go test ./... -run TestComposeDMail -count=1`

**Step 5: Commit**

```
feat: add ComposeDMail (write to outbox + archive)
```

---

### Task 5: ListDMail + ReceiveDMail

**Files:**
- Modify: `dmail.go`
- Modify: `dmail_test.go`

**Step 1: Write failing tests**

```go
func TestListDMail_Empty(t *testing.T) {
	dir := t.TempDir()
	EnsureMailDirs(dir)
	files, err := ListDMail(dir, "inbox")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestListDMail_FindsFiles(t *testing.T) {
	dir := t.TempDir()
	EnsureMailDirs(dir)
	// write two files
	os.WriteFile(filepath.Join(MailDir(dir, "inbox"), "a.md"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(MailDir(dir, "inbox"), "b.md"), []byte("y"), 0644)
	os.WriteFile(filepath.Join(MailDir(dir, "inbox"), "not-md.txt"), []byte("z"), 0644)
	files, err := ListDMail(dir, "inbox")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 .md files, got %d", len(files))
	}
}

func TestReceiveDMail_MovesToArchive(t *testing.T) {
	dir := t.TempDir()
	EnsureMailDirs(dir)
	mail := &DMail{
		Name:        "feedback-d-001",
		Kind:        DMailFeedback,
		Description: "Architecture drift detected",
		Severity:    "high",
		Body:        "# Feedback\n\nDrift in auth module.\n",
	}
	data, _ := MarshalDMail(mail)
	inboxPath := filepath.Join(MailDir(dir, "inbox"), mail.Filename())
	os.WriteFile(inboxPath, data, 0644)

	// receive
	received, err := ReceiveDMail(dir, mail.Filename())
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if received.Name != "feedback-d-001" {
		t.Errorf("name: got %s", received.Name)
	}
	if received.Severity != "high" {
		t.Errorf("severity: got %s", received.Severity)
	}

	// inbox file removed
	if _, err := os.Stat(inboxPath); !os.IsNotExist(err) {
		t.Error("inbox file should be removed after receive")
	}

	// archive file exists
	archivePath := filepath.Join(MailDir(dir, "archive"), mail.Filename())
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("archive file missing: %v", err)
	}
}

func TestReceiveDMail_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	EnsureMailDirs(dir)
	_, err := ReceiveDMail(dir, "nonexistent.md")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
```

**Step 2: Run tests — expect FAIL**

Run: `go test ./... -run "TestListDMail|TestReceiveDMail" -count=1`

**Step 3: Implement**

```go
func ListDMail(baseDir, sub string) ([]string, error) {
	dir := MailDir(baseDir, sub)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("dmail list %s: %w", sub, err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

func ReceiveDMail(baseDir, filename string) (*DMail, error) {
	inboxPath := filepath.Join(MailDir(baseDir, inboxDir), filename)
	data, err := os.ReadFile(inboxPath)
	if err != nil {
		return nil, fmt.Errorf("dmail read inbox: %w", err)
	}
	mail, err := ParseDMail(data)
	if err != nil {
		return nil, fmt.Errorf("dmail parse inbox %s: %w", filename, err)
	}
	archivePath := filepath.Join(MailDir(baseDir, archiveDir), filename)
	if err := os.WriteFile(archivePath, data, 0644); err != nil {
		return nil, fmt.Errorf("dmail archive %s: %w", filename, err)
	}
	if err := os.Remove(inboxPath); err != nil {
		return nil, fmt.Errorf("dmail remove inbox %s: %w", filename, err)
	}
	return mail, nil
}
```

**Step 4: Run ALL tests**

Run: `go test ./... -count=1`

**Step 5: Commit**

```
feat: add ListDMail and ReceiveDMail (inbox → archive lifecycle)
```

---

### Task 6: Final Verification

**Step 1:** `go test ./... -count=1` — all pass
**Step 2:** `go build ./...` — builds clean
**Step 3:** `go vet ./...` — no warnings

---

## File Summary

| File | Action |
|------|--------|
| `dmail.go` | **Create** — DMail type, DMailKind, ValidateDMail, MarshalDMail, ParseDMail, MailDir, EnsureMailDirs, ComposeDMail, ListDMail, ReceiveDMail |
| `dmail_test.go` | **Create** — ~15 test functions covering all public API |
| `state.go:28` | **Modify** — WriteGitIgnore content adds `inbox/\noutbox/\n` |
| `state_test.go` | **Modify** — Add `TestWriteGitIgnore_IncludesMailDirs`, update existing gitignore test assertions |
