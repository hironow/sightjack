package usecase

import (
	"context"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// LoadLatestState loads the most recent session state from event data.
func LoadLatestState(baseDir string) (*domain.SessionState, string, error) {
	return session.LoadLatestState(baseDir)
}

// RestoreWaves converts WaveState slices back to Wave slices.
func RestoreWaves(states []domain.WaveState) []domain.Wave {
	return domain.RestoreWaves(states)
}

// NeedsMoreWaves returns true when post-completion wave generation should run.
func NeedsMoreWaves(cluster domain.ClusterScanResult, waves []domain.Wave) bool {
	return domain.NeedsMoreWaves(cluster, waves)
}

// ReadExistingADRs reads ADR files from the ADR directory.
func ReadExistingADRs(adrDir string) ([]domain.ExistingADR, error) {
	return session.ReadExistingADRs(adrDir)
}

// CompletedWavesForCluster returns completed waves for a specific cluster.
func CompletedWavesForCluster(waves []domain.Wave, clusterName string) []domain.Wave {
	return domain.CompletedWavesForCluster(waves, clusterName)
}

// GenerateNextWavesDryRun saves the nextgen prompt without executing Claude.
func GenerateNextWavesDryRun(cfg *domain.Config, scanDir string, completedWave domain.Wave, cluster domain.ClusterScanResult, completedWaves []domain.Wave, existingADRs []domain.ExistingADR, rejectedActions []domain.WaveAction, strictness string, logger domain.Logger) error {
	return session.GenerateNextWavesDryRun(cfg, scanDir, completedWave, cluster, completedWaves, existingADRs, rejectedActions, strictness, nil, nil, logger)
}

// GenerateNextWaves executes post-completion wave generation for a cluster.
func GenerateNextWaves(ctx context.Context, cfg *domain.Config, scanDir string, completedWave domain.Wave, cluster domain.ClusterScanResult, completedWaves []domain.Wave, existingADRs []domain.ExistingADR, rejectedActions []domain.WaveAction, strictness string, logger domain.Logger) ([]domain.Wave, error) {
	return session.GenerateNextWaves(ctx, cfg, scanDir, completedWave, cluster, completedWaves, existingADRs, rejectedActions, strictness, nil, nil, logger)
}
