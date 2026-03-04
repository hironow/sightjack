package usecase

import (
	"bufio"
	"context"
	"io"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// AvailableWaves filters waves to those available and not completed.
func AvailableWaves(waves []domain.Wave, completed map[string]bool) []domain.Wave {
	return domain.AvailableWaves(waves, completed)
}

// PromptWaveSelection displays available waves and reads the user's choice.
func PromptWaveSelection(ctx context.Context, w io.Writer, s *bufio.Scanner, waves []domain.Wave) (domain.Wave, error) {
	return session.PromptWaveSelection(ctx, w, s, waves)
}

// WaveKey returns a globally unique key for a wave: "ClusterName:ID".
func WaveKey(w domain.Wave) string {
	return domain.WaveKey(w)
}
