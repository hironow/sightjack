package sightjack

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// StateFormatVersion is the version string written into SessionState files.
// Centralised so that all code paths (scan, session, recovery) produce
// consistent state files.
const StateFormatVersion = "0.0.11"

// CalcNewlyUnlocked computes how many waves were newly unlocked after completing a wave.
// oldAvailable is the available count before the wave was completed (includes the completing wave).
// newAvailable is the available count after completion and unlock evaluation.
func CalcNewlyUnlocked(oldAvailable, newAvailable int) int {
	// oldAvailable includes the wave being completed, so subtract 1 to get
	// the baseline of waves that were already available before this action.
	newCount := newAvailable - (oldAvailable - 1)
	if newCount < 0 {
		return 0
	}
	return newCount
}

// buildNotifier creates the appropriate Notifier based on config.
// If NotifyCmd is set, uses CmdNotifier. Otherwise uses LocalNotifier (OS-native).
func buildNotifier(cfg *Config) Notifier {
	if cfg.Gate.NotifyCmd != "" {
		return NewCmdNotifier(cfg.Gate.NotifyCmd)
	}
	return &LocalNotifier{}
}

// buildApprover creates the appropriate Approver based on config.
// Priority: AutoApprove → CmdApprover → StdinApprover.
func buildApprover(cfg *Config, input io.Reader, out io.Writer) Approver {
	if cfg.Gate.AutoApprove {
		return &AutoApprover{}
	}
	if cfg.Gate.ApproveCmd != "" {
		return NewCmdApprover(cfg.Gate.ApproveCmd)
	}
	return NewStdinApprover(input, out)
}

// RunSession runs the full session: Pass 1-3 (auto), then interactive wave loop.
func RunSession(ctx context.Context, cfg *Config, baseDir string, sessionID string, dryRun bool, input io.Reader, out io.Writer, recorder Recorder, logger *Logger) error {
	if logger == nil {
		logger = NewLogger(nil, false)
	}
	if !dryRun && input == nil {
		return fmt.Errorf("input reader is required for interactive session")
	}

	// Ensure D-Mail directories exist before any mail operations
	if err := EnsureMailDirs(baseDir); err != nil {
		return fmt.Errorf("ensure mail dirs: %w", err)
	}

	// Start inbox monitor (fsnotify-based) for feedback d-mails.
	// CollectFeedback accumulates initial + late-arriving feedback so
	// all feedback is available for nextgen prompt injection.
	var fbCollector *feedbackCollector
	if !dryRun {
		monitorCtx, monitorCancel := context.WithCancel(ctx)
		defer monitorCancel()
		inboxCh, monitorErr := MonitorInbox(monitorCtx, baseDir, logger)
		if monitorErr != nil {
			logger.Warn("D-Mail monitor failed: %v", monitorErr)
		}
		initial := DrainInboxFeedback(inboxCh, logger)

		// Convergence gate with re-drain: catches late-arriving convergence
		notifier := buildNotifier(cfg)
		approver := buildApprover(cfg, input, out)
		allDmails, approved, gateErr := RunConvergenceGateWithRedrain(ctx, initial, inboxCh, notifier, approver, logger)
		if gateErr != nil {
			return fmt.Errorf("convergence gate: %w", gateErr)
		}
		if !approved {
			logger.Warn("Session aborted: convergence gate denied")
			return nil
		}

		fbCollector = CollectFeedback(allDmails, inboxCh, notifier, logger)
	}

	scanDir, err := EnsureScanDir(baseDir, sessionID)
	if err != nil {
		return err
	}

	// --- Pass 1+2: Scan (reuse v0.1 RunScan) ---
	scanResult, err := RunScan(ctx, cfg, baseDir, sessionID, dryRun, out, logger)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	// In dry-run mode, RunScan writes classify prompt but returns nil ScanResult.
	// Continue to Pass 3 with sample cluster data so wave-generation prompts are also generated.
	if dryRun {
		sampleClusters := []ClusterScanResult{{
			Name:         "sample",
			Completeness: 0.5,
			Issues:       []IssueDetail{{ID: "SAMPLE-1", Identifier: "SAMPLE-1", Title: "Sample issue", Completeness: 0.5}},
			Observations: []string{"sample observation for dry-run"},
		}}
		if _, _, _, err := RunWaveGenerate(ctx, cfg, scanDir, sampleClusters, true, logger); err != nil {
			return fmt.Errorf("wave generate dry-run: %w", err)
		}
		// Also generate architect discuss prompt for dry-run
		sampleWave := Wave{
			ID:          "sample-w1",
			ClusterName: "sample",
			Title:       "Sample Wave",
			Actions:     []WaveAction{{Type: "add_dod", IssueID: "SAMPLE-1", Description: "Sample DoD"}},
		}
		if err := RunArchitectDiscussDryRun(cfg, scanDir, sampleWave, "sample discussion topic", string(cfg.Strictness.Default), logger); err != nil {
			return fmt.Errorf("architect discuss dry-run: %w", err)
		}
		// Also generate scribe ADR prompt for dry-run
		if cfg.Scribe.Enabled {
			sampleArchitectResp := &ArchitectResponse{
				Analysis:  "Sample architect analysis for dry-run",
				Reasoning: "Sample reasoning",
			}
			if err := RunScribeADRDryRun(cfg, scanDir, sampleWave, sampleArchitectResp, ADRDir(baseDir), string(cfg.Strictness.Default), logger); err != nil {
				return fmt.Errorf("scribe dry-run: %w", err)
			}
		}
		// Also generate nextgen prompt for dry-run
		sampleCompletedWaves := []Wave{sampleWave}
		sampleCluster := ClusterScanResult{Name: "sample", Completeness: 0.5, Issues: sampleClusters[0].Issues}
		if err := GenerateNextWavesDryRun(cfg, scanDir, sampleWave, sampleCluster, sampleCompletedWaves, nil, nil, string(cfg.Strictness.Default), nil, logger); err != nil {
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

	// Cache ScanResult for resume
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	if err := WriteScanResult(scanResultPath, scanResult); err != nil {
		logger.Warn("Failed to cache scan result: %v", err)
	}

	// Record session start + scan completed
	recorder.Record(EventSessionStarted, SessionStartedPayload{
		Project:         cfg.Linear.Project,
		StrictnessLevel: string(cfg.Strictness.Default),
	})
	var clusterStates []ClusterState
	for _, c := range scanResult.Clusters {
		clusterStates = append(clusterStates, ClusterState{
			Name: c.Name, Completeness: c.Completeness, IssueCount: len(c.Issues),
		})
	}
	recorder.Record(EventScanCompleted, ScanCompletedPayload{
		Clusters:       clusterStates,
		Completeness:   scanResult.Completeness,
		ShibitoCount:   len(scanResult.ShibitoWarnings),
		ScanResultPath: scanResultPath,
		LastScanned:    scanTime,
	})

	// --- Pass 3: Wave Generate ---
	waves, waveWarnings, _, err := RunWaveGenerate(ctx, cfg, scanDir, scanResult.Clusters, false, logger)
	if err != nil {
		return fmt.Errorf("wave generate: %w", err)
	}
	// waveWarnings are already logged by RunParallel; just propagate.
	_ = waveWarnings

	logger.OK("%d clusters, %d waves generated", len(scanResult.Clusters), len(waves))

	// Record waves generated
	recorder.Record(EventWavesGenerated, WavesGeneratedPayload{
		Waves: BuildWaveStates(waves),
	})

	completed := BuildCompletedWaveMap(waves)
	scanner := bufio.NewScanner(input)
	adrDir := ADRDir(baseDir)
	adrCount := CountADRFiles(adrDir)

	return runInteractiveLoop(ctx, cfg, baseDir, sessionID, scanDir, scanResultPath,
		scanResult, waves, completed, adrCount, scanner, adrDir, nil, scanTime, fbCollector, recorder, out, logger)
}

// selectPhaseResult describes the outcome of the wave selection phase.
type selectPhaseResult int

const (
	selectChosen selectPhaseResult = iota
	selectQuit
	selectRetry
)

// selectPhase handles the wave selection UI: navigator display, shibito warnings,
// wave selection prompt, go-back handling, and quit handling.
// Returns the selected wave, a result code, and the updated shibitoShown flag.
func selectPhase(ctx context.Context, scanner *bufio.Scanner,
	scanResult *ScanResult, cfg *Config, available []Wave, waves []Wave,
	adrCount int, resumedAt *time.Time, shibitoShown bool,
	out io.Writer, loopSpan trace.Span, logger *Logger) (Wave, selectPhaseResult, bool) {

	// Display Link Navigator
	nav := RenderMatrixNavigator(scanResult, cfg.Linear.Project, waves, adrCount, resumedAt, string(cfg.Strictness.Default), len(scanResult.ShibitoWarnings))
	fmt.Fprintln(out)
	fmt.Fprint(out, nav)

	// Display shibito warnings once (static data, does not change during session)
	if !shibitoShown {
		DisplayShibitoWarnings(out, scanResult.ShibitoWarnings)
		shibitoShown = true
	}

	// Prompt wave selection
	selected, err := PromptWaveSelection(ctx, out, scanner, available)
	if err == ErrQuit {
		loopSpan.AddEvent("session.paused")
		logger.Info("Session paused. State saved.")
		return Wave{}, selectQuit, shibitoShown
	}
	if err == ErrGoBack {
		completedList := CompletedWaves(waves)
		if len(completedList) == 0 {
			logger.Info("No completed waves to revisit.")
			return Wave{}, selectRetry, shibitoShown
		}
		revisit, backErr := PromptCompletedWaveSelection(ctx, out, scanner, completedList)
		if backErr == ErrQuit {
			logger.Info("Session paused. State saved.")
			return Wave{}, selectQuit, shibitoShown
		}
		if backErr != nil {
			return Wave{}, selectRetry, shibitoShown
		}
		DisplayCompletedWaveActions(out, revisit)
		return Wave{}, selectRetry, shibitoShown
	}
	if err != nil {
		logger.Warn("Invalid selection: %v", err)
		return Wave{}, selectRetry, shibitoShown
	}

	return selected, selectChosen, shibitoShown
}

// approvalPhaseResult describes the outcome of the wave approval phase.
type approvalPhaseResult int

const (
	approvalApproved approvalPhaseResult = iota
	approvalRejected
)

// approvalPhase handles the wave approval/reject/discuss/selective loop.
// waves is passed by value (not pointer) because this phase only mutates
// existing elements via PropagateWaveUpdate — it never appends or reassigns.
// Returns the (possibly modified) wave and whether it was approved.
func approvalPhase(ctx context.Context, scanner *bufio.Scanner,
	cfg *Config, scanDir, baseDir string, selected Wave, resolvedStrictness string,
	waves []Wave, completed map[string]bool,
	sessionRejected map[string][]WaveAction, adrDir string, adrCount *int,
	recorder Recorder,
	out io.Writer, loopSpan trace.Span, logger *Logger) (Wave, approvalPhaseResult) {

	for {
		choice, err := PromptWaveApproval(ctx, out, scanner, selected)
		if err == ErrQuit {
			return selected, approvalRejected
		}
		if err != nil {
			logger.Warn("Invalid input: %v", err)
			continue
		}

		switch choice {
		case ApprovalApprove:
			delete(sessionRejected, WaveKey(selected))
			loopSpan.AddEvent("wave.approved",
				trace.WithAttributes(
					attribute.String("wave.id", selected.ID),
					attribute.String("wave.cluster_name", selected.ClusterName),
				),
			)
			recorder.Record(EventWaveApproved, WaveIdentityPayload{
				WaveID: selected.ID, ClusterName: selected.ClusterName,
			})
			if err := ComposeSpecification(baseDir, selected); err != nil {
				logger.Warn("D-Mail specification failed (non-fatal): %v", err)
			}
			recorder.Record(EventSpecificationSent, WaveIdentityPayload{
				WaveID: selected.ID, ClusterName: selected.ClusterName,
			})
			return selected, approvalApproved
		case ApprovalReject:
			delete(sessionRejected, WaveKey(selected))
			loopSpan.AddEvent("wave.rejected",
				trace.WithAttributes(
					attribute.String("wave.id", selected.ID),
					attribute.String("wave.cluster_name", selected.ClusterName),
				),
			)
			recorder.Record(EventWaveRejected, WaveIdentityPayload{
				WaveID: selected.ID, ClusterName: selected.ClusterName,
			})
			logger.Info("Wave rejected.")
			return selected, approvalRejected
		case ApprovalDiscuss:
			topic, topicErr := PromptDiscussTopic(ctx, out, scanner)
			if topicErr == ErrQuit {
				continue
			}
			if topicErr != nil {
				logger.Warn("Invalid topic: %v", topicErr)
				continue
			}
			result, discussErr := RunArchitectDiscuss(ctx, cfg, scanDir, selected, topic, resolvedStrictness, out, logger)
			if discussErr != nil {
				logger.Error("Architect discussion failed: %v", discussErr)
				continue
			}
			DisplayArchitectResponse(out, result)
			if result.ModifiedWave != nil {
				selected = ApplyModifiedWave(selected, *result.ModifiedWave, completed)
				PropagateWaveUpdate(waves, selected)
				recorder.Record(EventWaveModified, WaveModifiedPayload{
					WaveID: selected.ID, ClusterName: selected.ClusterName,
					UpdatedWave: WaveState{
						ID: selected.ID, ClusterName: selected.ClusterName,
						Title: selected.Title, Status: selected.Status,
						Prerequisites: selected.Prerequisites,
						ActionCount:   len(selected.Actions), Actions: selected.Actions,
						Description: selected.Description, Delta: selected.Delta,
					},
				})
				// Trigger Scribe to generate ADR for the modification
				// (runs even for locked waves — the decision itself is worth recording)
				if cfg.Scribe.Enabled {
					scribeResp, scribeErr := RunScribeADR(ctx, cfg, scanDir, selected, result, adrDir, resolvedStrictness, out, logger)
					if scribeErr != nil {
						logger.Warn("Scribe failed (non-fatal): %v", scribeErr)
					} else {
						DisplayScribeResponse(out, scribeResp)
						DisplayADRConflicts(out, scribeResp.Conflicts)
						*adrCount++
						recorder.Record(EventADRGenerated, ADRGeneratedPayload{
							ADRID: scribeResp.ADRID, Title: scribeResp.Title,
						})
					}
				}
				if selected.Status == "locked" {
					logger.Warn("Architect added unmet prerequisites — wave is now locked.")
					return selected, approvalRejected
				}
			}
			continue // back to approval prompt with (possibly modified) wave
		case ApprovalSelective:
			approved, rejected, selErr := PromptSelectiveApproval(ctx, out, scanner, selected)
			if selErr == ErrQuit {
				return selected, approvalRejected
			}
			if selErr != nil {
				logger.Warn("Selective approval error: %v", selErr)
				continue
			}
			if len(approved) == 0 {
				logger.Info("No actions selected. Wave skipped.")
				return selected, approvalRejected
			}
			selected.Actions = approved
			// Recompute delta proportionally when actions were rejected
			totalActions := len(approved) + len(rejected)
			if totalActions > 0 && len(rejected) > 0 {
				fraction := float64(len(approved)) / float64(totalActions)
				selected.Delta.After = selected.Delta.Before + (selected.Delta.After-selected.Delta.Before)*fraction
			}
			PropagateWaveUpdate(waves, selected)
			sessionRejected[WaveKey(selected)] = rejected
			recorder.Record(EventWaveApproved, WaveIdentityPayload{
				WaveID: selected.ID, ClusterName: selected.ClusterName,
			})
			if err := ComposeSpecification(baseDir, selected); err != nil {
				logger.Warn("D-Mail specification failed (non-fatal): %v", err)
			}
			recorder.Record(EventSpecificationSent, WaveIdentityPayload{
				WaveID: selected.ID, ClusterName: selected.ClusterName,
			})
			return selected, approvalApproved
		}
		return selected, approvalRejected
	}
}

// applyPhase handles wave apply, partial failure check, completion marking,
// cluster completeness update, unlock evaluation, nextgen wave generation,
// ready labels, and mid-session state save.
// waves is passed as *[]Wave because append/EvaluateUnlocks may reallocate the slice.
func applyPhase(ctx context.Context, cfg *Config,
	scanDir, scanResultPath, baseDir, adrDir string,
	selected Wave, resolvedStrictness string,
	waves *[]Wave, completed map[string]bool,
	scanResult *ScanResult, sessionRejected map[string][]WaveAction,
	labeledReady map[string]bool,
	fbCollector *feedbackCollector, recorder Recorder, out io.Writer, loopSpan trace.Span, logger *Logger) {

	// --- Pass 4: Wave Apply ---
	applyResult, err := RunWaveApply(ctx, cfg, scanDir, selected, resolvedStrictness, out, logger)
	if err != nil {
		logger.Error("Apply failed: %v", err)
		return
	}

	// Count new waves unlocked by this completion
	oldAvailable := len(AvailableWaves(*waves, completed))

	// Record wave applied (always, even for partial failures)
	recorder.Record(EventWaveApplied, WaveAppliedPayload{
		WaveID: selected.ID, ClusterName: selected.ClusterName,
		Applied: applyResult.Applied, TotalCount: applyResult.TotalCount,
		Errors: applyResult.Errors,
	})

	if !IsWaveApplyComplete(applyResult) {
		loopSpan.AddEvent("wave.partial_failure",
			trace.WithAttributes(
				attribute.String("wave.id", selected.ID),
				attribute.String("wave.cluster_name", selected.ClusterName),
				attribute.Int("wave.error_count", len(applyResult.Errors)),
			),
		)
		logger.Warn("Wave %s partially failed (%d errors). Not marking as completed.", WaveKey(selected), len(applyResult.Errors))
		DisplayRippleEffects(out, applyResult.Ripples)
		return
	}

	loopSpan.AddEvent("wave.completed",
		trace.WithAttributes(
			attribute.String("wave.id", selected.ID),
			attribute.String("wave.cluster_name", selected.ClusterName),
			attribute.Int("wave.action_count", len(selected.Actions)),
		),
	)

	// Mark wave completed using composite key (ClusterName:ID)
	completed[WaveKey(selected)] = true
	selectedKey := WaveKey(selected)
	for i, w := range *waves {
		if WaveKey(w) == selectedKey {
			(*waves)[i].Status = "completed"
			break
		}
	}

	recorder.Record(EventWaveCompleted, WaveCompletedPayload{
		WaveID: selected.ID, ClusterName: selected.ClusterName,
		Applied: applyResult.Applied, TotalCount: applyResult.TotalCount,
	})

	// Compose report d-mail for the completed wave
	if err := ComposeReport(baseDir, selected, applyResult); err != nil {
		logger.Warn("D-Mail report failed (non-fatal): %v", err)
	}
	recorder.Record(EventReportSent, WaveIdentityPayload{
		WaveID: selected.ID, ClusterName: selected.ClusterName,
	})

	// Update cluster completeness from delta, then recalculate overall
	for i, c := range scanResult.Clusters {
		if c.Name == selected.ClusterName {
			adjustedAfter := PartialApplyDelta(applyResult, selected.Delta)
			scanResult.Clusters[i].Completeness = adjustedAfter
			// Note: per-issue completeness is NOT updated here because
			// action types vary (add_dod vs add_dependency) and we lack
			// accurate per-issue deltas. The nextgen prompt already
			// receives CompletedWaves JSON listing all applied actions,
			// which is sufficient for the LLM to avoid re-proposals.
			break
		}
	}
	scanResult.CalculateCompleteness()

	// Record completeness update
	var updatedClusterCompleteness float64
	for _, c := range scanResult.Clusters {
		if c.Name == selected.ClusterName {
			updatedClusterCompleteness = c.Completeness
			break
		}
	}
	recorder.Record(EventCompletenessUpdated, CompletenessUpdatedPayload{
		ClusterName:         selected.ClusterName,
		ClusterCompleteness: updatedClusterCompleteness,
		OverallCompleteness: scanResult.Completeness,
	})

	// Display rich completion summary with grouped ripple effects
	*waves = EvaluateUnlocks(*waves, completed)
	newAvailable := len(AvailableWaves(*waves, completed))
	newCount := CalcNewlyUnlocked(oldAvailable, newAvailable)
	if newCount > 0 {
		var unlockedIDs []string
		for _, w := range *waves {
			if w.Status == "available" {
				unlockedIDs = append(unlockedIDs, WaveKey(w))
			}
		}
		recorder.Record(EventWavesUnlocked, WavesUnlockedPayload{
			UnlockedWaveIDs: unlockedIDs,
		})
	}
	DisplayWaveCompletion(out, selected, applyResult.Ripples, scanResult.Completeness, newCount)

	// --- Post-completion: Generate next waves ---
	var clusterForNextgen ClusterScanResult
	for _, c := range scanResult.Clusters {
		if c.Name == selected.ClusterName {
			clusterForNextgen = c
			break
		}
	}
	if clusterForNextgen.Name == "" {
		logger.Warn("Cluster %q not found in scan results; skipping nextgen", selected.ClusterName)
	} else if !NeedsMoreWaves(clusterForNextgen, *waves) {
		loopSpan.AddEvent("nextgen.skipped",
			trace.WithAttributes(
				attribute.String("wave.cluster_name", selected.ClusterName),
			),
		)
		logger.Debug("Skipping nextgen for %s (complete, waves remain, or cap reached)", selected.ClusterName)
	} else {
		completedWavesForCluster := CompletedWavesForCluster(*waves, selected.ClusterName)
		existingADRs, adrErr := ReadExistingADRs(adrDir)
		if adrErr != nil {
			logger.Warn("Failed to read ADRs for nextgen (non-fatal): %v", adrErr)
		}
		rejectedForWave := sessionRejected[WaveKey(selected)]
		var feedback []*DMail
		if fbCollector != nil {
			feedback = fbCollector.FeedbackOnly()
		}
		newWaves, nextgenErr := GenerateNextWaves(ctx, cfg, scanDir, selected, clusterForNextgen, completedWavesForCluster, existingADRs, rejectedForWave, resolvedStrictness, feedback, logger)
		if nextgenErr != nil {
			logger.Warn("Nextgen failed (non-fatal): %v", nextgenErr)
		} else if len(newWaves) > 0 {
			*waves = append(*waves, newWaves...)
			*waves = EvaluateUnlocks(*waves, completed)
			recorder.Record(EventNextGenWavesAdded, NextGenWavesAddedPayload{
				ClusterName: selected.ClusterName,
				Waves:       BuildWaveStates(newWaves),
			})
		}
	}

	// Apply ready labels after nextgen so the final wave list is used.
	// Only label newly ready issues to avoid redundant API calls.
	if cfg.Labels.Enabled {
		readyIDs := ReadyIssueIDs(*waves)
		var newlyReady []string
		for _, id := range readyIDs {
			if !labeledReady[id] {
				newlyReady = append(newlyReady, id)
			}
		}
		if len(newlyReady) > 0 {
			readyIssueStr := strings.Join(newlyReady, ", ")
			if err := RunReadyLabel(ctx, cfg, readyIssueStr, out, logger); err != nil {
				logger.Warn("Ready label failed: %v", err)
			} else {
				for _, id := range newlyReady {
					labeledReady[id] = true
				}
				recorder.Record(EventReadyLabelsApplied, ReadyLabelsAppliedPayload{
					IssueIDs: newlyReady,
				})
			}
		}
	}

	// Save scan result cache (crash resilience)
	if err := WriteScanResult(scanResultPath, scanResult); err != nil {
		logger.Warn("Failed to update cached scan result: %v", err)
	}
}

// runInteractiveLoop runs the wave selection/approval/apply loop shared by
// RunSession, RunResumeSession, and RunRescanSession.
// resumedAt controls the Navigator "Session: resumed" banner (nil hides it).
// scanTimestamp is persisted in state as LastScanned and stays stable across saves.
func runInteractiveLoop(ctx context.Context, cfg *Config, baseDir, sessionID, scanDir, scanResultPath string,
	scanResult *ScanResult, waves []Wave, completed map[string]bool, adrCount int,
	scanner *bufio.Scanner, adrDir string, resumedAt *time.Time, scanTimestamp time.Time, fbCollector *feedbackCollector, recorder Recorder, out io.Writer, logger *Logger) error {

	ctx, loopSpan := tracer.Start(ctx, "interactive.loop",
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
	sessionRejected := make(map[string][]WaveAction)
	labeledReady := make(map[string]bool) // tracks issues already labeled ready
outerLoop:
	for {
		waves = EvaluateUnlocks(waves, completed)
		available := AvailableWaves(waves, completed)
		if len(available) == 0 {
			logger.OK("All waves completed or no available waves.")
			break
		}

		var selected Wave
		var result selectPhaseResult
		selected, result, shibitoShown = selectPhase(ctx, scanner, scanResult, cfg, available, waves, adrCount, resumedAt, shibitoShown, out, loopSpan, logger)
		switch result {
		case selectQuit:
			break outerLoop
		case selectRetry:
			continue
		}

		resolvedStrictness := string(ResolveStrictness(cfg.Strictness, scanResult.StrictnessKeys(selected.ClusterName)))

		selected, approvalResult := approvalPhase(ctx, scanner, cfg, scanDir, baseDir, selected, resolvedStrictness, waves, completed, sessionRejected, adrDir, &adrCount, recorder, out, loopSpan, logger)
		if approvalResult != approvalApproved {
			continue
		}

		applyPhase(ctx, cfg, scanDir, scanResultPath, baseDir, adrDir,
			selected, resolvedStrictness,
			&waves, completed, scanResult, sessionRejected,
			labeledReady, fbCollector, recorder, out, loopSpan, logger)
	}

	// Final consistency check
	if CheckCompletenessConsistency(scanResult.Completeness, scanResult.Clusters) {
		logger.Warn("Completeness mismatch detected. Recalculating...")
		scanResult.CalculateCompleteness()
	}

	// Save scan result cache
	if err := WriteScanResult(scanResultPath, scanResult); err != nil {
		logger.Warn("Failed to update cached scan result: %v", err)
	}

	logger.OK("Session events saved to %s", filepath.Join(baseDir, StateDir, "events"))
	return nil
}

// CanResume checks whether a saved session state supports resumption.
// It returns false when the cached ScanResult path is empty (e.g. v0.4
// state files) or the file no longer exists on disk.
func CanResume(state *SessionState) bool {
	if state.ScanResultPath == "" {
		return false
	}
	if len(state.Waves) == 0 {
		return false
	}
	_, err := os.Stat(state.ScanResultPath)
	return err == nil
}

// ResumeSession loads a previous session's state and cached scan result,
// restoring waves and completed map for the interactive loop.
func ResumeSession(baseDir string, state *SessionState) (*ScanResult, []Wave, map[string]bool, int, error) {
	if state.ScanResultPath == "" {
		return nil, nil, nil, 0, fmt.Errorf("no cached scan result path in state")
	}
	scanResult, err := LoadScanResult(state.ScanResultPath)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("load cached scan result: %w", err)
	}
	waves := RestoreWaves(state.Waves)
	completed := BuildCompletedWaveMap(waves)
	adrCount := CountADRFiles(ADRDir(baseDir))
	return scanResult, waves, completed, adrCount, nil
}

// ResumeScanDir returns the scan directory for a resumed session.
// It derives the directory from state.ScanResultPath when available,
// preserving the original path even if the directory layout has changed
// (e.g. .siren/scans/ → .siren/.run/). Falls back to ScanDir() when
// ScanResultPath is empty.
func ResumeScanDir(state *SessionState, baseDir string) string {
	if state.ScanResultPath != "" {
		return filepath.Dir(state.ScanResultPath)
	}
	return ScanDir(baseDir, state.SessionID)
}

// RunResumeSession resumes an existing session from saved state.
func RunResumeSession(ctx context.Context, cfg *Config, baseDir string, state *SessionState, input io.Reader, out io.Writer, recorder Recorder, logger *Logger) error {
	if logger == nil {
		logger = NewLogger(nil, false)
	}
	if input == nil {
		return fmt.Errorf("input reader is required for interactive session")
	}

	// Ensure D-Mail directories exist before any mail operations
	if err := EnsureMailDirs(baseDir); err != nil {
		return fmt.Errorf("ensure mail dirs: %w", err)
	}

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
	notifier := buildNotifier(cfg)
	approver := buildApprover(cfg, input, out)
	allDmails, approved, gateErr := RunConvergenceGateWithRedrain(ctx, initial, inboxCh, notifier, approver, logger)
	if gateErr != nil {
		return fmt.Errorf("convergence gate: %w", gateErr)
	}
	if !approved {
		logger.Warn("Session aborted: convergence gate denied")
		return nil
	}

	fbCollector := CollectFeedback(allDmails, inboxCh, notifier, logger)

	scanResult, waves, completed, adrCount, err := ResumeSession(baseDir, state)
	if err != nil {
		return fmt.Errorf("resume: %w", err)
	}
	scanDir := ResumeScanDir(state, baseDir)
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	scanner := bufio.NewScanner(input)
	adrDir := ADRDir(baseDir)
	lastScanned := state.LastScanned

	// Record resume event
	recorder.Record(EventSessionResumed, SessionResumedPayload{
		OriginalSessionID: state.SessionID,
	})

	logger.OK("Resumed session: %d waves, %d completed", len(waves), len(completed))

	return runInteractiveLoop(ctx, cfg, baseDir, state.SessionID, scanDir, scanResultPath,
		scanResult, waves, completed, adrCount, scanner, adrDir, &lastScanned, lastScanned, fbCollector, recorder, out, logger)
}

// RunRescanSession performs a fresh scan then merges completed status from old state.
func RunRescanSession(ctx context.Context, cfg *Config, baseDir string, oldState *SessionState, sessionID string, input io.Reader, out io.Writer, recorder Recorder, logger *Logger) error {
	if logger == nil {
		logger = NewLogger(nil, false)
	}
	if input == nil {
		return fmt.Errorf("input reader is required for interactive session")
	}

	// Ensure D-Mail directories exist before any mail operations
	if err := EnsureMailDirs(baseDir); err != nil {
		return fmt.Errorf("ensure mail dirs: %w", err)
	}

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
	notifier := buildNotifier(cfg)
	approver := buildApprover(cfg, input, out)
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
		return fmt.Errorf("re-scan: %w", err)
	}
	for _, w := range scanResult.ScanWarnings {
		logger.Warn("Partial scan: %s", w)
	}
	scanTime := time.Now()
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	if err := WriteScanResult(scanResultPath, scanResult); err != nil {
		logger.Warn("Failed to cache scan result: %v", err)
	}
	waves, rescanWarnings, failedNames, err := RunWaveGenerate(ctx, cfg, scanDir, scanResult.Clusters, false, logger)
	if err != nil {
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
	oldWaves := RestoreWaves(oldState.Waves)
	waves = mergeOldWaves(oldWaves, waves, scannedClusters, failedNames)
	oldCompleted := BuildCompletedWaveMap(oldWaves)
	waves = MergeCompletedStatus(oldCompleted, waves)
	waves = EvaluateUnlocks(waves, BuildCompletedWaveMap(waves))
	completed := BuildCompletedWaveMap(waves)
	adrDir := ADRDir(baseDir)
	adrCount := CountADRFiles(adrDir)
	scanner := bufio.NewScanner(input)

	// Record rescan events
	recorder.Record(EventSessionRescanned, SessionRescannedPayload{
		OriginalSessionID: oldState.SessionID,
	})
	recorder.Record(EventSessionStarted, SessionStartedPayload{
		Project:         cfg.Linear.Project,
		StrictnessLevel: string(cfg.Strictness.Default),
	})
	var clusterStates []ClusterState
	for _, c := range scanResult.Clusters {
		clusterStates = append(clusterStates, ClusterState{
			Name: c.Name, Completeness: c.Completeness, IssueCount: len(c.Issues),
		})
	}
	recorder.Record(EventScanCompleted, ScanCompletedPayload{
		Clusters:       clusterStates,
		Completeness:   scanResult.Completeness,
		ShibitoCount:   len(scanResult.ShibitoWarnings),
		ScanResultPath: scanResultPath,
		LastScanned:    scanTime,
	})
	recorder.Record(EventWavesGenerated, WavesGeneratedPayload{
		Waves: BuildWaveStates(waves),
	})

	logger.OK("Re-scanned: %d clusters, %d waves (%d previously completed)",
		len(scanResult.Clusters), len(waves), len(completed))

	return runInteractiveLoop(ctx, cfg, baseDir, sessionID, scanDir, scanResultPath,
		scanResult, waves, completed, adrCount, scanner, adrDir, nil, scanTime, fbCollector, recorder, out, logger)
}

// PartialApplyDelta computes the adjusted delta for a partially applied wave.
// When TotalCount is 0, the original delta.After is returned.
func PartialApplyDelta(result *WaveApplyResult, delta WaveDelta) float64 {
	if result.TotalCount == 0 || result.Applied >= result.TotalCount {
		return delta.After
	}
	if result.Applied == 0 {
		return delta.Before
	}
	successRate := float64(result.Applied) / float64(result.TotalCount)
	return delta.Before + (delta.After-delta.Before)*successRate
}

// IsWaveApplyComplete returns true when the apply result has no errors,
// indicating all actions were successfully applied.
func IsWaveApplyComplete(result *WaveApplyResult) bool {
	return len(result.Errors) == 0
}

// ApplyModifiedWave merges a modified wave from the architect into the original,
// preserving identity fields (ID, ClusterName) so that completion bookkeeping
// remains stable. Status is recomputed from the modified prerequisites against
// the completed map to prevent applying waves with unmet dependencies.
func ApplyModifiedWave(original, modified Wave, completed map[string]bool) Wave {
	modified.ID = original.ID
	modified.ClusterName = original.ClusterName

	// Preserve original fields when architect omits them (nil/zero from JSON).
	if modified.Actions == nil {
		modified.Actions = original.Actions
	}
	if modified.Prerequisites == nil {
		modified.Prerequisites = original.Prerequisites
	}
	if modified.Delta == (WaveDelta{}) {
		modified.Delta = original.Delta
	}

	// Normalize bare prerequisite IDs to composite "ClusterName:ID" format.
	for i, p := range modified.Prerequisites {
		if !strings.Contains(p, ":") {
			modified.Prerequisites[i] = modified.ClusterName + ":" + p
		}
	}

	// Recompute status: if any prerequisite is unmet, lock the wave.
	modified.Status = "available"
	for _, prereq := range modified.Prerequisites {
		if !completed[prereq] {
			modified.Status = "locked"
			break
		}
	}
	return modified
}

// PropagateWaveUpdate writes the updated wave back into the waves slice,
// matching by WaveKey so that subsequent AvailableWaves calls see the new state.
func PropagateWaveUpdate(waves []Wave, updated Wave) {
	key := WaveKey(updated)
	for i := range waves {
		if WaveKey(waves[i]) == key {
			waves[i] = updated
			return
		}
	}
}

// BuildCompletedWaveMap returns a set of completed waves keyed by WaveKey (ClusterName:ID).
func BuildCompletedWaveMap(waves []Wave) map[string]bool {
	completed := make(map[string]bool)
	for _, w := range waves {
		if w.Status == "completed" {
			completed[WaveKey(w)] = true
		}
	}
	return completed
}

// mergeOldWaves carries forward waves from clusters that failed wave
// generation but are still present in the current scan. Old waves whose
// cluster was removed from the scan (resolved issues, reorganized clusters)
// are dropped so stale work items do not persist.
//
// failedClusterNames is the set of cluster names where at least one instance
// failed generation (from detectFailedClusterNames). With duplicate cluster
// names, a name marked as failed causes ALL old waves with that name to be
// carried forward — safe over-inclusion to avoid progress loss. Old waves
// whose WaveKey already exists in newWaves are skipped to prevent duplicates.
func mergeOldWaves(oldWaves, newWaves []Wave, scannedClusters, failedClusterNames map[string]bool) []Wave {
	regenerated := make(map[string]bool, len(newWaves))
	newKeys := make(map[string]bool, len(newWaves))
	for _, w := range newWaves {
		regenerated[w.ClusterName] = true
		newKeys[WaveKey(w)] = true
	}
	merged := make([]Wave, 0, len(newWaves)+len(oldWaves))
	merged = append(merged, newWaves...)
	for _, w := range oldWaves {
		inScan := scannedClusters[w.ClusterName]
		noRegeneration := !regenerated[w.ClusterName]
		partialFailure := failedClusterNames[w.ClusterName]
		// Carry forward if cluster is still in scan AND either:
		// - no waves were regenerated for this name (complete failure), OR
		// - at least one instance with this name failed (handles duplicates)
		// Skip waves whose WaveKey already exists in newWaves to avoid duplicates.
		if inScan && (noRegeneration || partialFailure) && !newKeys[WaveKey(w)] {
			merged = append(merged, w)
		}
	}
	return merged
}

// MergeCompletedStatus preserves completed status from a previous session
// when waves are regenerated after a re-scan. Waves in newWaves that match
// a key in oldCompleted are marked "completed". Waves that were in the old
// session but not in newWaves are dropped (Linear removed them).
func MergeCompletedStatus(oldCompleted map[string]bool, newWaves []Wave) []Wave {
	result := make([]Wave, len(newWaves))
	copy(result, newWaves)
	for i, w := range result {
		if oldCompleted[WaveKey(w)] {
			result[i].Status = "completed"
		}
	}
	return result
}

// RestoreWaves converts persisted WaveState list back into Wave list for session resume.
func RestoreWaves(states []WaveState) []Wave {
	waves := make([]Wave, len(states))
	for i, s := range states {
		waves[i] = Wave{
			ID:            s.ID,
			ClusterName:   s.ClusterName,
			Title:         s.Title,
			Description:   s.Description,
			Actions:       s.Actions,
			Prerequisites: s.Prerequisites,
			Delta:         s.Delta,
			Status:        s.Status,
		}
	}
	return waves
}

// BuildWaveStates converts Wave list to WaveState list for persistence.
func BuildWaveStates(waves []Wave) []WaveState {
	states := make([]WaveState, len(waves))
	for i, w := range waves {
		states[i] = WaveState{
			ID:            w.ID,
			ClusterName:   w.ClusterName,
			Title:         w.Title,
			Status:        w.Status,
			Prerequisites: w.Prerequisites,
			ActionCount:   len(w.Actions),
			Actions:       w.Actions,
			Description:   w.Description,
			Delta:         w.Delta,
		}
	}
	return states
}

// CheckCompletenessConsistency verifies that the average of cluster completeness
// values matches the overall completeness within a tolerance. Returns true if a
// mismatch beyond the tolerance (5 percentage points) is detected.
func CheckCompletenessConsistency(overall float64, clusters []ClusterScanResult) bool {
	if len(clusters) == 0 {
		return false
	}
	var sum float64
	for _, c := range clusters {
		sum += c.Completeness
	}
	avg := sum / float64(len(clusters))
	diff := overall - avg
	if diff < 0 {
		diff = -diff
	}
	return diff > 0.05
}

// CompletedWavesForCluster returns all completed waves for the given cluster.
func CompletedWavesForCluster(waves []Wave, clusterName string) []Wave {
	var result []Wave
	for _, w := range waves {
		if w.ClusterName == clusterName && w.Status == "completed" {
			result = append(result, w)
		}
	}
	return result
}
