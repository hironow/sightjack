package usecase

import (
	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

// LoadConfig reads and validates the sightjack config file.
// Wraps session.LoadConfig to keep session out of the cmd layer.
func LoadConfig(path string) (*domain.Config, error) {
	return session.LoadConfig(path)
}
