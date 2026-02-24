package session_test

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/session"
)

func TestPromptWaveSelection(t *testing.T) {
	waves := []sightjack.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Delta: sightjack.WaveDelta{Before: 0.25, After: 0.40}},
		{ID: "api-w1", ClusterName: "API", Title: "Split", Delta: sightjack.WaveDelta{Before: 0.30, After: 0.45}},
	}

	scanner := bufio.NewScanner(strings.NewReader("1\n"))
	var output bytes.Buffer
	ctx := context.Background()

	selected, err := session.PromptWaveSelection(ctx, &output, scanner, waves)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.ID != "auth-w1" {
		t.Errorf("expected auth-w1, got %s", selected.ID)
	}
	if !strings.Contains(output.String(), "Auth") {
		t.Error("expected Auth in output")
	}
}

func TestPromptWaveSelection_Quit(t *testing.T) {
	waves := []sightjack.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps"},
	}

	scanner := bufio.NewScanner(strings.NewReader("q\n"))
	var output bytes.Buffer
	ctx := context.Background()

	_, err := session.PromptWaveSelection(ctx, &output, scanner, waves)
	if err != session.ErrQuit {
		t.Errorf("expected ErrQuit, got %v", err)
	}
}

func TestPromptWaveApproval_Approve(t *testing.T) {
	wave := sightjack.Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Dependency Ordering",
		Actions: []sightjack.WaveAction{
			{Type: "add_dependency", IssueID: "ENG-101", Description: "Auth before token"},
		},
		Delta: sightjack.WaveDelta{Before: 0.25, After: 0.40},
	}

	scanner := bufio.NewScanner(strings.NewReader("a\n"))
	var output bytes.Buffer
	ctx := context.Background()

	choice, err := session.PromptWaveApproval(ctx, &output, scanner, wave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != sightjack.ApprovalApprove {
		t.Errorf("expected ApprovalApprove, got %d", choice)
	}
}

func TestPromptWaveApproval_Reject(t *testing.T) {
	wave := sightjack.Wave{ID: "auth-w1", Actions: []sightjack.WaveAction{}}

	scanner := bufio.NewScanner(strings.NewReader("r\n"))
	var output bytes.Buffer
	ctx := context.Background()

	choice, err := session.PromptWaveApproval(ctx, &output, scanner, wave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != sightjack.ApprovalReject {
		t.Errorf("expected ApprovalReject, got %d", choice)
	}
}

func TestPromptWaveApproval_Discuss(t *testing.T) {
	wave := sightjack.Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Dependency Ordering",
		Actions:     []sightjack.WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
		Delta:       sightjack.WaveDelta{Before: 0.25, After: 0.40},
	}

	scanner := bufio.NewScanner(strings.NewReader("d\n"))
	var output bytes.Buffer
	ctx := context.Background()

	choice, err := session.PromptWaveApproval(ctx, &output, scanner, wave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != sightjack.ApprovalDiscuss {
		t.Errorf("expected ApprovalDiscuss, got %d", choice)
	}
	if !strings.Contains(output.String(), "[d] Discuss") {
		t.Error("expected [d] Discuss in prompt output")
	}
}

func TestPromptSequence_SelectionThenApproval(t *testing.T) {
	// given: piped input with both selection and approval on one reader
	waves := []sightjack.Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps",
			Actions: []sightjack.WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
			Delta:   sightjack.WaveDelta{Before: 0.25, After: 0.40}},
	}
	scanner := bufio.NewScanner(strings.NewReader("1\na\n"))
	var output bytes.Buffer
	ctx := context.Background()

	// when: selection then approval using same scanner
	selected, err := session.PromptWaveSelection(ctx, &output, scanner, waves)
	if err != nil {
		t.Fatalf("selection: unexpected error: %v", err)
	}
	if selected.ID != "auth-w1" {
		t.Errorf("expected auth-w1, got %s", selected.ID)
	}

	choice, err := session.PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("approval: unexpected error: %v", err)
	}

	// then: approval should read "a" from the same scanner
	if choice != sightjack.ApprovalApprove {
		t.Errorf("expected ApprovalApprove, got %d (scanner likely lost buffered input)", choice)
	}
}

func TestScanLine_ContextCancelled(t *testing.T) {
	// given: a scanner that blocks forever (no input), with a cancelled context
	r, _ := io.Pipe() // blocks on read forever
	scanner := bufio.NewScanner(r)
	ctx, cancel := context.WithCancel(context.Background())

	// when: cancel context after a short delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := session.ScanLine(ctx, scanner)

	// then: should return context error, not block
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestScanLine_NormalInput(t *testing.T) {
	// given
	scanner := bufio.NewScanner(strings.NewReader("hello\n"))
	ctx := context.Background()

	// when
	line, err := session.ScanLine(ctx, scanner)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if line != "hello" {
		t.Errorf("expected hello, got %s", line)
	}
}

func TestDisplayRippleEffects(t *testing.T) {
	ripples := []sightjack.Ripple{
		{ClusterName: "API", Description: "W2 unlocked"},
		{ClusterName: "DB", Description: "New dependency added"},
	}

	var output bytes.Buffer
	session.DisplayRippleEffects(&output, ripples)

	out := output.String()
	if !strings.Contains(out, "API") {
		t.Error("expected API in ripple output")
	}
	if !strings.Contains(out, "W2 unlocked") {
		t.Error("expected ripple description in output")
	}
}

func TestPromptDiscussTopic(t *testing.T) {
	// given
	scanner := bufio.NewScanner(strings.NewReader("Should we split ENG-101?\n"))
	var output bytes.Buffer
	ctx := context.Background()

	// when
	topic, err := session.PromptDiscussTopic(ctx, &output, scanner)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if topic != "Should we split ENG-101?" {
		t.Errorf("expected topic text, got: %s", topic)
	}
	if !strings.Contains(output.String(), "Topic") {
		t.Error("expected Topic prompt in output")
	}
}

func TestPromptDiscussTopic_Quit(t *testing.T) {
	// given
	scanner := bufio.NewScanner(strings.NewReader("q\n"))
	var output bytes.Buffer
	ctx := context.Background()

	// when
	_, err := session.PromptDiscussTopic(ctx, &output, scanner)

	// then
	if err != session.ErrQuit {
		t.Errorf("expected ErrQuit, got %v", err)
	}
}

func TestPromptDiscussTopic_Empty(t *testing.T) {
	// given: empty input should error
	scanner := bufio.NewScanner(strings.NewReader("\n"))
	var output bytes.Buffer
	ctx := context.Background()

	// when
	_, err := session.PromptDiscussTopic(ctx, &output, scanner)

	// then
	if err == nil {
		t.Fatal("expected error for empty topic")
	}
	if err == session.ErrQuit {
		t.Error("expected non-quit error for empty topic")
	}
}

func TestDisplayArchitectResponse_WithModifiedWave(t *testing.T) {
	// given
	resp := &sightjack.ArchitectResponse{
		Analysis: "Splitting is unnecessary for this scale.",
		ModifiedWave: &sightjack.Wave{
			ID:          "auth-w1",
			ClusterName: "Auth",
			Title:       "Dependency Ordering",
			Actions: []sightjack.WaveAction{
				{Type: "add_dependency", IssueID: "ENG-101", Description: "Auth before token"},
				{Type: "add_dod", IssueID: "ENG-101", Description: "Middleware interface"},
			},
			Delta: sightjack.WaveDelta{Before: 0.25, After: 0.42},
		},
		Reasoning: "Project scale favors fewer issues.",
	}
	var output bytes.Buffer

	// when
	session.DisplayArchitectResponse(&output, resp)

	// then
	out := output.String()
	if !strings.Contains(out, "Splitting is unnecessary") {
		t.Error("expected analysis text in output")
	}
	if !strings.Contains(out, "Middleware interface") {
		t.Error("expected modified action in output")
	}
	if !strings.Contains(out, "Project scale") {
		t.Error("expected reasoning in output")
	}
}

func TestDisplayArchitectResponse_NoModifications(t *testing.T) {
	// given
	resp := &sightjack.ArchitectResponse{
		Analysis:     "Current actions look good.",
		ModifiedWave: nil,
		Reasoning:    "No changes needed.",
	}
	var output bytes.Buffer

	// when
	session.DisplayArchitectResponse(&output, resp)

	// then
	out := output.String()
	if !strings.Contains(out, "Current actions look good") {
		t.Error("expected analysis text in output")
	}
	if strings.Contains(out, "Modified") {
		t.Error("should not show modified section when no modifications")
	}
}

func TestPromptWaveApproval_UppercaseInput(t *testing.T) {
	wave := sightjack.Wave{
		ID: "auth-w1", ClusterName: "Auth", Title: "Test",
		Actions: []sightjack.WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
		Delta:   sightjack.WaveDelta{Before: 0.25, After: 0.40},
	}

	tests := []struct {
		name     string
		input    string
		expected sightjack.ApprovalChoice
	}{
		{"uppercase A", "A\n", sightjack.ApprovalApprove},
		{"uppercase D", "D\n", sightjack.ApprovalDiscuss},
		{"uppercase R", "R\n", sightjack.ApprovalReject},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := bufio.NewScanner(strings.NewReader(tt.input))
			var output bytes.Buffer
			ctx := context.Background()

			choice, err := session.PromptWaveApproval(ctx, &output, scanner, wave)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if choice != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, choice)
			}
		})
	}
}

func TestPromptWaveApproval_UppercaseQ(t *testing.T) {
	wave := sightjack.Wave{ID: "auth-w1", Actions: []sightjack.WaveAction{}}
	scanner := bufio.NewScanner(strings.NewReader("Q\n"))
	var output bytes.Buffer
	ctx := context.Background()

	_, err := session.PromptWaveApproval(ctx, &output, scanner, wave)
	if err != session.ErrQuit {
		t.Errorf("expected ErrQuit for uppercase Q, got %v", err)
	}
}

func TestPromptWaveApproval_InvalidInput(t *testing.T) {
	wave := sightjack.Wave{ID: "auth-w1", Actions: []sightjack.WaveAction{}}

	tests := []struct {
		name  string
		input string
	}{
		{"unknown letter", "x\n"},
		{"number", "2\n"},
		{"empty after trim", "   \n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := bufio.NewScanner(strings.NewReader(tt.input))
			var output bytes.Buffer
			ctx := context.Background()

			_, err := session.PromptWaveApproval(ctx, &output, scanner, wave)
			if err == nil {
				t.Fatal("expected error for invalid input")
			}
			if err == session.ErrQuit {
				t.Error("expected non-quit error for invalid input")
			}
		})
	}
}

func TestPromptDiscussTopic_PaddedQuit(t *testing.T) {
	// given: "  q  " should be treated as quit after TrimSpace
	scanner := bufio.NewScanner(strings.NewReader("  q  \n"))
	var output bytes.Buffer
	ctx := context.Background()

	// when
	_, err := session.PromptDiscussTopic(ctx, &output, scanner)

	// then
	if err != session.ErrQuit {
		t.Errorf("expected ErrQuit for padded 'q', got %v", err)
	}
}

func TestPromptDiscussTopic_UppercaseQuit(t *testing.T) {
	// given: uppercase "Q" should also quit (consistent with PromptWaveApproval)
	scanner := bufio.NewScanner(strings.NewReader("Q\n"))
	var output bytes.Buffer
	ctx := context.Background()

	// when
	_, err := session.PromptDiscussTopic(ctx, &output, scanner)

	// then
	if err != session.ErrQuit {
		t.Errorf("expected ErrQuit for uppercase 'Q', got %v", err)
	}
}

func TestDisplayArchitectResponse_ZeroDelta(t *testing.T) {
	// given: ModifiedWave with zero-value delta (architect forgot to populate)
	resp := &sightjack.ArchitectResponse{
		Analysis: "Modified wave.",
		ModifiedWave: &sightjack.Wave{
			ID:      "auth-w1",
			Actions: []sightjack.WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "test"}},
			Delta:   sightjack.WaveDelta{Before: 0.0, After: 0.0},
		},
	}
	var output bytes.Buffer

	// when
	session.DisplayArchitectResponse(&output, resp)

	// then
	out := output.String()
	if !strings.Contains(out, "0% -> 0%") {
		t.Errorf("expected '0%% -> 0%%' for zero delta, got: %s", out)
	}
}

func TestDisplayArchitectResponse_EmptyAnalysis(t *testing.T) {
	// given: empty analysis string (Claude omitted the field)
	resp := &sightjack.ArchitectResponse{
		Analysis:  "",
		Reasoning: "some reasoning",
	}
	var output bytes.Buffer

	// when
	session.DisplayArchitectResponse(&output, resp)

	// then
	out := output.String()
	// Should still render the [Architect] prefix line
	if !strings.Contains(out, "[Architect]") {
		t.Error("expected [Architect] label even with empty analysis")
	}
}

func TestDisplayArchitectResponse_ModifiedWaveNilActions(t *testing.T) {
	// given: ModifiedWave is non-nil but Actions is nil
	resp := &sightjack.ArchitectResponse{
		Analysis: "Simplified wave.",
		ModifiedWave: &sightjack.Wave{
			ID:      "auth-w1",
			Actions: nil,
			Delta:   sightjack.WaveDelta{Before: 0.25, After: 0.35},
		},
	}
	var output bytes.Buffer

	// when
	session.DisplayArchitectResponse(&output, resp)

	// then
	out := output.String()
	if !strings.Contains(out, "Modified actions (0)") {
		t.Errorf("expected 'Modified actions (0)' for nil actions, got: %s", out)
	}
}

func TestDisplayScribeResponse(t *testing.T) {
	// given
	resp := &sightjack.ScribeResponse{
		ADRID:     "0003",
		Title:     "adopt-event-sourcing",
		Content:   "# 0003. Adopt Event Sourcing",
		Reasoning: "Discussion revealed need for event sourcing.",
	}
	var output bytes.Buffer

	// when
	session.DisplayScribeResponse(&output, resp)

	// then
	out := output.String()
	if !strings.Contains(out, "[Scribe]") {
		t.Error("expected [Scribe] label in output")
	}
	if !strings.Contains(out, "0003") {
		t.Error("expected ADR ID in output")
	}
	if !strings.Contains(out, "adopt-event-sourcing") {
		t.Error("expected ADR title in output")
	}
}

func TestDisplayScribeResponse_EmptyTitle(t *testing.T) {
	// given
	resp := &sightjack.ScribeResponse{
		ADRID: "0001",
		Title: "",
	}
	var output bytes.Buffer

	// when
	session.DisplayScribeResponse(&output, resp)

	// then
	out := output.String()
	if !strings.Contains(out, "[Scribe]") {
		t.Error("expected [Scribe] label even with empty title")
	}
	if !strings.Contains(out, "0001") {
		t.Error("expected ADR ID in output")
	}
}

func TestPromptResume_ChooseResume(t *testing.T) {
	state := &sightjack.SessionState{
		Completeness: 0.62,
		ADRCount:     4,
		LastScanned:  time.Date(2026, 2, 17, 15, 30, 0, 0, time.UTC),
	}
	input := "r\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	choice, err := session.PromptResume(ctx, &output, scanner, state)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != sightjack.ResumeChoiceResume {
		t.Errorf("expected ResumeChoiceResume, got %d", choice)
	}
	if !strings.Contains(output.String(), "62%") {
		t.Error("expected completeness in prompt")
	}
	if !strings.Contains(output.String(), "4 ADRs") {
		t.Error("expected ADR count in prompt")
	}
}

func TestPromptResume_ChooseNew(t *testing.T) {
	state := &sightjack.SessionState{Completeness: 0.30, ADRCount: 1, LastScanned: time.Now()}
	input := "n\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	choice, err := session.PromptResume(ctx, &output, scanner, state)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != sightjack.ResumeChoiceNew {
		t.Errorf("expected ResumeChoiceNew, got %d", choice)
	}
}

func TestPromptResume_ChooseRescan(t *testing.T) {
	state := &sightjack.SessionState{Completeness: 0.50, ADRCount: 2, LastScanned: time.Now()}
	input := "s\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	choice, err := session.PromptResume(ctx, &output, scanner, state)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != sightjack.ResumeChoiceRescan {
		t.Errorf("expected ResumeChoiceRescan, got %d", choice)
	}
}

func TestPromptResume_ChooseQuit(t *testing.T) {
	state := &sightjack.SessionState{Completeness: 0.50, LastScanned: time.Now()}
	input := "q\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	_, err := session.PromptResume(ctx, &output, scanner, state)

	if err != session.ErrQuit {
		t.Errorf("expected ErrQuit, got %v", err)
	}
}

func TestPromptResume_InvalidInput(t *testing.T) {
	state := &sightjack.SessionState{Completeness: 0.50, LastScanned: time.Now()}
	input := "x\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	var output bytes.Buffer
	ctx := context.Background()

	_, err := session.PromptResume(ctx, &output, scanner, state)

	if err == nil {
		t.Fatal("expected error for invalid input")
	}
	if err == session.ErrQuit {
		t.Error("should not be ErrQuit for invalid input")
	}
}

func TestDisplayShibitoWarnings_Empty(t *testing.T) {
	// given: nil warnings
	var output bytes.Buffer

	// when
	session.DisplayShibitoWarnings(&output, nil)

	// then: no output
	if output.Len() != 0 {
		t.Errorf("expected no output for nil warnings, got: %s", output.String())
	}
}

func TestDisplayShibitoWarnings_EmptySlice(t *testing.T) {
	// given: empty slice
	var output bytes.Buffer

	// when
	session.DisplayShibitoWarnings(&output, []sightjack.ShibitoWarning{})

	// then: no output
	if output.Len() != 0 {
		t.Errorf("expected no output for empty warnings, got: %s", output.String())
	}
}

func TestDisplayShibitoWarnings_MultipleWarnings(t *testing.T) {
	// given
	warnings := []sightjack.ShibitoWarning{
		{ClosedIssueID: "ENG-50", CurrentIssueID: "ENG-201", Description: "Login retry pattern reappeared", RiskLevel: "high"},
		{ClosedIssueID: "ENG-30", CurrentIssueID: "ENG-180", Description: "Similar caching approach", RiskLevel: "medium"},
	}
	var output bytes.Buffer

	// when
	session.DisplayShibitoWarnings(&output, warnings)

	// then
	out := output.String()
	if !strings.Contains(out, "Shibito") {
		t.Error("expected Shibito header in output")
	}
	if !strings.Contains(out, "ENG-50") {
		t.Error("expected closed issue ID ENG-50")
	}
	if !strings.Contains(out, "ENG-201") {
		t.Error("expected current issue ID ENG-201")
	}
	if !strings.Contains(out, "high") {
		t.Error("expected risk level in output")
	}
	if !strings.Contains(out, "Login retry pattern reappeared") {
		t.Error("expected description in output")
	}
	if !strings.Contains(out, "ENG-30") {
		t.Error("expected second warning closed issue ID")
	}
}

func TestDisplayADRConflicts_Empty(t *testing.T) {
	// given: nil conflicts
	var output bytes.Buffer

	// when
	session.DisplayADRConflicts(&output, nil)

	// then: no output
	if output.Len() != 0 {
		t.Errorf("expected no output for nil conflicts, got: %s", output.String())
	}
}

func TestDisplayADRConflicts_EmptySlice(t *testing.T) {
	// given
	var output bytes.Buffer

	// when
	session.DisplayADRConflicts(&output, []sightjack.ADRConflict{})

	// then
	if output.Len() != 0 {
		t.Errorf("expected no output for empty conflicts, got: %s", output.String())
	}
}

func TestDisplayADRConflicts_MultipleConflicts(t *testing.T) {
	// given
	conflicts := []sightjack.ADRConflict{
		{ExistingADRID: "0001", Description: "Contradicts auth approach"},
		{ExistingADRID: "0002", Description: "API versioning conflict"},
	}
	var output bytes.Buffer

	// when
	session.DisplayADRConflicts(&output, conflicts)

	// then
	out := output.String()
	if !strings.Contains(out, "[Scribe]") {
		t.Error("expected [Scribe] label in output")
	}
	if !strings.Contains(out, "0001") {
		t.Error("expected existing ADR ID 0001")
	}
	if !strings.Contains(out, "Contradicts auth approach") {
		t.Error("expected conflict description")
	}
	if !strings.Contains(out, "0002") {
		t.Error("expected second ADR ID")
	}
}

func TestCompletedWaves_FiltersCompleted(t *testing.T) {
	// given
	waves := []sightjack.Wave{
		{ID: "w1", ClusterName: "Auth", Title: "Deps", Status: "completed"},
		{ID: "w2", ClusterName: "Auth", Title: "DoD", Status: "available"},
		{ID: "w3", ClusterName: "API", Title: "Split", Status: "completed"},
	}

	// when
	result := session.CompletedWaves(waves)

	// then
	if len(result) != 2 {
		t.Fatalf("expected 2 completed, got %d", len(result))
	}
	if result[0].ID != "w1" {
		t.Errorf("expected w1, got %s", result[0].ID)
	}
	if result[1].ID != "w3" {
		t.Errorf("expected w3, got %s", result[1].ID)
	}
}

func TestCompletedWaves_NoneCompleted(t *testing.T) {
	// given
	waves := []sightjack.Wave{
		{ID: "w1", Status: "available"},
		{ID: "w2", Status: "locked"},
	}

	// when
	result := session.CompletedWaves(waves)

	// then
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

func TestDisplayCompletedWaveActions_ShowsActions(t *testing.T) {
	// given
	var buf bytes.Buffer
	wave := sightjack.Wave{
		ClusterName: "Auth",
		Title:       "DoD",
		Actions: []sightjack.WaveAction{
			{Type: "add_dod", IssueID: "ENG-101", Description: "Auth flow"},
			{Type: "add_dependency", IssueID: "ENG-102", Description: "Token dep"},
		},
		Delta: sightjack.WaveDelta{Before: 0.25, After: 0.40},
	}

	// when
	session.DisplayCompletedWaveActions(&buf, wave)

	// then
	output := buf.String()
	if !strings.Contains(output, "(completed)") {
		t.Error("expected (completed) label")
	}
	if !strings.Contains(output, "add_dod") {
		t.Error("expected action type")
	}
	if !strings.Contains(output, "ENG-101") {
		t.Error("expected issue ID")
	}
	if !strings.Contains(output, "Actions applied (2)") {
		t.Error("expected action count")
	}
}

func TestDisplayCompletedWaveActions_NoActions(t *testing.T) {
	// given
	var buf bytes.Buffer
	wave := sightjack.Wave{ClusterName: "Auth", Title: "Empty"}

	// when
	session.DisplayCompletedWaveActions(&buf, wave)

	// then
	output := buf.String()
	if !strings.Contains(output, "Actions applied (0)") {
		t.Error("expected zero actions")
	}
}

func TestPromptCompletedWaveSelection_ValidChoice(t *testing.T) {
	// given
	var buf bytes.Buffer
	input := strings.NewReader("2\n")
	scanner := bufio.NewScanner(input)
	completed := []sightjack.Wave{
		{ID: "w1", ClusterName: "Auth", Title: "Deps", Delta: sightjack.WaveDelta{Before: 0.25, After: 0.40}},
		{ID: "w3", ClusterName: "API", Title: "Split", Delta: sightjack.WaveDelta{Before: 0.30, After: 0.45}},
	}

	// when
	selected, err := session.PromptCompletedWaveSelection(context.Background(), &buf, scanner, completed)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.ID != "w3" {
		t.Errorf("expected w3, got %s", selected.ID)
	}
}

func TestPromptCompletedWaveSelection_Back(t *testing.T) {
	// given
	var buf bytes.Buffer
	input := strings.NewReader("b\n")
	scanner := bufio.NewScanner(input)
	completed := []sightjack.Wave{{ID: "w1", ClusterName: "Auth", Title: "Deps"}}

	// when
	_, err := session.PromptCompletedWaveSelection(context.Background(), &buf, scanner, completed)

	// then: b returns ErrGoBack (back to main navigator)
	if err != session.ErrGoBack {
		t.Errorf("expected ErrGoBack, got %v", err)
	}
}

func TestPromptCompletedWaveSelection_Invalid(t *testing.T) {
	// given
	var buf bytes.Buffer
	input := strings.NewReader("99\n")
	scanner := bufio.NewScanner(input)
	completed := []sightjack.Wave{{ID: "w1", ClusterName: "Auth", Title: "Deps"}}

	// when
	_, err := session.PromptCompletedWaveSelection(context.Background(), &buf, scanner, completed)

	// then
	if err == nil || err == session.ErrQuit {
		t.Error("expected invalid selection error")
	}
}

func TestDisplayWaveCompletion_Basic(t *testing.T) {
	// given
	var buf bytes.Buffer
	wave := sightjack.Wave{ClusterName: "Auth", Title: "DoD", Delta: sightjack.WaveDelta{Before: 0.40, After: 0.65}}
	ripples := []sightjack.Ripple{
		{ClusterName: "DB", Description: "Wave 2 unlocked"},
		{ClusterName: "DB", Description: "Schema dependency added"},
		{ClusterName: "API", Description: "DoD updated: token validation"},
	}

	// when
	session.DisplayWaveCompletion(&buf, wave, ripples, 0.52, 2)

	// then
	output := buf.String()
	if !strings.Contains(output, "Auth") {
		t.Error("expected cluster name")
	}
	if !strings.Contains(output, "DoD") {
		t.Error("expected wave title")
	}
	if !strings.Contains(output, "40%") {
		t.Error("expected before completeness")
	}
	if !strings.Contains(output, "65%") {
		t.Error("expected after completeness")
	}
	// Grouped by cluster
	if !strings.Contains(output, "DB:") {
		t.Error("expected DB cluster group header")
	}
	if !strings.Contains(output, "API:") {
		t.Error("expected API cluster group header")
	}
	if !strings.Contains(output, "New waves available: 2") {
		t.Error("expected new waves count")
	}
	if !strings.Contains(output, "52%") {
		t.Error("expected overall completeness in progress bar")
	}
}

func TestDisplayWaveCompletion_NoRipples(t *testing.T) {
	// given
	var buf bytes.Buffer
	wave := sightjack.Wave{ClusterName: "Auth", Title: "Deps", Delta: sightjack.WaveDelta{Before: 0.25, After: 0.40}}

	// when
	session.DisplayWaveCompletion(&buf, wave, nil, 0.36, 0)

	// then
	output := buf.String()
	if strings.Contains(output, "Ripple") {
		t.Error("should not show ripple section when empty")
	}
	if strings.Contains(output, "New waves") {
		t.Error("should not show new waves when 0")
	}
}

func TestDisplayWaveCompletion_ZeroNewWaves(t *testing.T) {
	// given
	var buf bytes.Buffer
	wave := sightjack.Wave{ClusterName: "Auth", Title: "DoD", Delta: sightjack.WaveDelta{Before: 0.40, After: 0.65}}
	ripples := []sightjack.Ripple{{ClusterName: "DB", Description: "Updated"}}

	// when
	session.DisplayWaveCompletion(&buf, wave, ripples, 0.50, 0)

	// then
	output := buf.String()
	if strings.Contains(output, "New waves") {
		t.Error("should not show new waves when 0")
	}
}

func TestPromptWaveSelection_BackOption(t *testing.T) {
	// given
	var buf bytes.Buffer
	input := strings.NewReader("b\n")
	scanner := bufio.NewScanner(input)
	waves := []sightjack.Wave{{ID: "w1", ClusterName: "Auth", Title: "Deps"}}

	// when
	_, err := session.PromptWaveSelection(context.Background(), &buf, scanner, waves)

	// then
	if err != session.ErrGoBack {
		t.Errorf("expected ErrGoBack, got %v", err)
	}
}

func TestPromptWaveApproval_Selective(t *testing.T) {
	wave := sightjack.Wave{
		ClusterName: "Auth",
		Title:       "JWT middleware",
		Actions:     []sightjack.WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "Add spec"}},
		Delta:       sightjack.WaveDelta{Before: 0.3, After: 0.5},
	}
	input := strings.NewReader("s\n")
	scanner := bufio.NewScanner(input)
	var buf bytes.Buffer

	choice, err := session.PromptWaveApproval(context.Background(), &buf, scanner, wave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != sightjack.ApprovalSelective {
		t.Errorf("expected ApprovalSelective, got %d", choice)
	}
}

func TestPromptSelectiveApproval_AllSelected(t *testing.T) {
	actions := []sightjack.WaveAction{
		{Type: "add_dod", IssueID: "ENG-101", Description: "Add spec"},
		{Type: "add_dep", IssueID: "ENG-102", Description: "Add dependency"},
	}
	wave := sightjack.Wave{ClusterName: "Auth", Title: "Test", Actions: actions}
	input := strings.NewReader("done\n")
	scanner := bufio.NewScanner(input)
	var buf bytes.Buffer

	approved, rejected, err := session.PromptSelectiveApproval(context.Background(), &buf, scanner, wave)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(approved) != 2 {
		t.Errorf("approved: got %d, want 2", len(approved))
	}
	if len(rejected) != 0 {
		t.Errorf("rejected: got %d, want 0", len(rejected))
	}
}

func TestPromptSelectiveApproval_ToggleOne(t *testing.T) {
	actions := []sightjack.WaveAction{
		{Type: "add_dod", IssueID: "ENG-101", Description: "Add spec"},
		{Type: "add_dep", IssueID: "ENG-102", Description: "Add dependency"},
		{Type: "create", IssueID: "ENG-103", Description: "Create sub-issue"},
	}
	wave := sightjack.Wave{ClusterName: "Auth", Title: "Test", Actions: actions}
	input := strings.NewReader("3\ndone\n")
	scanner := bufio.NewScanner(input)
	var buf bytes.Buffer

	approved, rejected, err := session.PromptSelectiveApproval(context.Background(), &buf, scanner, wave)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(approved) != 2 {
		t.Errorf("approved: got %d, want 2", len(approved))
	}
	if len(rejected) != 1 {
		t.Errorf("rejected: got %d, want 1", len(rejected))
	}
	if rejected[0].IssueID != "ENG-103" {
		t.Errorf("rejected[0].IssueID: got %q, want %q", rejected[0].IssueID, "ENG-103")
	}
}

func TestPromptSelectiveApproval_SelectNoneThenDone(t *testing.T) {
	actions := []sightjack.WaveAction{
		{Type: "add_dod", IssueID: "ENG-101", Description: "Add spec"},
	}
	wave := sightjack.Wave{ClusterName: "Auth", Title: "Test", Actions: actions}
	input := strings.NewReader("n\ndone\n")
	scanner := bufio.NewScanner(input)
	var buf bytes.Buffer

	approved, rejected, err := session.PromptSelectiveApproval(context.Background(), &buf, scanner, wave)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(approved) != 0 {
		t.Errorf("approved: got %d, want 0", len(approved))
	}
	if len(rejected) != 1 {
		t.Errorf("rejected: got %d, want 1", len(rejected))
	}
}

func TestPromptSelectiveApproval_SelectAll(t *testing.T) {
	actions := []sightjack.WaveAction{
		{Type: "add_dod", IssueID: "ENG-101", Description: "Spec"},
		{Type: "add_dep", IssueID: "ENG-102", Description: "Dep"},
	}
	wave := sightjack.Wave{ClusterName: "Auth", Title: "Test", Actions: actions}
	input := strings.NewReader("n\na\ndone\n")
	scanner := bufio.NewScanner(input)
	var buf bytes.Buffer

	approved, _, err := session.PromptSelectiveApproval(context.Background(), &buf, scanner, wave)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(approved) != 2 {
		t.Errorf("approved: got %d, want 2", len(approved))
	}
}

func TestPromptSelectiveApproval_Quit(t *testing.T) {
	actions := []sightjack.WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "Spec"}}
	wave := sightjack.Wave{ClusterName: "Auth", Title: "Test", Actions: actions}
	input := strings.NewReader("q\n")
	scanner := bufio.NewScanner(input)
	var buf bytes.Buffer

	_, _, err := session.PromptSelectiveApproval(context.Background(), &buf, scanner, wave)
	if err != session.ErrQuit {
		t.Errorf("expected ErrQuit, got %v", err)
	}
}

func TestDisplayCompletedWaveActions(t *testing.T) {
	// given: a completed wave with multiple actions
	wave := sightjack.Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Dependency Ordering",
		Status:      "completed",
		Actions: []sightjack.WaveAction{
			{Type: "add_dependency", IssueID: "ENG-101", Description: "ENG-101 blocks ENG-102"},
			{Type: "add_dod", IssueID: "ENG-102", Description: "Token lifecycle defined"},
			{Type: "add_dod", IssueID: "ENG-103", Description: "Reset flow documented"},
		},
	}
	var buf bytes.Buffer

	// when
	session.DisplayCompletedWaveActions(&buf, wave)

	// then: output should contain wave title and all action descriptions
	output := buf.String()
	if !strings.Contains(output, "Dependency Ordering") {
		t.Errorf("expected wave title in output, got:\n%s", output)
	}
	if !strings.Contains(output, "ENG-101") {
		t.Errorf("expected ENG-101 in output, got:\n%s", output)
	}
	if !strings.Contains(output, "ENG-103") {
		t.Errorf("expected ENG-103 in output, got:\n%s", output)
	}
}

func TestDisplayCompletedWaveActions_Empty(t *testing.T) {
	// given: a completed wave with no actions
	wave := sightjack.Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Empty Wave",
		Status:      "completed",
		Actions:     nil,
	}
	var buf bytes.Buffer

	// when
	session.DisplayCompletedWaveActions(&buf, wave)

	// then: should not panic, should still show wave title
	output := buf.String()
	if !strings.Contains(output, "Empty Wave") {
		t.Errorf("expected wave title in output, got:\n%s", output)
	}
}

func TestDisplayScribeResponse_SanitizedFilename(t *testing.T) {
	// given: title with uppercase and spaces (would be sanitized on write)
	resp := &sightjack.ScribeResponse{
		ADRID:   "0005",
		Title:   "Use FastAPI for API Layer",
		Content: "# 0005. Use FastAPI for API Layer",
	}
	var output bytes.Buffer

	// when
	session.DisplayScribeResponse(&output, resp)

	// then: "Saved to" line should show sanitized filename, not raw title
	out := output.String()
	if !strings.Contains(out, "use_fastapi_for_api_layer") {
		t.Errorf("expected sanitized title in file path, got: %s", out)
	}
}
