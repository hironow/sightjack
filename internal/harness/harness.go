// Package harness provides a unified facade for the harness sub-packages
// (policy and verifier). Callers should import this package rather than
// the sub-packages directly, so that internal reorganization does not
// ripple through the codebase.
package harness

import (
	"github.com/hironow/sightjack/internal/harness/filter"
	"github.com/hironow/sightjack/internal/harness/policy"
	"github.com/hironow/sightjack/internal/harness/verifier"
)

// --- Constants ---

// MaxWavesPerCluster is the cap on total waves per cluster.
const MaxWavesPerCluster = policy.MaxWavesPerCluster

// --- Wave policy re-exports ---

var SortWavesByComplexity = policy.SortWavesByComplexity
var NormalizeWavePrerequisites = policy.NormalizeWavePrerequisites
var RemoveSelfReferences = policy.RemoveSelfReferences
var ClampDelta = policy.ClampDelta
var MergeWaveResults = policy.MergeWaveResults
var AvailableWaves = policy.AvailableWaves
var EvaluateUnlocks = policy.EvaluateUnlocks
var CalcNewlyUnlocked = policy.CalcNewlyUnlocked
var PartialApplyDelta = policy.PartialApplyDelta
var ValidateWaveApplyResult = verifier.ValidateWaveApplyResult
var IsWaveApplyComplete = policy.IsWaveApplyComplete
var ApplyModifiedWave = policy.ApplyModifiedWave
var PropagateWaveUpdate = policy.PropagateWaveUpdate
var DetectWaveCycles = policy.DetectWaveCycles
var PruneStaleWaves = policy.PruneStaleWaves
var ValidateWavePrerequisites = verifier.ValidateWavePrerequisites
var RepairLockedWaves = policy.RepairLockedWaves
var BuildCompletedWaveMap = policy.BuildCompletedWaveMap
var MergeOldWaves = policy.MergeOldWaves
var MergeCompletedStatus = policy.MergeCompletedStatus
var RestoreWaves = policy.RestoreWaves
var BuildWaveStates = policy.BuildWaveStates
var CheckCompletenessConsistency = policy.CheckCompletenessConsistency
var ToApplyResult = policy.ToApplyResult
var FilterEmptyWaves = policy.FilterEmptyWaves
var AutoSelectWave = policy.AutoSelectWave
var CompletedWavesForCluster = policy.CompletedWavesForCluster
var NeedsMoreWaves = policy.NeedsMoreWaves
var ReadyIssueIDs = policy.ReadyIssueIDs
var ClustersForIssueIDs = policy.ClustersForIssueIDs
var LastCompletedWaveForCluster = policy.LastCompletedWaveForCluster
var CollectSpecSentIssueIDs = policy.CollectSpecSentIssueIDs
var CollectPROpenIssues = policy.CollectPROpenIssues
var FilterPROpenActions = policy.FilterPROpenActions

// --- Scan policy re-exports ---

var DetectFailedClusterNames = policy.DetectFailedClusterNames
var FilterEmptyClassifications = policy.FilterEmptyClassifications
var MergeClusterChunks = policy.MergeClusterChunks

// --- Config policy re-exports ---

var ResolveStrictness = policy.ResolveStrictness

// --- Convergence gate policy re-exports ---

const MaxConvergenceRedrainCycles = policy.MaxConvergenceRedrainCycles

var IsConvergenceKind = policy.IsConvergenceKind
var BuildConvergenceSummary = policy.BuildConvergenceSummary

// --- Review policy re-exports ---

var IsRateLimited = verifier.IsRateLimited
var SummarizeReview = policy.SummarizeReview

// --- Filter re-exports ---

// MustDefaultPromptRegistry returns the singleton or panics. Safe with embed.FS.
var MustDefaultPromptRegistry = filter.MustDefault

// --- Prompt render re-exports ---

var RenderClassifyPrompt = filter.RenderClassifyPrompt
var RenderDeepScanPrompt = filter.RenderDeepScanPrompt
var RenderWaveGeneratePrompt = filter.RenderWaveGeneratePrompt
var RenderWaveApplyPrompt = filter.RenderWaveApplyPrompt
var RenderScribeADRPrompt = filter.RenderScribeADRPrompt
var RenderArchitectDiscussPrompt = filter.RenderArchitectDiscussPrompt
var RenderReadyLabelPrompt = filter.RenderReadyLabelPrompt
var RenderNextGenPrompt = filter.RenderNextGenPrompt
var RenderAutoDiscussArchitectPrompt = filter.RenderAutoDiscussArchitectPrompt
var RenderAutoDiscussDevilsAdvocatePrompt = filter.RenderAutoDiscussDevilsAdvocatePrompt

// --- Verifier re-exports ---

// ClassifyProviderError inspects stderr output and classifies provider errors.
var ClassifyProviderError = verifier.ClassifyProviderError

// --- filter layer: optimization (Phase 3) ---

type PromptOptimizer = filter.PromptOptimizer
type EvalCase = filter.EvalCase
type OptimizedResult = filter.OptimizedResult

var SavePrompt = filter.Save
var PromptsDir = filter.PromptsDir
