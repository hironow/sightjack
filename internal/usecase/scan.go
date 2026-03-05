package usecase

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// RunScan validates the RunScanCommand, executes the scan, caches the result,
// and records session events — all in a single atomic usecase call.
// The sessionID is generated internally and returned for downstream use.
func RunScan(ctx context.Context, cmd domain.RunScanCommand, cfg *domain.Config, baseDir string, dryRun bool, streamOut io.Writer, logger domain.Logger) (*domain.ScanResult, string, error) {
	if errs := cmd.Validate(); len(errs) > 0 {
		return nil, "", fmt.Errorf("command validation: %w", errs[0])
	}
	sessionID := fmt.Sprintf("scan-%d-%d", time.Now().UnixMilli(), os.Getpid())
	result, err := session.RunScan(ctx, cfg, baseDir, sessionID, dryRun, streamOut, logger)
	if err != nil {
		return nil, sessionID, err
	}

	// In dry-run mode, no events to record
	if dryRun || result == nil {
		return result, sessionID, nil
	}

	// Record scan state: cache result + session start/scan completed events
	recorder, recErr := session.NewSessionRecorder(session.SessionEventsDir(baseDir, sessionID), sessionID)
	if recErr != nil {
		logger.Warn("session recorder: %v", recErr)
		return result, sessionID, nil
	}

	session.RecordScanState(baseDir, sessionID, result, cfg, recorder, time.Now(), logger)

	return result, sessionID, nil
}
