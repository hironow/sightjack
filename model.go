package sightjack

import (
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
}

// ClusterScanResult is the output of Pass 2 (per-cluster deep scan).
// Written by Claude Code to cluster_{name}.json.
type ClusterScanResult struct {
	Name         string        `json:"name"`
	Completeness float64       `json:"completeness"`
	Issues       []IssueDetail `json:"issues"`
	Observations []string      `json:"observations"`
}

// IssueDetail holds the deep scan analysis of a single issue.
type IssueDetail struct {
	ID           string   `json:"id"`
	Identifier   string   `json:"identifier"`
	Title        string   `json:"title"`
	Completeness float64  `json:"completeness"`
	Gaps         []string `json:"gaps"`
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
type ScanResult struct {
	Clusters        []ClusterScanResult
	TotalIssues     int
	Completeness    float64
	Observations    []string
	ShibitoWarnings []ShibitoWarning `json:"shibito_warnings,omitempty"`
	ScanWarnings    []string         `json:"scan_warnings,omitempty"`
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

// SessionState is the thin state file persisted to .siren/state.json.
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
type Wave struct {
	ID            string       `json:"id"`
	ClusterName   string       `json:"cluster_name"`
	Title         string       `json:"title"`
	Description   string       `json:"description"`
	Actions       []WaveAction `json:"actions"`
	Prerequisites []string     `json:"prerequisites"`
	Delta         WaveDelta    `json:"delta"`
	Status        string       `json:"status"`
}

// WaveAction is a single change proposed within a Wave.
type WaveAction struct {
	Type        string `json:"type"`
	IssueID     string `json:"issue_id"`
	Description string `json:"description"`
	Detail      string `json:"detail"`
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

// StrictnessLevel controls DoD analysis depth (SIREN difficulty system).
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
}
