package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/harness"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// RunSession runs the full session: Pass 1-3 (auto), then interactive wave loop.
func RunSession(ctx context.Context, cfg *domain.Config, baseDir string, sessionID string, dryRun bool, input io.Reader, out io.Writer, emitter port.SessionEventEmitter, logger domain.Logger) error {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	if !dryRun && input == nil {
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
	// CollectFeedback accumulates initial + late-arriving feedback so
	// all feedback is available for nextgen prompt injection.
	var fbCollector *FeedbackCollector
	if !dryRun {
		monitorCtx, monitorCancel := context.WithCancel(ctx)
		defer monitorCancel()
		inboxCh, monitorErr := MonitorInbox(monitorCtx, baseDir, logger)
		if monitorErr != nil {
			logger.Warn("D-Mail monitor failed: %v", monitorErr)
		}
		initial := DrainInboxFeedback(inboxCh, logger)

		// Convergence gate with re-drain: catches late-arriving convergence
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

		fbCollector = CollectFeedbackWithHook(ctx, allDmails, inboxCh, notifier, logger, buildCorrectionInsightHook(baseDir, logger))
	}

	// Create runners once at session startup.
	runner, runnerStore := NewTrackedRunner(AdapterConfigFromDomainConfig(cfg, baseDir), RetryConfigFromDomainConfig(cfg), logger)
	if runnerStore != nil {
		defer runnerStore.Close()
	}
	onceRunner, onceStore := NewOnceRunner(AdapterConfigFromDomainConfig(cfg, baseDir), logger)
	if onceStore != nil {
		defer onceStore.Close()
	}

	scanDir, err := EnsureScanDir(baseDir, sessionID)
	if err != nil {
		return err
	}

	// --- Pass 1+2: Scan (reuse v0.1 RunScan) ---
	scanResult, err := RunScan(ctx, cfg, baseDir, sessionID, dryRun, out, runner, onceRunner, logger)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	// In dry-run mode, RunScan writes classify prompt but returns nil ScanResult.
	// Continue to Pass 3 with sample cluster data so wave-generation prompts are also generated.
	if dryRun {
		sampleClusters := []domain.ClusterScanResult{{
			Name:         "sample",
			Completeness: 0.5,
			Issues:       []domain.IssueDetail{{ID: "SAMPLE-1", Identifier: "SAMPLE-1", Title: "Sample issue", Completeness: 0.5}},
			Observations: []string{"sample observation for dry-run"},
		}}
		if _, _, _, err := RunWaveGenerate(ctx, cfg, scanDir, sampleClusters, true, runner, logger); err != nil {
			return fmt.Errorf("wave generate dry-run: %w", err)
		}
		// Also generate architect discuss prompt for dry-run
		sampleWave := domain.Wave{
			ID:          "sample-w1",
			ClusterName: "sample",
			Title:       "Sample Wave",
			Actions:     []domain.WaveAction{{Type: "add_dod", IssueID: "SAMPLE-1", Description: "Sample DoD"}},
		}
		if err := RunArchitectDiscussDryRun(cfg, scanDir, sampleWave, "sample discussion topic", string(cfg.Strictness.Default), logger); err != nil {
			return fmt.Errorf("architect discuss dry-run: %w", err)
		}
		// Also generate scribe ADR prompt for dry-run
		if cfg.Scribe.Enabled {
			sampleArchitectResp := &domain.ArchitectResponse{
				Analysis:  "Sample architect analysis for dry-run",
				Reasoning: "Sample reasoning",
			}
			if err := RunScribeADRDryRun(cfg, scanDir, sampleWave, sampleArchitectResp, ADRDir(baseDir), string(cfg.Strictness.Default), logger); err != nil {
				return fmt.Errorf("scribe dry-run: %w", err)
			}
		}
		// Also generate nextgen prompt for dry-run
		sampleCompletedWaves := []domain.Wave{sampleWave}
		sampleCluster := domain.ClusterScanResult{Name: "sample", Completeness: 0.5, Issues: sampleClusters[0].Issues}
		if err := GenerateNextWavesDryRun(cfg, scanDir, sampleWave, sampleCluster, sampleCompletedWaves, nil, nil, string(cfg.Strictness.Default), nil, nil, logger); err != nil {
			return fmt.Errorf("nextgen dry-run: %w", err)
		}
		logger.OK("Dry-run complete. Check .siren/.run/ for generated prompts.")
		return nil
	}

	for _, w := range scanResult.ScanWarnings {
		logger.Warn("Partial scan: %s", w)
	}

	// Capture scan timestamp once so it stays stable across wave completions
	scanTime := time.Now()

	// Cache ScanResult + record session start / scan completed events
	scanResultPath := RecordScanState(baseDir, sessionID, scanResult, cfg, emitter, scanTime, logger)

	// Persist LLM-estimated strictness values into config for future resolution
	estimated := make(map[string]domain.StrictnessLevel)
	for _, c := range scanResult.Clusters {
		if c.EstimatedStrictness != "" && c.Key != "" {
			level := domain.StrictnessLevel(c.EstimatedStrictness)
			if level.Valid() {
				estimated[c.Key] = level
			}
		}
	}
	if len(estimated) > 0 {
		cfgPath := domain.ConfigPath(baseDir)
		if err := WriteEstimatedStrictness(cfgPath, estimated); err != nil {
			logger.Warn("Failed to write estimated strictness: %v", err)
		}
	}

	// --- Pass 3: Wave Generate ---
	waves, waveWarnings, _, err := RunWaveGenerate(ctx, cfg, scanDir, scanResult.Clusters, false, runner, logger)
	if err != nil {
		return fmt.Errorf("wave generate: %w", err)
	}
	// waveWarnings are already logged by RunParallel; just propagate.
	_ = waveWarnings

	logger.OK("%d clusters, %d waves generated", len(scanResult.Clusters), len(waves))

	// Record waves generated via emitter
	emitter.EmitRecordWavesGenerated(domain.WavesGeneratedPayload{
		Waves: harness.BuildWaveStates(waves),
	}, time.Now().UTC())

	completed := harness.BuildCompletedWaveMap(waves)
	scanner := bufio.NewScanner(input)
	adrDir := ADRDir(baseDir)
	adrCount := CountADRFiles(adrDir)

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

// ResumeSession loads a previous session's state and cached scan result,
// restoring waves and completed map for the interactive loop.
func ResumeSession(baseDir string, state *domain.SessionState) (*domain.ScanResult, []domain.Wave, map[string]bool, int, error) {
	if state.ScanResultPath == "" {
		return nil, nil, nil, 0, fmt.Errorf("no cached scan result path in state")
	}
	resolvedPath := domain.ResolveScanResultPath(baseDir, state.ScanResultPath)
	scanResult, err := LoadScanResult(resolvedPath)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("load cached scan result: %w", err)
	}
	waves := harness.RestoreWaves(state.Waves)
	completed := harness.BuildCompletedWaveMap(waves)
	adrCount := CountADRFiles(ADRDir(baseDir))
	return scanResult, waves, completed, adrCount, nil
}

// ResumeScanDir returns the scan directory for a resumed session.
// It derives the directory from state.ScanResultPath when available,
// preserving the original path even if the directory layout has changed
// (e.g. .siren/scans/ → .siren/.run/). Falls back to ScanDir() when
// ScanResultPath is empty.
func ResumeScanDir(state *domain.SessionState, baseDir string) string {
	if state.ScanResultPath != "" {
		return filepath.Dir(domain.ResolveScanResultPath(baseDir, state.ScanResultPath))
	}
	return domain.ScanDir(baseDir, state.SessionID)
}

// RunResumeSession resumes an existing session from saved state.
func RunResumeSession(ctx context.Context, cfg *domain.Config, baseDir string, state *domain.SessionState, input io.Reader, out io.Writer, emitter port.SessionEventEmitter, logger domain.Logger) error {
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
	runner, runnerStore := NewTrackedRunner(AdapterConfigFromDomainConfig(cfg, baseDir), RetryConfigFromDomainConfig(cfg), logger)
	if runnerStore != nil {
		defer runnerStore.Close()
	}
	onceRunner, onceStore := NewOnceRunner(AdapterConfigFromDomainConfig(cfg, baseDir), logger)
	if onceStore != nil {
		defer onceStore.Close()
	}

	scanResult, waves, completed, adrCount, err := ResumeSession(baseDir, state)
	if err != nil {
		return fmt.Errorf("resume: %w", err)
	}

	// Validate and repair wave state after restore.
	waves, danglingCount := harness.ValidateWavePrerequisites(waves)
	if danglingCount > 0 {
		logger.Warn("Removed %d dangling wave prerequisites", danglingCount)
	}
	waves, repairedCount := harness.RepairLockedWaves(waves, completed)
	if repairedCount > 0 {
		logger.Info("Repaired %d locked waves with met prerequisites", repairedCount)
	}
	scanDir := ResumeScanDir(state, baseDir)
	if err := os.MkdirAll(scanDir, 0755); err != nil {
		return fmt.Errorf("ensure scan dir: %w", err)
	}
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	scanner := bufio.NewScanner(input)
	adrDir := ADRDir(baseDir)
	lastScanned := state.LastScanned

	// Record resume event via emitter
	emitter.EmitResume(state.SessionID, time.Now().UTC())

	logger.OK("Resumed session: %d waves, %d completed", len(waves), len(completed))

	for {
		result, latestWaves, latestCompleted, err := runInteractiveLoop(ctx, cfg, baseDir, state.SessionID, scanDir, scanResultPath,
			scanResult, waves, completed, adrCount, scanner, adrDir, &lastScanned, lastScanned, fbCollector, outboxStore, runner, onceRunner, emitter, out, logger)
		if err != nil {
			return err
		}
		if result != loopResultRescanNeeded {
			return nil
		}
		logger.Info("Auto-rescan: D-Mail triggered fresh scan")
		var rescanTime time.Time
		scanDir, scanResultPath, scanResult, waves, completed, adrCount, rescanTime, err =
			RescanCore(ctx, cfg, baseDir, state.SessionID, latestWaves, latestCompleted, runner, onceRunner, emitter, out, logger)
		if err != nil {
			return fmt.Errorf("auto-rescan: %w", err)
		}
		lastScanned = rescanTime
	}
}
