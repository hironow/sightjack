package usecase

import (
	"context"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// RunWaveGenerate generates waves from scan result clusters.
func RunWaveGenerate(ctx context.Context, cfg *domain.Config, scanDir string, clusters []domain.ClusterScanResult, dryRun bool, logger domain.Logger) ([]domain.Wave, []string, map[string]bool, error) {
	return session.RunWaveGenerate(ctx, cfg, scanDir, clusters, dryRun, logger)
}
