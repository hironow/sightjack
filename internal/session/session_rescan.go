package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// RunRescanSession performs a fresh scan then merges completed status from old state.
func RunRescanSession(ctx context.Context, cfg *domain.Config, baseDir string, oldState *domain.SessionState, sessionID string, input io.Reader, out io.Writer, emitter port.SessionEventEmitter, logger domain.Logger) error {
	ctx, span := platform.Tracer.Start(ctx, "sightjack.rescan")
	defer span.End()

	if logger == nil {
		logger = &domain.NopLogger{}
	}
	if input == nil {
		return fmt.Errorf("input reader is required for interactive session")
	}

	// Ensure D-Mail directories exist before any mail operations
	if err := EnsureMailDirs(baseDir); err != nil {
		return fmt.Errorf("ensure mail dirs: %w", err)
	}

	// Transactional outbox: SQLite-backed stage → atomic flush to archive/ + outbox/
	outboxStore, err := NewOutboxStoreForDir(baseDir)
	if err != nil {
		return fmt.Errorf("outbox store: %w", err)
	}
	defer outboxStore.Close()

	// Start inbox monitor (fsnotify-based) for feedback d-mails.
	// CollectFeedback accumulates initial + late-arriving feedback.
	monitorCtx, monitorCancel := context.WithCancel(ctx)
	defer monitorCancel()
	inboxCh, monitorErr := MonitorInbox(monitorCtx, baseDir, logger)
	if monitorErr != nil {
		logger.Warn("D-Mail monitor failed: %v", monitorErr)
	}
	initial := DrainInboxFeedback(inboxCh, logger)

	// Convergence gate with re-drain: catches late-arriving convergence
	notifier := BuildNotifier(cfg)
	approver := BuildApprover(cfg, input, out)
	allDmails, approved, gateErr := RunConvergenceGateWithRedrain(ctx, initial, inboxCh, notifier, approver, logger)
	if gateErr != nil {
		return fmt.Errorf("convergence gate: %w", gateErr)
	}
	if !approved {
		logger.Warn("Session aborted: convergence gate denied")
		return nil
	}

	fbCollector := CollectFeedback(allDmails, inboxCh, notifier, logger)

	scanDir, err := EnsureScanDir(baseDir, sessionID)
	if err != nil {
		return err
	}
	scanResult, err := RunScan(ctx, cfg, baseDir, sessionID, false, out, logger)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "sightjack.rescan.scan"))
		return fmt.Errorf("re-scan: %w", err)
	}
	for _, w := range scanResult.ScanWarnings {
		logger.Warn("Partial scan: %s", w)
	}
	scanTime := time.Now()

	// Cache ScanResult + record session start / scan completed events
	scanResultPath := RecordScanState(baseDir, sessionID, scanResult, cfg, emitter, scanTime, logger)

	waves, rescanWarnings, failedNames, err := RunWaveGenerate(ctx, cfg, scanDir, scanResult.Clusters, false, logger)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "sightjack.rescan.wave_generate"))
		return fmt.Errorf("wave generate: %w", err)
	}
	// rescanWarnings are already logged by RunParallel; just propagate.
	_ = rescanWarnings

	// Carry forward old waves whose clusters failed regeneration so that
	// completed progress is never lost on transient partial failures.
	// Only carry forward clusters still present in the current scan;
	// clusters removed by the scan are intentionally dropped.
	scannedClusters := make(map[string]bool, len(scanResult.Clusters))
	for _, c := range scanResult.Clusters {
		scannedClusters[c.Name] = true
	}
	oldWaves := domain.RestoreWaves(oldState.Waves)
	waves = domain.MergeOldWaves(oldWaves, waves, scannedClusters, failedNames)
	oldCompleted := domain.BuildCompletedWaveMap(oldWaves)
	waves = domain.MergeCompletedStatus(oldCompleted, waves)
	waves = domain.EvaluateUnlocks(waves, domain.BuildCompletedWaveMap(waves))
	completed := domain.BuildCompletedWaveMap(waves)
	adrDir := ADRDir(baseDir)
	adrCount := CountADRFiles(adrDir)
	scanner := bufio.NewScanner(input)

	// Record rescan-specific events via emitter
	emitter.EmitRescan(oldState.SessionID, time.Now().UTC())
	emitter.EmitRecordWavesGenerated(domain.WavesGeneratedPayload{
		Waves: domain.BuildWaveStates(waves),
	}, time.Now().UTC())

	span.SetAttributes(
		attribute.Int("rescan.cluster.count", len(scanResult.Clusters)),
		attribute.Int("rescan.wave.count", len(waves)),
	)

	logger.OK("Re-scanned: %d clusters, %d waves (%d previously completed)",
		len(scanResult.Clusters), len(waves), len(completed))

	return runInteractiveLoop(ctx, cfg, baseDir, sessionID, scanDir, scanResultPath,
		scanResult, waves, completed, adrCount, scanner, adrDir, nil, scanTime, fbCollector, outboxStore, emitter, out, logger)
}
