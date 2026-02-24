package sightjack_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/hironow/sightjack"
)

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	logger := sightjack.NewLogger(&buf, false)
	logger.Info("hello %s", "world")
	if !strings.Contains(buf.String(), "INFO hello world") {
		t.Errorf("expected INFO prefix, got %q", buf.String())
	}
}

func TestLogger_OK(t *testing.T) {
	var buf bytes.Buffer
	logger := sightjack.NewLogger(&buf, false)
	logger.OK("done")
	if !strings.Contains(buf.String(), " OK  done") {
		t.Errorf("expected OK prefix, got %q", buf.String())
	}
}

func TestLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	logger := sightjack.NewLogger(&buf, false)
	logger.Warn("low disk space")
	if !strings.Contains(buf.String(), "WARN low disk space") {
		t.Errorf("expected WARN prefix, got %q", buf.String())
	}
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := sightjack.NewLogger(&buf, false)
	logger.Error("something failed: %s", "timeout")
	if !strings.Contains(buf.String(), " ERR something failed: timeout") {
		t.Errorf("expected ERR prefix, got %q", buf.String())
	}
}

func TestLogger_Scan(t *testing.T) {
	var buf bytes.Buffer
	logger := sightjack.NewLogger(&buf, false)
	logger.Scan("classifying")
	if !strings.Contains(buf.String(), "SCAN classifying") {
		t.Errorf("expected SCAN prefix, got %q", buf.String())
	}
}

func TestLogger_Nav(t *testing.T) {
	var buf bytes.Buffer
	logger := sightjack.NewLogger(&buf, false)
	logger.Nav("rendering")
	if !strings.Contains(buf.String(), " NAV rendering") {
		t.Errorf("expected NAV prefix, got %q", buf.String())
	}
}

func TestLogger_Debug_WhenVerbose(t *testing.T) {
	var buf bytes.Buffer
	logger := sightjack.NewLogger(&buf, true)
	logger.Debug("trace info")
	if !strings.Contains(buf.String(), "DBUG trace info") {
		t.Errorf("expected DBUG prefix, got %q", buf.String())
	}
}

func TestLogger_Debug_WhenNotVerbose(t *testing.T) {
	var buf bytes.Buffer
	logger := sightjack.NewLogger(&buf, false)
	logger.Debug("should not appear")
	if buf.Len() != 0 {
		t.Errorf("expected no output when verbose=false, got %q", buf.String())
	}
}

func TestLogger_TimestampFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := sightjack.NewLogger(&buf, false)
	logger.Info("test")
	line := buf.String()
	if line[0] != '[' {
		t.Errorf("expected timestamp prefix, got: %s", line)
	}
}

func TestLogger_SetLogFile_RotatesCorrectly(t *testing.T) {
	// given
	dir := t.TempDir()
	path1 := dir + "/log1.txt"
	path2 := dir + "/log2.txt"

	var buf bytes.Buffer
	logger := sightjack.NewLogger(&buf, false)

	if err := logger.SetLogFile(path1); err != nil {
		t.Fatalf("first SetLogFile failed: %v", err)
	}

	// when: log to first file, then rotate
	logger.Info("first-message")

	if err := logger.SetLogFile(path2); err != nil {
		t.Fatalf("second SetLogFile failed: %v", err)
	}
	defer logger.CloseLogFile()

	logger.Info("second-message")

	// then: first file should have first-message but not second-message
	data1, err := os.ReadFile(path1)
	if err != nil {
		t.Fatalf("read path1: %v", err)
	}
	if !strings.Contains(string(data1), "first-message") {
		t.Errorf("path1 should contain first-message, got: %s", string(data1))
	}
	if strings.Contains(string(data1), "second-message") {
		t.Errorf("path1 should NOT contain second-message (rotation failed)")
	}

	// second file should have second-message
	data2, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("read path2: %v", err)
	}
	if !strings.Contains(string(data2), "second-message") {
		t.Errorf("path2 should contain second-message, got: %s", string(data2))
	}
}

func TestLogger_Writer(t *testing.T) {
	var buf bytes.Buffer
	logger := sightjack.NewLogger(&buf, false)
	if logger.Writer() != &buf {
		t.Error("Writer() should return the configured writer")
	}
}
