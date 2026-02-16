package sightjack

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogLine_Format(t *testing.T) {
	var buf bytes.Buffer
	formatLogLine(&buf, "INFO", "", "hello %s", "world")
	line := buf.String()
	if !strings.Contains(line, "INFO") {
		t.Errorf("expected INFO prefix, got: %s", line)
	}
	if !strings.Contains(line, "hello world") {
		t.Errorf("expected 'hello world', got: %s", line)
	}
	if line[0] != '[' {
		t.Errorf("expected timestamp prefix, got: %s", line)
	}
}

func TestSetVerbose(t *testing.T) {
	// given: default is not verbose
	SetVerbose(false)

	// then
	if IsVerbose() {
		t.Error("expected verbose to be false by default")
	}

	// when
	SetVerbose(true)
	defer SetVerbose(false)

	// then
	if !IsVerbose() {
		t.Error("expected verbose to be true after SetVerbose(true)")
	}
}

func TestLogDebug_OnlyWhenVerbose(t *testing.T) {
	// given: verbose off, capture formatLogLine output
	SetVerbose(false)
	defer SetVerbose(false)

	var buf bytes.Buffer
	// LogDebug should not write when verbose is off
	formatDebugLine(&buf, "test message %d", 42)
	if buf.Len() != 0 {
		t.Errorf("expected no output when verbose=false, got: %s", buf.String())
	}

	// when: verbose on
	SetVerbose(true)
	formatDebugLine(&buf, "test message %d", 42)

	// then: should produce output
	if buf.Len() == 0 {
		t.Error("expected output when verbose=true")
	}
	if !strings.Contains(buf.String(), "DBUG") {
		t.Errorf("expected DBUG prefix, got: %s", buf.String())
	}
}

func TestInitLogFile_ClosesPreviousHandle(t *testing.T) {
	// given: first log file is opened
	dir := t.TempDir()
	path1 := dir + "/log1.txt"
	path2 := dir + "/log2.txt"

	if err := InitLogFile(path1); err != nil {
		t.Fatalf("first InitLogFile failed: %v", err)
	}

	// capture the first file handle to verify it gets closed
	logMu.Lock()
	firstHandle := logFile
	logMu.Unlock()

	// when: InitLogFile is called again with a different path
	if err := InitLogFile(path2); err != nil {
		t.Fatalf("second InitLogFile failed: %v", err)
	}
	defer CloseLogFile()

	// then: writing to the first handle should fail because it was closed
	_, err := firstHandle.WriteString("should fail")
	if err == nil {
		t.Error("expected write to closed first handle to fail, but it succeeded (FD leak)")
	}
}

func TestLogLine_WithColor(t *testing.T) {
	var buf bytes.Buffer
	formatLogLine(&buf, " OK ", colorGreen, "success")
	line := buf.String()
	if !strings.Contains(line, colorGreen) {
		t.Errorf("expected green color code in output")
	}
	if !strings.Contains(line, colorReset) {
		t.Errorf("expected color reset in output")
	}
}
