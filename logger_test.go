package sightjack

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, false)
	logger.Info("hello %s", "world")
	if !strings.Contains(buf.String(), "INFO hello world") {
		t.Errorf("expected INFO prefix, got %q", buf.String())
	}
}

func TestLogger_OK(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, false)
	logger.OK("done")
	if !strings.Contains(buf.String(), " OK  done") {
		t.Errorf("expected OK prefix, got %q", buf.String())
	}
}

func TestLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, false)
	logger.Warn("low disk space")
	if !strings.Contains(buf.String(), "WARN low disk space") {
		t.Errorf("expected WARN prefix, got %q", buf.String())
	}
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, false)
	logger.Error("something failed: %s", "timeout")
	if !strings.Contains(buf.String(), " ERR something failed: timeout") {
		t.Errorf("expected ERR prefix, got %q", buf.String())
	}
}

func TestLogger_Scan(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, false)
	logger.Scan("classifying")
	if !strings.Contains(buf.String(), "SCAN classifying") {
		t.Errorf("expected SCAN prefix, got %q", buf.String())
	}
}

func TestLogger_Nav(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, false)
	logger.Nav("rendering")
	if !strings.Contains(buf.String(), " NAV rendering") {
		t.Errorf("expected NAV prefix, got %q", buf.String())
	}
}

func TestLogger_Debug_WhenVerbose(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, true)
	logger.Debug("trace info")
	if !strings.Contains(buf.String(), "DBUG trace info") {
		t.Errorf("expected DBUG prefix, got %q", buf.String())
	}
}

func TestLogger_Debug_WhenNotVerbose(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, false)
	logger.Debug("should not appear")
	if buf.Len() != 0 {
		t.Errorf("expected no output when verbose=false, got %q", buf.String())
	}
}

func TestLogger_TimestampFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, false)
	logger.Info("test")
	line := buf.String()
	if line[0] != '[' {
		t.Errorf("expected timestamp prefix, got: %s", line)
	}
}

func TestLogger_SetLogFile(t *testing.T) {
	// given
	dir := t.TempDir()
	path1 := dir + "/log1.txt"
	path2 := dir + "/log2.txt"

	var buf bytes.Buffer
	logger := NewLogger(&buf, false)

	if err := logger.SetLogFile(path1); err != nil {
		t.Fatalf("first SetLogFile failed: %v", err)
	}

	// capture the first file handle to verify it gets closed
	logger.mu.Lock()
	firstHandle := logger.logFile
	logger.mu.Unlock()

	// when: SetLogFile is called again with a different path
	if err := logger.SetLogFile(path2); err != nil {
		t.Fatalf("second SetLogFile failed: %v", err)
	}
	defer logger.CloseLogFile()

	// then: writing to the first handle should fail because it was closed
	_, err := firstHandle.WriteString("should fail")
	if err == nil {
		t.Error("expected write to closed first handle to fail, but it succeeded (FD leak)")
	}
}

func TestLogger_Writer(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, false)
	if logger.Writer() != &buf {
		t.Error("Writer() should return the configured writer")
	}
}
