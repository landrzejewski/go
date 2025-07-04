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
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// log writes a log message
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
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

	// Fatal exits the program
	if level == LogFatal {
		os.Exit(1)
	}
}

// Debug logs a debug message
func Debug(format string, args ...interface{}) {
	if GlobalLogger != nil {
		GlobalLogger.log(LogDebug, format, args...)
	}
}

// Info logs an info message
func Info(format string, args ...interface{}) {
	if GlobalLogger != nil {
		GlobalLogger.log(LogInfo, format, args...)
	}
}

// Warn logs a warning message
func Warn(format string, args ...interface{}) {
	if GlobalLogger != nil {
		GlobalLogger.log(LogWarn, format, args...)
	}
}

// Error logs an error message
func Error(format string, args ...interface{}) {
	if GlobalLogger != nil {
		GlobalLogger.log(LogError, format, args...)
	}
}

// Fatal logs a fatal message and exits
func Fatal(format string, args ...interface{}) {
	if GlobalLogger != nil {
		GlobalLogger.log(LogFatal, format, args...)
	}
}

// GetMetrics returns logging metrics
func GetMetrics() map[string]interface{} {
	if GlobalLogger == nil || GlobalLogger.metrics == nil {
		return nil
	}

	GlobalLogger.metrics.mu.RLock()
	defer GlobalLogger.metrics.mu.RUnlock()

	metrics := make(map[string]interface{})
	for level, count := range GlobalLogger.metrics.counts {
		metrics[logLevelNames[level]] = count
	}
	metrics["last_log"] = GlobalLogger.metrics.lastLog

	return metrics
}
