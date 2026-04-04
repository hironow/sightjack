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
var ValidWaveActionType = policy.ValidWaveActionType
var CollectSpecSentIssueIDs = policy.CollectSpecSentIssueIDs
var CollectPROpenIssues = policy.CollectPROpenIssues
var FilterPROpenActions = policy.FilterPROpenActions

// --- Scan policy re-exports ---

var DetectFailedClusterNames = policy.DetectFailedClusterNames
var FilterEmptyClassifications = policy.FilterEmptyClassifications
var ClampCompleteness = policy.ClampCompleteness
var MergeClusterChunks = policy.MergeClusterChunks
var BuildScanRecoveryReport = policy.BuildScanRecoveryReport

// --- Config policy re-exports ---

var ResolveStrictness = policy.ResolveStrictness

// --- Review policy re-exports ---

var IsRateLimited = verifier.IsRateLimited
var SummarizeReview = policy.SummarizeReview

// --- Filter re-exports ---

// PromptRegistry is the type alias for the prompt registry.
type PromptRegistry = filter.Registry

// PromptConfig is the type alias for a prompt definition.
type PromptConfig = filter.PromptConfig

// NewPromptRegistry creates a new prompt registry from embedded YAML files.
var NewPromptRegistry = filter.NewRegistry

// MustNewPromptRegistry returns a Registry or panics. Safe with embed.FS.
var MustNewPromptRegistry = filter.MustNewRegistry

// ExpandPromptTemplate performs simple {key} replacement on a template string.
var ExpandPromptTemplate = filter.ExpandTemplate

// --- filter layer: optimization (Phase 3) ---

type PromptOptimizer = filter.PromptOptimizer
type EvalCase = filter.EvalCase
type OptimizedResult = filter.OptimizedResult

var SavePrompt = filter.Save
var PromptsDir = filter.PromptsDir

// --- Verifier re-exports ---

// ClassifyProviderError inspects stderr output and classifies provider errors.
var ClassifyProviderError = verifier.ClassifyProviderError

