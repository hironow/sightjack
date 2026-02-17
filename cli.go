package sightjack

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ErrQuit signals the user chose to quit.
var ErrQuit = errors.New("user quit")

// ScanLine reads one line from s, returning early if ctx is cancelled.
// The goroutine blocked on s.Scan() may outlive the call when the context
// fires first; this is acceptable for a CLI tool that exits shortly after.
func ScanLine(ctx context.Context, s *bufio.Scanner) (string, error) {
	type result struct{ ok bool }
	ch := make(chan result, 1)
	go func() {
		ch <- result{s.Scan()}
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-ch:
		if !r.ok {
			if err := s.Err(); err != nil {
				return "", err
			}
			return "", io.EOF
		}
		return s.Text(), nil
	}
}

// PromptWaveSelection displays available waves and reads the user's choice.
func PromptWaveSelection(ctx context.Context, w io.Writer, s *bufio.Scanner, waves []Wave) (Wave, error) {
	fmt.Fprintln(w, "\nAvailable waves:")
	for i, wave := range waves {
		fmt.Fprintf(w, "  %d. %-6s W: %-20s (%2.0f%% -> %2.0f%%)\n",
			i+1, wave.ClusterName, wave.Title,
			wave.Delta.Before*100, wave.Delta.After*100)
	}
	fmt.Fprintf(w, "\nSelect wave [1-%d, q=quit]: ", len(waves))

	line, err := ScanLine(ctx, s)
	if err != nil {
		return Wave{}, ErrQuit
	}
	input := strings.TrimSpace(line)
	if input == "q" {
		return Wave{}, ErrQuit
	}
	num, parseErr := strconv.Atoi(input)
	if parseErr != nil || num < 1 || num > len(waves) {
		return Wave{}, fmt.Errorf("invalid selection: %s", input)
	}
	return waves[num-1], nil
}

// PromptWaveApproval displays a wave proposal and reads approve/reject/discuss.
func PromptWaveApproval(ctx context.Context, w io.Writer, s *bufio.Scanner, wave Wave) (ApprovalChoice, error) {
	fmt.Fprintf(w, "\n--- %s - %s ---\n", wave.ClusterName, wave.Title)
	fmt.Fprintf(w, "  Proposed actions (%d):\n", len(wave.Actions))
	for i, a := range wave.Actions {
		fmt.Fprintf(w, "    %d. [%s] %s: %s\n", i+1, a.Type, a.IssueID, a.Description)
	}
	fmt.Fprintf(w, "\n  Expected: %.0f%% -> %.0f%%\n", wave.Delta.Before*100, wave.Delta.After*100)
	fmt.Fprint(w, "\n  [a] Approve all  [r] Reject  [d] Discuss  [q] Back to navigator: ")

	line, err := ScanLine(ctx, s)
	if err != nil {
		return ApprovalQuit, ErrQuit
	}
	input := strings.TrimSpace(strings.ToLower(line))
	switch input {
	case "a":
		return ApprovalApprove, nil
	case "r":
		return ApprovalReject, nil
	case "d":
		return ApprovalDiscuss, nil
	case "q":
		return ApprovalQuit, ErrQuit
	default:
		return ApprovalQuit, fmt.Errorf("invalid input: %s", input)
	}
}

// DisplayRippleEffects shows cross-cluster effects after a wave is applied.
func DisplayRippleEffects(w io.Writer, ripples []Ripple) {
	if len(ripples) == 0 {
		return
	}
	fmt.Fprintln(w, "\n  Ripple effects:")
	for _, r := range ripples {
		fmt.Fprintf(w, "    -> %s: %s\n", r.ClusterName, r.Description)
	}
}

// PromptDiscussTopic reads a free-text discussion topic from the user.
func PromptDiscussTopic(ctx context.Context, w io.Writer, s *bufio.Scanner) (string, error) {
	fmt.Fprint(w, "\n  Topic: ")

	line, err := ScanLine(ctx, s)
	if err != nil {
		return "", ErrQuit
	}
	input := strings.TrimSpace(line)
	if strings.EqualFold(input, "q") {
		return "", ErrQuit
	}
	if input == "" {
		return "", fmt.Errorf("empty topic")
	}
	return input, nil
}

// DisplayArchitectResponse shows the architect's analysis and any wave modifications.
func DisplayArchitectResponse(w io.Writer, resp *ArchitectResponse) {
	fmt.Fprintf(w, "\n  [Architect] %s\n", resp.Analysis)
	if resp.Reasoning != "" {
		fmt.Fprintf(w, "\n  Reasoning: %s\n", resp.Reasoning)
	}
	if resp.ModifiedWave != nil {
		fmt.Fprintf(w, "\n  Modified actions (%d):\n", len(resp.ModifiedWave.Actions))
		for i, a := range resp.ModifiedWave.Actions {
			fmt.Fprintf(w, "    %d. [%s] %s: %s\n", i+1, a.Type, a.IssueID, a.Description)
		}
		fmt.Fprintf(w, "\n  Expected: %.0f%% -> %.0f%%\n",
			resp.ModifiedWave.Delta.Before*100, resp.ModifiedWave.Delta.After*100)
	}
}

// DisplayScribeResponse shows the scribe's ADR generation result.
func DisplayScribeResponse(w io.Writer, resp *ScribeResponse) {
	fmt.Fprintf(w, "\n  [Scribe] ADR %s: %s\n", resp.ADRID, resp.Title)
	fmt.Fprintf(w, "  Saved to docs/adr/%s-%s.md\n", resp.ADRID, resp.Title)
}
