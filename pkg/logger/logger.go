// Package logger 提供日志功能
package logger

import (
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Config 日志配置
type Config struct {
	// Level 日志级别: debug, info, warn, error
	Level string
	// Format 输出格式: json, console
	Format string
	// Output 输出目标: stdout, stderr, 或文件路径
	Output string
	// TimeFormat 时间格式
	TimeFormat string
	// Caller 是否显示调用者信息
	Caller bool
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Level:      "info",
		Format:     "console",
		Output:     "stdout",
		TimeFormat: "2006-01-02 15:04:05",
		Caller:     true,
	}
}

// Init 初始化日志
func Init(cfg *Config) error {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// 设置日志级别
	level, err := parseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// 设置时间格式
	if cfg.TimeFormat == "" {
		cfg.TimeFormat = "2006-01-02 15:04:05"
	}
	zerolog.TimeFieldFormat = cfg.TimeFormat

	// 设置输出
	var output io.Writer
	switch strings.ToLower(cfg.Output) {
	case "stdout", "":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		// 文件输出
		file, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		output = file
	}

	// 设置格式
	if strings.ToLower(cfg.Format) == "console" {
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: cfg.TimeFormat,
			NoColor:    false,
		}
	}

	// 创建 logger
	logger := zerolog.New(output).With().Timestamp()

	if cfg.Caller {
		logger = logger.Caller()
	}

	log.Logger = logger.Logger()

	return nil
}

// parseLevel 解析日志级别
func parseLevel(level string) (zerolog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel, nil
	case "info":
		return zerolog.InfoLevel, nil
	case "warn", "warning":
		return zerolog.WarnLevel, nil
	case "error":
		return zerolog.ErrorLevel, nil
	case "fatal":
		return zerolog.FatalLevel, nil
	case "panic":
		return zerolog.PanicLevel, nil
	case "trace":
		return zerolog.TraceLevel, nil
	case "disabled", "none":
		return zerolog.Disabled, nil
	default:
		return zerolog.InfoLevel, nil
	}
}

// Debug 记录调试日志
func Debug() *zerolog.Event {
	return log.Debug()
}

// Info 记录信息日志
func Info() *zerolog.Event {
	return log.Info()
}

// Warn 记录警告日志
func Warn() *zerolog.Event {
	return log.Warn()
}

// Error 记录错误日志
func Error() *zerolog.Event {
	return log.Error()
}

// Fatal 记录致命错误日志并退出
func Fatal() *zerolog.Event {
	return log.Fatal()
}

// Panic 记录恐慌日志
func Panic() *zerolog.Event {
	return log.Panic()
}

// With 返回带有字段的 logger
func With() zerolog.Context {
	return log.With()
}

// Logger 返回全局 logger
func Logger() *zerolog.Logger {
	return &log.Logger
}

// SetLevel 设置日志级别
func SetLevel(level string) error {
	l, err := parseLevel(level)
	if err != nil {
		return err
	}
	zerolog.SetGlobalLevel(l)
	return nil
}

// Fields 快速创建字段
type Fields map[string]interface{}

// F 创建字段快捷方式
func F(key string, value interface{}) *zerolog.Event {
	return log.Info().Fields(map[string]interface{}{key: value})
}
