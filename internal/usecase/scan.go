package usecase

import (
	"context"
	"io"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// RunScan executes the scan, caches the result, and records session events.
// The command is always-valid by construction — no validation needed.
func RunScan(ctx context.Context, cmd domain.RunScanCommand, cfg *domain.Config, baseDir, sessionID string, dryRun bool, streamOut io.Writer, logger domain.Logger, scanner port.ScanRunner, factory port.RecorderFactory) (*domain.ScanResult, error) {
	result, err := scanner.RunScan(ctx, cfg, baseDir, sessionID, dryRun, streamOut, logger)
	if err != nil {
		return nil, err
	}

	// In dry-run mode, no events to record
	if dryRun || result == nil {
		return result, nil
	}

	// Record scan state: cache result + session start/scan completed events
	stateDir := factory.SessionEventsDir(baseDir, sessionID)
	recorder, recErr := factory.NewSessionRecorder(ctx, stateDir, sessionID, logger)
	if recErr != nil {
		logger.Warn("session recorder: %v", recErr)
		return result, nil
	}

	agg := domain.NewSessionAggregate()
	emitter := NewSessionEventEmitter(ctx, agg, recorder, logger)
	scanner.RecordScanState(baseDir, sessionID, result, cfg, emitter, time.Now(), logger)

	return result, nil
}
