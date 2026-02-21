//go:build e2e

package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestE2E_DMail_DirectoryLifecycle(t *testing.T) {
	// given: a configured directory with no pre-existing mail dirs
	dir := initDir(t)

	// when: run --dry-run (triggers EnsureMailDirs in RunSession)
	cmd := exec.Command(sightjackBin(), "run", "--dry-run", dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// then
	if err != nil {
		t.Fatalf("run --dry-run failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}
	assertDirExists(t, filepath.Join(dir, ".siren", "inbox"))
	assertDirExists(t, filepath.Join(dir, ".siren", "outbox"))
	assertDirExists(t, filepath.Join(dir, ".siren", "archive"))
}

func TestE2E_DMail_ArchivePrunePreservesRecent(t *testing.T) {
	// given: a configured directory with recent d-mails in archive
	dir := initDir(t)
	ensureMailDirs(t, dir)

	archiveDir := filepath.Join(dir, ".siren", "archive")
	recentFiles := []string{"spec-auth-w1.md", "report-auth-w1.md", "feedback-late-001.md"}
	for _, name := range recentFiles {
		content := marshalDMail(
			strings.TrimSuffix(name, ".md"), "specification", "Recent d-mail", "", "Recent body.", nil,
		)
		writeDMailToDir(t, dir, "archive", name, content)
	}

	// when: archive-prune dry-run (no --execute)
	cmd := exec.Command(sightjackBin(), "archive-prune", "-d", "30", dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// then
	if err != nil {
		t.Fatalf("archive-prune failed: %v\nstderr: %s", err, stderr.String())
	}
	// All files should still exist (all recent)
	for _, name := range recentFiles {
		assertFileExists(t, filepath.Join(archiveDir, name))
	}
	if !strings.Contains(stderr.String(), "No expired files") {
		t.Errorf("expected 'No expired files' message, got stderr: %s", stderr.String())
	}
}

func TestE2E_DMail_ArchivePruneIgnoresNonMd(t *testing.T) {
	// given: archive with mixed file types, all old
	dir := initDir(t)
	ensureMailDirs(t, dir)

	archiveDir := filepath.Join(dir, ".siren", "archive")
	oldTime := time.Now().Add(-60 * 24 * time.Hour)

	// Non-.md files (should be ignored)
	for _, name := range []string{"state.json", "notes.txt"} {
		p := filepath.Join(archiveDir, name)
		if err := os.WriteFile(p, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(p, oldTime, oldTime)
	}
	// .md file (should be pruned)
	mdContent := marshalDMail("report-old", "report", "Old report", "", "Old body.", nil)
	mdPath := writeDMailToDir(t, dir, "archive", "report-old.md", mdContent)
	os.Chtimes(mdPath, oldTime, oldTime)

	// when: archive-prune with --execute
	cmd := exec.Command(sightjackBin(), "archive-prune", "-d", "30", "-x", dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// then
	if err != nil {
		t.Fatalf("archive-prune failed: %v\nstderr: %s", err, stderr.String())
	}
	// .md file should be deleted
	assertFileNotExists(t, mdPath)
	// Non-.md files should remain
	assertFileExists(t, filepath.Join(archiveDir, "state.json"))
	assertFileExists(t, filepath.Join(archiveDir, "notes.txt"))
	// stderr should mention only 1 file
	if !strings.Contains(stderr.String(), "1 file(s)") {
		t.Errorf("expected '1 file(s)' in stderr, got: %s", stderr.String())
	}
}

func TestE2E_DMail_ArchivePruneRealDMails(t *testing.T) {
	// given: archive with properly formatted d-mails at various ages
	dir := initDir(t)
	ensureMailDirs(t, dir)

	archiveDir := filepath.Join(dir, ".siren", "archive")
	oldTime60 := time.Now().Add(-60 * 24 * time.Hour)
	oldTime45 := time.Now().Add(-45 * 24 * time.Hour)

	// Old d-mails (should be pruned)
	specOld := marshalDMail("spec-auth-old", "specification", "Old spec", "", "Old spec body.", []string{"AUTH-1"})
	specPath := writeDMailToDir(t, dir, "archive", "spec-auth-old.md", specOld)
	os.Chtimes(specPath, oldTime60, oldTime60)

	reportOld := marshalDMail("report-api-old", "report", "Old report", "", "Old report body.", []string{"API-1"})
	reportPath := writeDMailToDir(t, dir, "archive", "report-api-old.md", reportOld)
	os.Chtimes(reportPath, oldTime45, oldTime45)

	// Recent d-mail (should be preserved)
	feedbackRecent := marshalDMail("feedback-recent", "feedback", "Recent feedback", "high", "Recent body.", []string{"AUTH-1"})
	writeDMailToDir(t, dir, "archive", "feedback-recent.md", feedbackRecent)

	// when: archive-prune with --execute, 30 day threshold
	cmd := exec.Command(sightjackBin(), "archive-prune", "-d", "30", "-x", dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// then
	if err != nil {
		t.Fatalf("archive-prune failed: %v\nstderr: %s", err, stderr.String())
	}
	assertFileNotExists(t, specPath)
	assertFileNotExists(t, reportPath)
	assertFileExists(t, filepath.Join(archiveDir, "feedback-recent.md"))
	if !strings.Contains(stderr.String(), "2 file(s) older than 30 days") {
		t.Errorf("expected '2 file(s) older than 30 days', got stderr: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Pruned 2 file(s)") {
		t.Errorf("expected 'Pruned 2 file(s)', got stderr: %s", stderr.String())
	}
}

func TestE2E_DMail_SpecAndReport(t *testing.T) {
	// given: a configured directory
	dir := initDir(t)

	// when: run full interactive session (scan → waves → approve → apply → nextgen → quit)
	runFullSession(t, dir)

	// then: verify spec and report d-mails were generated
	outboxDir := filepath.Join(dir, ".siren", "outbox")
	archiveDir := filepath.Join(dir, ".siren", "archive")

	t.Run("Specification", func(t *testing.T) {
		specFile := "spec-auth-auth-w1.md"
		assertFileExists(t, filepath.Join(outboxDir, specFile))
		assertFileExists(t, filepath.Join(archiveDir, specFile))

		dm := parseDMailFile(t, filepath.Join(outboxDir, specFile))
		if dm.Kind != "specification" {
			t.Errorf("expected kind=specification, got %q", dm.Kind)
		}
		if dm.Name != "spec-auth-auth-w1" {
			t.Errorf("expected name=spec-auth-auth-w1, got %q", dm.Name)
		}
		if len(dm.Issues) == 0 {
			t.Error("expected non-empty issues list")
		}
	})

	t.Run("Report", func(t *testing.T) {
		reportFile := "report-auth-auth-w1.md"
		assertFileExists(t, filepath.Join(outboxDir, reportFile))
		assertFileExists(t, filepath.Join(archiveDir, reportFile))

		dm := parseDMailFile(t, filepath.Join(outboxDir, reportFile))
		if dm.Kind != "report" {
			t.Errorf("expected kind=report, got %q", dm.Kind)
		}
		if dm.Name != "report-auth-auth-w1" {
			t.Errorf("expected name=report-auth-auth-w1, got %q", dm.Name)
		}
		if dm.Body == "" {
			t.Error("expected non-empty body in report d-mail")
		}
	})

	t.Run("Format", func(t *testing.T) {
		for _, file := range []string{"spec-auth-auth-w1.md", "report-auth-auth-w1.md"} {
			path := filepath.Join(outboxDir, file)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}
			content := string(data)
			if !strings.HasPrefix(content, "---\n") {
				t.Errorf("%s: missing opening delimiter", file)
			}
			// Verify YAML frontmatter is parseable
			dm := parseDMailBytes(t, data)
			if dm.Description == "" {
				t.Errorf("%s: expected non-empty description", file)
			}
			if dm.Body == "" {
				t.Errorf("%s: expected non-empty body", file)
			}
			// Body should contain Markdown heading
			if !strings.Contains(dm.Body, "#") {
				t.Errorf("%s: expected Markdown heading in body, got: %s", file, dm.Body)
			}
		}
	})
}

func TestE2E_DMail_FeedbackConsumed(t *testing.T) {
	// given: a configured directory with pre-existing feedback in inbox
	dir := initDir(t)
	ensureMailDirs(t, dir)

	// Place feedback d-mail in inbox
	feedbackContent := fixtureBytes(t, "feedback_review.md")
	writeDMailToDir(t, dir, "inbox", "feedback-review-001.md", feedbackContent)

	// when: run full interactive session
	runFullSession(t, dir)

	// then: feedback should be consumed (moved from inbox to archive)
	inboxPath := filepath.Join(dir, ".siren", "inbox", "feedback-review-001.md")
	archivePath := filepath.Join(dir, ".siren", "archive", "feedback-review-001.md")

	assertFileNotExists(t, inboxPath)
	assertFileExists(t, archivePath)

	// Verify archived feedback is parseable and has correct kind
	dm := parseDMailFile(t, archivePath)
	if dm.Kind != "feedback" {
		t.Errorf("expected kind=feedback, got %q", dm.Kind)
	}
}

func TestE2E_DMail_DedupSkipsDuplicate(t *testing.T) {
	// given: same-named d-mail in both inbox and archive (simulating already-processed)
	dir := initDir(t)
	ensureMailDirs(t, dir)

	// Create feedback d-mail with known content
	content := marshalDMail("feedback-dup-001", "feedback", "Duplicate feedback", "high", "Original body.", []string{"AUTH-1"})
	originalContent := make([]byte, len(content))
	copy(originalContent, content)

	// Place in both inbox and archive
	writeDMailToDir(t, dir, "inbox", "feedback-dup-001.md", content)
	writeDMailToDir(t, dir, "archive", "feedback-dup-001.md", content)

	// when: run full interactive session
	runFullSession(t, dir)

	// then: inbox copy removed, archive copy unchanged
	inboxPath := filepath.Join(dir, ".siren", "inbox", "feedback-dup-001.md")
	archivePath := filepath.Join(dir, ".siren", "archive", "feedback-dup-001.md")

	assertFileNotExists(t, inboxPath)
	assertFileExists(t, archivePath)

	// Archive content should be unchanged (dedup didn't overwrite)
	archivedBytes, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	if string(archivedBytes) != string(originalContent) {
		t.Error("archive content was modified by dedup (should be unchanged)")
	}
}

func TestE2E_DMail_FsnotifyAndInjection(t *testing.T) {
	// given: a configured directory with prompt logging enabled
	dir := initDir(t)
	ensureMailDirs(t, dir)

	promptLogDir := filepath.Join(dir, ".siren", "prompt_log")

	// Prepare feedback d-mail content to write mid-session
	lateContent := marshalDMail(
		"feedback-late-001", "feedback",
		"Late architecture feedback", "high",
		"Session-time feedback body.", []string{"AUTH-1"},
	)

	// when: run session with feedback injection mid-session
	runFullSession(t, dir,
		withEnv("FAKE_CLAUDE_PROMPT_LOG_DIR="+promptLogDir),
		withAfterFirstSelect(func() {
			// Write feedback to inbox while session is running (fsnotify watcher active)
			writeDMailToDir(t, dir, "inbox", "feedback-late-001.md", lateContent)
		}),
	)

	// then
	t.Run("FsnotifyDetected", func(t *testing.T) {
		// Inbox should be empty (fsnotify consumed the file)
		inboxFiles := listMailDir(t, dir, "inbox")
		for _, f := range inboxFiles {
			if f == "feedback-late-001.md" {
				t.Errorf("feedback-late-001.md still in inbox (fsnotify did not consume)")
			}
		}

		// Archive should contain the feedback file
		assertFileExists(t, filepath.Join(dir, ".siren", "archive", "feedback-late-001.md"))
	})

	t.Run("InjectedIntoPrompt", func(t *testing.T) {
		// Read all prompt log files and check for feedback content
		entries, err := os.ReadDir(promptLogDir)
		if err != nil {
			t.Fatalf("read prompt log dir: %v", err)
		}
		if len(entries) == 0 {
			t.Fatal("no prompt log files found (FAKE_CLAUDE_PROMPT_LOG_DIR not working)")
		}

		found := false
		for _, e := range entries {
			data, err := os.ReadFile(filepath.Join(promptLogDir, e.Name()))
			if err != nil {
				continue
			}
			content := string(data)
			// FormatFeedbackForPrompt includes the d-mail description
			if strings.Contains(content, "Late architecture feedback") {
				found = true
				break
			}
		}
		if !found {
			t.Error("feedback description not found in any nextgen prompt (injection failed)")
		}
	})
}
