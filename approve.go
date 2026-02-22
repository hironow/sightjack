package sightjack

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Approver requests user approval for a convergence gate.
type Approver interface {
	RequestApproval(ctx context.Context, message string) (approved bool, err error)
}

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
	if a.out != nil {
		fmt.Fprintf(a.out, "[CONVERGENCE] %s\nApprove? (y/N): ", message)
	}

	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		scanner := bufio.NewScanner(a.input)
		if scanner.Scan() {
			ch <- result{line: scanner.Text()}
		} else {
			ch <- result{err: scanner.Err()}
		}
	}()

	select {
	case <-ctx.Done():
		return false, nil
	case r := <-ch:
		if r.err != nil {
			return false, nil
		}
		answer := strings.TrimSpace(strings.ToLower(r.line))
		return answer == "y" || answer == "yes", nil
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
	expanded := strings.ReplaceAll(a.template, "{message}", shellQuote(message))
	cmd := a.factory()(ctx, "sh", "-c", expanded)
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
