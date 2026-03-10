package session

import (
	"fmt"

	"github.com/hironow/sightjack/internal/platform"
)

// PreflightCheck verifies that required binaries are available in PATH.
// Uses platform.LookPathShell which handles shell-aware lookups.
func PreflightCheck(binaries ...string) error {
	for _, bin := range binaries {
		if _, err := platform.LookPathShell(bin); err != nil {
			_, resolved, _ := platform.ParseShellCommand(bin)
			return fmt.Errorf("preflight: %s not found in PATH (from %q)", resolved, bin)
		}
	}
	return nil
}
