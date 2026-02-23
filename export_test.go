package sightjack

import (
	"context"
	"os/exec"
)

// Test-only exports for external test packages (package sightjack_test).
// These are compiled only during testing — they do not appear in production builds.

var (
	ExportSanitizeName             = sanitizeName
	ExportClusterFileName          = clusterFileName
	ExportChunkSlice               = chunkSlice
	ExportMergeClusterChunks       = mergeClusterChunks
	ExportDetectFailedClusterNames = detectFailedClusterNames
	ExportGenerateWaveForCluster   = generateWaveForCluster
)

// SetNewCmd replaces the command constructor for testing and returns a cleanup function.
func SetNewCmd(fn func(ctx context.Context, name string, args ...string) *exec.Cmd) func() {
	old := newCmd
	newCmd = fn
	return func() { newCmd = old }
}
