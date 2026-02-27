package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// AutoApprover always approves — for CI or --auto-approve flag.
type AutoApprover struct{}

func (a *AutoApprover) RequestApproval(_ context.Context, _ string) (bool, error) {
	return true, nil
}

// StdinApprover prompts the user on a terminal and reads y/n.
// Uses goroutine + channel for context cancellation support.
// Safe default: empty or non-y input = deny.
type StdinApprover struct {
	input io.Reader
	out   io.Writer
}

// NewStdinApprover creates a StdinApprover with the given input/output.
func NewStdinApprover(input io.Reader, out io.Writer) *StdinApprover {
	return &StdinApprover{input: input, out: out}
}

func (a *StdinApprover) RequestApproval(ctx context.Context, message string) (bool, error) {
	if a.input == nil {
		return false, nil
	}

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	if a.out != nil {
		fmt.Fprintf(a.out, "[CONVERGENCE] %s\nApprove? (y/N): ", message)
	}

	// Read in a goroutine so we can select on ctx.Done().
	// We intentionally do NOT close the reader on cancel — it may be
	// os.Stdin, and closing FD 0 would break subsequent reads in the
	// same process. The goroutine may leak until the read returns (e.g.
	// on process exit), which is acceptable for a single approval prompt.
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := readLine(a.input)
		ch <- result{line: line, err: err}
	}()

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case r := <-ch:
		// Evaluate answer even on io.EOF — piped input may not end with newline.
		// Only deny on error if no content was read.
		answer := strings.TrimSpace(strings.ToLower(r.line))
		if answer == "" && r.err != nil {
			return false, nil
		}
		return answer == "y" || answer == "yes", nil
	}
}

// readLine reads one line from r without buffering ahead.
// It reads one byte at a time to avoid consuming data beyond the newline,
// which is critical when r is a shared reader (e.g. stdin).
func readLine(r io.Reader) (string, error) {
	var buf []byte
	b := make([]byte, 1)
	for {
		n, err := r.Read(b)
		if n > 0 {
			if b[0] == '\n' {
				return string(buf), nil
			}
			buf = append(buf, b[0])
		}
		if err != nil {
			return string(buf), err
		}
	}
}

// CmdApprover runs an external command for approval.
// Exit 0 = approve, non-zero ExitError = deny, other error = fail.
type CmdApprover struct {
	template   string
	cmdFactory cmdFactoryFunc
}

// NewCmdApprover creates a CmdApprover from a shell command template.
func NewCmdApprover(template string) *CmdApprover {
	return &CmdApprover{template: template}
}

func (a *CmdApprover) factory() cmdFactoryFunc {
	if a.cmdFactory != nil {
		return a.cmdFactory
	}
	return defaultCmdFactory
}

func (a *CmdApprover) RequestApproval(ctx context.Context, message string) (bool, error) {
	if a.template == "" {
		return false, fmt.Errorf("approve: empty command template")
	}
	expanded := strings.ReplaceAll(a.template, "{message}", ShellQuote(message))
	cmd := a.factory()(ctx, shellName(), shellFlag(), expanded)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}
	return false, fmt.Errorf("approve command: %w", err)
}
