package usecase

import (
	"context"
	"fmt"
	"io"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// RunScan validates the RunScanCommand, then delegates to session.RunScan.
func RunScan(ctx context.Context, cmd domain.RunScanCommand, cfg *domain.Config, baseDir, sessionID string, dryRun bool, streamOut io.Writer, logger *domain.Logger) (*domain.ScanResult, error) {
	if errs := cmd.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("command validation: %w", errs[0])
	}
	return session.RunScan(ctx, cfg, baseDir, sessionID, dryRun, streamOut, logger)
}
