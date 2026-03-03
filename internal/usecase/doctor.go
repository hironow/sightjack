package usecase

import (
	"context"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// RunDoctor checks environment health and tool availability.
func RunDoctor(ctx context.Context, configPath, baseDir string, logger domain.Logger) []domain.CheckResult {
	return session.RunDoctor(ctx, configPath, baseDir, logger)
}
