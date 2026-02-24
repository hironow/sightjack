package sightjack

import "time"

// SessionStartedPayload is the payload for EventSessionStarted.
type SessionStartedPayload struct {
	Project         string `json:"project"`
	StrictnessLevel string `json:"strictness_level"`
}

// ScanCompletedPayload is the payload for EventScanCompleted.
type ScanCompletedPayload struct {
	Clusters       []ClusterState `json:"clusters"`
	Completeness   float64        `json:"completeness"`
	ShibitoCount   int            `json:"shibito_count"`
	ScanResultPath string         `json:"scan_result_path"`
	LastScanned    time.Time      `json:"last_scanned"`
}

// WavesGeneratedPayload is the payload for EventWavesGenerated.
type WavesGeneratedPayload struct {
	Waves []WaveState `json:"waves"`
}

// WaveIdentityPayload is a shared payload for events that reference a single wave.
// Used by: EventWaveApproved, EventWaveRejected, EventSpecificationSent, EventReportSent.
type WaveIdentityPayload struct {
	WaveID      string `json:"wave_id"`
	ClusterName string `json:"cluster_name"`
}

// WaveModifiedPayload is the payload for EventWaveModified.
type WaveModifiedPayload struct {
	WaveID      string    `json:"wave_id"`
	ClusterName string    `json:"cluster_name"`
	UpdatedWave WaveState `json:"updated_wave"`
}

// WaveAppliedPayload is the payload for EventWaveApplied.
type WaveAppliedPayload struct {
	WaveID      string   `json:"wave_id"`
	ClusterName string   `json:"cluster_name"`
	Applied     int      `json:"applied"`
	TotalCount  int      `json:"total_count"`
	Errors      []string `json:"errors,omitempty"`
}

// WaveCompletedPayload is the payload for EventWaveCompleted.
type WaveCompletedPayload struct {
	WaveID      string `json:"wave_id"`
	ClusterName string `json:"cluster_name"`
	Applied     int    `json:"applied"`
	TotalCount  int    `json:"total_count"`
}

// CompletenessUpdatedPayload is the payload for EventCompletenessUpdated.
type CompletenessUpdatedPayload struct {
	ClusterName         string  `json:"cluster_name"`
	ClusterCompleteness float64 `json:"cluster_completeness"`
	OverallCompleteness float64 `json:"overall_completeness"`
}

// WavesUnlockedPayload is the payload for EventWavesUnlocked.
type WavesUnlockedPayload struct {
	UnlockedWaveIDs []string `json:"unlocked_wave_ids"`
}

// NextGenWavesAddedPayload is the payload for EventNextGenWavesAdded.
type NextGenWavesAddedPayload struct {
	ClusterName string      `json:"cluster_name"`
	Waves       []WaveState `json:"waves"`
}

// ADRGeneratedPayload is the payload for EventADRGenerated.
type ADRGeneratedPayload struct {
	ADRID string `json:"adr_id"`
	Title string `json:"title"`
}

// ReadyLabelsAppliedPayload is the payload for EventReadyLabelsApplied.
type ReadyLabelsAppliedPayload struct {
	IssueIDs []string `json:"issue_ids"`
}

// SessionResumedPayload is the payload for EventSessionResumed.
type SessionResumedPayload struct {
	OriginalSessionID string `json:"original_session_id"`
}

// SessionRescannedPayload is the payload for EventSessionRescanned.
type SessionRescannedPayload struct {
	OriginalSessionID string `json:"original_session_id"`
}
