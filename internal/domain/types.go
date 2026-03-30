package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ClassifyResult is the output of Pass 1 (cluster classification).
// Written by Claude Code to classify.json.
type ClassifyResult struct {
	Clusters        []ClusterClassification `json:"clusters"`
	TotalIssues     int                     `json:"total_issues"`
	ShibitoWarnings []ShibitoWarning        `json:"shibito_warnings,omitempty"`
}

// ClusterClassification holds a cluster name and its issue IDs from Pass 1.
type ClusterClassification struct {
	Name     string   `json:"name"`
	IssueIDs []string `json:"issue_ids"`
	Labels   []string `json:"labels,omitempty"`
}

// ClusterScanResult is the output of Pass 2 (per-cluster deep scan).
// Written by Claude Code to cluster_{name}.json.
type ClusterScanResult struct {
	Name                string        `json:"name"`
	Key                 string        `json:"key"`
	Completeness        float64       `json:"completeness"`
	Issues              []IssueDetail `json:"issues"`
	Observations        []string      `json:"observations"`
	Labels              []string      `json:"labels,omitempty"`
	EstimatedStrictness string        `json:"estimated_strictness,omitempty"`
	StrictnessReasoning string        `json:"strictness_reasoning,omitempty"`
	IssueCount          int           `json:"-"` // computed; used when Issues is nil (e.g. show command)
}

// NumIssues returns the number of issues. It prefers len(Issues) when
// the slice is populated and falls back to the IssueCount field.
func (c ClusterScanResult) NumIssues() int {
	if len(c.Issues) > 0 {
		return len(c.Issues)
	}
	return c.IssueCount
}

// IssueDetail holds the deep scan analysis of a single issue.
type IssueDetail struct {
	ID           string   `json:"id"`
	Identifier   string   `json:"identifier"`
	Title        string   `json:"title"`
	Status       string   `json:"status"`
	Completeness float64  `json:"completeness"`
	Gaps         []string `json:"gaps"`
	Labels       []string `json:"labels,omitempty"`
}

// PROpenLabel is the label indicating that paintress has created a PR for this issue.
const PROpenLabel = "paintress:pr-open"

// HasPROpen reports whether this issue has the paintress:pr-open label.
func (d IssueDetail) HasPROpen() bool {
	for _, l := range d.Labels {
		if l == PROpenLabel {
			return true
		}
	}
	return false
}

// ShibitoWarning represents a detected resurrection risk — a previously
// closed issue pattern re-emerging in current issues.
type ShibitoWarning struct {
	ClosedIssueID  string `json:"closed_issue_id"`
	CurrentIssueID string `json:"current_issue_id"`
	Description    string `json:"description"`
	RiskLevel      string `json:"risk_level"`
}

// ScanResult is the merged result of Pass 1 + Pass 2.
// Wire format: output of `scan --json`.
type ScanResult struct {
	Clusters        []ClusterScanResult `json:"clusters"`
	TotalIssues     int                 `json:"total_issues"`
	Completeness    float64             `json:"completeness"`
	Observations    []string            `json:"observations"`
	ShibitoWarnings []ShibitoWarning    `json:"shibito_warnings,omitempty"`
	ScanWarnings    []string            `json:"scan_warnings,omitempty"`
}

// ClusterLabels returns the labels for a named cluster, or nil if not found.
func (r *ScanResult) ClusterLabels(clusterName string) []string {
	for _, c := range r.Clusters {
		if c.Name == clusterName {
			return c.Labels
		}
	}
	return nil
}

// StrictnessKeys returns the lookup keys for ResolveStrictness: cluster name + key + labels.
func (r *ScanResult) StrictnessKeys(clusterName string) []string {
	keys := []string{clusterName}
	for _, c := range r.Clusters {
		if c.Name == clusterName && c.Key != "" {
			keys = append(keys, c.Key)
			break
		}
	}
	return append(keys, r.ClusterLabels(clusterName)...)
}

// CalculateCompleteness computes overall completeness as the average of cluster completeness values,
// and tallies total issues across all clusters.
func (r *ScanResult) CalculateCompleteness() {
	if len(r.Clusters) == 0 {
		return
	}
	var sum float64
	var total int
	for _, c := range r.Clusters {
		sum += c.Completeness
		total += len(c.Issues)
	}
	r.Completeness = sum / float64(len(r.Clusters))
	r.TotalIssues = total
}

// SessionState is the materialized view projected from event replay.
type SessionState struct {
	Version         string         `json:"version"`
	SessionID       string         `json:"session_id"`
	Project         string         `json:"project"`
	LastScanned     time.Time      `json:"last_scanned"`
	Completeness    float64        `json:"completeness"`
	Clusters        []ClusterState `json:"clusters"`
	Waves           []WaveState    `json:"waves,omitempty"`
	ADRCount        int            `json:"adr_count,omitempty"`
	ShibitoCount    int            `json:"shibito_count,omitempty"`
	FeedbackCount   int            `json:"feedback_count,omitempty"`
	StrictnessLevel string         `json:"strictness_level,omitempty"`
	ScanResultPath  string         `json:"scan_result_path,omitempty"`
}

// ClusterState is the per-cluster state within SessionState.
type ClusterState struct {
	Name         string  `json:"name"`
	Completeness float64 `json:"completeness"`
	IssueCount   int     `json:"issue_count"`
}

// WaveState is the per-wave state within SessionState.
type WaveState struct {
	ID            string       `json:"id"`
	ClusterName   string       `json:"cluster_name"`
	Title         string       `json:"title"`
	Status        string       `json:"status"`
	Prerequisites []string     `json:"prerequisites,omitempty"`
	ActionCount   int          `json:"action_count"`
	Actions       []WaveAction `json:"actions,omitempty"`
	Description   string       `json:"description,omitempty"`
	Delta         WaveDelta    `json:"delta,omitempty"`
}

// Wave is a unit of work proposed by AI for a cluster.
// Wire format: input to `discuss` and `apply` subcommands.
type Wave struct {
	ID              string             `json:"id"`
	ClusterName     string             `json:"cluster_name"`
	ClusterKey      string             `json:"cluster_key,omitempty"`
	Title           string             `json:"title"`
	Description     string             `json:"description"`
	Actions         []WaveAction       `json:"actions"`
	Prerequisites   []string           `json:"prerequisites"`
	Delta           WaveDelta          `json:"delta"`
	Status          string             `json:"status"`
	ClusterContext  *ClusterScanResult `json:"cluster_context,omitempty"`
	ComplexityScore float64            `json:"complexity_score,omitempty"`
}

// WaveAction is a single change proposed within a Wave.
type WaveAction struct {
	Type        string `json:"type"`
	IssueID     string `json:"issue_id"`
	Description string `json:"description"`
	Detail      string `json:"detail"`
}

// WaveStepDef defines a single step within a wave specification (D-Mail schema).
type WaveStepDef struct {
	ID            string   `yaml:"id" json:"id"`
	Title         string   `yaml:"title" json:"title"`
	Description   string   `yaml:"description,omitempty" json:"description,omitempty"`
	Targets       []string `yaml:"targets,omitempty" json:"targets,omitempty"`
	Acceptance    string   `yaml:"acceptance,omitempty" json:"acceptance,omitempty"`
	Prerequisites []string `yaml:"prerequisites,omitempty" json:"prerequisites,omitempty"`
}

// WaveReference links a D-Mail to a wave and optionally a specific step.
type WaveReference struct {
	ID    string        `yaml:"id" json:"id"`
	Step  string        `yaml:"step,omitempty" json:"step,omitempty"`
	Steps []WaveStepDef `yaml:"steps,omitempty" json:"steps,omitempty"`
}

// WaveDelta holds expected completeness change.
type WaveDelta struct {
	Before float64 `json:"before"`
	After  float64 `json:"after"`
}

// WaveGenerateResult is the Pass 3 output per cluster.
type WaveGenerateResult struct {
	ClusterName string `json:"cluster_name"`
	Waves       []Wave `json:"waves"`
}

// NextGenResult is the output of post-completion wave generation.
type NextGenResult struct {
	ClusterName string `json:"cluster_name"`
	Waves       []Wave `json:"waves"`
	Reasoning   string `json:"reasoning"`
}

// WaveApplyResult is the Pass 4 output per wave.
type WaveApplyResult struct {
	WaveID     string   `json:"wave_id"`
	Applied    int      `json:"applied"`
	TotalCount int      `json:"total_count,omitempty"`
	Errors     []string `json:"errors"`
	Ripples    []Ripple `json:"ripples"`
}

// Ripple is a cross-cluster effect from applying a wave.
type Ripple struct {
	ClusterName string `json:"cluster_name"`
	Description string `json:"description"`
}

// ApprovalChoice represents the human's choice at the wave approval prompt.
type ApprovalChoice int

const (
	ApprovalApprove ApprovalChoice = iota
	ApprovalReject
	ApprovalDiscuss
	ApprovalQuit
	ApprovalSelective
)

// ResumeChoice represents the user's choice when a previous session is detected.
type ResumeChoice int

const (
	ResumeChoiceResume ResumeChoice = iota
	ResumeChoiceNew
	ResumeChoiceRescan
)

// ParseSessionMode converts a --session-mode flag value to a ResumeChoice.
func ParseSessionMode(s string) (ResumeChoice, error) {
	switch s {
	case "resume":
		return ResumeChoiceResume, nil
	case "new":
		return ResumeChoiceNew, nil
	case "rescan":
		return ResumeChoiceRescan, nil
	default:
		return 0, fmt.Errorf("invalid session mode %q: must be resume, new, or rescan", s)
	}
}

// StrictnessLevel controls change tolerance for existing implementations.
type StrictnessLevel string

const (
	StrictnessFog      StrictnessLevel = "fog"
	StrictnessAlert    StrictnessLevel = "alert"
	StrictnessLockdown StrictnessLevel = "lockdown"
)

// ParseStrictnessLevel parses a string into a StrictnessLevel.
// Case-insensitive. Returns error for unknown values.
func ParseStrictnessLevel(s string) (StrictnessLevel, error) {
	level := StrictnessLevel(strings.ToLower(s))
	if !level.Valid() {
		return "", fmt.Errorf("unknown strictness level: %q (valid: fog, alert, lockdown)", s)
	}
	return level, nil
}

// Valid returns true if the level is a known strictness value.
func (l StrictnessLevel) Valid() bool {
	switch l {
	case StrictnessFog, StrictnessAlert, StrictnessLockdown:
		return true
	}
	return false
}

// ADRConflict represents a detected contradiction between a new ADR and an existing one.
type ADRConflict struct {
	ExistingADRID string `json:"existing_adr_id"`
	Description   string `json:"description"`
}

// ScribeResponse is the output of the Scribe Agent (ADR generation).
type ScribeResponse struct {
	ADRID     string        `json:"adr_id"`
	Title     string        `json:"title"`
	Content   string        `json:"content"`
	Reasoning string        `json:"reasoning"`
	Conflicts []ADRConflict `json:"conflicts,omitempty"`
}

// ArchitectResponse is the output of an architect discussion round.
type ArchitectResponse struct {
	Analysis     string `json:"analysis"`
	ModifiedWave *Wave  `json:"modified_wave"`
	Reasoning    string `json:"reasoning"`
	Decision     string `json:"decision,omitempty"`
}

// --- Wire format types (pipe interface) ---

// PipeType represents the type of JSON wire data in the pipe interface.
type PipeType int

const (
	PipeTypeUnknown    PipeType = iota
	PipeTypeScanResult          // JSON with top-level "clusters" key
	PipeTypeWavePlan            // JSON with top-level "waves" key
)

// DetectPipeType identifies the wire type of JSON data by checking
// for the presence of discriminating top-level keys.
// "clusters" → ScanResult, "waves" → WavePlan.
func DetectPipeType(data []byte) PipeType {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return PipeTypeUnknown
	}
	if _, ok := raw["clusters"]; ok {
		return PipeTypeScanResult
	}
	if _, ok := raw["waves"]; ok {
		return PipeTypeWavePlan
	}
	return PipeTypeUnknown
}

// WavePlan is the output of `waves` subcommand.
// Contains generated waves and optionally the scan result for context.
type WavePlan struct {
	Waves      []Wave      `json:"waves"`
	ScanResult *ScanResult `json:"scan_result,omitempty"`
}

// DiscussResult is the output of `discuss` subcommand.
// Captures the architect discussion outcome for a single wave.
type DiscussResult struct {
	WaveID        string             `json:"wave_id"`
	Analysis      string             `json:"analysis"`
	Reasoning     string             `json:"reasoning"`
	Decision      string             `json:"decision"`
	Modifications []WaveModification `json:"modifications,omitempty"`
	ADRWorthy     bool               `json:"adr_worthy"`
	ADRTitle      string             `json:"adr_title,omitempty"`
}

// WaveModification describes a change made to a wave action during discussion.
type WaveModification struct {
	ActionIndex int    `json:"action_index"`
	Change      string `json:"change"`
}

// ApplyResult is the output of `apply` subcommand.
// Reports per-action outcomes and downstream effects.
// CompletedWave carries the wave context so downstream pipe commands
// (e.g. nextgen) can operate without replaying event history.
// RemainingWaves carries sibling waves from the original plan so that
// nextgen can accurately determine whether follow-up generation is needed.
type ApplyResult struct {
	WaveID          string         `json:"wave_id"`
	AppliedActions  []ActionResult `json:"applied_actions"`
	RippleEffects   []Ripple       `json:"ripple_effects,omitempty"`
	NewCompleteness float64        `json:"new_completeness"`
	CompletedWave   *Wave          `json:"completed_wave,omitempty"`
	RemainingWaves  []Wave         `json:"remaining_waves,omitempty"`
}

// ActionResult reports the outcome of a single wave action application.
type ActionResult struct {
	Type    string `json:"type"`
	IssueID string `json:"issue_id"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ExistingADR holds the filename and content of an existing ADR file.
type ExistingADR struct {
	Filename string
	Content  string
}
