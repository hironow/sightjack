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

	approved, err := PromptWaveApproval(ctx, &output, scanner, wave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("expected approval")
	}
}

func TestPromptWaveApproval_Reject(t *testing.T) {
	wave := Wave{ID: "auth-w1", Actions: []WaveAction{}}

	scanner := bufio.NewScanner(strings.NewReader("r\n"))
	var output bytes.Buffer
	ctx := context.Background()

	approved, err := PromptWaveApproval(ctx, &output, scanner, wave)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if approved {
		t.Error("expected rejection")
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

	approved, err := PromptWaveApproval(ctx, &output, scanner, selected)
	if err != nil {
		t.Fatalf("approval: unexpected error: %v", err)
	}

	// then: approval should read "a" from the same scanner
	if !approved {
		t.Error("expected approval, but got rejection (scanner likely lost buffered input)")
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
