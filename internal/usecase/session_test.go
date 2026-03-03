package usecase

import (
	"context"
	"io"
	"testing"

	sightjack "github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func TestRunSession_InvalidCommand(t *testing.T) {
	// given: empty RepoPath
	cmd := domain.RunSessionCommand{}

	// when
	err := RunSession(context.Background(), cmd, nil, "", "", false, nil, io.Discard, session.NopRecorder{}, sightjack.NewLogger(io.Discard, false))

	// then
	if err == nil {
		t.Fatal("expected validation error for empty RepoPath")
	}
	if got := err.Error(); got != "command validation: RepoPath is required" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestResumeSession_InvalidCommand(t *testing.T) {
	// given: empty SessionID
	cmd := domain.ResumeSessionCommand{RepoPath: "/tmp"}

	// when
	err := ResumeSession(context.Background(), cmd, nil, "", nil, nil, io.Discard, session.NopRecorder{}, sightjack.NewLogger(io.Discard, false))

	// then
	if err == nil {
		t.Fatal("expected validation error for empty SessionID")
	}
	if got := err.Error(); got != "command validation: SessionID is required" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestRescanSession_InvalidCommand(t *testing.T) {
	// given: empty RepoPath
	cmd := domain.RunSessionCommand{}

	// when
	err := RescanSession(context.Background(), cmd, nil, "", nil, "", nil, io.Discard, session.NopRecorder{}, sightjack.NewLogger(io.Discard, false))

	// then
	if err == nil {
		t.Fatal("expected validation error for empty RepoPath")
	}
}
