package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	configShowSensitive bool
	configFormat        string
)

// configCmd 配置命令
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "配置管理",
	Long: `管理 TaskBridge 运行配置（已迁移到环境变量和命令行参数）。

子命令:
  show     显示当前配置
  set      设置配置项（已弃用）
  get      获取配置项
  init     初始化配置文件（已弃用）
  validate 验证配置

示例:
  taskbridge config show
  taskbridge config set storage.path ./mydata
  taskbridge config get providers.google.enabled
  taskbridge config init`,
}

// configShowCmd 显示配置
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "显示当前配置",
	Long:  `显示当前加载的配置信息`,
	Run:   runConfigShow,
}

// configSetCmd 设置配置
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "设置配置项",
	Long: `设置指定的配置项。

示例:
  taskbridge config set storage.path ./mydata
  taskbridge config set providers.google.enabled true
  taskbridge config set sync.interval 10m`,
	Args: cobra.ExactArgs(2),
	Run:  runConfigSet,
}

// configGetCmd 获取配置
var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "获取配置项",
	Long: `获取指定配置项的值。

示例:
  taskbridge config get storage.path
  taskbridge config get providers.google.enabled`,
	Args: cobra.ExactArgs(1),
	Run:  runConfigGet,
}

// configInitCmd 初始化配置
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "初始化配置文件",
	Long:  `在当前目录或指定位置创建默认配置文件`,
	Run:   runConfigInit,
}

// configValidateCmd 验证配置
var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "验证配置",
	Long:  `验证当前配置是否有效`,
	Run:   runConfigValidate,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configValidateCmd)

	configShowCmd.Flags().BoolVar(&configShowSensitive, "sensitive", false, "显示敏感信息")
	configShowCmd.Flags().StringVarP(&configFormat, "format", "f", "yaml", "输出格式 (yaml, json)")

	configInitCmd.Flags().StringVar(&cfgFile, "output", "", "配置文件输出路径")
}

func runConfigShow(cmd *cobra.Command, args []string) {
	switch configFormat {
	case "json":
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			fmt.Printf("❌ 序列化配置失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	default:
		data, err := yaml.Marshal(cfg)
		if err != nil {
			fmt.Printf("❌ 序列化配置失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	}

	fmt.Println("\n配置来源: 默认值 + 环境变量 + 命令行参数（config.yaml 已弃用）")
}

func runConfigSet(cmd *cobra.Command, args []string) {
	_ = args
	fmt.Println("❌ `taskbridge config set` 已弃用。请改用环境变量或命令行参数。")
	fmt.Println("   示例: TASKBRIDGE_STORAGE_PATH=./data taskbridge list")
	os.Exit(1)
}

func runConfigGet(cmd *cobra.Command, args []string) {
	key := args[0]

	// 简化实现，根据 key 获取值
	var value interface{}
	parts := strings.Split(key, ".")

	switch parts[0] {
	case "storage":
		if len(parts) > 1 {
			switch parts[1] {
			case "type":
				value = cfg.Storage.Type
			case "path":
				value = cfg.Storage.Path
			case "file":
				if len(parts) > 2 && parts[2] == "format" {
					value = cfg.Storage.File.Format
				} else {
					value = cfg.Storage.File
				}
			case "nosql":
				if len(parts) > 2 && parts[2] == "url" {
					value = cfg.Storage.NoSQL.URL
				} else {
					value = cfg.Storage.NoSQL
				}
			default:
				value = cfg.Storage
			}
		} else {
			value = cfg.Storage
		}
	case "sync":
		if len(parts) > 1 {
			switch parts[1] {
			case "mode":
				value = cfg.Sync.Mode
			case "interval":
				value = cfg.Sync.Interval.String()
			case "conflict_resolution":
				value = cfg.Sync.ConflictResolution
			default:
				value = cfg.Sync
			}
		} else {
			value = cfg.Sync
		}
	case "mcp":
		if len(parts) > 1 {
			switch parts[1] {
			case "enabled":
				value = cfg.MCP.Enabled
			case "transport":
				value = cfg.MCP.Transport
			case "port":
				value = cfg.MCP.Port
			default:
				value = cfg.MCP
			}
		} else {
			value = cfg.MCP
		}
	case "providers":
		if len(parts) > 1 {
			switch parts[1] {
			case "google":
				if len(parts) > 2 && parts[2] == "enabled" {
					value = cfg.Providers.Google.Enabled
				} else {
					value = cfg.Providers.Google
				}
			case "microsoft":
				value = cfg.Providers.Microsoft
			case "feishu":
				value = cfg.Providers.Feishu
			case "ticktick":
				value = cfg.Providers.TickTick
			case "dida":
				value = cfg.Providers.Dida
			case "todoist":
				value = cfg.Providers.Todoist
			default:
				value = cfg.Providers
			}
		} else {
			value = cfg.Providers
		}
	case "app":
		if len(parts) > 1 {
			switch parts[1] {
			case "name":
				value = cfg.App.Name
			case "version":
				value = cfg.App.Version
			case "log_level":
				value = cfg.App.LogLevel
			default:
				value = cfg.App
			}
		} else {
			value = cfg.App
		}
	default:
		fmt.Printf("❌ 未知的配置项: %s\n", key)
		os.Exit(1)
	}

	// 输出值
	data, err := yaml.Marshal(value)
	if err != nil {
		fmt.Printf("%v\n", value)
	} else {
		fmt.Printf("%s", string(data))
	}
}

func runConfigInit(cmd *cobra.Command, args []string) {
	_ = cmd
	_ = args
	fmt.Println("❌ `taskbridge config init` 已弃用。请改用环境变量或命令行参数。")
	fmt.Println("   示例: taskbridge --providers microsoft,todoist mcp start")
	os.Exit(1)
}

func runConfigValidate(cmd *cobra.Command, args []string) {
	_ = cmd
	_ = args

	exitCode := writeValidationReport(os.Stdout, cfg.Validate())
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
