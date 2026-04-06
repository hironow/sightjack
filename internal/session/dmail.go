package session

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
	"gopkg.in/yaml.v3"
)

// DMail is an alias for the domain-owned D-Mail model (SPEC-004 Stage 2).
// Session layer retains orchestration and I/O; type and validation are in domain.
type DMail = domain.DMail

// DMailKind is an alias for the domain-owned kind type.
type DMailKind = domain.DMailKind

// Kind constants re-exported from domain for backward compatibility.
// These will be removed in Stage 3 when all call sites use domain directly.
const (
	DMailSpecification  = domain.KindSpecification
	DMailReport         = domain.KindReport
	DMailDesignFeedback = domain.KindDesignFeedback
	DMailImplFeedback   = domain.KindImplFeedback
	DMailConvergence    = domain.KindConvergence
	DMailCIResult       = domain.KindCIResult
)

// Filename is now defined on domain.DMail — no session-level duplicate needed.

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
func ComposeDMail(ctx context.Context, store port.OutboxStore, mail *DMail) error {
	if err := ValidateDMail(mail); err != nil {
		return err
	}
	data, err := MarshalDMail(mail)
	if err != nil {
		return err
	}
	if err := store.Stage(ctx, mail.Filename(), data); err != nil {
		return fmt.Errorf("dmail stage: %w", err)
	}
	n, err := store.Flush(ctx)
	if err != nil {
		return fmt.Errorf("dmail flush: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("dmail flush: item not delivered (write failure, will retry)")
	}
	return nil
}

// ValidateDMail delegates to domain.ValidateDMail (SPEC-004 Stage 2).
// Session retains this wrapper for nil-check that domain doesn't handle.
func ValidateDMail(mail *DMail) error {
	if mail == nil {
		return fmt.Errorf("dmail: mail is nil")
	}
	return domain.ValidateDMail(mail)
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
		if rmErr := os.Remove(filepath.Join(domain.MailDir(baseDir, domain.InboxDir), filename)); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			logger.Warn("dedup remove %s: %v", filename, rmErr)
		}
		return nil
	}

	mail, err := ReceiveDMail(baseDir, filename)
	if err != nil {
		logger.Warn("Failed to receive d-mail %s: %v", filename, err)
		return nil
	}
	if mail.Kind != DMailDesignFeedback && mail.Kind != DMailImplFeedback && mail.Kind != DMailConvergence && mail.Kind != DMailReport {
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
		domain.LogBanner(logger, domain.BannerRecv, string(fb.Kind), fb.Name, fb.Description)
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
	mu          sync.Mutex
	items       []*DMail
	convNames   []string
	notifier    port.Notifier
	notify      chan struct{} // signals new D-Mail arrival (buffered, size 1)
	snapshotIdx int           // index up to which items have been seen
}

// CollectFeedback creates a FeedbackCollector seeded with initial feedback
// and starts a background goroutine to accumulate late-arriving items
// from the channel. Convergence d-mails trigger a notification via notifier.
// Safe to call with nil initial, nil channel, or nil notifier.
func CollectFeedback(initial []*DMail, ch <-chan *DMail, notifier port.Notifier, logger domain.Logger) *FeedbackCollector {
	return collectFeedback(initial, ch, notifier, logger, nil)
}

func CollectFeedbackWithHook(initial []*DMail, ch <-chan *DMail, notifier port.Notifier, logger domain.Logger, onMail func(*DMail)) *FeedbackCollector {
	return collectFeedback(initial, ch, notifier, logger, onMail)
}

func collectFeedback(initial []*DMail, ch <-chan *DMail, notifier port.Notifier, logger domain.Logger, onMail func(*DMail)) *FeedbackCollector {
	if notifier == nil {
		notifier = &port.NopNotifier{}
	}
	c := &FeedbackCollector{
		notifier: notifier,
		notify:   make(chan struct{}, 1),
	}
	if len(initial) > 0 {
		c.items = make([]*DMail, len(initial))
		copy(c.items, initial)
		if onMail != nil {
			for _, mail := range initial {
				onMail(mail)
			}
		}
	}
	if ch != nil {
		go func() {
			for mail := range ch {
				domain.LogBanner(logger, domain.BannerRecv, string(mail.Kind), mail.Name, mail.Description)
				c.addMail(mail)
				if onMail != nil {
					onMail(mail)
				}
				select {
				case c.notify <- struct{}{}:
				default: // non-blocking, don't block if already signaled
				}

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
		c.convNames = append(c.convNames, mail.Name)
	}
}

// NotifyCh returns a channel that receives a signal when new D-Mails arrive.
// Used by the waiting phase to wake up when inbox content changes.
func (c *FeedbackCollector) NotifyCh() <-chan struct{} { return c.notify }

// Snapshot marks the current position in the collected items.
// Call before entering the waiting phase.
func (c *FeedbackCollector) Snapshot() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshotIdx = len(c.items)
}

// NewSinceSnapshot returns D-Mails received since the last Snapshot call.
func (c *FeedbackCollector) NewSinceSnapshot() []*DMail {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.snapshotIdx >= len(c.items) {
		return nil
	}
	result := make([]*DMail, len(c.items)-c.snapshotIdx)
	copy(result, c.items[c.snapshotIdx:])
	return result
}

// convergenceNames returns a copy of convergence d-mail names received
// mid-session. Used for journaling/state persistence.
func (c *FeedbackCollector) convergenceNames() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.convNames) == 0 {
		return nil
	}
	cp := make([]string, len(c.convNames))
	copy(cp, c.convNames)
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
		if m.Kind == DMailDesignFeedback || m.Kind == DMailImplFeedback {
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

// uuidFunc is the UUID generator used by DMailName. Override in tests for determinism.
var uuidFunc = shortUUID

// DMailName generates a collision-safe d-mail name with tool prefix and UUID suffix.
// Format: sj-{kind}-{sanitized-key}_{uuid8}
// Example: DMailName("spec", "error-handling:w1") → "sj-spec-error-handling-w1_a3f2b7c4"
func DMailName(prefix, waveKey string) string {
	key := sanitizeDMailKey(waveKey)
	if key == "" {
		return "sj-" + prefix + "_" + uuidFunc()
	}
	return "sj-" + prefix + "-" + key + "_" + uuidFunc()
}

// sanitizeDMailKey normalizes a key for use in D-Mail filenames.
// Keeps a-z, 0-9, -. Converts : and space to -. Compresses consecutive - or _.
// Trims leading/trailing - and _.
func sanitizeDMailKey(key string) string {
	var b strings.Builder
	prev := rune(0)
	for _, r := range strings.ToLower(key) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prev = r
		case r == '-', r == ':', r == ' ', r == '_':
			if prev != '-' {
				b.WriteRune('-')
				prev = '-'
			}
		default:
			// skip non-ascii (Japanese, emoji, etc.)
		}
	}
	return strings.Trim(b.String(), "-")
}

// shortUUID returns the first 8 hex characters of a UUID v4.
func shortUUID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// Fallback: use timestamp-based ID if crypto/rand fails
		return fmt.Sprintf("%08x", time.Now().UnixNano()&0xFFFFFFFF)
	}
	buf[6] = (buf[6] & 0x0f) | 0x40 // version 4
	buf[8] = (buf[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%08x", buf[:4])
}
