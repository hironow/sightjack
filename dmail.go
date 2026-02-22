package sightjack

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// DMail represents a d-mail message: YAML frontmatter + Markdown body.
type DMail struct {
	Name          string            `yaml:"name"`
	Kind          DMailKind         `yaml:"kind"`
	Description   string            `yaml:"description"`
	SchemaVersion string            `yaml:"dmail-schema-version,omitempty"`
	Issues        []string          `yaml:"issues,omitempty"`
	Severity      string            `yaml:"severity,omitempty"`
	Metadata      map[string]string `yaml:"metadata,omitempty"`
	Body          string            `yaml:"-"`
}

// DMailKind is the message type for d-mails.
type DMailKind string

const (
	DMailSpecification DMailKind = "specification"
	DMailReport        DMailKind = "report"
	DMailFeedback      DMailKind = "feedback"
	DMailConvergence   DMailKind = "convergence"
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
	for _, sub := range []string{archiveDir, outboxDir} {
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
	case DMailSpecification, DMailReport, DMailFeedback, DMailConvergence:
		// valid
	default:
		return fmt.Errorf("dmail: invalid kind %q (valid: specification, report, feedback, convergence)", mail.Kind)
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

// receiveDMailIfNew reads a d-mail from inbox, applies consumer-side dedup (MY-271),
// archives it, and returns it only if it is a feedback d-mail.
// Returns nil for already-archived, non-feedback, or unreadable files.
func receiveDMailIfNew(baseDir, filename string, logger *Logger) *DMail {
	// Consumer-side dedup: skip if already in archive.
	// NOTE: Dedup is filename-based by design — the d-mail filename acts as a
	// message ID in the protocol. Senders that need to deliver updated content
	// for the same wave must use a distinct filename (e.g. append a sequence number).
	archivePath := filepath.Join(MailDir(baseDir, archiveDir), filename)
	if _, err := os.Stat(archivePath); err == nil {
		os.Remove(filepath.Join(MailDir(baseDir, inboxDir), filename))
		return nil
	}

	mail, err := ReceiveDMail(baseDir, filename)
	if err != nil {
		logger.Warn("Failed to receive d-mail %s: %v", filename, err)
		return nil
	}
	if mail.Kind != DMailFeedback && mail.Kind != DMailConvergence {
		return nil
	}
	return mail
}

// MonitorInbox starts monitoring the inbox directory for feedback d-mails.
// It first drains existing files (initial scan), then watches for new files via fsnotify.
// Each d-mail is received (archived + removed from inbox). Only feedback d-mails
// are sent to the returned channel. Consumer-side dedup is applied (MY-271).
// The channel is closed when the context is cancelled.
func MonitorInbox(ctx context.Context, baseDir string, logger *Logger) (<-chan *DMail, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("dmail monitor: create watcher: %w", err)
	}

	inboxPath := MailDir(baseDir, inboxDir)
	if err := watcher.Add(inboxPath); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("dmail monitor: add inbox: %w", err)
	}

	// Phase 1: Initial drain (synchronous, before goroutine starts).
	// watcher.Add is called first so files created during the drain
	// are caught by fsnotify and deduplicated by receiveDMailIfNew.
	var initial []*DMail
	files, listErr := ListDMail(baseDir, inboxDir)
	if listErr == nil {
		for _, filename := range files {
			if mail := receiveDMailIfNew(baseDir, filename, logger); mail != nil {
				initial = append(initial, mail)
			}
		}
	}

	ch := make(chan *DMail, len(initial))
	for _, mail := range initial {
		ch <- mail
	}

	// Phase 2: Watch for new files (async).
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
				// NOTE: Write events handle partial-write resilience. If Create fires
				// before the file is fully written, receiveDMailIfNew fails and leaves
				// the file in inbox/. Subsequent Write events re-trigger processing.
				// Archive-based dedup in receiveDMailIfNew prevents double delivery.
				if (event.Has(fsnotify.Create) || event.Has(fsnotify.Write)) && strings.HasSuffix(event.Name, ".md") {
					filename := filepath.Base(event.Name)
					if mail := receiveDMailIfNew(baseDir, filename, logger); mail != nil {
						select {
						case ch <- mail:
						case <-ctx.Done():
							return
						}
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

// DrainInboxFeedback reads all currently buffered feedback from the monitor channel
// and displays them to the CLI. Returns the drained feedback messages for downstream use.
func DrainInboxFeedback(ch <-chan *DMail, logger *Logger) []*DMail {
	if ch == nil {
		return nil
	}
	var feedback []*DMail
loop:
	for {
		select {
		case mail, ok := <-ch:
			if !ok {
				break loop
			}
			feedback = append(feedback, mail)
		default:
			break loop
		}
	}
	if len(feedback) == 0 {
		return nil
	}
	logger.Info("Received %d feedback d-mail(s):", len(feedback))
	for _, fb := range feedback {
		switch fb.Severity {
		case "high":
			logger.Warn("[%s] %s (severity: HIGH)", fb.Name, fb.Description)
		default:
			logger.Info("[%s] %s", fb.Name, fb.Description)
		}
	}
	return feedback
}

// FormatFeedbackForPrompt formats feedback d-mails as a Markdown section
// suitable for injection into wave generation prompts. HIGH severity items
// are emphasized with a ### [HIGH] header. Returns "" for nil or empty input.
func FormatFeedbackForPrompt(feedback []*DMail) string {
	if len(feedback) == 0 {
		return ""
	}
	var b strings.Builder
	for _, fb := range feedback {
		if fb.Severity == "high" {
			fmt.Fprintf(&b, "### [HIGH] %s\n", fb.Name)
		} else {
			fmt.Fprintf(&b, "### %s\n", fb.Name)
		}
		fmt.Fprintf(&b, "%s\n", fb.Description)
		if fb.Body != "" {
			fmt.Fprintf(&b, "\n%s\n", fb.Body)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// feedbackCollector accumulates feedback d-mails from both the initial
// drain and late-arriving items on the monitor channel. It replaces
// LogInboxFeedbackAsync by both displaying AND storing late arrivals,
// so they can be included in nextgen prompts.
// Convergence d-mails are tracked separately for journaling.
type feedbackCollector struct {
	mu               sync.Mutex
	items            []*DMail
	convergenceNames []string
	notifier         Notifier
}

// CollectFeedback creates a feedbackCollector seeded with initial feedback
// and starts a background goroutine to accumulate late-arriving items
// from the channel. Convergence d-mails trigger a notification via notifier.
// Safe to call with nil initial, nil channel, or nil notifier.
func CollectFeedback(initial []*DMail, ch <-chan *DMail, notifier Notifier, logger *Logger) *feedbackCollector {
	if notifier == nil {
		notifier = &NopNotifier{}
	}
	c := &feedbackCollector{notifier: notifier}
	if len(initial) > 0 {
		c.items = make([]*DMail, len(initial))
		copy(c.items, initial)
	}
	if ch != nil {
		go func() {
			for mail := range ch {
				c.mu.Lock()
				c.items = append(c.items, mail)
				if mail.Kind == DMailConvergence {
					c.convergenceNames = append(c.convergenceNames, mail.Name)
				}
				c.mu.Unlock()

				if mail.Kind == DMailConvergence {
					logger.Warn("[D-Mail] [CONVERGENCE] %s: %s", mail.Name, mail.Description)
					if err := c.notifier.Notify(context.Background(), "Sightjack Convergence", mail.Description); err != nil {
						logger.Warn("Convergence notification failed (non-fatal): %v", err)
					}
				} else {
					switch mail.Severity {
					case "high":
						logger.Warn("[D-Mail] [%s] %s (severity: HIGH)", mail.Name, mail.Description)
					default:
						logger.Info("[D-Mail] [%s] %s", mail.Name, mail.Description)
					}
				}
			}
		}()
	}
	return c
}

// ConvergenceNames returns a copy of convergence d-mail names received
// mid-session. Used for journaling/state persistence.
func (c *feedbackCollector) ConvergenceNames() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.convergenceNames) == 0 {
		return nil
	}
	cp := make([]string, len(c.convergenceNames))
	copy(cp, c.convergenceNames)
	return cp
}

// All returns a copy of all accumulated feedback (initial + late arrivals).
// Non-destructive: repeated calls return the same data plus any new arrivals.
func (c *feedbackCollector) All() []*DMail {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) == 0 {
		return nil
	}
	cp := make([]*DMail, len(c.items))
	copy(cp, c.items)
	return cp
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
		Name:          DMailName("report", key),
		Kind:          DMailReport,
		Description:   fmt.Sprintf("Wave %s completed", key),
		SchemaVersion: "1",
		Issues:        waveIssueIDs(wave),
		Body:          reportBody(wave, result),
	}
	return ComposeDMail(baseDir, mail)
}

// ComposeSpecification creates and sends a specification d-mail for an approved wave.
func ComposeSpecification(baseDir string, wave Wave) error {
	key := WaveKey(wave)
	mail := &DMail{
		Name:          DMailName("spec", key),
		Kind:          DMailSpecification,
		Description:   wave.Title,
		SchemaVersion: "1",
		Issues:        waveIssueIDs(wave),
		Body:          specificationBody(wave),
	}
	return ComposeDMail(baseDir, mail)
}
