package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogLevel defines the logging level.
type LogLevel = zapcore.Level

// Global logger instance.
var (
	globalLogger *zap.Logger
	globalSugar  *zap.SugaredLogger
	logMutex     sync.RWMutex
)

// LogConfig holds the logger configuration.
type LogConfig struct {
	// Log level (debug, info, warn, error, dpanic, panic, fatal)
	Level LogLevel

	// Enable console output
	EnableConsole bool

	// Enable file output
	EnableFile bool

	// Log file directory
	LogDir string

	// Max size of log file before rotation (in MB)
	MaxFileSize int

	// Max number of backups to retain
	MaxBackups int

	// Max days to retain log files
	MaxAge int

	// Use development mode (human-readable output)
	Development bool

	// Add caller information
	AddCaller bool

	// Add stack trace for warn and above levels
	AddStacktrace bool
}

// DefaultLogConfig returns the default logger configuration.
func DefaultLogConfig() *LogConfig {
	homeDir, _ := os.UserHomeDir()
	// 日志目录使用固定路径，每次启动时创建新的日志文件（带时间戳）
	logDir := filepath.Join(homeDir, ".taskbridge-mcp", "logs")

	return &LogConfig{
		Level:         zapcore.InfoLevel,
		EnableConsole: true,
		EnableFile:    true,
		LogDir:        logDir,
		MaxFileSize:   100, // 100MB
		MaxBackups:    10,  // 保留最多10个备份
		MaxAge:        30,  // 保留30天
		Development:   false,
		AddCaller:     false,
		AddStacktrace: false,
	}
}

// Init initializes the global logger.
func Init(cfg *LogConfig) error {
	if cfg == nil {
		cfg = DefaultLogConfig()
	}

	// Create log directory if it doesn't exist
	if cfg.EnableFile {
		if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	// Build encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	if cfg.Development {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderConfig.ConsoleSeparator = " | "
	}

	// Build cores
	var cores []zapcore.Core

	// Console core
	if cfg.EnableConsole {
		consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
		consoleSyncer := zapcore.AddSync(os.Stdout)
		consoleCore := zapcore.NewCore(consoleEncoder, consoleSyncer, cfg.Level)
		cores = append(cores, consoleCore)
	}

	// File core
	if cfg.EnableFile {
		fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
		fileSyncer := zapcore.AddSync(newRotatingFile(cfg))
		fileCore := zapcore.NewCore(fileEncoder, fileSyncer, cfg.Level)
		cores = append(cores, fileCore)
	}

	// Create logger
	core := zapcore.NewTee(cores...)

	opts := []zap.Option{
		zap.AddStacktrace(zapcore.ErrorLevel),
	}

	if cfg.AddCaller {
		opts = append(opts, zap.AddCaller())
	}

	if cfg.AddStacktrace {
		opts = append(opts, zap.AddStacktrace(zapcore.WarnLevel))
	}

	logger := zap.New(core, opts...)
	globalSugar = logger.Sugar()

	logMutex.Lock()
	defer logMutex.Unlock()

	globalLogger = logger

	return nil
}

// GetLogger returns the global logger.
func GetLogger() *zap.Logger {
	logMutex.RLock()
	defer logMutex.RUnlock()

	if globalLogger == nil {
		// Initialize with default config if not already initialized
		_ = Init(DefaultLogConfig())
		return globalLogger
	}

	return globalLogger
}

// GetSugaredLogger returns the global sugared logger (easier to use).
func GetSugaredLogger() *zap.SugaredLogger {
	logMutex.RLock()
	defer logMutex.RUnlock()

	if globalSugar == nil {
		// Initialize with default config if not already initialized
		_ = Init(DefaultLogConfig())
		return globalSugar
	}

	return globalSugar
}

// Sync flushes the logger.
func Sync() error {
	logMutex.RLock()
	defer logMutex.RUnlock()

	if globalLogger != nil {
		return globalLogger.Sync()
	}

	return nil
}

// Debug logs a debug message.
func Debug(msg string, fields ...zap.Field) {
	GetLogger().Debug(msg, fields...)
}

// Debugf logs a formatted debug message.
func Debugf(format string, args ...any) {
	GetSugaredLogger().Debugf(format, args...)
}

// Info logs an info message.
func Info(msg string, fields ...zap.Field) {
	GetLogger().Info(msg, fields...)
}

// Infof logs a formatted info message.
func Infof(format string, args ...any) {
	GetSugaredLogger().Infof(format, args...)
}

// Warn logs a warning message.
func Warn(msg string, fields ...zap.Field) {
	GetLogger().Warn(msg, fields...)
}

// Warnf logs a formatted warning message.
func Warnf(format string, args ...any) {
	GetSugaredLogger().Warnf(format, args...)
}

// Error logs an error message.
func Error(msg string, fields ...zap.Field) {
	GetLogger().Error(msg, fields...)
}

// Errorf logs a formatted error message.
func Errorf(format string, args ...any) {
	GetSugaredLogger().Errorf(format, args...)
}

// Fatal logs a fatal message and exits.
func Fatal(msg string, fields ...zap.Field) {
	GetLogger().Fatal(msg, fields...)
}

// Fatalf logs a formatted fatal message and exits.
func Fatalf(format string, args ...any) {
	GetSugaredLogger().Fatalf(format, args...)
}

// WithFields returns a logger with additional fields.
func WithFields(fields ...zap.Field) *zap.Logger {
	return GetLogger().With(fields...)
}

// WithValues returns a sugared logger with additional key-value pairs.
func WithValues(args ...any) *zap.SugaredLogger {
	return GetSugaredLogger().With(args...)
}

// GetLogDirectory returns the current log directory.
func GetLogDirectory() string {
	cfg := DefaultLogConfig()
	return cfg.LogDir
}
