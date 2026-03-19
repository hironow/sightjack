//go:build scenario

package scenario_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// Observer provides high-level assertion helpers for scenario tests.
// It wraps a Workspace and testing.T to verify mailbox state, D-Mail
// content, and closed-loop completion.
type Observer struct {
	ws *Workspace
	t  *testing.T
}

// NewObserver creates an Observer for the given workspace.
func NewObserver(ws *Workspace, t *testing.T) *Observer {
	return &Observer{ws: ws, t: t}
}

// AssertMailboxState verifies file counts in mailbox directories.
// Keys are relative paths like ".siren/inbox", ".expedition/archive".
func (o *Observer) AssertMailboxState(expectations map[string]int) {
	o.t.Helper()
	for relPath, want := range expectations {
		dir := filepath.Join(o.ws.RepoPath, relPath)
		got := o.ws.CountFiles(o.t, dir)
		if got != want {
			o.t.Errorf("mailbox %s: got %d files, want %d", relPath, got, want)
		}
	}
}

// AssertAllOutboxEmpty verifies that all tool outboxes contain no .md files.
func (o *Observer) AssertAllOutboxEmpty() {
	o.t.Helper()
	tools := []string{".siren", ".expedition", ".gate"}
	for _, tool := range tools {
		dir := filepath.Join(o.ws.RepoPath, tool, "outbox")
		files := o.ws.ListFiles(o.t, dir)
		var mdFiles []string
		for _, f := range files {
			if strings.HasSuffix(f, ".md") {
				mdFiles = append(mdFiles, f)
			}
		}
		if len(mdFiles) > 0 {
			o.t.Errorf("%s/outbox not empty: %v", tool, mdFiles)
		}
	}
}

// AssertArchiveContains verifies that a tool's archive directory contains
// D-Mail files with the expected kinds in their frontmatter.
func (o *Observer) AssertArchiveContains(toolDir string, kinds []string) {
	o.t.Helper()
	dir := filepath.Join(o.ws.RepoPath, toolDir, "archive")
	files := o.ws.ListFiles(o.t, dir)
	if len(files) == 0 && len(kinds) > 0 {
		o.t.Errorf("%s/archive: expected D-Mails with kinds %v, but archive is empty", toolDir, kinds)
		return
	}

	// Collect all kinds found in archive
	foundKinds := make(map[string]bool)
	for _, f := range files {
		if !strings.HasSuffix(f, ".md") {
			continue
		}
		path := filepath.Join(dir, f)
		fm, _ := o.ws.ReadDMail(o.t, path)
		if kind, ok := fm["kind"].(string); ok {
			foundKinds[kind] = true
		}
	}

	for _, want := range kinds {
		if !foundKinds[want] {
			o.t.Errorf("%s/archive: missing D-Mail with kind %q (found kinds: %v)", toolDir, want, foundKinds)
		}
	}
}

// AssertDMailKind verifies that a D-Mail file has the expected kind.
func (o *Observer) AssertDMailKind(path, expectedKind string) {
	o.t.Helper()
	fm, _ := o.ws.ReadDMail(o.t, path)
	kind, ok := fm["kind"].(string)
	if !ok {
		o.t.Errorf("D-Mail %s: missing kind field in frontmatter", path)
		return
	}
	if kind != expectedKind {
		o.t.Errorf("D-Mail %s: got kind %q, want %q", path, kind, expectedKind)
	}
}

// WaitForClosedLoop waits for a complete closed loop (specification -> report -> feedback).
// It polls all 3 delivery points:
//  1. specification in .expedition/inbox
//  2. report in .gate/inbox
//  3. feedback in .siren/inbox AND .expedition/inbox
func (o *Observer) WaitForClosedLoop(timeout time.Duration) {
	o.t.Helper()
	stepTimeout := timeout / 3
	if stepTimeout < 10*time.Second {
		stepTimeout = 10 * time.Second
	}

	o.ws.WaitForDMail(o.t, ".expedition", "inbox", stepTimeout)
	o.ws.WaitForDMail(o.t, ".gate", "inbox", stepTimeout)
	o.ws.WaitForDMail(o.t, ".siren", "inbox", stepTimeout)
}

// --- Config assertion helpers (proposal 014) ---

// AssertSirenConfigStrictness reads .siren/config.yaml and verifies that
// the computed.estimated_strictness field for the given cluster matches
// the expected strictness level. This tests the scan -> estimate ->
// config write-back pipeline end-to-end.
func (o *Observer) AssertSirenConfigStrictness(clusterName, expectedLevel string) {
	o.t.Helper()
	cfgPath := filepath.Join(o.ws.RepoPath, ".siren", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		o.t.Fatalf("read siren config: %v", err)
	}

	var cfg map[string]any
	if yamlErr := yaml.Unmarshal(data, &cfg); yamlErr != nil {
		o.t.Fatalf("parse siren config: %v", yamlErr)
	}

	computed, ok := cfg["computed"].(map[string]any)
	if !ok {
		o.t.Logf("siren config has no 'computed' section (strictness may not have been estimated yet)")
		return
	}

	estimated, ok := computed["estimated_strictness"].(map[string]any)
	if !ok {
		o.t.Logf("siren config has no 'computed.estimated_strictness' section")
		return
	}

	got, ok := estimated[clusterName].(string)
	if !ok {
		o.t.Errorf("cluster %q not found in estimated_strictness (available: %v)", clusterName, estimated)
		return
	}
	if got != expectedLevel {
		o.t.Errorf("strictness for cluster %q: got %q, want %q", clusterName, got, expectedLevel)
	}
}

// AssertSirenConfigExists verifies that .siren/config.yaml exists.
func (o *Observer) AssertSirenConfigExists() {
	o.t.Helper()
	cfgPath := filepath.Join(o.ws.RepoPath, ".siren", "config.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		o.t.Errorf(".siren/config.yaml not found: %v", err)
	}
}

// --- NextGen and Doctor assertion helpers (proposals 027, 028) ---

// AssertEventCount scans .siren/events/*.jsonl for events of the given type
// and verifies the count matches.
func (o *Observer) AssertEventCount(eventType string, wantCount int) {
	o.t.Helper()
	eventsDir := filepath.Join(o.ws.RepoPath, ".siren", "events")
	entries, err := os.ReadDir(eventsDir)
	if err != nil {
		o.t.Fatalf("read events dir: %v", err)
	}

	count := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(eventsDir, entry.Name()))
		if readErr != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, `"type":"`+eventType+`"`) {
				count++
			}
		}
	}
	if count != wantCount {
		o.t.Errorf("event %q: got %d occurrences, want %d", eventType, count, wantCount)
	}
}

// AssertEventExists scans .siren/events/*.jsonl for at least one event of the given type.
func (o *Observer) AssertEventExists(eventType string) {
	o.t.Helper()
	eventsDir := filepath.Join(o.ws.RepoPath, ".siren", "events")
	entries, err := os.ReadDir(eventsDir)
	if err != nil {
		o.t.Fatalf("read events dir: %v", err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(eventsDir, entry.Name()))
		if readErr != nil {
			continue
		}
		if strings.Contains(string(data), `"type":"`+eventType+`"`) {
			return
		}
	}
	o.t.Errorf("event %q not found in .siren/events/*.jsonl", eventType)
}

// --- Waiting mode and Label helpers (proposals 033, 034) ---

// AssertWaitingModeNotActive verifies that the current scenario is NOT in
// waiting mode (the default when --wait-timeout -1s is passed). This documents
// the structural blind spot: all scenario tests disable waiting mode.
func (o *Observer) AssertWaitingModeNotActive() {
	o.t.Helper()
	// In waiting mode, .siren/run/watch.pid would exist.
	// When disabled (--wait-timeout -1s), no watch.pid should be present.
	pidPath := filepath.Join(o.ws.RepoPath, ".siren", "run", "watch.pid")
	if _, err := os.Stat(pidPath); err == nil {
		o.t.Error("watch.pid exists — waiting mode should not be active in scenario tests (--wait-timeout -1s)")
	}
}

// AssertLabelsDisabled verifies that Labels.Enabled is false in the siren
// config. Documents that all scenario tests run with labels disabled.
func (o *Observer) AssertLabelsDisabled() {
	o.t.Helper()
	cfgPath := filepath.Join(o.ws.RepoPath, ".siren", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		o.t.Fatalf("read siren config: %v", err)
	}

	var cfg map[string]any
	if yamlErr := yaml.Unmarshal(data, &cfg); yamlErr != nil {
		o.t.Fatalf("parse siren config: %v", yamlErr)
	}

	labels, ok := cfg["labels"].(map[string]any)
	if !ok {
		// No labels section means disabled (default)
		return
	}

	enabled, _ := labels["enabled"].(bool)
	if enabled {
		o.t.Error("labels.enabled is true — expected false in default scenario config")
	}
}
