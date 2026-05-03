// Package session archive_reader.go: Phase 1.1B archive D-Mail loader for
// `rival export reasons --wave <id>`.
//
// The archive is normally a write-only store (see .semgrep/no-archive-read.yaml).
// This file is the sanctioned exception per the rule's own carve-out:
// `paths.exclude: internal/session/archive_reader.go`. The export tool's
// projection MUST consider every archived contract revision so the
// deterministic ProjectCurrentContracts winner-selection logic produces
// the same result here as in amadeus's verifier.
//
// Refs: refs/plans/2026-05-03-rival-contract-v1-1-extensions.md §"Phase 1.1B"
package session

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
)

// ReadArchiveDMails loads every `*.md` D-Mail file under
// <baseDir>/.siren/archive/, parses the YAML+Markdown payload, and returns
// the parsed D-Mails sorted by name for deterministic ordering.
//
// Files that fail to parse are skipped silently (Postel-liberal); the
// caller (Rival Contract projection) is expected to ignore D-Mails it
// cannot interpret. I/O errors on directory read are surfaced; per-file
// read errors are skipped to keep the projection robust against partial
// archives.
//
// The function is read-only; it never modifies the archive.
func ReadArchiveDMails(baseDir string) ([]domain.DMail, error) { // nosemgrep: no-archive-read-funcs -- sanctioned exception per .semgrep/no-archive-read.yaml carve-out for rival export reasons --wave projection (Phase 1.1B) [permanent]
	dir := domain.MailDir(baseDir, domain.ArchiveDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read archive dir %s: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	out := make([]domain.DMail, 0, len(names))
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		mail, err := ParseDMail(data)
		if err != nil || mail == nil {
			continue
		}
		out = append(out, *mail)
	}
	return out, nil
}
