package session_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/session"
	"github.com/hironow/sightjack/internal/usecase/port"
)

func TestDMailKind_Valid(t *testing.T) {
	kinds := []session.DMailKind{session.DMailSpecification, session.DMailReport, session.DMailDesignFeedback, session.DMailImplFeedback, session.DMailConvergence, session.DMailCIResult}
	for _, k := range kinds {
		if k == "" {
			t.Errorf("kind constant should not be empty")
		}
	}
}

func TestValidateDMail_ConvergenceKind(t *testing.T) {
	// given: a d-mail with convergence kind
	mail := &session.DMail{
		Name:          "convergence-test",
		Kind:          session.DMailConvergence,
		Description:   "Convergence signal from phonewave",
		SchemaVersion: "1",
	}

	// when
	err := session.ValidateDMail(mail)

	// then
	if err != nil {
		t.Errorf("expected convergence kind to be valid, got: %v", err)
	}
}

func TestMarshalDMail_SchemaVersion(t *testing.T) {
	// given: a d-mail with SchemaVersion set
	mail := &session.DMail{
		Name:          "schema-v1",
		Kind:          session.DMailSpecification,
		Description:   "Schema version test",
		SchemaVersion: "1",
	}

	// when
	data, err := session.MarshalDMail(mail)

	// then
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "dmail-schema-version") {
		t.Error("expected dmail-schema-version in frontmatter")
	}
	if !strings.Contains(content, `"1"`) && !strings.Contains(content, "'1'") && !strings.Contains(content, ": \"1\"") {
		// YAML may quote differently; just check the value is present
		if !strings.Contains(content, "1") {
			t.Error("expected schema version value '1' in frontmatter")
		}
	}

	// roundtrip: parse should preserve SchemaVersion
	parsed, parseErr := session.ParseDMail(data)
	if parseErr != nil {
		t.Fatalf("parse: %v", parseErr)
	}
	if parsed.SchemaVersion != "1" {
		t.Errorf("SchemaVersion roundtrip: got %q, want %q", parsed.SchemaVersion, "1")
	}
}

// testOutboxStore creates a SQLiteOutboxStore for testing and registers cleanup.
func testOutboxStore(t *testing.T, dir string) port.OutboxStore {
	t.Helper()
	store, err := session.NewOutboxStoreForDir(dir)
	if err != nil {
		t.Fatalf("create outbox store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestComposeSpecification_SetsSchemaVersion(t *testing.T) {
	// given
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "gate",
		Title:       "Gate test",
		Actions:     []domain.WaveAction{{Type: "add_dod", IssueID: "MY-1", Description: "test"}},
	}

	// when
	err := session.ComposeSpecification(context.Background(), store, wave)

	// then
	if err != nil {
		t.Fatalf("ComposeSpecification: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(domain.MailDir(dir, "outbox"), "spec-gate-w1.md"))
	mail, _ := session.ParseDMail(data)
	if mail.SchemaVersion != "1" {
		t.Errorf("SchemaVersion: got %q, want %q", mail.SchemaVersion, "1")
	}
}

func TestComposeReport_SetsSchemaVersion(t *testing.T) {
	// given
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "gate",
		Title:       "Gate test",
		Actions:     []domain.WaveAction{{Type: "add_dod", IssueID: "MY-1", Description: "test"}},
	}
	result := &domain.WaveApplyResult{WaveID: "w1", Applied: 1}

	// when
	err := session.ComposeReport(context.Background(), store, wave, result)

	// then
	if err != nil {
		t.Fatalf("ComposeReport: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(domain.MailDir(dir, "outbox"), "report-gate-w1.md"))
	mail, _ := session.ParseDMail(data)
	if mail.SchemaVersion != "1" {
		t.Errorf("SchemaVersion: got %q, want %q", mail.SchemaVersion, "1")
	}
}

func TestValidateDMail_Valid(t *testing.T) {
	mail := &session.DMail{
		Name:          "spec-my-42",
		Kind:          session.DMailSpecification,
		Description:   "Issue MY-42 ready for implementation",
		SchemaVersion: "1",
	}
	if err := session.ValidateDMail(mail); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateDMail_MissingName(t *testing.T) {
	mail := &session.DMail{Kind: session.DMailSpecification, Description: "desc"}
	if err := session.ValidateDMail(mail); err == nil {
		t.Error("expected error for missing name")
	}
}

func TestValidateDMail_MissingKind(t *testing.T) {
	mail := &session.DMail{Name: "test", Description: "desc", SchemaVersion: "1"}
	err := session.ValidateDMail(mail)
	if err == nil {
		t.Error("expected error for missing kind")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid kind") {
		t.Errorf("expected kind validation error, got: %v", err)
	}
}

func TestValidateDMail_InvalidKind(t *testing.T) {
	mail := &session.DMail{Name: "test", Kind: "invalid", Description: "desc", SchemaVersion: "1"}
	err := session.ValidateDMail(mail)
	if err == nil {
		t.Error("expected error for invalid kind")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid kind") {
		t.Errorf("expected kind validation error, got: %v", err)
	}
}

func TestValidateDMail_MissingDescription(t *testing.T) {
	mail := &session.DMail{Name: "test", Kind: session.DMailDesignFeedback}
	if err := session.ValidateDMail(mail); err == nil {
		t.Error("expected error for missing description")
	}
}

func TestValidateDMail_MissingSchemaVersion(t *testing.T) {
	// given: d-mail with all fields except SchemaVersion
	mail := &session.DMail{
		Name:        "test",
		Kind:        session.DMailDesignFeedback,
		Description: "feedback message",
	}

	// when
	err := session.ValidateDMail(mail)

	// then
	if err == nil {
		t.Error("expected error for missing schema version")
	}
}

func TestValidateDMail_ValidSchemaVersion(t *testing.T) {
	// given: d-mail with SchemaVersion set
	mail := &session.DMail{
		Name:          "test",
		Kind:          session.DMailDesignFeedback,
		Description:   "feedback message",
		SchemaVersion: "1",
	}

	// when
	err := session.ValidateDMail(mail)

	// then
	if err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestValidateDMail_Nil(t *testing.T) {
	if err := session.ValidateDMail(nil); err == nil {
		t.Error("expected error for nil mail")
	}
}

func TestDMail_Filename(t *testing.T) {
	mail := &session.DMail{Name: "spec-my-42"}
	if got := mail.Filename(); got != "spec-my-42.md" {
		t.Errorf("got %s, want spec-my-42.md", got)
	}
}

func TestMarshalDMail_Basic(t *testing.T) {
	mail := &session.DMail{
		Name:        "spec-my-42",
		Kind:        session.DMailSpecification,
		Description: "Issue MY-42 ready",
		Issues:      []string{"MY-42"},
		Body:        "# Rate Limiting\n\n## DoD\n- Token bucket\n",
	}
	data, err := session.MarshalDMail(mail)
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
	original := &session.DMail{
		Name:        "report-my-99",
		Kind:        session.DMailReport,
		Description: "PR merged for MY-99",
		Issues:      []string{"MY-99"},
		Severity:    "medium",
		Metadata:    map[string]string{"created_at": "2026-02-20T12:00:00Z"},
		Body:        "# Implementation Report\n\nPR #42 merged.\n",
	}
	data, err := session.MarshalDMail(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	parsed, err := session.ParseDMail(data)
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
	_, err := session.ParseDMail(data)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestParseDMail_NoFrontmatter(t *testing.T) {
	data := []byte("just markdown body\n")
	_, err := session.ParseDMail(data)
	if err == nil {
		t.Error("expected error for missing frontmatter")
	}
}

func TestMailDir(t *testing.T) {
	got := domain.MailDir("/project", "inbox")
	want := filepath.Join("/project", ".siren", "inbox")
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestEnsureMailDirs_CreatesAll(t *testing.T) {
	dir := t.TempDir()
	if err := session.EnsureMailDirs(dir); err != nil {
		t.Fatalf("EnsureMailDirs: %v", err)
	}
	for _, sub := range []string{"inbox", "outbox", "archive"} {
		path := domain.MailDir(dir, sub)
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
	if err := session.EnsureMailDirs(dir); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := session.EnsureMailDirs(dir); err != nil {
		t.Fatalf("second: %v", err)
	}
}

func TestComposeDMail_WritesToOutboxAndArchive(t *testing.T) {
	dir := t.TempDir()
	if err := session.EnsureMailDirs(dir); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	store := testOutboxStore(t, dir)
	mail := &session.DMail{
		Name:          "spec-my-42",
		Kind:          session.DMailSpecification,
		Description:   "Ready for impl",
		SchemaVersion: "1",
		Body:          "# DoD\n- item 1\n",
	}
	if err := session.ComposeDMail(context.Background(), store, mail); err != nil {
		t.Fatalf("compose: %v", err)
	}

	// outbox file exists
	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), "spec-my-42.md")
	if _, err := os.Stat(outboxPath); err != nil {
		t.Errorf("outbox file missing: %v", err)
	}

	// archive file exists
	archivePath := filepath.Join(domain.MailDir(dir, "archive"), "spec-my-42.md")
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("archive file missing: %v", err)
	}

	// content is parseable
	data, _ := os.ReadFile(outboxPath)
	parsed, err := session.ParseDMail(data)
	if err != nil {
		t.Fatalf("parse outbox: %v", err)
	}
	if parsed.Name != "spec-my-42" {
		t.Errorf("name: got %s", parsed.Name)
	}
}

func TestComposeDMail_ValidationError(t *testing.T) {
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)
	mail := &session.DMail{Name: "", Kind: session.DMailSpecification, Description: "bad"}
	if err := session.ComposeDMail(context.Background(), store, mail); err == nil {
		t.Error("expected validation error for empty name")
	}
}

func TestListDMail_Empty(t *testing.T) {
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	files, err := session.ListDMail(dir, "inbox")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestListDMail_FindsFiles(t *testing.T) {
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	os.WriteFile(filepath.Join(domain.MailDir(dir, "inbox"), "a.md"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(domain.MailDir(dir, "inbox"), "b.md"), []byte("y"), 0644)
	os.WriteFile(filepath.Join(domain.MailDir(dir, "inbox"), "not-md.txt"), []byte("z"), 0644)
	files, err := session.ListDMail(dir, "inbox")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 .md files, got %d", len(files))
	}
}

func TestReceiveDMail_MovesToArchive(t *testing.T) {
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	mail := &session.DMail{
		Name:        "feedback-d-001",
		Kind:        session.DMailDesignFeedback,
		Description: "Architecture drift detected",
		Severity:    "high",
		Body:        "# Feedback\n\nDrift in auth module.\n",
	}
	data, _ := session.MarshalDMail(mail)
	inboxPath := filepath.Join(domain.MailDir(dir, "inbox"), mail.Filename())
	os.WriteFile(inboxPath, data, 0644)

	// receive
	received, err := session.ReceiveDMail(dir, mail.Filename())
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
	archivePath := filepath.Join(domain.MailDir(dir, "archive"), mail.Filename())
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("archive file missing: %v", err)
	}
}

func TestReceiveDMail_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	_, err := session.ReceiveDMail(dir, "nonexistent.md")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestDMailName_SanitizesWaveKey(t *testing.T) {
	got := session.DMailName("spec", "auth:w1")
	if got != "spec-auth-w1" {
		t.Errorf("got %s, want spec-auth-w1", got)
	}
}

func TestDMailName_HandlesSpecialChars(t *testing.T) {
	got := session.DMailName("report", "My Cluster:wave-2")
	if got != "report-my_cluster-wave-2" {
		t.Errorf("got %s, want report-my_cluster-wave-2", got)
	}
}

func TestComposeSpecification_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)

	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "auth",
		Title:       "Add DoD to auth issues",
		Description: "First wave for auth cluster",
		Actions: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-42", Description: "Token bucket rate limiting"},
			{Type: "add_dependency", IssueID: "MY-42", Description: "Depends on auth middleware"},
			{Type: "add_dod", IssueID: "MY-43", Description: "Session management"},
		},
	}

	err := session.ComposeSpecification(context.Background(), store, wave)
	if err != nil {
		t.Fatalf("ComposeSpecification: %v", err)
	}

	// outbox file exists
	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), "spec-auth-w1.md")
	data, readErr := os.ReadFile(outboxPath)
	if readErr != nil {
		t.Fatalf("outbox file missing: %v", readErr)
	}

	// parse and verify
	mail, parseErr := session.ParseDMail(data)
	if parseErr != nil {
		t.Fatalf("parse: %v", parseErr)
	}
	if mail.Kind != session.DMailSpecification {
		t.Errorf("kind: got %s, want specification", mail.Kind)
	}
	if mail.Name != "spec-auth-w1" {
		t.Errorf("name: got %s", mail.Name)
	}
	// issues should be unique and sorted
	if len(mail.Issues) != 2 {
		t.Errorf("issues count: got %d, want 2 (MY-42, MY-43)", len(mail.Issues))
	}
	if mail.Body == "" {
		t.Error("body should not be empty")
	}
	if !strings.Contains(mail.Body, "Token bucket rate limiting") {
		t.Error("body should contain action descriptions")
	}

	// archive file also exists
	archivePath := filepath.Join(domain.MailDir(dir, "archive"), "spec-auth-w1.md")
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("archive file missing: %v", err)
	}
}

func TestComposeReport_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)

	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "auth",
		Title:       "Add DoD to auth issues",
		Actions: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-42", Description: "Token bucket"},
		},
	}
	applyResult := &domain.WaveApplyResult{
		WaveID:  "w1",
		Applied: 1,
		Ripples: []domain.Ripple{
			{ClusterName: "api", Description: "Rate limiting affects API cluster"},
		},
	}

	err := session.ComposeReport(context.Background(), store, wave, applyResult)
	if err != nil {
		t.Fatalf("ComposeReport: %v", err)
	}

	// outbox file exists and is parseable
	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), "report-auth-w1.md")
	data, readErr := os.ReadFile(outboxPath)
	if readErr != nil {
		t.Fatalf("outbox file missing: %v", readErr)
	}

	mail, parseErr := session.ParseDMail(data)
	if parseErr != nil {
		t.Fatalf("parse: %v", parseErr)
	}
	if mail.Kind != session.DMailReport {
		t.Errorf("kind: got %s, want report", mail.Kind)
	}
	if mail.Name != "report-auth-w1" {
		t.Errorf("name: got %s", mail.Name)
	}
	if !strings.Contains(mail.Body, "Rate limiting affects API cluster") {
		t.Error("body should contain ripple description")
	}

	// archive file also exists
	archivePath := filepath.Join(domain.MailDir(dir, "archive"), "report-auth-w1.md")
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("archive file missing: %v", err)
	}
}

func TestComposeReport_NoRipples(t *testing.T) {
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)

	wave := domain.Wave{
		ID:          "w2",
		ClusterName: "db",
		Title:       "Database migrations",
		Actions: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-50", Description: "Schema migration"},
		},
	}
	applyResult := &domain.WaveApplyResult{
		WaveID:  "w2",
		Applied: 1,
	}

	err := session.ComposeReport(context.Background(), store, wave, applyResult)
	if err != nil {
		t.Fatalf("ComposeReport: %v", err)
	}

	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), "report-db-w2.md")
	data, _ := os.ReadFile(outboxPath)
	mail, _ := session.ParseDMail(data)
	if mail.Body == "" {
		t.Error("body should not be empty even without ripples")
	}
}

func TestMonitorInbox_DrainsExistingFeedback(t *testing.T) {
	dir := t.TempDir()
	session.EnsureMailDirs(dir)

	// Place feedback in inbox before starting monitor
	fb := &session.DMail{
		Name:        "feedback-mon-001",
		Kind:        session.DMailDesignFeedback,
		Description: "Drift detected",
		Severity:    "high",
		Body:        "# Feedback\n",
	}
	data, _ := session.MarshalDMail(fb)
	os.WriteFile(filepath.Join(domain.MailDir(dir, "inbox"), fb.Filename()), data, 0644)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := session.MonitorInbox(ctx, dir, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("MonitorInbox: %v", err)
	}

	// Should receive the existing feedback immediately (buffered)
	select {
	case mail, ok := <-ch:
		if !ok {
			t.Fatal("channel closed unexpectedly")
		}
		if mail.Name != "feedback-mon-001" {
			t.Errorf("name: got %s, want feedback-mon-001", mail.Name)
		}
		if mail.Severity != "high" {
			t.Errorf("severity: got %s, want high", mail.Severity)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for initial feedback")
	}

	// inbox should be empty (archived)
	files, _ := session.ListDMail(dir, "inbox")
	if len(files) != 0 {
		t.Errorf("inbox not empty: %d files", len(files))
	}

	// archive should have the file
	archivePath := filepath.Join(domain.MailDir(dir, "archive"), fb.Filename())
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("archive file missing: %v", err)
	}
}

func TestMonitorInbox_SkipsNonFeedback(t *testing.T) {
	dir := t.TempDir()
	session.EnsureMailDirs(dir)

	// Place a specification d-mail in inbox
	spec := &session.DMail{
		Name:        "spec-mon-001",
		Kind:        session.DMailSpecification,
		Description: "Spec for issue",
		Body:        "# Spec\n",
	}
	data, _ := session.MarshalDMail(spec)
	os.WriteFile(filepath.Join(domain.MailDir(dir, "inbox"), spec.Filename()), data, 0644)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := session.MonitorInbox(ctx, dir, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("MonitorInbox: %v", err)
	}

	// Should NOT receive anything (spec is not feedback)
	select {
	case mail := <-ch:
		t.Errorf("unexpected feedback: %s", mail.Name)
	case <-time.After(200 * time.Millisecond):
		// expected: no feedback
	}

	// But the spec should be archived (received, just not sent to channel)
	archivePath := filepath.Join(domain.MailDir(dir, "archive"), spec.Filename())
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("archive file missing: %v", err)
	}
}

func TestMonitorInbox_DedupSkipsArchived(t *testing.T) {
	dir := t.TempDir()
	session.EnsureMailDirs(dir)

	// Place feedback in both inbox and archive (already processed)
	fb := &session.DMail{
		Name:        "feedback-mon-dup",
		Kind:        session.DMailDesignFeedback,
		Description: "Duplicate",
		Body:        "# Dup\n",
	}
	data, _ := session.MarshalDMail(fb)
	os.WriteFile(filepath.Join(domain.MailDir(dir, "inbox"), fb.Filename()), data, 0644)
	os.WriteFile(filepath.Join(domain.MailDir(dir, "archive"), fb.Filename()), data, 0644)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := session.MonitorInbox(ctx, dir, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("MonitorInbox: %v", err)
	}

	// Should NOT receive anything (already archived = dedup)
	select {
	case mail := <-ch:
		t.Errorf("unexpected feedback (should be deduped): %s", mail.Name)
	case <-time.After(200 * time.Millisecond):
		// expected: dedup skipped
	}

	// inbox should be cleaned up
	files, _ := session.ListDMail(dir, "inbox")
	if len(files) != 0 {
		t.Errorf("inbox should be empty after dedup cleanup: %d files", len(files))
	}
}

func TestMonitorInbox_DetectsNewFile(t *testing.T) {
	dir := t.TempDir()
	session.EnsureMailDirs(dir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := session.MonitorInbox(ctx, dir, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("MonitorInbox: %v", err)
	}

	// Wait for watcher to be ready
	time.Sleep(50 * time.Millisecond)

	// Write a new feedback d-mail to inbox
	fb := &session.DMail{
		Name:        "feedback-mon-new",
		Kind:        session.DMailDesignFeedback,
		Description: "New feedback via fsnotify",
		Severity:    "high",
		Body:        "# New\n",
	}
	data, _ := session.MarshalDMail(fb)
	os.WriteFile(filepath.Join(domain.MailDir(dir, "inbox"), fb.Filename()), data, 0644)

	// Should receive via fsnotify
	select {
	case mail, ok := <-ch:
		if !ok {
			t.Fatal("channel closed unexpectedly")
		}
		if mail.Name != "feedback-mon-new" {
			t.Errorf("name: got %s, want feedback-mon-new", mail.Name)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for fsnotify notification")
	}
}

func TestMonitorInbox_StopsOnCancel(t *testing.T) {
	dir := t.TempDir()
	session.EnsureMailDirs(dir)

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := session.MonitorInbox(ctx, dir, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("MonitorInbox: %v", err)
	}

	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed after cancel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for channel close")
	}
}

func TestDrainInboxFeedback_DrainsFeedback(t *testing.T) {
	dir := t.TempDir()
	session.EnsureMailDirs(dir)

	// Place feedback in inbox
	fb := &session.DMail{
		Name:        "feedback-drain-001",
		Kind:        session.DMailDesignFeedback,
		Description: "Drain test",
		Severity:    "high",
		Body:        "# Drain\n",
	}
	data, _ := session.MarshalDMail(fb)
	os.WriteFile(filepath.Join(domain.MailDir(dir, "inbox"), fb.Filename()), data, 0644)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := session.MonitorInbox(ctx, dir, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("MonitorInbox: %v", err)
	}

	feedback := session.DrainInboxFeedback(ch, platform.NewLogger(io.Discard, false))
	if len(feedback) != 1 {
		t.Errorf("expected 1 drained, got %d", len(feedback))
	}
	if feedback[0].Name != "feedback-drain-001" {
		t.Errorf("name: got %s, want feedback-drain-001", feedback[0].Name)
	}
}

func TestDrainInboxFeedback_NilChannel(t *testing.T) {
	feedback := session.DrainInboxFeedback(nil, platform.NewLogger(io.Discard, false))
	if feedback != nil {
		t.Errorf("expected nil for nil channel, got %d items", len(feedback))
	}
}

func TestFeedbackCollector_AccumulatesInitialAndLate(t *testing.T) {
	dir := t.TempDir()
	session.EnsureMailDirs(dir)

	// Pre-place feedback in inbox (initial)
	initialFb := &session.DMail{
		Name:        "fb-init-001",
		Kind:        session.DMailDesignFeedback,
		Description: "Initial feedback",
		Severity:    "high",
		Body:        "Initial body.",
	}
	data, _ := session.MarshalDMail(initialFb)
	os.WriteFile(filepath.Join(domain.MailDir(dir, "inbox"), initialFb.Filename()), data, 0644)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := session.MonitorInbox(ctx, dir, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("MonitorInbox: %v", err)
	}

	// Drain initial feedback
	initial := session.DrainInboxFeedback(ch, platform.NewLogger(io.Discard, false))

	// Start collector with initial feedback
	collector := session.CollectFeedback(initial, ch, &port.NopNotifier{}, platform.NewLogger(io.Discard, false))

	// All() should return initial feedback
	all := collector.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 initial item, got %d", len(all))
	}
	if all[0].Name != "fb-init-001" {
		t.Errorf("expected fb-init-001, got %q", all[0].Name)
	}

	// Wait for watcher to be ready
	time.Sleep(50 * time.Millisecond)

	// Write late-arriving feedback
	lateFb := &session.DMail{
		Name:        "fb-late-001",
		Kind:        session.DMailDesignFeedback,
		Description: "Late feedback",
		Severity:    "high",
		Body:        "Late body.",
	}
	lateData, _ := session.MarshalDMail(lateFb)
	os.WriteFile(filepath.Join(domain.MailDir(dir, "inbox"), lateFb.Filename()), lateData, 0644)

	// Wait for fsnotify + background goroutine
	time.Sleep(500 * time.Millisecond)

	// All() should now return both initial AND late feedback
	all = collector.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 items (initial + late), got %d", len(all))
	}

	// Verify late feedback was archived
	archivePath := filepath.Join(domain.MailDir(dir, "archive"), lateFb.Filename())
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("late feedback not archived: %v", err)
	}
}

func TestFeedbackCollector_AllIsNonDestructive(t *testing.T) {
	// given: collector with items
	initial := []*session.DMail{
		{Name: "fb-001", Kind: session.DMailDesignFeedback, Description: "Item 1"},
		{Name: "fb-002", Kind: session.DMailDesignFeedback, Description: "Item 2"},
	}
	collector := session.CollectFeedback(initial, nil, &port.NopNotifier{}, platform.NewLogger(io.Discard, false))

	// when: call All() multiple times
	first := collector.All()
	second := collector.All()

	// then: same data each time (non-destructive)
	if len(first) != 2 {
		t.Fatalf("first All(): expected 2, got %d", len(first))
	}
	if len(second) != 2 {
		t.Fatalf("second All(): expected 2, got %d", len(second))
	}
}

func TestFeedbackCollector_NilChannel(t *testing.T) {
	// given: nil channel
	collector := session.CollectFeedback(nil, nil, &port.NopNotifier{}, platform.NewLogger(io.Discard, false))

	// then: All() returns nil
	if all := collector.All(); all != nil {
		t.Errorf("expected nil for nil initial + nil channel, got %d items", len(all))
	}
}

func TestFeedbackCollector_NilInitialWithChannel(t *testing.T) {
	// given: nil initial but channel that will receive items
	ch := make(chan *session.DMail, 1)
	collector := session.CollectFeedback(nil, ch, &port.NopNotifier{}, platform.NewLogger(io.Discard, false))

	// when: send one item
	ch <- &session.DMail{Name: "fb-late", Kind: session.DMailDesignFeedback, Description: "Late only"}
	close(ch)

	// Wait for goroutine
	time.Sleep(50 * time.Millisecond)

	// then: All() returns the late item
	all := collector.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 item, got %d", len(all))
	}
	if all[0].Name != "fb-late" {
		t.Errorf("expected fb-late, got %q", all[0].Name)
	}
}

func TestFormatFeedbackForPrompt_Empty(t *testing.T) {
	got := session.FormatFeedbackForPrompt(nil)
	if got != "" {
		t.Errorf("expected empty string for nil, got %q", got)
	}
	got = session.FormatFeedbackForPrompt([]*session.DMail{})
	if got != "" {
		t.Errorf("expected empty string for empty slice, got %q", got)
	}
}

func TestFormatFeedbackForPrompt_Single(t *testing.T) {
	feedback := []*session.DMail{
		{Name: "fb-001", Kind: session.DMailDesignFeedback, Description: "Architecture drift", Severity: "high", Body: "Auth module drift detected."},
	}
	got := session.FormatFeedbackForPrompt(feedback)
	if !strings.Contains(got, "### [HIGH]") {
		t.Error("expected HIGH severity header")
	}
	if !strings.Contains(got, "fb-001") {
		t.Error("expected feedback name")
	}
	if !strings.Contains(got, "Architecture drift") {
		t.Error("expected description")
	}
	if !strings.Contains(got, "Auth module drift detected.") {
		t.Error("expected body content")
	}
}

func TestFormatFeedbackForPrompt_Multiple(t *testing.T) {
	feedback := []*session.DMail{
		{Name: "fb-001", Kind: session.DMailDesignFeedback, Description: "High severity item", Severity: "high", Body: "Details here."},
		{Name: "fb-002", Kind: session.DMailDesignFeedback, Description: "Normal item", Severity: "", Body: "Normal details."},
	}
	got := session.FormatFeedbackForPrompt(feedback)
	if !strings.Contains(got, "### [HIGH]") {
		t.Error("expected HIGH header for first item")
	}
	if !strings.Contains(got, "### fb-002") {
		t.Error("expected normal header for second item")
	}
	if !strings.Contains(got, "Normal details.") {
		t.Error("expected second body")
	}
}

func TestFormatFeedbackForPrompt_NoBody(t *testing.T) {
	feedback := []*session.DMail{
		{Name: "fb-003", Kind: session.DMailDesignFeedback, Description: "No body feedback", Severity: "high"},
	}
	got := session.FormatFeedbackForPrompt(feedback)
	if !strings.Contains(got, "No body feedback") {
		t.Error("expected description even without body")
	}
}

func TestFeedbackCollector_FeedbackOnly_ExcludesConvergence(t *testing.T) {
	// given: collector with mixed feedback + convergence d-mails
	initial := []*session.DMail{
		{Name: "fb-001", Kind: session.DMailDesignFeedback, Description: "Architecture drift"},
		{Name: "conv-001", Kind: session.DMailConvergence, Description: "Convergence signal"},
		{Name: "fb-002", Kind: session.DMailDesignFeedback, Description: "Naming convention"},
	}
	c := session.CollectFeedback(initial, nil, &port.NopNotifier{}, platform.NewLogger(io.Discard, false))

	// when
	feedbackOnly := c.FeedbackOnly()

	// then: only feedback d-mails, no convergence
	if len(feedbackOnly) != 2 {
		t.Fatalf("expected 2 feedback d-mails, got %d", len(feedbackOnly))
	}
	for _, m := range feedbackOnly {
		if m.Kind == session.DMailConvergence {
			t.Errorf("FeedbackOnly should not contain convergence d-mail: %s", m.Name)
		}
	}
}

// --- Receiving test group ---

func TestReceiveDMail_MalformedContent(t *testing.T) {
	// given: a file in inbox that is not valid d-mail format
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	inboxPath := filepath.Join(domain.MailDir(dir, "inbox"), "bad.md")
	os.WriteFile(inboxPath, []byte("not a d-mail"), 0644)

	// when
	_, err := session.ReceiveDMail(dir, "bad.md")

	// then: parse error, not a file-not-found error
	if err == nil {
		t.Fatal("expected error for malformed content")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %v", err)
	}
	// inbox file should still exist (parse failed before move)
	if _, statErr := os.Stat(inboxPath); os.IsNotExist(statErr) {
		t.Error("inbox file should remain after parse failure")
	}
}

func TestReceiveDMail_PreservesAllFields(t *testing.T) {
	// given: a d-mail with all fields populated
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	original := &session.DMail{
		Name:        "feedback-full",
		Kind:        session.DMailDesignFeedback,
		Description: "Full field test",
		Issues:      []string{"MY-100", "MY-101"},
		Severity:    "high",
		Metadata:    map[string]string{"source": "phonewave", "version": "1.0"},
		Body:        "# Full\n\nAll fields present.\n",
	}
	data, _ := session.MarshalDMail(original)
	os.WriteFile(filepath.Join(domain.MailDir(dir, "inbox"), original.Filename()), data, 0644)

	// when
	received, err := session.ReceiveDMail(dir, original.Filename())

	// then: all fields preserved
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if received.Name != "feedback-full" {
		t.Errorf("name: got %q", received.Name)
	}
	if received.Kind != session.DMailDesignFeedback {
		t.Errorf("kind: got %q", received.Kind)
	}
	if received.Description != "Full field test" {
		t.Errorf("description: got %q", received.Description)
	}
	if len(received.Issues) != 2 || received.Issues[0] != "MY-100" || received.Issues[1] != "MY-101" {
		t.Errorf("issues: got %v", received.Issues)
	}
	if received.Severity != "high" {
		t.Errorf("severity: got %q", received.Severity)
	}
	if received.Metadata["source"] != "phonewave" || received.Metadata["version"] != "1.0" {
		t.Errorf("metadata: got %v", received.Metadata)
	}
	if received.Body != original.Body {
		t.Errorf("body: got %q, want %q", received.Body, original.Body)
	}
}

func TestMonitorInbox_MultipleFeedbackInitialDrain(t *testing.T) {
	// given: 3 feedback files in inbox before monitor starts
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	for _, name := range []string{"feedback-a", "feedback-b", "feedback-c"} {
		fb := &session.DMail{
			Name:        name,
			Kind:        session.DMailDesignFeedback,
			Description: name + " desc",
		}
		data, _ := session.MarshalDMail(fb)
		os.WriteFile(filepath.Join(domain.MailDir(dir, "inbox"), fb.Filename()), data, 0644)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// when
	ch, err := session.MonitorInbox(ctx, dir, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("MonitorInbox: %v", err)
	}

	// then: all 3 drained
	feedback := session.DrainInboxFeedback(ch, platform.NewLogger(io.Discard, false))
	if len(feedback) != 3 {
		t.Errorf("expected 3 feedback, got %d", len(feedback))
	}
	// all archived
	archiveFiles, _ := session.ListDMail(dir, "archive")
	if len(archiveFiles) != 3 {
		t.Errorf("expected 3 archived files, got %d", len(archiveFiles))
	}
	// inbox empty
	inboxFiles, _ := session.ListDMail(dir, "inbox")
	if len(inboxFiles) != 0 {
		t.Errorf("expected empty inbox, got %d files", len(inboxFiles))
	}
}

func TestMonitorInbox_MixedKindsInitialDrain(t *testing.T) {
	// given: mixed feedback + specification + report in inbox
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	mails := []*session.DMail{
		{Name: "feedback-mix-1", Kind: session.DMailDesignFeedback, Description: "feedback 1"},
		{Name: "spec-mix-1", Kind: session.DMailSpecification, Description: "spec 1"},
		{Name: "feedback-mix-2", Kind: session.DMailDesignFeedback, Description: "feedback 2"},
		{Name: "report-mix-1", Kind: session.DMailReport, Description: "report 1"},
	}
	for _, m := range mails {
		data, _ := session.MarshalDMail(m)
		os.WriteFile(filepath.Join(domain.MailDir(dir, "inbox"), m.Filename()), data, 0644)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// when
	ch, err := session.MonitorInbox(ctx, dir, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("MonitorInbox: %v", err)
	}

	// then: 3 items come through channel (2 feedback + 1 report; spec excluded)
	feedback := session.DrainInboxFeedback(ch, platform.NewLogger(io.Discard, false))
	if len(feedback) != 3 {
		t.Fatalf("expected 3 items (2 feedback + 1 report), got %d", len(feedback))
	}
	names := make(map[string]bool)
	for _, fb := range feedback {
		names[fb.Name] = true
	}
	if !names["feedback-mix-1"] || !names["feedback-mix-2"] || !names["report-mix-1"] {
		t.Errorf("expected feedback-mix-1, feedback-mix-2, report-mix-1, got %v", names)
	}
	// all 4 should be archived (spec is received but not channeled)
	archiveFiles, _ := session.ListDMail(dir, "archive")
	if len(archiveFiles) != 4 {
		t.Errorf("expected 4 archived files, got %d", len(archiveFiles))
	}
}

func TestDrainInboxFeedback_MultipleFeedback(t *testing.T) {
	// given: buffered channel with 3 items
	ch := make(chan *session.DMail, 3)
	ch <- &session.DMail{Name: "fb-1", Kind: session.DMailDesignFeedback, Description: "first", Severity: "high"}
	ch <- &session.DMail{Name: "fb-2", Kind: session.DMailDesignFeedback, Description: "second"}
	ch <- &session.DMail{Name: "fb-3", Kind: session.DMailDesignFeedback, Description: "third", Severity: "high"}

	// when
	feedback := session.DrainInboxFeedback(ch, platform.NewLogger(io.Discard, false))

	// then
	if len(feedback) != 3 {
		t.Fatalf("expected 3, got %d", len(feedback))
	}
	if feedback[0].Name != "fb-1" || feedback[1].Name != "fb-2" || feedback[2].Name != "fb-3" {
		t.Errorf("order: got %s, %s, %s", feedback[0].Name, feedback[1].Name, feedback[2].Name)
	}
}

func TestDrainInboxFeedback_ClosedChannel(t *testing.T) {
	// given: a closed channel with 1 buffered item
	ch := make(chan *session.DMail, 1)
	ch <- &session.DMail{Name: "fb-closed", Kind: session.DMailDesignFeedback, Description: "before close"}
	close(ch)

	// when
	feedback := session.DrainInboxFeedback(ch, platform.NewLogger(io.Discard, false))

	// then: should drain the buffered item
	if len(feedback) != 1 {
		t.Fatalf("expected 1, got %d", len(feedback))
	}
	if feedback[0].Name != "fb-closed" {
		t.Errorf("name: got %q", feedback[0].Name)
	}
}

func TestDrainInboxFeedback_EmptyChannel(t *testing.T) {
	// given: an empty buffered channel (not nil, not closed)
	ch := make(chan *session.DMail, 5)

	// when
	feedback := session.DrainInboxFeedback(ch, platform.NewLogger(io.Discard, false))

	// then: returns nil (no feedback)
	if feedback != nil {
		t.Errorf("expected nil for empty channel, got %d items", len(feedback))
	}
}

// --- Parse edge cases ---

func TestParseDMail_FrontmatterOnly_NoBody(t *testing.T) {
	// given: valid frontmatter with no body content
	mail := &session.DMail{
		Name:        "no-body-mail",
		Kind:        session.DMailDesignFeedback,
		Description: "No body content",
	}
	data, err := session.MarshalDMail(mail)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// when
	parsed, err := session.ParseDMail(data)

	// then
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Name != "no-body-mail" {
		t.Errorf("name: got %q", parsed.Name)
	}
	if parsed.Body != "" {
		t.Errorf("body: got %q, want empty", parsed.Body)
	}
}

func TestParseDMail_MissingClosingDelimiter(t *testing.T) {
	// given: opening --- but no closing ---
	data := []byte("---\nname: test\nkind: feedback\n")

	// when
	_, err := session.ParseDMail(data)

	// then
	if err == nil {
		t.Fatal("expected error for missing closing delimiter")
	}
	if !strings.Contains(err.Error(), "closing frontmatter") {
		t.Errorf("error should mention closing delimiter: %v", err)
	}
}

func TestMarshalDMail_NoBody(t *testing.T) {
	// given: d-mail with empty body
	mail := &session.DMail{
		Name:        "no-body",
		Kind:        session.DMailReport,
		Description: "Report without body",
	}

	// when
	data, err := session.MarshalDMail(mail)

	// then
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	content := string(data)
	// should end with closing --- and newline, no extra blank line
	if !strings.HasSuffix(content, "---\n") {
		t.Errorf("expected to end with ---\\n, got suffix: %q", content[len(content)-20:])
	}
}

// --- Sending test group ---

func TestComposeDMail_NilMail(t *testing.T) {
	// given
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)

	// when
	err := session.ComposeDMail(context.Background(), store, nil)

	// then
	if err == nil {
		t.Error("expected error for nil mail")
	}
}

func TestComposeReport_WithErrors(t *testing.T) {
	// given: apply result with errors
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "api",
		Title:       "API hardening",
		Actions: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-60", Description: "Rate limiting"},
			{Type: "add_dependency", IssueID: "MY-61", Description: "Auth dependency"},
		},
	}
	result := &domain.WaveApplyResult{
		WaveID:  "w1",
		Applied: 1,
		Errors:  []string{"Failed to update MY-61: permission denied"},
	}

	// when
	err := session.ComposeReport(context.Background(), store, wave, result)

	// then
	if err != nil {
		t.Fatalf("ComposeReport: %v", err)
	}
	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), "report-api-w1.md")
	data, _ := os.ReadFile(outboxPath)
	mail, _ := session.ParseDMail(data)
	if !strings.Contains(mail.Body, "## Errors") {
		t.Error("body should contain Errors section")
	}
	if !strings.Contains(mail.Body, "permission denied") {
		t.Error("body should contain error message")
	}
}

func TestComposeReport_WithErrorsAndRipples(t *testing.T) {
	// given: apply result with both errors and ripples
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w3",
		ClusterName: "infra",
		Title:       "Infra setup",
		Actions: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-70", Description: "Docker setup"},
		},
	}
	result := &domain.WaveApplyResult{
		WaveID:  "w3",
		Applied: 1,
		Errors:  []string{"Partial failure on sub-issue creation"},
		Ripples: []domain.Ripple{
			{ClusterName: "api", Description: "API now requires Docker"},
			{ClusterName: "frontend", Description: "Frontend builds affected"},
		},
	}

	// when
	err := session.ComposeReport(context.Background(), store, wave, result)

	// then
	if err != nil {
		t.Fatalf("ComposeReport: %v", err)
	}
	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), "report-infra-w3.md")
	data, _ := os.ReadFile(outboxPath)
	mail, _ := session.ParseDMail(data)
	if !strings.Contains(mail.Body, "## Errors") {
		t.Error("body should contain Errors section")
	}
	if !strings.Contains(mail.Body, "## Ripple Effects") {
		t.Error("body should contain Ripple Effects section")
	}
	if !strings.Contains(mail.Body, "API now requires Docker") {
		t.Error("body should contain ripple description")
	}
}

func TestComposeSpecification_WaveWithDescription(t *testing.T) {
	// given: wave with non-empty description
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "db",
		Title:       "Database Migrations",
		Description: "Critical migration wave for schema changes.",
		Actions: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-80", Description: "Migration script"},
		},
	}

	// when
	err := session.ComposeSpecification(context.Background(), store, wave)

	// then
	if err != nil {
		t.Fatalf("ComposeSpecification: %v", err)
	}
	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), "spec-db-w1.md")
	data, _ := os.ReadFile(outboxPath)
	mail, _ := session.ParseDMail(data)
	if !strings.Contains(mail.Body, "Critical migration wave") {
		t.Error("body should contain wave description")
	}
	if !strings.Contains(mail.Body, "# Database Migrations") {
		t.Error("body should contain wave title as heading")
	}
}

func TestComposeSpecification_EmptyActions(t *testing.T) {
	// given: wave with no actions (edge case)
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "misc",
		Title:       "Empty wave",
		Actions:     []domain.WaveAction{},
	}

	// when
	err := session.ComposeSpecification(context.Background(), store, wave)

	// then: should succeed (empty actions section)
	if err != nil {
		t.Fatalf("ComposeSpecification: %v", err)
	}
	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), "spec-misc-w1.md")
	data, _ := os.ReadFile(outboxPath)
	mail, _ := session.ParseDMail(data)
	if !strings.Contains(mail.Body, "## Actions") {
		t.Error("body should still have Actions heading")
	}
}

func TestComposeSpecification_IssueDedup(t *testing.T) {
	// given: wave with duplicate issue IDs across actions
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "auth",
		Title:       "Auth DoD",
		Actions: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-42", Description: "First DoD"},
			{Type: "add_dependency", IssueID: "MY-42", Description: "Dependency"},
			{Type: "add_dod", IssueID: "MY-43", Description: "Second DoD"},
		},
	}

	// when
	err := session.ComposeSpecification(context.Background(), store, wave)

	// then: issues list should be deduplicated
	if err != nil {
		t.Fatalf("ComposeSpecification: %v", err)
	}
	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), "spec-auth-w1.md")
	data, _ := os.ReadFile(outboxPath)
	mail, _ := session.ParseDMail(data)
	if len(mail.Issues) != 2 {
		t.Errorf("expected 2 unique issues, got %d: %v", len(mail.Issues), mail.Issues)
	}
}

func TestComposeReport_IssuesSorted(t *testing.T) {
	// given: wave with unsorted issue IDs
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "sort",
		Title:       "Sort test",
		Actions: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-99", Description: "Last"},
			{Type: "add_dod", IssueID: "MY-10", Description: "First"},
			{Type: "add_dod", IssueID: "MY-50", Description: "Middle"},
		},
	}
	result := &domain.WaveApplyResult{WaveID: "w1", Applied: 3}

	// when
	err := session.ComposeReport(context.Background(), store, wave, result)

	// then: issues are sorted
	if err != nil {
		t.Fatalf("ComposeReport: %v", err)
	}
	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), "report-sort-w1.md")
	data, _ := os.ReadFile(outboxPath)
	mail, _ := session.ParseDMail(data)
	if len(mail.Issues) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(mail.Issues))
	}
	if mail.Issues[0] != "MY-10" || mail.Issues[1] != "MY-50" || mail.Issues[2] != "MY-99" {
		t.Errorf("issues not sorted: %v", mail.Issues)
	}
}

// --- Helper function tests ---

func TestWaveIssueIDs_Dedup(t *testing.T) {
	// given: actions with duplicate issue IDs
	wave := domain.Wave{
		Actions: []domain.WaveAction{
			{IssueID: "MY-1"},
			{IssueID: "MY-2"},
			{IssueID: "MY-1"},
			{IssueID: "MY-3"},
			{IssueID: "MY-2"},
		},
	}

	// when
	ids := session.WaveIssueIDs(wave)

	// then
	if len(ids) != 3 {
		t.Fatalf("expected 3 unique IDs, got %d: %v", len(ids), ids)
	}
}

func TestWaveIssueIDs_Empty(t *testing.T) {
	// given: wave with no actions
	wave := domain.Wave{Actions: []domain.WaveAction{}}

	// when
	ids := session.WaveIssueIDs(wave)

	// then
	if len(ids) != 0 {
		t.Errorf("expected 0 IDs, got %d", len(ids))
	}
}

func TestWaveIssueIDs_SkipsEmptyID(t *testing.T) {
	// given: actions where some have empty issue IDs
	wave := domain.Wave{
		Actions: []domain.WaveAction{
			{IssueID: "MY-1"},
			{IssueID: ""},
			{IssueID: "MY-2"},
		},
	}

	// when
	ids := session.WaveIssueIDs(wave)

	// then: empty IDs are excluded
	if len(ids) != 2 {
		t.Errorf("expected 2 IDs (empty excluded), got %d: %v", len(ids), ids)
	}
}

func TestWaveIssueIDs_Sorted(t *testing.T) {
	// given: unsorted issue IDs
	wave := domain.Wave{
		Actions: []domain.WaveAction{
			{IssueID: "MY-99"},
			{IssueID: "MY-10"},
			{IssueID: "MY-50"},
		},
	}

	// when
	ids := session.WaveIssueIDs(wave)

	// then: sorted
	if ids[0] != "MY-10" || ids[1] != "MY-50" || ids[2] != "MY-99" {
		t.Errorf("not sorted: %v", ids)
	}
}

func TestSpecificationBody_Format(t *testing.T) {
	// given
	wave := domain.Wave{
		Title:       "Auth Wave",
		Description: "Setting up authentication.",
		Actions: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-1", Description: "Add unit tests"},
			{Type: "add_dependency", IssueID: "MY-2", Description: "Depends on auth"},
		},
	}

	// when
	body := session.SpecificationBody(wave)

	// then
	if !strings.HasPrefix(body, "# Auth Wave\n") {
		t.Error("body should start with title heading")
	}
	if !strings.Contains(body, "Setting up authentication.") {
		t.Error("body should contain description")
	}
	if !strings.Contains(body, "## Actions") {
		t.Error("body should contain Actions heading")
	}
	if !strings.Contains(body, "- [add_dod] MY-1: Add unit tests") {
		t.Error("body should contain formatted action")
	}
	if !strings.Contains(body, "- [add_dependency] MY-2: Depends on auth") {
		t.Error("body should contain second action")
	}
}

func TestSpecificationBody_NoDescription(t *testing.T) {
	// given: wave without description
	wave := domain.Wave{
		Title: "Minimal Wave",
		Actions: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-1", Description: "DoD item"},
		},
	}

	// when
	body := session.SpecificationBody(wave)

	// then: no double blank line between title and Actions
	if strings.Contains(body, "# Minimal Wave\n\n\n") {
		t.Error("should not have extra blank line when description is empty")
	}
	if !strings.Contains(body, "## Actions") {
		t.Error("body should contain Actions heading")
	}
}

func TestReportBody_Format(t *testing.T) {
	// given
	wave := domain.Wave{Title: "Deploy Wave"}
	result := &domain.WaveApplyResult{
		Applied: 3,
		Errors:  []string{"timeout on MY-5"},
		Ripples: []domain.Ripple{
			{ClusterName: "api", Description: "API needs rebuild"},
		},
	}

	// when
	body := session.ReportBody(wave, result)

	// then
	if !strings.HasPrefix(body, "# Wave Completed: Deploy Wave\n") {
		t.Error("body should start with completion heading")
	}
	if !strings.Contains(body, "Applied 3 action(s).") {
		t.Error("body should contain applied count")
	}
	if !strings.Contains(body, "## Errors") {
		t.Error("body should contain Errors section")
	}
	if !strings.Contains(body, "timeout on MY-5") {
		t.Error("body should contain error detail")
	}
	if !strings.Contains(body, "## Ripple Effects") {
		t.Error("body should contain Ripple Effects section")
	}
	if !strings.Contains(body, "[api] API needs rebuild") {
		t.Error("body should contain ripple detail")
	}
}

func TestReportBody_NoErrorsNoRipples(t *testing.T) {
	// given: clean apply
	wave := domain.Wave{Title: "Clean Wave"}
	result := &domain.WaveApplyResult{Applied: 2}

	// when
	body := session.ReportBody(wave, result)

	// then: no Errors or Ripple sections
	if strings.Contains(body, "## Errors") {
		t.Error("should not have Errors section when no errors")
	}
	if strings.Contains(body, "## Ripple Effects") {
		t.Error("should not have Ripple section when no ripples")
	}
}

func TestDMailName_EmptyWaveKey(t *testing.T) {
	got := session.DMailName("spec", "")
	if got != "spec-" {
		t.Errorf("got %q, want %q", got, "spec-")
	}
}

func TestDMailName_MultipleColons(t *testing.T) {
	got := session.DMailName("report", "ns:cluster:w1")
	if got != "report-ns-cluster-w1" {
		t.Errorf("got %q, want %q", got, "report-ns-cluster-w1")
	}
}

func TestDMailName_TrailingSpecialChars(t *testing.T) {
	// given: wave key ending with special characters
	got := session.DMailName("spec", "auth:w1!!!")

	// then: trailing underscores should be trimmed
	if strings.HasSuffix(got, "_") {
		t.Errorf("trailing underscores not trimmed: %q", got)
	}
}

func TestCollectFeedback_ConvergenceNotification(t *testing.T) {
	// given: channel that will receive a convergence d-mail
	ch := make(chan *session.DMail, 1)
	var notifyCalled atomic.Bool
	notifier := &testNotifier{onNotify: func(title, message string) {
		notifyCalled.Store(true)
	}}
	collector := session.CollectFeedback(nil, ch, notifier, platform.NewLogger(io.Discard, false))

	// when: convergence arrives
	ch <- &session.DMail{Name: "conv-late-001", Kind: session.DMailConvergence, Description: "Late convergence"}
	close(ch)
	time.Sleep(100 * time.Millisecond)

	// then: convergence name recorded
	names := collector.ConvergenceNames()
	if len(names) != 1 {
		t.Fatalf("expected 1 convergence name, got %d", len(names))
	}
	if names[0] != "conv-late-001" {
		t.Errorf("expected conv-late-001, got %q", names[0])
	}
	// and: notifier was called
	if !notifyCalled.Load() {
		t.Error("expected notifier to be called for convergence d-mail")
	}
	// and: convergence also present in All()
	all := collector.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 item in All(), got %d", len(all))
	}
}

func TestCollectFeedback_MixedFeedbackAndConvergence(t *testing.T) {
	// given: initial feedback + channel with convergence
	initial := []*session.DMail{
		{Name: "fb-init", Kind: session.DMailDesignFeedback, Description: "initial"},
	}
	ch := make(chan *session.DMail, 1)
	collector := session.CollectFeedback(initial, ch, &port.NopNotifier{}, platform.NewLogger(io.Discard, false))

	ch <- &session.DMail{Name: "conv-mix-001", Kind: session.DMailConvergence, Description: "convergence"}
	close(ch)
	time.Sleep(100 * time.Millisecond)

	// then: All() returns both
	all := collector.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 items, got %d", len(all))
	}
	// convergence names only contains convergence
	names := collector.ConvergenceNames()
	if len(names) != 1 {
		t.Fatalf("expected 1 convergence name, got %d", len(names))
	}
}

// testNotifier captures Notify calls for testing.
type testNotifier struct {
	onNotify func(title, message string)
}

func (n *testNotifier) Notify(_ context.Context, title, message string) error {
	if n.onNotify != nil {
		n.onNotify(title, message)
	}
	return nil
}

func TestCollectFeedback_HangingNotifierDoesNotBlockDrain(t *testing.T) {
	// given: a notifier that blocks for a long time + channel with convergence then feedback.
	// The collector must drain both d-mails promptly even if the notifier hangs.
	ch := make(chan *session.DMail, 2)
	hangingNotifier := &testNotifier{onNotify: func(title, message string) {
		time.Sleep(10 * time.Second) // simulate hung notifier
	}}
	collector := session.CollectFeedback(nil, ch, hangingNotifier, platform.NewLogger(io.Discard, false))

	// when: send convergence (triggers hanging notify) then feedback
	ch <- &session.DMail{Name: "conv-hang", Kind: session.DMailConvergence, Description: "convergence"}
	ch <- &session.DMail{Name: "fb-after", Kind: session.DMailDesignFeedback, Description: "feedback after convergence"}
	close(ch)

	// then: both d-mails should be collected within a reasonable time,
	// not blocked by the hanging notifier
	deadline := time.After(2 * time.Second)
	for {
		all := collector.All()
		if len(all) >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("collector blocked: only %d items collected, expected 2 (notifier hanging)", len(collector.All()))
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestMonitorInbox_DeliversConvergence(t *testing.T) {
	// given: convergence d-mail in inbox
	dir := t.TempDir()
	session.EnsureMailDirs(dir)

	conv := &session.DMail{
		Name:        "convergence-mon-001",
		Kind:        session.DMailConvergence,
		Description: "Convergence signal from phonewave",
	}
	data, _ := session.MarshalDMail(conv)
	os.WriteFile(filepath.Join(domain.MailDir(dir, "inbox"), conv.Filename()), data, 0644)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := session.MonitorInbox(ctx, dir, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("MonitorInbox: %v", err)
	}

	// then: convergence should be delivered through channel
	select {
	case mail, ok := <-ch:
		if !ok {
			t.Fatal("channel closed unexpectedly")
		}
		if mail.Name != "convergence-mon-001" {
			t.Errorf("name: got %s, want convergence-mon-001", mail.Name)
		}
		if mail.Kind != session.DMailConvergence {
			t.Errorf("kind: got %s, want convergence", mail.Kind)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for convergence d-mail")
	}
}

func TestMonitorInbox_MixedFeedbackAndConvergence(t *testing.T) {
	// given: both feedback and convergence d-mails in inbox
	dir := t.TempDir()
	session.EnsureMailDirs(dir)

	mails := []*session.DMail{
		{Name: "feedback-mix-conv-1", Kind: session.DMailDesignFeedback, Description: "feedback"},
		{Name: "convergence-mix-1", Kind: session.DMailConvergence, Description: "convergence"},
		{Name: "spec-mix-conv-1", Kind: session.DMailSpecification, Description: "spec"},
	}
	for _, m := range mails {
		data, _ := session.MarshalDMail(m)
		os.WriteFile(filepath.Join(domain.MailDir(dir, "inbox"), m.Filename()), data, 0644)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := session.MonitorInbox(ctx, dir, platform.NewLogger(io.Discard, false))
	if err != nil {
		t.Fatalf("MonitorInbox: %v", err)
	}

	// then: 2 d-mails delivered (feedback + convergence), spec excluded
	drained := session.DrainInboxFeedback(ch, platform.NewLogger(io.Discard, false))
	if len(drained) != 2 {
		t.Fatalf("expected 2 (feedback + convergence), got %d", len(drained))
	}
	names := make(map[string]bool)
	for _, m := range drained {
		names[m.Name] = true
	}
	if !names["feedback-mix-conv-1"] {
		t.Error("expected feedback-mix-conv-1")
	}
	if !names["convergence-mix-1"] {
		t.Error("expected convergence-mix-1")
	}

	// all 3 should be archived
	archiveFiles, _ := session.ListDMail(dir, "archive")
	if len(archiveFiles) != 3 {
		t.Errorf("expected 3 archived, got %d", len(archiveFiles))
	}
}

func TestFeedbackCollector_ReportsOnly(t *testing.T) {
	// given: collector with mixed feedback, report, and convergence d-mails
	initial := []*session.DMail{
		{Name: "fb-001", Kind: session.DMailDesignFeedback, Description: "Feedback item"},
		{Name: "rp-001", Kind: session.DMailReport, Description: "Report from amadeus"},
		{Name: "cv-001", Kind: session.DMailConvergence, Description: "Convergence"},
		{Name: "rp-002", Kind: session.DMailReport, Description: "Another report"},
	}
	c := session.CollectFeedback(initial, nil, nil, nil)

	// when
	reports := c.ReportsOnly()

	// then
	if len(reports) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(reports))
	}
	if reports[0].Name != "rp-001" {
		t.Errorf("expected rp-001, got %q", reports[0].Name)
	}
	if reports[1].Name != "rp-002" {
		t.Errorf("expected rp-002, got %q", reports[1].Name)
	}
}

func TestFormatReportsForPrompt_Empty(t *testing.T) {
	// when
	got := session.FormatReportsForPrompt(nil)

	// then
	if got != "" {
		t.Errorf("expected empty string for nil, got %q", got)
	}
}

func TestFormatReportsForPrompt_Single(t *testing.T) {
	// given
	reports := []*session.DMail{
		{Name: "rp-001", Kind: session.DMailReport, Description: "Drift detected in auth module", Body: "Details of the drift."},
	}

	// when
	got := session.FormatReportsForPrompt(reports)

	// then
	if !strings.Contains(got, "rp-001") {
		t.Error("expected report name")
	}
	if !strings.Contains(got, "Drift detected") {
		t.Error("expected description")
	}
	if !strings.Contains(got, "Details of the drift.") {
		t.Error("expected body")
	}
}

func TestDMailIdempotencyKey_Deterministic(t *testing.T) {
	// given
	mail := &session.DMail{
		Name:        "spec-001",
		Kind:        session.DMailSpecification,
		Description: "test spec",
		Body:        "body content\n",
	}

	// when
	key1 := session.DMailIdempotencyKey(mail)
	key2 := session.DMailIdempotencyKey(mail)

	// then
	if key1 != key2 {
		t.Errorf("not deterministic: %q != %q", key1, key2)
	}
	if len(key1) != 64 {
		t.Errorf("expected 64-char hex, got %d: %q", len(key1), key1)
	}
}

func TestMarshalDMail_IdempotencyKey(t *testing.T) {
	// given
	mail := &session.DMail{
		Name:        "spec-001",
		Kind:        session.DMailSpecification,
		Description: "test spec",
		Body:        "body content\n",
	}

	// when
	data, err := session.MarshalDMail(mail)
	if err != nil {
		t.Fatalf("MarshalDMail: %v", err)
	}

	// then
	parsed, err := session.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail: %v", err)
	}
	key, ok := parsed.Metadata["idempotency_key"]
	if !ok {
		t.Fatal("expected idempotency_key in metadata")
	}
	expected := session.DMailIdempotencyKey(mail)
	if key != expected {
		t.Errorf("got %q, want %q", key, expected)
	}
}

func TestMarshalDMail_IdempotencyKey_DoesNotMutateOriginal(t *testing.T) {
	// given: DMail with no metadata
	mail := &session.DMail{
		Name:        "spec-001",
		Kind:        session.DMailSpecification,
		Description: "test",
	}

	// when
	_, err := session.MarshalDMail(mail)
	if err != nil {
		t.Fatalf("MarshalDMail: %v", err)
	}

	// then: original metadata should not be modified
	if mail.Metadata != nil {
		t.Errorf("original metadata mutated: %v", mail.Metadata)
	}
}

// --- O2: sightjack → amadeus feedback D-Mail ---

func TestComposeFeedback_StagesInOutbox(t *testing.T) {
	// given: a completed wave with apply result
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "auth",
		Title:       "Auth hardening",
		Actions: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-42", Description: "Token bucket"},
			{Type: "add_dependency", IssueID: "MY-43", Description: "Auth dep"},
		},
	}
	result := &domain.WaveApplyResult{
		WaveID:  "w1",
		Applied: 2,
		Ripples: []domain.Ripple{
			{ClusterName: "api", Description: "Rate limiting affects API cluster"},
		},
	}

	// when
	err := session.ComposeFeedback(context.Background(), store, wave, result)

	// then: no error
	if err != nil {
		t.Fatalf("ComposeFeedback: %v", err)
	}

	// and: outbox file exists and is parseable
	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), "feedback-auth-w1.md")
	data, readErr := os.ReadFile(outboxPath)
	if readErr != nil {
		t.Fatalf("outbox file missing: %v", readErr)
	}
	mail, parseErr := session.ParseDMail(data)
	if parseErr != nil {
		t.Fatalf("parse outbox: %v", parseErr)
	}

	// and: d-mail fields are correct
	if mail.Kind != session.DMailReport {
		t.Errorf("kind: got %q, want %q", mail.Kind, session.DMailReport)
	}
	if mail.Name != "feedback-auth-w1" {
		t.Errorf("name: got %q, want %q", mail.Name, "feedback-auth-w1")
	}
	if mail.SchemaVersion != "1" {
		t.Errorf("schema version: got %q, want %q", mail.SchemaVersion, "1")
	}
	if len(mail.Issues) != 2 {
		t.Errorf("expected 2 issues, got %d: %v", len(mail.Issues), mail.Issues)
	}

	// and: archive file also exists
	archivePath := filepath.Join(domain.MailDir(dir, "archive"), "feedback-auth-w1.md")
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Error("expected archive file to exist")
	}
}

func TestComposeFeedback_BodyFormat(t *testing.T) {
	// given: wave with errors and ripples
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w2",
		ClusterName: "infra",
		Title:       "Infra setup wave",
		Actions: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-70", Description: "Docker setup"},
			{Type: "add_dod", IssueID: "MY-71", Description: "CI pipeline"},
		},
	}
	result := &domain.WaveApplyResult{
		WaveID:  "w2",
		Applied: 1,
		Errors:  []string{"Failed to update MY-71: permission denied"},
		Ripples: []domain.Ripple{
			{ClusterName: "api", Description: "API now requires Docker"},
		},
	}

	// when
	err := session.ComposeFeedback(context.Background(), store, wave, result)

	// then
	if err != nil {
		t.Fatalf("ComposeFeedback: %v", err)
	}
	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), "feedback-infra-w2.md")
	data, _ := os.ReadFile(outboxPath)
	mail, _ := session.ParseDMail(data)

	// body contains key information
	if !strings.Contains(mail.Body, "Infra setup wave") {
		t.Error("body should contain wave title")
	}
	if !strings.Contains(mail.Body, "Applied 1 action(s)") {
		t.Error("body should contain applied count")
	}
	if !strings.Contains(mail.Body, "## Errors") {
		t.Error("body should contain Errors section")
	}
	if !strings.Contains(mail.Body, "permission denied") {
		t.Error("body should contain error detail")
	}
	if !strings.Contains(mail.Body, "## Ripple Effects") {
		t.Error("body should contain Ripple Effects section")
	}
	if !strings.Contains(mail.Body, "API now requires Docker") {
		t.Error("body should contain ripple detail")
	}
}

func TestComposeFeedback_NoErrorsNoRipples(t *testing.T) {
	// given: clean apply with no errors or ripples
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w3",
		ClusterName: "db",
		Title:       "Database migrations",
		Actions: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-50", Description: "Schema migration"},
		},
	}
	result := &domain.WaveApplyResult{
		WaveID:  "w3",
		Applied: 1,
	}

	// when
	err := session.ComposeFeedback(context.Background(), store, wave, result)

	// then
	if err != nil {
		t.Fatalf("ComposeFeedback: %v", err)
	}
	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), "feedback-db-w3.md")
	data, _ := os.ReadFile(outboxPath)
	mail, _ := session.ParseDMail(data)

	// body should not contain error or ripple sections
	if strings.Contains(mail.Body, "## Errors") {
		t.Error("should not have Errors section when no errors")
	}
	if strings.Contains(mail.Body, "## Ripple Effects") {
		t.Error("should not have Ripple section when no ripples")
	}
	// but should still have the applied count
	if !strings.Contains(mail.Body, "Applied 1 action(s)") {
		t.Error("body should contain applied count")
	}
}

func TestFeedbackBody_Format(t *testing.T) {
	// given
	wave := domain.Wave{Title: "Feedback Wave"}
	result := &domain.WaveApplyResult{
		Applied: 3,
		Errors:  []string{"timeout on MY-5"},
		Ripples: []domain.Ripple{
			{ClusterName: "api", Description: "API needs rebuild"},
		},
	}

	// when
	body := session.FeedbackBody(wave, result)

	// then
	if !strings.HasPrefix(body, "# Wave Feedback: Feedback Wave\n") {
		t.Errorf("body should start with feedback heading, got: %q", body[:50])
	}
	if !strings.Contains(body, "Applied 3 action(s).") {
		t.Error("body should contain applied count")
	}
	if !strings.Contains(body, "## Errors") {
		t.Error("body should contain Errors section")
	}
	if !strings.Contains(body, "timeout on MY-5") {
		t.Error("body should contain error detail")
	}
	if !strings.Contains(body, "## Ripple Effects") {
		t.Error("body should contain Ripple Effects section")
	}
	if !strings.Contains(body, "[api] API needs rebuild") {
		t.Error("body should contain ripple detail")
	}
}

func TestFeedbackBody_NoErrorsNoRipples(t *testing.T) {
	// given: clean apply
	wave := domain.Wave{Title: "Clean Wave"}
	result := &domain.WaveApplyResult{Applied: 2}

	// when
	body := session.FeedbackBody(wave, result)

	// then: no Errors or Ripple sections
	if strings.Contains(body, "## Errors") {
		t.Error("should not have Errors section when no errors")
	}
	if strings.Contains(body, "## Ripple Effects") {
		t.Error("should not have Ripple section when no ripples")
	}
}

func TestComposeFeedback_IssuesSorted(t *testing.T) {
	// given: wave with unsorted issue IDs
	dir := t.TempDir()
	session.EnsureMailDirs(dir)
	store := testOutboxStore(t, dir)
	wave := domain.Wave{
		ID:          "w1",
		ClusterName: "sort",
		Title:       "Sort test",
		Actions: []domain.WaveAction{
			{Type: "add_dod", IssueID: "MY-99", Description: "Last"},
			{Type: "add_dod", IssueID: "MY-10", Description: "First"},
			{Type: "add_dod", IssueID: "MY-50", Description: "Middle"},
		},
	}
	result := &domain.WaveApplyResult{WaveID: "w1", Applied: 3}

	// when
	err := session.ComposeFeedback(context.Background(), store, wave, result)

	// then: issues are sorted
	if err != nil {
		t.Fatalf("ComposeFeedback: %v", err)
	}
	outboxPath := filepath.Join(domain.MailDir(dir, "outbox"), "feedback-sort-w1.md")
	data, _ := os.ReadFile(outboxPath)
	mail, _ := session.ParseDMail(data)
	if len(mail.Issues) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(mail.Issues))
	}
	if mail.Issues[0] != "MY-10" || mail.Issues[1] != "MY-50" || mail.Issues[2] != "MY-99" {
		t.Errorf("issues not sorted: %v", mail.Issues)
	}
}

func TestValidateDMail_CIResultKind(t *testing.T) {
	// given: a d-mail with ci-result kind
	mail := &session.DMail{
		Name:          "ci-result-pr-123",
		Kind:          session.DMailCIResult,
		Description:   "CI pipeline result for PR #123",
		SchemaVersion: "1",
	}

	// when
	err := session.ValidateDMail(mail)

	// then
	if err != nil {
		t.Errorf("expected ci-result kind to be valid, got: %v", err)
	}
}

func TestDMail_ActionPriorityFields(t *testing.T) {
	// given: a d-mail with action and priority fields set
	original := &session.DMail{
		Name:          "ci-result-roundtrip",
		Kind:          session.DMailCIResult,
		Description:   "CI result with action and priority",
		SchemaVersion: "1",
		Action:        "review",
		Priority:      3,
		Body:          "# CI Result\n\nPipeline passed.\n",
	}

	// when: marshal then parse (round-trip)
	data, err := session.MarshalDMail(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	parsed, err := session.ParseDMail(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// then: action and priority are preserved
	if parsed.Action != "review" {
		t.Errorf("action: got %q, want %q", parsed.Action, "review")
	}
	if parsed.Priority != 3 {
		t.Errorf("priority: got %d, want %d", parsed.Priority, 3)
	}
}

func TestDMail_ActionPriorityOmitEmpty(t *testing.T) {
	// given: a d-mail without action and priority (zero values)
	mail := &session.DMail{
		Name:          "no-action-priority",
		Kind:          session.DMailDesignFeedback,
		Description:   "Feedback without action/priority",
		SchemaVersion: "1",
	}

	// when
	data, err := session.MarshalDMail(mail)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	content := string(data)

	// then: action and priority should be omitted from YAML
	if strings.Contains(content, "action:") {
		t.Error("expected action field to be omitted when empty")
	}
	if strings.Contains(content, "priority:") {
		t.Error("expected priority field to be omitted when zero")
	}
}

func TestFeedbackCollector_Snapshot(t *testing.T) {
	// given: collector with channel
	ch := make(chan *session.DMail, 5)
	fc := session.CollectFeedback(nil, ch, nil, &domain.NopLogger{})

	// send two d-mails before snapshot
	ch <- &session.DMail{Kind: session.DMailDesignFeedback, Name: "fb-001"}
	ch <- &session.DMail{Kind: session.DMailReport, Name: "rpt-001"}
	time.Sleep(100 * time.Millisecond)

	// when: take snapshot
	fc.Snapshot()

	// and: send one more after snapshot
	ch <- &session.DMail{Kind: session.DMailSpecification, Name: "spec-001"}
	time.Sleep(100 * time.Millisecond)

	// then: NewSinceSnapshot returns only the post-snapshot item
	newMails := fc.NewSinceSnapshot()
	if len(newMails) != 1 {
		t.Fatalf("got %d new mails, want 1", len(newMails))
	}
	if newMails[0].Name != "spec-001" {
		t.Errorf("got name %q, want spec-001", newMails[0].Name)
	}
}

func TestFeedbackCollector_NewSinceSnapshot_noNew(t *testing.T) {
	// given: collector with channel
	ch := make(chan *session.DMail, 5)
	fc := session.CollectFeedback(nil, ch, nil, &domain.NopLogger{})

	// send one d-mail and snapshot after it
	ch <- &session.DMail{Kind: session.DMailDesignFeedback, Name: "fb-001"}
	time.Sleep(100 * time.Millisecond)

	fc.Snapshot()

	// when: no new d-mails after snapshot
	newMails := fc.NewSinceSnapshot()

	// then
	if len(newMails) != 0 {
		t.Fatalf("got %d new mails, want 0", len(newMails))
	}
}

func TestMarshalDMail_ContextRoundTrip(t *testing.T) {
	// given
	mail := &session.DMail{
		Name:          "spec-context-01",
		Kind:          session.DMailSpecification,
		Description:   "wave with insight context",
		SchemaVersion: "1",
		Context: &domain.InsightContext{
			Insights: []domain.InsightSummary{
				{Source: "sightjack", Summary: "Shibito count reduced to 3"},
				{Source: "amadeus", Summary: "ADR compliance at 95%"},
			},
		},
		Body: "Wave body here.\n",
	}

	// when
	data, err := session.MarshalDMail(mail)
	if err != nil {
		t.Fatalf("MarshalDMail: %v", err)
	}
	parsed, err := session.ParseDMail(data)
	if err != nil {
		t.Fatalf("ParseDMail: %v", err)
	}

	// then
	if parsed.Context == nil {
		t.Fatal("expected non-nil Context after round-trip")
	}
	if len(parsed.Context.Insights) != 2 {
		t.Fatalf("expected 2 insights, got %d", len(parsed.Context.Insights))
	}
	if parsed.Context.Insights[0].Source != "sightjack" {
		t.Errorf("insight[0].Source = %q, want %q", parsed.Context.Insights[0].Source, "sightjack")
	}
	if parsed.Context.Insights[0].Summary != "Shibito count reduced to 3" {
		t.Errorf("insight[0].Summary = %q, want %q", parsed.Context.Insights[0].Summary, "Shibito count reduced to 3")
	}
	if parsed.Context.Insights[1].Source != "amadeus" {
		t.Errorf("insight[1].Source = %q, want %q", parsed.Context.Insights[1].Source, "amadeus")
	}
}

func TestMarshalDMail_NilContextOmitted(t *testing.T) {
	// given — DMail with nil Context
	mail := &session.DMail{
		Name:          "spec-no-context",
		Kind:          session.DMailSpecification,
		Description:   "no context",
		SchemaVersion: "1",
	}

	// when
	data, err := session.MarshalDMail(mail)
	if err != nil {
		t.Fatalf("MarshalDMail: %v", err)
	}

	// then — context should not appear in output
	if strings.Contains(string(data), "context:") {
		t.Error("nil Context should be omitted from marshalled output")
	}
}
