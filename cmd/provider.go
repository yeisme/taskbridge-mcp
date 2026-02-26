package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/pkg/ui"
)

// providerCmd Provider 管理命令
var providerCmd = &cobra.Command{
	Use:   "provider",
	Short: "Provider 管理",
	Long: `管理 Todo Provider。

子命令:
  list       列出所有 Provider
  enable     启用 Provider
  disable    禁用 Provider
  configure  配置 Provider
  test       测试 Provider 连接

示例:
  taskbridge provider list
  taskbridge provider enable google
  taskbridge provider test google`,
}

// providerListCmd 列出 Provider
var providerListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有 Provider",
	Long:  `列出所有支持的 Provider 及其状态`,
	Run:   runProviderList,
}

// providerEnableCmd 启用 Provider
var providerEnableCmd = &cobra.Command{
	Use:   "enable <provider>",
	Short: "启用 Provider",
	Long:  `启用指定的 Provider`,
	Args:  cobra.ExactArgs(1),
	Run:   runProviderEnable,
}

// providerDisableCmd 禁用 Provider
var providerDisableCmd = &cobra.Command{
	Use:   "disable <provider>",
	Short: "禁用 Provider",
	Long:  `禁用指定的 Provider`,
	Args:  cobra.ExactArgs(1),
	Run:   runProviderDisable,
}

// providerTestCmd 测试 Provider
var providerTestCmd = &cobra.Command{
	Use:   "test <provider>",
	Short: "测试 Provider 连接",
	Long:  `测试指定 Provider 的连接和认证状态`,
	Args:  cobra.ExactArgs(1),
	Run:   runProviderTest,
}

// providerInfoCmd 显示 Provider 信息
var providerInfoCmd = &cobra.Command{
	Use:   "info <provider>",
	Short: "显示 Provider 详细信息",
	Long:  `显示指定 Provider 的详细信息和能力`,
	Args:  cobra.ExactArgs(1),
	Run:   runProviderInfo,
}

func init() {
	rootCmd.AddCommand(providerCmd)
	providerCmd.AddCommand(providerListCmd)
	providerCmd.AddCommand(providerEnableCmd)
	providerCmd.AddCommand(providerDisableCmd)
	providerCmd.AddCommand(providerTestCmd)
	providerCmd.AddCommand(providerInfoCmd)
}

// ProviderInfo Provider 信息
type ProviderInfo struct {
	Name         string
	ShortName    string
	DisplayName  string
	Description  string
	AuthType     string
	Enabled      bool
	Connected    bool
	Capabilities []string
}

// getProviderInfos 获取所有 Provider 信息
func getProviderInfos() map[string]ProviderInfo {
	return map[string]ProviderInfo{
		"google": {
			Name:        "google",
			ShortName:   "google",
			DisplayName: "Google Tasks",
			Description: "Google 任务管理服务",
			AuthType:    "OAuth2",
			Enabled:     cfg.Providers.Google.Enabled,
		},
		"microsoft": {
			Name:        "microsoft",
			ShortName:   "ms",
			DisplayName: "Microsoft To Do",
			Description: "微软任务管理服务",
			AuthType:    "OAuth2",
			Enabled:     cfg.Providers.Microsoft.Enabled,
		},
		"feishu": {
			Name:        "feishu",
			ShortName:   "feishu",
			DisplayName: "飞书任务",
			Description: "飞书任务管理",
			AuthType:    "App ID/Secret",
			Enabled:     cfg.Providers.Feishu.Enabled,
		},
		"ticktick": {
			Name:        "ticktick",
			ShortName:   "tick",
			DisplayName: "TickTick",
			Description: "TickTick 任务管理",
			AuthType:    "API Token",
			Enabled:     cfg.Providers.TickTick.Enabled,
		},
		"dida": {
			Name:        "dida",
			ShortName:   "tick_cn",
			DisplayName: "Dida365",
			Description: "滴答清单（国内）",
			AuthType:    "API Token",
			Enabled:     cfg.Providers.Dida.Enabled,
		},
		"todoist": {
			Name:        "todoist",
			ShortName:   "todo",
			DisplayName: "Todoist",
			Description: "Todoist 任务管理",
			AuthType:    "API Token",
			Enabled:     cfg.Providers.Todoist.Enabled,
		},
	}
}

func runProviderList(_ *cobra.Command, _ []string) {
	providers := getProviderInfos()

	// 使用 lipgloss table 组件
	table := ui.NewTable("名称", "简写", "状态", "认证方式", "描述")

	order := []string{"google", "microsoft", "feishu", "ticktick", "dida", "todoist"}
	for _, name := range order {
		p := providers[name]
		// 获取实际认证状态
		authStatus, _, _ := getProviderStatus(name)
		var status string
		switch authStatus {
		case "✅ Connected":
			status = ui.StatusConnected
		case "⚠️ Expired":
			status = ui.StatusExpired
		case "❌ Not authenticated":
			status = ui.Warning("未认证")
		default:
			if p.Enabled {
				status = ui.StatusEnabled
			} else {
				status = ui.StatusDisabled
			}
		}
		table.AddRow(p.DisplayName, p.ShortName, status, p.AuthType, p.Description)
	}

	fmt.Println()
	fmt.Println(table.Render())
	fmt.Println()
	fmt.Println(ui.Dim("提示: 使用 'taskbridge provider info <简写>' 查看详细信息"))
	fmt.Println()
}

func runProviderEnable(_ *cobra.Command, args []string) {
	// 解析 Provider 名称（支持简写）
	providerName := provider.ResolveProviderName(args[0])

	// 检查 Provider 是否存在
	if !provider.IsValidProvider(providerName) {
		fmt.Println(ui.Error("未知的 Provider: " + args[0]))
		os.Exit(1)
	}

	// 更新配置
	switch providerName {
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
	}

	fmt.Println(ui.Success("Provider " + providerName + " 已启用"))
	fmt.Println()
	fmt.Println(ui.Bold("下一步:"))
	fmt.Printf("  1. 配置认证: %s\n", ui.Highlight("taskbridge auth login "+providerName))
	fmt.Printf("  2. 测试连接: %s\n", ui.Highlight("taskbridge provider test "+providerName))
	fmt.Println()
}

func runProviderDisable(_ *cobra.Command, args []string) {
	// 解析 Provider 名称（支持简写）
	providerName := provider.ResolveProviderName(args[0])

	// 检查 Provider 是否存在
	if !provider.IsValidProvider(providerName) {
		fmt.Println(ui.Error("未知的 Provider: " + args[0]))
		os.Exit(1)
	}

	// 更新配置
	switch providerName {
	case "google":
		cfg.Providers.Google.Enabled = false
	case "microsoft":
		cfg.Providers.Microsoft.Enabled = false
	case "feishu":
		cfg.Providers.Feishu.Enabled = false
	case "ticktick":
		cfg.Providers.TickTick.Enabled = false
	case "dida":
		cfg.Providers.Dida.Enabled = false
	case "todoist":
		cfg.Providers.Todoist.Enabled = false
	}

	fmt.Println(ui.Success("Provider " + providerName + " 已禁用"))
}

func runProviderTest(_ *cobra.Command, args []string) {
	// 解析 Provider 名称（支持简写）
	providerName := provider.ResolveProviderName(args[0])

	// 检查 Provider 是否存在
	if !provider.IsValidProvider(providerName) {
		fmt.Println(ui.Error("未知的 Provider: " + args[0]))
		os.Exit(1)
	}

	// 获取 Provider 定义
	def, _ := provider.GetProviderDefinition(providerName)

	fmt.Println()
	fmt.Println(ui.Highlight("🔍 测试 Provider: " + def.DisplayName))
	fmt.Println()

	// 检查是否启用
	providerInfos := getProviderInfos()
	p := providerInfos[providerName]
	if !p.Enabled {
		fmt.Println(ui.Error("Provider 未启用"))
		fmt.Printf("   运行 '%s' 启用\n", ui.Highlight("taskbridge provider enable "+providerName))
		return
	}
	fmt.Println(ui.Success("Provider 已启用"))

	// 检查认证状态
	status, _, _ := getProviderStatus(providerName)
	switch status {
	case "✅ Connected":
		fmt.Println(ui.Success("认证状态: 已连接"))
	case "⚠️ Expired":
		fmt.Println(ui.Warning("认证状态: Token 已过期"))
		fmt.Printf("   运行 '%s' 刷新\n", ui.Highlight("taskbridge auth refresh "+providerName))
	default:
		fmt.Println(ui.Error("认证状态: 未认证"))
		fmt.Printf("   运行 '%s' 进行认证\n", ui.Highlight("taskbridge auth login "+providerName))
	}

	fmt.Println()
}

func runProviderInfo(cmd *cobra.Command, args []string) {
	// 解析 Provider 名称（支持简写）
	providerName := provider.ResolveProviderName(args[0])

	// 检查 Provider 是否存在
	if !provider.IsValidProvider(providerName) {
		fmt.Println(ui.Error("未知的 Provider: " + args[0]))
		os.Exit(1)
	}

	// 获取 Provider 定义
	def, _ := provider.GetProviderDefinition(providerName)

	// 获取配置状态
	providerInfos := getProviderInfos()
	p := providerInfos[providerName]

	// 获取能力列表
	capabilities := getProviderCapabilities(providerName)

	// 转换为 CheckItem 格式
	var checkItems []ui.CheckItem
	for _, cap := range capabilities {
		// 检查是否包含"不支持"来判断是否支持该功能
		isSupported := !strings.Contains(cap, "不支持")
		checkItems = append(checkItems, ui.CheckItem{
			Text:    cap,
			Checked: isSupported,
		})
	}

	// 确定状态
	var status string
	if p.Enabled {
		status = "已启用"
	} else {
		status = "未启用"
	}

	// 使用 ProviderCard 组件输出
	fmt.Println()
	fmt.Println(ui.ProviderCard(def.Name, def.DisplayName, def.Description, p.AuthType, status, checkItems))
	fmt.Println()
}

func getProviderCapabilities(providerName string) []string {
	switch providerName {
	case "google":
		return []string{
			"截止日期",
			"任务列表",
			"子任务（有限支持）",
			"优先级（不支持）",
			"标签（不支持）",
			"增量同步（不支持）",
		}
	case "microsoft":
		return []string{
			"截止日期",
			"任务列表",
			"子任务",
			"优先级",
			"提醒",
			"标签（不支持）",
		}
	case "feishu":
		return []string{
			"截止日期",
			"任务列表",
			"优先级",
			"标签",
			"子任务（有限支持）",
		}
	case "ticktick":
		return []string{
			"截止日期",
			"任务列表",
			"子任务",
			"优先级",
			"标签",
			"提醒",
		}
	case "dida":
		return []string{
			"截止日期",
			"任务列表",
			"子任务",
			"优先级",
			"标签",
			"提醒",
		}
	case "todoist":
		return []string{
			"截止日期",
			"项目",
			"子任务",
			"优先级",
			"标签",
		}
	default:
		return []string{"未知"}
	}
}
