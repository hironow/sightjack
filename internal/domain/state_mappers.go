package domain

// WavesToStates maps full Wave records to their WaveState snapshots for
// event payloads (refs issue 0032: the register_waves write path emits
// EventWavesGenerated with the same snapshot shape the session
// aggregate replays).
func WavesToStates(waves []Wave) []WaveState {
	states := make([]WaveState, 0, len(waves))
	for _, w := range waves {
		states = append(states, WaveState{
			ID:            w.ID,
			ClusterName:   w.ClusterName,
			Title:         w.Title,
			Status:        w.Status,
			Prerequisites: w.Prerequisites,
			ActionCount:   len(w.Actions),
			Actions:       w.Actions,
			Description:   w.Description,
			Delta:         w.Delta,
		})
	}
	return states
}

// ClustersToStates maps ClusterScanResult records to ClusterState
// snapshots for ScanCompletedPayload (refs issue 0032: the
// save_scan_result write path).
func ClustersToStates(clusters []ClusterScanResult) []ClusterState {
	states := make([]ClusterState, 0, len(clusters))
	for _, c := range clusters {
		states = append(states, ClusterState{
			Name:         c.Name,
			Completeness: c.Completeness,
			IssueCount:   len(c.Issues),
		})
	}
	return states
}
