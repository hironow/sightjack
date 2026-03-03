package usecase

import (
	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// ADRDir returns the ADR directory path for the project.
func ADRDir(baseDir string) string {
	return session.ADRDir(baseDir)
}

// NextADRNumber returns the next sequential ADR number.
func NextADRNumber(adrDir string) (int, error) {
	return session.NextADRNumber(adrDir)
}

// RenderADRFromDiscuss renders a DiscussResult as ADR Markdown.
func RenderADRFromDiscuss(dr domain.DiscussResult, adrNum int) string {
	return session.RenderADRFromDiscuss(dr, adrNum)
}
