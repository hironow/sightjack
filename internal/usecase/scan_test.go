package usecase

import (
	"context"
	"io"
	"testing"

	sightjack "github.com/hironow/sightjack"
	"github.com/hironow/sightjack/internal/domain"
)

func TestRunScan_InvalidCommand(t *testing.T) {
	// given: empty RepoPath
	cmd := domain.RunScanCommand{}

	// when
	_, err := RunScan(context.Background(), cmd, nil, "", "", false, io.Discard, sightjack.NewLogger(io.Discard, false))

	// then
	if err == nil {
		t.Fatal("expected validation error for empty RepoPath")
	}
	if got := err.Error(); got != "command validation: RepoPath is required" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestRunScan_InvalidLang(t *testing.T) {
	// given: invalid lang
	cmd := domain.RunScanCommand{
		RepoPath: "/tmp",
		Lang:     "invalid",
	}

	// when
	_, err := RunScan(context.Background(), cmd, nil, "", "", false, io.Discard, sightjack.NewLogger(io.Discard, false))

	// then
	if err == nil {
		t.Fatal("expected validation error for invalid lang")
	}
}
