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

// ErrGoBack signals the user chose to go back to the previous menu.
var ErrGoBack = errors.New("go back")

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
	fmt.Fprintf(w, "\nSelect wave [1-%d, b=back, q=quit]: ", len(waves))

	line, err := ScanLine(ctx, s)
	if err != nil {
		return Wave{}, ErrQuit
	}
	input := strings.TrimSpace(line)
	if input == "q" {
		return Wave{}, ErrQuit
	}
	if input == "b" {
		return Wave{}, ErrGoBack
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
	fmt.Fprint(w, "\n  [a] Approve all  [s] Selective  [r] Reject  [d] Discuss  [q] Back to navigator: ")

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
	case "s":
		return ApprovalSelective, nil
	case "q":
		return ApprovalQuit, ErrQuit
	default:
		return ApprovalQuit, fmt.Errorf("invalid input: %s", input)
	}
}

// PromptSelectiveApproval displays wave actions with toggle checkboxes.
// Returns approved and rejected action lists.
func PromptSelectiveApproval(ctx context.Context, w io.Writer, s *bufio.Scanner, wave Wave) ([]WaveAction, []WaveAction, error) {
	if len(wave.Actions) == 0 {
		return nil, nil, nil
	}
	selected := make([]bool, len(wave.Actions))
	for i := range selected {
		selected[i] = true // default: all selected
	}

	for {
		// Display current state
		fmt.Fprintf(w, "\n  --- %s - %s ---\n", wave.ClusterName, wave.Title)
		for i, a := range wave.Actions {
			mark := "x"
			if !selected[i] {
				mark = " "
			}
			fmt.Fprintf(w, "    %d. [%s] [%s] %s: %s\n", i+1, mark, a.Type, a.IssueID, a.Description)
		}
		fmt.Fprintf(w, "\n  Toggle [1-%d, a=all, n=none, done=confirm, q=quit]: ", len(wave.Actions))

		line, err := ScanLine(ctx, s)
		if err != nil {
			return nil, nil, ErrQuit
		}
		input := strings.TrimSpace(strings.ToLower(line))

		switch input {
		case "q":
			return nil, nil, ErrQuit
		case "done":
			var approved, rejected []WaveAction
			for i, a := range wave.Actions {
				if selected[i] {
					approved = append(approved, a)
				} else {
					rejected = append(rejected, a)
				}
			}
			return approved, rejected, nil
		case "a":
			for i := range selected {
				selected[i] = true
			}
			continue
		case "n":
			for i := range selected {
				selected[i] = false
			}
			continue
		default:
			num, parseErr := strconv.Atoi(input)
			if parseErr != nil || num < 1 || num > len(wave.Actions) {
				fmt.Fprintf(w, "  Invalid input: %s\n", input)
				continue
			}
			selected[num-1] = !selected[num-1]
		}
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

// PromptResume displays previous session info and asks the user to resume, start new, or re-scan.
func PromptResume(ctx context.Context, w io.Writer, s *bufio.Scanner, state *SessionState) (ResumeChoice, error) {
	completePct := int(state.Completeness * 100)
	fmt.Fprintf(w, "\n  Previous session found (%d%% complete, %d ADRs)\n", completePct, state.ADRCount)
	fmt.Fprintf(w, "  Last scan: %s\n\n", state.LastScanned.Format("2006-01-02 15:04"))
	if CanResume(state) {
		fmt.Fprintln(w, "  [r] Resume session")
	} else {
		fmt.Fprintln(w, "  [r] Resume session (unavailable — rescan required)")
	}
	fmt.Fprintln(w, "  [n] Start new session")
	fmt.Fprintln(w, "  [s] Re-scan Linear and resume")
	fmt.Fprint(w, "\n  Choice: ")

	line, err := ScanLine(ctx, s)
	if err != nil {
		return ResumeChoiceResume, ErrQuit
	}
	input := strings.TrimSpace(strings.ToLower(line))
	switch input {
	case "r":
		return ResumeChoiceResume, nil
	case "n":
		return ResumeChoiceNew, nil
	case "s":
		return ResumeChoiceRescan, nil
	case "q":
		return ResumeChoiceResume, ErrQuit
	default:
		return ResumeChoiceResume, fmt.Errorf("invalid input: %s", input)
	}
}

// CompletedWaves filters waves to only those with "completed" status.
func CompletedWaves(waves []Wave) []Wave {
	var result []Wave
	for _, w := range waves {
		if w.Status == "completed" {
			result = append(result, w)
		}
	}
	return result
}

// PromptCompletedWaveSelection displays completed waves and reads the user's choice.
func PromptCompletedWaveSelection(ctx context.Context, w io.Writer, s *bufio.Scanner, completed []Wave) (Wave, error) {
	fmt.Fprintln(w, "\n  Completed waves:")
	for i, wave := range completed {
		fmt.Fprintf(w, "    %d. %-6s W: %-20s (%2.0f%% -> %2.0f%%)\n",
			i+1, wave.ClusterName, wave.Title,
			wave.Delta.Before*100, wave.Delta.After*100)
	}
	fmt.Fprintf(w, "\n  Select [1-%d, b=back]: ", len(completed))

	line, err := ScanLine(ctx, s)
	if err != nil {
		return Wave{}, ErrQuit
	}
	input := strings.TrimSpace(line)
	if input == "b" {
		return Wave{}, ErrGoBack
	}
	num, parseErr := strconv.Atoi(input)
	if parseErr != nil || num < 1 || num > len(completed) {
		return Wave{}, fmt.Errorf("invalid selection: %s", input)
	}
	return completed[num-1], nil
}
