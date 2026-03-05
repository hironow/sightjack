package session

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/port"
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
	Action        string            `yaml:"action,omitempty"`
	Priority      int               `yaml:"priority,omitempty"`
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
	DMailCIResult      DMailKind = "ci-result"
)

// validActions is the set of valid action values per D-Mail schema v1.
// Strict on send, liberal on receive (Postel's law / S0021).
var validActions = map[string]bool{
	"retry":    true,
	"escalate": true,
	"resolve":  true,
}

// Filename returns the canonical filename: "<name>.md".
func (d *DMail) Filename() string {
	return d.Name + ".md"
}

const frontmatterDelim = "---"

// DMailIdempotencyKey computes a SHA256 content-based idempotency key from
// the core fields of a DMail (name, kind, description, body).
func DMailIdempotencyKey(mail *DMail) string {
	h := sha256.New()
	h.Write([]byte(mail.Name))
	h.Write([]byte{0})
	h.Write([]byte(string(mail.Kind)))
	h.Write([]byte{0})
	h.Write([]byte(mail.Description))
	h.Write([]byte{0})
	h.Write([]byte(mail.Body))
	return hex.EncodeToString(h.Sum(nil))
}

// MarshalDMail serializes a DMail to YAML frontmatter + Markdown body.
// Automatically injects an idempotency_key into metadata based on content hash.
func MarshalDMail(mail *DMail) ([]byte, error) {
	// Create a shallow copy to avoid mutating the caller's DMail.
	cp := *mail
	meta := make(map[string]string, len(mail.Metadata)+1)
	for k, v := range mail.Metadata {
		meta[k] = v
	}
	meta["idempotency_key"] = DMailIdempotencyKey(mail)
	cp.Metadata = meta

	var buf bytes.Buffer
	buf.WriteString(frontmatterDelim + "\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&cp); err != nil {
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

// ComposeDMail stages a d-mail via the transactional outbox store, then
// flushes it to archive/ and outbox/ using atomic file writes.
func ComposeDMail(store port.OutboxStore, mail *DMail) error {
	if err := ValidateDMail(mail); err != nil {
		return err
	}
	data, err := MarshalDMail(mail)
	if err != nil {
		return err
	}
	if err := store.Stage(mail.Filename(), data); err != nil {
		return fmt.Errorf("dmail stage: %w", err)
	}
	n, err := store.Flush()
	if err != nil {
		return fmt.Errorf("dmail flush: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("dmail flush: item not delivered (write failure, will retry)")
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
	if mail.SchemaVersion == "" {
		return fmt.Errorf("dmail: dmail-schema-version is required")
	}
	switch mail.Kind {
	case DMailSpecification, DMailReport, DMailFeedback, DMailConvergence, DMailCIResult:
		// valid
	default:
		return fmt.Errorf("dmail: invalid kind %q (valid: specification, report, feedback, convergence, ci-result)", mail.Kind)
	}
	if mail.Action != "" && !validActions[mail.Action] {
		return fmt.Errorf("dmail: invalid action %q (valid: retry, escalate, resolve)", mail.Action)
	}
	return nil
}

// ListDMail returns all .md filenames in the given mail subdirectory.
func ListDMail(baseDir, sub string) ([]string, error) {
	dir := domain.MailDir(baseDir, sub)
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
func receiveDMailIfNew(baseDir, filename string, logger domain.Logger) *DMail {
	// Consumer-side dedup: skip if already in archive.
	// NOTE: Dedup is filename-based by design — the d-mail filename acts as a
	// message ID in the protocol. Senders that need to deliver updated content
	// for the same wave must use a distinct filename (e.g. append a sequence number).
	archivePath := filepath.Join(domain.MailDir(baseDir, domain.ArchiveDir), filename)
	if _, err := os.Stat(archivePath); err == nil {
		if rmErr := os.Remove(filepath.Join(domain.MailDir(baseDir, domain.InboxDir), filename)); rmErr != nil && !os.IsNotExist(rmErr) {
			logger.Warn("dedup remove %s: %v", filename, rmErr)
		}
		return nil
	}

	mail, err := ReceiveDMail(baseDir, filename)
	if err != nil {
		logger.Warn("Failed to receive d-mail %s: %v", filename, err)
		return nil
	}
	if mail.Kind != DMailFeedback && mail.Kind != DMailConvergence && mail.Kind != DMailReport {
		return nil
	}
	return mail
}

// MonitorInbox starts monitoring the inbox directory for feedback and convergence d-mails.
// It first drains existing files (initial scan), then watches for new files via fsnotify.
// Each d-mail is received (archived + removed from inbox). Feedback, convergence,
// and report d-mails are sent to the returned channel. Consumer-side dedup is applied (MY-271).
// The channel is closed when the context is cancelled.
func MonitorInbox(ctx context.Context, baseDir string, logger domain.Logger) (<-chan *DMail, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("dmail monitor: create watcher: %w", err)
	}

	inboxPath := domain.MailDir(baseDir, domain.InboxDir)
	if err := watcher.Add(inboxPath); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("dmail monitor: add inbox: %w", err)
	}

	// Phase 1: Initial drain (synchronous, before goroutine starts).
	// watcher.Add is called first so files created during the drain
	// are caught by fsnotify and deduplicated by receiveDMailIfNew.
	var initial []*DMail
	files, listErr := ListDMail(baseDir, domain.InboxDir)
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

// DrainInboxFeedback reads all currently buffered d-mails (feedback and convergence)
// from the monitor channel and logs them. Returns the drained messages for downstream use.
func DrainInboxFeedback(ch <-chan *DMail, logger domain.Logger) []*DMail {
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
	logger.Info("Received %d d-mail(s):", len(feedback))
	for _, fb := range feedback {
		prefix := "[D-Mail]"
		if fb.Kind == DMailConvergence {
			prefix = "[D-Mail] [CONVERGENCE]"
		}
		switch fb.Severity {
		case "high":
			logger.Warn("%s [%s] %s (severity: HIGH)", prefix, fb.Name, fb.Description)
		default:
			logger.Info("%s [%s] %s", prefix, fb.Name, fb.Description)
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

// FormatReportsForPrompt formats report d-mails as a Markdown section
// suitable for injection into wave generation prompts. Reports come from
// other tools (e.g. amadeus check results) and provide cross-tool context.
// Returns "" for nil or empty input.
func FormatReportsForPrompt(reports []*DMail) string {
	if len(reports) == 0 {
		return ""
	}
	var b strings.Builder
	for _, rpt := range reports {
		fmt.Fprintf(&b, "### %s\n", rpt.Name)
		fmt.Fprintf(&b, "%s\n", rpt.Description)
		if rpt.Body != "" {
			fmt.Fprintf(&b, "\n%s\n", rpt.Body)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// FeedbackCollector accumulates feedback d-mails from both the initial
// drain and late-arriving items on the monitor channel. It replaces
// LogInboxFeedbackAsync by both displaying AND storing late arrivals,
// so they can be included in nextgen prompts.
// Convergence d-mails are tracked separately for journaling.
type FeedbackCollector struct {
	mu               sync.Mutex
	items            []*DMail
	convergenceNames []string
	notifier         port.Notifier
}

// CollectFeedback creates a FeedbackCollector seeded with initial feedback
// and starts a background goroutine to accumulate late-arriving items
// from the channel. Convergence d-mails trigger a notification via notifier.
// Safe to call with nil initial, nil channel, or nil notifier.
func CollectFeedback(initial []*DMail, ch <-chan *DMail, notifier port.Notifier, logger domain.Logger) *FeedbackCollector {
	if notifier == nil {
		notifier = &port.NopNotifier{}
	}
	c := &FeedbackCollector{notifier: notifier}
	if len(initial) > 0 {
		c.items = make([]*DMail, len(initial))
		copy(c.items, initial)
	}
	if ch != nil {
		go func() {
			for mail := range ch {
				c.addMail(mail)

				if mail.Kind == DMailConvergence {
					logger.Warn("[D-Mail] [CONVERGENCE] %s: %s", mail.Name, mail.Description)
					// Fire-and-forget with timeout to avoid blocking the drain loop.
					go func(desc string) {
						notifyCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer cancel()
						if err := c.notifier.Notify(notifyCtx, "Sightjack Convergence", desc); err != nil {
							logger.Warn("Convergence notification failed (non-fatal): %v", err)
						}
					}(mail.Description)
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

// addMail appends a d-mail to the collector under the mutex.
func (c *FeedbackCollector) addMail(mail *DMail) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = append(c.items, mail)
	if mail.Kind == DMailConvergence {
		c.convergenceNames = append(c.convergenceNames, mail.Name)
	}
}

// ConvergenceNames returns a copy of convergence d-mail names received
// mid-session. Used for journaling/state persistence.
func (c *FeedbackCollector) ConvergenceNames() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.convergenceNames) == 0 {
		return nil
	}
	cp := make([]string, len(c.convergenceNames))
	copy(cp, c.convergenceNames)
	return cp
}

// FeedbackOnly returns a copy of accumulated d-mails filtered to feedback kind
// only (excludes convergence). Use this for nextgen prompt injection where only
// feedback d-mails are relevant.
func (c *FeedbackCollector) FeedbackOnly() []*DMail {
	c.mu.Lock()
	defer c.mu.Unlock()
	var result []*DMail
	for _, m := range c.items {
		if m.Kind == DMailFeedback {
			result = append(result, m)
		}
	}
	return result
}

// ReportsOnly returns a copy of accumulated d-mails filtered to report kind
// only (excludes feedback and convergence). Use this for nextgen prompt injection
// where cross-tool reports (e.g. amadeus check results) should inform wave planning.
func (c *FeedbackCollector) ReportsOnly() []*DMail {
	c.mu.Lock()
	defer c.mu.Unlock()
	var result []*DMail
	for _, m := range c.items {
		if m.Kind == DMailReport {
			result = append(result, m)
		}
	}
	return result
}

// All returns a copy of all accumulated feedback (initial + late arrivals).
// Non-destructive: repeated calls return the same data plus any new arrivals.
func (c *FeedbackCollector) All() []*DMail {
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
	inboxPath := filepath.Join(domain.MailDir(baseDir, domain.InboxDir), filename)
	data, err := os.ReadFile(inboxPath)
	if err != nil {
		return nil, fmt.Errorf("dmail read inbox: %w", err)
	}
	mail, err := ParseDMail(data)
	if err != nil {
		return nil, fmt.Errorf("dmail parse inbox %s: %w", filename, err)
	}
	archivePath := filepath.Join(domain.MailDir(baseDir, domain.ArchiveDir), filename)
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

// WaveIssueIDs extracts unique, sorted issue IDs from wave actions.
func WaveIssueIDs(wave domain.Wave) []string {
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

// SpecificationBody formats wave actions as Markdown body for a specification d-mail.
func SpecificationBody(wave domain.Wave) string {
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

// ReportBody formats wave apply results as Markdown body for a report d-mail.
func ReportBody(wave domain.Wave, result *domain.WaveApplyResult) string {
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
func ComposeReport(store port.OutboxStore, wave domain.Wave, result *domain.WaveApplyResult) error {
	key := domain.WaveKey(wave)
	mail := &DMail{
		Name:          DMailName("report", key),
		Kind:          DMailReport,
		Description:   fmt.Sprintf("Wave %s completed", key),
		SchemaVersion: "1",
		Issues:        WaveIssueIDs(wave),
		Body:          ReportBody(wave, result),
	}
	return ComposeDMail(store, mail)
}

// FeedbackBody formats wave apply results as Markdown body for a feedback d-mail.
// Distinct from ReportBody: uses "Wave Feedback" heading to differentiate the
// sightjack → amadeus feedback loop (O2) from the standard report d-mail.
func FeedbackBody(wave domain.Wave, result *domain.WaveApplyResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Wave Feedback: %s\n\n", wave.Title)
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

// ComposeFeedback stages a feedback D-Mail for amadeus consumption.
// Called after successful wave apply to complete the sightjack → amadeus feedback loop (O2).
func ComposeFeedback(store port.OutboxStore, wave domain.Wave, result *domain.WaveApplyResult) error {
	key := domain.WaveKey(wave)
	mail := &DMail{
		Name:          DMailName("feedback", key),
		Kind:          DMailFeedback,
		Description:   fmt.Sprintf("Wave %s feedback for amadeus", key),
		SchemaVersion: "1",
		Issues:        WaveIssueIDs(wave),
		Body:          FeedbackBody(wave, result),
	}
	return ComposeDMail(store, mail)
}

// ComposeSpecification creates and sends a specification d-mail for an approved wave.
func ComposeSpecification(store port.OutboxStore, wave domain.Wave) error {
	key := domain.WaveKey(wave)
	mail := &DMail{
		Name:          DMailName("spec", key),
		Kind:          DMailSpecification,
		Description:   wave.Title,
		SchemaVersion: "1",
		Issues:        WaveIssueIDs(wave),
		Body:          SpecificationBody(wave),
	}
	return ComposeDMail(store, mail)
}
