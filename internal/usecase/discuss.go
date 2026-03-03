package usecase

import (
	"context"
	"io"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// RunArchitectDiscussDryRun saves the discuss prompt without executing Claude.
func RunArchitectDiscussDryRun(cfg *domain.Config, scanDir string, wave domain.Wave, topic, strictness string, logger domain.Logger) error {
	return session.RunArchitectDiscussDryRun(cfg, scanDir, wave, topic, strictness, logger)
}

// RunArchitectDiscuss executes the architect discussion via Claude.
func RunArchitectDiscuss(ctx context.Context, cfg *domain.Config, scanDir string, wave domain.Wave, topic, strictness string, out io.Writer, logger domain.Logger) (*domain.ArchitectResponse, error) {
	return session.RunArchitectDiscuss(ctx, cfg, scanDir, wave, topic, strictness, out, logger)
}

// ToDiscussResult converts a wave, architect response, and topic into a DiscussResult.
func ToDiscussResult(wave domain.Wave, resp *domain.ArchitectResponse, topic string) domain.DiscussResult {
	return session.ToDiscussResult(wave, resp, topic)
}
