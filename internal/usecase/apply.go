package usecase

import (
	"context"
	"io"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// RunWaveApply executes a wave apply via Claude Code.
func RunWaveApply(ctx context.Context, cfg *domain.Config, scanDir string, wave domain.Wave, strictness string, out io.Writer, logger domain.Logger) (*domain.WaveApplyResult, error) {
	return session.RunWaveApply(ctx, cfg, scanDir, wave, strictness, out, logger)
}

// ToApplyResult converts the internal WaveApplyResult to the pipe wire format ApplyResult.
func ToApplyResult(wave domain.Wave, internal *domain.WaveApplyResult) domain.ApplyResult {
	return session.ToApplyResult(wave, internal)
}
