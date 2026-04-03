package domain

// WaveKey returns a globally unique key for a wave: "ClusterKey:ID".
// Falls back to ClusterName if ClusterKey is not set (backward compat).
func WaveKey(w Wave) string {
	key := w.ClusterKey
	if key == "" {
		key = w.ClusterName
	}
	return key + ":" + w.ID
}
