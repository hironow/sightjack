package session

import "github.com/hironow/sightjack/internal/domain"

func correctionMetadataForWave(feedback []*domain.DMail, wave domain.Wave) domain.CorrectionMetadata {
	waveKey := domain.WaveKey(wave)
	issueIDs := WaveIssueIDs(wave)
	for i := len(feedback) - 1; i >= 0; i-- {
		mail := feedback[i]
		if mail == nil {
			continue
		}
		meta := domain.CorrectionMetadataFromMap(mail.Metadata)
		if !meta.IsImprovement() || !meta.HasSupportedVocabulary() {
			continue
		}
		if mail.Wave != nil && mail.Wave.ID == waveKey {
			return meta.ForwardForRecheck()
		}
		if overlapsIssueIDs(mail.Issues, issueIDs) {
			return meta.ForwardForRecheck()
		}
	}
	return domain.CorrectionMetadata{}
}

func overlapsIssueIDs(left, right []string) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(left))
	for _, id := range left {
		seen[id] = struct{}{}
	}
	for _, id := range right {
		if _, ok := seen[id]; ok {
			return true
		}
	}
	return false
}
