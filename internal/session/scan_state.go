package session

import (
	"path/filepath"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// RecordScanState caches the scan result and records session start + scan
// completed events via the SessionAggregate. This is the single authoritative
// path for scan state persistence — both the scan command and the run session
// converge here.
//
// Returns the cached scan result path for downstream use (e.g. interactive loop).
func RecordScanState(baseDir, sessionID string, result *domain.ScanResult, cfg *domain.Config, emitter port.SessionEventEmitter, scanTime time.Time, logger domain.Logger) string {
	scanDir := domain.ScanDir(baseDir, sessionID)
	scanResultPath := filepath.Join(scanDir, "scan_result.json")
	if err := WriteScanResult(scanResultPath, result); err != nil {
		logger.Warn("Failed to cache scan result: %v", err)
	}

	clusters := make([]domain.ClusterState, 0, len(result.Clusters))
	for _, c := range result.Clusters {
		clusters = append(clusters, domain.ClusterState{
			Name:         c.Name,
			Completeness: c.Completeness,
			IssueCount:   len(c.Issues),
		})
	}

	if err := emitter.EmitStart(cfg.Tracker.Project, string(cfg.Strictness.Default), scanTime); err != nil { // nosemgrep: adr0003-otel-span-without-defer-end — SessionEventEmitter.EmitStart, not tracer.Start
		logger.Warn("emit start: %v", err)
		return scanResultPath
	}

	scanPayload := domain.ScanCompletedPayload{
		Clusters:       clusters,
		Completeness:   result.Completeness,
		ShibitoCount:   len(result.ShibitoWarnings),
		ScanResultPath: domain.RelativeScanResultPath(baseDir, scanResultPath),
		LastScanned:    scanTime,
	}
	if err := emitter.EmitRecordScan(scanPayload, scanTime); err != nil {
		logger.Warn("emit scan: %v", err)
		return scanResultPath
	}
	logger.OK("Events saved to %s", EventStorePath(baseDir, sessionID))

	return scanResultPath
}
