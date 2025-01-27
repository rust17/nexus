package logger

import (
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

// Logger struct encapsulates logging functionality
type Logger struct {
	mu       sync.RWMutex
	logger   *log.Logger
	level    LogLevel
	exitFunc func(int) // Add exit function field
}

// LogLevel defines the type for log levels
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

func (l *Logger) Level() LogLevel {
	return l.level
}

func (l *Logger) ToLogLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return LevelInfo
	}
}

// NewLogger creates a new logger instance
func NewLogger(level LogLevel) *Logger {
	return &Logger{
		logger:   log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile),
		level:    level,
		exitFunc: os.Exit, // Default to os.Exit
	}
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.level = level
}

// SetOutput sets the logging output destination
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logger.SetOutput(w)
}

// SetExitFunc sets the exit function
func (l *Logger) SetExitFunc(f func(int)) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.exitFunc = f
}

// Debug outputs debug level logs
func (l *Logger) Debug(format string, v ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.level <= LevelDebug {
		l.logger.Printf("[DEBUG] "+format, v...)
	}
}

// Info outputs information level logs
func (l *Logger) Info(format string, v ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.level <= LevelInfo {
		l.logger.Printf("[INFO] "+format, v...)
	}
}

// Warn outputs warning level logs
func (l *Logger) Warn(format string, v ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.level <= LevelWarn {
		l.logger.Printf("[WARN] "+format, v...)
	}
}

// Error outputs error level logs
func (l *Logger) Error(format string, v ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.level <= LevelError {
		l.logger.Printf("[ERROR] "+format, v...)
	}
}

// Fatal outputs fatal error logs and exits the program
func (l *Logger) Fatal(format string, v ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.level <= LevelFatal {
		l.logger.Printf("[FATAL] "+format, v...)
		l.exitFunc(1) // Use custom exit function
	}
}
