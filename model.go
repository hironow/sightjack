package sightjack

import "time"

// ClassifyResult is the output of Pass 1 (cluster classification).
// Written by Claude Code to classify.json.
type ClassifyResult struct {
	Clusters    []ClusterClassification `json:"clusters"`
	TotalIssues int                     `json:"total_issues"`
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

// ScanResult is the merged result of Pass 1 + Pass 2.
type ScanResult struct {
	Clusters     []ClusterScanResult
	TotalIssues  int
	Completeness float64
	Observations []string
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
	Version      string         `json:"version"`
	SessionID    string         `json:"session_id"`
	Project      string         `json:"project"`
	LastScanned  time.Time      `json:"last_scanned"`
	Completeness float64        `json:"completeness"`
	Clusters     []ClusterState `json:"clusters"`
	Waves        []WaveState    `json:"waves,omitempty"`
	ADRCount     int            `json:"adr_count,omitempty"`
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

// WaveApplyResult is the Pass 4 output per wave.
type WaveApplyResult struct {
	WaveID  string   `json:"wave_id"`
	Applied int      `json:"applied"`
	Errors  []string `json:"errors"`
	Ripples []Ripple `json:"ripples"`
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
)

// ResumeChoice represents the user's choice when a previous session is detected.
type ResumeChoice int

const (
	ResumeChoiceResume ResumeChoice = iota
	ResumeChoiceNew
	ResumeChoiceRescan
)

// ScribeResponse is the output of the Scribe Agent (ADR generation).
type ScribeResponse struct {
	ADRID     string `json:"adr_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Reasoning string `json:"reasoning"`
}

// ArchitectResponse is the output of an architect discussion round.
type ArchitectResponse struct {
	Analysis     string `json:"analysis"`
	ModifiedWave *Wave  `json:"modified_wave"`
	Reasoning    string `json:"reasoning"`
}
