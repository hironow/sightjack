package sightjack

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

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

const (
	inboxDir   = "inbox"
	outboxDir  = "outbox"
	archiveDir = "archive"
)

// MailDir returns the path to a mail subdirectory under the state root.
func MailDir(baseDir, sub string) string {
	return filepath.Join(baseDir, stateDir, sub)
}

// EnsureMailDirs creates inbox/, outbox/, archive/ under .siren/.
func EnsureMailDirs(baseDir string) error {
	for _, sub := range []string{inboxDir, outboxDir, archiveDir} {
		if err := os.MkdirAll(MailDir(baseDir, sub), 0755); err != nil {
			return fmt.Errorf("create %s dir: %w", sub, err)
		}
	}
	return nil
}

// Filename returns the canonical filename: "<name>.md".
func (d *DMail) Filename() string {
	return d.Name + ".md"
}

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

// ComposeDMail writes a d-mail to both outbox/ and archive/.
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

// ValidateDMail checks required fields and kind validity.
func ValidateDMail(mail *DMail) error {
	if mail == nil {
		return fmt.Errorf("dmail: mail is nil")
	}
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

// ListDMail returns all .md filenames in the given mail subdirectory.
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

// WatchInbox monitors the inbox/ directory for new .md files using fsnotify.
// Returns a channel that receives filenames of newly created d-mails.
// The channel is closed when the context is cancelled.
func WatchInbox(ctx context.Context, baseDir string) (<-chan string, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("dmail watch: create watcher: %w", err)
	}

	inboxPath := MailDir(baseDir, inboxDir)
	if err := watcher.Add(inboxPath); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("dmail watch: add inbox: %w", err)
	}

	ch := make(chan string)
	go func() {
		defer close(ch)
		defer watcher.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Create) && strings.HasSuffix(event.Name, ".md") {
					filename := filepath.Base(event.Name)
					select {
					case ch <- filename:
					case <-ctx.Done():
						return
					}
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	return ch, nil
}

// ReceiveDMail reads a d-mail from inbox/, parses it, and moves it to archive/.
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

// DMailName generates a sanitized d-mail name from a prefix and wave key.
// Example: DMailName("spec", "auth:w1") → "spec-auth-w1"
func DMailName(prefix, waveKey string) string {
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteRune('-')
	for _, r := range strings.ToLower(waveKey) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r == ':':
			b.WriteRune('-')
		case r == ' ':
			b.WriteRune('_')
		default:
			b.WriteRune('_')
		}
	}
	return strings.TrimRight(b.String(), "_")
}

// waveIssueIDs extracts unique, sorted issue IDs from wave actions.
func waveIssueIDs(wave Wave) []string {
	seen := make(map[string]bool)
	for _, a := range wave.Actions {
		if a.IssueID != "" {
			seen[a.IssueID] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// specificationBody formats wave actions as Markdown body for a specification d-mail.
func specificationBody(wave Wave) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", wave.Title)
	if wave.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", wave.Description)
	}
	fmt.Fprintf(&b, "## Actions\n\n")
	for _, a := range wave.Actions {
		fmt.Fprintf(&b, "- [%s] %s: %s\n", a.Type, a.IssueID, a.Description)
	}
	return b.String()
}

// reportBody formats wave apply results as Markdown body for a report d-mail.
func reportBody(wave Wave, result *WaveApplyResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Wave Completed: %s\n\n", wave.Title)
	fmt.Fprintf(&b, "Applied %d action(s).\n\n", result.Applied)
	if len(result.Errors) > 0 {
		fmt.Fprintf(&b, "## Errors\n\n")
		for _, e := range result.Errors {
			fmt.Fprintf(&b, "- %s\n", e)
		}
		b.WriteString("\n")
	}
	if len(result.Ripples) > 0 {
		fmt.Fprintf(&b, "## Ripple Effects\n\n")
		for _, r := range result.Ripples {
			fmt.Fprintf(&b, "- [%s] %s\n", r.ClusterName, r.Description)
		}
	}
	return b.String()
}

// ComposeReport creates and sends a report d-mail for a completed wave.
func ComposeReport(baseDir string, wave Wave, result *WaveApplyResult) error {
	key := WaveKey(wave)
	mail := &DMail{
		Name:        DMailName("report", key),
		Kind:        DMailReport,
		Description: fmt.Sprintf("Wave %s completed", key),
		Issues:      waveIssueIDs(wave),
		Body:        reportBody(wave, result),
	}
	return ComposeDMail(baseDir, mail)
}

// ComposeSpecification creates and sends a specification d-mail for an approved wave.
func ComposeSpecification(baseDir string, wave Wave) error {
	key := WaveKey(wave)
	mail := &DMail{
		Name:        DMailName("spec", key),
		Kind:        DMailSpecification,
		Description: wave.Title,
		Issues:      waveIssueIDs(wave),
		Body:        specificationBody(wave),
	}
	return ComposeDMail(baseDir, mail)
}
