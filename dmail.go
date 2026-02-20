package sightjack

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
