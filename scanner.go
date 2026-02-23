package sightjack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
func RunScan(ctx context.Context, cfg *Config, baseDir string, sessionID string, dryRun bool, out io.Writer, logger *Logger) (*ScanResult, error) {
	if logger == nil {
		logger = NewLogger(nil, false)
	}
	ctx, scanSpan := tracer.Start(ctx, "scan",
		trace.WithAttributes(attribute.String("sightjack.session_id", sessionID)),
	)
	defer scanSpan.End()

	scanDir, err := EnsureScanDir(baseDir, sessionID)
	if err != nil {
		return nil, err
	}

	// --- Pass 1: Classify ---
	logger.Scan("Pass 1: Classifying issues...")
	classifyCtx, classifySpan := tracer.Start(ctx, "classify")
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
		classifySpan.End()
		return nil, fmt.Errorf("render classify prompt: %w", err)
	}

	if dryRun {
		classifySpan.End()
		return nil, RunClaudeDryRun(cfg, classifyPrompt, scanDir, "classify", logger)
	}

	// Save prompt for debugging (survives signal:killed).
	promptFile := filepath.Join(scanDir, "classify_prompt.md")
	if err := os.WriteFile(promptFile, []byte(classifyPrompt), 0644); err != nil {
		logger.Warn("save classify prompt: %v", err)
	}

	// Tee claude output to a log file for incremental visibility.
	logFile, logErr := os.Create(filepath.Join(scanDir, "classify_output.log"))
	claudeOut := out
	if logErr == nil {
		defer logFile.Close()
		claudeOut = io.MultiWriter(out, logFile)
	} else {
		logger.Warn("create classify log: %v", logErr)
	}

	// Use RunClaudeOnce when labels are enabled because classify applies
	// side-effects (:analyzed labels). Retrying could duplicate label mutations.
	linearTools := WithAllowedTools(LinearMCPAllowedTools...)
	if cfg.Labels.Enabled {
		if _, err := RunClaudeOnce(classifyCtx, cfg, classifyPrompt, claudeOut, logger, linearTools); err != nil {
			classifySpan.End()
			return nil, fmt.Errorf("classify scan: %w", err)
		}
	} else {
		if _, err := RunClaude(classifyCtx, cfg, classifyPrompt, claudeOut, logger, linearTools); err != nil {
			classifySpan.End()
			return nil, fmt.Errorf("classify scan: %w", err)
		}
	}

	if normErr := normalizeJSONFile(classifyOutput); normErr != nil {
		logger.Warn("normalize classify JSON: %v", normErr)
	}
	classify, err := ParseClassifyResult(classifyOutput)
	if err != nil {
		classifySpan.End()
		return nil, err
	}
	logger.OK("Found %d clusters with %d total issues", len(classify.Clusters), classify.TotalIssues)
	classifySpan.End()

	scanSpan.SetAttributes(
		attribute.Int("scan.cluster_count", len(classify.Clusters)),
		attribute.Int("scan.total_issues", classify.TotalIssues),
	)

	// --- Pass 2: Deep scan per cluster (parallel) ---
	deepscanCtx, deepscanSpan := tracer.Start(ctx, "deepscan")
	logger.Scan("Pass 2: Deep scanning %d clusters...", len(classify.Clusters))

	// Build scan cluster list from classify results. The index parameter in
	// DeepScanFunc maps directly to classify.Clusters, so duplicate cluster
	// names are handled safely without a name-keyed map.
	scanClusters := make([]ClusterScanResult, len(classify.Clusters))
	for i, cc := range classify.Clusters {
		scanClusters[i] = ClusterScanResult{Name: cc.Name, Labels: cc.Labels}
	}

	deepScanFn := func(ctx context.Context, cfg *Config, scanDir string, index int, cluster ClusterScanResult) (ClusterScanResult, error) {
		ctx, clusterSpan := tracer.Start(ctx, "deepscan.cluster",
			trace.WithAttributes(attribute.String("cluster.name", cluster.Name)),
		)
		defer clusterSpan.End()

		cc := classify.Clusters[index]
		chunks := chunkSlice(cc.IssueIDs, cfg.Scan.ChunkSize)
		var chunkResults []ClusterScanResult

		for j, chunk := range chunks {
			chunkFile := filepath.Join(scanDir, fmt.Sprintf("cluster_%02d_%s_c%02d.json", index, sanitizeName(cc.Name), j))
			prompt, renderErr := RenderDeepScanPrompt(cfg.Lang, DeepScanPromptData{
				ClusterName:     cc.Name,
				IssueIDs:        strings.Join(chunk, ", "),
				OutputPath:      chunkFile,
				StrictnessLevel: string(ResolveStrictness(cfg.Strictness, append([]string{cc.Name}, cc.Labels...))),
			})
			if renderErr != nil {
				return ClusterScanResult{}, fmt.Errorf("render deepscan prompt for %s chunk %d: %w", cc.Name, j, renderErr)
			}

			// Save prompt + tee output for debugging.
			promptBase := fmt.Sprintf("cluster_%02d_%s_c%02d", index, sanitizeName(cc.Name), j)
			if err := os.WriteFile(filepath.Join(scanDir, promptBase+"_prompt.md"), []byte(prompt), 0644); err != nil {
				logger.Warn("save deepscan prompt: %v", err)
			}
			chunkLog, chunkLogErr := os.Create(filepath.Join(scanDir, promptBase+"_output.log"))
			chunkOut := io.Writer(io.Discard)
			if chunkLogErr == nil {
				chunkOut = chunkLog
			} else {
				logger.Warn("create deepscan log: %v", chunkLogErr)
			}

			logger.Scan("Scanning cluster: %s (%d/%d issues, chunk %d/%d)", cc.Name, len(chunk), len(cc.IssueIDs), j+1, len(chunks))
			_, runErr := RunClaude(ctx, cfg, prompt, chunkOut, logger, linearTools)
			if chunkLog != nil {
				chunkLog.Close()
			}
			if runErr != nil {
				return ClusterScanResult{}, fmt.Errorf("deepscan %s chunk %d: %w", cc.Name, j, runErr)
			}

			if normErr := normalizeJSONFile(chunkFile); normErr != nil {
				logger.Warn("normalize cluster JSON: %v", normErr)
			}
			result, parseErr := ParseClusterScanResult(chunkFile)
			if parseErr != nil {
				return ClusterScanResult{}, fmt.Errorf("parse %s chunk %d: %w", cc.Name, j, parseErr)
			}
			chunkResults = append(chunkResults, *result)
		}

		merged := mergeClusterChunks(cc.Name, chunkResults)
		merged.Labels = cc.Labels
		logger.OK("Cluster %s: %.0f%% complete", cc.Name, merged.Completeness*100)
		return merged, nil
	}

	clusters, scanWarnings := RunParallelDeepScan(deepscanCtx, cfg, scanDir, scanClusters, deepScanFn, logger)
	deepscanSpan.End()
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
// Failed clusters are skipped with warnings (partial success), matching the
// fault-tolerance pattern of RunParallelDeepScan. Returns an error only when
// ALL clusters fail.
func RunWaveGenerate(ctx context.Context, cfg *Config, scanDir string, clusters []ClusterScanResult, dryRun bool, logger *Logger) ([]Wave, []string, map[string]bool, error) {
	ctx, waveGenSpan := tracer.Start(ctx, "wave.generate",
		trace.WithAttributes(attribute.Int("scan.cluster_count", len(clusters))),
	)
	defer waveGenSpan.End()

	logger.Scan("Pass 3: Generating waves for %d clusters...", len(clusters))

	linearTools := WithAllowedTools(LinearMCPAllowedTools...)

	successResults, warnings := RunParallel(ctx, clusters, cfg.Scan.MaxConcurrency,
		func(ctx context.Context, index int, cluster ClusterScanResult) (WaveGenerateResult, error) {
			return generateWaveForCluster(ctx, cfg, scanDir, index, cluster, dryRun, linearTools, logger)
		},
		func(c ClusterScanResult) string { return c.Name },
		logger)

	// Context cancellation must be surfaced even when some clusters
	// succeeded, so interrupted runs are never treated as complete.
	if ctx.Err() != nil {
		return nil, warnings, nil, ctx.Err()
	}

	failedNames := detectFailedClusterNames(clusters, successResults)

	if len(successResults) == 0 && len(clusters) > 0 {
		return nil, warnings, failedNames, fmt.Errorf("all %d clusters failed wave generation", len(clusters))
	}

	return MergeWaveResults(successResults), warnings, failedNames, nil
}

// detectFailedClusterNames compares input cluster counts to success counts
// and returns names where at least one instance failed wave generation.
// With duplicate cluster names, a name is marked failed if fewer instances
// succeeded than existed in the input.
func detectFailedClusterNames(clusters []ClusterScanResult, successes []WaveGenerateResult) map[string]bool {
	inputCount := make(map[string]int, len(clusters))
	for _, c := range clusters {
		inputCount[c.Name]++
	}
	successCount := make(map[string]int, len(successes))
	for _, r := range successes {
		successCount[r.ClusterName]++
	}
	failed := make(map[string]bool)
	for name, total := range inputCount {
		if successCount[name] < total {
			failed[name] = true
		}
	}
	return failed
}

// generateWaveForCluster generates waves for a single cluster.
func generateWaveForCluster(ctx context.Context, cfg *Config, scanDir string, index int, cluster ClusterScanResult, dryRun bool, linearTools RunOption, logger *Logger) (WaveGenerateResult, error) {
	waveFile := filepath.Join(scanDir, fmt.Sprintf("wave_%02d_%s.json", index, sanitizeName(cluster.Name)))

	issuesJSON, err := json.Marshal(cluster.Issues)
	if err != nil {
		return WaveGenerateResult{}, fmt.Errorf("marshal issues for %s: %w", cluster.Name, err)
	}

	dodSection := ResolveDoDSection(cfg.DoDTemplates, cluster.Name)

	prompt, err := RenderWaveGeneratePrompt(cfg.Lang, WaveGeneratePromptData{
		ClusterName:     cluster.Name,
		Completeness:    fmt.Sprintf("%.0f", cluster.Completeness*100),
		Issues:          string(issuesJSON),
		Observations:    strings.Join(cluster.Observations, "\n"),
		DoDSection:      dodSection,
		OutputPath:      waveFile,
		StrictnessLevel: string(ResolveStrictness(cfg.Strictness, append([]string{cluster.Name}, cluster.Labels...))),
	})
	if err != nil {
		return WaveGenerateResult{}, fmt.Errorf("render wave prompt for %s: %w", cluster.Name, err)
	}

	if dryRun {
		dryRunName := fmt.Sprintf("wave_%02d_%s", index, sanitizeName(cluster.Name))
		return WaveGenerateResult{ClusterName: cluster.Name}, RunClaudeDryRun(cfg, prompt, scanDir, dryRunName, logger)
	}

	// Save prompt + tee output for debugging (consistent with deep scan).
	promptBase := fmt.Sprintf("wave_%02d_%s", index, sanitizeName(cluster.Name))
	if err := os.WriteFile(filepath.Join(scanDir, promptBase+"_prompt.md"), []byte(prompt), 0644); err != nil {
		logger.Warn("save wave prompt: %v", err)
	}
	waveLog, waveLogErr := os.Create(filepath.Join(scanDir, promptBase+"_output.log"))
	waveOut := io.Writer(io.Discard)
	if waveLogErr == nil {
		defer waveLog.Close()
		waveOut = waveLog
	} else {
		logger.Warn("create wave log: %v", waveLogErr)
	}

	logger.Scan("Generating waves: %s", cluster.Name)
	if _, err := RunClaude(ctx, cfg, prompt, waveOut, logger, linearTools); err != nil {
		return WaveGenerateResult{}, fmt.Errorf("wave generate %s: %w", cluster.Name, err)
	}

	if normErr := normalizeJSONFile(waveFile); normErr != nil {
		logger.Warn("normalize wave JSON: %v", normErr)
	}
	result, err := ParseWaveGenerateResult(waveFile)
	if err != nil {
		return WaveGenerateResult{}, fmt.Errorf("parse waves %s: %w", cluster.Name, err)
	}
	// Normalize to input cluster name — model output may omit or mislabel it.
	result.ClusterName = cluster.Name
	logger.OK("Cluster %s: %d waves generated", cluster.Name, len(result.Waves))
	return *result, nil
}

// DeepScanFunc is the function signature for scanning a single cluster.
// The index parameter identifies the cluster's position in the original slice,
// enabling safe lookup even when duplicate cluster names exist.
type DeepScanFunc func(ctx context.Context, cfg *Config, scanDir string, index int, cluster ClusterScanResult) (ClusterScanResult, error)

// RunParallelDeepScan executes deep scan across clusters with bounded concurrency.
// Delegates to RunParallel for pond-based parallel orchestration.
// Failed clusters produce warnings and are skipped; successful results preserve order.
func RunParallelDeepScan(ctx context.Context, cfg *Config, scanDir string,
	clusters []ClusterScanResult, scanFn DeepScanFunc, logger *Logger) ([]ClusterScanResult, []string) {

	return RunParallel(ctx, clusters, cfg.Scan.MaxConcurrency,
		func(ctx context.Context, index int, cluster ClusterScanResult) (ClusterScanResult, error) {
			return scanFn(ctx, cfg, scanDir, index, cluster)
		},
		func(c ClusterScanResult) string { return c.Name },
		logger)
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
