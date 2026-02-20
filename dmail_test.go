package sightjack

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestWatchInbox_DetectsNewFile(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureMailDirs(dir); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := WatchInbox(ctx, dir)
	if err != nil {
		t.Fatalf("watch: %v", err)
	}

	// give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// write a .md file to inbox
	mail := &DMail{
		Name:        "spec-test-1",
		Kind:        DMailSpecification,
		Description: "test",
		Body:        "body\n",
	}
	data, _ := MarshalDMail(mail)
	inboxPath := filepath.Join(MailDir(dir, "inbox"), mail.Filename())
	if err := os.WriteFile(inboxPath, data, 0644); err != nil {
		t.Fatalf("write inbox: %v", err)
	}

	// expect notification
	select {
	case filename := <-ch:
		if filename != "spec-test-1.md" {
			t.Errorf("got %s, want spec-test-1.md", filename)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for inbox notification")
	}
}

func TestWatchInbox_IgnoresNonMD(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureMailDirs(dir); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := WatchInbox(ctx, dir)
	if err != nil {
		t.Fatalf("watch: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// write a non-.md file
	txtPath := filepath.Join(MailDir(dir, "inbox"), "notes.txt")
	os.WriteFile(txtPath, []byte("not a dmail"), 0644)

	// should NOT receive notification
	select {
	case filename := <-ch:
		t.Errorf("unexpected notification for non-.md file: %s", filename)
	case <-time.After(200 * time.Millisecond):
		// expected: no notification
	}
}

func TestDMailName_SanitizesWaveKey(t *testing.T) {
	got := DMailName("spec", "auth:w1")
	if got != "spec-auth-w1" {
		t.Errorf("got %s, want spec-auth-w1", got)
	}
}

func TestDMailName_HandlesSpecialChars(t *testing.T) {
	got := DMailName("report", "My Cluster:wave-2")
	if got != "report-my_cluster-wave-2" {
		t.Errorf("got %s, want report-my_cluster-wave-2", got)
	}
}

func TestComposeSpecification_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	EnsureMailDirs(dir)

	wave := Wave{
		ID:          "w1",
		ClusterName: "auth",
		Title:       "Add DoD to auth issues",
		Description: "First wave for auth cluster",
		Actions: []WaveAction{
			{Type: "add_dod", IssueID: "MY-42", Description: "Token bucket rate limiting"},
			{Type: "add_dependency", IssueID: "MY-42", Description: "Depends on auth middleware"},
			{Type: "add_dod", IssueID: "MY-43", Description: "Session management"},
		},
	}

	err := ComposeSpecification(dir, wave)
	if err != nil {
		t.Fatalf("ComposeSpecification: %v", err)
	}

	// outbox file exists
	outboxPath := filepath.Join(MailDir(dir, "outbox"), "spec-auth-w1.md")
	data, readErr := os.ReadFile(outboxPath)
	if readErr != nil {
		t.Fatalf("outbox file missing: %v", readErr)
	}

	// parse and verify
	mail, parseErr := ParseDMail(data)
	if parseErr != nil {
		t.Fatalf("parse: %v", parseErr)
	}
	if mail.Kind != DMailSpecification {
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
	archivePath := filepath.Join(MailDir(dir, "archive"), "spec-auth-w1.md")
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("archive file missing: %v", err)
	}
}

func TestComposeReport_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	EnsureMailDirs(dir)

	wave := Wave{
		ID:          "w1",
		ClusterName: "auth",
		Title:       "Add DoD to auth issues",
		Actions: []WaveAction{
			{Type: "add_dod", IssueID: "MY-42", Description: "Token bucket"},
		},
	}
	applyResult := &WaveApplyResult{
		WaveID:  "w1",
		Applied: 1,
		Ripples: []Ripple{
			{ClusterName: "api", Description: "Rate limiting affects API cluster"},
		},
	}

	err := ComposeReport(dir, wave, applyResult)
	if err != nil {
		t.Fatalf("ComposeReport: %v", err)
	}

	// outbox file exists and is parseable
	outboxPath := filepath.Join(MailDir(dir, "outbox"), "report-auth-w1.md")
	data, readErr := os.ReadFile(outboxPath)
	if readErr != nil {
		t.Fatalf("outbox file missing: %v", readErr)
	}

	mail, parseErr := ParseDMail(data)
	if parseErr != nil {
		t.Fatalf("parse: %v", parseErr)
	}
	if mail.Kind != DMailReport {
		t.Errorf("kind: got %s, want report", mail.Kind)
	}
	if mail.Name != "report-auth-w1" {
		t.Errorf("name: got %s", mail.Name)
	}
	if !strings.Contains(mail.Body, "Rate limiting affects API cluster") {
		t.Error("body should contain ripple description")
	}

	// archive file also exists
	archivePath := filepath.Join(MailDir(dir, "archive"), "report-auth-w1.md")
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("archive file missing: %v", err)
	}
}

func TestComposeReport_NoRipples(t *testing.T) {
	dir := t.TempDir()
	EnsureMailDirs(dir)

	wave := Wave{
		ID:          "w2",
		ClusterName: "db",
		Title:       "Database migrations",
		Actions: []WaveAction{
			{Type: "add_dod", IssueID: "MY-50", Description: "Schema migration"},
		},
	}
	applyResult := &WaveApplyResult{
		WaveID:  "w2",
		Applied: 1,
	}

	err := ComposeReport(dir, wave, applyResult)
	if err != nil {
		t.Fatalf("ComposeReport: %v", err)
	}

	outboxPath := filepath.Join(MailDir(dir, "outbox"), "report-db-w2.md")
	data, _ := os.ReadFile(outboxPath)
	mail, _ := ParseDMail(data)
	if mail.Body == "" {
		t.Error("body should not be empty even without ripples")
	}
}

func TestProcessInbox_ReceivesFeedback(t *testing.T) {
	dir := t.TempDir()
	EnsureMailDirs(dir)

	// Place a feedback d-mail in inbox
	fb := &DMail{
		Name:        "feedback-d-001",
		Kind:        DMailFeedback,
		Description: "Architecture drift detected",
		Severity:    "high",
		Body:        "# Feedback\n\nDrift in auth module.\n",
	}
	data, _ := MarshalDMail(fb)
	os.WriteFile(filepath.Join(MailDir(dir, "inbox"), fb.Filename()), data, 0644)

	// Process inbox
	received, err := ProcessInbox(dir)
	if err != nil {
		t.Fatalf("ProcessInbox: %v", err)
	}
	if len(received) != 1 {
		t.Fatalf("expected 1 feedback, got %d", len(received))
	}
	if received[0].Name != "feedback-d-001" {
		t.Errorf("name: got %s", received[0].Name)
	}
	if received[0].Severity != "high" {
		t.Errorf("severity: got %s", received[0].Severity)
	}

	// inbox should be empty
	files, _ := ListDMail(dir, "inbox")
	if len(files) != 0 {
		t.Errorf("inbox not empty after processing: %d files", len(files))
	}

	// archive should have the file
	archivePath := filepath.Join(MailDir(dir, "archive"), fb.Filename())
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("archive file missing: %v", err)
	}
}

func TestProcessInbox_SkipsNonFeedback(t *testing.T) {
	dir := t.TempDir()
	EnsureMailDirs(dir)

	// Place a specification d-mail in inbox (wrong kind for sightjack consumer)
	spec := &DMail{
		Name:        "spec-my-42",
		Kind:        DMailSpecification,
		Description: "Spec for MY-42",
		Body:        "# Spec\n",
	}
	data, _ := MarshalDMail(spec)
	os.WriteFile(filepath.Join(MailDir(dir, "inbox"), spec.Filename()), data, 0644)

	received, err := ProcessInbox(dir)
	if err != nil {
		t.Fatalf("ProcessInbox: %v", err)
	}
	// Non-feedback should be received but filtered out from return
	if len(received) != 0 {
		t.Errorf("expected 0 feedback, got %d", len(received))
	}
}

func TestProcessInbox_DedupSkipsAlreadyArchived(t *testing.T) {
	dir := t.TempDir()
	EnsureMailDirs(dir)

	// Place feedback in both inbox and archive (already processed)
	fb := &DMail{
		Name:        "feedback-d-002",
		Kind:        DMailFeedback,
		Description: "Duplicate feedback",
		Body:        "# Already processed\n",
	}
	data, _ := MarshalDMail(fb)
	os.WriteFile(filepath.Join(MailDir(dir, "inbox"), fb.Filename()), data, 0644)
	os.WriteFile(filepath.Join(MailDir(dir, "archive"), fb.Filename()), data, 0644)

	received, err := ProcessInbox(dir)
	if err != nil {
		t.Fatalf("ProcessInbox: %v", err)
	}
	// Should be skipped (dedup)
	if len(received) != 0 {
		t.Errorf("expected 0 (dedup), got %d", len(received))
	}

	// inbox file should be removed (cleanup)
	files, _ := ListDMail(dir, "inbox")
	if len(files) != 0 {
		t.Errorf("inbox should be cleaned up after dedup: %d files", len(files))
	}
}

func TestDisplayInboxFeedback_NoError(t *testing.T) {
	dir := t.TempDir()
	EnsureMailDirs(dir)

	// Place feedback in inbox
	fb := &DMail{
		Name:        "feedback-d-010",
		Kind:        DMailFeedback,
		Description: "Test feedback",
		Severity:    "high",
		Body:        "# Test\n",
	}
	data, _ := MarshalDMail(fb)
	os.WriteFile(filepath.Join(MailDir(dir, "inbox"), fb.Filename()), data, 0644)

	// Should not panic or error
	DisplayInboxFeedback(dir)

	// inbox should be empty after processing
	files, _ := ListDMail(dir, "inbox")
	if len(files) != 0 {
		t.Errorf("inbox not empty: %d files", len(files))
	}
}

func TestProcessInbox_EmptyInbox(t *testing.T) {
	dir := t.TempDir()
	EnsureMailDirs(dir)

	received, err := ProcessInbox(dir)
	if err != nil {
		t.Fatalf("ProcessInbox: %v", err)
	}
	if len(received) != 0 {
		t.Errorf("expected 0, got %d", len(received))
	}
}

func TestWatchInbox_StopsOnCancel(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureMailDirs(dir); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := WatchInbox(ctx, dir)
	if err != nil {
		t.Fatalf("watch: %v", err)
	}

	// cancel context
	cancel()

	// channel should close
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed after cancel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for channel close")
	}
}
