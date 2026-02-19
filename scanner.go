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
// shibitoWarnings are propagated from the Pass 1 classify result.
func MergeScanResults(clusters []ClusterScanResult, shibitoWarnings []ShibitoWarning, scanWarnings []string) ScanResult {
	result := ScanResult{Clusters: clusters, ShibitoWarnings: shibitoWarnings, ScanWarnings: scanWarnings}
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
		TeamFilter:      cfg.Linear.Team,
		ProjectFilter:   cfg.Linear.Project,
		CycleFilter:     cfg.Linear.Cycle,
		OutputPath:      classifyOutput,
		StrictnessLevel: string(cfg.Strictness.Default),
		LabelsEnabled:   cfg.Labels.Enabled,
		LabelPrefix:     cfg.Labels.Prefix,
	})
	if err != nil {
		return nil, fmt.Errorf("render classify prompt: %w", err)
	}

	if dryRun {
		return nil, RunClaudeDryRun(cfg, classifyPrompt, scanDir, "classify")
	}

	// Use RunClaudeOnce when labels are enabled because classify applies
	// side-effects (:analyzed labels). Retrying could duplicate label mutations.
	if cfg.Labels.Enabled {
		if _, err := RunClaudeOnce(ctx, cfg, classifyPrompt, os.Stdout); err != nil {
			return nil, fmt.Errorf("classify scan: %w", err)
		}
	} else {
		if _, err := RunClaude(ctx, cfg, classifyPrompt, os.Stdout); err != nil {
			return nil, fmt.Errorf("classify scan: %w", err)
		}
	}

	classify, err := ParseClassifyResult(classifyOutput)
	if err != nil {
		return nil, err
	}
	LogOK("Found %d clusters with %d total issues", len(classify.Clusters), classify.TotalIssues)

	// --- Pass 2: Deep scan per cluster (parallel) ---
	LogScan("Pass 2: Deep scanning %d clusters...", len(classify.Clusters))

	// Build scan cluster list from classify results. The index parameter in
	// DeepScanFunc maps directly to classify.Clusters, so duplicate cluster
	// names are handled safely without a name-keyed map.
	scanClusters := make([]ClusterScanResult, len(classify.Clusters))
	for i, cc := range classify.Clusters {
		scanClusters[i] = ClusterScanResult{Name: cc.Name}
	}

	deepScanFn := func(ctx context.Context, cfg *Config, scanDir string, index int, cluster ClusterScanResult) (ClusterScanResult, error) {
		cc := classify.Clusters[index]
		chunks := chunkSlice(cc.IssueIDs, cfg.Scan.ChunkSize)
		var chunkResults []ClusterScanResult

		for j, chunk := range chunks {
			chunkFile := filepath.Join(scanDir, fmt.Sprintf("cluster_%02d_%s_c%02d.json", index, sanitizeName(cc.Name), j))
			prompt, renderErr := RenderDeepScanPrompt(cfg.Lang, DeepScanPromptData{
				ClusterName:     cc.Name,
				IssueIDs:        strings.Join(chunk, ", "),
				OutputPath:      chunkFile,
				StrictnessLevel: string(cfg.Strictness.Default),
			})
			if renderErr != nil {
				return ClusterScanResult{}, fmt.Errorf("render deepscan prompt for %s chunk %d: %w", cc.Name, j, renderErr)
			}

			LogScan("Scanning cluster: %s (%d/%d issues, chunk %d/%d)", cc.Name, len(chunk), len(cc.IssueIDs), j+1, len(chunks))
			if _, runErr := RunClaude(ctx, cfg, prompt, io.Discard); runErr != nil {
				return ClusterScanResult{}, fmt.Errorf("deepscan %s chunk %d: %w", cc.Name, j, runErr)
			}

			result, parseErr := ParseClusterScanResult(chunkFile)
			if parseErr != nil {
				return ClusterScanResult{}, fmt.Errorf("parse %s chunk %d: %w", cc.Name, j, parseErr)
			}
			chunkResults = append(chunkResults, *result)
		}

		merged := mergeClusterChunks(cc.Name, chunkResults)
		LogOK("Cluster %s: %.0f%% complete", cc.Name, merged.Completeness*100)
		return merged, nil
	}

	clusters, scanWarnings := RunParallelDeepScan(ctx, cfg, scanDir, scanClusters, deepScanFn)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if len(clusters) == 0 && len(scanWarnings) > 0 {
		return nil, fmt.Errorf("all clusters failed during deep scan: %v", scanWarnings)
	}

	merged := MergeScanResults(clusters, classify.ShibitoWarnings, scanWarnings)
	return &merged, nil
}

// RunWaveGenerate executes Pass 3: generate waves for each cluster in parallel.
func RunWaveGenerate(ctx context.Context, cfg *Config, scanDir string, clusters []ClusterScanResult, dryRun bool) ([]Wave, error) {
	LogScan("Pass 3: Generating waves for %d clusters...", len(clusters))

	waveResults := make([]WaveGenerateResult, len(clusters))

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.Scan.MaxConcurrency)

	for i, cluster := range clusters {
		g.Go(func() error {
			waveFile := filepath.Join(scanDir, fmt.Sprintf("wave_%02d_%s.json", i, sanitizeName(cluster.Name)))

			issuesJSON, err := json.Marshal(cluster.Issues)
			if err != nil {
				return fmt.Errorf("marshal issues for %s: %w", cluster.Name, err)
			}

			var dodSection string
			if cfg.DoDTemplates != nil {
				if matched, key := MatchDoDTemplate(cfg.DoDTemplates, cluster.Name); matched {
					dodSection = FormatDoDSection(cfg.DoDTemplates[key])
				}
			}

			prompt, err := RenderWaveGeneratePrompt(cfg.Lang, WaveGeneratePromptData{
				ClusterName:     cluster.Name,
				Completeness:    fmt.Sprintf("%.0f", cluster.Completeness*100),
				Issues:          string(issuesJSON),
				Observations:    strings.Join(cluster.Observations, "\n"),
				DoDSection:      dodSection,
				OutputPath:      waveFile,
				StrictnessLevel: string(cfg.Strictness.Default),
			})
			if err != nil {
				return fmt.Errorf("render wave prompt for %s: %w", cluster.Name, err)
			}

			if dryRun {
				dryRunName := fmt.Sprintf("wave_%02d_%s", i, sanitizeName(cluster.Name))
				return RunClaudeDryRun(cfg, prompt, scanDir, dryRunName)
			}

			LogScan("Generating waves: %s", cluster.Name)
			if _, err := RunClaude(gCtx, cfg, prompt, io.Discard); err != nil {
				return fmt.Errorf("wave generate %s: %w", cluster.Name, err)
			}

			result, err := ParseWaveGenerateResult(waveFile)
			if err != nil {
				return fmt.Errorf("parse waves %s: %w", cluster.Name, err)
			}
			waveResults[i] = *result
			LogOK("Cluster %s: %d waves generated", cluster.Name, len(result.Waves))
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return MergeWaveResults(waveResults), nil
}

// DeepScanFunc is the function signature for scanning a single cluster.
// The index parameter identifies the cluster's position in the original slice,
// enabling safe lookup even when duplicate cluster names exist.
type DeepScanFunc func(ctx context.Context, cfg *Config, scanDir string, index int, cluster ClusterScanResult) (ClusterScanResult, error)

// RunParallelDeepScan executes deep scan across clusters using goroutines with
// semaphore-based concurrency control. Failed clusters get LogWarn and are skipped;
// remaining clusters continue. Returns successful results and warning messages.
func RunParallelDeepScan(ctx context.Context, cfg *Config, scanDir string,
	clusters []ClusterScanResult, scanFn DeepScanFunc) ([]ClusterScanResult, []string) {

	maxConcurrency := cfg.Scan.MaxConcurrency
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}

	type scanResult struct {
		index   int
		cluster ClusterScanResult
		err     error
	}

	sem := make(chan struct{}, maxConcurrency)
	results := make(chan scanResult, len(clusters))

	// NOTE: i and cluster are safe to capture in the goroutine closure.
	// Go 1.22+ scopes loop variables per iteration (go.mod requires go 1.25.0).
	launched := 0
	for i, cluster := range clusters {
		if ctx.Err() != nil {
			break
		}
		// Use select to respect cancellation while waiting for semaphore.
		acquired := false
		select {
		case <-ctx.Done():
		case sem <- struct{}{}:
			acquired = true
		}
		if !acquired || ctx.Err() != nil {
			if acquired {
				<-sem // release the slot we just took
			}
			break
		}
		launched++
		go func() {
			defer func() { <-sem }()
			result, err := scanFn(ctx, cfg, scanDir, i, cluster)
			results <- scanResult{index: i, cluster: result, err: err}
		}()
	}

	var successful []ClusterScanResult
	var warnings []string
	for range launched {
		r := <-results
		if r.err != nil {
			msg := fmt.Sprintf("Cluster %q scan failed: %v", clusters[r.index].Name, r.err)
			LogWarn("%s", msg)
			warnings = append(warnings, msg)
			continue
		}
		successful = append(successful, r.cluster)
	}

	return successful, warnings
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
