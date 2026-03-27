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
	parentSpan.SetAttributes(attribute.String("sightjack.session_id", platform.SanitizeUTF8(sessionID)))

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

			resolvedStrictness := string(domain.ResolveStrictness(cfg.Strictness, cfg.Computed.EstimatedStrictness, scanResult.StrictnessKeys(selected.ClusterName)))

			waveKey := domain.WaveKey(selected)
			waveCtx, waveSpan := platform.Tracer.Start(ctx, fmt.Sprintf("wave[%s]", waveKey), // nosemgrep: adr0003-otel-span-without-defer-end -- End() called per branch in loop [permanent]
				trace.WithAttributes(
					attribute.String("wave.id", platform.SanitizeUTF8(selected.ID)),
					attribute.String("wave.cluster", platform.SanitizeUTF8(selected.ClusterName)),
				),
			)

			selected, approvalResult := approvalPhase(waveCtx, scanner, cfg, scanDir, selected, resolvedStrictness, waves, completed, sessionRejected, adrDir, &adrCount, fbCollector.FeedbackOnly(), store, emitter, RunArchitectDiscuss, out, waveSpan, logger)
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

		// Process any design-feedback D-Mails that arrived during scanning
		// (before entering waiting phase). Without this, feedback collected
		// during scan/apply/review would be missed by NewSinceSnapshot.
		preFeedback := fbCollector.NewSinceSnapshot()
		emitDesignFeedback(preFeedback, emitter, logger)

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
		hasSpec, hasReport, hasDesignFeedback, reportIssueIDs := classifyNewMails(newMails)

		if hasSpec {
			logger.Info("New specification received. Rescanning not yet supported in waiting mode.")
		}

		// Emit event for each design-feedback D-Mail (preserves individual names for traceability)
		if hasDesignFeedback {
			emitDesignFeedback(newMails, emitter, logger)
		}

		// Generate next waves for clusters affected by report D-Mails
		if hasReport && len(reportIssueIDs) > 0 {
			affectedClusters := domain.ClustersForIssueIDs(scanResult.Clusters, reportIssueIDs)
			for _, cluster := range affectedClusters {
				lastWave, ok := domain.LastCompletedWaveForCluster(waves, cluster.Name)
				if !ok {
					logger.Debug("No completed wave for cluster %s; skipping nextgen from report", cluster.Name)
					continue
				}
				logger.Info("Report D-Mail triggered nextgen for cluster %s", cluster.Name)
				resolvedStrictness := string(domain.ResolveStrictness(cfg.Strictness, cfg.Computed.EstimatedStrictness, scanResult.StrictnessKeys(cluster.Name)))
				_, waveSpan := platform.Tracer.Start(ctx, fmt.Sprintf("report-nextgen[%s]", cluster.Name), // nosemgrep: adr0003-otel-span-without-defer-end -- End() called after generateNextWavesIfNeeded below [permanent]
					trace.WithAttributes(
						attribute.String("wave.cluster", platform.SanitizeUTF8(cluster.Name)),
						attribute.String("trigger", "report-dmail"),
					),
				)
				generateNextWavesIfNeeded(ctx, cfg, scanDir, adrDir, lastWave, resolvedStrictness,
					&waves, completed, scanResult, sessionRejected, fbCollector, emitter, waveSpan, logger)
				waveSpan.End()
			}
		}

		if !hasSpec && !hasReport && !hasDesignFeedback {
			logger.Info("New feedback received. Resuming interactive loop...")
		}

		// Re-evaluate waves with new feedback context before resuming
		waves = domain.EvaluateUnlocks(waves, completed)
	}

	logger.OK("Session events saved to %s", filepath.Join(baseDir, domain.StateDir, "events"))
	return nil
}

// classifyNewMails categorizes newly arrived D-Mails into specification,
// report (with issue IDs), design-feedback, or other kinds.
func classifyNewMails(mails []*DMail) (hasSpec, hasReport, hasDesignFeedback bool, reportIssueIDs []string) {
	for _, m := range mails {
		switch m.Kind {
		case DMailSpecification:
			hasSpec = true
		case DMailReport:
			hasReport = true
			reportIssueIDs = append(reportIssueIDs, m.Issues...)
		case DMailDesignFeedback:
			hasDesignFeedback = true
		}
	}
	return
}

// emitDesignFeedback emits feedback_received events for any design-feedback D-Mails
// in the batch. Each D-Mail's actual name is preserved for traceability (P3 fix).
func emitDesignFeedback(mails []*DMail, emitter port.SessionEventEmitter, logger domain.Logger) {
	var names []string
	for _, m := range mails {
		if m.Kind == DMailDesignFeedback {
			names = append(names, m.Name)
		}
	}
	if len(names) == 0 {
		return
	}
	logger.Info("Design-feedback received (%d D-Mail(s)); re-scan will incorporate feedback", len(names))
	for _, name := range names {
		if err := emitter.EmitReceiveFeedback(domain.FeedbackReceivedPayload{
			Kind:  string(DMailDesignFeedback),
			Name:  name,
			Count: 1,
		}, time.Now().UTC()); err != nil {
			logger.Warn("Failed to emit feedback_received event for %s: %v", name, err)
		}
	}
}
