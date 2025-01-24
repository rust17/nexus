package internal

import (
	"io"
	"log"
	"os"
	"sync"
)

// Logger 结构体封装日志功能
type Logger struct {
	mu       sync.RWMutex
	logger   *log.Logger
	level    LogLevel
	exitFunc func(int) // 添加退出函数字段
}

// LogLevel 定义日志级别类型
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// NewLogger 创建一个新的日志实例
func NewLogger(level LogLevel) *Logger {
	return &Logger{
		logger:   log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile),
		level:    level,
		exitFunc: os.Exit, // 默认使用 os.Exit
	}
}

// SetLevel 设置日志级别
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.level = level
}

// SetOutput 设置日志输出目标
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logger.SetOutput(w)
}

// SetExitFunc 设置退出函数
func (l *Logger) SetExitFunc(f func(int)) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.exitFunc = f
}

// Debug 输出调试日志
func (l *Logger) Debug(format string, v ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.level <= LevelDebug {
		l.logger.Printf("[DEBUG] "+format, v...)
	}
}

// Info 输出信息日志
func (l *Logger) Info(format string, v ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.level <= LevelInfo {
		l.logger.Printf("[INFO] "+format, v...)
	}
}

// Warn 输出警告日志
func (l *Logger) Warn(format string, v ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.level <= LevelWarn {
		l.logger.Printf("[WARN] "+format, v...)
	}
}

// Error 输出错误日志
func (l *Logger) Error(format string, v ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.level <= LevelError {
		l.logger.Printf("[ERROR] "+format, v...)
	}
}

// Fatal 输出致命错误日志并退出程序
func (l *Logger) Fatal(format string, v ...interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.level <= LevelFatal {
		l.logger.Printf("[FATAL] "+format, v...)
		l.exitFunc(1) // 使用自定义退出函数
	}
}
