package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go_redis/pkg/utils"
)

// Log levels
const (
	LevelDebug = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger provides structured logging with configurable verbosity, levels, request tracing, and output targets.
type Logger struct {
	mu          sync.Mutex
	enabled     bool
	level       int
	logRequests bool
	logger      *log.Logger
	file        *os.File
}

// NewLogger initializes a Logger instance.
func NewLogger(enabled bool, levelStr string, logRequests bool, filePath string) (*Logger, error) {
	l := &Logger{
		enabled:     enabled,
		level:       parseLevel(levelStr),
		logRequests: logRequests,
	}

	if !enabled {
		l.logger = log.New(io.Discard, "", 0)
		return l, nil
	}

	var output io.Writer = os.Stdout

	if filePath != "" {
		dir := filepath.Dir(filePath)
		if dir != "." && dir != "" {
			_ = os.MkdirAll(dir, 0755)
		}
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", filePath, err)
		}
		l.file = f
		output = io.MultiWriter(os.Stdout, f)
	}

	l.logger = log.New(output, "", log.LstdFlags|log.Lmicroseconds)
	return l, nil
}

func parseLevel(l string) int {
	switch strings.ToLower(strings.TrimSpace(l)) {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "info":
		fallthrough
	default:
		return LevelInfo
	}
}

// Debug logs debug level messages.
func (l *Logger) Debug(format string, v ...interface{}) {
	if !l.enabled || l.level > LevelDebug {
		return
	}
	l.logger.Printf("[DEBUG] "+format, v...)
}

// Info logs info level messages.
func (l *Logger) Info(format string, v ...interface{}) {
	if !l.enabled || l.level > LevelInfo {
		return
	}
	l.logger.Printf("[INFO] "+format, v...)
}

// Warn logs warning level messages.
func (l *Logger) Warn(format string, v ...interface{}) {
	if !l.enabled || l.level > LevelWarn {
		return
	}
	l.logger.Printf("[WARN] "+format, v...)
}

// Error logs error level messages.
func (l *Logger) Error(format string, v ...interface{}) {
	if !l.enabled || l.level > LevelError {
		return
	}
	l.logger.Printf("[ERROR] "+format, v...)
}

// Request logs individual client request details (origin IP, command, payload summary, duration).
func (l *Logger) Request(originIP string, cmdName string, args [][]byte, duration time.Duration, err error) {
	if !l.enabled || !l.logRequests {
		return
	}

	// Format concise payload summary
	payloadSummary := formatPayload(args)
	errStr := "OK"
	if err != nil {
		errStr = err.Error()
	}

	l.logger.Printf("[REQ] origin=%s cmd=%s payload=%s duration=%.3fms status=%s",
		originIP, cmdName, payloadSummary, float64(duration.Microseconds())/1000.0, errStr)
}

func formatPayload(args [][]byte) string {
	if len(args) == 0 {
		return "[]"
	}

	var sb strings.Builder
	sb.WriteString("[")
	for i, arg := range args {
		if i > 0 {
			sb.WriteString(", ")
		}
		if i >= 5 {
			sb.WriteString(fmt.Sprintf("... (%d more)", len(args)-i))
			break
		}
		str := utils.BytesToString(arg)
		if len(str) > 50 {
			str = str[:50] + "..."
		}
		sb.WriteString(fmt.Sprintf("%q", str))
	}
	sb.WriteString("]")
	return sb.String()
}

// ToggleLogRequests toggles request tracing on/off and returns new state.
func (l *Logger) ToggleLogRequests() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logRequests = !l.logRequests
	return l.logRequests
}

// CycleLogLevel advances log level (DEBUG -> INFO -> WARN -> ERROR -> DEBUG) and returns level string.
func (l *Logger) CycleLogLevel() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = (l.level + 1) % 4
	return l.getLogLevelStrLocked()
}

// GetLogLevelStr returns current log level string.
func (l *Logger) GetLogLevelStr() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.getLogLevelStrLocked()
}

func (l *Logger) getLogLevelStrLocked() string {
	switch l.level {
	case LevelDebug:
		return "DEBUG"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelInfo:
		fallthrough
	default:
		return "INFO"
	}
}

// GetLogRequests returns whether request logging is enabled.
func (l *Logger) GetLogRequests() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.logRequests
}

// Close flushes and closes log file handle if open.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
