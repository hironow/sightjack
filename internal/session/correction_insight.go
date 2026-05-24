package session

import (
	"github.com/hironow/sightjack/internal/domain"
)

func WriteCorrectionInsights(w *InsightWriter, mails []*domain.DMail, logger domain.Logger) {
	for _, mail := range mails {
		WriteCorrectionInsight(w, mail, logger)
	}
}

func WriteCorrectionInsight(w *InsightWriter, mail *domain.DMail, logger domain.Logger) {
	if w == nil || mail == nil {
		return
	}
	meta := domain.CorrectionMetadataFromMap(mail.Metadata)
	if !meta.IsImprovement() || !meta.HasSupportedVocabulary() {
		return
	}
	entry := meta.InsightEntry(mail.Name)
	if err := w.Append("improvement-loop.md", "improvement-loop", "sightjack", entry); err != nil && logger != nil {
		logger.Warn("write correction insight: %v", err)
	}
}
