package sightjack

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsWaveApplyComplete_NoErrors(t *testing.T) {
	// given
	result := &WaveApplyResult{
		WaveID:  "auth-w1",
		Applied: 5,
		Errors:  []string{},
		Ripples: []Ripple{{ClusterName: "API", Description: "W2 unlocked"}},
	}

	// when
	complete := IsWaveApplyComplete(result)

	// then
	if !complete {
		t.Error("expected complete when no errors")
	}
}

func TestIsWaveApplyComplete_WithErrors(t *testing.T) {
	// given
	result := &WaveApplyResult{
		WaveID:  "auth-w1",
		Applied: 3,
		Errors:  []string{"failed to update ENG-101", "failed to update ENG-102"},
		Ripples: []Ripple{},
	}

	// when
	complete := IsWaveApplyComplete(result)

	// then
	if complete {
		t.Error("expected not complete when errors present")
	}
}

func TestIsWaveApplyComplete_NilErrors(t *testing.T) {
	// given
	result := &WaveApplyResult{
		WaveID:  "auth-w1",
		Applied: 5,
		Errors:  nil,
	}

	// when
	complete := IsWaveApplyComplete(result)

	// then
	if !complete {
		t.Error("expected complete when errors is nil")
	}
}

func TestRunSession_DryRunGeneratesWavePrompts(t *testing.T) {
	// given: dry-run session should generate both classify and wave_generate prompts
	baseDir := t.TempDir()
	cfg := &Config{
		Lang: "en",
		Claude: ClaudeConfig{
			Command:    "claude",
			TimeoutSec: 60,
		},
		Scan: ScanConfig{
			MaxConcurrency: 1,
			ChunkSize:      50,
		},
		Linear: LinearConfig{
			Team:    "ENG",
			Project: "Test",
		},
	}
	sessionID := "test-dry-run"
	ctx := context.Background()

	// when
	err := RunSession(ctx, cfg, baseDir, sessionID, true, nil)

	// then: no error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// then: classify prompt was generated (Pass 1)
	scanDir := ScanDir(baseDir, sessionID)
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
}

func TestRunSession_NilInputReturnsError(t *testing.T) {
	// given: non-dry-run session with nil input should return error early
	cfg := &Config{
		Lang:   "en",
		Claude: ClaudeConfig{Command: "claude", TimeoutSec: 60},
		Scan:   ScanConfig{MaxConcurrency: 1, ChunkSize: 50},
		Linear: LinearConfig{Team: "ENG", Project: "Test"},
	}

	// when
	err := RunSession(context.Background(), cfg, t.TempDir(), "test-nil-input", false, nil)

	// then: should get an input-related error, not a panic or scan error
	if err == nil {
		t.Fatal("expected error for nil input in non-dry-run mode")
	}
	if !strings.Contains(err.Error(), "input") {
		t.Errorf("expected input-related error, got: %v", err)
	}
}

func TestBuildCompletedWaveMap(t *testing.T) {
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Status: "completed"},
		{ID: "auth-w2", ClusterName: "Auth", Status: "available"},
		{ID: "api-w1", ClusterName: "API", Status: "completed"},
	}

	completed := BuildCompletedWaveMap(waves)
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
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed", Prerequisites: nil, Actions: make([]WaveAction, 3)},
		{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "available", Prerequisites: []string{"auth-w1"}, Actions: make([]WaveAction, 5)},
	}

	states := BuildWaveStates(waves)
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
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps",
			Actions: []WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
			Delta:   WaveDelta{Before: 0.25, After: 0.40}},
	}
	input := "1\nd\nShould we split?\na\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when: selection
	selected, err := PromptWaveSelection(ctx, &output, scanner, waves)
	if err != nil {
		t.Fatalf("selection error: %v", err)
	}
	if selected.ID != "auth-w1" {
		t.Fatalf("expected auth-w1, got %s", selected.ID)
	}

	// when: first approval -> discuss
	choice, err := PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("first approval error: %v", err)
	}
	if choice != ApprovalDiscuss {
		t.Fatalf("expected ApprovalDiscuss, got %d", choice)
	}

	// when: topic input
	topic, err := PromptDiscussTopic(ctx, &output, scanner)
	if err != nil {
		t.Fatalf("topic error: %v", err)
	}
	if topic != "Should we split?" {
		t.Errorf("expected topic, got: %s", topic)
	}

	// when: second approval -> approve
	choice, err = PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("second approval error: %v", err)
	}
	if choice != ApprovalApprove {
		t.Errorf("expected ApprovalApprove after discuss, got %d", choice)
	}
}

func TestBuildCompletedWaveMap_Empty(t *testing.T) {
	// given: nil and empty wave slices
	tests := []struct {
		name  string
		waves []Wave
	}{
		{"nil", nil},
		{"empty", []Wave{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			completed := BuildCompletedWaveMap(tt.waves)

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
	waves := []Wave{
		{ID: "w1", ClusterName: "Auth", Status: "completed"},
		{ID: "w1", ClusterName: "API", Status: "completed"},
	}

	// when
	completed := BuildCompletedWaveMap(waves)

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
		waves []Wave
	}{
		{"nil", nil},
		{"empty", []Wave{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			states := BuildWaveStates(tt.waves)

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
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps",
			Actions: []WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
			Delta:   WaveDelta{Before: 0.25, After: 0.40}},
	}
	input := "1\nd\nShould we split?\nr\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when: selection
	selected, err := PromptWaveSelection(ctx, &output, scanner, waves)
	if err != nil {
		t.Fatalf("selection error: %v", err)
	}

	// when: first approval -> discuss
	choice, err := PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("first approval error: %v", err)
	}
	if choice != ApprovalDiscuss {
		t.Fatalf("expected ApprovalDiscuss, got %d", choice)
	}

	// when: topic input
	topic, err := PromptDiscussTopic(ctx, &output, scanner)
	if err != nil {
		t.Fatalf("topic error: %v", err)
	}
	if topic != "Should we split?" {
		t.Errorf("expected topic, got: %s", topic)
	}

	// when: second approval -> reject
	choice, err = PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("second approval error: %v", err)
	}
	if choice != ApprovalReject {
		t.Errorf("expected ApprovalReject after discuss, got %d", choice)
	}
}

func TestDiscussBranchQuitAtTopic(t *testing.T) {
	// given: piped input: select wave 1, discuss, then quit at topic prompt
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps",
			Actions: []WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
			Delta:   WaveDelta{Before: 0.25, After: 0.40}},
	}
	input := "1\nd\nq\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when: selection
	selected, err := PromptWaveSelection(ctx, &output, scanner, waves)
	if err != nil {
		t.Fatalf("selection error: %v", err)
	}

	// when: approval -> discuss
	choice, err := PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("approval error: %v", err)
	}
	if choice != ApprovalDiscuss {
		t.Fatalf("expected ApprovalDiscuss, got %d", choice)
	}

	// when: topic -> quit
	_, err = PromptDiscussTopic(ctx, &output, scanner)
	if err != ErrQuit {
		t.Errorf("expected ErrQuit when quitting at topic, got %v", err)
	}
}

func TestMultipleDiscussRounds(t *testing.T) {
	// given: two discuss rounds then approve
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps",
			Actions: []WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
			Delta:   WaveDelta{Before: 0.25, After: 0.40}},
	}
	input := "1\nd\nFirst topic\nd\nSecond topic\na\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when: selection
	selected, err := PromptWaveSelection(ctx, &output, scanner, waves)
	if err != nil {
		t.Fatalf("selection error: %v", err)
	}

	// Round 1: discuss
	choice, err := PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("round 1 approval error: %v", err)
	}
	if choice != ApprovalDiscuss {
		t.Fatalf("round 1: expected ApprovalDiscuss, got %d", choice)
	}
	topic, err := PromptDiscussTopic(ctx, &output, scanner)
	if err != nil {
		t.Fatalf("round 1 topic error: %v", err)
	}
	if topic != "First topic" {
		t.Errorf("round 1: expected 'First topic', got: %s", topic)
	}

	// Round 2: discuss again
	choice, err = PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("round 2 approval error: %v", err)
	}
	if choice != ApprovalDiscuss {
		t.Fatalf("round 2: expected ApprovalDiscuss, got %d", choice)
	}
	topic, err = PromptDiscussTopic(ctx, &output, scanner)
	if err != nil {
		t.Fatalf("round 2 topic error: %v", err)
	}
	if topic != "Second topic" {
		t.Errorf("round 2: expected 'Second topic', got: %s", topic)
	}

	// Final: approve
	choice, err = PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("final approval error: %v", err)
	}
	if choice != ApprovalApprove {
		t.Errorf("expected ApprovalApprove after two discuss rounds, got %d", choice)
	}
}
