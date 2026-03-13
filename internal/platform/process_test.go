package platform

// white-box-reason: tests internal IsProcessAlive logic across platforms (EPERM, invalid PID)

import (
	"os"
	"testing"
)

func TestIsProcessAlive_CurrentProcess(t *testing.T) {
	// given
	pid := os.Getpid()

	// when
	alive := IsProcessAlive(pid)

	// then
	if !alive {
		t.Errorf("IsProcessAlive(%d) = false, want true (own process)", pid)
	}
}

func TestIsProcessAlive_ZeroPID(t *testing.T) {
	// given/when
	alive := IsProcessAlive(0)

	// then
	if alive {
		t.Error("IsProcessAlive(0) = true, want false")
	}
}

func TestIsProcessAlive_NegativePID(t *testing.T) {
	// given/when
	alive := IsProcessAlive(-1)

	// then
	if alive {
		t.Error("IsProcessAlive(-1) = true, want false")
	}
}

func TestIsProcessAlive_UnlikelyHighPID(t *testing.T) {
	// given — PID 4194304 is very unlikely to exist
	alive := IsProcessAlive(4194304)

	// then
	if alive {
		t.Error("IsProcessAlive(4194304) = true, want false (unlikely PID)")
	}
}
