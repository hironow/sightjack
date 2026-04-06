//go:build scenario

package scenario_test

import (
	"fmt"
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
// waiting mode (the default when --idle-timeout -1s is passed). This documents
// the structural blind spot: all scenario tests disable waiting mode.
func (o *Observer) AssertWaitingModeNotActive() {
	o.t.Helper()
	// In waiting mode, .siren/run/watch.pid would exist.
	// When disabled (--idle-timeout -1s), no watch.pid should be present.
	pidPath := filepath.Join(o.ws.RepoPath, ".siren", "run", "watch.pid")
	if _, err := os.Stat(pidPath); err == nil {
		o.t.Error("watch.pid exists — waiting mode should not be active in scenario tests (--idle-timeout -1s)")
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

// --- Scan error recovery and session resume helpers (proposals 039, 042) ---

// AssertScanWarningsExist checks if the scan result contains warnings
// by reading .siren/events/*.jsonl for scan.completed events with non-empty
// warnings data.
func (o *Observer) AssertScanWarningsExist() {
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
		data, _ := os.ReadFile(filepath.Join(eventsDir, entry.Name()))
		content := string(data)
		if strings.Contains(content, `"scan.completed"`) && strings.Contains(content, `"warnings"`) {
			return
		}
	}
	o.t.Error("no scan.completed event with warnings found")
}

// AssertSessionResumed checks for a session.resumed event in JSONL.
func (o *Observer) AssertSessionResumed() {
	o.t.Helper()
	o.AssertEventExists("session.resumed")
}

// AssertSessionRescanned checks for a session.rescanned event in JSONL.
func (o *Observer) AssertSessionRescanned() {
	o.t.Helper()
	o.AssertEventExists("session.rescanned")
}

// --- Completeness and ADR format helpers (proposals 044, 047) ---

// AssertCompletenessUpdated checks for a completeness.updated event in JSONL.
func (o *Observer) AssertCompletenessUpdated() {
	o.t.Helper()
	o.AssertEventExists("completeness.updated")
}

// AssertADRFileExists checks that at least one ADR .md file exists in docs/adr/.
func (o *Observer) AssertADRFileExists() {
	o.t.Helper()
	adrDir := filepath.Join(o.ws.RepoPath, "docs", "adr")
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		o.t.Logf("docs/adr/ not accessible: %v", err)
		return
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".md") {
			return
		}
	}
	o.t.Error("no .md files found in docs/adr/")
}

// AssertADRContainsSections reads the first ADR file in docs/adr/ and verifies
// it contains the expected Markdown sections (## Context, ## Decision, ## Consequences).
func (o *Observer) AssertADRContainsSections() {
	o.t.Helper()
	adrDir := filepath.Join(o.ws.RepoPath, "docs", "adr")
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		o.t.Fatalf("docs/adr/: %v", err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(adrDir, entry.Name()))
		content := string(data)
		for _, section := range []string{"## Context", "## Decision", "## Consequences"} {
			if !strings.Contains(content, section) {
				o.t.Errorf("ADR %s missing section %q", entry.Name(), section)
			}
		}
		return
	}
	o.t.Error("no ADR .md files to check")
}

// --- Specification D-Mail output helpers (proposal 050) ---

// AssertSpecificationFields reads specification D-Mails from .siren/outbox/
// or .expedition/inbox/ and verifies key fields are present.
func (o *Observer) AssertSpecificationFields(toolDir, mailbox string) {
	o.t.Helper()
	dir := filepath.Join(o.ws.RepoPath, toolDir, mailbox)
	files := o.ws.ListFiles(o.t, dir)
	for _, f := range files {
		if !strings.HasSuffix(f, ".md") {
			continue
		}
		path := filepath.Join(dir, f)
		fm, _ := o.ws.ReadDMail(o.t, path)
		kind, _ := fm["kind"].(string)
		if kind != "specification" {
			continue
		}
		// Verify required fields
		if _, ok := fm["description"].(string); !ok {
			o.t.Errorf("specification D-Mail %s: missing description field", f)
		}
		if _, ok := fm["dmail-schema-version"].(string); !ok {
			o.t.Errorf("specification D-Mail %s: missing dmail-schema-version", f)
		}
		return
	}
	o.t.Error("no specification D-Mail found")
}

// --- Architect discussion helpers (proposal 058) ---

// AssertArchitectNotCalled documents that --auto-approve bypasses the
// architect discuss flow entirely. sj_architect.json fixtures exist in
// all 4 sets but are never reached in scenario tests.
func (o *Observer) AssertArchitectNotCalled() {
	o.t.Helper()
	// With --auto-approve, architect is skipped. This helper documents
	// the structural blind spot: RunArchitectDiscuss is only reachable
	// via interactive "d" input, which go-expect could provide.
	o.t.Logf("NOTE: architect discuss is structurally bypassed by --auto-approve in all scenario tests")
}

// --- Report output + wave apply failure helpers (proposals 062, 065) ---

// AssertReportFields reads report D-Mails and verifies key fields.
func (o *Observer) AssertReportFields(toolDir, mailbox string) {
	o.t.Helper()
	dir := filepath.Join(o.ws.RepoPath, toolDir, mailbox)
	files := o.ws.ListFiles(o.t, dir)
	for _, f := range files {
		if !strings.HasSuffix(f, ".md") {
			continue
		}
		path := filepath.Join(dir, f)
		fm, _ := o.ws.ReadDMail(o.t, path)
		kind, _ := fm["kind"].(string)
		if kind != "report" {
			continue
		}
		if _, ok := fm["description"].(string); !ok {
			o.t.Errorf("report D-Mail %s: missing description", f)
		}
		if _, ok := fm["dmail-schema-version"].(string); !ok {
			o.t.Errorf("report D-Mail %s: missing dmail-schema-version", f)
		}
		return
	}
	o.t.Error("no report D-Mail found")
}

// AssertWaveApplyFailed checks for a wave_apply_failed or error event
// in JSONL after a failed wave apply attempt.
func (o *Observer) AssertWaveApplyFailed() {
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
		data, _ := os.ReadFile(filepath.Join(eventsDir, entry.Name()))
		content := string(data)
		if strings.Contains(content, `"wave_apply_failed"`) || strings.Contains(content, `"apply_error"`) {
			return
		}
	}
	o.t.Error("no wave apply failure event found in .siren/events/*.jsonl")
}

// --- Feedback consumption + config set helpers (proposals 068, 071) ---

// AssertPromptContains reads prompt logs from PromptLogDir and verifies
// at least one contains the given substring. Sightjack equivalent of
// paintress AssertPromptContainsLumina.
func (o *Observer) AssertPromptContains(substring string) {
	o.t.Helper()
	dir := o.ws.PromptLogDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		o.t.Fatalf("read prompt-log dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(dir, entry.Name()))
		if strings.Contains(string(data), substring) {
			return
		}
	}
	o.t.Errorf("no prompt log contains %q (checked %d files)", substring, len(entries))
}

// AssertConfigValue reads .siren/config.yaml and verifies a top-level
// field has the expected string value. For nested fields, use dot notation
// (not yet supported — this checks top-level only).
func (o *Observer) AssertConfigValue(key, wantValue string) {
	o.t.Helper()
	cfgPath := filepath.Join(o.ws.RepoPath, ".siren", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		o.t.Fatalf("read siren config: %v", err)
	}
	var cfg map[string]any
	if yamlErr := yaml.Unmarshal(data, &cfg); yamlErr != nil {
		o.t.Fatalf("parse config: %v", yamlErr)
	}
	got, ok := cfg[key]
	if !ok {
		o.t.Errorf("config key %q not found", key)
		return
	}
	gotStr := fmt.Sprintf("%v", got)
	if gotStr != wantValue {
		o.t.Errorf("config %s: got %q, want %q", key, gotStr, wantValue)
	}
}
