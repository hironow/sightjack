//go:build windows

package platform

import (
	"os/exec"
	"strconv"
	"strings"
)

// IsProcessAlive checks whether a process with the given PID is still running.
// On Windows, os.FindProcess always succeeds, so we use tasklist to verify.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	cmd := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/NH")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	// tasklist /NH /FI "PID eq X" returns "INFO: No tasks..." when not found
	s := string(out)
	if strings.HasPrefix(s, "INFO") {
		return false
	}
	return strings.Contains(s, strconv.Itoa(pid))
}
