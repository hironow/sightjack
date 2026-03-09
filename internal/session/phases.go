package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
	"github.com/hironow/sightjack/internal/usecase/port"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

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
	out io.Writer, sessionSpan trace.Span, logger domain.Logger) (domain.Wave, selectPhaseResult, bool) {

	gate := cfg.Gate

	// Auto-select first available wave when --auto-approve is set.
	if gate.IsAutoApprove() {
		if w, ok := domain.AutoSelectWave(available); ok {
			logger.Info("Auto-selecting wave: %s", w.Title)
			return w, selectChosen, shibitoShown
		}
		return domain.Wave{}, selectQuit, shibitoShown
	}

	// Display Link Navigator
	nav := RenderMatrixNavigator(scanResult, cfg.Tracker.Project, waves, adrCount, resumedAt, string(cfg.Strictness.Default), len(scanResult.ShibitoWarnings))
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
		sessionSpan.AddEvent("session.paused")
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
	feedback []*DMail,
	store port.OutboxStore, emitter port.SessionEventEmitter,
	out io.Writer, waveSpan trace.Span, logger domain.Logger) (domain.Wave, approvalPhaseResult) {

	gate := cfg.Gate

	// Auto-approve when --auto-approve is set.
	if gate.IsAutoApprove() {
		waveSpan.AddEvent("wave.auto_approved",
			trace.WithAttributes(
				attribute.String("wave.id", selected.ID),
				attribute.String("wave.cluster_name", selected.ClusterName),
			),
		)
		emitter.EmitApproveWave(selected.ID, selected.ClusterName, time.Now().UTC())

		// Auto-discuss: Devil's Advocate debate → ADR generation
		if cfg.Scribe.Enabled && cfg.Scribe.AutoDiscussRounds > 0 {
			discussResult, discussErr := RunAutoDiscuss(ctx, cfg, scanDir, selected, feedback, adrDir, resolvedStrictness, out, logger)
			if discussErr != nil {
				logger.Warn("Auto-discuss failed (non-fatal): %v", discussErr)
			} else if discussResult != nil {
				archResp := discussResult.ToArchitectResponse()
				if len(discussResult.OpenIssues) > 0 {
					logger.Info("Auto-discuss: %d open issues identified", len(discussResult.OpenIssues))
				}
				scribeResp, scribeErr := RunScribeADR(ctx, cfg, scanDir, selected, archResp, adrDir, resolvedStrictness, out, logger)
				if scribeErr != nil {
					logger.Warn("Auto-discuss ADR generation failed (non-fatal): %v", scribeErr)
				} else {
					logger.OK("Auto-discuss: ADR generated — %s-%s", scribeResp.ADRID, scribeResp.Title)
					*adrCount++
					emitter.EmitGenerateADR(domain.ADRGeneratedPayload{
						ADRID: scribeResp.ADRID, Title: scribeResp.Title,
					}, time.Now().UTC())
				}
			}
		}

		domain.LogBanner(logger, domain.BannerSend, string(DMailSpecification), DMailName("spec", domain.WaveKey(selected)), selected.Title)
		if err := ComposeSpecification(ctx, store, selected); err != nil {
			logger.Warn("D-Mail specification failed (non-fatal): %v", err)
		} else {
			emitter.EmitSendSpecification(selected.ID, selected.ClusterName, time.Now().UTC())
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
			waveSpan.AddEvent("wave.approved",
				trace.WithAttributes(
					attribute.String("wave.id", selected.ID),
					attribute.String("wave.cluster_name", selected.ClusterName),
				),
			)
			emitter.EmitApproveWave(selected.ID, selected.ClusterName, time.Now().UTC())
			domain.LogBanner(logger, domain.BannerSend, string(DMailSpecification), DMailName("spec", domain.WaveKey(selected)), selected.Title)
			if err := ComposeSpecification(ctx, store, selected); err != nil {
				logger.Warn("D-Mail specification failed (non-fatal): %v", err)
			} else {
				emitter.EmitSendSpecification(selected.ID, selected.ClusterName, time.Now().UTC())
			}
			return selected, approvalApproved
		case domain.ApprovalReject:
			delete(sessionRejected, domain.WaveKey(selected))
			waveSpan.AddEvent("wave.rejected",
				trace.WithAttributes(
					attribute.String("wave.id", selected.ID),
					attribute.String("wave.cluster_name", selected.ClusterName),
				),
			)
			emitter.EmitRejectWave(selected.ID, selected.ClusterName, time.Now().UTC())
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
				emitter.EmitModifyWave(domain.WaveModifiedPayload{
					WaveID: selected.ID, ClusterName: selected.ClusterName,
					UpdatedWave: domain.WaveState{
						ID: selected.ID, ClusterName: selected.ClusterName,
						Title: selected.Title, Status: selected.Status,
						Prerequisites: selected.Prerequisites,
						ActionCount:   len(selected.Actions), Actions: selected.Actions,
						Description: selected.Description, Delta: selected.Delta,
					},
				}, time.Now().UTC())
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
						emitter.EmitGenerateADR(domain.ADRGeneratedPayload{
							ADRID: scribeResp.ADRID, Title: scribeResp.Title,
						}, time.Now().UTC())
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
			emitter.EmitApproveWave(selected.ID, selected.ClusterName, time.Now().UTC())
			domain.LogBanner(logger, domain.BannerSend, string(DMailSpecification), DMailName("spec", domain.WaveKey(selected)), selected.Title)
			if err := ComposeSpecification(ctx, store, selected); err != nil {
				logger.Warn("D-Mail specification failed (non-fatal): %v", err)
			} else {
				emitter.EmitSendSpecification(selected.ID, selected.ClusterName, time.Now().UTC())
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
	emitter port.SessionEventEmitter, out io.Writer, waveSpan trace.Span, logger domain.Logger) (*domain.WaveApplyResult, int, bool) {

	applyResult, err := RunWaveApply(ctx, cfg, scanDir, selected, resolvedStrictness, out, logger)
	if err != nil {
		logger.Error("Apply failed: %v", err)
		return nil, 0, false
	}

	oldAvailable := len(domain.AvailableWaves(*waves, completed))

	emitter.EmitApplyWave(domain.WaveAppliedPayload{
		WaveID: selected.ID, ClusterName: selected.ClusterName,
		Applied: applyResult.Applied, TotalCount: applyResult.TotalCount,
		Errors: applyResult.Errors,
	}, time.Now().UTC())
	platform.RecordWave(ctx, "applied")

	if !domain.IsWaveApplyComplete(applyResult) {
		waveSpan.AddEvent("wave.partial_failure",
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
	fbCollector *FeedbackCollector, emitter port.SessionEventEmitter, waveSpan trace.Span, logger domain.Logger) {

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
		waveSpan.AddEvent("nextgen.skipped",
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
		emitter.EmitAddNextGenWaves(domain.NextGenWavesAddedPayload{
			ClusterName: selected.ClusterName,
			Waves:       domain.BuildWaveStates(newWaves),
		}, time.Now().UTC())
	}
}

// applyReadyLabelsIfEnabled applies "ready" labels to issues that are newly
// ready (all prerequisite waves completed). Only labels issues not yet tracked
// in labeledReady to avoid redundant API calls.
func applyReadyLabelsIfEnabled(ctx context.Context, cfg *domain.Config,
	waves *[]domain.Wave, completed map[string]bool,
	labeledReady map[string]bool, emitter port.SessionEventEmitter, out io.Writer, logger domain.Logger) {

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
	emitter.EmitApplyReadyLabels(domain.ReadyLabelsAppliedPayload{
		IssueIDs: newlyReady,
	}, time.Now().UTC())
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
	fbCollector *FeedbackCollector, store port.OutboxStore, emitter port.SessionEventEmitter, out io.Writer, waveSpan trace.Span, logger domain.Logger) {

	gate := cfg.Gate

	applyResult, oldAvailable, ok := executeAndRecordApply(ctx, cfg, scanDir, selected, resolvedStrictness, waves, completed, emitter, out, waveSpan, logger)
	if !ok {
		return
	}

	waveSpan.AddEvent("wave.completed",
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

	emitter.EmitCompleteWave(domain.WaveCompletedPayload{
		WaveID: selected.ID, ClusterName: selected.ClusterName,
		Applied: applyResult.Applied, TotalCount: applyResult.TotalCount,
	}, time.Now().UTC())

	// Review gate: run review before composing report (outbox is read immediately by phonewave)
	if gate.HasReviewCmd() {
		_, reviewSpan := platform.Tracer.Start(ctx, "wave.review", // nosemgrep: adr0003-otel-span-without-defer-end -- End() called per branch [permanent]
			trace.WithAttributes(
				attribute.String("wave.id", selected.ID),
				attribute.String("wave.cluster_name", selected.ClusterName),
			),
		)
		passed, reviewErr := RunReviewGate(ctx, gate, cfg, scanDir, logger)
		if reviewErr != nil {
			reviewSpan.SetAttributes(attribute.String("review.error", reviewErr.Error()))
			logger.Warn("Review gate error (non-fatal): %v", reviewErr)
		}
		reviewSpan.SetAttributes(attribute.Bool("review.passed", passed))
		reviewSpan.End()
		if !passed {
			logger.Warn("Review gate: not passed — skipping ComposeReport for wave %s", domain.WaveKey(selected))
			return
		}
	}

	// Compose report d-mail for the completed wave
	domain.LogBanner(logger, domain.BannerSend, string(DMailReport), DMailName("report", domain.WaveKey(selected)), selected.Title)
	if err := ComposeReport(ctx, store, selected, applyResult); err != nil {
		logger.Warn("D-Mail report failed (non-fatal): %v", err)
	} else {
		emitter.EmitSendReport(selected.ID, selected.ClusterName, time.Now().UTC())
	}

	// O2: sightjack → amadeus feedback D-Mail
	domain.LogBanner(logger, domain.BannerSend, string(DMailReport), DMailName("feedback", domain.WaveKey(selected)), fmt.Sprintf("Wave %s report for amadeus", domain.WaveKey(selected)))
	if feedbackErr := ComposeFeedback(ctx, store, selected, applyResult); feedbackErr != nil {
		logger.Warn("D-Mail feedback failed (non-fatal): %v", feedbackErr)
	} else {
		emitter.EmitSendFeedback(selected.ID, selected.ClusterName, time.Now().UTC())
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
	emitter.EmitUpdateCompleteness(selected.ClusterName, updatedClusterCompleteness, scanResult.Completeness, time.Now().UTC())

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
		emitter.EmitUnlockWaves(unlockedIDs, time.Now().UTC())
	}
	DisplayWaveCompletion(out, selected, applyResult.Ripples, scanResult.Completeness, newCount)

	generateNextWavesIfNeeded(ctx, cfg, scanDir, adrDir, selected, resolvedStrictness,
		waves, completed, scanResult, sessionRejected, fbCollector, emitter, waveSpan, logger)

	applyReadyLabelsIfEnabled(ctx, cfg, waves, completed, labeledReady, emitter, out, logger)

	// Save scan result cache (crash resilience)
	if err := WriteScanResult(scanResultPath, scanResult); err != nil {
		logger.Warn("Failed to update cached scan result: %v", err)
	}
}
