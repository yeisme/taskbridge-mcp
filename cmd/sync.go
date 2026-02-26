package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/internal/provider/feishu"
	"github.com/yeisme/taskbridge/internal/provider/google"
	"github.com/yeisme/taskbridge/internal/provider/microsoft"
	"github.com/yeisme/taskbridge/internal/provider/ticktick"
	"github.com/yeisme/taskbridge/internal/provider/todoist"
	"github.com/yeisme/taskbridge/internal/storage/filestore"
	"github.com/yeisme/taskbridge/internal/sync"
	"github.com/yeisme/taskbridge/pkg/ui"
)

// syncCmd 同步命令
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "任务同步",
	Long: `同步本地与远程 Todo 服务的任务。

支持三种同步模式:
  - pull: 从远程拉取到本地
  - push: 从本地推送到远程
  - bidirectional: 双向同步

示例:
  taskbridge sync pull google
  taskbridge sync push google --dry-run
  taskbridge sync bidirectional google
  taskbridge sync watch google --interval 5m`,
}

// syncPullCmd 拉取命令
var syncPullCmd = &cobra.Command{
	Use:   "pull <provider>",
	Short: "从远程拉取任务到本地",
	Long: `从指定的 Provider 拉取所有任务到本地存储。

示例:
  taskbridge sync pull google
  taskbridge sync pull google --dry-run`,
	Args: cobra.ExactArgs(1),
	Run:  runSyncPull,
}

// syncPushCmd 推送命令
var syncPushCmd = &cobra.Command{
	Use:   "push <provider>",
	Short: "从本地推送任务到远程",
	Long: `将本地存储的任务推送到指定的 Provider。

示例:
  taskbridge sync push google
  taskbridge sync push google --force`,
	Args: cobra.ExactArgs(1),
	Run:  runSyncPush,
}

// syncBidirectionalCmd 双向同步命令
var syncBidirectionalCmd = &cobra.Command{
	Use:   "bidirectional <provider>",
	Short: "双向同步任务",
	Long: `执行双向同步，先拉取后推送。

示例:
  taskbridge sync bidirectional google`,
	Args: cobra.ExactArgs(1),
	Run:  runSyncBidirectional,
}

// syncWatchCmd 持续同步命令
var syncWatchCmd = &cobra.Command{
	Use:   "watch <provider>",
	Short: "持续监听并同步",
	Long: `持续监听并定期同步任务。

示例:
  taskbridge sync watch google
  taskbridge sync watch google --interval 5m`,
	Args: cobra.ExactArgs(1),
	Run:  runSyncWatch,
}

// syncStatusCmd 同步状态命令
var syncStatusCmd = &cobra.Command{
	Use:   "status [provider]",
	Short: "查看同步状态",
	Long: `显示指定 Provider 或所有 Provider 的同步状态。

示例:
  taskbridge sync status
  taskbridge sync status google`,
	Args: cobra.MaximumNArgs(1),
	Run:  runSyncStatus,
}

var (
	syncDryRun       bool
	syncForce        bool
	syncInterval     time.Duration
	syncOutput       string
	syncDeleteRemote bool
)

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.AddCommand(syncPullCmd)
	syncCmd.AddCommand(syncPushCmd)
	syncCmd.AddCommand(syncBidirectionalCmd)
	syncCmd.AddCommand(syncWatchCmd)
	syncCmd.AddCommand(syncStatusCmd)

	// 通用选项
	for _, cmd := range []*cobra.Command{syncPullCmd, syncPushCmd, syncBidirectionalCmd} {
		cmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "模拟执行，不实际同步")
		cmd.Flags().BoolVar(&syncForce, "force", false, "强制同步，忽略冲突检测")
		cmd.Flags().StringVarP(&syncOutput, "output", "o", "text", "输出格式 (text, json)")
	}

	// push 命令特有选项
	syncPushCmd.Flags().BoolVar(&syncDeleteRemote, "delete", false, "删除远程存在但本地不存在的任务")

	// watch 命令选项
	syncWatchCmd.Flags().DurationVar(&syncInterval, "interval", 5*time.Minute, "同步间隔")
}

// getSyncEngine 获取同步引擎
func getSyncEngine() (*sync.Engine, error) {
	return getSyncEngineForProvider("")
}

// getSyncEngineForProvider 获取指定 Provider 的同步引擎
func getSyncEngineForProvider(providerName string) (*sync.Engine, error) {
	// 解析 provider 简写
	providerName = provider.ResolveProviderName(providerName)

	// 创建存储
	store, err := filestore.New(cfg.Storage.Path, cfg.Storage.File.Format)
	if err != nil {
		return nil, fmt.Errorf("创建存储失败: %w", err)
	}

	// 创建 Provider 映射
	providers := make(map[string]provider.Provider)

	// 初始化 Google Provider
	if providerName == "" || providerName == "google" {
		//即使配置中未启用，如果用户明确指定了 google，也尝试初始化
		googleProvider, err := google.NewProviderFromHome()
		if err != nil {
			if providerName == "google" {
				return nil, fmt.Errorf("初始化 Google Provider 失败: %w\n请运行 'taskbridge auth google' 进行认证", err)
			}
			// 如果只是扫描所有 Provider，静默跳过
		} else if !googleProvider.IsAuthenticated() {
			if providerName == "google" {
				return nil, fmt.Errorf("google Provider 未认证，请运行 'taskbridge auth google' 进行认证")
			}
			// 如果只是扫描所有 Provider，静默跳过
		} else {
			providers["google"] = googleProvider
		}
	}

	// 初始化 Microsoft Provider
	if providerName == "" || providerName == "microsoft" {
		microsoftProvider, err := microsoft.NewProviderFromHome()
		if err != nil {
			if providerName == "microsoft" {
				return nil, fmt.Errorf("初始化 Microsoft Provider 失败: %w\n请运行 'taskbridge auth microsoft' 进行认证", err)
			}
			// 如果只是扫描所有 Provider，静默跳过
		} else if !microsoftProvider.IsAuthenticated() {
			if providerName == "microsoft" {
				return nil, fmt.Errorf("microsoft Provider 未认证，请运行 'taskbridge auth microsoft' 进行认证")
			}
			// 如果只是扫描所有 Provider，静默跳过
		} else {
			providers["microsoft"] = microsoftProvider
		}
	}

	// 初始化 Todoist Provider
	if providerName == "" || providerName == "todoist" {
		todoistProvider, err := todoist.NewProviderFromHome()
		if err != nil {
			if providerName == "todoist" {
				return nil, fmt.Errorf("初始化 Todoist Provider 失败: %w\n请运行 'taskbridge auth login todoist' 进行认证", err)
			}
		} else if err := todoistProvider.Authenticate(context.Background(), nil); err != nil {
			if providerName == "todoist" {
				return nil, fmt.Errorf("todoist Provider 未认证，请运行 'taskbridge auth login todoist' 进行认证")
			}
		} else {
			providers["todoist"] = todoistProvider
		}
	}

	// 初始化 Feishu Provider
	if providerName == "" || providerName == "feishu" {
		feishuProvider, err := feishu.NewProviderFromHome()
		if err != nil {
			if providerName == "feishu" {
				return nil, fmt.Errorf("初始化 Feishu Provider 失败: %w\n请运行 'taskbridge auth login feishu' 进行认证", err)
			}
		} else if !feishuProvider.IsAuthenticated() {
			if providerName == "feishu" {
				return nil, fmt.Errorf("feishu Provider 未认证，请运行 'taskbridge auth login feishu' 进行认证")
			}
		} else {
			providers["feishu"] = feishuProvider
		}
	}

	// 初始化 TickTick Provider
	if providerName == "" || providerName == "ticktick" {
		tickProvider, err := ticktick.NewProviderFromHomeByName("ticktick")
		if err != nil {
			if providerName == "ticktick" {
				return nil, fmt.Errorf("初始化 TickTick Provider 失败: %w\n请运行 'taskbridge auth login ticktick' 进行认证", err)
			}
		} else if err := tickProvider.Authenticate(context.Background(), nil); err != nil {
			if providerName == "ticktick" {
				return nil, fmt.Errorf("ticktick Provider 未认证: %w\n请运行 'taskbridge auth login ticktick' 进行认证", err)
			}
		} else {
			providers["ticktick"] = tickProvider
		}
	}
	// 初始化 Dida Provider
	if providerName == "" || providerName == "dida" {
		didaProvider, err := ticktick.NewProviderFromHomeByName("dida")
		if err != nil {
			if providerName == "dida" {
				return nil, fmt.Errorf("初始化 Dida Provider 失败: %w\n请运行 'taskbridge auth login dida' 进行认证", err)
			}
		} else if err := didaProvider.Authenticate(context.Background(), nil); err != nil {
			if providerName == "dida" {
				return nil, fmt.Errorf("dida Provider 未认证: %w\n请运行 'taskbridge auth login dida' 进行认证", err)
			}
		} else {
			providers["dida"] = didaProvider
		}
	}

	return sync.NewEngine(providers, store), nil
}

// runSyncPull 执行拉取
func runSyncPull(cmd *cobra.Command, args []string) {
	providerName := provider.ResolveProviderName(args[0])

	engine, err := getSyncEngineForProvider(providerName)
	if err != nil {
		fmt.Printf("❌ 初始化同步引擎失败: %v\n", err)
		os.Exit(1)
	}

	opts := sync.Options{
		Direction: sync.DirectionPull,
		Provider:  providerName,
		DryRun:    syncDryRun,
		Force:     syncForce,
	}

	result, err := engine.Sync(context.Background(), opts)
	if err != nil {
		fmt.Printf("❌ 同步失败: %v\n", err)
		os.Exit(1)
	}

	printSyncResult(result)
}

// runSyncPush 执行推送
func runSyncPush(cmd *cobra.Command, args []string) {
	providerName := provider.ResolveProviderName(args[0])

	engine, err := getSyncEngineForProvider(providerName)
	if err != nil {
		fmt.Printf("❌ 初始化同步引擎失败: %v\n", err)
		os.Exit(1)
	}

	opts := sync.Options{
		Direction:    sync.DirectionPush,
		Provider:     providerName,
		DryRun:       syncDryRun,
		Force:        syncForce,
		DeleteRemote: syncDeleteRemote,
	}

	result, err := engine.Sync(context.Background(), opts)
	if err != nil {
		fmt.Printf("❌ 同步失败: %v\n", err)
		os.Exit(1)
	}

	printSyncResult(result)
}

// runSyncBidirectional 执行双向同步
func runSyncBidirectional(cmd *cobra.Command, args []string) {
	providerName := provider.ResolveProviderName(args[0])

	engine, err := getSyncEngineForProvider(providerName)
	if err != nil {
		fmt.Printf("❌ 初始化同步引擎失败: %v\n", err)
		os.Exit(1)
	}

	opts := sync.Options{
		Direction: sync.DirectionBidirectional,
		Provider:  providerName,
		DryRun:    syncDryRun,
		Force:     syncForce,
	}

	result, err := engine.Sync(context.Background(), opts)
	if err != nil {
		fmt.Printf("❌ 同步失败: %v\n", err)
		os.Exit(1)
	}

	printSyncResult(result)
}

// runSyncWatch 执行持续同步
func runSyncWatch(cmd *cobra.Command, args []string) {
	providerName := provider.ResolveProviderName(args[0])

	engine, err := getSyncEngineForProvider(providerName)
	if err != nil {
		fmt.Printf("❌ 初始化同步引擎失败: %v\n", err)
		os.Exit(1)
	}

	opts := sync.Options{
		Direction: sync.DirectionBidirectional,
		Provider:  providerName,
	}

	fmt.Printf("🔄 开始持续同步 %s (间隔: %v)\n", providerName, syncInterval)
	fmt.Println("按 Ctrl+C 停止")

	err = engine.Watch(context.Background(), opts, syncInterval)
	if err != nil {
		fmt.Printf("❌ 持续同步失败: %v\n", err)
		os.Exit(1)
	}
}

// runSyncStatus 执行状态查询
func runSyncStatus(cmd *cobra.Command, args []string) {
	engine, err := getSyncEngine()
	if err != nil {
		fmt.Printf("❌ 初始化同步引擎失败: %v\n", err)
		os.Exit(1)
	}

	if len(args) > 0 {
		// 查询指定 Provider
		providerName := args[0]
		status, err := engine.GetStatus(context.Background(), providerName)
		if err != nil {
			fmt.Printf("❌ 获取同步状态失败: %v\n", err)
			os.Exit(1)
		}
		printSyncStatus(status)
	} else {
		// 查询所有 Provider
		providers := []string{"google", "microsoft", "feishu", "ticktick", "dida", "todoist"}
		for _, p := range providers {
			status, err := engine.GetStatus(context.Background(), p)
			if err != nil {
				continue
			}
			printSyncStatus(status)
		}
	}
}

// printSyncResult 打印同步结果
func printSyncResult(result *sync.Result) {
	if syncOutput == "json" {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Printf("❌ 序列化结果失败: %v\n", err)
			return
		}
		fmt.Println(string(data))
		return
	}

	// 文本格式
	fmt.Println()
	fmt.Printf("📋 同步结果 - %s\n", result.Provider)

	table := ui.NewSimpleTable(
		ui.Column{Header: "字段", Width: 10, AlignLeft: true},
		ui.Column{Header: "值", Width: 24, AlignLeft: true},
		ui.Column{Header: "字段", Width: 10, AlignLeft: true},
		ui.Column{Header: "值", Width: 24, AlignLeft: true},
	)

	table.AddRow("方向", string(result.Direction), "耗时", result.Duration.String())
	table.AddRow("拉取", fmt.Sprintf("%d", result.Pulled), "推送", fmt.Sprintf("%d", result.Pushed))
	table.AddRow("更新", fmt.Sprintf("%d", result.Updated), "删除", fmt.Sprintf("%d", result.Deleted))
	table.AddRow("跳过", fmt.Sprintf("%d", result.Skipped), "错误数", fmt.Sprintf("%d", len(result.Errors)))
	fmt.Println(table.Render())

	if len(result.Errors) > 0 {
		fmt.Printf("\n⚠️ 错误 (%d):\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("  - %s: %s\n", e.Operation, e.Error)
		}
	}

	if syncDryRun {
		fmt.Println("\nℹ️ 这是模拟执行，未实际修改数据")
	}
	fmt.Println()
}

// printSyncStatus 打印同步状态
func printSyncStatus(status *sync.Status) {
	providerNames := map[string]string{
		"google":    "Google Tasks",
		"microsoft": "Microsoft Todo",
		"feishu":    "飞书任务",
		"ticktick":  "TickTick",
		"dida":      "Dida365",
		"todoist":   "Todoist",
	}

	name := providerNames[status.Provider]
	if name == "" {
		name = status.Provider
	}

	fmt.Println()
	fmt.Printf("📋 %s 同步状态\n", name)
	fmt.Println("   ─────────────────────────────────")

	if status.Authenticated {
		fmt.Printf("   认证: ✅ 已认证\n")
	} else {
		fmt.Printf("   认证: ❌ 未认证\n")
	}

	if !status.LastSyncTime.IsZero() {
		fmt.Printf("   最后同步: %s\n", status.LastSyncTime.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("   最后同步: 从未同步\n")
	}

	if status.PendingChanges > 0 {
		fmt.Printf("   待同步: %d 条变更\n", status.PendingChanges)
	}
}
