package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"

	pond "github.com/alitto/pond/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ParseClassifyResult reads and parses the classify.json output file.
func ParseClassifyResult(path string) (*domain.ClassifyResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read classify result: %w", err)
	}
	var result domain.ClassifyResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse classify result: %w", err)
	}
	return &result, nil
}

// ParseClusterScanResult reads and parses a cluster_{name}.json output file.
func ParseClusterScanResult(path string) (*domain.ClusterScanResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cluster result: %w", err)
	}
	var result domain.ClusterScanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse cluster result: %w", err)
	}
	return &result, nil
}

// MergeScanResults combines per-cluster deep scan results into a single domain.ScanResult.
// shibitoWarnings are propagated from the Pass 1 classify result.
func MergeScanResults(clusters []domain.ClusterScanResult, shibitoWarnings []domain.ShibitoWarning, scanWarnings []string) domain.ScanResult {
	result := domain.ScanResult{Clusters: clusters, ShibitoWarnings: shibitoWarnings, ScanWarnings: scanWarnings}
	result.CalculateCompleteness()

	for _, c := range clusters {
		result.Observations = append(result.Observations, c.Observations...)
	}
	return result
}

// RunScan executes the full two-pass scan.
// Pass 1: Classify all issues into clusters.
// Pass 2: Deep scan each cluster in parallel.
func RunScan(ctx context.Context, cfg *domain.Config, baseDir string, sessionID string, dryRun bool, out io.Writer, logger *domain.Logger) (*domain.ScanResult, error) {
	if logger == nil {
		logger = domain.NewLogger(nil, false)
	}
	ctx, scanSpan := platform.Tracer.Start(ctx, "scan",
		trace.WithAttributes(attribute.String("sightjack.session_id", sessionID)),
	)
	defer scanSpan.End()

	scanDir, err := EnsureScanDir(baseDir, sessionID)
	if err != nil {
		return nil, err
	}

	// --- Pass 1: Classify ---
	logger.Info("Pass 1: Classifying issues...")
	classifyCtx, classifySpan := platform.Tracer.Start(ctx, "classify")
	classifyOutput := filepath.Join(scanDir, "classify.json")

	classifyPrompt, err := domain.RenderClassifyPrompt(cfg.Lang, domain.ClassifyPromptData{
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
	linearTools := WithAllowedTools(slices.Concat(BaseAllowedTools, GHAllowedTools, LinearMCPAllowedTools)...)
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

	if normErr := NormalizeJSONFile(classifyOutput); normErr != nil {
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
	deepscanCtx, deepscanSpan := platform.Tracer.Start(ctx, "deepscan")
	logger.Info("Pass 2: Deep scanning %d clusters...", len(classify.Clusters))

	// Build scan cluster list from classify results. The index parameter in
	// DeepScanFunc maps directly to classify.Clusters, so duplicate cluster
	// names are handled safely without a name-keyed map.
	scanClusters := make([]domain.ClusterScanResult, len(classify.Clusters))
	for i, cc := range classify.Clusters {
		scanClusters[i] = domain.ClusterScanResult{Name: cc.Name, Labels: cc.Labels}
	}

	deepScanFn := func(ctx context.Context, cfg *domain.Config, scanDir string, index int, cluster domain.ClusterScanResult) (domain.ClusterScanResult, error) {
		ctx, clusterSpan := platform.Tracer.Start(ctx, "deepscan.cluster",
			trace.WithAttributes(attribute.String("cluster.name", cluster.Name)),
		)
		defer clusterSpan.End()

		cc := classify.Clusters[index]
		chunks := domain.ChunkSlice(cc.IssueIDs, cfg.Scan.ChunkSize)
		var chunkResults []domain.ClusterScanResult

		for j, chunk := range chunks {
			chunkFile := filepath.Join(scanDir, fmt.Sprintf("cluster_%02d_%s_c%02d.json", index, domain.SanitizeName(cc.Name), j))
			prompt, renderErr := domain.RenderDeepScanPrompt(cfg.Lang, domain.DeepScanPromptData{
				ClusterName:     cc.Name,
				IssueIDs:        strings.Join(chunk, ", "),
				OutputPath:      chunkFile,
				StrictnessLevel: string(domain.ResolveStrictness(cfg.Strictness, append([]string{cc.Name}, cc.Labels...))),
			})
			if renderErr != nil {
				return domain.ClusterScanResult{}, fmt.Errorf("render deepscan prompt for %s chunk %d: %w", cc.Name, j, renderErr)
			}

			promptBase := fmt.Sprintf("cluster_%02d_%s_c%02d", index, domain.SanitizeName(cc.Name), j)
			chunkOut, closeChunkLog := savePromptAndCreateLog(scanDir, promptBase, prompt, logger)

			logger.Info("Scanning cluster: %s (%d/%d issues, chunk %d/%d)", cc.Name, len(chunk), len(cc.IssueIDs), j+1, len(chunks))
			_, runErr := RunClaude(ctx, cfg, prompt, chunkOut, logger, linearTools)
			closeChunkLog()
			if runErr != nil {
				return domain.ClusterScanResult{}, fmt.Errorf("deepscan %s chunk %d: %w", cc.Name, j, runErr)
			}

			if normErr := NormalizeJSONFile(chunkFile); normErr != nil {
				logger.Warn("normalize cluster JSON: %v", normErr)
			}
			result, parseErr := ParseClusterScanResult(chunkFile)
			if parseErr != nil {
				return domain.ClusterScanResult{}, fmt.Errorf("parse %s chunk %d: %w", cc.Name, j, parseErr)
			}
			chunkResults = append(chunkResults, *result)
		}

		merged := domain.MergeClusterChunks(cc.Name, chunkResults)
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
func RunWaveGenerate(ctx context.Context, cfg *domain.Config, scanDir string, clusters []domain.ClusterScanResult, dryRun bool, logger *domain.Logger) ([]domain.Wave, []string, map[string]bool, error) {
	ctx, waveGenSpan := platform.Tracer.Start(ctx, "wave.generate",
		trace.WithAttributes(attribute.Int("scan.cluster_count", len(clusters))),
	)
	defer waveGenSpan.End()

	logger.Info("Pass 3: Generating waves for %d clusters...", len(clusters))

	linearTools := WithAllowedTools(slices.Concat(BaseAllowedTools, GHAllowedTools, LinearMCPAllowedTools)...)

	successResults, warnings := RunParallel(ctx, clusters, cfg.Scan.MaxConcurrency,
		func(ctx context.Context, index int, cluster domain.ClusterScanResult) (domain.WaveGenerateResult, error) {
			return generateWaveForCluster(ctx, cfg, scanDir, index, cluster, dryRun, linearTools, logger)
		},
		func(c domain.ClusterScanResult) string { return c.Name },
		logger)

	// Context cancellation must be surfaced even when some clusters
	// succeeded, so interrupted runs are never treated as complete.
	if ctx.Err() != nil {
		return nil, warnings, nil, ctx.Err()
	}

	failedNames := domain.DetectFailedClusterNames(clusters, successResults)

	if len(successResults) == 0 && len(clusters) > 0 {
		return nil, warnings, failedNames, fmt.Errorf("all %d clusters failed wave generation", len(clusters))
	}

	return domain.MergeWaveResults(successResults), warnings, failedNames, nil
}

// waveFileBase returns the base name for wave-related files (prompt, log, output).
func waveFileBase(index int, clusterName string) string {
	return fmt.Sprintf("wave_%02d_%s", index, domain.SanitizeName(clusterName))
}

// savePromptAndCreateLog writes the prompt file and creates a log writer.
// Returns the log writer and a cleanup function for the log file.
func savePromptAndCreateLog(scanDir, base, prompt string, logger *domain.Logger) (io.Writer, func()) {
	if err := os.WriteFile(filepath.Join(scanDir, base+"_prompt.md"), []byte(prompt), 0644); err != nil {
		logger.Warn("save prompt: %v", err)
	}
	logFile, err := os.Create(filepath.Join(scanDir, base+"_output.log"))
	if err != nil {
		logger.Warn("create log: %v", err)
		return io.Discard, func() {}
	}
	return logFile, func() { logFile.Close() }
}

// parseAndNormalizeWaveResult parses the wave JSON and overrides ClusterName
// to the canonical input name — model output may omit or mislabel it.
func parseAndNormalizeWaveResult(path, clusterName string) (*domain.WaveGenerateResult, error) {
	result, err := ParseWaveGenerateResult(path)
	if err != nil {
		return nil, err
	}
	result.ClusterName = clusterName
	return result, nil
}

// generateWaveForCluster generates waves for a single cluster.
func generateWaveForCluster(ctx context.Context, cfg *domain.Config, scanDir string, index int, cluster domain.ClusterScanResult, dryRun bool, linearTools RunOption, logger *domain.Logger) (domain.WaveGenerateResult, error) {
	base := waveFileBase(index, cluster.Name)
	waveFile := filepath.Join(scanDir, base+".json")

	issuesJSON, err := json.Marshal(cluster.Issues)
	if err != nil {
		return domain.WaveGenerateResult{}, fmt.Errorf("marshal issues for %s: %w", cluster.Name, err)
	}

	prompt, err := domain.RenderWaveGeneratePrompt(cfg.Lang, domain.WaveGeneratePromptData{
		ClusterName:     cluster.Name,
		Completeness:    fmt.Sprintf("%.0f", cluster.Completeness*100),
		Issues:          string(issuesJSON),
		Observations:    strings.Join(cluster.Observations, "\n"),
		DoDSection:      domain.ResolveDoDSection(cfg.DoDTemplates, cluster.Name),
		OutputPath:      waveFile,
		StrictnessLevel: string(domain.ResolveStrictness(cfg.Strictness, append([]string{cluster.Name}, cluster.Labels...))),
	})
	if err != nil {
		return domain.WaveGenerateResult{}, fmt.Errorf("render wave prompt for %s: %w", cluster.Name, err)
	}

	if dryRun {
		return domain.WaveGenerateResult{ClusterName: cluster.Name}, RunClaudeDryRun(cfg, prompt, scanDir, base, logger)
	}

	logOut, closeLog := savePromptAndCreateLog(scanDir, base, prompt, logger)
	defer closeLog()

	logger.Info("Generating waves: %s", cluster.Name)
	if _, err := RunClaude(ctx, cfg, prompt, logOut, logger, linearTools); err != nil {
		return domain.WaveGenerateResult{}, fmt.Errorf("wave generate %s: %w", cluster.Name, err)
	}

	if normErr := NormalizeJSONFile(waveFile); normErr != nil {
		logger.Warn("normalize wave JSON: %v", normErr)
	}
	result, err := parseAndNormalizeWaveResult(waveFile, cluster.Name)
	if err != nil {
		return domain.WaveGenerateResult{}, fmt.Errorf("parse waves %s: %w", cluster.Name, err)
	}
	logger.OK("Cluster %s: %d waves generated", cluster.Name, len(result.Waves))
	return *result, nil
}

// DeepScanFunc is the function signature for scanning a single cluster.
// The index parameter identifies the cluster's position in the original slice,
// enabling safe lookup even when duplicate cluster names exist.
type DeepScanFunc func(ctx context.Context, cfg *domain.Config, scanDir string, index int, cluster domain.ClusterScanResult) (domain.ClusterScanResult, error)

// RunParallelDeepScan executes deep scan across clusters with bounded concurrency.
// Delegates to RunParallel for pond-based parallel orchestration.
// Failed clusters produce warnings and are skipped; successful results preserve order.
func RunParallelDeepScan(ctx context.Context, cfg *domain.Config, scanDir string,
	clusters []domain.ClusterScanResult, scanFn DeepScanFunc, logger *domain.Logger) ([]domain.ClusterScanResult, []string) {

	return RunParallel(ctx, clusters, cfg.Scan.MaxConcurrency,
		func(ctx context.Context, index int, cluster domain.ClusterScanResult) (domain.ClusterScanResult, error) {
			return scanFn(ctx, cfg, scanDir, index, cluster)
		},
		func(c domain.ClusterScanResult) string { return c.Name },
		logger)
}

// RunParallel executes work for each item with bounded concurrency using a
// pond worker pool. Failed items produce warnings and are skipped; successful
// results are returned in the original item order.
//
// Panics inside work are recovered and converted to warnings (no deadlock).
func RunParallel[I, R any](
	ctx context.Context,
	items []I,
	concurrency int,
	work func(ctx context.Context, index int, item I) (R, error),
	itemName func(I) string,
	logger *domain.Logger,
) ([]R, []string) {
	if len(items) == 0 {
		return nil, nil
	}
	if concurrency < 1 {
		concurrency = 1
	}

	// slot holds the outcome of a single work item.
	// Each goroutine writes to a unique index — no synchronization needed.
	type slot struct {
		result R
		err    error
		done   bool // distinguishes completed tasks from cancelled/unstarted
	}
	slots := make([]slot, len(items))

	pool := pond.NewPool(concurrency)
	group := pool.NewGroupContext(ctx)

	for i, item := range items {
		// Stop enqueuing once context is canceled so remaining items
		// are left unstarted and the pool drains promptly.
		if ctx.Err() != nil {
			break
		}
		group.Submit(func() {
			// Inner panic recovery captures the error before pond's outer
			// recovery fires, preserving which task panicked and why.
			defer func() {
				if r := recover(); r != nil {
					slots[i] = slot{err: fmt.Errorf("panic: %v", r), done: true}
				}
			}()
			result, err := work(ctx, i, item)
			slots[i] = slot{result: result, err: err, done: true}
		})
	}
	_ = group.Wait()

	// Stop pool workers to prevent goroutine leaks.
	pool.StopAndWait()

	// Collect results in submission order, skipping failures and unstarted tasks.
	var results []R
	var warnings []string
	for idx, s := range slots {
		if !s.done {
			continue
		}
		if s.err != nil {
			msg := fmt.Sprintf("%q failed: %v", itemName(items[idx]), s.err)
			logger.Warn("%s", msg)
			warnings = append(warnings, msg)
		} else {
			results = append(results, s.result)
		}
	}
	return results, warnings
}
