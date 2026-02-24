package sightjack_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sightjack "github.com/hironow/sightjack"
)

func TestIsWaveApplyComplete_NoErrors(t *testing.T) {
	// given
	result := &sightjack.WaveApplyResult{
		WaveID:  "auth-w1",
		Applied: 5,
		Errors:  []string{},
		Ripples: []sightjack.Ripple{{ClusterName: "API", Description: "W2 unlocked"}},
	}

	// when
	complete := sightjack.IsWaveApplyComplete(result)

	// then
	if !complete {
		t.Error("expected complete when no errors")
	}
}

func TestIsWaveApplyComplete_WithErrors(t *testing.T) {
	// given
	result := &sightjack.WaveApplyResult{
		WaveID:  "auth-w1",
		Applied: 3,
		Errors:  []string{"failed to update ENG-101", "failed to update ENG-102"},
		Ripples: []sightjack.Ripple{},
	}

	// when
	complete := sightjack.IsWaveApplyComplete(result)

	// then
	if complete {
		t.Error("expected not complete when errors present")
	}
}

func TestIsWaveApplyComplete_NilErrors(t *testing.T) {
	// given
	result := &sightjack.WaveApplyResult{
		WaveID:  "auth-w1",
		Applied: 5,
		Errors:  nil,
	}

	// when
	complete := sightjack.IsWaveApplyComplete(result)

	// then
	if !complete {
		t.Error("expected complete when errors is nil")
	}
}

func TestRunSession_DryRunGeneratesWavePrompts(t *testing.T) {
	// given: dry-run session should generate both classify and wave_generate prompts
	baseDir := t.TempDir()
	cfg := &sightjack.Config{
		Lang: "en",
		Claude: sightjack.ClaudeConfig{
			Command:    "claude",
			TimeoutSec: 60,
		},
		Scan: sightjack.ScanConfig{
			MaxConcurrency: 1,
			ChunkSize:      50,
		},
		Linear: sightjack.LinearConfig{
			Team:    "ENG",
			Project: "Test",
		},
		Scribe: sightjack.ScribeConfig{Enabled: true},
	}
	sessionID := "test-dry-run"
	ctx := context.Background()

	// when
	err := sightjack.RunSession(ctx, cfg, baseDir, sessionID, true, nil, io.Discard, sightjack.NewLogger(io.Discard, false))

	// then: no error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// then: classify prompt was generated (Pass 1)
	scanDir := sightjack.ScanDir(baseDir, sessionID)
	classifyPrompt := filepath.Join(scanDir, "classify_prompt.md")
	if _, err := os.Stat(classifyPrompt); os.IsNotExist(err) {
		t.Error("classify_prompt.md not generated")
	}

	// then: wave_generate prompt was generated (Pass 3)
	wavePrompt := filepath.Join(scanDir, "wave_00_sample_prompt.md")
	if _, err := os.Stat(wavePrompt); os.IsNotExist(err) {
		t.Error("wave_00_sample_prompt.md not generated — dry-run did not reach Pass 3")
	}

	// then: architect discuss prompt was generated
	architectPrompt := filepath.Join(scanDir, "architect_sample_sample-w1_prompt.md")
	if _, err := os.Stat(architectPrompt); os.IsNotExist(err) {
		t.Error("architect_sample_sample-w1_prompt.md not generated — dry-run did not reach architect step")
	}

	// then: scribe ADR prompt was generated
	scribePrompt := filepath.Join(scanDir, "scribe_sample_sample-w1_prompt.md")
	if _, err := os.Stat(scribePrompt); os.IsNotExist(err) {
		t.Error("scribe_sample_sample-w1_prompt.md not generated — dry-run did not reach scribe step")
	}
}

func TestRunSession_DryRunSkipsScribeWhenDisabled(t *testing.T) {
	// given: dry-run with Scribe disabled
	baseDir := t.TempDir()
	cfg := &sightjack.Config{
		Lang: "en",
		Claude: sightjack.ClaudeConfig{
			Command:    "claude",
			TimeoutSec: 60,
		},
		Scan: sightjack.ScanConfig{
			MaxConcurrency: 1,
			ChunkSize:      50,
		},
		Linear: sightjack.LinearConfig{
			Team:    "ENG",
			Project: "Test",
		},
		Scribe: sightjack.ScribeConfig{Enabled: false},
	}
	sessionID := "test-dry-run-no-scribe"
	ctx := context.Background()

	// when
	err := sightjack.RunSession(ctx, cfg, baseDir, sessionID, true, nil, io.Discard, sightjack.NewLogger(io.Discard, false))

	// then: no error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// then: scribe prompt should NOT be generated
	scanDir := sightjack.ScanDir(baseDir, sessionID)
	scribePrompt := filepath.Join(scanDir, "scribe_sample_sample-w1_prompt.md")
	if _, err := os.Stat(scribePrompt); !os.IsNotExist(err) {
		t.Error("scribe prompt should not be generated when Scribe is disabled")
	}
}

func TestRunSession_NilInputReturnsError(t *testing.T) {
	// given: non-dry-run session with nil input should return error early
	cfg := &sightjack.Config{
		Lang:   "en",
		Claude: sightjack.ClaudeConfig{Command: "claude", TimeoutSec: 60},
		Scan:   sightjack.ScanConfig{MaxConcurrency: 1, ChunkSize: 50},
		Linear: sightjack.LinearConfig{Team: "ENG", Project: "Test"},
	}

	// when
	err := sightjack.RunSession(context.Background(), cfg, t.TempDir(), "test-nil-input", false, nil, io.Discard, sightjack.NewLogger(io.Discard, false))

	// then: should get an input-related error, not a panic or scan error
	if err == nil {
		t.Fatal("expected error for nil input in non-dry-run mode")
	}
	if !strings.Contains(err.Error(), "input") {
		t.Errorf("expected input-related error, got: %v", err)
	}
}

func TestBuildCompletedWaveMap(t *testing.T) {
	waves := []sightjack.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Status: "completed"},
		{ID: "auth-w2", ClusterName: "Auth", Status: "available"},
		{ID: "api-w1", ClusterName: "API", Status: "completed"},
	}

	completed := sightjack.BuildCompletedWaveMap(waves)
	if len(completed) != 2 {
		t.Fatalf("expected 2 completed, got %d", len(completed))
	}
	if !completed["Auth:auth-w1"] {
		t.Error("expected Auth:auth-w1 completed")
	}
	if completed["Auth:auth-w2"] {
		t.Error("Auth:auth-w2 should not be completed")
	}
	if !completed["API:api-w1"] {
		t.Error("expected API:api-w1 completed")
	}
}

func TestBuildWaveStates(t *testing.T) {
	waves := []sightjack.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed", Prerequisites: nil, Actions: make([]sightjack.WaveAction, 3)},
		{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "available", Prerequisites: []string{"auth-w1"}, Actions: make([]sightjack.WaveAction, 5)},
	}

	states := sightjack.BuildWaveStates(waves)
	if len(states) != 2 {
		t.Fatalf("expected 2, got %d", len(states))
	}
	if states[0].ActionCount != 3 {
		t.Errorf("expected 3 actions, got %d", states[0].ActionCount)
	}
	if states[1].Prerequisites[0] != "auth-w1" {
		t.Errorf("expected prerequisite auth-w1")
	}
}

func TestDiscussBranchReturnsToApproval(t *testing.T) {
	// This tests the session-level logic: after a discuss round,
	// the approval loop should re-prompt (not exit).
	// We verify this indirectly through PromptWaveApproval behavior:
	// input "d\n" followed by topic, then "a\n" should eventually approve.

	// given: piped input sequence: select wave 1, discuss, enter topic, then approve
	waves := []sightjack.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps",
			Actions: []sightjack.WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
			Delta:   sightjack.WaveDelta{Before: 0.25, After: 0.40}},
	}
	input := "1\nd\nShould we split?\na\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when: selection
	selected, err := sightjack.PromptWaveSelection(ctx, &output, scanner, waves)
	if err != nil {
		t.Fatalf("selection error: %v", err)
	}
	if selected.ID != "auth-w1" {
		t.Fatalf("expected auth-w1, got %s", selected.ID)
	}

	// when: first approval -> discuss
	choice, err := sightjack.PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("first approval error: %v", err)
	}
	if choice != sightjack.ApprovalDiscuss {
		t.Fatalf("expected ApprovalDiscuss, got %d", choice)
	}

	// when: topic input
	topic, err := sightjack.PromptDiscussTopic(ctx, &output, scanner)
	if err != nil {
		t.Fatalf("topic error: %v", err)
	}
	if topic != "Should we split?" {
		t.Errorf("expected topic, got: %s", topic)
	}

	// when: second approval -> approve
	choice, err = sightjack.PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("second approval error: %v", err)
	}
	if choice != sightjack.ApprovalApprove {
		t.Errorf("expected ApprovalApprove after discuss, got %d", choice)
	}
}

func TestBuildCompletedWaveMap_Empty(t *testing.T) {
	// given: nil and empty wave slices
	tests := []struct {
		name  string
		waves []sightjack.Wave
	}{
		{"nil", nil},
		{"empty", []sightjack.Wave{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			completed := sightjack.BuildCompletedWaveMap(tt.waves)

			// then: should return non-nil empty map (callers do completed[key] lookups)
			if completed == nil {
				t.Fatal("expected non-nil map for empty input")
			}
			if len(completed) != 0 {
				t.Errorf("expected empty map, got %d entries", len(completed))
			}
		})
	}
}

func TestBuildCompletedWaveMap_DuplicateIDsAcrossClusters(t *testing.T) {
	// given: same wave ID "w1" in two different clusters, both completed
	waves := []sightjack.Wave{
		{ID: "w1", ClusterName: "Auth", Status: "completed"},
		{ID: "w1", ClusterName: "API", Status: "completed"},
	}

	// when
	completed := sightjack.BuildCompletedWaveMap(waves)

	// then: composite keys should be distinct
	if len(completed) != 2 {
		t.Fatalf("expected 2 entries (distinct composite keys), got %d", len(completed))
	}
	if !completed["Auth:w1"] {
		t.Error("expected Auth:w1 to be completed")
	}
	if !completed["API:w1"] {
		t.Error("expected API:w1 to be completed")
	}
}

func TestBuildWaveStates_Empty(t *testing.T) {
	// given: nil and empty wave slices
	tests := []struct {
		name  string
		waves []sightjack.Wave
	}{
		{"nil", nil},
		{"empty", []sightjack.Wave{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			states := sightjack.BuildWaveStates(tt.waves)

			// then: make([]WaveState, 0) returns non-nil empty slice
			if states == nil {
				t.Fatal("expected non-nil slice for empty input")
			}
			if len(states) != 0 {
				t.Errorf("expected empty slice, got %d entries", len(states))
			}
		})
	}
}

func TestDiscussBranchThenReject(t *testing.T) {
	// given: piped input: select wave 1, discuss, enter topic, then reject
	waves := []sightjack.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps",
			Actions: []sightjack.WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
			Delta:   sightjack.WaveDelta{Before: 0.25, After: 0.40}},
	}
	input := "1\nd\nShould we split?\nr\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when: selection
	selected, err := sightjack.PromptWaveSelection(ctx, &output, scanner, waves)
	if err != nil {
		t.Fatalf("selection error: %v", err)
	}

	// when: first approval -> discuss
	choice, err := sightjack.PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("first approval error: %v", err)
	}
	if choice != sightjack.ApprovalDiscuss {
		t.Fatalf("expected ApprovalDiscuss, got %d", choice)
	}

	// when: topic input
	topic, err := sightjack.PromptDiscussTopic(ctx, &output, scanner)
	if err != nil {
		t.Fatalf("topic error: %v", err)
	}
	if topic != "Should we split?" {
		t.Errorf("expected topic, got: %s", topic)
	}

	// when: second approval -> reject
	choice, err = sightjack.PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("second approval error: %v", err)
	}
	if choice != sightjack.ApprovalReject {
		t.Errorf("expected ApprovalReject after discuss, got %d", choice)
	}
}

func TestDiscussBranchQuitAtTopic(t *testing.T) {
	// given: piped input: select wave 1, discuss, then quit at topic prompt
	waves := []sightjack.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps",
			Actions: []sightjack.WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
			Delta:   sightjack.WaveDelta{Before: 0.25, After: 0.40}},
	}
	input := "1\nd\nq\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when: selection
	selected, err := sightjack.PromptWaveSelection(ctx, &output, scanner, waves)
	if err != nil {
		t.Fatalf("selection error: %v", err)
	}

	// when: approval -> discuss
	choice, err := sightjack.PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("approval error: %v", err)
	}
	if choice != sightjack.ApprovalDiscuss {
		t.Fatalf("expected ApprovalDiscuss, got %d", choice)
	}

	// when: topic -> quit
	_, err = sightjack.PromptDiscussTopic(ctx, &output, scanner)
	if err != sightjack.ErrQuit {
		t.Errorf("expected ErrQuit when quitting at topic, got %v", err)
	}
}

func TestMultipleDiscussRounds(t *testing.T) {
	// given: two discuss rounds then approve
	waves := []sightjack.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps",
			Actions: []sightjack.WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
			Delta:   sightjack.WaveDelta{Before: 0.25, After: 0.40}},
	}
	input := "1\nd\nFirst topic\nd\nSecond topic\na\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when: selection
	selected, err := sightjack.PromptWaveSelection(ctx, &output, scanner, waves)
	if err != nil {
		t.Fatalf("selection error: %v", err)
	}

	// Round 1: discuss
	choice, err := sightjack.PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("round 1 approval error: %v", err)
	}
	if choice != sightjack.ApprovalDiscuss {
		t.Fatalf("round 1: expected ApprovalDiscuss, got %d", choice)
	}
	topic, err := sightjack.PromptDiscussTopic(ctx, &output, scanner)
	if err != nil {
		t.Fatalf("round 1 topic error: %v", err)
	}
	if topic != "First topic" {
		t.Errorf("round 1: expected 'First topic', got: %s", topic)
	}

	// Round 2: discuss again
	choice, err = sightjack.PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("round 2 approval error: %v", err)
	}
	if choice != sightjack.ApprovalDiscuss {
		t.Fatalf("round 2: expected ApprovalDiscuss, got %d", choice)
	}
	topic, err = sightjack.PromptDiscussTopic(ctx, &output, scanner)
	if err != nil {
		t.Fatalf("round 2 topic error: %v", err)
	}
	if topic != "Second topic" {
		t.Errorf("round 2: expected 'Second topic', got: %s", topic)
	}

	// Final: approve
	choice, err = sightjack.PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("final approval error: %v", err)
	}
	if choice != sightjack.ApprovalApprove {
		t.Errorf("expected ApprovalApprove after two discuss rounds, got %d", choice)
	}
}

func TestApplyModifiedWave_PreservesIdentity(t *testing.T) {
	// given: original wave with known identity
	original := sightjack.Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Original Title",
		Actions:     []sightjack.WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "original"}},
		Delta:       sightjack.WaveDelta{Before: 0.25, After: 0.40},
		Status:      "available",
	}
	// given: architect returns modified wave with CHANGED identity fields
	modified := sightjack.Wave{
		ID:          "new-w1",
		ClusterName: "Authentication",
		Title:       "Better Title",
		Actions: []sightjack.WaveAction{
			{Type: "add_dependency", IssueID: "ENG-101", Description: "original"},
			{Type: "add_dod", IssueID: "ENG-101", Description: "new action"},
		},
		Delta:  sightjack.WaveDelta{Before: 0.25, After: 0.50},
		Status: "modified",
	}

	// when: no prerequisites, empty completed map
	result := sightjack.ApplyModifiedWave(original, modified, map[string]bool{})

	// then: identity preserved from original
	if result.ID != "auth-w1" {
		t.Errorf("expected original ID 'auth-w1', got '%s'", result.ID)
	}
	if result.ClusterName != "Auth" {
		t.Errorf("expected original ClusterName 'Auth', got '%s'", result.ClusterName)
	}

	// then: content taken from modified
	if result.Title != "Better Title" {
		t.Errorf("expected modified title, got '%s'", result.Title)
	}
	if len(result.Actions) != 2 {
		t.Errorf("expected 2 modified actions, got %d", len(result.Actions))
	}
	if result.Delta.After != 0.50 {
		t.Errorf("expected modified delta after 0.50, got %f", result.Delta.After)
	}
	if result.Status != "available" {
		t.Errorf("expected status 'available' (no unmet prereqs), got '%s'", result.Status)
	}
}

func TestApplyModifiedWave_LocksOnUnmetPrerequisites(t *testing.T) {
	// given: original available wave with no prerequisites
	original := sightjack.Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Original",
		Status:      "available",
	}
	// given: architect adds a prerequisite that hasn't been completed
	modified := sightjack.Wave{
		ID:            "auth-w1",
		ClusterName:   "Auth",
		Title:         "Modified",
		Prerequisites: []string{"API:api-w1"},
		Actions:       []sightjack.WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "new"}},
	}
	// given: api-w1 is NOT in the completed map
	completed := map[string]bool{}

	// when
	result := sightjack.ApplyModifiedWave(original, modified, completed)

	// then: status should be "locked" because prerequisite is unmet
	if result.Status != "locked" {
		t.Errorf("expected 'locked' for unmet prerequisites, got '%s'", result.Status)
	}
	if len(result.Prerequisites) != 1 || result.Prerequisites[0] != "API:api-w1" {
		t.Errorf("expected prerequisites from modified wave, got %v", result.Prerequisites)
	}
}

func TestApplyModifiedWave_AvailableWhenPrereqsMet(t *testing.T) {
	// given: architect adds a prerequisite that HAS been completed
	original := sightjack.Wave{
		ID:          "auth-w2",
		ClusterName: "Auth",
		Title:       "Original",
		Status:      "available",
	}
	modified := sightjack.Wave{
		ID:            "auth-w2",
		ClusterName:   "Auth",
		Title:         "Modified",
		Prerequisites: []string{"Auth:auth-w1"},
		Actions:       []sightjack.WaveAction{{Type: "add_dod", IssueID: "ENG-102", Description: "new"}},
	}
	completed := map[string]bool{"Auth:auth-w1": true}

	// when
	result := sightjack.ApplyModifiedWave(original, modified, completed)

	// then: status should remain "available" because prerequisite is met
	if result.Status != "available" {
		t.Errorf("expected 'available' for met prerequisites, got '%s'", result.Status)
	}
}

func TestApplyModifiedWave_NormalizesBarePrerequisites(t *testing.T) {
	// given: architect returns bare ID "auth-w1" instead of composite "Auth:auth-w1"
	original := sightjack.Wave{
		ID:          "auth-w2",
		ClusterName: "Auth",
		Title:       "Original",
		Status:      "available",
	}
	modified := sightjack.Wave{
		ID:            "auth-w2",
		ClusterName:   "Auth",
		Title:         "Modified",
		Prerequisites: []string{"auth-w1"}, // bare ID, not composite
		Actions:       []sightjack.WaveAction{{Type: "add_dod", IssueID: "ENG-102", Description: "new"}},
	}
	// given: "Auth:auth-w1" IS completed (composite key)
	completed := map[string]bool{"Auth:auth-w1": true}

	// when
	result := sightjack.ApplyModifiedWave(original, modified, completed)

	// then: should be "available" because bare "auth-w1" normalizes to "Auth:auth-w1"
	if result.Status != "available" {
		t.Errorf("expected 'available' after normalizing bare prereq, got '%s'", result.Status)
	}
	// then: prerequisites should be stored in normalized composite form
	if len(result.Prerequisites) != 1 || result.Prerequisites[0] != "Auth:auth-w1" {
		t.Errorf("expected normalized prereq 'Auth:auth-w1', got %v", result.Prerequisites)
	}
}

func TestApplyModifiedWave_PropagatesLockToWaves(t *testing.T) {
	// given: a waves slice and an architect-modified wave that becomes locked
	waves := []sightjack.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Wave 1", Status: "available"},
		{ID: "auth-w2", ClusterName: "Auth", Title: "Wave 2", Status: "available"},
	}
	modified := sightjack.Wave{
		ID:            "auth-w1",
		ClusterName:   "Auth",
		Title:         "Modified",
		Prerequisites: []string{"API:api-w1"}, // unmet
		Actions:       []sightjack.WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "new"}},
	}
	completed := map[string]bool{}

	// when: apply modification
	result := sightjack.ApplyModifiedWave(waves[0], modified, completed)
	if result.Status != "locked" {
		t.Fatalf("precondition: expected locked, got %s", result.Status)
	}

	// when: propagate back to waves slice
	sightjack.PropagateWaveUpdate(waves, result)

	// then: the waves slice entry should be updated
	if waves[0].Status != "locked" {
		t.Errorf("expected waves[0] to be locked, got '%s'", waves[0].Status)
	}
	if waves[0].Title != "Modified" {
		t.Errorf("expected waves[0] title to be updated, got '%s'", waves[0].Title)
	}
	// other waves unaffected
	if waves[1].Status != "available" {
		t.Errorf("expected waves[1] unchanged, got '%s'", waves[1].Status)
	}
}

func TestApplyModifiedWave_PreservesOriginalPrerequisitesWhenNil(t *testing.T) {
	// given: original wave has prerequisites, modified wave omits them (nil from JSON)
	original := sightjack.Wave{
		ID:            "auth-w2",
		ClusterName:   "Auth",
		Title:         "Original",
		Status:        "locked",
		Prerequisites: []string{"Auth:auth-w1"},
		Delta:         sightjack.WaveDelta{Before: 0.20, After: 0.40},
	}
	modified := sightjack.Wave{
		Title:         "Modified Title",
		Prerequisites: nil, // architect omitted the field
		Actions:       []sightjack.WaveAction{{Type: "add_dod", IssueID: "ENG-102", Description: "new"}},
	}
	completed := map[string]bool{} // auth-w1 NOT completed

	// when
	result := sightjack.ApplyModifiedWave(original, modified, completed)

	// then: prerequisites should fall back to original, not be empty
	if len(result.Prerequisites) != 1 || result.Prerequisites[0] != "Auth:auth-w1" {
		t.Errorf("expected original prereqs preserved, got %v", result.Prerequisites)
	}
	// then: wave should be locked because auth-w1 is not completed
	if result.Status != "locked" {
		t.Errorf("expected 'locked' with unmet original prereqs, got '%s'", result.Status)
	}
}

func TestApplyModifiedWave_PreservesOriginalDeltaWhenZero(t *testing.T) {
	// given: original wave has meaningful delta, modified wave omits it (zero value from JSON)
	original := sightjack.Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Original",
		Status:      "available",
		Delta:       sightjack.WaveDelta{Before: 0.25, After: 0.50},
	}
	modified := sightjack.Wave{
		Title:   "Modified Title",
		Actions: []sightjack.WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "new"}},
		Delta:   sightjack.WaveDelta{}, // zero value — architect omitted the field
	}
	completed := map[string]bool{}

	// when
	result := sightjack.ApplyModifiedWave(original, modified, completed)

	// then: delta should fall back to original
	if result.Delta.Before != 0.25 || result.Delta.After != 0.50 {
		t.Errorf("expected original delta {0.25, 0.50}, got {%v, %v}", result.Delta.Before, result.Delta.After)
	}
}

func TestMergeCompletedStatus_PreservesCompleted(t *testing.T) {
	// given: old completed waves
	oldCompleted := map[string]bool{
		"Auth:auth-w1": true,
		"API:api-w1":   true,
	}
	// given: new waves from re-scan (auth-w1 still exists, api-w2 is new)
	newWaves := []sightjack.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "available"},
		{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "locked"},
		{ID: "api-w2", ClusterName: "API", Title: "New Wave", Status: "available"},
	}

	// when
	merged := sightjack.MergeCompletedStatus(oldCompleted, newWaves)

	// then: auth-w1 should be completed (was in old)
	for _, w := range merged {
		if sightjack.WaveKey(w) == "Auth:auth-w1" && w.Status != "completed" {
			t.Errorf("expected Auth:auth-w1 completed, got %s", w.Status)
		}
	}
	// then: api-w1 not in new waves (dropped from Linear) — not present at all
	for _, w := range merged {
		if sightjack.WaveKey(w) == "API:api-w1" {
			t.Error("API:api-w1 should not appear in merged result")
		}
	}
	// then: auth-w2 and api-w2 keep original status
	for _, w := range merged {
		if sightjack.WaveKey(w) == "Auth:auth-w2" && w.Status != "locked" {
			t.Errorf("expected Auth:auth-w2 locked, got %s", w.Status)
		}
		if sightjack.WaveKey(w) == "API:api-w2" && w.Status != "available" {
			t.Errorf("expected API:api-w2 available, got %s", w.Status)
		}
	}
}

func TestMergeCompletedStatus_EmptyOld(t *testing.T) {
	// given: no old completed waves
	oldCompleted := map[string]bool{}
	newWaves := []sightjack.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Status: "available"},
	}

	// when
	merged := sightjack.MergeCompletedStatus(oldCompleted, newWaves)

	// then: all waves keep original status
	if len(merged) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(merged))
	}
	if merged[0].Status != "available" {
		t.Errorf("expected available, got %s", merged[0].Status)
	}
}

func TestMergeCompletedStatus_EmptyNew(t *testing.T) {
	// given: old waves completed but new scan returns nothing
	oldCompleted := map[string]bool{"Auth:auth-w1": true}
	var newWaves []sightjack.Wave

	// when
	merged := sightjack.MergeCompletedStatus(oldCompleted, newWaves)

	// then
	if len(merged) != 0 {
		t.Errorf("expected 0 waves, got %d", len(merged))
	}
}

func TestBuildWaveStates_IncludesFullFields(t *testing.T) {
	// given
	waves := []sightjack.Wave{
		{
			ID:            "auth-w1",
			ClusterName:   "Auth",
			Title:         "Deps",
			Status:        "completed",
			Prerequisites: []string{"Auth:auth-w0"},
			Actions: []sightjack.WaveAction{
				{Type: "add_dependency", IssueID: "ENG-101", Description: "dep"},
				{Type: "add_dod", IssueID: "ENG-102", Description: "dod"},
			},
			Description: "Order dependencies first",
			Delta:       sightjack.WaveDelta{Before: 0.20, After: 0.40},
		},
	}

	// when
	states := sightjack.BuildWaveStates(waves)

	// then
	s := states[0]
	if len(s.Actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(s.Actions))
	}
	if s.Description != "Order dependencies first" {
		t.Errorf("expected description, got %s", s.Description)
	}
	if s.Delta.Before != 0.20 || s.Delta.After != 0.40 {
		t.Errorf("expected delta {0.20, 0.40}, got {%v, %v}", s.Delta.Before, s.Delta.After)
	}
}

func TestRestoreWaves_ConvertsWaveStatesToWaves(t *testing.T) {
	// given
	states := []sightjack.WaveState{
		{
			ID:            "auth-w1",
			ClusterName:   "Auth",
			Title:         "Deps",
			Status:        "completed",
			Prerequisites: []string{"Auth:auth-w0"},
			ActionCount:   2,
			Actions: []sightjack.WaveAction{
				{Type: "add_dependency", IssueID: "ENG-101", Description: "dep"},
				{Type: "add_dod", IssueID: "ENG-102", Description: "dod"},
			},
			Description: "Order dependencies first",
			Delta:       sightjack.WaveDelta{Before: 0.20, After: 0.40},
		},
		{
			ID:          "auth-w2",
			ClusterName: "Auth",
			Title:       "DoD",
			Status:      "available",
			ActionCount: 1,
			Actions:     []sightjack.WaveAction{{Type: "add_dod", IssueID: "ENG-103", Description: "dod2"}},
			Delta:       sightjack.WaveDelta{Before: 0.40, After: 0.60},
		},
	}

	// when
	waves := sightjack.RestoreWaves(states)

	// then
	if len(waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(waves))
	}
	w := waves[0]
	if w.ID != "auth-w1" {
		t.Errorf("expected auth-w1, got %s", w.ID)
	}
	if w.ClusterName != "Auth" {
		t.Errorf("expected Auth, got %s", w.ClusterName)
	}
	if w.Status != "completed" {
		t.Errorf("expected completed, got %s", w.Status)
	}
	if len(w.Actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(w.Actions))
	}
	if w.Description != "Order dependencies first" {
		t.Errorf("expected description, got %s", w.Description)
	}
	if w.Delta.Before != 0.20 {
		t.Errorf("expected delta before 0.20, got %v", w.Delta.Before)
	}
}

func TestRestoreWaves_EmptyInput(t *testing.T) {
	// given
	var states []sightjack.WaveState

	// when
	waves := sightjack.RestoreWaves(states)

	// then
	if waves == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(waves) != 0 {
		t.Errorf("expected empty slice, got %d", len(waves))
	}
}

func TestRunSession_DryRunDoesNotCacheScanResult(t *testing.T) {
	// given: dry-run should NOT write scan_result.json (no real scan happened)
	baseDir := t.TempDir()
	cfg := &sightjack.Config{
		Lang:   "en",
		Claude: sightjack.ClaudeConfig{Command: "claude", TimeoutSec: 60},
		Scan:   sightjack.ScanConfig{MaxConcurrency: 1, ChunkSize: 50},
		Linear: sightjack.LinearConfig{Team: "ENG", Project: "Test"},
		Scribe: sightjack.ScribeConfig{Enabled: true},
	}
	sessionID := "test-no-cache"
	ctx := context.Background()

	// when
	err := sightjack.RunSession(ctx, cfg, baseDir, sessionID, true, nil, io.Discard, sightjack.NewLogger(io.Discard, false))

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	scanDir := sightjack.ScanDir(baseDir, sessionID)
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	if _, err := os.Stat(scanResultPath); !os.IsNotExist(err) {
		t.Error("scan_result.json should not exist in dry-run mode")
	}
}

func TestCalcNewlyUnlocked_CompletingWaveUnlocksOne(t *testing.T) {
	// given: 1 available wave (the one being completed), completing it unlocks 1 new wave
	// oldAvailable = 1 (includes the completing wave)
	// After completion: completing wave removed, 1 new wave unlocked → newAvailable = 1
	// Expected: 1 newly unlocked wave
	oldAvailable := 1
	newAvailable := 1

	// when
	got := sightjack.CalcNewlyUnlocked(oldAvailable, newAvailable)

	// then
	if got != 1 {
		t.Errorf("expected 1 newly unlocked wave, got %d", got)
	}
}

func TestCalcNewlyUnlocked_CompletingWaveUnlocksTwo(t *testing.T) {
	// given: 2 available waves, completing one unlocks 2 more
	// oldAvailable = 2, after: 1 remaining + 2 unlocked = 3 → newAvailable = 3
	// Expected: 2 newly unlocked waves
	oldAvailable := 2
	newAvailable := 3

	// when
	got := sightjack.CalcNewlyUnlocked(oldAvailable, newAvailable)

	// then
	if got != 2 {
		t.Errorf("expected 2 newly unlocked waves, got %d", got)
	}
}

func TestCalcNewlyUnlocked_CompletingWaveUnlocksNone(t *testing.T) {
	// given: 3 available waves, completing one unlocks nothing
	// oldAvailable = 3, after: 2 remaining + 0 unlocked = 2 → newAvailable = 2
	// Expected: 0 newly unlocked waves
	oldAvailable := 3
	newAvailable := 2

	// when
	got := sightjack.CalcNewlyUnlocked(oldAvailable, newAvailable)

	// then
	if got != 0 {
		t.Errorf("expected 0 newly unlocked waves, got %d", got)
	}
}

func TestBuildSessionState(t *testing.T) {
	// given
	scanResult := &sightjack.ScanResult{
		Clusters: []sightjack.ClusterScanResult{
			{Name: "Auth", Completeness: 0.50, Issues: make([]sightjack.IssueDetail, 3)},
		},
		Completeness: 0.50,
	}
	waves := []sightjack.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed",
			Actions: []sightjack.WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "d"}},
			Delta:   sightjack.WaveDelta{Before: 0.25, After: 0.50}},
	}
	cfg := &sightjack.Config{Linear: sightjack.LinearConfig{Project: "TestProject"}}
	sessionID := "test-123"
	adrCount := 2

	// when
	state := sightjack.BuildSessionState(cfg, sessionID, scanResult, waves, adrCount, nil)

	// then
	if state.Version != sightjack.StateFormatVersion {
		t.Errorf("expected version %s, got %s", sightjack.StateFormatVersion, state.Version)
	}
	if state.SessionID != "test-123" {
		t.Errorf("expected test-123, got %s", state.SessionID)
	}
	if state.Completeness != 0.50 {
		t.Errorf("expected 0.50, got %f", state.Completeness)
	}
	if state.ADRCount != 2 {
		t.Errorf("expected 2, got %d", state.ADRCount)
	}
	if len(state.Clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(state.Clusters))
	}
	if state.Clusters[0].Name != "Auth" {
		t.Errorf("expected cluster name Auth, got %s", state.Clusters[0].Name)
	}
	if state.Clusters[0].Completeness != 0.50 {
		t.Errorf("expected cluster completeness 0.50, got %f", state.Clusters[0].Completeness)
	}
	if state.Clusters[0].IssueCount != 3 {
		t.Errorf("expected issue count 3, got %d", state.Clusters[0].IssueCount)
	}
	if len(state.Waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(state.Waves))
	}
	if state.Waves[0].ID != "auth-w1" {
		t.Errorf("expected wave ID auth-w1, got %s", state.Waves[0].ID)
	}
	if state.Project != "TestProject" {
		t.Errorf("expected project TestProject, got %s", state.Project)
	}
}

func TestBuildSessionState_PreservesLastScanned(t *testing.T) {
	// given: a specific lastScanned time (simulating resume)
	scanResult := &sightjack.ScanResult{
		Clusters:     []sightjack.ClusterScanResult{{Name: "Auth", Completeness: 0.50, Issues: make([]sightjack.IssueDetail, 1)}},
		Completeness: 0.50,
	}
	waves := []sightjack.Wave{{ID: "w1", ClusterName: "Auth", Status: "available"}}
	cfg := &sightjack.Config{Linear: sightjack.LinearConfig{Project: "P"}}
	originalScanTime := time.Date(2026, 2, 17, 15, 30, 0, 0, time.UTC)

	// when: BuildSessionState is called with a prior lastScanned
	state := sightjack.BuildSessionState(cfg, "s1", scanResult, waves, 0, &originalScanTime)

	// then: LastScanned should be the original, not time.Now()
	if !state.LastScanned.Equal(originalScanTime) {
		t.Errorf("expected LastScanned %v, got %v", originalScanTime, state.LastScanned)
	}
}

func TestBuildSessionState_NilLastScannedUsesNow(t *testing.T) {
	// given
	scanResult := &sightjack.ScanResult{Completeness: 0.50}
	cfg := &sightjack.Config{Linear: sightjack.LinearConfig{Project: "P"}}
	before := time.Now()

	// when: nil lastScanned means fresh session
	state := sightjack.BuildSessionState(cfg, "s1", scanResult, nil, 0, nil)

	// then: LastScanned should be approximately now
	if state.LastScanned.Before(before) {
		t.Errorf("expected LastScanned >= %v, got %v", before, state.LastScanned)
	}
}

func TestBuildSessionState_ShibitoCount(t *testing.T) {
	// given
	scanResult := &sightjack.ScanResult{
		Clusters:     []sightjack.ClusterScanResult{{Name: "Auth", Completeness: 0.50, Issues: make([]sightjack.IssueDetail, 1)}},
		Completeness: 0.50,
		ShibitoWarnings: []sightjack.ShibitoWarning{
			{ClosedIssueID: "ENG-50", CurrentIssueID: "ENG-201", Description: "Login pattern", RiskLevel: "high"},
			{ClosedIssueID: "ENG-30", CurrentIssueID: "ENG-180", Description: "Caching", RiskLevel: "medium"},
		},
	}
	cfg := &sightjack.Config{Linear: sightjack.LinearConfig{Project: "P"}}

	// when
	state := sightjack.BuildSessionState(cfg, "s1", scanResult, nil, 0, nil)

	// then
	if state.ShibitoCount != 2 {
		t.Errorf("expected ShibitoCount 2, got %d", state.ShibitoCount)
	}
}

func TestBuildSessionState_ShibitoCountZero(t *testing.T) {
	// given: no shibito warnings
	scanResult := &sightjack.ScanResult{Completeness: 0.50}
	cfg := &sightjack.Config{Linear: sightjack.LinearConfig{Project: "P"}}

	// when
	state := sightjack.BuildSessionState(cfg, "s1", scanResult, nil, 0, nil)

	// then
	if state.ShibitoCount != 0 {
		t.Errorf("expected ShibitoCount 0, got %d", state.ShibitoCount)
	}
}

func TestBuildSessionState_StrictnessLevel(t *testing.T) {
	// given: config with alert strictness
	scanResult := &sightjack.ScanResult{Completeness: 0.50}
	cfg := &sightjack.Config{
		Linear:     sightjack.LinearConfig{Project: "P"},
		Strictness: sightjack.StrictnessConfig{Default: sightjack.StrictnessAlert},
	}

	// when
	state := sightjack.BuildSessionState(cfg, "s1", scanResult, nil, 0, nil)

	// then: state should capture the configured strictness level
	if state.StrictnessLevel != "alert" {
		t.Errorf("expected StrictnessLevel 'alert', got %q", state.StrictnessLevel)
	}
}

func TestBuildSessionState_StrictnessLevelDefault(t *testing.T) {
	// given: config with default (fog) strictness
	scanResult := &sightjack.ScanResult{Completeness: 0.50}
	cfg := &sightjack.Config{
		Linear:     sightjack.LinearConfig{Project: "P"},
		Strictness: sightjack.StrictnessConfig{Default: sightjack.StrictnessFog},
	}

	// when
	state := sightjack.BuildSessionState(cfg, "s1", scanResult, nil, 0, nil)

	// then
	if state.StrictnessLevel != "fog" {
		t.Errorf("expected StrictnessLevel 'fog', got %q", state.StrictnessLevel)
	}
}

func TestApplyModifiedWave_PreservesOriginalActionsWhenNil(t *testing.T) {
	// given: original wave has actions, modified wave omits them (nil from JSON)
	originalActions := []sightjack.WaveAction{
		{Type: "add_dod", IssueID: "ENG-101", Description: "Original action 1"},
		{Type: "add_dependency", IssueID: "ENG-102", Description: "Original action 2"},
	}
	original := sightjack.Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Original",
		Status:      "available",
		Actions:     originalActions,
		Delta:       sightjack.WaveDelta{Before: 0.20, After: 0.40},
	}
	modified := sightjack.Wave{
		Title:   "Modified Title",
		Actions: nil, // architect omitted the field
	}
	completed := map[string]bool{}

	// when
	result := sightjack.ApplyModifiedWave(original, modified, completed)

	// then: actions should fall back to original
	if len(result.Actions) != 2 {
		t.Fatalf("expected 2 original actions preserved, got %d", len(result.Actions))
	}
	if result.Actions[0].IssueID != "ENG-101" {
		t.Errorf("expected first action ENG-101, got %s", result.Actions[0].IssueID)
	}
}

func TestResumeSession_RestoresWavesFromState(t *testing.T) {
	// given: a saved state with completed and available waves + cached scan result
	baseDir := t.TempDir()

	// Create scan result cache
	scanDir := sightjack.ScanDir(baseDir, "old-session")
	os.MkdirAll(scanDir, 0755)
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	scanResult := &sightjack.ScanResult{
		Clusters: []sightjack.ClusterScanResult{
			{Name: "Auth", Completeness: 0.50, Issues: []sightjack.IssueDetail{
				{ID: "ENG-101", Identifier: "ENG-101", Title: "Login", Completeness: 0.50},
			}},
		},
		TotalIssues:  1,
		Completeness: 0.50,
	}
	if err := sightjack.WriteScanResult(scanResultPath, scanResult); err != nil {
		t.Fatalf("write scan result: %v", err)
	}

	// Create state pointing to that scan result
	state := &sightjack.SessionState{
		Version:        "0.5",
		SessionID:      "old-session",
		Project:        "TestProject",
		LastScanned:    time.Now(),
		Completeness:   0.50,
		ScanResultPath: scanResultPath,
		Clusters: []sightjack.ClusterState{
			{Name: "Auth", Completeness: 0.50, IssueCount: 1},
		},
		Waves: []sightjack.WaveState{
			{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed",
				ActionCount: 1,
				Actions:     []sightjack.WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "d"}},
				Delta:       sightjack.WaveDelta{Before: 0.25, After: 0.50}},
			{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "available",
				ActionCount: 1,
				Actions:     []sightjack.WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "d2"}},
				Delta:       sightjack.WaveDelta{Before: 0.50, After: 0.75}},
		},
		ADRCount: 2,
	}
	if err := sightjack.WriteState(baseDir, state); err != nil {
		t.Fatalf("write state: %v", err)
	}

	// when: ResumeSession loads state and returns waves + scan result
	resumedScanResult, waves, completed, adrCount, err := sightjack.ResumeSession(baseDir, state)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(waves))
	}
	if waves[0].Status != "completed" {
		t.Errorf("expected auth-w1 completed, got %s", waves[0].Status)
	}
	if !completed["Auth:auth-w1"] {
		t.Error("expected Auth:auth-w1 in completed map")
	}
	if resumedScanResult.Completeness != 0.50 {
		t.Errorf("expected completeness 0.50, got %f", resumedScanResult.Completeness)
	}
	if adrCount != 0 {
		t.Errorf("expected adrCount 0 (no ADR files on disk), got %d", adrCount)
	}
}

func TestRunResumeSession_NilInputReturnsError(t *testing.T) {
	// given: nil input should return error
	cfg := &sightjack.Config{
		Lang:   "en",
		Claude: sightjack.ClaudeConfig{Command: "claude", TimeoutSec: 60},
		Linear: sightjack.LinearConfig{Team: "ENG", Project: "Test"},
	}
	state := &sightjack.SessionState{
		Version:        "0.5",
		SessionID:      "old-session",
		ScanResultPath: "/some/path.json",
	}

	// when
	err := sightjack.RunResumeSession(context.Background(), cfg, t.TempDir(), state, nil, io.Discard, sightjack.NewLogger(io.Discard, false))

	// then
	if err == nil {
		t.Fatal("expected error for nil input")
	}
	if !strings.Contains(err.Error(), "input") {
		t.Errorf("expected input-related error, got: %v", err)
	}
}

func TestRunRescanSession_NilInputReturnsError(t *testing.T) {
	// given: nil input should return error
	cfg := &sightjack.Config{
		Lang:   "en",
		Claude: sightjack.ClaudeConfig{Command: "claude", TimeoutSec: 60},
		Linear: sightjack.LinearConfig{Team: "ENG", Project: "Test"},
	}
	state := &sightjack.SessionState{
		Version:   "0.5",
		SessionID: "old-session",
	}

	// when
	err := sightjack.RunRescanSession(context.Background(), cfg, t.TempDir(), state, nil, io.Discard, sightjack.NewLogger(io.Discard, false))

	// then
	if err == nil {
		t.Fatal("expected error for nil input")
	}
	if !strings.Contains(err.Error(), "input") {
		t.Errorf("expected input-related error, got: %v", err)
	}
}

func TestResumeSession_ErrorOnMissingScanResultPath(t *testing.T) {
	// given: state with empty scan result path
	state := &sightjack.SessionState{
		Version:        "0.5",
		SessionID:      "old-session",
		ScanResultPath: "",
	}

	// when
	_, _, _, _, err := sightjack.ResumeSession(t.TempDir(), state)

	// then
	if err == nil {
		t.Fatal("expected error for empty scan result path")
	}
	if !strings.Contains(err.Error(), "no cached scan result path") {
		t.Errorf("expected scan result path error, got: %v", err)
	}
}

func TestResumeSession_ErrorOnMissingScanResultFile(t *testing.T) {
	// given: state with non-existent scan result path
	state := &sightjack.SessionState{
		Version:        "0.5",
		SessionID:      "old-session",
		ScanResultPath: "/nonexistent/scan_result.json",
	}

	// when
	_, _, _, _, err := sightjack.ResumeSession(t.TempDir(), state)

	// then
	if err == nil {
		t.Fatal("expected error for missing scan result file")
	}
	if !strings.Contains(err.Error(), "load cached scan result") {
		t.Errorf("expected load error, got: %v", err)
	}
}

func TestResumeSession_RecomputesADRCountFromFilesystem(t *testing.T) {
	// given: state says ADRCount=2, but filesystem has 3 ADR files
	baseDir := t.TempDir()
	scanDir := filepath.Join(baseDir, ".siren", ".run", "old-session")
	os.MkdirAll(scanDir, 0755)

	scanResult := &sightjack.ScanResult{
		Clusters:     []sightjack.ClusterScanResult{{Name: "Auth", Completeness: 0.50, Issues: []sightjack.IssueDetail{{ID: "E1", Identifier: "E1", Title: "t"}}}},
		TotalIssues:  1,
		Completeness: 0.50,
	}
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	if err := sightjack.WriteScanResult(scanResultPath, scanResult); err != nil {
		t.Fatalf("write scan result: %v", err)
	}

	// Create 3 ADR files on filesystem
	adrDir := sightjack.ADRDir(baseDir)
	os.MkdirAll(adrDir, 0755)
	for _, name := range []string{"0001-first.md", "0002-second.md", "0003-third.md"} {
		os.WriteFile(filepath.Join(adrDir, name), []byte("# ADR"), 0644)
	}

	state := &sightjack.SessionState{
		Version:        "0.5",
		SessionID:      "old-session",
		ScanResultPath: scanResultPath,
		Waves:          []sightjack.WaveState{{ID: "w1", ClusterName: "Auth", Status: "available"}},
		ADRCount:       2, // stale: says 2 but filesystem has 3
	}
	if err := sightjack.WriteState(baseDir, state); err != nil {
		t.Fatalf("write state: %v", err)
	}

	// when
	_, _, _, adrCount, err := sightjack.ResumeSession(baseDir, state)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adrCount != 3 {
		t.Errorf("expected adrCount 3 (from filesystem), got %d", adrCount)
	}
}

func TestCanResume_ValidState(t *testing.T) {
	// given: state with valid ScanResultPath and non-empty Waves
	dir := t.TempDir()
	scanDir := filepath.Join(dir, ".siren", ".run", "s1")
	os.MkdirAll(scanDir, 0755)
	path := filepath.Join(scanDir, "scan_result.json")
	os.WriteFile(path, []byte(`{}`), 0644)

	state := &sightjack.SessionState{
		ScanResultPath: path,
		Waves:          []sightjack.WaveState{{ID: "w1", ClusterName: "auth", Status: "pending"}},
	}

	// when / then
	if !sightjack.CanResume(state) {
		t.Error("expected CanResume true for valid state with waves")
	}
}

func TestCanResume_EmptyWaves(t *testing.T) {
	// given: state with valid ScanResultPath but no waves (recovered state)
	dir := t.TempDir()
	scanDir := filepath.Join(dir, ".siren", ".run", "s1")
	os.MkdirAll(scanDir, 0755)
	path := filepath.Join(scanDir, "scan_result.json")
	os.WriteFile(path, []byte(`{}`), 0644)

	state := &sightjack.SessionState{ScanResultPath: path, Waves: nil}

	// when / then
	if sightjack.CanResume(state) {
		t.Error("expected CanResume false when waves are empty")
	}
}

func TestCanResume_EmptyPath(t *testing.T) {
	// given: state with empty ScanResultPath (fallback to ScanDir)
	state := &sightjack.SessionState{ScanResultPath: ""}

	// when / then
	if sightjack.CanResume(state) {
		t.Error("expected CanResume false for empty path")
	}
}

func TestPartialApplyDelta(t *testing.T) {
	tests := []struct {
		name      string
		applied   int
		total     int
		before    float64
		after     float64
		wantAfter float64
	}{
		{"full success", 5, 5, 0.3, 0.6, 0.6},
		{"partial 3/5", 3, 5, 0.3, 0.6, 0.48},
		{"zero applied", 0, 5, 0.3, 0.6, 0.3},
		{"zero total", 0, 0, 0.3, 0.6, 0.6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			result := &sightjack.WaveApplyResult{Applied: tt.applied, TotalCount: tt.total}
			delta := sightjack.WaveDelta{Before: tt.before, After: tt.after}

			// when
			got := sightjack.PartialApplyDelta(result, delta)

			// then
			if fmt.Sprintf("%.4f", got) != fmt.Sprintf("%.4f", tt.wantAfter) {
				t.Errorf("PartialApplyDelta: got %.4f, want %.4f", got, tt.wantAfter)
			}
		})
	}
}

func TestCheckCompletenessConsistency(t *testing.T) {
	tests := []struct {
		name     string
		overall  float64
		clusters []sightjack.ClusterScanResult
		wantWarn bool
	}{
		{"consistent", 0.5, []sightjack.ClusterScanResult{
			{Name: "a", Completeness: 0.4},
			{Name: "b", Completeness: 0.6},
		}, false},
		{"inconsistent", 0.9, []sightjack.ClusterScanResult{
			{Name: "a", Completeness: 0.4},
			{Name: "b", Completeness: 0.6},
		}, true},
		{"empty clusters", 0.0, nil, false},
		{"within tolerance", 0.54, []sightjack.ClusterScanResult{
			{Name: "a", Completeness: 0.5},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sightjack.CheckCompletenessConsistency(tt.overall, tt.clusters)
			if got != tt.wantWarn {
				t.Errorf("CheckCompletenessConsistency: got %v, want %v", got, tt.wantWarn)
			}
		})
	}
}

func TestRecoverStateFromScan(t *testing.T) {
	// given
	scanResult := &sightjack.ScanResult{
		Clusters: []sightjack.ClusterScanResult{
			{Name: "auth", Completeness: 0.4, Issues: []sightjack.IssueDetail{{ID: "A-1"}}},
			{Name: "infra", Completeness: 0.6, Issues: []sightjack.IssueDetail{{ID: "I-1"}, {ID: "I-2"}}},
		},
		Completeness: 0.5,
	}
	waves := []sightjack.Wave{
		{ID: "w1", ClusterName: "auth", Status: "completed"},
		{ID: "w2", ClusterName: "auth", Status: "available"},
	}

	dir := t.TempDir()
	adrDir := filepath.Join(dir, "docs", "adr")
	os.MkdirAll(adrDir, 0755)
	os.WriteFile(filepath.Join(adrDir, "0001-test.md"), []byte("adr"), 0644)
	os.WriteFile(filepath.Join(adrDir, "0002-test2.md"), []byte("adr2"), 0644)

	// when
	state := sightjack.RecoverStateFromScan(scanResult, waves, adrDir)

	// then
	if state.Completeness != 0.5 {
		t.Errorf("Completeness: expected 0.5, got %f", state.Completeness)
	}
	if len(state.Clusters) != 2 {
		t.Errorf("Clusters: expected 2, got %d", len(state.Clusters))
	}
	if state.Clusters[0].Name != "auth" {
		t.Errorf("Clusters[0].Name: expected auth, got %s", state.Clusters[0].Name)
	}
	if state.Clusters[0].IssueCount != 1 {
		t.Errorf("Clusters[0].IssueCount: expected 1, got %d", state.Clusters[0].IssueCount)
	}
	if state.Clusters[1].IssueCount != 2 {
		t.Errorf("Clusters[1].IssueCount: expected 2, got %d", state.Clusters[1].IssueCount)
	}
	if state.ADRCount != 2 {
		t.Errorf("ADRCount: expected 2, got %d", state.ADRCount)
	}
	if len(state.Waves) != 2 {
		t.Errorf("Waves: expected 2, got %d", len(state.Waves))
	}
	if state.Version != sightjack.StateFormatVersion {
		t.Errorf("Version: expected %s, got %s", sightjack.StateFormatVersion, state.Version)
	}
	if state.ShibitoCount != 0 {
		t.Errorf("ShibitoCount: expected 0, got %d", state.ShibitoCount)
	}
}

func TestRecoverStateFromScanEmpty(t *testing.T) {
	// given
	scanResult := &sightjack.ScanResult{}

	// when
	state := sightjack.RecoverStateFromScan(scanResult, nil, "/nonexistent")

	// then
	if state.Completeness != 0 {
		t.Errorf("Completeness: expected 0, got %f", state.Completeness)
	}
	if len(state.Clusters) != 0 {
		t.Errorf("Clusters: expected 0, got %d", len(state.Clusters))
	}
	if len(state.Waves) != 0 {
		t.Errorf("Waves: expected 0, got %d", len(state.Waves))
	}
	if state.ADRCount != 0 {
		t.Errorf("ADRCount: expected 0, got %d", state.ADRCount)
	}
}

func TestRecoverStateFromScan_ShibitoWarnings(t *testing.T) {
	// given: scan result with shibito warnings
	scanResult := &sightjack.ScanResult{
		Clusters:     []sightjack.ClusterScanResult{{Name: "auth", Completeness: 0.3, Issues: []sightjack.IssueDetail{{ID: "A-1"}}}},
		Completeness: 0.3,
		ShibitoWarnings: []sightjack.ShibitoWarning{
			{ClosedIssueID: "ENG-50", CurrentIssueID: "ENG-201", Description: "Login pattern", RiskLevel: "high"},
			{ClosedIssueID: "ENG-30", CurrentIssueID: "ENG-180", Description: "Caching", RiskLevel: "medium"},
			{ClosedIssueID: "ENG-10", CurrentIssueID: "ENG-100", Description: "Auth flow", RiskLevel: "low"},
		},
	}

	// when
	state := sightjack.RecoverStateFromScan(scanResult, nil, "/nonexistent")

	// then
	if state.ShibitoCount != 3 {
		t.Errorf("ShibitoCount: expected 3, got %d", state.ShibitoCount)
	}
}

func TestCanResume_MissingFile(t *testing.T) {
	// given: state with ScanResultPath pointing to deleted file
	state := &sightjack.SessionState{ScanResultPath: "/nonexistent/scan_result.json"}

	// when / then
	if sightjack.CanResume(state) {
		t.Error("expected CanResume false for missing file")
	}
}

func TestTryRecoverState(t *testing.T) {
	dir := t.TempDir()

	// given: a cached scan result without a state.json
	sessionID := "test-session"
	scanDir, err := sightjack.EnsureScanDir(dir, sessionID)
	if err != nil {
		t.Fatalf("EnsureScanDir: %v", err)
	}
	scanResult := &sightjack.ScanResult{
		Clusters:     []sightjack.ClusterScanResult{{Name: "auth", Completeness: 0.5}},
		Completeness: 0.5,
	}
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	if err := sightjack.WriteScanResult(scanResultPath, scanResult); err != nil {
		t.Fatalf("WriteScanResult: %v", err)
	}

	// when
	recovered, recErr := sightjack.TryRecoverState(dir, sessionID, sightjack.NewLogger(io.Discard, false))

	// then
	if recErr != nil {
		t.Fatalf("TryRecoverState: %v", recErr)
	}
	if recovered == nil {
		t.Fatal("expected recovered state, got nil")
	}
	if recovered.Completeness != 0.5 {
		t.Errorf("Completeness: expected 0.5, got %f", recovered.Completeness)
	}
	if recovered.SessionID != sessionID {
		t.Errorf("SessionID: expected %s, got %s", sessionID, recovered.SessionID)
	}
	if recovered.ScanResultPath != scanResultPath {
		t.Errorf("ScanResultPath: expected %s, got %s", scanResultPath, recovered.ScanResultPath)
	}
}

func TestResumeSession_EvaluateUnlocksAfterRestore(t *testing.T) {
	// given: saved state where auth-w1 is completed and auth-w2 is locked (depends on auth-w1)
	// After restore + EvaluateUnlocks, auth-w2 should become available
	baseDir := t.TempDir()

	scanDir := sightjack.ScanDir(baseDir, "resume-unlock")
	os.MkdirAll(scanDir, 0755)
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	scanResult := &sightjack.ScanResult{
		Clusters:     []sightjack.ClusterScanResult{{Name: "Auth", Completeness: 0.40, Issues: []sightjack.IssueDetail{{ID: "E1", Identifier: "E1", Title: "t"}}}},
		TotalIssues:  1,
		Completeness: 0.40,
	}
	if err := sightjack.WriteScanResult(scanResultPath, scanResult); err != nil {
		t.Fatalf("write scan result: %v", err)
	}

	state := &sightjack.SessionState{
		Version:        sightjack.StateFormatVersion,
		SessionID:      "resume-unlock",
		Project:        "Test",
		ScanResultPath: scanResultPath,
		Completeness:   0.40,
		Clusters:       []sightjack.ClusterState{{Name: "Auth", Completeness: 0.40, IssueCount: 1}},
		Waves: []sightjack.WaveState{
			{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed",
				ActionCount: 1, Actions: []sightjack.WaveAction{{Type: "add_dod", IssueID: "E1", Description: "d"}},
				Delta: sightjack.WaveDelta{Before: 0.20, After: 0.40}},
			{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "locked",
				Prerequisites: []string{"Auth:auth-w1"},
				ActionCount:   1, Actions: []sightjack.WaveAction{{Type: "add_dod", IssueID: "E1", Description: "d2"}},
				Delta: sightjack.WaveDelta{Before: 0.40, After: 0.65}},
		},
	}

	// when: restore waves and evaluate unlocks
	_, waves, completed, _, err := sightjack.ResumeSession(baseDir, state)
	if err != nil {
		t.Fatalf("ResumeSession: %v", err)
	}
	waves = sightjack.EvaluateUnlocks(waves, completed)

	// then: auth-w2 should be unlocked since auth-w1 is completed
	var w2Status string
	for _, w := range waves {
		if w.ID == "auth-w2" {
			w2Status = w.Status
		}
	}
	if w2Status != "available" {
		t.Errorf("expected auth-w2 available after unlock evaluation, got %s", w2Status)
	}
}

func TestMergeCompletedStatus_AllCompleted(t *testing.T) {
	// given: all old waves were completed; rescan generates new waves for the same clusters
	oldCompleted := map[string]bool{
		"Auth:auth-w1": true,
		"Auth:auth-w2": true,
		"API:api-w1":   true,
	}
	// Rescan produces new waves — some match old keys, some are new
	newWaves := []sightjack.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps v2", Status: "available"},
		{ID: "auth-w2", ClusterName: "Auth", Title: "DoD v2", Status: "locked"},
		{ID: "auth-w3", ClusterName: "Auth", Title: "New", Status: "locked"},
		{ID: "api-w1", ClusterName: "API", Title: "Endpoints v2", Status: "available"},
	}

	// when
	merged := sightjack.MergeCompletedStatus(oldCompleted, newWaves)

	// then: auth-w1, auth-w2, api-w1 should be completed (carried over)
	completedCount := 0
	for _, w := range merged {
		if w.Status == "completed" {
			completedCount++
		}
	}
	if completedCount != 3 {
		t.Errorf("expected 3 completed waves after merge, got %d", completedCount)
	}
	// auth-w3 should remain locked (not in old completed)
	for _, w := range merged {
		if w.ID == "auth-w3" && w.Status != "locked" {
			t.Errorf("expected auth-w3 locked, got %s", w.Status)
		}
	}
}

func TestResumeScanDir_DerivedFromScanResultPath(t *testing.T) {
	// given: state with ScanResultPath set
	state := &sightjack.SessionState{
		SessionID:      "old-session",
		ScanResultPath: "/project/.siren/.run/old-session/scan_result.json",
	}

	// when
	got := sightjack.ResumeScanDir(state, "/project")

	// then: should derive scanDir from ScanResultPath
	want := "/project/.siren/.run/old-session"
	if got != want {
		t.Errorf("ResumeScanDir: expected %q, got %q", want, got)
	}
}

func TestResumeScanDir_EmptyScanResultPath_FallsBack(t *testing.T) {
	// given: state with empty ScanResultPath (fallback to ScanDir)
	state := &sightjack.SessionState{
		SessionID:      "new-session",
		ScanResultPath: "",
	}

	// when
	got := sightjack.ResumeScanDir(state, "/project")

	// then: should fall back to ScanDir()
	want := sightjack.ScanDir("/project", "new-session")
	if got != want {
		t.Errorf("ResumeScanDir: expected %q, got %q", want, got)
	}
}

func TestResumeScanDir_CurrentPathFormat(t *testing.T) {
	// given: state with ScanResultPath using current .siren/.run/ format
	state := &sightjack.SessionState{
		SessionID:      "current-session",
		ScanResultPath: "/project/.siren/.run/current-session/scan_result.json",
	}

	// when
	got := sightjack.ResumeScanDir(state, "/project")

	// then: should derive from ScanResultPath
	want := "/project/.siren/.run/current-session"
	if got != want {
		t.Errorf("ResumeScanDir: expected %q, got %q", want, got)
	}
}

func TestRecoverLatestState_PrefersNewest(t *testing.T) {
	// given: two sessions in .run/, should pick the newest
	dir := t.TempDir()

	// Older session
	oldID := "session-1000-1"
	oldDir := filepath.Join(dir, ".siren", ".run", oldID)
	os.MkdirAll(oldDir, 0755)
	sightjack.WriteScanResult(filepath.Join(oldDir, "scan_result.json"), &sightjack.ScanResult{
		Clusters:     []sightjack.ClusterScanResult{{Name: "old", Completeness: 0.3}},
		Completeness: 0.3,
	})

	// Newer session
	newID := "session-2000-1"
	newDir := filepath.Join(dir, ".siren", ".run", newID)
	os.MkdirAll(newDir, 0755)
	sightjack.WriteScanResult(filepath.Join(newDir, "scan_result.json"), &sightjack.ScanResult{
		Clusters:     []sightjack.ClusterScanResult{{Name: "new", Completeness: 0.7}},
		Completeness: 0.7,
	})

	// when
	recovered, err := sightjack.RecoverLatestState(dir, sightjack.NewLogger(io.Discard, false))

	// then: should pick the newest (session-2000-1)
	if err != nil {
		t.Fatalf("RecoverLatestState failed: %v", err)
	}
	if recovered.SessionID != newID {
		t.Errorf("SessionID: expected %q, got %q", newID, recovered.SessionID)
	}
}

func TestRecoverLatestState_MixedPrefixes_PrefersNewerScan(t *testing.T) {
	// given: older "session-" and newer "scan-" with higher timestamp
	dir := t.TempDir()

	// Older session
	oldID := "session-1000-1"
	oldDir := filepath.Join(dir, ".siren", ".run", oldID)
	os.MkdirAll(oldDir, 0755)
	sightjack.WriteScanResult(filepath.Join(oldDir, "scan_result.json"), &sightjack.ScanResult{
		Clusters:     []sightjack.ClusterScanResult{{Name: "old", Completeness: 0.3}},
		Completeness: 0.3,
	})

	// Newer scan (higher timestamp, but "scan-" < "session-" lexicographically)
	newID := "scan-2000-1"
	newDir := filepath.Join(dir, ".siren", ".run", newID)
	os.MkdirAll(newDir, 0755)
	sightjack.WriteScanResult(filepath.Join(newDir, "scan_result.json"), &sightjack.ScanResult{
		Clusters:     []sightjack.ClusterScanResult{{Name: "new", Completeness: 0.7}},
		Completeness: 0.7,
	})

	// when
	recovered, err := sightjack.RecoverLatestState(dir, sightjack.NewLogger(io.Discard, false))

	// then: should pick scan-2000-1 (newer timestamp) not session-1000-1
	if err != nil {
		t.Fatalf("RecoverLatestState failed: %v", err)
	}
	if recovered.SessionID != newID {
		t.Errorf("SessionID: expected %q, got %q", newID, recovered.SessionID)
	}
}

func TestRecoverLatestState_NoSessions(t *testing.T) {
	// given: empty .siren/ with no session dirs
	dir := t.TempDir()

	// when
	recovered, err := sightjack.RecoverLatestState(dir, sightjack.NewLogger(io.Discard, false))

	// then
	if err == nil {
		t.Fatal("expected error for no sessions")
	}
	if recovered != nil {
		t.Error("expected nil state")
	}
}

func TestTryRecoverStateNoFiles(t *testing.T) {
	dir := t.TempDir()

	// when
	recovered, err := sightjack.TryRecoverState(dir, "nonexistent", sightjack.NewLogger(io.Discard, false))

	// then
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
	if recovered != nil {
		t.Error("expected nil state")
	}
}

func TestMergeOldWaves_CarriesForwardFailedClusters(t *testing.T) {
	oldWaves := []sightjack.Wave{
		{ID: "1", ClusterName: "auth", Title: "Auth wave", Status: "completed"},
		{ID: "2", ClusterName: "db", Title: "DB wave", Status: "pending"},
		{ID: "3", ClusterName: "api", Title: "API wave", Status: "completed"},
	}
	// Only "auth" and "api" regenerated; "db" failed but is still in scan.
	newWaves := []sightjack.Wave{
		{ID: "1", ClusterName: "auth", Title: "Auth wave v2"},
		{ID: "3", ClusterName: "api", Title: "API wave v2"},
	}
	scannedClusters := map[string]bool{"auth": true, "db": true, "api": true}
	failedNames := map[string]bool{"db": true}

	merged := sightjack.MergeOldWaves(oldWaves, newWaves, scannedClusters, failedNames)

	// Expect 3 waves: 2 new + 1 carried forward (db failed but still scanned).
	if len(merged) != 3 {
		t.Fatalf("expected 3 waves, got %d: %v", len(merged), merged)
	}
	if merged[0].Title != "Auth wave v2" {
		t.Errorf("merged[0] should be new auth wave, got %q", merged[0].Title)
	}
	if merged[1].Title != "API wave v2" {
		t.Errorf("merged[1] should be new api wave, got %q", merged[1].Title)
	}
	if merged[2].ClusterName != "db" || merged[2].Title != "DB wave" {
		t.Errorf("merged[2] should be old db wave, got cluster=%q title=%q", merged[2].ClusterName, merged[2].Title)
	}
	if merged[2].Status != "pending" {
		t.Errorf("carried-forward wave should preserve original status, got %q", merged[2].Status)
	}
}

func TestMergeOldWaves_DropsRemovedClusters(t *testing.T) {
	oldWaves := []sightjack.Wave{
		{ID: "1", ClusterName: "auth", Title: "Auth wave", Status: "completed"},
		{ID: "2", ClusterName: "obsolete", Title: "Obsolete wave", Status: "completed"},
	}
	// "auth" regenerated; "obsolete" is gone from scan entirely.
	newWaves := []sightjack.Wave{
		{ID: "1", ClusterName: "auth", Title: "Auth wave v2"},
	}
	scannedClusters := map[string]bool{"auth": true}
	failedNames := map[string]bool{} // no failures

	merged := sightjack.MergeOldWaves(oldWaves, newWaves, scannedClusters, failedNames)

	if len(merged) != 1 {
		t.Fatalf("expected 1 wave (obsolete dropped), got %d: %v", len(merged), merged)
	}
	if merged[0].Title != "Auth wave v2" {
		t.Errorf("should use new auth wave, got %q", merged[0].Title)
	}
}

func TestMergeOldWaves_AllClustersRegenerated(t *testing.T) {
	oldWaves := []sightjack.Wave{
		{ID: "1", ClusterName: "auth", Title: "Auth old"},
	}
	newWaves := []sightjack.Wave{
		{ID: "1", ClusterName: "auth", Title: "Auth new"},
	}
	scannedClusters := map[string]bool{"auth": true}
	failedNames := map[string]bool{} // no failures

	merged := sightjack.MergeOldWaves(oldWaves, newWaves, scannedClusters, failedNames)

	if len(merged) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(merged))
	}
	if merged[0].Title != "Auth new" {
		t.Errorf("should use new wave, got %q", merged[0].Title)
	}
}

func TestMergeOldWaves_NoClustersRegenerated(t *testing.T) {
	oldWaves := []sightjack.Wave{
		{ID: "1", ClusterName: "auth", Title: "Auth old", Status: "completed"},
		{ID: "2", ClusterName: "db", Title: "DB old", Status: "pending"},
	}
	var newWaves []sightjack.Wave
	scannedClusters := map[string]bool{"auth": true, "db": true}
	failedNames := map[string]bool{"auth": true, "db": true}

	merged := sightjack.MergeOldWaves(oldWaves, newWaves, scannedClusters, failedNames)

	if len(merged) != 2 {
		t.Fatalf("expected 2 carried-forward waves, got %d", len(merged))
	}
	if merged[0].ClusterName != "auth" || merged[1].ClusterName != "db" {
		t.Errorf("all old waves should be carried forward, got %v", merged)
	}
}

func TestMergeOldWaves_DuplicateName_PartialFailure(t *testing.T) {
	// Two "Auth" clusters existed; one regenerated, one failed.
	oldWaves := []sightjack.Wave{
		{ID: "1", ClusterName: "Auth", Title: "Auth instance 1", Status: "completed"},
		{ID: "2", ClusterName: "Auth", Title: "Auth instance 2", Status: "completed"},
	}
	newWaves := []sightjack.Wave{
		{ID: "10", ClusterName: "Auth", Title: "Auth new"},
	}
	scannedClusters := map[string]bool{"Auth": true}
	// detectFailedClusterNames: 2 input "Auth", 1 success → failed
	failedNames := map[string]bool{"Auth": true}

	merged := sightjack.MergeOldWaves(oldWaves, newWaves, scannedClusters, failedNames)

	// 1 new + 2 old carried forward (safe over-inclusion for duplicates)
	if len(merged) != 3 {
		t.Fatalf("expected 3 waves (1 new + 2 old), got %d: %v", len(merged), merged)
	}
	if merged[0].Title != "Auth new" {
		t.Errorf("merged[0] should be new wave, got %q", merged[0].Title)
	}
}

func TestMergeOldWaves_DuplicateName_AllSucceeded(t *testing.T) {
	// Two "Auth" clusters, both regenerated successfully.
	oldWaves := []sightjack.Wave{
		{ID: "1", ClusterName: "Auth", Title: "Auth old 1"},
		{ID: "2", ClusterName: "Auth", Title: "Auth old 2"},
	}
	newWaves := []sightjack.Wave{
		{ID: "10", ClusterName: "Auth", Title: "Auth new 1"},
		{ID: "20", ClusterName: "Auth", Title: "Auth new 2"},
	}
	scannedClusters := map[string]bool{"Auth": true}
	failedNames := map[string]bool{} // both succeeded

	merged := sightjack.MergeOldWaves(oldWaves, newWaves, scannedClusters, failedNames)

	// Only new waves, no carry-forward.
	if len(merged) != 2 {
		t.Fatalf("expected 2 new waves, got %d: %v", len(merged), merged)
	}
	if merged[0].Title != "Auth new 1" || merged[1].Title != "Auth new 2" {
		t.Errorf("should only have new waves, got %v", merged)
	}
}
