package sightjack

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestPromptWaveSelection(t *testing.T) {
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps", Delta: WaveDelta{Before: 0.25, After: 0.40}},
		{ID: "api-w1", ClusterName: "API", Title: "Split", Delta: WaveDelta{Before: 0.30, After: 0.45}},
	}

	scanner := bufio.NewScanner(strings.NewReader("1\n"))
	var output bytes.Buffer
	ctx := context.Background()

	selected, err := PromptWaveSelection(ctx, &output, scanner, waves)
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
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps"},
	}

	scanner := bufio.NewScanner(strings.NewReader("q\n"))
	var output bytes.Buffer
	ctx := context.Background()

	_, err := PromptWaveSelection(ctx, &output, scanner, waves)
	if err != ErrQuit {
		t.Errorf("expected ErrQuit, got %v", err)
	}
}

func TestPromptWaveApproval_Approve(t *testing.T) {
	wave := Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Dependency Ordering",
		Actions: []WaveAction{
			{Type: "add_dependency", IssueID: "ENG-101", Description: "Auth before token"},
		},
		Delta: WaveDelta{Before: 0.25, After: 0.40},
	}

	scanner := bufio.NewScanner(strings.NewReader("a\n"))
	var output bytes.Buffer
	ctx := context.Background()

	choice, err := PromptWaveApproval(ctx, &output, scanner, wave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != ApprovalApprove {
		t.Errorf("expected ApprovalApprove, got %d", choice)
	}
}

func TestPromptWaveApproval_Reject(t *testing.T) {
	wave := Wave{ID: "auth-w1", Actions: []WaveAction{}}

	scanner := bufio.NewScanner(strings.NewReader("r\n"))
	var output bytes.Buffer
	ctx := context.Background()

	choice, err := PromptWaveApproval(ctx, &output, scanner, wave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != ApprovalReject {
		t.Errorf("expected ApprovalReject, got %d", choice)
	}
}

func TestPromptWaveApproval_Discuss(t *testing.T) {
	wave := Wave{
		ID:          "auth-w1",
		ClusterName: "Auth",
		Title:       "Dependency Ordering",
		Actions:     []WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
		Delta:       WaveDelta{Before: 0.25, After: 0.40},
	}

	scanner := bufio.NewScanner(strings.NewReader("d\n"))
	var output bytes.Buffer
	ctx := context.Background()

	choice, err := PromptWaveApproval(ctx, &output, scanner, wave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if choice != ApprovalDiscuss {
		t.Errorf("expected ApprovalDiscuss, got %d", choice)
	}
	if !strings.Contains(output.String(), "[d] Discuss") {
		t.Error("expected [d] Discuss in prompt output")
	}
}

func TestPromptSequence_SelectionThenApproval(t *testing.T) {
	// given: piped input with both selection and approval on one reader
	waves := []Wave{
		{ID: "auth-w1", ClusterName: "Auth", Title: "Deps",
			Actions: []WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
			Delta:   WaveDelta{Before: 0.25, After: 0.40}},
	}
	scanner := bufio.NewScanner(strings.NewReader("1\na\n"))
	var output bytes.Buffer
	ctx := context.Background()

	// when: selection then approval using same scanner
	selected, err := PromptWaveSelection(ctx, &output, scanner, waves)
	if err != nil {
		t.Fatalf("selection: unexpected error: %v", err)
	}
	if selected.ID != "auth-w1" {
		t.Errorf("expected auth-w1, got %s", selected.ID)
	}

	choice, err := PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("approval: unexpected error: %v", err)
	}

	// then: approval should read "a" from the same scanner
	if choice != ApprovalApprove {
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

	_, err := ScanLine(ctx, scanner)

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
	line, err := ScanLine(ctx, scanner)

	// then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if line != "hello" {
		t.Errorf("expected hello, got %s", line)
	}
}

func TestDisplayRippleEffects(t *testing.T) {
	ripples := []Ripple{
		{ClusterName: "API", Description: "W2 unlocked"},
		{ClusterName: "DB", Description: "New dependency added"},
	}

	var output bytes.Buffer
	DisplayRippleEffects(&output, ripples)

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
	topic, err := PromptDiscussTopic(ctx, &output, scanner)

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
	_, err := PromptDiscussTopic(ctx, &output, scanner)

	// then
	if err != ErrQuit {
		t.Errorf("expected ErrQuit, got %v", err)
	}
}

func TestPromptDiscussTopic_Empty(t *testing.T) {
	// given: empty input should error
	scanner := bufio.NewScanner(strings.NewReader("\n"))
	var output bytes.Buffer
	ctx := context.Background()

	// when
	_, err := PromptDiscussTopic(ctx, &output, scanner)

	// then
	if err == nil {
		t.Fatal("expected error for empty topic")
	}
	if err == ErrQuit {
		t.Error("expected non-quit error for empty topic")
	}
}

func TestDisplayArchitectResponse_WithModifiedWave(t *testing.T) {
	// given
	resp := &ArchitectResponse{
		Analysis: "Splitting is unnecessary for this scale.",
		ModifiedWave: &Wave{
			ID:          "auth-w1",
			ClusterName: "Auth",
			Title:       "Dependency Ordering",
			Actions: []WaveAction{
				{Type: "add_dependency", IssueID: "ENG-101", Description: "Auth before token"},
				{Type: "add_dod", IssueID: "ENG-101", Description: "Middleware interface"},
			},
			Delta: WaveDelta{Before: 0.25, After: 0.42},
		},
		Reasoning: "Project scale favors fewer issues.",
	}
	var output bytes.Buffer

	// when
	DisplayArchitectResponse(&output, resp)

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
	resp := &ArchitectResponse{
		Analysis:     "Current actions look good.",
		ModifiedWave: nil,
		Reasoning:    "No changes needed.",
	}
	var output bytes.Buffer

	// when
	DisplayArchitectResponse(&output, resp)

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
	wave := Wave{
		ID: "auth-w1", ClusterName: "Auth", Title: "Test",
		Actions: []WaveAction{{Type: "add_dependency", IssueID: "ENG-101", Description: "test"}},
		Delta:   WaveDelta{Before: 0.25, After: 0.40},
	}

	tests := []struct {
		name     string
		input    string
		expected ApprovalChoice
	}{
		{"uppercase A", "A\n", ApprovalApprove},
		{"uppercase D", "D\n", ApprovalDiscuss},
		{"uppercase R", "R\n", ApprovalReject},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := bufio.NewScanner(strings.NewReader(tt.input))
			var output bytes.Buffer
			ctx := context.Background()

			choice, err := PromptWaveApproval(ctx, &output, scanner, wave)
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
	wave := Wave{ID: "auth-w1", Actions: []WaveAction{}}
	scanner := bufio.NewScanner(strings.NewReader("Q\n"))
	var output bytes.Buffer
	ctx := context.Background()

	_, err := PromptWaveApproval(ctx, &output, scanner, wave)
	if err != ErrQuit {
		t.Errorf("expected ErrQuit for uppercase Q, got %v", err)
	}
}

func TestPromptWaveApproval_InvalidInput(t *testing.T) {
	wave := Wave{ID: "auth-w1", Actions: []WaveAction{}}

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

			_, err := PromptWaveApproval(ctx, &output, scanner, wave)
			if err == nil {
				t.Fatal("expected error for invalid input")
			}
			if err == ErrQuit {
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
	_, err := PromptDiscussTopic(ctx, &output, scanner)

	// then
	if err != ErrQuit {
		t.Errorf("expected ErrQuit for padded 'q', got %v", err)
	}
}

func TestPromptDiscussTopic_UppercaseQuit(t *testing.T) {
	// given: uppercase "Q" should also quit (consistent with PromptWaveApproval)
	scanner := bufio.NewScanner(strings.NewReader("Q\n"))
	var output bytes.Buffer
	ctx := context.Background()

	// when
	_, err := PromptDiscussTopic(ctx, &output, scanner)

	// then
	if err != ErrQuit {
		t.Errorf("expected ErrQuit for uppercase 'Q', got %v", err)
	}
}

func TestDisplayArchitectResponse_ZeroDelta(t *testing.T) {
	// given: ModifiedWave with zero-value delta (architect forgot to populate)
	resp := &ArchitectResponse{
		Analysis: "Modified wave.",
		ModifiedWave: &Wave{
			ID:      "auth-w1",
			Actions: []WaveAction{{Type: "add_dod", IssueID: "ENG-101", Description: "test"}},
			Delta:   WaveDelta{Before: 0.0, After: 0.0},
		},
	}
	var output bytes.Buffer

	// when
	DisplayArchitectResponse(&output, resp)

	// then
	out := output.String()
	if !strings.Contains(out, "0% -> 0%") {
		t.Errorf("expected '0%% -> 0%%' for zero delta, got: %s", out)
	}
}

func TestDisplayArchitectResponse_EmptyAnalysis(t *testing.T) {
	// given: empty analysis string (Claude omitted the field)
	resp := &ArchitectResponse{
		Analysis:  "",
		Reasoning: "some reasoning",
	}
	var output bytes.Buffer

	// when
	DisplayArchitectResponse(&output, resp)

	// then
	out := output.String()
	// Should still render the [Architect] prefix line
	if !strings.Contains(out, "[Architect]") {
		t.Error("expected [Architect] label even with empty analysis")
	}
}

func TestDisplayArchitectResponse_ModifiedWaveNilActions(t *testing.T) {
	// given: ModifiedWave is non-nil but Actions is nil
	resp := &ArchitectResponse{
		Analysis: "Simplified wave.",
		ModifiedWave: &Wave{
			ID:      "auth-w1",
			Actions: nil,
			Delta:   WaveDelta{Before: 0.25, After: 0.35},
		},
	}
	var output bytes.Buffer

	// when
	DisplayArchitectResponse(&output, resp)

	// then
	out := output.String()
	if !strings.Contains(out, "Modified actions (0)") {
		t.Errorf("expected 'Modified actions (0)' for nil actions, got: %s", out)
	}
}

func TestDisplayScribeResponse(t *testing.T) {
	// given
	resp := &ScribeResponse{
		ADRID:     "0003",
		Title:     "adopt-event-sourcing",
		Content:   "# 0003. Adopt Event Sourcing",
		Reasoning: "Discussion revealed need for event sourcing.",
	}
	var output bytes.Buffer

	// when
	DisplayScribeResponse(&output, resp)

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
	resp := &ScribeResponse{
		ADRID: "0001",
		Title: "",
	}
	var output bytes.Buffer

	// when
	DisplayScribeResponse(&output, resp)

	// then
	out := output.String()
	if !strings.Contains(out, "[Scribe]") {
		t.Error("expected [Scribe] label even with empty title")
	}
	if !strings.Contains(out, "0001") {
		t.Error("expected ADR ID in output")
	}
}
