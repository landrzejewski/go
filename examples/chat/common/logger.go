package common

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	LogDebug LogLevel = iota
	LogInfo
	LogWarn
	LogError
	LogFatal
)

var logLevelNames = map[LogLevel]string{
	LogDebug: "DEBUG",
	LogInfo:  "INFO",
	LogWarn:  "WARN",
	LogError: "ERROR",
	LogFatal: "FATAL",
}

// Logger provides structured logging
type Logger struct {
	level   LogLevel
	file    *os.File
	logger  *log.Logger
	mu      sync.Mutex
	metrics *LogMetrics
}

// LogMetrics tracks logging statistics
type LogMetrics struct {
	mu      sync.RWMutex
	counts  map[LogLevel]int64
	lastLog time.Time
}

// GlobalLogger is the default logger instance
var GlobalLogger *Logger

// InitLogger initializes the global logger
func InitLogger(filename string, level LogLevel) error {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, GetFileMode())
	if err != nil {
		return err
	}

	GlobalLogger = &Logger{
		level:  level,
		file:   file,
		logger: log.New(file, "", 0),
		metrics: &LogMetrics{
			counts: make(map[LogLevel]int64),
		},
	}

	return nil
}

// Close closes the log file
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// log writes a log message
func (l *Logger) log(level LogLevel, format string, args ...any) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Update metrics
	l.metrics.mu.Lock()
	l.metrics.counts[level]++
	l.metrics.lastLog = time.Now()
	l.metrics.mu.Unlock()

	// Format message
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	levelStr := logLevelNames[level]
	message := fmt.Sprintf(format, args...)

	// Write log
	logLine := fmt.Sprintf("[%s] [%s] %s", timestamp, levelStr, message)
	l.logger.Println(logLine)

	// Also print to console for errors and above
	if level >= LogError {
		log.Println(logLine)
	}

	// TEACHING NOTE: os.Exit does NOT run deferred calls, so a deferred Unlock in
	// this method would never execute. Here that is harmless (the process is ending
	// anyway), but it is a classic trap. The exit itself was moved into Fatal -
	// see the comment there.
}

// Debug logs a debug message
func Debug(format string, args ...any) {
	if GlobalLogger != nil {
		GlobalLogger.log(LogDebug, format, args...)
	}
}

// Info logs an info message
func Info(format string, args ...any) {
	if GlobalLogger != nil {
		GlobalLogger.log(LogInfo, format, args...)
	}
}

// Warn logs a warning message
func Warn(format string, args ...any) {
	if GlobalLogger != nil {
		GlobalLogger.log(LogWarn, format, args...)
	}
}

// Error logs an error message
func Error(format string, args ...any) {
	if GlobalLogger != nil {
		GlobalLogger.log(LogError, format, args...)
	}
}

// Fatal logs a fatal message and exits.
//
// os.Exit has to be HERE and not in Logger.log: when logger initialisation failed
// and GlobalLogger is nil, the previous version logged nothing AND did not end the
// program - main returned normally and the process exited with status 0 despite a
// fatal error.
func Fatal(format string, args ...any) {
	if GlobalLogger != nil {
		GlobalLogger.log(LogFatal, format, args...)
	} else {
		log.Printf("[FATAL] "+format, args...)
	}
	os.Exit(1)
}

// GetMetrics returns logging metrics
func GetMetrics() map[string]any {
	if GlobalLogger == nil || GlobalLogger.metrics == nil {
		return nil
	}

	GlobalLogger.metrics.mu.RLock()
	defer GlobalLogger.metrics.mu.RUnlock()

	metrics := make(map[string]any)
	for level, count := range GlobalLogger.metrics.counts {
		metrics[logLevelNames[level]] = count
	}
	metrics["last_log"] = GlobalLogger.metrics.lastLog

	return metrics
}
