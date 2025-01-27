package test

import (
	"bytes"
	"testing"

	lg "nexus/internal/logger"
)

const (
	testLogMessage = "test log message"
)

func TestLogger_Levels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		level         lg.LogLevel
		logFunc       func(*lg.Logger)
		expectMessage string
	}{
		{
			name:  "DebugLevel",
			level: lg.LevelDebug,
			logFunc: func(l *lg.Logger) {
				l.Debug(testLogMessage)
			},
			expectMessage: "[DEBUG] " + testLogMessage,
		},
		{
			name:  "InfoLevel",
			level: lg.LevelInfo,
			logFunc: func(l *lg.Logger) {
				l.Info(testLogMessage)
			},
			expectMessage: "[INFO] " + testLogMessage,
		},
		{
			name:  "WarnLevel",
			level: lg.LevelWarn,
			logFunc: func(l *lg.Logger) {
				l.Warn(testLogMessage)
			},
			expectMessage: "[WARN] " + testLogMessage,
		},
		{
			name:  "ErrorLevel",
			level: lg.LevelError,
			logFunc: func(l *lg.Logger) {
				l.Error(testLogMessage)
			},
			expectMessage: "[ERROR] " + testLogMessage,
		},
		{
			name:  "FatalLevel",
			level: lg.LevelFatal,
			logFunc: func(l *lg.Logger) {
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
			logger := lg.GetInstance()
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
		level         lg.LogLevel
		logFunc       func(*lg.Logger)
		expectMessage bool
	}{
		{
			name:  "DebugLevelFiltered",
			level: lg.LevelInfo,
			logFunc: func(l *lg.Logger) {
				l.Debug(testLogMessage)
			},
			expectMessage: false,
		},
		{
			name:  "InfoLevelNotFiltered",
			level: lg.LevelInfo,
			logFunc: func(l *lg.Logger) {
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
			logger := lg.GetInstance()
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

	logger := lg.GetInstance()
	logger.SetLevel(lg.LevelFatal)
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
	logger := lg.GetInstance()
	logger.SetLevel(lg.LevelDebug)

	if logger.Level() != lg.LevelDebug {
		t.Errorf("Expected log level Debug, got %v", logger.Level())
	}
}
