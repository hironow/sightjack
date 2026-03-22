package session_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/session"
)

func TestSupersedeADR_PatchesStatusLine(t *testing.T) {
	// given: an ADR file with "Status: Accepted"
	dir := t.TempDir()
	adrPath := filepath.Join(dir, "0001-use-sqlite.md")
	content := "# 0001. Use SQLite\n\n**Date:** 2025-01-01\n**Status:** Accepted\n\n## Context\n\nSome context.\n"
	if err := os.WriteFile(adrPath, []byte(content), 0644); err != nil {
		t.Fatalf("write adr file: %v", err)
	}

	// when
	err := session.SupersedeADR(adrPath, "0002")

	// then
	if err != nil {
		t.Fatalf("SupersedeADR: unexpected error: %v", err)
	}
	updated, err := os.ReadFile(adrPath)
	if err != nil {
		t.Fatalf("read updated adr: %v", err)
	}
	updatedStr := string(updated)
	if !strings.Contains(updatedStr, "Superseded by [0002]") {
		t.Errorf("SupersedeADR: expected status to contain 'Superseded by [0002]', got:\n%s", updatedStr)
	}
	// original "Accepted" should no longer be standalone
	if strings.Contains(updatedStr, "**Status:** Accepted\n") {
		t.Errorf("SupersedeADR: expected old Accepted status to be replaced, got:\n%s", updatedStr)
	}
}

func TestSupersedeADR_FileNotFound(t *testing.T) {
	// given: a non-existent file
	adrPath := filepath.Join(t.TempDir(), "nonexistent.md")

	// when
	err := session.SupersedeADR(adrPath, "0003")

	// then
	if err == nil {
		t.Error("SupersedeADR: expected error for non-existent file, got nil")
	}
}

func TestSupersedeADR_NoStatusLine(t *testing.T) {
	// given: an ADR file without a Status line
	dir := t.TempDir()
	adrPath := filepath.Join(dir, "0001-no-status.md")
	content := "# 0001. Some Decision\n\n## Context\n\nNo status line here.\n"
	if err := os.WriteFile(adrPath, []byte(content), 0644); err != nil {
		t.Fatalf("write adr file: %v", err)
	}

	// when
	err := session.SupersedeADR(adrPath, "0002")

	// then: should return an error since there is no Status line to patch
	if err == nil {
		t.Error("SupersedeADR: expected error when no Status line found, got nil")
	}
}
