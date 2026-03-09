package session

import (
	"bufio"
	"context"
	"fmt"
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

	parentSpan := trace.SpanFromContext(ctx)
	parentSpan.SetAttributes(attribute.String("sightjack.session_id", sessionID))

	// --- Interactive Loop with D-Mail Waiting Cycle ---
	shibitoShown := false
	// sessionRejected tracks user-rejected actions per wave (keyed by WaveKey).
	// Scoped per-wave intentionally: rejected actions are only fed back to the
	// nextgen call triggered by that specific wave's completion, not accumulated
	// across the entire cluster.
	sessionRejected := make(map[string][]domain.WaveAction)
	labeledReady := make(map[string]bool) // tracks issues already labeled ready

	// waitingCycle wraps the interactive loop: after all waves are processed,
	// enter a D-Mail waiting phase. When D-Mails arrive, classify and resume.
waitingCycle:
	for {
		userQuit := false
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
			selected, result, shibitoShown = selectPhase(ctx, scanner, scanResult, cfg, available, waves, adrCount, resumedAt, shibitoShown, out, trace.SpanFromContext(ctx), logger)
			switch result {
			case selectQuit:
				userQuit = true
				break outerLoop
			case selectRetry:
				continue
			}

			resolvedStrictness := string(domain.ResolveStrictness(cfg.Strictness, scanResult.StrictnessKeys(selected.ClusterName)))

			waveKey := domain.WaveKey(selected)
			waveCtx, waveSpan := platform.Tracer.Start(ctx, fmt.Sprintf("wave[%s]", waveKey),
				trace.WithAttributes(
					attribute.String("wave.id", selected.ID),
					attribute.String("wave.cluster", selected.ClusterName),
				),
			)

			selected, approvalResult := approvalPhase(waveCtx, scanner, cfg, scanDir, selected, resolvedStrictness, waves, completed, sessionRejected, adrDir, &adrCount, fbCollector.FeedbackOnly(), store, emitter, out, waveSpan, logger)
			if approvalResult != approvalApproved {
				waveSpan.End()
				continue
			}

			applyPhase(waveCtx, cfg, scanDir, scanResultPath, adrDir,
				selected, resolvedStrictness,
				&waves, completed, scanResult, sessionRejected,
				labeledReady, fbCollector, store, emitter, out, waveSpan, logger)
			waveSpan.End()
		}

		// Consistency check after each outerLoop iteration
		if domain.CheckCompletenessConsistency(scanResult.Completeness, scanResult.Clusters) {
			logger.Warn("Completeness mismatch detected. Recalculating...")
			scanResult.CalculateCompleteness()
		}

		// Save scan result cache
		if err := WriteScanResult(scanResultPath, scanResult); err != nil {
			logger.Warn("Failed to update cached scan result: %v", err)
		}

		// Exit waiting cycle if user quit via 'q'
		if userQuit {
			break waitingCycle
		}

		// Negative WaitTimeout disables waiting mode
		if cfg.Gate.WaitTimeout < 0 {
			break waitingCycle
		}

		// Snapshot before entering waiting phase
		fbCollector.Snapshot()

		// Wait for D-Mail arrival
		arrived, waitErr := waitForDMail(ctx, fbCollector, cfg.Gate.WaitTimeout, logger)
		if waitErr != nil {
			return waitErr
		}
		if !arrived {
			break waitingCycle
		}

		// Classify new D-Mails since snapshot
		newMails := fbCollector.NewSinceSnapshot()
		hasSpec := false
		for _, m := range newMails {
			if m.Kind == DMailSpecification {
				hasSpec = true
				break
			}
		}

		if hasSpec {
			logger.Info("New specification received. Rescanning not yet supported in waiting mode.")
		} else {
			logger.Info("New feedback received. Resuming interactive loop...")
		}

		// Re-evaluate waves with new feedback context before resuming
		waves = domain.EvaluateUnlocks(waves, completed)
	}

	logger.OK("Session events saved to %s", filepath.Join(baseDir, domain.StateDir, "events"))
	return nil
}
