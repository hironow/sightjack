package usecase

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// RunScan validates the RunScanCommand, then delegates to session.RunScan.
// The sessionID is generated internally and returned for downstream use.
func RunScan(ctx context.Context, cmd domain.RunScanCommand, cfg *domain.Config, baseDir string, dryRun bool, streamOut io.Writer, logger domain.Logger) (*domain.ScanResult, string, error) {
	if errs := cmd.Validate(); len(errs) > 0 {
		return nil, "", fmt.Errorf("command validation: %w", errs[0])
	}
	sessionID := fmt.Sprintf("scan-%d-%d", time.Now().UnixMilli(), os.Getpid())
	result, err := session.RunScan(ctx, cfg, baseDir, sessionID, dryRun, streamOut, logger)
	return result, sessionID, err
}

// RecordScanEvents caches the scan result and records session events via SessionAggregate.
// The aggregate generates event content; the recorder handles metadata and persistence.
func RecordScanEvents(baseDir, sessionID string, result *domain.ScanResult, cfg *domain.Config, logger domain.Logger) {
	// Cache scan result for pipe replay
	scanResultPath := filepath.Join(domain.ScanDir(baseDir, sessionID), "scan_result.json")
	if err := session.WriteScanResult(scanResultPath, result); err != nil {
		logger.Warn("Failed to cache scan result: %v", err)
	}

	// Build cluster state for event payload
	clusters := make([]domain.ClusterState, 0, len(result.Clusters))
	for _, c := range result.Clusters {
		clusters = append(clusters, domain.ClusterState{
			Name:         c.Name,
			Completeness: c.Completeness,
			IssueCount:   len(c.Issues),
		})
	}

	// Record events via aggregate → recorder pipeline
	store := session.NewEventStore(baseDir, sessionID)
	recorder, recErr := session.NewSessionRecorder(store, sessionID)
	if recErr != nil {
		logger.Warn("session recorder: %v", recErr)
		return
	}

	agg := domain.NewSessionAggregate()
	now := time.Now()

	// Aggregate generates event content (type + payload)
	startEvt, err := agg.Start(cfg.Linear.Project, string(cfg.Strictness.Default), now) // nosemgrep: adr0003-otel-span-without-defer-end -- SessionAggregate.Start, not tracer.Start
	if err != nil {
		logger.Warn("aggregate start: %v", err)
		return
	}
	// Recorder handles SessionID/CorrelationID/CausationID and persistence
	if err := recorder.Record(startEvt.Type, startEvt.Data); err != nil {
		logger.Warn("Failed to record session start: %v", err)
	}

	scanPayload := domain.ScanCompletedPayload{
		Clusters:       clusters,
		Completeness:   result.Completeness,
		ShibitoCount:   len(result.ShibitoWarnings),
		ScanResultPath: domain.RelativeScanResultPath(baseDir, scanResultPath),
		LastScanned:    now,
	}
	scanEvt, err := agg.RecordScan(scanPayload, now)
	if err != nil {
		logger.Warn("aggregate scan: %v", err)
		return
	}
	if err := recorder.Record(scanEvt.Type, scanEvt.Data); err != nil {
		logger.Warn("Failed to record scan completed: %v", err)
	} else {
		logger.OK("Events saved to %s", session.EventStorePath(baseDir, sessionID))
	}
}
