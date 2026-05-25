package session

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

// ADRSubdir is the conventional ADR directory under a project root.
const ADRSubdir = "docs/adr"

var adrPattern = regexp.MustCompile(`^(\d{4})-.*\.md$`)

// ADRDir returns the ADR directory path under baseDir.
func ADRDir(baseDir string) string {
	return filepath.Join(baseDir, ADRSubdir)
}

// NextADRNumber scans adrDir for files matching NNNN-*.md and returns max(NNNN)+1.
// Returns 1 if the directory is empty or does not exist.
func NextADRNumber(adrDir string) (int, error) {
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 1, nil
		}
		return 0, err
	}

	maxNum := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		matches := adrPattern.FindStringSubmatch(e.Name())
		if matches == nil {
			continue
		}
		num, parseErr := strconv.Atoi(matches[1])
		if parseErr != nil {
			continue
		}
		if num > maxNum {
			maxNum = num
		}
	}

	return maxNum + 1, nil
}

// SanitizeADRTitle ensures an ADR title is safe for use in filenames.
// Prevents path traversal by stripping everything except [a-z0-9-_].
// Returns "untitled" for empty input.
func SanitizeADRTitle(title string) string {
	s := domain.SanitizeName(title)
	if s == "" {
		return "untitled"
	}
	return s
}

// RenderADRFromDiscuss generates an ADR Markdown document from a domain.DiscussResult.
// This is a pure transformer — no Claude invocation needed.
func RenderADRFromDiscuss(dr domain.DiscussResult, adrNum int) string {
	title := dr.ADRTitle
	if title == "" {
		title = dr.WaveID
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %04d. %s\n\n", adrNum, title)
	fmt.Fprintf(&b, "**Date:** %s\n", time.Now().Format("2006-01-02"))
	fmt.Fprintf(&b, "**Status:** Accepted\n\n")
	fmt.Fprintf(&b, "## Context\n\n%s\n\n", dr.Analysis)
	fmt.Fprintf(&b, "## Decision\n\n%s\n\n", dr.Decision)
	fmt.Fprintf(&b, "## Consequences\n\n%s\n", dr.Reasoning)

	if len(dr.Modifications) > 0 {
		fmt.Fprintf(&b, "\n### Modifications\n\n")
		for _, m := range dr.Modifications {
			fmt.Fprintf(&b, "- Action %d: %s\n", m.ActionIndex, m.Change)
		}
	}

	return b.String()
}

// CountADRFiles returns the number of files matching NNNN-*.md in adrDir.
// Returns 0 if the directory is empty or does not exist.
func CountADRFiles(adrDir string) int {
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && adrPattern.MatchString(e.Name()) {
			count++
		}
	}
	return count
}
