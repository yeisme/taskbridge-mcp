package logger

import (
	"path/filepath"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// newRotatingFile creates a new rotating file writer using lumberjack
// Create a new log file with timestamp every time it is started,
// making it easy to distinguish between different service running processes.
func newRotatingFile(cfg *LogConfig) *lumberjack.Logger {
	// Add a timestamp to the log file name, format: taskbridge-mcp-20060102150405.log
	timestamp := time.Now().Format("20060102150405")
	filename := "taskbridge-mcp-" + timestamp + ".log"
	logFile := filepath.Join(cfg.LogDir, filename)

	return &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    cfg.MaxFileSize, // in MB
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge, // in days
		Compress:   true,       // 压缩旧日志
		LocalTime:  true,       // 使用本地时间
	}
}
