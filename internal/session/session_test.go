package session_test

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

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func TestIsWaveApplyComplete_NoErrors(t *testing.T) {
	// given
	result := &domain.WaveApplyResult{
		WaveID:  "auth-w1",
		Applied: 5,
		Errors:  []string{},
		Ripples: []domain.Ripple{{ClusterName: "API", Description: "W2 unlocked"}},
	}

	// when
	complete := domain.IsWaveApplyComplete(result)

	// then
	if !complete {
		t.Error("expected complete when no errors")
	}
}

func TestIsWaveApplyComplete_WithErrors(t *testing.T) {
	// given
	result := &domain.WaveApplyResult{
		WaveID:  "auth-w1",
		Applied: 3,
		Errors:  []string{"failed to update ENG-101", "failed to update ENG-102"},
		Ripples: []domain.Ripple{},
	}

	// when
	complete := domain.IsWaveApplyComplete(result)

	// then
	if complete {
		t.Error("expected not complete when errors present")
	}
}

func TestIsWaveApplyComplete_NilErrors(t *testing.T) {
	// given
	result := &domain.WaveApplyResult{
		WaveID:  "auth-w1",
		Applied: 5,
		Errors:  nil,
	}

	// when
	complete := domain.IsWaveApplyComplete(result)

	// then
	if !complete {
		t.Error("expected complete when errors is nil")
	}
}

func TestRunSession_DryRunGeneratesWavePrompts(t *testing.T) {
	// given: dry-run session should generate both classify and wave_generate prompts
	baseDir := t.TempDir()
	cfg := &domain.Config{
		Lang: "en",
		Claude: domain.ClaudeConfig{
			Command:    "claude",
			TimeoutSec: 60,
		},
		Scan: domain.ScanConfig{
			MaxConcurrency: 1,
			ChunkSize:      50,
		},
		Linear: domain.LinearConfig{
			Team:    "ENG",
			Project: "Test",
		},
		Scribe: domain.ScribeConfig{Enabled: true},
	}
	sessionID := "test-dry-run"
	ctx := context.Background()

	// when
	err := session.RunSession(ctx, cfg, baseDir, sessionID, true, nil, io.Discard, session.NopRecorder{}, domain.NewLogger(io.Discard, false))

	// then: no error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// then: classify prompt was generated (Pass 1)
	scanDir := domain.ScanDir(baseDir, sessionID)
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
	cfg := &domain.Config{
		Lang: "en",
		Claude: domain.ClaudeConfig{
			Command:    "claude",
			TimeoutSec: 60,
		},
		Scan: domain.ScanConfig{
			MaxConcurrency: 1,
			ChunkSize:      50,
		},
		Linear: domain.LinearConfig{
			Team:    "ENG",
			Project: "Test",
		},
		Scribe: domain.ScribeConfig{Enabled: false},
	}
	sessionID := "test-dry-run-no-scribe"
	ctx := context.Background()

	// when
	err := session.RunSession(ctx, cfg, baseDir, sessionID, true, nil, io.Discard, session.NopRecorder{}, domain.NewLogger(io.Discard, false))

	// then: no error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// then: scribe prompt should NOT be generated
	scanDir := domain.ScanDir(baseDir, sessionID)
	scribePrompt := filepath.Join(scanDir, "scribe_sample_sample-w1_prompt.md")
	if _, err := os.Stat(scribePrompt); !os.IsNotExist(err) {
		t.Error("scribe prompt should not be generated when Scribe is disabled")
	}
}

func TestRunSession_NilInputReturnsError(t *testing.T) {
	// given: non-dry-run session with nil input should return error early
	cfg := &domain.Config{
		Lang:   "en",
		Claude: domain.ClaudeConfig{Command: "claude", TimeoutSec: 60},
		Scan:   domain.ScanConfig{MaxConcurrency: 1, ChunkSize: 50},
		Linear: domain.LinearConfig{Team: "ENG", Project: "Test"},
	}

	// when
	err := session.RunSession(context.Background(), cfg, t.TempDir(), "test-nil-input", false, nil, io.Discard, session.NopRecorder{}, domain.NewLogger(io.Discard, false))

	// then: should get an input-related error, not a panic or scan error
	if err == nil {
		t.Fatal("expected error for nil input in non-dry-run mode")
	}
	if !strings.Contains(err.Error(), "input") {
		t.Errorf("expected input-related error, got: %v", err)
	}
}

func TestBuildCompletedWaveMap(t *testing.T) {
	waves := []domain.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Status: "completed"},
		{ID: "auth-w2", ClusterName: "Auth", Status: "available"},
		{ID: "api-w1", ClusterName: "API", Status: "completed"},
	}

	completed := domain.BuildCompletedWaveMap(waves)
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
	waves := []domain.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed", Prerequisites: nil, Actions: make([]domain.WaveAction, 3)},
		{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "available", Prerequisites: []string{"auth-w1"}, Actions: make([]domain.WaveAction, 5)},
	}

	states := domain.BuildWaveStates(waves)
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
	waves := []domain.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps",
			Actions: []domain.WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
			Delta:   domain.WaveDelta{Before: 0.25, After: 0.40}},
	}
	input := "1\nd\nShould we split?\na\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when: selection
	selected, err := session.PromptWaveSelection(ctx, &output, scanner, waves)
	if err != nil {
		t.Fatalf("selection error: %v", err)
	}
	if selected.ID != "auth-w1" {
		t.Fatalf("expected auth-w1, got %s", selected.ID)
	}

	// when: first approval -> discuss
	choice, err := session.PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("first approval error: %v", err)
	}
	if choice != domain.ApprovalDiscuss {
		t.Fatalf("expected ApprovalDiscuss, got %d", choice)
	}

	// when: topic input
	topic, err := session.PromptDiscussTopic(ctx, &output, scanner)
	if err != nil {
		t.Fatalf("topic error: %v", err)
	}
	if topic != "Should we split?" {
		t.Errorf("expected topic, got: %s", topic)
	}

	// when: second approval -> approve
	choice, err = session.PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("second approval error: %v", err)
	}
	if choice != domain.ApprovalApprove {
		t.Errorf("expected ApprovalApprove after discuss, got %d", choice)
	}
}

func TestBuildCompletedWaveMap_Empty(t *testing.T) {
	// given: nil and empty wave slices
	tests := []struct {
		name  string
		waves []domain.Wave
	}{
		{"nil", nil},
		{"empty", []domain.Wave{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			completed := domain.BuildCompletedWaveMap(tt.waves)

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
	waves := []domain.Wave{
		{ID: "w1", ClusterName: "Auth", Status: "completed"},
		{ID: "w1", ClusterName: "API", Status: "completed"},
	}

	// when
	completed := domain.BuildCompletedWaveMap(waves)

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
		waves []domain.Wave
	}{
		{"nil", nil},
		{"empty", []domain.Wave{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			states := domain.BuildWaveStates(tt.waves)

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
	waves := []domain.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps",
			Actions: []domain.WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
			Delta:   domain.WaveDelta{Before: 0.25, After: 0.40}},
	}
	input := "1\nd\nShould we split?\nr\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when: selection
	selected, err := session.PromptWaveSelection(ctx, &output, scanner, waves)
	if err != nil {
		t.Fatalf("selection error: %v", err)
	}

	// when: first approval -> discuss
	choice, err := session.PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("first approval error: %v", err)
	}
	if choice != domain.ApprovalDiscuss {
		t.Fatalf("expected ApprovalDiscuss, got %d", choice)
	}

	// when: topic input
	topic, err := session.PromptDiscussTopic(ctx, &output, scanner)
	if err != nil {
		t.Fatalf("topic error: %v", err)
	}
	if topic != "Should we split?" {
		t.Errorf("expected topic, got: %s", topic)
	}

	// when: second approval -> reject
	choice, err = session.PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("second approval error: %v", err)
	}
	if choice != domain.ApprovalReject {
		t.Errorf("expected ApprovalReject after discuss, got %d", choice)
	}
}

func TestDiscussBranchQuitAtTopic(t *testing.T) {
	// given: piped input: select wave 1, discuss, then quit at topic prompt
	waves := []domain.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps",
			Actions: []domain.WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
			Delta:   domain.WaveDelta{Before: 0.25, After: 0.40}},
	}
	input := "1\nd\nq\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when: selection
	selected, err := session.PromptWaveSelection(ctx, &output, scanner, waves)
	if err != nil {
		t.Fatalf("selection error: %v", err)
	}

	// when: approval -> discuss
	choice, err := session.PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("approval error: %v", err)
	}
	if choice != domain.ApprovalDiscuss {
		t.Fatalf("expected ApprovalDiscuss, got %d", choice)
	}

	// when: topic -> quit
	_, err = session.PromptDiscussTopic(ctx, &output, scanner)
	if err != session.ErrQuit {
		t.Errorf("expected ErrQuit when quitting at topic, got %v", err)
	}
}

func TestMultipleDiscussRounds(t *testing.T) {
	// given: two discuss rounds then approve
	waves := []domain.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps",
			Actions: []domain.WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
			Delta:   domain.WaveDelta{Before: 0.25, After: 0.40}},
	}
	input := "1\nd\nFirst topic\nd\nSecond topic\na\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	// when: selection
	selected, err := session.PromptWaveSelection(ctx, &output, scanner, waves)
	if err != nil {
		t.Fatalf("selection error: %v", err)
	}

	// Round 1: discuss
	choice, err := session.PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("round 1 approval error: %v", err)
	}
	if choice != domain.ApprovalDiscuss {
		t.Fatalf("round 1: expected ApprovalDiscuss, got %d", choice)
	}
	topic, err := session.PromptDiscussTopic(ctx, &output, scanner)
	if err != nil {
		t.Fatalf("round 1 topic error: %v", err)
	}
	if topic != "First topic" {
		t.Errorf("round 1: expected 'First topic', got: %s", topic)
	}

	// Round 2: discuss again
	choice, err = session.PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("round 2 approval error: %v", err)
	}
	if choice != domain.ApprovalDiscuss {
		t.Fatalf("round 2: expected ApprovalDiscuss, got %d", choice)
	}
	topic, err = session.PromptDiscussTopic(ctx, &output, scanner)
	if err != nil {
		t.Fatalf("round 2 topic error: %v", err)
	}
	if topic != "Second topic" {
		t.Errorf("round 2: expected 'Second topic', got: %s", topic)
	}

	// Final: approve
	choice, err = session.PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("final approval error: %v", err)
	}
	if choice != domain.ApprovalApprove {
		t.Errorf("expected ApprovalApprove after two discuss rounds, got %d", choice)
	}
}

func TestApplyModifiedWave_PreservesIdentity(t *testing.T) {
	// given: original wave with known identity
	original := domain.Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Original Title",
		Actions:     []domain.WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "original"}},
		Delta:       domain.WaveDelta{Before: 0.25, After: 0.40},
		Status:      "available",
	}
	// given: architect returns modified wave with CHANGED identity fields
	modified := domain.Wave{
		ID:          "new-w1",
		ClusterName: "Authentication",
		Title:       "Better Title",
		Actions: []domain.WaveAction{
			{Type: "add_dependency", IssueID: "ENG-101", Description: "original"},
			{Type: "add_dod", IssueID: "ENG-101", Description: "new action"},
		},
		Delta:  domain.WaveDelta{Before: 0.25, After: 0.50},
		Status: "modified",
	}

	// when: no prerequisites, empty completed map
	result := domain.ApplyModifiedWave(original, modified, map[string]bool{})

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
	original := domain.Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Original",
		Status:      "available",
	}
	// given: architect adds a prerequisite that hasn't been completed
	modified := domain.Wave{
		ID:            "auth-w1",
		ClusterName:   "Auth",
		Title:         "Modified",
		Prerequisites: []string{"API:api-w1"},
		Actions:       []domain.WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "new"}},
	}
	// given: api-w1 is NOT in the completed map
	completed := map[string]bool{}

	// when
	result := domain.ApplyModifiedWave(original, modified, completed)

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
	original := domain.Wave{
		ID:          "auth-w2",
		ClusterName: "Auth",
		Title:       "Original",
		Status:      "available",
	}
	modified := domain.Wave{
		ID:            "auth-w2",
		ClusterName:   "Auth",
		Title:         "Modified",
		Prerequisites: []string{"Auth:auth-w1"},
		Actions:       []domain.WaveAction{{Type: "add_dod", IssueID: "ENG-102", Description: "new"}},
	}
	completed := map[string]bool{"Auth:auth-w1": true}

	// when
	result := domain.ApplyModifiedWave(original, modified, completed)

	// then: status should remain "available" because prerequisite is met
	if result.Status != "available" {
		t.Errorf("expected 'available' for met prerequisites, got '%s'", result.Status)
	}
}

func TestApplyModifiedWave_NormalizesBarePrerequisites(t *testing.T) {
	// given: architect returns bare ID "auth-w1" instead of composite "Auth:auth-w1"
	original := domain.Wave{
		ID:          "auth-w2",
		ClusterName: "Auth",
		Title:       "Original",
		Status:      "available",
	}
	modified := domain.Wave{
		ID:            "auth-w2",
		ClusterName:   "Auth",
		Title:         "Modified",
		Prerequisites: []string{"auth-w1"}, // bare ID, not composite
		Actions:       []domain.WaveAction{{Type: "add_dod", IssueID: "ENG-102", Description: "new"}},
	}
	// given: "Auth:auth-w1" IS completed (composite key)
	completed := map[string]bool{"Auth:auth-w1": true}

	// when
	result := domain.ApplyModifiedWave(original, modified, completed)

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
	waves := []domain.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Wave 1", Status: "available"},
		{ID: "auth-w2", ClusterName: "Auth", Title: "Wave 2", Status: "available"},
	}
	modified := domain.Wave{
		ID:            "auth-w1",
		ClusterName:   "Auth",
		Title:         "Modified",
		Prerequisites: []string{"API:api-w1"}, // unmet
		Actions:       []domain.WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "new"}},
	}
	completed := map[string]bool{}

	// when: apply modification
	result := domain.ApplyModifiedWave(waves[0], modified, completed)
	if result.Status != "locked" {
		t.Fatalf("precondition: expected locked, got %s", result.Status)
	}

	// when: propagate back to waves slice
	domain.PropagateWaveUpdate(waves, result)

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
	original := domain.Wave{
		ID:            "auth-w2",
		ClusterName:   "Auth",
		Title:         "Original",
		Status:        "locked",
		Prerequisites: []string{"Auth:auth-w1"},
		Delta:         domain.WaveDelta{Before: 0.20, After: 0.40},
	}
	modified := domain.Wave{
		Title:         "Modified Title",
		Prerequisites: nil, // architect omitted the field
		Actions:       []domain.WaveAction{{Type: "add_dod", IssueID: "ENG-102", Description: "new"}},
	}
	completed := map[string]bool{} // auth-w1 NOT completed

	// when
	result := domain.ApplyModifiedWave(original, modified, completed)

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
	original := domain.Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Original",
		Status:      "available",
		Delta:       domain.WaveDelta{Before: 0.25, After: 0.50},
	}
	modified := domain.Wave{
		Title:   "Modified Title",
		Actions: []domain.WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "new"}},
		Delta:   domain.WaveDelta{}, // zero value — architect omitted the field
	}
	completed := map[string]bool{}

	// when
	result := domain.ApplyModifiedWave(original, modified, completed)

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
	newWaves := []domain.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "available"},
		{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "locked"},
		{ID: "api-w2", ClusterName: "API", Title: "New Wave", Status: "available"},
	}

	// when
	merged := domain.MergeCompletedStatus(oldCompleted, newWaves)

	// then: auth-w1 should be completed (was in old)
	for _, w := range merged {
		if domain.WaveKey(w) == "Auth:auth-w1" && w.Status != "completed" {
			t.Errorf("expected Auth:auth-w1 completed, got %s", w.Status)
		}
	}
	// then: api-w1 not in new waves (dropped from Linear) — not present at all
	for _, w := range merged {
		if domain.WaveKey(w) == "API:api-w1" {
			t.Error("API:api-w1 should not appear in merged result")
		}
	}
	// then: auth-w2 and api-w2 keep original status
	for _, w := range merged {
		if domain.WaveKey(w) == "Auth:auth-w2" && w.Status != "locked" {
			t.Errorf("expected Auth:auth-w2 locked, got %s", w.Status)
		}
		if domain.WaveKey(w) == "API:api-w2" && w.Status != "available" {
			t.Errorf("expected API:api-w2 available, got %s", w.Status)
		}
	}
}

func TestMergeCompletedStatus_EmptyOld(t *testing.T) {
	// given: no old completed waves
	oldCompleted := map[string]bool{}
	newWaves := []domain.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Status: "available"},
	}

	// when
	merged := domain.MergeCompletedStatus(oldCompleted, newWaves)

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
	var newWaves []domain.Wave

	// when
	merged := domain.MergeCompletedStatus(oldCompleted, newWaves)

	// then
	if len(merged) != 0 {
		t.Errorf("expected 0 waves, got %d", len(merged))
	}
}

func TestBuildWaveStates_IncludesFullFields(t *testing.T) {
	// given
	waves := []domain.Wave{
		{
			ID:            "auth-w1",
			ClusterName:   "Auth",
			Title:         "Deps",
			Status:        "completed",
			Prerequisites: []string{"Auth:auth-w0"},
			Actions: []domain.WaveAction{
				{Type: "add_dependency", IssueID: "ENG-101", Description: "dep"},
				{Type: "add_dod", IssueID: "ENG-102", Description: "dod"},
			},
			Description: "Order dependencies first",
			Delta:       domain.WaveDelta{Before: 0.20, After: 0.40},
		},
	}

	// when
	states := domain.BuildWaveStates(waves)

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
	states := []domain.WaveState{
		{
			ID:            "auth-w1",
			ClusterName:   "Auth",
			Title:         "Deps",
			Status:        "completed",
			Prerequisites: []string{"Auth:auth-w0"},
			ActionCount:   2,
			Actions: []domain.WaveAction{
				{Type: "add_dependency", IssueID: "ENG-101", Description: "dep"},
				{Type: "add_dod", IssueID: "ENG-102", Description: "dod"},
			},
			Description: "Order dependencies first",
			Delta:       domain.WaveDelta{Before: 0.20, After: 0.40},
		},
		{
			ID:          "auth-w2",
			ClusterName: "Auth",
			Title:       "DoD",
			Status:      "available",
			ActionCount: 1,
			Actions:     []domain.WaveAction{{Type: "add_dod", IssueID: "ENG-103", Description: "dod2"}},
			Delta:       domain.WaveDelta{Before: 0.40, After: 0.60},
		},
	}

	// when
	waves := domain.RestoreWaves(states)

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
	var states []domain.WaveState

	// when
	waves := domain.RestoreWaves(states)

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
	cfg := &domain.Config{
		Lang:   "en",
		Claude: domain.ClaudeConfig{Command: "claude", TimeoutSec: 60},
		Scan:   domain.ScanConfig{MaxConcurrency: 1, ChunkSize: 50},
		Linear: domain.LinearConfig{Team: "ENG", Project: "Test"},
		Scribe: domain.ScribeConfig{Enabled: true},
	}
	sessionID := "test-no-cache"
	ctx := context.Background()

	// when
	err := session.RunSession(ctx, cfg, baseDir, sessionID, true, nil, io.Discard, session.NopRecorder{}, domain.NewLogger(io.Discard, false))

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	scanDir := domain.ScanDir(baseDir, sessionID)
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
	got := domain.CalcNewlyUnlocked(oldAvailable, newAvailable)

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
	got := domain.CalcNewlyUnlocked(oldAvailable, newAvailable)

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
	got := domain.CalcNewlyUnlocked(oldAvailable, newAvailable)

	// then
	if got != 0 {
		t.Errorf("expected 0 newly unlocked waves, got %d", got)
	}
}

func TestApplyModifiedWave_PreservesOriginalActionsWhenNil(t *testing.T) {
	// given: original wave has actions, modified wave omits them (nil from JSON)
	originalActions := []domain.WaveAction{
		{Type: "add_dod", IssueID: "ENG-101", Description: "Original action 1"},
		{Type: "add_dependency", IssueID: "ENG-102", Description: "Original action 2"},
	}
	original := domain.Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Original",
		Status:      "available",
		Actions:     originalActions,
		Delta:       domain.WaveDelta{Before: 0.20, After: 0.40},
	}
	modified := domain.Wave{
		Title:   "Modified Title",
		Actions: nil, // architect omitted the field
	}
	completed := map[string]bool{}

	// when
	result := domain.ApplyModifiedWave(original, modified, completed)

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
	scanDir := domain.ScanDir(baseDir, "old-session")
	os.MkdirAll(scanDir, 0755)
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	scanResult := &domain.ScanResult{
		Clusters: []domain.ClusterScanResult{
			{Name: "Auth", Completeness: 0.50, Issues: []domain.IssueDetail{
				{ID: "ENG-101", Identifier: "ENG-101", Title: "Login", Completeness: 0.50},
			}},
		},
		TotalIssues:  1,
		Completeness: 0.50,
	}
	if err := session.WriteScanResult(scanResultPath, scanResult); err != nil {
		t.Fatalf("write scan result: %v", err)
	}

	// Create state pointing to that scan result
	state := &domain.SessionState{
		Version:        "0.5",
		SessionID:      "old-session",
		Project:        "TestProject",
		LastScanned:    time.Now(),
		Completeness:   0.50,
		ScanResultPath: scanResultPath,
		Clusters: []domain.ClusterState{
			{Name: "Auth", Completeness: 0.50, IssueCount: 1},
		},
		Waves: []domain.WaveState{
			{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed",
				ActionCount: 1,
				Actions:     []domain.WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "d"}},
				Delta:       domain.WaveDelta{Before: 0.25, After: 0.50}},
			{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "available",
				ActionCount: 1,
				Actions:     []domain.WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "d2"}},
				Delta:       domain.WaveDelta{Before: 0.50, After: 0.75}},
		},
		ADRCount: 2,
	}
	// when: ResumeSession loads state and returns waves + scan result
	resumedScanResult, waves, completed, adrCount, err := session.ResumeSession(baseDir, state)

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
	cfg := &domain.Config{
		Lang:   "en",
		Claude: domain.ClaudeConfig{Command: "claude", TimeoutSec: 60},
		Linear: domain.LinearConfig{Team: "ENG", Project: "Test"},
	}
	state := &domain.SessionState{
		Version:        "0.5",
		SessionID:      "old-session",
		ScanResultPath: "/some/path.json",
	}

	// when
	err := session.RunResumeSession(context.Background(), cfg, t.TempDir(), state, nil, io.Discard, session.NopRecorder{}, domain.NewLogger(io.Discard, false))

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
	cfg := &domain.Config{
		Lang:   "en",
		Claude: domain.ClaudeConfig{Command: "claude", TimeoutSec: 60},
		Linear: domain.LinearConfig{Team: "ENG", Project: "Test"},
	}
	state := &domain.SessionState{
		Version:   "0.5",
		SessionID: "old-session",
	}

	// when
	err := session.RunRescanSession(context.Background(), cfg, t.TempDir(), state, "test-rescan", nil, io.Discard, session.NopRecorder{}, domain.NewLogger(io.Discard, false))

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
	state := &domain.SessionState{
		Version:        "0.5",
		SessionID:      "old-session",
		ScanResultPath: "",
	}

	// when
	_, _, _, _, err := session.ResumeSession(t.TempDir(), state)

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
	state := &domain.SessionState{
		Version:        "0.5",
		SessionID:      "old-session",
		ScanResultPath: "/nonexistent/scan_result.json",
	}

	// when
	_, _, _, _, err := session.ResumeSession(t.TempDir(), state)

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

	scanResult := &domain.ScanResult{
		Clusters:     []domain.ClusterScanResult{{Name: "Auth", Completeness: 0.50, Issues: []domain.IssueDetail{{ID: "E1", Identifier: "E1", Title: "t"}}}},
		TotalIssues:  1,
		Completeness: 0.50,
	}
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	if err := session.WriteScanResult(scanResultPath, scanResult); err != nil {
		t.Fatalf("write scan result: %v", err)
	}

	// Create 3 ADR files on filesystem
	adrDir := session.ADRDir(baseDir)
	os.MkdirAll(adrDir, 0755)
	for _, name := range []string{"0001-first.md", "0002-second.md", "0003-third.md"} {
		os.WriteFile(filepath.Join(adrDir, name), []byte("# ADR"), 0644)
	}

	state := &domain.SessionState{
		Version:        "0.5",
		SessionID:      "old-session",
		ScanResultPath: scanResultPath,
		Waves:          []domain.WaveState{{ID: "w1", ClusterName: "Auth", Status: "available"}},
		ADRCount:       2, // stale: says 2 but filesystem has 3
	}
	// when
	_, _, _, adrCount, err := session.ResumeSession(baseDir, state)

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

	state := &domain.SessionState{
		ScanResultPath: path,
		Waves:          []domain.WaveState{{ID: "w1", ClusterName: "auth", Status: "pending"}},
	}

	// when / then
	if !session.CanResume(dir, state) {
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

	state := &domain.SessionState{ScanResultPath: path, Waves: nil}

	// when / then
	if session.CanResume(dir, state) {
		t.Error("expected CanResume false when waves are empty")
	}
}

func TestCanResume_EmptyPath(t *testing.T) {
	// given: state with empty ScanResultPath (fallback to ScanDir)
	state := &domain.SessionState{ScanResultPath: ""}

	// when / then
	if session.CanResume("", state) {
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
			result := &domain.WaveApplyResult{Applied: tt.applied, TotalCount: tt.total}
			delta := domain.WaveDelta{Before: tt.before, After: tt.after}

			// when
			got := domain.PartialApplyDelta(result, delta)

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
		clusters []domain.ClusterScanResult
		wantWarn bool
	}{
		{"consistent", 0.5, []domain.ClusterScanResult{
			{Name: "a", Completeness: 0.4},
			{Name: "b", Completeness: 0.6},
		}, false},
		{"inconsistent", 0.9, []domain.ClusterScanResult{
			{Name: "a", Completeness: 0.4},
			{Name: "b", Completeness: 0.6},
		}, true},
		{"empty clusters", 0.0, nil, false},
		{"within tolerance", 0.54, []domain.ClusterScanResult{
			{Name: "a", Completeness: 0.5},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.CheckCompletenessConsistency(tt.overall, tt.clusters)
			if got != tt.wantWarn {
				t.Errorf("CheckCompletenessConsistency: got %v, want %v", got, tt.wantWarn)
			}
		})
	}
}

func TestCanResume_MissingFile(t *testing.T) {
	// given: state with ScanResultPath pointing to deleted file
	state := &domain.SessionState{ScanResultPath: "/nonexistent/scan_result.json"}

	// when / then
	if session.CanResume("", state) {
		t.Error("expected CanResume false for missing file")
	}
}

func TestResumeSession_EvaluateUnlocksAfterRestore(t *testing.T) {
	// given: saved state where auth-w1 is completed and auth-w2 is locked (depends on auth-w1)
	// After restore + EvaluateUnlocks, auth-w2 should become available
	baseDir := t.TempDir()

	scanDir := domain.ScanDir(baseDir, "resume-unlock")
	os.MkdirAll(scanDir, 0755)
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	scanResult := &domain.ScanResult{
		Clusters:     []domain.ClusterScanResult{{Name: "Auth", Completeness: 0.40, Issues: []domain.IssueDetail{{ID: "E1", Identifier: "E1", Title: "t"}}}},
		TotalIssues:  1,
		Completeness: 0.40,
	}
	if err := session.WriteScanResult(scanResultPath, scanResult); err != nil {
		t.Fatalf("write scan result: %v", err)
	}

	state := &domain.SessionState{
		Version:        domain.StateFormatVersion,
		SessionID:      "resume-unlock",
		Project:        "Test",
		ScanResultPath: scanResultPath,
		Completeness:   0.40,
		Clusters:       []domain.ClusterState{{Name: "Auth", Completeness: 0.40, IssueCount: 1}},
		Waves: []domain.WaveState{
			{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "completed",
				ActionCount: 1, Actions: []domain.WaveAction{{Type: "add_dod", IssueID: "E1", Description: "d"}},
				Delta: domain.WaveDelta{Before: 0.20, After: 0.40}},
			{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "locked",
				Prerequisites: []string{"Auth:auth-w1"},
				ActionCount:   1, Actions: []domain.WaveAction{{Type: "add_dod", IssueID: "E1", Description: "d2"}},
				Delta: domain.WaveDelta{Before: 0.40, After: 0.65}},
		},
	}

	// when: restore waves and evaluate unlocks
	_, waves, completed, _, err := session.ResumeSession(baseDir, state)
	if err != nil {
		t.Fatalf("ResumeSession: %v", err)
	}
	waves = domain.EvaluateUnlocks(waves, completed)

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
	newWaves := []domain.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps v2", Status: "available"},
		{ID: "auth-w2", ClusterName: "Auth", Title: "DoD v2", Status: "locked"},
		{ID: "auth-w3", ClusterName: "Auth", Title: "New", Status: "locked"},
		{ID: "api-w1", ClusterName: "API", Title: "Endpoints v2", Status: "available"},
	}

	// when
	merged := domain.MergeCompletedStatus(oldCompleted, newWaves)

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
	state := &domain.SessionState{
		SessionID:      "old-session",
		ScanResultPath: "/project/.siren/.run/old-session/scan_result.json",
	}

	// when
	got := session.ResumeScanDir(state, "/project")

	// then: should derive scanDir from ScanResultPath
	want := "/project/.siren/.run/old-session"
	if got != want {
		t.Errorf("ResumeScanDir: expected %q, got %q", want, got)
	}
}

func TestResumeScanDir_EmptyScanResultPath_FallsBack(t *testing.T) {
	// given: state with empty ScanResultPath (fallback to ScanDir)
	state := &domain.SessionState{
		SessionID:      "new-session",
		ScanResultPath: "",
	}

	// when
	got := session.ResumeScanDir(state, "/project")

	// then: should fall back to ScanDir()
	want := domain.ScanDir("/project", "new-session")
	if got != want {
		t.Errorf("ResumeScanDir: expected %q, got %q", want, got)
	}
}

func TestResumeScanDir_CurrentPathFormat(t *testing.T) {
	// given: state with ScanResultPath using current .siren/.run/ format
	state := &domain.SessionState{
		SessionID:      "current-session",
		ScanResultPath: "/project/.siren/.run/current-session/scan_result.json",
	}

	// when
	got := session.ResumeScanDir(state, "/project")

	// then: should derive from ScanResultPath
	want := "/project/.siren/.run/current-session"
	if got != want {
		t.Errorf("ResumeScanDir: expected %q, got %q", want, got)
	}
}

func TestMergeOldWaves_CarriesForwardFailedClusters(t *testing.T) {
	oldWaves := []domain.Wave{
		{ID: "1", ClusterName: "auth", Title: "Auth wave", Status: "completed"},
		{ID: "2", ClusterName: "db", Title: "DB wave", Status: "pending"},
		{ID: "3", ClusterName: "api", Title: "API wave", Status: "completed"},
	}
	// Only "auth" and "api" regenerated; "db" failed but is still in scan.
	newWaves := []domain.Wave{
		{ID: "1", ClusterName: "auth", Title: "Auth wave v2"},
		{ID: "3", ClusterName: "api", Title: "API wave v2"},
	}
	scannedClusters := map[string]bool{"auth": true, "db": true, "api": true}
	failedNames := map[string]bool{"db": true}

	merged := domain.MergeOldWaves(oldWaves, newWaves, scannedClusters, failedNames)

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
	oldWaves := []domain.Wave{
		{ID: "1", ClusterName: "auth", Title: "Auth wave", Status: "completed"},
		{ID: "2", ClusterName: "obsolete", Title: "Obsolete wave", Status: "completed"},
	}
	// "auth" regenerated; "obsolete" is gone from scan entirely.
	newWaves := []domain.Wave{
		{ID: "1", ClusterName: "auth", Title: "Auth wave v2"},
	}
	scannedClusters := map[string]bool{"auth": true}
	failedNames := map[string]bool{} // no failures

	merged := domain.MergeOldWaves(oldWaves, newWaves, scannedClusters, failedNames)

	if len(merged) != 1 {
		t.Fatalf("expected 1 wave (obsolete dropped), got %d: %v", len(merged), merged)
	}
	if merged[0].Title != "Auth wave v2" {
		t.Errorf("should use new auth wave, got %q", merged[0].Title)
	}
}

func TestMergeOldWaves_AllClustersRegenerated(t *testing.T) {
	oldWaves := []domain.Wave{
		{ID: "1", ClusterName: "auth", Title: "Auth old"},
	}
	newWaves := []domain.Wave{
		{ID: "1", ClusterName: "auth", Title: "Auth new"},
	}
	scannedClusters := map[string]bool{"auth": true}
	failedNames := map[string]bool{} // no failures

	merged := domain.MergeOldWaves(oldWaves, newWaves, scannedClusters, failedNames)

	if len(merged) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(merged))
	}
	if merged[0].Title != "Auth new" {
		t.Errorf("should use new wave, got %q", merged[0].Title)
	}
}

func TestMergeOldWaves_NoClustersRegenerated(t *testing.T) {
	oldWaves := []domain.Wave{
		{ID: "1", ClusterName: "auth", Title: "Auth old", Status: "completed"},
		{ID: "2", ClusterName: "db", Title: "DB old", Status: "pending"},
	}
	var newWaves []domain.Wave
	scannedClusters := map[string]bool{"auth": true, "db": true}
	failedNames := map[string]bool{"auth": true, "db": true}

	merged := domain.MergeOldWaves(oldWaves, newWaves, scannedClusters, failedNames)

	if len(merged) != 2 {
		t.Fatalf("expected 2 carried-forward waves, got %d", len(merged))
	}
	if merged[0].ClusterName != "auth" || merged[1].ClusterName != "db" {
		t.Errorf("all old waves should be carried forward, got %v", merged)
	}
}

func TestMergeOldWaves_DuplicateName_PartialFailure(t *testing.T) {
	// Two "Auth" clusters existed; one regenerated, one failed.
	oldWaves := []domain.Wave{
		{ID: "1", ClusterName: "Auth", Title: "Auth instance 1", Status: "completed"},
		{ID: "2", ClusterName: "Auth", Title: "Auth instance 2", Status: "completed"},
	}
	newWaves := []domain.Wave{
		{ID: "10", ClusterName: "Auth", Title: "Auth new"},
	}
	scannedClusters := map[string]bool{"Auth": true}
	// detectFailedClusterNames: 2 input "Auth", 1 success → failed
	failedNames := map[string]bool{"Auth": true}

	merged := domain.MergeOldWaves(oldWaves, newWaves, scannedClusters, failedNames)

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
	oldWaves := []domain.Wave{
		{ID: "1", ClusterName: "Auth", Title: "Auth old 1"},
		{ID: "2", ClusterName: "Auth", Title: "Auth old 2"},
	}
	newWaves := []domain.Wave{
		{ID: "10", ClusterName: "Auth", Title: "Auth new 1"},
		{ID: "20", ClusterName: "Auth", Title: "Auth new 2"},
	}
	scannedClusters := map[string]bool{"Auth": true}
	failedNames := map[string]bool{} // both succeeded

	merged := domain.MergeOldWaves(oldWaves, newWaves, scannedClusters, failedNames)

	// Only new waves, no carry-forward.
	if len(merged) != 2 {
		t.Fatalf("expected 2 new waves, got %d: %v", len(merged), merged)
	}
	if merged[0].Title != "Auth new 1" || merged[1].Title != "Auth new 2" {
		t.Errorf("should only have new waves, got %v", merged)
	}
}

func TestMergeOldWaves_DuplicateName_DedupsWaveKey(t *testing.T) {
	// Copilot review: when partialFailure carries forward old waves,
	// old waves whose WaveKey already exists in newWaves must be skipped
	// to avoid duplicate WaveKey entries in the merged slice.
	//
	// Scenario: Two "Auth" instances. Instance 1 succeeds and regenerates
	// wave ABC-123. Instance 2 fails. Old session also had wave ABC-123.
	// Without dedup, Auth:ABC-123 appears twice.
	oldWaves := []domain.Wave{
		{ID: "ABC-123", ClusterName: "Auth", Title: "Auth old", Status: "completed"},
		{ID: "DEF-456", ClusterName: "Auth", Title: "Auth old 2", Status: "pending"},
	}
	newWaves := []domain.Wave{
		{ID: "ABC-123", ClusterName: "Auth", Title: "Auth regenerated"},
	}
	scannedClusters := map[string]bool{"Auth": true}
	failedNames := map[string]bool{"Auth": true} // instance 2 failed

	merged := domain.MergeOldWaves(oldWaves, newWaves, scannedClusters, failedNames)

	// Expected: 1 new (ABC-123) + 1 old carried forward (DEF-456) = 2
	// ABC-123 must NOT appear twice.
	if len(merged) != 2 {
		t.Fatalf("expected 2 waves (deduped), got %d: %v", len(merged), merged)
	}

	// Verify no duplicate WaveKeys
	seen := make(map[string]bool)
	for _, w := range merged {
		key := w.ClusterName + ":" + w.ID
		if seen[key] {
			t.Errorf("duplicate WaveKey %q in merged result", key)
		}
		seen[key] = true
	}
}
