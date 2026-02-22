package sightjack

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestAutoApprover_AlwaysApproves(t *testing.T) {
	// given
	a := &AutoApprover{}

	// when
	approved, err := a.RequestApproval(context.Background(), "deploy?")

	// then
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("AutoApprover should always approve")
	}
}

func TestStdinApprover_Yes(t *testing.T) {
	// given: input reader with "y\n"
	input := strings.NewReader("y\n")
	a := &StdinApprover{input: input}

	// when
	approved, err := a.RequestApproval(context.Background(), "proceed?")

	// then
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("expected approval for 'y' input")
	}
}

func TestStdinApprover_YesUppercase(t *testing.T) {
	// given: input reader with "Y\n"
	input := strings.NewReader("Y\n")
	a := &StdinApprover{input: input}

	// when
	approved, err := a.RequestApproval(context.Background(), "proceed?")

	// then
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("expected approval for 'Y' input")
	}
}

func TestStdinApprover_No(t *testing.T) {
	// given: input reader with "n\n"
	input := strings.NewReader("n\n")
	a := &StdinApprover{input: input}

	// when
	approved, err := a.RequestApproval(context.Background(), "proceed?")

	// then
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if approved {
		t.Error("expected denial for 'n' input")
	}
}

func TestStdinApprover_EmptyInput(t *testing.T) {
	// given: empty input (safe default = deny)
	input := strings.NewReader("\n")
	a := &StdinApprover{input: input}

	// when
	approved, err := a.RequestApproval(context.Background(), "proceed?")

	// then
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if approved {
		t.Error("expected denial for empty input (safe default)")
	}
}

func TestStdinApprover_ContextCancel(t *testing.T) {
	// given: context that is already cancelled + a reader that blocks
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Use a reader that never returns data (pipe, but we close it)
	input := strings.NewReader("")
	a := &StdinApprover{input: input}

	// when
	approved, err := a.RequestApproval(ctx, "proceed?")

	// then: should deny (context cancelled)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if approved {
		t.Error("expected denial when context is cancelled")
	}
}

func TestCmdApprover_Approve(t *testing.T) {
	// given: command that exits 0
	a := &CmdApprover{
		template: "true",
		cmdFactory: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("true")
		},
	}

	// when
	approved, err := a.RequestApproval(context.Background(), "deploy?")

	// then
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("expected approval for exit 0")
	}
}

func TestCmdApprover_Deny(t *testing.T) {
	// given: command that exits 1
	a := &CmdApprover{
		template: "false",
		cmdFactory: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("false")
		},
	}

	// when
	approved, err := a.RequestApproval(context.Background(), "deploy?")

	// then: denied, no error (ExitError is not a failure)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if approved {
		t.Error("expected denial for exit 1")
	}
}

func TestCmdApprover_EmptyTemplate(t *testing.T) {
	// given: empty template
	a := &CmdApprover{template: ""}

	// when
	_, err := a.RequestApproval(context.Background(), "msg")

	// then
	if err == nil {
		t.Error("expected error for empty template")
	}
}
