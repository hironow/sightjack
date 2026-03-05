package usecase

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// RunScan validates the RunScanCommand, executes the scan, caches the result,
// and records session events — all in a single atomic usecase call.
// The sessionID is provided by the caller (typically generated in cmd layer).
func RunScan(ctx context.Context, cmd domain.RunScanCommand, cfg *domain.Config, baseDir, sessionID string, dryRun bool, streamOut io.Writer, logger domain.Logger, scanner port.ScanRunner, factory port.RecorderFactory) (*domain.ScanResult, error) {
	if errs := cmd.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("command validation: %w", errs[0])
	}
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
	recorder, recErr := factory.NewSessionRecorder(stateDir, sessionID, logger)
	if recErr != nil {
		logger.Warn("session recorder: %v", recErr)
		return result, nil
	}

	agg := domain.NewSessionAggregate()
	emitter := NewSessionEventEmitter(agg, recorder, logger)
	scanner.RecordScanState(baseDir, sessionID, result, cfg, emitter, time.Now(), logger)

	return result, nil
}
