// white-box-reason: tests unexported isProcessAlive function
package session

import (
	"os"
	"runtime"
	"testing"
)

func TestIsProcessAlive_CurrentProcess(t *testing.T) {
	// given: our own PID, which is definitely alive
	pid := os.Getpid()

	// when
	alive := isProcessAlive(pid)

	// then
	if !alive {
		t.Errorf("isProcessAlive(%d) = false for current process; want true", pid)
	}
}

func TestIsProcessAlive_DeadProcess(t *testing.T) {
	// given: PID that almost certainly does not exist
	// Use a very high PID that is unlikely to be in use.
	deadPID := 4999999

	// when
	alive := isProcessAlive(deadPID)

	// then: on Unix, FindProcess always succeeds but Signal(0) fails with ESRCH.
	// On Windows, FindProcess for a non-existent PID returns an error.
	if alive {
		t.Errorf("isProcessAlive(%d) = true for dead PID; want false", deadPID)
	}
}

func TestIsProcessAlive_WindowsSignalUnsupported(t *testing.T) {
	// This test verifies the logic path for Windows where Signal(0) returns
	// "not supported". On non-Windows, we skip since the code path isn't hit.
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	// given: our own PID (alive on Windows)
	pid := os.Getpid()

	// when
	alive := isProcessAlive(pid)

	// then: on Windows, FindProcess succeeds for a live process
	// and isProcessAlive should return true even though Signal(0) is unsupported.
	if !alive {
		t.Errorf("isProcessAlive(%d) = false on Windows for live process; want true", pid)
	}
}
