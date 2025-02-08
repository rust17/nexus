package logger

import (
	"bytes"
	"testing"
)

const (
	testLogMessage = "test log message"
)

func TestLogger_Levels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		level         LogLevel
		logFunc       func(*Logger)
		expectMessage string
	}{
		{
			name:  "DebugLevel",
			level: LevelDebug,
			logFunc: func(l *Logger) {
				l.Debug(testLogMessage)
			},
			expectMessage: "[DEBUG] " + testLogMessage,
		},
		{
			name:  "InfoLevel",
			level: LevelInfo,
			logFunc: func(l *Logger) {
				l.Info(testLogMessage)
			},
			expectMessage: "[INFO] " + testLogMessage,
		},
		{
			name:  "WarnLevel",
			level: LevelWarn,
			logFunc: func(l *Logger) {
				l.Warn(testLogMessage)
			},
			expectMessage: "[WARN] " + testLogMessage,
		},
		{
			name:  "ErrorLevel",
			level: LevelError,
			logFunc: func(l *Logger) {
				l.Error(testLogMessage)
			},
			expectMessage: "[ERROR] " + testLogMessage,
		},
		{
			name:  "FatalLevel",
			level: LevelFatal,
			logFunc: func(l *Logger) {
				l.SetExitFunc(func(int) {}) // Prevent exit
				l.Fatal(testLogMessage)
			},
			expectMessage: "[FATAL] " + testLogMessage,
		},
	}

	for _, tt := range tests {
		tt := tt // Prevent closure issues
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			logger := GetInstance()
			logger.SetLevel(tt.level)
			logger.SetOutput(&buf)

			tt.logFunc(logger)

			if !bytes.Contains(buf.Bytes(), []byte(tt.expectMessage)) {
				t.Errorf("Expected log message %q, got %q", tt.expectMessage, buf.String())
			}
		})
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		level         LogLevel
		logFunc       func(*Logger)
		expectMessage bool
	}{
		{
			name:  "DebugLevelFiltered",
			level: LevelInfo,
			logFunc: func(l *Logger) {
				l.Debug(testLogMessage)
			},
			expectMessage: false,
		},
		{
			name:  "InfoLevelNotFiltered",
			level: LevelInfo,
			logFunc: func(l *Logger) {
				l.Info(testLogMessage)
			},
			expectMessage: true,
		},
	}

	for _, tt := range tests {
		tt := tt // Prevent closure issues
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			logger := GetInstance()
			logger.SetLevel(tt.level)
			logger.SetOutput(&buf)

			tt.logFunc(logger)

			if tt.expectMessage != (buf.Len() > 0) {
				t.Errorf("Expected message presence=%v, got %v", tt.expectMessage, buf.Len() > 0)
			}
		})
	}
}

func TestLogger_Fatal(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	exitCalled := false

	logger := GetInstance()
	logger.SetLevel(LevelFatal)
	logger.SetOutput(&buf)
	logger.SetExitFunc(func(int) {
		exitCalled = true
	})

	logger.Fatal(testLogMessage)

	if !exitCalled {
		t.Error("Fatal log should call exit function")
	}

	if !bytes.Contains(buf.Bytes(), []byte("[FATAL] "+testLogMessage)) {
		t.Errorf("Expected fatal log message, got %q", buf.String())
	}
}

func TestLogger_UpdateLevel(t *testing.T) {
	logger := GetInstance()
	logger.SetLevel(LevelDebug)

	if logger.Level() != LevelDebug {
		t.Errorf("Expected log level Debug, got %v", logger.Level())
	}
}
