package usecase

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

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

// RecordScanEvents caches the scan result and records session events.
// This consolidates the post-scan orchestration that belongs in the usecase layer.
func RecordScanEvents(baseDir, sessionID string, result *domain.ScanResult, cfg *domain.Config, logger *domain.Logger) {
	// Cache scan result for pipe replay
	scanResultPath := filepath.Join(domain.ScanDir(baseDir, sessionID), "scan_result.json")
	if err := session.WriteScanResult(scanResultPath, result); err != nil {
		logger.Warn("Failed to cache scan result: %v", err)
	}

	// Build cluster state for event payload
	clusters := make([]domain.ClusterState, 0, len(result.Clusters))
	for _, c := range result.Clusters {
		clusters = append(clusters, domain.ClusterState{
			Name:         c.Name,
			Completeness: c.Completeness,
			IssueCount:   len(c.Issues),
		})
	}

	// Record events for state reconstruction
	store := session.NewEventStore(baseDir, sessionID)
	recorder, recErr := session.NewSessionRecorder(store, sessionID)
	if recErr != nil {
		logger.Warn("session recorder: %v", recErr)
		return
	}
	if err := recorder.Record(domain.EventSessionStarted, domain.SessionStartedPayload{
		Project:         cfg.Linear.Project,
		StrictnessLevel: string(cfg.Strictness.Default),
	}); err != nil {
		logger.Warn("Failed to record session start: %v", err)
	}
	if err := recorder.Record(domain.EventScanCompleted, domain.ScanCompletedPayload{
		Clusters:       clusters,
		Completeness:   result.Completeness,
		ShibitoCount:   len(result.ShibitoWarnings),
		ScanResultPath: domain.RelativeScanResultPath(baseDir, scanResultPath),
		LastScanned:    time.Now(),
	}); err != nil {
		logger.Warn("Failed to record scan completed: %v", err)
	} else {
		logger.OK("Events saved to %s", session.EventStorePath(baseDir, sessionID))
	}
}
