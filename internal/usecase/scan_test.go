package usecase

import (
	"context"
	"io"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
)

func TestRunScan_InvalidCommand(t *testing.T) {
	// given: empty RepoPath
	cmd := domain.RunScanCommand{}

	// when: scanner and factory are nil because validation should fail before they are used
	_, _, err := RunScan(context.Background(), cmd, nil, "", false, io.Discard, platform.NewLogger(io.Discard, false), nil, nil)

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

	// when: scanner and factory are nil because validation should fail before they are used
	_, _, err := RunScan(context.Background(), cmd, nil, "", false, io.Discard, platform.NewLogger(io.Discard, false), nil, nil)

	// then
	if err == nil {
		t.Fatal("expected validation error for invalid lang")
	}
}
