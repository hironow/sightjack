package usecase

import (
	"fmt"
	"strings"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// RunInit validates the InitCommand (with defaults applied) and delegates
// project initialization to the InitRunner port.
// Returns warnings for non-fatal errors (e.g. skill install failures).
func RunInit(cmd domain.InitCommand, runner port.InitRunner) ([]string, error) {
	cmd = cmd.WithDefaults()
	if errs := cmd.Validate(); len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return nil, fmt.Errorf("invalid init command: %s", strings.Join(msgs, "; "))
	}
	return runner.InitProject(cmd.BaseDir, cmd.Team, cmd.Project, cmd.Lang, cmd.Strictness)
}
