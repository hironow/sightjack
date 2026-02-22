package sightjack

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestNopNotifier_NoError(t *testing.T) {
	// given
	n := &NopNotifier{}

	// when
	err := n.Notify(context.Background(), "title", "message")

	// then
	if err != nil {
		t.Errorf("NopNotifier should never error, got: %v", err)
	}
}

func TestLocalNotifier_Darwin(t *testing.T) {
	// given: LocalNotifier forced to darwin, with captured command
	var captured []string
	n := &LocalNotifier{
		forceOS: "darwin",
		cmdFactory: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			captured = append(captured, name)
			captured = append(captured, args...)
			return exec.Command("true")
		},
	}

	// when
	err := n.Notify(context.Background(), "Test Title", "Test Message")

	// then
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(captured) == 0 {
		t.Fatal("expected command to be captured")
	}
	if captured[0] != "osascript" {
		t.Errorf("expected osascript, got %s", captured[0])
	}
	joined := strings.Join(captured, " ")
	if !strings.Contains(joined, "Test Title") {
		t.Error("expected title in osascript args")
	}
}

func TestLocalNotifier_Linux(t *testing.T) {
	// given: LocalNotifier forced to linux
	var captured []string
	n := &LocalNotifier{
		forceOS: "linux",
		cmdFactory: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			captured = append(captured, name)
			captured = append(captured, args...)
			return exec.Command("true")
		},
	}

	// when
	err := n.Notify(context.Background(), "Test Title", "Test Message")

	// then
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(captured) == 0 {
		t.Fatal("expected command to be captured")
	}
	if captured[0] != "notify-send" {
		t.Errorf("expected notify-send, got %s", captured[0])
	}
}

func TestLocalNotifier_UnsupportedOS(t *testing.T) {
	// given: unsupported OS
	n := &LocalNotifier{
		forceOS: "windows",
		cmdFactory: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("true")
		},
	}

	// when
	err := n.Notify(context.Background(), "Title", "Message")

	// then: should return error for unsupported OS
	if err == nil {
		t.Error("expected error for unsupported OS")
	}
}

func TestCmdNotifier_Placeholders(t *testing.T) {
	// given: template with placeholders, using echo to verify substitution
	var captured []string
	n := &CmdNotifier{
		template: "echo {title}: {message}",
		cmdFactory: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			captured = append(captured, name)
			captured = append(captured, args...)
			return exec.Command("true")
		},
	}

	// when
	err := n.Notify(context.Background(), "Hello", "World")

	// then
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	joined := strings.Join(captured, " ")
	if !strings.Contains(joined, "'Hello'") {
		t.Errorf("expected quoted title in command, got: %s", joined)
	}
	if !strings.Contains(joined, "'World'") {
		t.Errorf("expected quoted message in command, got: %s", joined)
	}
}

func TestCmdNotifier_EmptyTemplate(t *testing.T) {
	// given: empty template
	n := &CmdNotifier{template: ""}

	// when
	err := n.Notify(context.Background(), "Title", "Message")

	// then: should error
	if err == nil {
		t.Error("expected error for empty template")
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "'hello'"},
		{"it's", "'it'\\''s'"},
		{"normal text", "'normal text'"},
		{"", "''"},
		{"$(rm -rf /)", "'$(rm -rf /)'"},
	}
	for _, tt := range tests {
		got := shellQuote(tt.input)
		if got != tt.want {
			t.Errorf("shellQuote(%q): got %q, want %q", tt.input, got, tt.want)
		}
	}
}
