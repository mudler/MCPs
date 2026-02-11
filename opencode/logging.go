package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

// Logger provides structured logging with different levels
type Logger struct {
	logger   *log.Logger
	logLevel LogLevel
	logFile  *os.File
}

// LogLevel represents the logging level
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

var defaultLogger *Logger

// InitLogger initializes the default logger
func InitLogger() {
	logLevel := getEnvLogLevel("OPENCODE_LOG_LEVEL", INFO)
	logFile := getEnv("OPENCODE_LOG_FILE", "")

	var writer *os.File
	if logFile != "" {
		// Ensure log directory exists
		logDir := filepath.Dir(logFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create log directory: %v\n", err)
			writer = os.Stderr
		} else {
			var err error
			writer, err = os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
				writer = os.Stderr
			}
		}
	} else {
		writer = os.Stderr
	}

	defaultLogger = &Logger{
		logger:   log.New(writer, "", log.LstdFlags),
		logLevel: logLevel,
		logFile:  writer,
	}
}

// GetLogger returns the default logger
func GetLogger() *Logger {
	if defaultLogger == nil {
		InitLogger()
	}
	return defaultLogger
}

// log logs a message at the specified level
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.logLevel {
		return
	}

	msg := fmt.Sprintf(format, args...)
	l.logger.Printf("[%s] %s", level.String(), msg)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, format, args...)
	os.Exit(1)
}

// Close closes the log file if opened
func (l *Logger) Close() {
	if l.logFile != os.Stderr && l.logFile != nil {
		l.logFile.Close()
	}
}

// WithRecovery wraps a function with panic recovery
func WithRecovery(operation string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			logger := GetLogger()
			logger.Error("Panic in %s: %v\nStack trace:\n%s", operation, r, debug.Stack())
		}
	}()
	fn()
}

// getEnvLogLevel parses log level from environment
func getEnvLogLevel(key string, defaultValue LogLevel) LogLevel {
	value := os.Getenv(key)
	switch value {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN", "WARNING":
		return WARN
	case "ERROR":
		return ERROR
	case "FATAL":
		return FATAL
	default:
		return defaultValue
	}
}

// SessionMetrics tracks session-related metrics
type SessionMetrics struct {
	TotalSessionsCreated   int
	TotalSessionsCompleted int
	TotalSessionsFailed    int
	ActiveSessions         int
	StartTime              time.Time
}

var metrics = &SessionMetrics{
	StartTime: time.Now(),
}

// RecordSessionCreated records a new session creation
func RecordSessionCreated() {
	metrics.TotalSessionsCreated++
	metrics.ActiveSessions++
}

// RecordSessionCompleted records a session completion
func RecordSessionCompleted() {
	metrics.TotalSessionsCompleted++
	metrics.ActiveSessions--
	if metrics.ActiveSessions < 0 {
		metrics.ActiveSessions = 0
	}
}

// RecordSessionFailed records a session failure
func RecordSessionFailed() {
	metrics.TotalSessionsFailed++
	metrics.ActiveSessions--
	if metrics.ActiveSessions < 0 {
		metrics.ActiveSessions = 0
	}
}

// GetMetrics returns current metrics
func GetMetrics() SessionMetrics {
	return *metrics
}

// LogMetrics logs current metrics
func LogMetrics() {
	logger := GetLogger()
	logger.Info("Session metrics - Created: %d, Completed: %d, Failed: %d, Active: %d, Uptime: %s",
		metrics.TotalSessionsCreated,
		metrics.TotalSessionsCompleted,
		metrics.TotalSessionsFailed,
		metrics.ActiveSessions,
		time.Since(metrics.StartTime).Round(time.Second),
	)
}
