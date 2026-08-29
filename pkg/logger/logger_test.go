package logger

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoggerLevels(t *testing.T) {
	l, err := NewLogger(true, "warn", false, "")
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	buf := new(bytes.Buffer)
	l.logger.SetOutput(buf)

	l.Debug("debug message")
	l.Info("info message")
	l.Warn("warn message")
	l.Error("error message")

	out := buf.String()
	if strings.Contains(out, "[DEBUG]") || strings.Contains(out, "[INFO]") {
		t.Errorf("expected debug/info messages to be filtered out at WARN level, got: %s", out)
	}

	if !strings.Contains(out, "[WARN] warn message") {
		t.Errorf("expected [WARN] message in log output")
	}
	if !strings.Contains(out, "[ERROR] error message") {
		t.Errorf("expected [ERROR] message in log output")
	}
}

func TestLoggerRequestTracing(t *testing.T) {
	l, err := NewLogger(true, "info", true, "")
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	buf := new(bytes.Buffer)
	l.logger.SetOutput(buf)

	l.Request("127.0.0.1:54321", "SET", [][]byte{[]byte("key1"), []byte("val1")}, 1500*time.Microsecond, nil)

	out := buf.String()
	if !strings.Contains(out, "[REQ] origin=127.0.0.1:54321 cmd=SET payload=[\"key1\", \"val1\"]") || !strings.Contains(out, "status=OK") {
		t.Errorf("unexpected request log output: %s", out)
	}
}

func TestLoggerDisabled(t *testing.T) {
	l, err := NewLogger(false, "info", true, "")
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	buf := new(bytes.Buffer)
	l.logger.SetOutput(buf)

	l.Info("test info")
	l.Request("127.0.0.1:1234", "GET", nil, time.Millisecond, errors.New("err"))

	if buf.Len() > 0 {
		t.Errorf("expected no log output when disabled, got: %s", buf.String())
	}
}

func TestLoggerFile(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	l, err := NewLogger(true, "info", true, logPath)
	if err != nil {
		t.Fatalf("NewLogger with file failed: %v", err)
	}

	l.Info("file log test")
	_ = l.Close()

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if !strings.Contains(string(content), "[INFO] file log test") {
		t.Errorf("expected file log content, got: %s", string(content))
	}
}
