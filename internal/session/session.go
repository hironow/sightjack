package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	sightjack "github.com/hironow/sightjack"
)

// tracer is the package-level OTel tracer for session operations.
var tracer = otel.Tracer("session")

// RunSession runs the full session: Pass 1-3 (auto), then interactive wave loop.
func RunSession(ctx context.Context, cfg *sightjack.Config, baseDir string, sessionID string, dryRun bool, input io.Reader, out io.Writer, recorder sightjack.Recorder, logger *sightjack.Logger) error {
	if logger == nil {
		logger = sightjack.NewLogger(nil, false)
	}
	recorder = sightjack.NewLoggingRecorder(recorder, logger)
	if !dryRun && input == nil {
		return fmt.Errorf("input reader is required for interactive session")
	}

	// Ensure D-Mail directories exist before any mail operations
	if err := sightjack.EnsureMailDirs(baseDir); err != nil {
		return fmt.Errorf("ensure mail dirs: %w", err)
	}

	// Start inbox monitor (fsnotify-based) for feedback d-mails.
	// CollectFeedback accumulates initial + late-arriving feedback so
	// all feedback is available for nextgen prompt injection.
	var fbCollector *sightjack.FeedbackCollector
	if !dryRun {
		monitorCtx, monitorCancel := context.WithCancel(ctx)
		defer monitorCancel()
		inboxCh, monitorErr := sightjack.MonitorInbox(monitorCtx, baseDir, logger)
		if monitorErr != nil {
			logger.Warn("D-Mail monitor failed: %v", monitorErr)
		}
		initial := sightjack.DrainInboxFeedback(inboxCh, logger)

		// Convergence gate with re-drain: catches late-arriving convergence
		notifier := sightjack.BuildNotifier(cfg)
		approver := sightjack.BuildApprover(cfg, input, out)
		allDmails, approved, gateErr := sightjack.RunConvergenceGateWithRedrain(ctx, initial, inboxCh, notifier, approver, logger)
		if gateErr != nil {
			return fmt.Errorf("convergence gate: %w", gateErr)
		}
		if !approved {
			logger.Warn("Session aborted: convergence gate denied")
			return nil
		}

		fbCollector = sightjack.CollectFeedback(allDmails, inboxCh, notifier, logger)
	}

	scanDir, err := sightjack.EnsureScanDir(baseDir, sessionID)
	if err != nil {
		return err
	}

	// --- Pass 1+2: Scan (reuse v0.1 RunScan) ---
	scanResult, err := sightjack.RunScan(ctx, cfg, baseDir, sessionID, dryRun, out, logger)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	// In dry-run mode, RunScan writes classify prompt but returns nil ScanResult.
	// Continue to Pass 3 with sample cluster data so wave-generation prompts are also generated.
	if dryRun {
		sampleClusters := []sightjack.ClusterScanResult{{
			Name:         "sample",
			Completeness: 0.5,
			Issues:       []sightjack.IssueDetail{{ID: "SAMPLE-1", Identifier: "SAMPLE-1", Title: "Sample issue", Completeness: 0.5}},
			Observations: []string{"sample observation for dry-run"},
		}}
		if _, _, _, err := sightjack.RunWaveGenerate(ctx, cfg, scanDir, sampleClusters, true, logger); err != nil {
			return fmt.Errorf("wave generate dry-run: %w", err)
		}
		// Also generate architect discuss prompt for dry-run
		sampleWave := sightjack.Wave{
			ID:          "sample-w1",
			ClusterName: "sample",
			Title:       "Sample Wave",
			Actions:     []sightjack.WaveAction{{Type: "add_dod", IssueID: "SAMPLE-1", Description: "Sample DoD"}},
		}
		if err := sightjack.RunArchitectDiscussDryRun(cfg, scanDir, sampleWave, "sample discussion topic", string(cfg.Strictness.Default), logger); err != nil {
			return fmt.Errorf("architect discuss dry-run: %w", err)
		}
		// Also generate scribe ADR prompt for dry-run
		if cfg.Scribe.Enabled {
			sampleArchitectResp := &sightjack.ArchitectResponse{
				Analysis:  "Sample architect analysis for dry-run",
				Reasoning: "Sample reasoning",
			}
			if err := sightjack.RunScribeADRDryRun(cfg, scanDir, sampleWave, sampleArchitectResp, sightjack.ADRDir(baseDir), string(cfg.Strictness.Default), logger); err != nil {
				return fmt.Errorf("scribe dry-run: %w", err)
			}
		}
		// Also generate nextgen prompt for dry-run
		sampleCompletedWaves := []sightjack.Wave{sampleWave}
		sampleCluster := sightjack.ClusterScanResult{Name: "sample", Completeness: 0.5, Issues: sampleClusters[0].Issues}
		if err := sightjack.GenerateNextWavesDryRun(cfg, scanDir, sampleWave, sampleCluster, sampleCompletedWaves, nil, nil, string(cfg.Strictness.Default), nil, logger); err != nil {
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
	if err := sightjack.WriteScanResult(scanResultPath, scanResult); err != nil {
		logger.Warn("Failed to cache scan result: %v", err)
	}

	// Record session start + scan completed
	recorder.Record(sightjack.EventSessionStarted, sightjack.SessionStartedPayload{
		Project:         cfg.Linear.Project,
		StrictnessLevel: string(cfg.Strictness.Default),
	})
	var clusterStates []sightjack.ClusterState
	for _, c := range scanResult.Clusters {
		clusterStates = append(clusterStates, sightjack.ClusterState{
			Name: c.Name, Completeness: c.Completeness, IssueCount: len(c.Issues),
		})
	}
	recorder.Record(sightjack.EventScanCompleted, sightjack.ScanCompletedPayload{
		Clusters:       clusterStates,
		Completeness:   scanResult.Completeness,
		ShibitoCount:   len(scanResult.ShibitoWarnings),
		ScanResultPath: scanResultPath,
		LastScanned:    scanTime,
	})

	// --- Pass 3: Wave Generate ---
	waves, waveWarnings, _, err := sightjack.RunWaveGenerate(ctx, cfg, scanDir, scanResult.Clusters, false, logger)
	if err != nil {
		return fmt.Errorf("wave generate: %w", err)
	}
	// waveWarnings are already logged by RunParallel; just propagate.
	_ = waveWarnings

	logger.OK("%d clusters, %d waves generated", len(scanResult.Clusters), len(waves))

	// Record waves generated
	recorder.Record(sightjack.EventWavesGenerated, sightjack.WavesGeneratedPayload{
		Waves: sightjack.BuildWaveStates(waves),
	})

	completed := sightjack.BuildCompletedWaveMap(waves)
	scanner := bufio.NewScanner(input)
	adrDir := sightjack.ADRDir(baseDir)
	adrCount := sightjack.CountADRFiles(adrDir)

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
	scanResult *sightjack.ScanResult, cfg *sightjack.Config, available []sightjack.Wave, waves []sightjack.Wave,
	adrCount int, resumedAt *time.Time, shibitoShown bool,
	out io.Writer, loopSpan trace.Span, logger *sightjack.Logger) (sightjack.Wave, selectPhaseResult, bool) {

	// Display Link Navigator
	nav := sightjack.RenderMatrixNavigator(scanResult, cfg.Linear.Project, waves, adrCount, resumedAt, string(cfg.Strictness.Default), len(scanResult.ShibitoWarnings))
	fmt.Fprintln(out)
	fmt.Fprint(out, nav)

	// Display shibito warnings once (static data, does not change during session)
	if !shibitoShown {
		sightjack.DisplayShibitoWarnings(out, scanResult.ShibitoWarnings)
		shibitoShown = true
	}

	// Prompt wave selection
	selected, err := sightjack.PromptWaveSelection(ctx, out, scanner, available)
	if err == sightjack.ErrQuit {
		loopSpan.AddEvent("session.paused")
		logger.Info("Session paused. State saved.")
		return sightjack.Wave{}, selectQuit, shibitoShown
	}
	if err == sightjack.ErrGoBack {
		completedList := sightjack.CompletedWaves(waves)
		if len(completedList) == 0 {
			logger.Info("No completed waves to revisit.")
			return sightjack.Wave{}, selectRetry, shibitoShown
		}
		revisit, backErr := sightjack.PromptCompletedWaveSelection(ctx, out, scanner, completedList)
		if backErr == sightjack.ErrQuit {
			logger.Info("Session paused. State saved.")
			return sightjack.Wave{}, selectQuit, shibitoShown
		}
		if backErr != nil {
			return sightjack.Wave{}, selectRetry, shibitoShown
		}
		sightjack.DisplayCompletedWaveActions(out, revisit)
		return sightjack.Wave{}, selectRetry, shibitoShown
	}
	if err != nil {
		logger.Warn("Invalid selection: %v", err)
		return sightjack.Wave{}, selectRetry, shibitoShown
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
	cfg *sightjack.Config, scanDir, baseDir string, selected sightjack.Wave, resolvedStrictness string,
	waves []sightjack.Wave, completed map[string]bool,
	sessionRejected map[string][]sightjack.WaveAction, adrDir string, adrCount *int,
	recorder sightjack.Recorder,
	out io.Writer, loopSpan trace.Span, logger *sightjack.Logger) (sightjack.Wave, approvalPhaseResult) {

	for {
		choice, err := sightjack.PromptWaveApproval(ctx, out, scanner, selected)
		if err == sightjack.ErrQuit {
			return selected, approvalRejected
		}
		if err != nil {
			logger.Warn("Invalid input: %v", err)
			continue
		}

		switch choice {
		case sightjack.ApprovalApprove:
			delete(sessionRejected, sightjack.WaveKey(selected))
			loopSpan.AddEvent("wave.approved",
				trace.WithAttributes(
					attribute.String("wave.id", selected.ID),
					attribute.String("wave.cluster_name", selected.ClusterName),
				),
			)
			recorder.Record(sightjack.EventWaveApproved, sightjack.WaveIdentityPayload{
				WaveID: selected.ID, ClusterName: selected.ClusterName,
			})
			if err := sightjack.ComposeSpecification(baseDir, selected); err != nil {
				logger.Warn("D-Mail specification failed (non-fatal): %v", err)
			}
			recorder.Record(sightjack.EventSpecificationSent, sightjack.WaveIdentityPayload{
				WaveID: selected.ID, ClusterName: selected.ClusterName,
			})
			return selected, approvalApproved
		case sightjack.ApprovalReject:
			delete(sessionRejected, sightjack.WaveKey(selected))
			loopSpan.AddEvent("wave.rejected",
				trace.WithAttributes(
					attribute.String("wave.id", selected.ID),
					attribute.String("wave.cluster_name", selected.ClusterName),
				),
			)
			recorder.Record(sightjack.EventWaveRejected, sightjack.WaveIdentityPayload{
				WaveID: selected.ID, ClusterName: selected.ClusterName,
			})
			logger.Info("Wave rejected.")
			return selected, approvalRejected
		case sightjack.ApprovalDiscuss:
			topic, topicErr := sightjack.PromptDiscussTopic(ctx, out, scanner)
			if topicErr == sightjack.ErrQuit {
				continue
			}
			if topicErr != nil {
				logger.Warn("Invalid topic: %v", topicErr)
				continue
			}
			result, discussErr := sightjack.RunArchitectDiscuss(ctx, cfg, scanDir, selected, topic, resolvedStrictness, out, logger)
			if discussErr != nil {
				logger.Error("Architect discussion failed: %v", discussErr)
				continue
			}
			sightjack.DisplayArchitectResponse(out, result)
			if result.ModifiedWave != nil {
				selected = sightjack.ApplyModifiedWave(selected, *result.ModifiedWave, completed)
				sightjack.PropagateWaveUpdate(waves, selected)
				recorder.Record(sightjack.EventWaveModified, sightjack.WaveModifiedPayload{
					WaveID: selected.ID, ClusterName: selected.ClusterName,
					UpdatedWave: sightjack.WaveState{
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
					scribeResp, scribeErr := sightjack.RunScribeADR(ctx, cfg, scanDir, selected, result, adrDir, resolvedStrictness, out, logger)
					if scribeErr != nil {
						logger.Warn("Scribe failed (non-fatal): %v", scribeErr)
					} else {
						sightjack.DisplayScribeResponse(out, scribeResp)
						sightjack.DisplayADRConflicts(out, scribeResp.Conflicts)
						*adrCount++
						recorder.Record(sightjack.EventADRGenerated, sightjack.ADRGeneratedPayload{
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
		case sightjack.ApprovalSelective:
			approved, rejected, selErr := sightjack.PromptSelectiveApproval(ctx, out, scanner, selected)
			if selErr == sightjack.ErrQuit {
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
			sightjack.PropagateWaveUpdate(waves, selected)
			sessionRejected[sightjack.WaveKey(selected)] = rejected
			recorder.Record(sightjack.EventWaveApproved, sightjack.WaveIdentityPayload{
				WaveID: selected.ID, ClusterName: selected.ClusterName,
			})
			if err := sightjack.ComposeSpecification(baseDir, selected); err != nil {
				logger.Warn("D-Mail specification failed (non-fatal): %v", err)
			}
			recorder.Record(sightjack.EventSpecificationSent, sightjack.WaveIdentityPayload{
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
func applyPhase(ctx context.Context, cfg *sightjack.Config,
	scanDir, scanResultPath, baseDir, adrDir string,
	selected sightjack.Wave, resolvedStrictness string,
	waves *[]sightjack.Wave, completed map[string]bool,
	scanResult *sightjack.ScanResult, sessionRejected map[string][]sightjack.WaveAction,
	labeledReady map[string]bool,
	fbCollector *sightjack.FeedbackCollector, recorder sightjack.Recorder, out io.Writer, loopSpan trace.Span, logger *sightjack.Logger) {

	// --- Pass 4: Wave Apply ---
	applyResult, err := sightjack.RunWaveApply(ctx, cfg, scanDir, selected, resolvedStrictness, out, logger)
	if err != nil {
		logger.Error("Apply failed: %v", err)
		return
	}

	// Count new waves unlocked by this completion
	oldAvailable := len(sightjack.AvailableWaves(*waves, completed))

	// Record wave applied (always, even for partial failures)
	recorder.Record(sightjack.EventWaveApplied, sightjack.WaveAppliedPayload{
		WaveID: selected.ID, ClusterName: selected.ClusterName,
		Applied: applyResult.Applied, TotalCount: applyResult.TotalCount,
		Errors: applyResult.Errors,
	})

	if !sightjack.IsWaveApplyComplete(applyResult) {
		loopSpan.AddEvent("wave.partial_failure",
			trace.WithAttributes(
				attribute.String("wave.id", selected.ID),
				attribute.String("wave.cluster_name", selected.ClusterName),
				attribute.Int("wave.error_count", len(applyResult.Errors)),
			),
		)
		logger.Warn("Wave %s partially failed (%d errors). Not marking as completed.", sightjack.WaveKey(selected), len(applyResult.Errors))
		sightjack.DisplayRippleEffects(out, applyResult.Ripples)
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
	completed[sightjack.WaveKey(selected)] = true
	selectedKey := sightjack.WaveKey(selected)
	for i, w := range *waves {
		if sightjack.WaveKey(w) == selectedKey {
			(*waves)[i].Status = "completed"
			break
		}
	}

	recorder.Record(sightjack.EventWaveCompleted, sightjack.WaveCompletedPayload{
		WaveID: selected.ID, ClusterName: selected.ClusterName,
		Applied: applyResult.Applied, TotalCount: applyResult.TotalCount,
	})

	// Compose report d-mail for the completed wave
	if err := sightjack.ComposeReport(baseDir, selected, applyResult); err != nil {
		logger.Warn("D-Mail report failed (non-fatal): %v", err)
	}
	recorder.Record(sightjack.EventReportSent, sightjack.WaveIdentityPayload{
		WaveID: selected.ID, ClusterName: selected.ClusterName,
	})

	// Update cluster completeness from delta, then recalculate overall
	for i, c := range scanResult.Clusters {
		if c.Name == selected.ClusterName {
			adjustedAfter := sightjack.PartialApplyDelta(applyResult, selected.Delta)
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
	recorder.Record(sightjack.EventCompletenessUpdated, sightjack.CompletenessUpdatedPayload{
		ClusterName:         selected.ClusterName,
		ClusterCompleteness: updatedClusterCompleteness,
		OverallCompleteness: scanResult.Completeness,
	})

	// Display rich completion summary with grouped ripple effects
	*waves = sightjack.EvaluateUnlocks(*waves, completed)
	newAvailable := len(sightjack.AvailableWaves(*waves, completed))
	newCount := sightjack.CalcNewlyUnlocked(oldAvailable, newAvailable)
	if newCount > 0 {
		var unlockedIDs []string
		for _, w := range *waves {
			if w.Status == "available" {
				unlockedIDs = append(unlockedIDs, sightjack.WaveKey(w))
			}
		}
		recorder.Record(sightjack.EventWavesUnlocked, sightjack.WavesUnlockedPayload{
			UnlockedWaveIDs: unlockedIDs,
		})
	}
	sightjack.DisplayWaveCompletion(out, selected, applyResult.Ripples, scanResult.Completeness, newCount)

	// --- Post-completion: Generate next waves ---
	var clusterForNextgen sightjack.ClusterScanResult
	for _, c := range scanResult.Clusters {
		if c.Name == selected.ClusterName {
			clusterForNextgen = c
			break
		}
	}
	if clusterForNextgen.Name == "" {
		logger.Warn("Cluster %q not found in scan results; skipping nextgen", selected.ClusterName)
	} else if !sightjack.NeedsMoreWaves(clusterForNextgen, *waves) {
		loopSpan.AddEvent("nextgen.skipped",
			trace.WithAttributes(
				attribute.String("wave.cluster_name", selected.ClusterName),
			),
		)
		logger.Debug("Skipping nextgen for %s (complete, waves remain, or cap reached)", selected.ClusterName)
	} else {
		completedWavesForCluster := sightjack.CompletedWavesForCluster(*waves, selected.ClusterName)
		existingADRs, adrErr := sightjack.ReadExistingADRs(adrDir)
		if adrErr != nil {
			logger.Warn("Failed to read ADRs for nextgen (non-fatal): %v", adrErr)
		}
		rejectedForWave := sessionRejected[sightjack.WaveKey(selected)]
		var feedback []*sightjack.DMail
		if fbCollector != nil {
			feedback = fbCollector.FeedbackOnly()
		}
		newWaves, nextgenErr := sightjack.GenerateNextWaves(ctx, cfg, scanDir, selected, clusterForNextgen, completedWavesForCluster, existingADRs, rejectedForWave, resolvedStrictness, feedback, logger)
		if nextgenErr != nil {
			logger.Warn("Nextgen failed (non-fatal): %v", nextgenErr)
		} else if len(newWaves) > 0 {
			*waves = append(*waves, newWaves...)
			*waves = sightjack.EvaluateUnlocks(*waves, completed)
			recorder.Record(sightjack.EventNextGenWavesAdded, sightjack.NextGenWavesAddedPayload{
				ClusterName: selected.ClusterName,
				Waves:       sightjack.BuildWaveStates(newWaves),
			})
		}
	}

	// Apply ready labels after nextgen so the final wave list is used.
	// Only label newly ready issues to avoid redundant API calls.
	if cfg.Labels.Enabled {
		readyIDs := sightjack.ReadyIssueIDs(*waves)
		var newlyReady []string
		for _, id := range readyIDs {
			if !labeledReady[id] {
				newlyReady = append(newlyReady, id)
			}
		}
		if len(newlyReady) > 0 {
			readyIssueStr := strings.Join(newlyReady, ", ")
			if err := sightjack.RunReadyLabel(ctx, cfg, readyIssueStr, out, logger); err != nil {
				logger.Warn("Ready label failed: %v", err)
			} else {
				for _, id := range newlyReady {
					labeledReady[id] = true
				}
				recorder.Record(sightjack.EventReadyLabelsApplied, sightjack.ReadyLabelsAppliedPayload{
					IssueIDs: newlyReady,
				})
			}
		}
	}

	// Save scan result cache (crash resilience)
	if err := sightjack.WriteScanResult(scanResultPath, scanResult); err != nil {
		logger.Warn("Failed to update cached scan result: %v", err)
	}
}

// runInteractiveLoop runs the wave selection/approval/apply loop shared by
// RunSession, RunResumeSession, and RunRescanSession.
// resumedAt controls the Navigator "Session: resumed" banner (nil hides it).
// scanTimestamp is persisted in state as LastScanned and stays stable across saves.
func runInteractiveLoop(ctx context.Context, cfg *sightjack.Config, baseDir, sessionID, scanDir, scanResultPath string,
	scanResult *sightjack.ScanResult, waves []sightjack.Wave, completed map[string]bool, adrCount int,
	scanner *bufio.Scanner, adrDir string, resumedAt *time.Time, scanTimestamp time.Time, fbCollector *sightjack.FeedbackCollector, recorder sightjack.Recorder, out io.Writer, logger *sightjack.Logger) error {

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
	sessionRejected := make(map[string][]sightjack.WaveAction)
	labeledReady := make(map[string]bool) // tracks issues already labeled ready
outerLoop:
	for {
		waves = sightjack.EvaluateUnlocks(waves, completed)
		available := sightjack.AvailableWaves(waves, completed)
		if len(available) == 0 {
			logger.OK("All waves completed or no available waves.")
			break
		}

		var selected sightjack.Wave
		var result selectPhaseResult
		selected, result, shibitoShown = selectPhase(ctx, scanner, scanResult, cfg, available, waves, adrCount, resumedAt, shibitoShown, out, loopSpan, logger)
		switch result {
		case selectQuit:
			break outerLoop
		case selectRetry:
			continue
		}

		resolvedStrictness := string(sightjack.ResolveStrictness(cfg.Strictness, scanResult.StrictnessKeys(selected.ClusterName)))

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
	if sightjack.CheckCompletenessConsistency(scanResult.Completeness, scanResult.Clusters) {
		logger.Warn("Completeness mismatch detected. Recalculating...")
		scanResult.CalculateCompleteness()
	}

	// Save scan result cache
	if err := sightjack.WriteScanResult(scanResultPath, scanResult); err != nil {
		logger.Warn("Failed to update cached scan result: %v", err)
	}

	logger.OK("Session events saved to %s", filepath.Join(baseDir, sightjack.StateDir, "events"))
	return nil
}

// ResumeSession loads a previous session's state and cached scan result,
// restoring waves and completed map for the interactive loop.
func ResumeSession(baseDir string, state *sightjack.SessionState) (*sightjack.ScanResult, []sightjack.Wave, map[string]bool, int, error) {
	if state.ScanResultPath == "" {
		return nil, nil, nil, 0, fmt.Errorf("no cached scan result path in state")
	}
	scanResult, err := sightjack.LoadScanResult(state.ScanResultPath)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("load cached scan result: %w", err)
	}
	waves := sightjack.RestoreWaves(state.Waves)
	completed := sightjack.BuildCompletedWaveMap(waves)
	adrCount := sightjack.CountADRFiles(sightjack.ADRDir(baseDir))
	return scanResult, waves, completed, adrCount, nil
}

// ResumeScanDir returns the scan directory for a resumed session.
// It derives the directory from state.ScanResultPath when available,
// preserving the original path even if the directory layout has changed
// (e.g. .siren/scans/ → .siren/.run/). Falls back to ScanDir() when
// ScanResultPath is empty.
func ResumeScanDir(state *sightjack.SessionState, baseDir string) string {
	if state.ScanResultPath != "" {
		return filepath.Dir(state.ScanResultPath)
	}
	return sightjack.ScanDir(baseDir, state.SessionID)
}

// RunResumeSession resumes an existing session from saved state.
func RunResumeSession(ctx context.Context, cfg *sightjack.Config, baseDir string, state *sightjack.SessionState, input io.Reader, out io.Writer, recorder sightjack.Recorder, logger *sightjack.Logger) error {
	if logger == nil {
		logger = sightjack.NewLogger(nil, false)
	}
	recorder = sightjack.NewLoggingRecorder(recorder, logger)
	if input == nil {
		return fmt.Errorf("input reader is required for interactive session")
	}

	// Ensure D-Mail directories exist before any mail operations
	if err := sightjack.EnsureMailDirs(baseDir); err != nil {
		return fmt.Errorf("ensure mail dirs: %w", err)
	}

	// Start inbox monitor (fsnotify-based) for feedback d-mails.
	// CollectFeedback accumulates initial + late-arriving feedback.
	monitorCtx, monitorCancel := context.WithCancel(ctx)
	defer monitorCancel()
	inboxCh, monitorErr := sightjack.MonitorInbox(monitorCtx, baseDir, logger)
	if monitorErr != nil {
		logger.Warn("D-Mail monitor failed: %v", monitorErr)
	}
	initial := sightjack.DrainInboxFeedback(inboxCh, logger)

	// Convergence gate with re-drain: catches late-arriving convergence
	notifier := sightjack.BuildNotifier(cfg)
	approver := sightjack.BuildApprover(cfg, input, out)
	allDmails, approved, gateErr := sightjack.RunConvergenceGateWithRedrain(ctx, initial, inboxCh, notifier, approver, logger)
	if gateErr != nil {
		return fmt.Errorf("convergence gate: %w", gateErr)
	}
	if !approved {
		logger.Warn("Session aborted: convergence gate denied")
		return nil
	}

	fbCollector := sightjack.CollectFeedback(allDmails, inboxCh, notifier, logger)

	scanResult, waves, completed, adrCount, err := ResumeSession(baseDir, state)
	if err != nil {
		return fmt.Errorf("resume: %w", err)
	}
	scanDir := ResumeScanDir(state, baseDir)
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	scanner := bufio.NewScanner(input)
	adrDir := sightjack.ADRDir(baseDir)
	lastScanned := state.LastScanned

	// Record resume event
	recorder.Record(sightjack.EventSessionResumed, sightjack.SessionResumedPayload{
		OriginalSessionID: state.SessionID,
	})

	logger.OK("Resumed session: %d waves, %d completed", len(waves), len(completed))

	return runInteractiveLoop(ctx, cfg, baseDir, state.SessionID, scanDir, scanResultPath,
		scanResult, waves, completed, adrCount, scanner, adrDir, &lastScanned, lastScanned, fbCollector, recorder, out, logger)
}

// RunRescanSession performs a fresh scan then merges completed status from old state.
func RunRescanSession(ctx context.Context, cfg *sightjack.Config, baseDir string, oldState *sightjack.SessionState, sessionID string, input io.Reader, out io.Writer, recorder sightjack.Recorder, logger *sightjack.Logger) error {
	if logger == nil {
		logger = sightjack.NewLogger(nil, false)
	}
	recorder = sightjack.NewLoggingRecorder(recorder, logger)
	if input == nil {
		return fmt.Errorf("input reader is required for interactive session")
	}

	// Ensure D-Mail directories exist before any mail operations
	if err := sightjack.EnsureMailDirs(baseDir); err != nil {
		return fmt.Errorf("ensure mail dirs: %w", err)
	}

	// Start inbox monitor (fsnotify-based) for feedback d-mails.
	// CollectFeedback accumulates initial + late-arriving feedback.
	monitorCtx, monitorCancel := context.WithCancel(ctx)
	defer monitorCancel()
	inboxCh, monitorErr := sightjack.MonitorInbox(monitorCtx, baseDir, logger)
	if monitorErr != nil {
		logger.Warn("D-Mail monitor failed: %v", monitorErr)
	}
	initial := sightjack.DrainInboxFeedback(inboxCh, logger)

	// Convergence gate with re-drain: catches late-arriving convergence
	notifier := sightjack.BuildNotifier(cfg)
	approver := sightjack.BuildApprover(cfg, input, out)
	allDmails, approved, gateErr := sightjack.RunConvergenceGateWithRedrain(ctx, initial, inboxCh, notifier, approver, logger)
	if gateErr != nil {
		return fmt.Errorf("convergence gate: %w", gateErr)
	}
	if !approved {
		logger.Warn("Session aborted: convergence gate denied")
		return nil
	}

	fbCollector := sightjack.CollectFeedback(allDmails, inboxCh, notifier, logger)

	scanDir, err := sightjack.EnsureScanDir(baseDir, sessionID)
	if err != nil {
		return err
	}
	scanResult, err := sightjack.RunScan(ctx, cfg, baseDir, sessionID, false, out, logger)
	if err != nil {
		return fmt.Errorf("re-scan: %w", err)
	}
	for _, w := range scanResult.ScanWarnings {
		logger.Warn("Partial scan: %s", w)
	}
	scanTime := time.Now()
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	if err := sightjack.WriteScanResult(scanResultPath, scanResult); err != nil {
		logger.Warn("Failed to cache scan result: %v", err)
	}
	waves, rescanWarnings, failedNames, err := sightjack.RunWaveGenerate(ctx, cfg, scanDir, scanResult.Clusters, false, logger)
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
	oldWaves := sightjack.RestoreWaves(oldState.Waves)
	waves = sightjack.MergeOldWaves(oldWaves, waves, scannedClusters, failedNames)
	oldCompleted := sightjack.BuildCompletedWaveMap(oldWaves)
	waves = sightjack.MergeCompletedStatus(oldCompleted, waves)
	waves = sightjack.EvaluateUnlocks(waves, sightjack.BuildCompletedWaveMap(waves))
	completed := sightjack.BuildCompletedWaveMap(waves)
	adrDir := sightjack.ADRDir(baseDir)
	adrCount := sightjack.CountADRFiles(adrDir)
	scanner := bufio.NewScanner(input)

	// Record rescan events
	recorder.Record(sightjack.EventSessionRescanned, sightjack.SessionRescannedPayload{
		OriginalSessionID: oldState.SessionID,
	})
	recorder.Record(sightjack.EventSessionStarted, sightjack.SessionStartedPayload{
		Project:         cfg.Linear.Project,
		StrictnessLevel: string(cfg.Strictness.Default),
	})
	var clusterStates []sightjack.ClusterState
	for _, c := range scanResult.Clusters {
		clusterStates = append(clusterStates, sightjack.ClusterState{
			Name: c.Name, Completeness: c.Completeness, IssueCount: len(c.Issues),
		})
	}
	recorder.Record(sightjack.EventScanCompleted, sightjack.ScanCompletedPayload{
		Clusters:       clusterStates,
		Completeness:   scanResult.Completeness,
		ShibitoCount:   len(scanResult.ShibitoWarnings),
		ScanResultPath: scanResultPath,
		LastScanned:    scanTime,
	})
	recorder.Record(sightjack.EventWavesGenerated, sightjack.WavesGeneratedPayload{
		Waves: sightjack.BuildWaveStates(waves),
	})

	logger.OK("Re-scanned: %d clusters, %d waves (%d previously completed)",
		len(scanResult.Clusters), len(waves), len(completed))

	return runInteractiveLoop(ctx, cfg, baseDir, sessionID, scanDir, scanResultPath,
		scanResult, waves, completed, adrCount, scanner, adrDir, nil, scanTime, fbCollector, recorder, out, logger)
}
