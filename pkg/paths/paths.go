// Package paths 提供路径相关的工具函数
package paths

import (
	"os"
	"path/filepath"
)

const (
	// AppName 应用名称
	AppName = "taskbridge"
	// CredentialsDir 凭证目录名称
	CredentialsDir = "credentials"
)

// GetHomeDir 获取用户 HOME 目录
func GetHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// 回退到当前目录
		return "."
	}
	return home
}

// GetAppDir 获取应用配置目录 (~/.taskbridge)
func GetAppDir() string {
	return filepath.Join(GetHomeDir(), "."+AppName)
}

// GetCredentialsDir 获取凭证目录 (~/.taskbridge/credentials)
func GetCredentialsDir() string {
	return filepath.Join(GetAppDir(), CredentialsDir)
}

// GetTokenPath 获取指定 Provider 的 token 文件路径
// 例如: ~/.taskbridge/credentials/google_token.json
func GetTokenPath(provider string) string {
	return filepath.Join(GetCredentialsDir(), provider+"_token.json")
}

// GetCredentialsPath 获取指定 Provider 的凭证文件路径
// 例如: ~/.taskbridge/credentials/google_credentials.json
func GetCredentialsPath(provider string) string {
	return filepath.Join(GetCredentialsDir(), provider+"_credentials.json")
}

// EnsureDir 确保目录存在
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0700)
}

// EnsureCredentialsDir 确保凭证目录存在
func EnsureCredentialsDir() error {
	return EnsureDir(GetCredentialsDir())
}

// GetConfigPath 获取配置文件路径
// 优先级:
// 1. 环境变量 TASKBRIDGE_CONFIG
// 2. 当前目录 ./configs/config.yaml
// 3. HOME 目录 ~/.taskbridge/config.yaml
func GetConfigPath() string {
	// 环境变量
	if path := os.Getenv("TASKBRIDGE_CONFIG"); path != "" {
		return path
	}

	// 当前目录
	localConfig := "./configs/config.yaml"
	if _, err := os.Stat(localConfig); err == nil {
		return localConfig
	}

	// HOME 目录
	return filepath.Join(GetAppDir(), "config.yaml")
}

// GetDataDir 获取数据目录 (~/.taskbridge/data)
func GetDataDir() string {
	return filepath.Join(GetAppDir(), "data")
}

// GetCacheDir 获取缓存目录 (~/.taskbridge/cache)
func GetCacheDir() string {
	return filepath.Join(GetAppDir(), "cache")
}

// GetLogsDir 获取日志目录 (~/.taskbridge/logs)
func GetLogsDir() string {
	return filepath.Join(GetAppDir(), "logs")
}
