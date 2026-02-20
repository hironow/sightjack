package sightjack

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestListExpiredArchive_EmptyDir(t *testing.T) {
	// given
	baseDir := t.TempDir()
	if err := EnsureMailDirs(baseDir); err != nil {
		t.Fatal(err)
	}

	// when
	files, err := ListExpiredArchive(baseDir, 30)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 expired files, got %d", len(files))
	}
}

func TestListExpiredArchive_FiltersByMtime(t *testing.T) {
	// given
	baseDir := t.TempDir()
	if err := EnsureMailDirs(baseDir); err != nil {
		t.Fatal(err)
	}
	archiveDir := MailDir(baseDir, archiveDir)

	// Create old file (40 days ago)
	oldFile := filepath.Join(archiveDir, "report-old-w1.md")
	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	// Create recent file (5 days ago)
	recentFile := filepath.Join(archiveDir, "spec-new-w2.md")
	if err := os.WriteFile(recentFile, []byte("recent"), 0644); err != nil {
		t.Fatal(err)
	}
	recentTime := time.Now().Add(-5 * 24 * time.Hour)
	if err := os.Chtimes(recentFile, recentTime, recentTime); err != nil {
		t.Fatal(err)
	}

	// when — threshold 30 days
	files, err := ListExpiredArchive(baseDir, 30)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 expired file, got %d", len(files))
	}
	if files[0] != "report-old-w1.md" {
		t.Errorf("expected report-old-w1.md, got %s", files[0])
	}
}

func TestListExpiredArchive_OnlyMdFiles(t *testing.T) {
	// given
	baseDir := t.TempDir()
	if err := EnsureMailDirs(baseDir); err != nil {
		t.Fatal(err)
	}
	archiveDir := MailDir(baseDir, archiveDir)

	// Create old .md file
	mdFile := filepath.Join(archiveDir, "feedback-001.md")
	if err := os.WriteFile(mdFile, []byte("md"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create old .txt file (should be ignored)
	txtFile := filepath.Join(archiveDir, "notes.txt")
	if err := os.WriteFile(txtFile, []byte("txt"), 0644); err != nil {
		t.Fatal(err)
	}

	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	os.Chtimes(mdFile, oldTime, oldTime)
	os.Chtimes(txtFile, oldTime, oldTime)

	// when
	files, err := ListExpiredArchive(baseDir, 30)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 expired file, got %d", len(files))
	}
	if files[0] != "feedback-001.md" {
		t.Errorf("expected feedback-001.md, got %s", files[0])
	}
}

func TestListExpiredArchive_NoDirReturnsEmpty(t *testing.T) {
	// given — no archive dir exists
	baseDir := t.TempDir()

	// when
	files, err := ListExpiredArchive(baseDir, 30)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files when archive dir missing, got %d", len(files))
	}
}

func TestPruneArchive_DeletesExpiredFiles(t *testing.T) {
	// given
	baseDir := t.TempDir()
	if err := EnsureMailDirs(baseDir); err != nil {
		t.Fatal(err)
	}
	ad := MailDir(baseDir, archiveDir)

	oldFile := filepath.Join(ad, "report-old-w1.md")
	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-40 * 24 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	keepFile := filepath.Join(ad, "spec-new-w2.md")
	if err := os.WriteFile(keepFile, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	// when
	deleted, err := PruneArchive(baseDir, 30)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 1 {
		t.Fatalf("expected 1 deleted, got %d", len(deleted))
	}
	if deleted[0] != "report-old-w1.md" {
		t.Errorf("expected report-old-w1.md, got %s", deleted[0])
	}

	// Verify file was actually removed
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("expected old file to be deleted")
	}
	// Verify keep file still exists
	if _, err := os.Stat(keepFile); err != nil {
		t.Error("expected keep file to remain")
	}
}

func TestListExpiredArchive_NegativeDaysReturnsError(t *testing.T) {
	// given
	baseDir := t.TempDir()

	// when
	_, err := ListExpiredArchive(baseDir, -1)

	// then
	if err == nil {
		t.Fatal("expected error for negative days")
	}
	if err.Error() != "days must be non-negative, got -1" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPruneArchive_NegativeDaysReturnsError(t *testing.T) {
	// given
	baseDir := t.TempDir()

	// when
	_, err := PruneArchive(baseDir, -1)

	// then
	if err == nil {
		t.Fatal("expected error for negative days")
	}
}

func TestPruneArchive_NoDirReturnsEmpty(t *testing.T) {
	// given
	baseDir := t.TempDir()

	// when
	deleted, err := PruneArchive(baseDir, 30)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deleted) != 0 {
		t.Errorf("expected 0 deleted when archive dir missing, got %d", len(deleted))
	}
}
