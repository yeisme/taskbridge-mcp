// Package cmd 提供 CLI 命令
package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/pkg/config"
	"github.com/yeisme/taskbridge/pkg/logger"
)

func init() {
	// 设置 Windows 控制台输出为 UTF-8
	if err := os.Setenv("LANG", "en_US.UTF-8"); err != nil {
		// 非关键错误，仅记录
		fmt.Fprintf(os.Stderr, "警告: 设置环境变量失败: %v\n", err)
	}
}

var (
	cfgFile     string
	verbose     bool
	storagePath string
	storageType string
	logLevel    string
	providers   string
	cfg         *config.Config
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "taskbridge",
	Short: "TaskBridge - 连接 AI 与 Todo 软件的桥梁",
	Long: `TaskBridge 是一个 MCP (Model Context Protocol) 工具，
用于连接各种 Todo 软件与 AI，让 AI 能够理解和管理任务。

支持的平台：
  - Microsoft Todo
  - Google Tasks
  - 飞书任务
  - TickTick
  - Dida365（滴答清单国内）
  - Todoist
  - OmniFocus (macOS)
  - Apple Reminders (macOS/iOS)

示例：
  taskbridge sync              # 执行单次同步
  taskbridge serve             # 启动后台服务
  taskbridge list              # 列出所有任务
  taskbridge analyze           # 分析任务（四象限视图）`,
}

// Execute 执行命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "配置文件路径（已弃用，不再读取）")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "详细输出")
	rootCmd.PersistentFlags().StringVar(&storagePath, "storage-path", "", "任务存储路径（可用环境变量 TASKBRIDGE_STORAGE_PATH）")
	rootCmd.PersistentFlags().StringVar(&storageType, "storage-type", "", "存储类型：file|mongodb（可用环境变量 TASKBRIDGE_STORAGE_TYPE）")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "日志级别：debug|info|warn|error（可用环境变量 TASKBRIDGE_LOG_LEVEL）")
	rootCmd.PersistentFlags().StringVar(&providers, "providers", "", "启用的 provider，逗号分隔（可用环境变量 TASKBRIDGE_PROVIDERS）")
	_ = rootCmd.PersistentFlags().MarkDeprecated("config", "配置文件已弃用，请改用环境变量和命令行参数")
}

// initConfig 初始化配置
func initConfig() {
	cfg = config.DefaultConfig()

	// 1) 环境变量覆盖
	if v := strings.TrimSpace(os.Getenv("TASKBRIDGE_STORAGE_PATH")); v != "" {
		cfg.Storage.Path = v
	}
	if v := strings.TrimSpace(os.Getenv("TASKBRIDGE_STORAGE_TYPE")); v != "" {
		cfg.Storage.Type = v
	}
	if v := strings.TrimSpace(os.Getenv("TASKBRIDGE_STORAGE_FORMAT")); v != "" {
		cfg.Storage.File.Format = v
	}
	if v := strings.TrimSpace(os.Getenv("TASKBRIDGE_LOG_LEVEL")); v != "" {
		cfg.App.LogLevel = v
	}
	if v := strings.TrimSpace(os.Getenv("TASKBRIDGE_MCP_TRANSPORT")); v != "" {
		cfg.MCP.Transport = v
	}
	if v := strings.TrimSpace(os.Getenv("TASKBRIDGE_MCP_PORT")); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			cfg.MCP.Port = p
		}
	}
	applyProvidersFromList(strings.TrimSpace(os.Getenv("TASKBRIDGE_PROVIDERS")))

	// 2) 命令行参数覆盖环境变量
	if storagePath != "" {
		cfg.Storage.Path = storagePath
	}
	if storageType != "" {
		cfg.Storage.Type = storageType
	}
	if logLevel != "" {
		cfg.App.LogLevel = logLevel
	}
	if verbose {
		cfg.App.LogLevel = "debug"
	}
	if providers != "" {
		applyProvidersFromList(providers)
	}

	// 初始化全局日志级别，避免调试日志误判为错误
	if err := logger.Init(&logger.Config{
		Level:      cfg.App.LogLevel,
		Format:     "json",
		Output:     "stderr",
		TimeFormat: "",
		Caller:     false,
	}); err != nil {
		// 日志初始化失败不应中断主流程，回退到 info
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		fmt.Fprintf(os.Stderr, "警告: 初始化日志失败，已回退到 info 级别: %v\n", err)
	}
}

func applyProvidersFromList(value string) {
	if strings.TrimSpace(value) == "" {
		return
	}

	// 清空后按列表启用
	cfg.Providers.Google.Enabled = false
	cfg.Providers.Microsoft.Enabled = false
	cfg.Providers.Feishu.Enabled = false
	cfg.Providers.TickTick.Enabled = false
	cfg.Providers.Dida.Enabled = false
	cfg.Providers.Todoist.Enabled = false

	for _, raw := range strings.Split(value, ",") {
		name := provider.ResolveProviderName(strings.TrimSpace(raw))
		switch name {
		case "google":
			cfg.Providers.Google.Enabled = true
		case "microsoft":
			cfg.Providers.Microsoft.Enabled = true
		case "feishu":
			cfg.Providers.Feishu.Enabled = true
		case "ticktick":
			cfg.Providers.TickTick.Enabled = true
		case "dida":
			cfg.Providers.Dida.Enabled = true
		case "todoist":
			cfg.Providers.Todoist.Enabled = true
		case "":
			// ignore empty entry
		default:
			fmt.Fprintf(os.Stderr, "警告: 忽略未知 provider: %s\n", raw)
		}
	}
}

// GetConfig 获取配置
func GetConfig() *config.Config {
	return cfg
}
