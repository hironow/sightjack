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
		Scribe: ScribeConfig{Enabled: true},
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

	// then: scribe ADR prompt was generated
	scribePrompt := filepath.Join(scanDir, "scribe_sample_sample-w1_prompt.md")
	if _, err := os.Stat(scribePrompt); os.IsNotExist(err) {
		t.Error("scribe_sample_sample-w1_prompt.md not generated — dry-run did not reach scribe step")
	}
}

func TestRunSession_DryRunSkipsScribeWhenDisabled(t *testing.T) {
	// given: dry-run with Scribe disabled
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
		Scribe: ScribeConfig{Enabled: false},
	}
	sessionID := "test-dry-run-no-scribe"
	ctx := context.Background()

	// when
	err := RunSession(ctx, cfg, baseDir, sessionID, true, nil)

	// then: no error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// then: scribe prompt should NOT be generated
	scanDir := ScanDir(baseDir, sessionID)
	scribePrompt := filepath.Join(scanDir, "scribe_sample_sample-w1_prompt.md")
	if _, err := os.Stat(scribePrompt); !os.IsNotExist(err) {
		t.Error("scribe prompt should not be generated when Scribe is disabled")
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

func TestApplyModifiedWave_PreservesIdentity(t *testing.T) {
	// given: original wave with known identity
	original := Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Original Title",
		Actions:     []WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "original"}},
		Delta:       WaveDelta{Before: 0.25, After: 0.40},
		Status:      "available",
	}
	// given: architect returns modified wave with CHANGED identity fields
	modified := Wave{
		ID:          "new-w1",
		ClusterName: "Authentication",
		Title:       "Better Title",
		Actions: []WaveAction{
			{Type: "add_dependency", IssueID: "ENG-101", Description: "original"},
			{Type: "add_dod", IssueID: "ENG-101", Description: "new action"},
		},
		Delta:  WaveDelta{Before: 0.25, After: 0.50},
		Status: "modified",
	}

	// when: no prerequisites, empty completed map
	result := ApplyModifiedWave(original, modified, map[string]bool{})

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
	original := Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Original",
		Status:      "available",
	}
	// given: architect adds a prerequisite that hasn't been completed
	modified := Wave{
		ID:            "auth-w1",
		ClusterName:   "Auth",
		Title:         "Modified",
		Prerequisites: []string{"API:api-w1"},
		Actions:       []WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "new"}},
	}
	// given: api-w1 is NOT in the completed map
	completed := map[string]bool{}

	// when
	result := ApplyModifiedWave(original, modified, completed)

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
	original := Wave{
		ID:          "auth-w2",
		ClusterName: "Auth",
		Title:       "Original",
		Status:      "available",
	}
	modified := Wave{
		ID:            "auth-w2",
		ClusterName:   "Auth",
		Title:         "Modified",
		Prerequisites: []string{"Auth:auth-w1"},
		Actions:       []WaveAction{{Type: "add_dod", IssueID: "ENG-102", Description: "new"}},
	}
	completed := map[string]bool{"Auth:auth-w1": true}

	// when
	result := ApplyModifiedWave(original, modified, completed)

	// then: status should remain "available" because prerequisite is met
	if result.Status != "available" {
		t.Errorf("expected 'available' for met prerequisites, got '%s'", result.Status)
	}
}

func TestApplyModifiedWave_NormalizesBarePrerequisites(t *testing.T) {
	// given: architect returns bare ID "auth-w1" instead of composite "Auth:auth-w1"
	original := Wave{
		ID:          "auth-w2",
		ClusterName: "Auth",
		Title:       "Original",
		Status:      "available",
	}
	modified := Wave{
		ID:            "auth-w2",
		ClusterName:   "Auth",
		Title:         "Modified",
		Prerequisites: []string{"auth-w1"}, // bare ID, not composite
		Actions:       []WaveAction{{Type: "add_dod", IssueID: "ENG-102", Description: "new"}},
	}
	// given: "Auth:auth-w1" IS completed (composite key)
	completed := map[string]bool{"Auth:auth-w1": true}

	// when
	result := ApplyModifiedWave(original, modified, completed)

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
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Wave 1", Status: "available"},
		{ID: "auth-w2", ClusterName: "Auth", Title: "Wave 2", Status: "available"},
	}
	modified := Wave{
		ID:            "auth-w1",
		ClusterName:   "Auth",
		Title:         "Modified",
		Prerequisites: []string{"API:api-w1"}, // unmet
		Actions:       []WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "new"}},
	}
	completed := map[string]bool{}

	// when: apply modification
	result := ApplyModifiedWave(waves[0], modified, completed)
	if result.Status != "locked" {
		t.Fatalf("precondition: expected locked, got %s", result.Status)
	}

	// when: propagate back to waves slice
	PropagateWaveUpdate(waves, result)

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
	original := Wave{
		ID:            "auth-w2",
		ClusterName:   "Auth",
		Title:         "Original",
		Status:        "locked",
		Prerequisites: []string{"Auth:auth-w1"},
		Delta:         WaveDelta{Before: 0.20, After: 0.40},
	}
	modified := Wave{
		Title:         "Modified Title",
		Prerequisites: nil, // architect omitted the field
		Actions:       []WaveAction{{Type: "add_dod", IssueID: "ENG-102", Description: "new"}},
	}
	completed := map[string]bool{} // auth-w1 NOT completed

	// when
	result := ApplyModifiedWave(original, modified, completed)

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
	original := Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Original",
		Status:      "available",
		Delta:       WaveDelta{Before: 0.25, After: 0.50},
	}
	modified := Wave{
		Title:   "Modified Title",
		Actions: []WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "new"}},
		Delta:   WaveDelta{}, // zero value — architect omitted the field
	}
	completed := map[string]bool{}

	// when
	result := ApplyModifiedWave(original, modified, completed)

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
	newWaves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Status: "available"},
		{ID: "auth-w2", ClusterName: "Auth", Title: "DoD", Status: "locked"},
		{ID: "api-w2", ClusterName: "API", Title: "New Wave", Status: "available"},
	}

	// when
	merged := MergeCompletedStatus(oldCompleted, newWaves)

	// then: auth-w1 should be completed (was in old)
	for _, w := range merged {
		if WaveKey(w) == "Auth:auth-w1" && w.Status != "completed" {
			t.Errorf("expected Auth:auth-w1 completed, got %s", w.Status)
		}
	}
	// then: api-w1 not in new waves (dropped from Linear) — not present at all
	for _, w := range merged {
		if WaveKey(w) == "API:api-w1" {
			t.Error("API:api-w1 should not appear in merged result")
		}
	}
	// then: auth-w2 and api-w2 keep original status
	for _, w := range merged {
		if WaveKey(w) == "Auth:auth-w2" && w.Status != "locked" {
			t.Errorf("expected Auth:auth-w2 locked, got %s", w.Status)
		}
		if WaveKey(w) == "API:api-w2" && w.Status != "available" {
			t.Errorf("expected API:api-w2 available, got %s", w.Status)
		}
	}
}

func TestMergeCompletedStatus_EmptyOld(t *testing.T) {
	// given: no old completed waves
	oldCompleted := map[string]bool{}
	newWaves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Status: "available"},
	}

	// when
	merged := MergeCompletedStatus(oldCompleted, newWaves)

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
	var newWaves []Wave

	// when
	merged := MergeCompletedStatus(oldCompleted, newWaves)

	// then
	if len(merged) != 0 {
		t.Errorf("expected 0 waves, got %d", len(merged))
	}
}

func TestBuildWaveStates_IncludesFullFields(t *testing.T) {
	// given
	waves := []Wave{
		{
			ID:            "auth-w1",
			ClusterName:   "Auth",
			Title:         "Deps",
			Status:        "completed",
			Prerequisites: []string{"Auth:auth-w0"},
			Actions: []WaveAction{
				{Type: "add_dependency", IssueID: "ENG-101", Description: "dep"},
				{Type: "add_dod", IssueID: "ENG-102", Description: "dod"},
			},
			Description: "Order dependencies first",
			Delta:       WaveDelta{Before: 0.20, After: 0.40},
		},
	}

	// when
	states := BuildWaveStates(waves)

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
	states := []WaveState{
		{
			ID:            "auth-w1",
			ClusterName:   "Auth",
			Title:         "Deps",
			Status:        "completed",
			Prerequisites: []string{"Auth:auth-w0"},
			ActionCount:   2,
			Actions: []WaveAction{
				{Type: "add_dependency", IssueID: "ENG-101", Description: "dep"},
				{Type: "add_dod", IssueID: "ENG-102", Description: "dod"},
			},
			Description: "Order dependencies first",
			Delta:       WaveDelta{Before: 0.20, After: 0.40},
		},
		{
			ID:          "auth-w2",
			ClusterName: "Auth",
			Title:       "DoD",
			Status:      "available",
			ActionCount: 1,
			Actions:     []WaveAction{{Type: "add_dod", IssueID: "ENG-103", Description: "dod2"}},
			Delta:       WaveDelta{Before: 0.40, After: 0.60},
		},
	}

	// when
	waves := RestoreWaves(states)

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
	var states []WaveState

	// when
	waves := RestoreWaves(states)

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
	cfg := &Config{
		Lang:   "en",
		Claude: ClaudeConfig{Command: "claude", TimeoutSec: 60},
		Scan:   ScanConfig{MaxConcurrency: 1, ChunkSize: 50},
		Linear: LinearConfig{Team: "ENG", Project: "Test"},
		Scribe: ScribeConfig{Enabled: true},
	}
	sessionID := "test-no-cache"
	ctx := context.Background()

	// when
	err := RunSession(ctx, cfg, baseDir, sessionID, true, nil)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	scanDir := ScanDir(baseDir, sessionID)
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	if _, err := os.Stat(scanResultPath); !os.IsNotExist(err) {
		t.Error("scan_result.json should not exist in dry-run mode")
	}
}

func TestApplyModifiedWave_PreservesOriginalActionsWhenNil(t *testing.T) {
	// given: original wave has actions, modified wave omits them (nil from JSON)
	originalActions := []WaveAction{
		{Type: "add_dod", IssueID: "ENG-101", Description: "Original action 1"},
		{Type: "add_dependency", IssueID: "ENG-102", Description: "Original action 2"},
	}
	original := Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Original",
		Status:      "available",
		Actions:     originalActions,
		Delta:       WaveDelta{Before: 0.20, After: 0.40},
	}
	modified := Wave{
		Title:   "Modified Title",
		Actions: nil, // architect omitted the field
	}
	completed := map[string]bool{}

	// when
	result := ApplyModifiedWave(original, modified, completed)

	// then: actions should fall back to original
	if len(result.Actions) != 2 {
		t.Fatalf("expected 2 original actions preserved, got %d", len(result.Actions))
	}
	if result.Actions[0].IssueID != "ENG-101" {
		t.Errorf("expected first action ENG-101, got %s", result.Actions[0].IssueID)
	}
}
