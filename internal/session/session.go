package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// RunSession runs the full session: Pass 1-3 (auto), then interactive wave loop.
func RunSession(ctx context.Context, cfg *domain.Config, baseDir string, sessionID string, dryRun bool, input io.Reader, out io.Writer, recorder domain.Recorder, logger domain.Logger) error {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	recorder = NewLoggingRecorder(recorder, logger)
	if !dryRun && input == nil {
		return fmt.Errorf("input reader is required for interactive session")
	}

	// Ensure D-Mail directories exist before any mail operations
	if err := EnsureMailDirs(baseDir); err != nil {
		return fmt.Errorf("ensure mail dirs: %w", err)
	}

	// Transactional outbox: SQLite-backed stage → atomic flush to archive/ + outbox/
	outboxStore, err := NewOutboxStoreForBase(baseDir)
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
		sampleClusters := []domain.ClusterScanResult{{
			Name:         "sample",
			Completeness: 0.5,
			Issues:       []domain.IssueDetail{{ID: "SAMPLE-1", Identifier: "SAMPLE-1", Title: "Sample issue", Completeness: 0.5}},
			Observations: []string{"sample observation for dry-run"},
		}}
		if _, _, _, err := RunWaveGenerate(ctx, cfg, scanDir, sampleClusters, true, logger); err != nil {
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
	scanResultPath := RecordScanState(baseDir, sessionID, scanResult, cfg, recorder, scanTime, logger)

	// --- Pass 3: Wave Generate ---
	waves, waveWarnings, _, err := RunWaveGenerate(ctx, cfg, scanDir, scanResult.Clusters, false, logger)
	if err != nil {
		return fmt.Errorf("wave generate: %w", err)
	}
	// waveWarnings are already logged by RunParallel; just propagate.
	_ = waveWarnings

	logger.OK("%d clusters, %d waves generated", len(scanResult.Clusters), len(waves))

	// Record waves generated
	recorder.Record(domain.EventWavesGenerated, domain.WavesGeneratedPayload{
		Waves: domain.BuildWaveStates(waves),
	})

	completed := domain.BuildCompletedWaveMap(waves)
	scanner := bufio.NewScanner(input)
	adrDir := ADRDir(baseDir)
	adrCount := CountADRFiles(adrDir)

	return runInteractiveLoop(ctx, cfg, baseDir, sessionID, scanDir, scanResultPath,
		scanResult, waves, completed, adrCount, scanner, adrDir, nil, scanTime, fbCollector, outboxStore, recorder, out, logger)
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
	scanResult *domain.ScanResult, cfg *domain.Config, available []domain.Wave, waves []domain.Wave,
	adrCount int, resumedAt *time.Time, shibitoShown bool,
	out io.Writer, loopSpan trace.Span, logger domain.Logger) (domain.Wave, selectPhaseResult, bool) {

	// Auto-select first available wave when --auto-approve is set.
	if cfg.Gate.AutoApprove {
		if w, ok := domain.AutoSelectWave(available); ok {
			logger.Info("Auto-selecting wave: %s", w.Title)
			return w, selectChosen, shibitoShown
		}
		return domain.Wave{}, selectQuit, shibitoShown
	}

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
	if err == domain.ErrQuit {
		loopSpan.AddEvent("session.paused")
		logger.Info("Session paused. State saved.")
		return domain.Wave{}, selectQuit, shibitoShown
	}
	if err == domain.ErrGoBack {
		completedList := CompletedWaves(waves)
		if len(completedList) == 0 {
			logger.Info("No completed waves to revisit.")
			return domain.Wave{}, selectRetry, shibitoShown
		}
		revisit, backErr := PromptCompletedWaveSelection(ctx, out, scanner, completedList)
		if backErr == domain.ErrQuit {
			logger.Info("Session paused. State saved.")
			return domain.Wave{}, selectQuit, shibitoShown
		}
		if backErr != nil {
			return domain.Wave{}, selectRetry, shibitoShown
		}
		DisplayCompletedWaveActions(out, revisit)
		return domain.Wave{}, selectRetry, shibitoShown
	}
	if err != nil {
		logger.Warn("Invalid selection: %v", err)
		return domain.Wave{}, selectRetry, shibitoShown
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
	cfg *domain.Config, scanDir string, selected domain.Wave, resolvedStrictness string,
	waves []domain.Wave, completed map[string]bool,
	sessionRejected map[string][]domain.WaveAction, adrDir string, adrCount *int,
	store domain.OutboxStore, recorder domain.Recorder,
	out io.Writer, loopSpan trace.Span, logger domain.Logger) (domain.Wave, approvalPhaseResult) {

	// Auto-approve when --auto-approve is set.
	if cfg.Gate.AutoApprove {
		loopSpan.AddEvent("wave.auto_approved",
			trace.WithAttributes(
				attribute.String("wave.id", selected.ID),
				attribute.String("wave.cluster_name", selected.ClusterName),
			),
		)
		recorder.Record(domain.EventWaveApproved, domain.WaveIdentityPayload{
			WaveID: selected.ID, ClusterName: selected.ClusterName,
		})
		if err := ComposeSpecification(store, selected); err != nil {
			logger.Warn("D-Mail specification failed (non-fatal): %v", err)
		} else {
			recorder.Record(domain.EventSpecificationSent, domain.WaveIdentityPayload{
				WaveID: selected.ID, ClusterName: selected.ClusterName,
			})
		}
		return selected, approvalApproved
	}

	for {
		choice, err := PromptWaveApproval(ctx, out, scanner, selected)
		if err == domain.ErrQuit {
			return selected, approvalRejected
		}
		if err != nil {
			logger.Warn("Invalid input: %v", err)
			continue
		}

		switch choice {
		case domain.ApprovalApprove:
			delete(sessionRejected, domain.WaveKey(selected))
			loopSpan.AddEvent("wave.approved",
				trace.WithAttributes(
					attribute.String("wave.id", selected.ID),
					attribute.String("wave.cluster_name", selected.ClusterName),
				),
			)
			recorder.Record(domain.EventWaveApproved, domain.WaveIdentityPayload{
				WaveID: selected.ID, ClusterName: selected.ClusterName,
			})
			if err := ComposeSpecification(store, selected); err != nil {
				logger.Warn("D-Mail specification failed (non-fatal): %v", err)
			} else {
				recorder.Record(domain.EventSpecificationSent, domain.WaveIdentityPayload{
					WaveID: selected.ID, ClusterName: selected.ClusterName,
				})
			}
			return selected, approvalApproved
		case domain.ApprovalReject:
			delete(sessionRejected, domain.WaveKey(selected))
			loopSpan.AddEvent("wave.rejected",
				trace.WithAttributes(
					attribute.String("wave.id", selected.ID),
					attribute.String("wave.cluster_name", selected.ClusterName),
				),
			)
			recorder.Record(domain.EventWaveRejected, domain.WaveIdentityPayload{
				WaveID: selected.ID, ClusterName: selected.ClusterName,
			})
			platform.RecordWave(ctx, "rejected")
			logger.Info("Wave rejected.")
			return selected, approvalRejected
		case domain.ApprovalDiscuss:
			topic, topicErr := PromptDiscussTopic(ctx, out, scanner)
			if topicErr == domain.ErrQuit {
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
				selected = domain.ApplyModifiedWave(selected, *result.ModifiedWave, completed)
				domain.PropagateWaveUpdate(waves, selected)
				recorder.Record(domain.EventWaveModified, domain.WaveModifiedPayload{
					WaveID: selected.ID, ClusterName: selected.ClusterName,
					UpdatedWave: domain.WaveState{
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
						recorder.Record(domain.EventADRGenerated, domain.ADRGeneratedPayload{
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
		case domain.ApprovalSelective:
			approved, rejected, selErr := PromptSelectiveApproval(ctx, out, scanner, selected)
			if selErr == domain.ErrQuit {
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
			domain.PropagateWaveUpdate(waves, selected)
			sessionRejected[domain.WaveKey(selected)] = rejected
			recorder.Record(domain.EventWaveApproved, domain.WaveIdentityPayload{
				WaveID: selected.ID, ClusterName: selected.ClusterName,
			})
			if err := ComposeSpecification(store, selected); err != nil {
				logger.Warn("D-Mail specification failed (non-fatal): %v", err)
			} else {
				recorder.Record(domain.EventSpecificationSent, domain.WaveIdentityPayload{
					WaveID: selected.ID, ClusterName: selected.ClusterName,
				})
			}
			return selected, approvalApproved
		}
		return selected, approvalRejected
	}
}

// executeAndRecordApply runs the wave apply step, records the event, and checks
// for partial failure. Returns the apply result, the count of previously
// available waves (needed for unlock diff), and whether the apply succeeded.
func executeAndRecordApply(ctx context.Context, cfg *domain.Config,
	scanDir string, selected domain.Wave, resolvedStrictness string,
	waves *[]domain.Wave, completed map[string]bool,
	recorder domain.Recorder, out io.Writer, loopSpan trace.Span, logger domain.Logger) (*domain.WaveApplyResult, int, bool) {

	applyResult, err := RunWaveApply(ctx, cfg, scanDir, selected, resolvedStrictness, out, logger)
	if err != nil {
		logger.Error("Apply failed: %v", err)
		return nil, 0, false
	}

	oldAvailable := len(domain.AvailableWaves(*waves, completed))

	recorder.Record(domain.EventWaveApplied, domain.WaveAppliedPayload{
		WaveID: selected.ID, ClusterName: selected.ClusterName,
		Applied: applyResult.Applied, TotalCount: applyResult.TotalCount,
		Errors: applyResult.Errors,
	})
	platform.RecordWave(ctx, "applied")

	if !domain.IsWaveApplyComplete(applyResult) {
		loopSpan.AddEvent("wave.partial_failure",
			trace.WithAttributes(
				attribute.String("wave.id", selected.ID),
				attribute.String("wave.cluster_name", selected.ClusterName),
				attribute.Int("wave.error_count", len(applyResult.Errors)),
			),
		)
		logger.Warn("Wave %s partially failed (%d errors). Not marking as completed.", domain.WaveKey(selected), len(applyResult.Errors))
		DisplayRippleEffects(out, applyResult.Ripples)
		return applyResult, oldAvailable, false
	}

	return applyResult, oldAvailable, true
}

// generateNextWavesIfNeeded generates next-gen waves for the completed wave's
// cluster if more waves are needed. Updates the waves slice in place.
func generateNextWavesIfNeeded(ctx context.Context, cfg *domain.Config,
	scanDir, adrDir string, selected domain.Wave, resolvedStrictness string,
	waves *[]domain.Wave, completed map[string]bool,
	scanResult *domain.ScanResult, sessionRejected map[string][]domain.WaveAction,
	fbCollector *FeedbackCollector, recorder domain.Recorder, loopSpan trace.Span, logger domain.Logger) {

	var clusterForNextgen domain.ClusterScanResult
	for _, c := range scanResult.Clusters {
		if c.Name == selected.ClusterName {
			clusterForNextgen = c
			break
		}
	}
	if clusterForNextgen.Name == "" {
		logger.Warn("Cluster %q not found in scan results; skipping nextgen", selected.ClusterName)
		return
	}
	if !NeedsMoreWaves(clusterForNextgen, *waves) {
		loopSpan.AddEvent("nextgen.skipped",
			trace.WithAttributes(
				attribute.String("wave.cluster_name", selected.ClusterName),
			),
		)
		logger.Debug("Skipping nextgen for %s (complete, waves remain, or cap reached)", selected.ClusterName)
		return
	}

	completedWavesForCluster := domain.CompletedWavesForCluster(*waves, selected.ClusterName)
	existingADRs, adrErr := ReadExistingADRs(adrDir)
	if adrErr != nil {
		logger.Warn("Failed to read ADRs for nextgen (non-fatal): %v", adrErr)
	}
	rejectedForWave := sessionRejected[domain.WaveKey(selected)]
	var feedback []*DMail
	var reports []*DMail
	if fbCollector != nil {
		feedback = fbCollector.FeedbackOnly()
		reports = fbCollector.ReportsOnly()
	}
	newWaves, nextgenErr := GenerateNextWaves(ctx, cfg, scanDir, selected, clusterForNextgen, completedWavesForCluster, existingADRs, rejectedForWave, resolvedStrictness, feedback, reports, logger)
	if nextgenErr != nil {
		logger.Warn("Nextgen failed (non-fatal): %v", nextgenErr)
	} else if len(newWaves) > 0 {
		*waves = append(*waves, newWaves...)
		*waves = domain.EvaluateUnlocks(*waves, completed)
		recorder.Record(domain.EventNextGenWavesAdded, domain.NextGenWavesAddedPayload{
			ClusterName: selected.ClusterName,
			Waves:       domain.BuildWaveStates(newWaves),
		})
	}
}

// applyReadyLabelsIfEnabled applies "ready" labels to issues that are newly
// ready (all prerequisite waves completed). Only labels issues not yet tracked
// in labeledReady to avoid redundant API calls.
func applyReadyLabelsIfEnabled(ctx context.Context, cfg *domain.Config,
	waves *[]domain.Wave, completed map[string]bool,
	labeledReady map[string]bool, recorder domain.Recorder, out io.Writer, logger domain.Logger) {

	if !cfg.Labels.Enabled {
		return
	}
	readyIDs := ReadyIssueIDs(*waves)
	var newlyReady []string
	for _, id := range readyIDs {
		if !labeledReady[id] {
			newlyReady = append(newlyReady, id)
		}
	}
	if len(newlyReady) == 0 {
		return
	}
	readyIssueStr := strings.Join(newlyReady, ", ")
	if err := RunReadyLabel(ctx, cfg, readyIssueStr, out, logger); err != nil {
		logger.Warn("Ready label failed: %v", err)
		return
	}
	for _, id := range newlyReady {
		labeledReady[id] = true
	}
	recorder.Record(domain.EventReadyLabelsApplied, domain.ReadyLabelsAppliedPayload{
		IssueIDs: newlyReady,
	})
}

// applyPhase handles wave apply, partial failure check, completion marking,
// cluster completeness update, unlock evaluation, nextgen wave generation,
// ready labels, and mid-session state save.
// waves is passed as *[]Wave because append/EvaluateUnlocks may reallocate the slice.
func applyPhase(ctx context.Context, cfg *domain.Config,
	scanDir, scanResultPath, adrDir string,
	selected domain.Wave, resolvedStrictness string,
	waves *[]domain.Wave, completed map[string]bool,
	scanResult *domain.ScanResult, sessionRejected map[string][]domain.WaveAction,
	labeledReady map[string]bool,
	fbCollector *FeedbackCollector, store domain.OutboxStore, recorder domain.Recorder, out io.Writer, loopSpan trace.Span, logger domain.Logger) {

	applyResult, oldAvailable, ok := executeAndRecordApply(ctx, cfg, scanDir, selected, resolvedStrictness, waves, completed, recorder, out, loopSpan, logger)
	if !ok {
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
	completed[domain.WaveKey(selected)] = true
	selectedKey := domain.WaveKey(selected)
	for i, w := range *waves {
		if domain.WaveKey(w) == selectedKey {
			(*waves)[i].Status = "completed"
			break
		}
	}

	recorder.Record(domain.EventWaveCompleted, domain.WaveCompletedPayload{
		WaveID: selected.ID, ClusterName: selected.ClusterName,
		Applied: applyResult.Applied, TotalCount: applyResult.TotalCount,
	})

	// Review gate: run review before composing report (outbox is read immediately by phonewave)
	if cfg.Gate.ReviewCmd != "" {
		passed, reviewErr := RunReviewGate(ctx, cfg, scanDir, logger)
		if reviewErr != nil {
			logger.Warn("Review gate error (non-fatal): %v", reviewErr)
		}
		if !passed {
			logger.Warn("Review gate: not passed — skipping ComposeReport for wave %s", domain.WaveKey(selected))
			return
		}
	}

	// Compose report d-mail for the completed wave
	if err := ComposeReport(store, selected, applyResult); err != nil {
		logger.Warn("D-Mail report failed (non-fatal): %v", err)
	} else {
		recorder.Record(domain.EventReportSent, domain.WaveIdentityPayload{
			WaveID: selected.ID, ClusterName: selected.ClusterName,
		})
	}

	// O2: sightjack → amadeus feedback D-Mail
	if feedbackErr := ComposeFeedback(store, selected, applyResult); feedbackErr != nil {
		logger.Warn("D-Mail feedback failed (non-fatal): %v", feedbackErr)
	} else {
		recorder.Record(domain.EventFeedbackSent, domain.WaveIdentityPayload{
			WaveID: selected.ID, ClusterName: selected.ClusterName,
		})
	}

	// Update cluster completeness from delta, then recalculate overall
	for i, c := range scanResult.Clusters {
		if c.Name == selected.ClusterName {
			adjustedAfter := domain.PartialApplyDelta(applyResult, selected.Delta)
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
	recorder.Record(domain.EventCompletenessUpdated, domain.CompletenessUpdatedPayload{
		ClusterName:         selected.ClusterName,
		ClusterCompleteness: updatedClusterCompleteness,
		OverallCompleteness: scanResult.Completeness,
	})

	// Display rich completion summary with grouped ripple effects
	// Capture available wave keys before unlock to compute the diff
	beforeAvailable := make(map[string]bool)
	for _, w := range domain.AvailableWaves(*waves, completed) {
		beforeAvailable[domain.WaveKey(w)] = true
	}
	*waves = domain.EvaluateUnlocks(*waves, completed)
	newAvailable := len(domain.AvailableWaves(*waves, completed))
	newCount := domain.CalcNewlyUnlocked(oldAvailable, newAvailable)
	if newCount > 0 {
		var unlockedIDs []string
		for _, w := range domain.AvailableWaves(*waves, completed) {
			key := domain.WaveKey(w)
			if !beforeAvailable[key] {
				unlockedIDs = append(unlockedIDs, key)
			}
		}
		recorder.Record(domain.EventWavesUnlocked, domain.WavesUnlockedPayload{
			UnlockedWaveIDs: unlockedIDs,
		})
	}
	DisplayWaveCompletion(out, selected, applyResult.Ripples, scanResult.Completeness, newCount)

	generateNextWavesIfNeeded(ctx, cfg, scanDir, adrDir, selected, resolvedStrictness,
		waves, completed, scanResult, sessionRejected, fbCollector, recorder, loopSpan, logger)

	applyReadyLabelsIfEnabled(ctx, cfg, waves, completed, labeledReady, recorder, out, logger)

	// Save scan result cache (crash resilience)
	if err := WriteScanResult(scanResultPath, scanResult); err != nil {
		logger.Warn("Failed to update cached scan result: %v", err)
	}
}

// runInteractiveLoop runs the wave selection/approval/apply loop shared by
// RunSession, RunResumeSession, and RunRescanSession.
// resumedAt controls the Navigator "Session: resumed" banner (nil hides it).
// scanTimestamp is persisted in state as LastScanned and stays stable across saves.
func runInteractiveLoop(ctx context.Context, cfg *domain.Config, baseDir, sessionID, scanDir, scanResultPath string,
	scanResult *domain.ScanResult, waves []domain.Wave, completed map[string]bool, adrCount int,
	scanner *bufio.Scanner, adrDir string, resumedAt *time.Time, scanTimestamp time.Time, fbCollector *FeedbackCollector,
	store domain.OutboxStore, recorder domain.Recorder, out io.Writer, logger domain.Logger) error {

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

		selected, approvalResult := approvalPhase(ctx, scanner, cfg, scanDir, selected, resolvedStrictness, waves, completed, sessionRejected, adrDir, &adrCount, store, recorder, out, loopSpan, logger)
		if approvalResult != approvalApproved {
			continue
		}

		applyPhase(ctx, cfg, scanDir, scanResultPath, adrDir,
			selected, resolvedStrictness,
			&waves, completed, scanResult, sessionRejected,
			labeledReady, fbCollector, store, recorder, out, loopSpan, logger)
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
	waves := domain.RestoreWaves(state.Waves)
	completed := domain.BuildCompletedWaveMap(waves)
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
func RunResumeSession(ctx context.Context, cfg *domain.Config, baseDir string, state *domain.SessionState, input io.Reader, out io.Writer, recorder domain.Recorder, logger domain.Logger) error {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	recorder = NewLoggingRecorder(recorder, logger)
	if input == nil {
		return fmt.Errorf("input reader is required for interactive session")
	}

	// Ensure D-Mail directories exist before any mail operations
	if err := EnsureMailDirs(baseDir); err != nil {
		return fmt.Errorf("ensure mail dirs: %w", err)
	}

	// Transactional outbox: SQLite-backed stage → atomic flush to archive/ + outbox/
	outboxStore, err := NewOutboxStoreForBase(baseDir)
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
	recorder.Record(domain.EventSessionResumed, domain.SessionResumedPayload{
		OriginalSessionID: state.SessionID,
	})

	logger.OK("Resumed session: %d waves, %d completed", len(waves), len(completed))

	return runInteractiveLoop(ctx, cfg, baseDir, state.SessionID, scanDir, scanResultPath,
		scanResult, waves, completed, adrCount, scanner, adrDir, &lastScanned, lastScanned, fbCollector, outboxStore, recorder, out, logger)
}

// RunRescanSession performs a fresh scan then merges completed status from old state.
func RunRescanSession(ctx context.Context, cfg *domain.Config, baseDir string, oldState *domain.SessionState, sessionID string, input io.Reader, out io.Writer, recorder domain.Recorder, logger domain.Logger) error {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	recorder = NewLoggingRecorder(recorder, logger)
	if input == nil {
		return fmt.Errorf("input reader is required for interactive session")
	}

	// Ensure D-Mail directories exist before any mail operations
	if err := EnsureMailDirs(baseDir); err != nil {
		return fmt.Errorf("ensure mail dirs: %w", err)
	}

	// Transactional outbox: SQLite-backed stage → atomic flush to archive/ + outbox/
	outboxStore, err := NewOutboxStoreForBase(baseDir)
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
		return fmt.Errorf("re-scan: %w", err)
	}
	for _, w := range scanResult.ScanWarnings {
		logger.Warn("Partial scan: %s", w)
	}
	scanTime := time.Now()

	// Cache ScanResult + record session start / scan completed events
	scanResultPath := RecordScanState(baseDir, sessionID, scanResult, cfg, recorder, scanTime, logger)

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
	oldWaves := domain.RestoreWaves(oldState.Waves)
	waves = domain.MergeOldWaves(oldWaves, waves, scannedClusters, failedNames)
	oldCompleted := domain.BuildCompletedWaveMap(oldWaves)
	waves = domain.MergeCompletedStatus(oldCompleted, waves)
	waves = domain.EvaluateUnlocks(waves, domain.BuildCompletedWaveMap(waves))
	completed := domain.BuildCompletedWaveMap(waves)
	adrDir := ADRDir(baseDir)
	adrCount := CountADRFiles(adrDir)
	scanner := bufio.NewScanner(input)

	// Record rescan-specific events
	recorder.Record(domain.EventSessionRescanned, domain.SessionRescannedPayload{
		OriginalSessionID: oldState.SessionID,
	})
	recorder.Record(domain.EventWavesGenerated, domain.WavesGeneratedPayload{
		Waves: domain.BuildWaveStates(waves),
	})

	logger.OK("Re-scanned: %d clusters, %d waves (%d previously completed)",
		len(scanResult.Clusters), len(waves), len(completed))

	return runInteractiveLoop(ctx, cfg, baseDir, sessionID, scanDir, scanResultPath,
		scanResult, waves, completed, adrCount, scanner, adrDir, nil, scanTime, fbCollector, outboxStore, recorder, out, logger)
}
