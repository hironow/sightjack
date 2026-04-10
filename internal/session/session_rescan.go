package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// RescanCore performs the scan+wavegen+merge cycle without session setup
// (D-Mail monitoring, convergence gate, outbox). It is the reusable core
// shared by RunRescanSession (cmd-layer rescan) and the in-loop auto-rescan
// triggered by design-feedback D-Mail arrival.
func RescanCore(ctx context.Context, cfg *domain.Config, baseDir, sessionID string,
	oldWaves []domain.Wave, oldCompleted map[string]bool,
	runner port.ClaudeRunner, onceRunner port.ClaudeRunner,
	emitter port.SessionEventEmitter, out io.Writer, logger domain.Logger,
) (scanDir, scanResultPath string, scanResult *domain.ScanResult,
	waves []domain.Wave, completed map[string]bool,
	adrCount int, scanTime time.Time, err error) {

	scanDir, err = EnsureScanDir(baseDir, sessionID)
	if err != nil {
		return
	}
	scanResult, err = RunScan(ctx, cfg, baseDir, sessionID, false, out, runner, onceRunner, logger)
	if err != nil {
		err = fmt.Errorf("re-scan: %w", err)
		return
	}
	for _, w := range scanResult.ScanWarnings {
		logger.Warn("Partial scan: %s", w)
	}
	scanTime = time.Now()

	// Cache ScanResult + record session start / scan completed events
	scanResultPath = RecordScanState(baseDir, sessionID, scanResult, cfg, emitter, scanTime, logger)

	// Collect issue IDs from previously completed waves as race condition guard:
	// these issues have already received spec D-Mails but paintress may not have
	// applied the pr-open label yet.
	specSentIssues := harness.CollectSpecSentIssueIDs(oldCompleted, oldWaves)

	var failedNames map[string]bool
	waves, _, failedNames, err = RunWaveGenerate(ctx, cfg, scanDir, scanResult.Clusters, false, runner, logger, specSentIssues)
	if err != nil {
		err = fmt.Errorf("wave generate: %w", err)
		return
	}

	// Prune stale waves from old state before restoring — removes waves
	// whose clusters no longer exist in the fresh scan results.
	validClusters := make([]domain.ClusterState, len(scanResult.Clusters))
	for i, c := range scanResult.Clusters {
		validClusters[i] = domain.ClusterState{Name: c.Name, Completeness: c.Completeness, IssueCount: len(c.Issues)}
	}
	// Build a temporary SessionState for pruning (PruneStaleWaves expects *SessionState).
	oldState := &domain.SessionState{
		Waves: harness.BuildWaveStates(oldWaves),
	}
	if pruned := harness.PruneStaleWaves(oldState, validClusters); pruned > 0 {
		logger.Warn("Pruned %d stale waves from previous session", pruned)
	}

	// Carry forward old waves whose clusters failed regeneration.
	scannedClusters := make(map[string]bool, len(scanResult.Clusters))
	for _, c := range scanResult.Clusters {
		scannedClusters[c.Name] = true
	}
	oldWavesRestored := harness.RestoreWaves(oldState.Waves)
	waves = harness.MergeOldWaves(oldWavesRestored, waves, scannedClusters, failedNames)
	waves = harness.MergeCompletedStatus(oldCompleted, waves)
	waves = harness.EvaluateUnlocks(waves, harness.BuildCompletedWaveMap(waves))
	completed = harness.BuildCompletedWaveMap(waves)
	adrCount = CountADRFiles(ADRDir(baseDir))

	// Record rescan events
	emitter.EmitRescan(sessionID, time.Now().UTC())
	emitter.EmitRecordWavesGenerated(domain.WavesGeneratedPayload{
		Waves: harness.BuildWaveStates(waves),
	}, time.Now().UTC())

	logger.OK("Re-scanned: %d clusters, %d waves (%d previously completed)",
		len(scanResult.Clusters), len(waves), len(completed))

	return
}

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
	if oldState == nil {
		return fmt.Errorf("rescan requires previous session state (oldState is nil)")
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
	monitorCtx, monitorCancel := context.WithCancel(ctx)
	defer monitorCancel()
	inboxCh, monitorErr := MonitorInbox(monitorCtx, baseDir, logger)
	if monitorErr != nil {
		logger.Warn("D-Mail monitor failed: %v", monitorErr)
	}
	initial := DrainInboxFeedback(inboxCh, logger)

	// Convergence gate with re-drain
	notifier := BuildNotifier(cfg.Gate)
	approver := BuildApprover(cfg.Gate, input, out)
	allDmails, approved, gateErr := RunConvergenceGateWithRedrain(ctx, initial, inboxCh, notifier, approver, logger)
	if gateErr != nil {
		return fmt.Errorf("convergence gate: %w", gateErr)
	}
	if !approved {
		logger.Warn("Session aborted: convergence gate denied")
		return nil
	}

	fbCollector := CollectFeedbackWithHook(ctx, allDmails, inboxCh, notifier, logger, buildCorrectionInsightHook(baseDir, logger))

	// Create runners once at session startup.
	runner := NewTrackedRunner(cfg, baseDir, logger)
	onceRunner := NewOnceRunner(cfg, baseDir, logger)

	// Initial rescan via RescanCore
	oldWaves := harness.RestoreWaves(oldState.Waves)
	oldCompleted := harness.BuildCompletedWaveMap(oldWaves)
	scanDir, scanResultPath, scanResult, waves, completed, adrCount, scanTime, err :=
		RescanCore(ctx, cfg, baseDir, sessionID, oldWaves, oldCompleted, runner, onceRunner, emitter, out, logger)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.stage", "sightjack.rescan.core"))
		return err
	}

	span.SetAttributes(
		attribute.Int("rescan.cluster.count", len(scanResult.Clusters)),
		attribute.Int("rescan.wave.count", len(waves)),
	)

	scanner := bufio.NewScanner(input)
	adrDir := ADRDir(baseDir)

	for {
		result, latestWaves, latestCompleted, err := runInteractiveLoop(ctx, cfg, baseDir, sessionID, scanDir, scanResultPath,
			scanResult, waves, completed, adrCount, scanner, adrDir, nil, scanTime, fbCollector, outboxStore, runner, onceRunner, emitter, out, logger)
		if err != nil {
			return err
		}
		if result != loopResultRescanNeeded {
			return nil
		}
		logger.Info("Auto-rescan: D-Mail triggered fresh scan")
		scanDir, scanResultPath, scanResult, waves, completed, adrCount, scanTime, err =
			RescanCore(ctx, cfg, baseDir, sessionID, latestWaves, latestCompleted, runner, onceRunner, emitter, out, logger)
		if err != nil {
			return fmt.Errorf("auto-rescan: %w", err)
		}
	}
}
