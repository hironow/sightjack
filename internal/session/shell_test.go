package session

import (
	"runtime"
	"testing"
)

func TestShellName_ReturnsNonEmpty(t *testing.T) {
	name := shellName()
	if name == "" {
		t.Error("shellName() returned empty string")
	}
}

func TestShellFlag_ReturnsNonEmpty(t *testing.T) {
	flag := shellFlag()
	if flag == "" {
		t.Error("shellFlag() returned empty string")
	}
}

func TestShellName_MatchesPlatform(t *testing.T) {
	name := shellName()
	switch runtime.GOOS {
	case "windows":
		if name != "cmd" {
			t.Errorf("on windows, shellName() = %q, want %q", name, "cmd")
		}
	default:
		if name != "sh" {
			t.Errorf("on %s, shellName() = %q, want %q", runtime.GOOS, name, "sh")
		}
	}
}

func TestShellFlag_MatchesPlatform(t *testing.T) {
	flag := shellFlag()
	switch runtime.GOOS {
	case "windows":
		if flag != "/c" {
			t.Errorf("on windows, shellFlag() = %q, want %q", flag, "/c")
		}
	default:
		if flag != "-c" {
			t.Errorf("on %s, shellFlag() = %q, want %q", runtime.GOOS, flag, "-c")
		}
	}
}
