package sightjack

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type Logger struct {
	out     io.Writer
	mu      sync.Mutex
	logFile *os.File
	verbose bool
}

func NewLogger(out io.Writer, verbose bool) *Logger {
	if out == nil {
		out = io.Discard
	}
	return &Logger{out: out, verbose: verbose}
}

func (l *Logger) logLine(prefix, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s %s\n", ts, prefix, msg)
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprint(l.out, line)
	if l.logFile != nil {
		fmt.Fprint(l.logFile, line)
	}
}

func (l *Logger) Info(format string, args ...any)  { l.logLine("INFO", format, args...) }
func (l *Logger) OK(format string, args ...any)    { l.logLine(" OK ", format, args...) }
func (l *Logger) Warn(format string, args ...any)  { l.logLine("WARN", format, args...) }
func (l *Logger) Error(format string, args ...any) { l.logLine(" ERR", format, args...) }
func (l *Logger) Scan(format string, args ...any)  { l.logLine("SCAN", format, args...) }
func (l *Logger) Nav(format string, args ...any)   { l.logLine(" NAV", format, args...) }

func (l *Logger) Debug(format string, args ...any) {
	if l.verbose {
		l.logLine("DBUG", format, args...)
	}
}

func (l *Logger) Writer() io.Writer { return l.out }

func (l *Logger) SetLogFile(path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.logFile != nil {
		l.logFile.Close()
		l.logFile = nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	l.logFile = f
	return nil
}

func (l *Logger) CloseLogFile() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.logFile != nil {
		l.logFile.Close()
		l.logFile = nil
	}
}
