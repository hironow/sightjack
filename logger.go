package sightjack

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
)

var (
	logMu       sync.Mutex
	logFile     *os.File
	verboseMode bool
)

func InitLogFile(path string) error {
	logMu.Lock()
	defer logMu.Unlock()
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	logFile = f
	return nil
}

func CloseLogFile() {
	logMu.Lock()
	defer logMu.Unlock()
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

func formatLogLine(w io.Writer, prefix, color string, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format("15:04:05")
	if color != "" {
		fmt.Fprintf(w, "[%s] %s%s%s %s\n", ts, color, prefix, colorReset, msg)
	} else {
		fmt.Fprintf(w, "[%s] %s %s\n", ts, prefix, msg)
	}
}

func logLine(prefix, color string, format string, args ...any) {
	formatLogLine(os.Stdout, prefix, color, format, args...)
	logMu.Lock()
	defer logMu.Unlock()
	if logFile != nil {
		msg := fmt.Sprintf(format, args...)
		ts := time.Now().Format("15:04:05")
		fmt.Fprintf(logFile, "[%s] %s %s\n", ts, prefix, msg)
	}
}

func LogInfo(format string, args ...any)  { logLine("INFO", colorCyan, format, args...) }
func LogOK(format string, args ...any)    { logLine(" OK ", colorGreen, format, args...) }
func LogWarn(format string, args ...any)   { logLine("WARN", colorYellow, format, args...) }
func LogError(format string, args ...any)  { logLine(" ERR", colorRed, format, args...) }
func LogScan(format string, args ...any)   { logLine("SCAN", colorBlue, format, args...) }
func LogNav(format string, args ...any)    { logLine(" NAV", colorPurple, format, args...) }
func LogDebug(format string, args ...any) {
	if verboseMode {
		logLine("DBUG", colorCyan, format, args...)
	}
}

func SetVerbose(v bool) { verboseMode = v }
func IsVerbose() bool   { return verboseMode }

func formatDebugLine(w io.Writer, format string, args ...any) {
	if verboseMode {
		formatLogLine(w, "DBUG", colorCyan, format, args...)
	}
}
