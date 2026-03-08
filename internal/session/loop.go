package session

import (
	"bufio"
	"context"
	"io"
	"path/filepath"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/usecase/port"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// runInteractiveLoop runs the wave selection/approval/apply loop shared by
// RunSession, RunResumeSession, and RunRescanSession.
// resumedAt controls the Navigator "Session: resumed" banner (nil hides it).
// scanTimestamp is persisted in state as LastScanned and stays stable across saves.
func runInteractiveLoop(ctx context.Context, cfg *domain.Config, baseDir, sessionID, scanDir, scanResultPath string,
	scanResult *domain.ScanResult, waves []domain.Wave, completed map[string]bool, adrCount int,
	scanner *bufio.Scanner, adrDir string, resumedAt *time.Time, scanTimestamp time.Time, fbCollector *FeedbackCollector,
	store port.OutboxStore, emitter port.SessionEventEmitter, out io.Writer, logger domain.Logger) error {

	ctx, loopSpan := platform.Tracer.Start(ctx, "interactive.loop",
		trace.WithAttributes(
			attribute.String("sightjack.session_id", sessionID),
		),
	)
	defer loopSpan.End()

	// --- Interactive Loop ---
	shibitoShown := false
	// sessionRejected tracks user-rejected actions per wave (keyed by WaveKey).
	// Scoped per-wave intentionally: rejected actions are only fed back to the
	// nextgen call triggered by that specific wave's completion, not accumulated
	// across the entire cluster.
	sessionRejected := make(map[string][]domain.WaveAction)
	labeledReady := make(map[string]bool) // tracks issues already labeled ready
outerLoop:
	for {
		waves = domain.EvaluateUnlocks(waves, completed)
		available := domain.AvailableWaves(waves, completed)
		if len(available) == 0 {
			logger.OK("All waves completed or no available waves.")
			break
		}

		var selected domain.Wave
		var result selectPhaseResult
		selected, result, shibitoShown = selectPhase(ctx, scanner, scanResult, cfg, available, waves, adrCount, resumedAt, shibitoShown, out, loopSpan, logger)
		switch result {
		case selectQuit:
			break outerLoop
		case selectRetry:
			continue
		}

		resolvedStrictness := string(domain.ResolveStrictness(cfg.Strictness, scanResult.StrictnessKeys(selected.ClusterName)))

		selected, approvalResult := approvalPhase(ctx, scanner, cfg, scanDir, selected, resolvedStrictness, waves, completed, sessionRejected, adrDir, &adrCount, fbCollector.FeedbackOnly(), store, emitter, out, loopSpan, logger)
		if approvalResult != approvalApproved {
			continue
		}

		applyPhase(ctx, cfg, scanDir, scanResultPath, adrDir,
			selected, resolvedStrictness,
			&waves, completed, scanResult, sessionRejected,
			labeledReady, fbCollector, store, emitter, out, loopSpan, logger)
	}

	// Final consistency check
	if domain.CheckCompletenessConsistency(scanResult.Completeness, scanResult.Clusters) {
		logger.Warn("Completeness mismatch detected. Recalculating...")
		scanResult.CalculateCompleteness()
	}

	// Save scan result cache
	if err := WriteScanResult(scanResultPath, scanResult); err != nil {
		logger.Warn("Failed to update cached scan result: %v", err)
	}

	logger.OK("Session events saved to %s", filepath.Join(baseDir, domain.StateDir, "events"))
	return nil
}
