package sightjack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sync/errgroup"
)

// ParseClassifyResult reads and parses the classify.json output file.
func ParseClassifyResult(path string) (*ClassifyResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read classify result: %w", err)
	}
	var result ClassifyResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse classify result: %w", err)
	}
	return &result, nil
}

// ParseClusterScanResult reads and parses a cluster_{name}.json output file.
func ParseClusterScanResult(path string) (*ClusterScanResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cluster result: %w", err)
	}
	var result ClusterScanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse cluster result: %w", err)
	}
	return &result, nil
}

// MergeScanResults combines per-cluster deep scan results into a single ScanResult.
func MergeScanResults(clusters []ClusterScanResult) ScanResult {
	result := ScanResult{Clusters: clusters}
	result.CalculateCompleteness()

	for _, c := range clusters {
		result.Observations = append(result.Observations, c.Observations...)
	}
	return result
}

// RunScan executes the full two-pass scan.
// Pass 1: Classify all issues into clusters.
// Pass 2: Deep scan each cluster in parallel.
func RunScan(ctx context.Context, cfg *Config, baseDir string, sessionID string, dryRun bool) (*ScanResult, error) {
	scanDir, err := EnsureScanDir(baseDir, sessionID)
	if err != nil {
		return nil, err
	}

	// --- Pass 1: Classify ---
	LogScan("Pass 1: Classifying issues...")
	classifyOutput := filepath.Join(scanDir, "classify.json")

	classifyPrompt, err := RenderClassifyPrompt(cfg.Lang, ClassifyPromptData{
		TeamFilter:    cfg.Linear.Team,
		ProjectFilter: cfg.Linear.Project,
		CycleFilter:   cfg.Linear.Cycle,
		OutputPath:    classifyOutput,
	})
	if err != nil {
		return nil, fmt.Errorf("render classify prompt: %w", err)
	}

	if dryRun {
		return nil, RunClaudeDryRun(cfg, classifyPrompt, scanDir)
	}

	if _, err := RunClaude(ctx, cfg, classifyPrompt, os.Stdout); err != nil {
		return nil, fmt.Errorf("classify scan: %w", err)
	}

	classify, err := ParseClassifyResult(classifyOutput)
	if err != nil {
		return nil, err
	}
	LogOK("Found %d clusters with %d total issues", len(classify.Clusters), classify.TotalIssues)

	// --- Pass 2: Deep scan per cluster (parallel) ---
	LogScan("Pass 2: Deep scanning %d clusters...", len(classify.Clusters))

	clusters := make([]ClusterScanResult, len(classify.Clusters))

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.Scan.MaxConcurrency)

	for i, cc := range classify.Clusters {
		g.Go(func() error {
			chunks := chunkSlice(cc.IssueIDs, cfg.Scan.ChunkSize)
			var chunkResults []ClusterScanResult

			for j, chunk := range chunks {
				chunkFile := filepath.Join(scanDir, fmt.Sprintf("cluster_%02d_%s_c%02d.json", i, sanitizeName(cc.Name), j))
				prompt, renderErr := RenderDeepScanPrompt(cfg.Lang, DeepScanPromptData{
					ClusterName: cc.Name,
					IssueIDs:    strings.Join(chunk, ", "),
					OutputPath:  chunkFile,
				})
				if renderErr != nil {
					return fmt.Errorf("render deepscan prompt for %s chunk %d: %w", cc.Name, j, renderErr)
				}

				LogScan("Scanning cluster: %s (%d/%d issues, chunk %d/%d)", cc.Name, len(chunk), len(cc.IssueIDs), j+1, len(chunks))
				if _, runErr := RunClaude(gCtx, cfg, prompt, io.Discard); runErr != nil {
					return fmt.Errorf("deepscan %s chunk %d: %w", cc.Name, j, runErr)
				}

				result, parseErr := ParseClusterScanResult(chunkFile)
				if parseErr != nil {
					return fmt.Errorf("parse %s chunk %d: %w", cc.Name, j, parseErr)
				}
				chunkResults = append(chunkResults, *result)
			}

			clusters[i] = mergeClusterChunks(cc.Name, chunkResults)
			LogOK("Cluster %s: %.0f%% complete", cc.Name, clusters[i].Completeness*100)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	merged := MergeScanResults(clusters)
	return &merged, nil
}

// chunkSlice splits items into sub-slices of at most size elements.
func chunkSlice(items []string, size int) [][]string {
	if len(items) == 0 {
		return nil
	}
	if size <= 0 {
		return [][]string{items}
	}
	var chunks [][]string
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks
}

// mergeClusterChunks combines multiple chunk results from the same cluster
// into a single ClusterScanResult, recalculating completeness from individual issues.
func mergeClusterChunks(name string, chunks []ClusterScanResult) ClusterScanResult {
	merged := ClusterScanResult{Name: name}
	for _, c := range chunks {
		merged.Issues = append(merged.Issues, c.Issues...)
		merged.Observations = append(merged.Observations, c.Observations...)
	}
	if len(merged.Issues) > 0 {
		total := 0.0
		for _, issue := range merged.Issues {
			total += issue.Completeness
		}
		merged.Completeness = total / float64(len(merged.Issues))
	}
	return merged
}

// clusterFileName returns a collision-safe filename for a cluster scan result.
// The index prefix ensures uniqueness even when distinct names sanitize to the same string.
func clusterFileName(index int, name string) string {
	return fmt.Sprintf("cluster_%02d_%s.json", index, sanitizeName(name))
}

// sanitizeName converts a cluster name to a safe filename component.
// Only ASCII alphanumeric, hyphen, and underscore are kept; everything else becomes underscore.
func sanitizeName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
