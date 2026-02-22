package sightjack

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

func TestStdinApprover_ContextCancelUnblocksRead(t *testing.T) {
	// given: a closable reader that tracks Close calls.
	// When context cancels, the reader should be closed to unblock Read.
	cr := &trackingReadCloser{blocking: true}
	ctx, cancel := context.WithCancel(context.Background())
	a := &StdinApprover{input: cr}

	// when: cancel context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	done := make(chan struct{})
	go func() {
		a.RequestApproval(ctx, "proceed?")
		close(done)
	}()

	// then: RequestApproval returns within a reasonable time
	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("RequestApproval did not return after context cancel")
	}

	// then: the reader was closed to unblock the read goroutine.
	// context.AfterFunc fires in a separate goroutine, so allow a brief
	// moment for it to execute after ctx.Done() resolves.
	deadline := time.After(1 * time.Second)
	for !cr.closed.Load() {
		select {
		case <-deadline:
			t.Fatal("expected reader to be closed on context cancel to prevent goroutine leak")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// trackingReadCloser is a test helper that blocks on Read and tracks Close calls.
type trackingReadCloser struct {
	blocking bool
	closed   atomic.Bool
	ch       chan struct{}
}

func (r *trackingReadCloser) Read(p []byte) (int, error) {
	if r.blocking && !r.closed.Load() {
		if r.ch == nil {
			r.ch = make(chan struct{})
		}
		<-r.ch // block until closed
	}
	return 0, io.EOF
}

func (r *trackingReadCloser) Close() error {
	r.closed.Store(true)
	if r.ch != nil {
		select {
		case <-r.ch:
		default:
			close(r.ch)
		}
	}
	return nil
}

func TestStdinApprover_NilInput(t *testing.T) {
	// given: StdinApprover with nil input (library/non-interactive usage)
	a := &StdinApprover{input: nil}

	// when: should not panic
	approved, err := a.RequestApproval(context.Background(), "proceed?")

	// then: safe default = deny, no error
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if approved {
		t.Error("expected denial for nil input")
	}
}

func TestStdinApprover_SharedReader(t *testing.T) {
	// given: a shared reader with approval line + subsequent data.
	// After RequestApproval consumes "y\n", the remaining "next-line\n"
	// must still be readable from the same reader.
	input := strings.NewReader("y\nnext-line\n")
	a := &StdinApprover{input: input}

	// when
	approved, err := a.RequestApproval(context.Background(), "proceed?")

	// then: approved
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Fatal("expected approval")
	}

	// then: remaining data is still available from the shared reader
	remaining := make([]byte, 64)
	n, _ := input.Read(remaining)
	got := string(remaining[:n])
	if got != "next-line\n" {
		t.Errorf("shared reader lost data: got %q, want %q", got, "next-line\n")
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
