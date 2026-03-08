package platform_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/hironow/sightjack/internal/platform"
)

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.Info("hello %s", "world")
	if !strings.Contains(buf.String(), "INFO hello world") {
		t.Errorf("expected INFO prefix, got %q", buf.String())
	}
}

func TestLogger_OK(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.OK("done")
	if !strings.Contains(buf.String(), " OK  done") {
		t.Errorf("expected OK prefix, got %q", buf.String())
	}
}

func TestLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.Warn("low disk space")
	if !strings.Contains(buf.String(), "WARN low disk space") {
		t.Errorf("expected WARN prefix, got %q", buf.String())
	}
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.Error("something failed: %s", "timeout")
	if !strings.Contains(buf.String(), " ERR something failed: timeout") {
		t.Errorf("expected ERR prefix, got %q", buf.String())
	}
}

func TestLogger_Info_FormerScan(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.Info("classifying")
	if !strings.Contains(buf.String(), "INFO classifying") {
		t.Errorf("expected INFO prefix, got %q", buf.String())
	}
}

func TestLogger_Info_FormerNav(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.Info("rendering")
	if !strings.Contains(buf.String(), "INFO rendering") {
		t.Errorf("expected INFO prefix, got %q", buf.String())
	}
}

func TestLogger_Debug_WhenVerbose(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, true)
	logger.Debug("trace info")
	if !strings.Contains(buf.String(), "DBUG trace info") {
		t.Errorf("expected DBUG prefix, got %q", buf.String())
	}
}

func TestLogger_Debug_WhenNotVerbose(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.Debug("should not appear")
	if buf.Len() != 0 {
		t.Errorf("expected no output when verbose=false, got %q", buf.String())
	}
}

func TestLogger_TimestampFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.Info("test")
	line := buf.String()
	if line[0] != '[' {
		t.Errorf("expected timestamp prefix, got: %s", line)
	}
}

func TestLogger_SetExtraWriter_DualWrite(t *testing.T) {
	// given
	dir := t.TempDir()
	path := dir + "/extra.log"

	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open temp file: %v", err)
	}

	// when: set extra writer and log a message
	logger.SetExtraWriter(f)
	logger.Info("dual-message")

	// then: both buffer and file contain the message
	if !strings.Contains(buf.String(), "dual-message") {
		t.Errorf("buffer should contain dual-message, got: %s", buf.String())
	}
	fileData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(string(fileData), "dual-message") {
		t.Errorf("file should contain dual-message, got: %s", string(fileData))
	}

	// when: set extra writer to nil and log another message
	logger.SetExtraWriter(nil)
	logger.Info("buffer-only-message")

	// then: buffer has the new message, file does not
	if !strings.Contains(buf.String(), "buffer-only-message") {
		t.Errorf("buffer should contain buffer-only-message, got: %s", buf.String())
	}
	fileData2, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file after nil: %v", err)
	}
	if strings.Contains(string(fileData2), "buffer-only-message") {
		t.Errorf("file should NOT contain buffer-only-message after SetExtraWriter(nil)")
	}

	// cleanup: caller is responsible for closing
	f.Close()
}

func TestLogger_Writer(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	if logger.Writer() != &buf {
		t.Error("Writer() should return the configured writer")
	}
}

func TestLogger_NoColorWhenNotTerminal(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.Info("no terminal")
	if strings.Contains(buf.String(), "\033[") {
		t.Errorf("expected no ANSI codes for non-terminal writer, got %q", buf.String())
	}
}

func TestLogger_ColorWhenEnabled(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.SetNoColor(false)
	logger.Info("colored")
	if !strings.Contains(buf.String(), "\033[") {
		t.Errorf("expected ANSI codes when color enabled, got %q", buf.String())
	}
}

func TestLogger_SetNoColor(t *testing.T) {
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.SetNoColor(false)
	logger.Info("on")
	colored := buf.String()

	buf.Reset()
	logger.SetNoColor(true)
	logger.Info("off")
	plain := buf.String()

	if !strings.Contains(colored, "\033[") {
		t.Errorf("expected color when on, got %q", colored)
	}
	if strings.Contains(plain, "\033[") {
		t.Errorf("expected no color when off, got %q", plain)
	}
}

func TestLogger_NoColorEnvVar(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	logger := platform.NewLogger(&buf, false)
	logger.Info("env test")
	if strings.Contains(buf.String(), "\033[") {
		t.Errorf("NO_COLOR=1 should disable color, got %q", buf.String())
	}
}

func TestLogger_ExtraWriterPlainText(t *testing.T) {
	var primary bytes.Buffer
	logger := platform.NewLogger(&primary, false)
	logger.SetNoColor(false)

	var extra bytes.Buffer
	logger.SetExtraWriter(&extra)

	logger.Info("dual")

	if !strings.Contains(primary.String(), "\033[") {
		t.Errorf("primary should have ANSI codes, got %q", primary.String())
	}
	if strings.Contains(extra.String(), "\033[") {
		t.Errorf("extra writer should be plain text, got %q", extra.String())
	}
}

func TestLogger_ConcurrentSetExtraWriterAndWrite(t *testing.T) {
	logger := platform.NewLogger(io.Discard, false)

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(3)
		go func() {
			defer wg.Done()
			var buf bytes.Buffer
			logger.SetExtraWriter(&buf)
		}()
		go func(n int) {
			defer wg.Done()
			logger.Info("race test info %d", n)
			logger.Warn("race test warn %d", n)
		}(i)
		go func() {
			defer wg.Done()
			logger.SetExtraWriter(nil)
		}()
	}
	wg.Wait()

	// Clean up
	logger.SetExtraWriter(nil)
}
