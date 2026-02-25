// Package cmd 提供 CLI 命令
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yeisme/taskbridge/pkg/config"
)

func init() {
	// 设置 Windows 控制台输出为 UTF-8
	if err := os.Setenv("LANG", "en_US.UTF-8"); err != nil {
		// 非关键错误，仅记录
		fmt.Fprintf(os.Stderr, "警告: 设置环境变量失败: %v\n", err)
	}
}

var (
	cfgFile string
	verbose bool
	cfg     *config.Config
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

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "配置文件路径")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "详细输出")
}

// initConfig 初始化配置
func initConfig() {
	var err error
	cfg, err = config.Load(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}
}

// GetConfig 获取配置
func GetConfig() *config.Config {
	return cfg
}
