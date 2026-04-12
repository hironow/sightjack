package usecase

import (
	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// RunInit delegates project initialization to the InitRunner port.
// The command is always-valid by construction — no validation needed.
// Default values for lang/strictness are applied at the cmd layer before construction.
func RunInit(cmd domain.InitCommand, runner port.InitRunner) ([]string, error) {
	return runner.InitProject(
		cmd.BaseDir().String(),
		port.WithTeam(cmd.Team()),
		port.WithProject(cmd.Project()),
		port.WithLang(cmd.Lang()),
		port.WithStrictness(cmd.Strictness()),
	)
}
