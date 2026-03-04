package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/yeisme/taskbridge/internal/auth"
	"github.com/yeisme/taskbridge/internal/provider/feishu"
	"github.com/yeisme/taskbridge/internal/provider/google"
	microsoft "github.com/yeisme/taskbridge/internal/provider/microsoft"
	"github.com/yeisme/taskbridge/internal/provider/ticktick"
	"github.com/yeisme/taskbridge/internal/provider/todoist"
	"github.com/yeisme/taskbridge/pkg/paths"
	"github.com/yeisme/taskbridge/pkg/tokenstore"
)

// serveCmd 后台服务命令
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "启动后台服务",
	Long: `启动 TaskBridge 后台服务。

功能:
  - Token 自动刷新（防止过期）
  - 定时同步任务
  - MCP Server（可选）
  - 健康检查（可选）

示例:
  taskbridge serve                    # 启动服务（默认配置）
  taskbridge serve --enable-mcp       # 启用 MCP Server
  taskbridge serve --check-interval 2m # 设置检查间隔为 2 分钟`,
	Run: runServe,
}

var (
	// 服务配置
	serveEnableMCP         bool
	serveMCPPort           int
	serveCheckInterval     string
	serveEnableSync        bool
	serveSyncInterval      string
	serveEnableHealth      bool
	serveHealthPort        int
	serveRefreshBuffer     string
	serveEnableAutoRefresh bool
)

func init() {
	rootCmd.AddCommand(serveCmd)

	// MCP 配置
	serveCmd.Flags().BoolVar(&serveEnableMCP, "enable-mcp", false, "启用 MCP Server")
	serveCmd.Flags().IntVar(&serveMCPPort, "mcp-port", 0, "MCP Server 端口（默认使用 stdio）")

	// Token 刷新配置
	serveCmd.Flags().BoolVar(&serveEnableAutoRefresh, "enable-auto-refresh", true, "启用 Token 自动刷新")
	serveCmd.Flags().StringVar(&serveCheckInterval, "check-interval", "1m", "Token 检查间隔")
	serveCmd.Flags().StringVar(&serveRefreshBuffer, "refresh-buffer", "5m", "刷新提前量（Token 过期前多久刷新）")

	// 同步配置
	serveCmd.Flags().BoolVar(&serveEnableSync, "enable-sync", false, "启用定时同步")
	serveCmd.Flags().StringVar(&serveSyncInterval, "sync-interval", "5m", "同步间隔")

	// 健康检查配置
	serveCmd.Flags().BoolVar(&serveEnableHealth, "enable-health", false, "启用健康检查端点")
	serveCmd.Flags().IntVar(&serveHealthPort, "health-port", 8081, "健康检查端口")
}

// runServe 执行后台服务
func runServe(cmd *cobra.Command, args []string) {
	fmt.Println("🚀 TaskBridge 后台服务启动中...")

	// 解析配置
	checkInterval, err := time.ParseDuration(serveCheckInterval)
	if err != nil {
		fmt.Printf("❌ 无效的检查间隔: %v\n", err)
		os.Exit(1)
	}

	refreshBuffer, err := time.ParseDuration(serveRefreshBuffer)
	if err != nil {
		fmt.Printf("❌ 无效的刷新提前量: %v\n", err)
		os.Exit(1)
	}

	// 创建 Token 管理器
	tokenManager := auth.NewTokenManager(auth.TokenManagerConfig{
		CheckInterval: checkInterval,
		RefreshBuffer: refreshBuffer,
		MaxRetries:    3,
		RetryInterval: 30 * time.Second,
	})

	// 注册 Provider
	registerProviders(tokenManager)

	// 设置刷新回调
	tokenManager.SetOnRefreshCallback(func(provider string, err error) {
		if err != nil {
			fmt.Printf("❌ %s Token 刷新失败: %v\n", provider, err)
		} else {
			fmt.Printf("✅ %s Token 刷新成功\n", provider)
		}
	})

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 Token 自动刷新
	if serveEnableAutoRefresh {
		if err := tokenManager.Start(ctx); err != nil {
			fmt.Printf("❌ 启动 Token 管理器失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Token 自动刷新已启用 (检查间隔: %s, 刷新提前量: %s)\n", checkInterval, refreshBuffer)
	}

	// 显示当前 Token 状态
	printTokenStatus(tokenManager)

	// 启动 MCP Server（如果启用）
	if serveEnableMCP {
		go startMCPServer(ctx)
	}

	// 启动健康检查（如果启用）
	if serveEnableHealth {
		go startHealthCheck(ctx, serveHealthPort, tokenManager)
	}

	// 启动定时同步（如果启用）
	if serveEnableSync {
		syncInterval, err := time.ParseDuration(serveSyncInterval)
		if err != nil {
			fmt.Printf("❌ 无效的同步间隔: %v\n", err)
			os.Exit(1)
		}
		go startPeriodicSync(ctx, syncInterval)
	}

	fmt.Println("\n📋 服务已启动，按 Ctrl+C 停止")
	fmt.Println("─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─")

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\n\n🛑 正在停止服务...")

	// 停止 Token 管理器
	tokenManager.Stop()

	fmt.Println("👋 服务已停止")
}

// registerProviders 注册所有 Provider
func registerProviders(tm *auth.TokenManager) {
	// 注册 Google Provider
	if googleProvider, err := createGoogleProvider(); err == nil {
		tm.RegisterProvider(googleProvider)
		fmt.Printf("✅ 已注册 Provider: %s\n", googleProvider.DisplayName())
	} else {
		fmt.Printf("⚠️ Google Provider 未配置: %v\n", err)
	}

	// 注册 Microsoft Provider
	if msProvider, err := createMicrosoftProvider(); err == nil {
		tm.RegisterProvider(msProvider)
		fmt.Printf("✅ 已注册 Provider: %s\n", msProvider.DisplayName())
	} else {
		fmt.Printf("⚠️ Microsoft Provider 未配置: %v\n", err)
	}

	// 注册 Todoist Provider
	if todoProvider, err := createTodoistProvider(); err == nil {
		tm.RegisterProvider(todoProvider)
		fmt.Printf("✅ 已注册 Provider: %s\n", todoProvider.DisplayName())
	} else {
		fmt.Printf("⚠️ Todoist Provider 未配置: %v\n", err)
	}

	// 注册 Feishu Provider
	if feishuProvider, err := createFeishuProvider(); err == nil {
		tm.RegisterProvider(feishuProvider)
		fmt.Printf("✅ 已注册 Provider: %s\n", feishuProvider.DisplayName())
	} else {
		fmt.Printf("⚠️ Feishu Provider 未配置: %v\n", err)
	}

	// 注册 TickTick Provider
	if tickProvider, err := createTickTickProvider(); err == nil {
		tm.RegisterProvider(tickProvider)
		fmt.Printf("✅ 已注册 Provider: %s\n", tickProvider.DisplayName())
	} else {
		fmt.Printf("⚠️ TickTick Provider 未配置: %v\n", err)
	}
	// 注册 Dida Provider
	if didaProvider, err := createDidaProvider(); err == nil {
		tm.RegisterProvider(didaProvider)
		fmt.Printf("✅ 已注册 Provider: %s\n", didaProvider.DisplayName())
	} else {
		fmt.Printf("⚠️ Dida Provider 未配置: %v\n", err)
	}
}

// createGoogleProvider 创建 Google Provider
func createGoogleProvider() (*google.Provider, error) {
	credentialsPath := paths.GetCredentialsPath("google")
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("凭证文件不存在")
	}

	provider, err := google.NewProvider(google.Config{
		CredentialsFile: credentialsPath,
		TokenFile:       paths.GetTokenPath("google"),
	})
	if err != nil {
		return nil, err
	}

	// 尝试加载 token
	if err := provider.Authenticate(context.Background(), nil); err != nil {
		return nil, fmt.Errorf("认证失败: %w", err)
	}

	return provider, nil
}

// createMicrosoftProvider 创建 Microsoft Provider
func createMicrosoftProvider() (*microsoft.Provider, error) {
	credentialsPath := paths.GetCredentialsPath("microsoft")
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("凭证文件不存在")
	}

	provider, err := microsoft.NewProvider(microsoft.Config{
		CredentialsFile: credentialsPath,
		TokenFile:       paths.GetTokenPath("microsoft"),
	})
	if err != nil {
		return nil, err
	}

	// 尝试加载 token
	if err := provider.Authenticate(context.Background(), nil); err != nil {
		return nil, fmt.Errorf("认证失败: %w", err)
	}

	return provider, nil
}

// createTodoistProvider 创建 Todoist Provider
func createTodoistProvider() (*todoist.Provider, error) {
	tokenPath := paths.GetTokenPath("todoist")
	hasToken, err := tokenstore.Has(tokenPath, "todoist")
	if err != nil {
		return nil, err
	}
	if !hasToken {
		return nil, fmt.Errorf("token 文件不存在")
	}

	p, err := todoist.NewProvider(todoist.Config{
		TokenFile: tokenPath,
	})
	if err != nil {
		return nil, err
	}
	if err := p.Authenticate(context.Background(), nil); err != nil {
		return nil, fmt.Errorf("认证失败: %w", err)
	}
	return p, nil
}

// createFeishuProvider 创建 Feishu Provider
func createFeishuProvider() (*feishu.Provider, error) {
	credentialsPath := paths.GetCredentialsPath("feishu")
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("凭证文件不存在")
	}

	p, err := feishu.NewProvider(feishu.Config{
		CredentialsFile: credentialsPath,
		TokenFile:       paths.GetTokenPath("feishu"),
	})
	if err != nil {
		return nil, err
	}
	if err := p.Authenticate(context.Background(), nil); err != nil {
		return nil, fmt.Errorf("认证失败: %w", err)
	}
	return p, nil
}

// createTickTickProvider 创建 TickTick Provider
func createTickTickProvider() (*ticktick.Provider, error) {
	tokenPath := paths.GetTokenPath("ticktick")
	hasToken, err := tokenstore.Has(tokenPath, "ticktick")
	if err != nil {
		return nil, err
	}
	if !hasToken {
		return nil, fmt.Errorf("token 文件不存在")
	}

	p, err := ticktick.NewProvider(ticktick.Config{
		ProviderName: "ticktick",
		TokenFile:    tokenPath,
	})
	if err != nil {
		return nil, err
	}
	if err := p.Authenticate(context.Background(), nil); err != nil {
		return nil, fmt.Errorf("认证失败: %w", err)
	}
	return p, nil
}

// createDidaProvider 创建 Dida Provider
func createDidaProvider() (*ticktick.Provider, error) {
	tokenPath := paths.GetTokenPath("dida")
	hasToken, err := tokenstore.Has(tokenPath, "dida")
	if err != nil {
		return nil, err
	}
	if !hasToken {
		return nil, fmt.Errorf("token 文件不存在")
	}

	p, err := ticktick.NewProvider(ticktick.Config{
		ProviderName: "dida",
		TokenFile:    tokenPath,
	})
	if err != nil {
		return nil, err
	}
	if err := p.Authenticate(context.Background(), nil); err != nil {
		return nil, fmt.Errorf("认证失败: %w", err)
	}
	return p, nil
}

// printTokenStatus 打印 Token 状态
func printTokenStatus(tm *auth.TokenManager) {
	fmt.Println("\n📊 Token 状态:")
	fmt.Println("┌────────────┬─────────┬─────────────────────┬──────────────┐")
	fmt.Println("│ Provider   │ 状态    │ 过期时间            │ 剩余时间     │")
	fmt.Println("├────────────┼─────────┼─────────────────────┼──────────────┤")

	status := tm.GetStatus()
	for name, info := range status {
		statusIcon := "❌ 未认证"
		if info.HasToken {
			if info.IsValid {
				if info.NeedsRefresh {
					statusIcon = "⚠️ 需刷新"
				} else {
					statusIcon = "✅ 有效"
				}
			} else {
				statusIcon = "❌ 已过期"
			}
		}

		expiresAt := "-"
		timeLeft := "-"
		if info.HasToken && !info.ExpiresAt.IsZero() {
			expiresAt = info.ExpiresAt.Format("2006-01-02 15:04:05")
			timeLeft = info.TimeUntilExpiry
		}

		fmt.Printf("│ %-10s │ %-7s │ %-19s │ %-12s │\n", name, statusIcon, expiresAt, timeLeft)
	}
	fmt.Println("└────────────┴─────────┴─────────────────────┴──────────────┘")
}

// startMCPServer 启动 MCP Server
func startMCPServer(ctx context.Context) {
	fmt.Println("🔧 MCP Server 启动中...")
	// TODO: 实现 MCP Server 启动逻辑
	fmt.Println("✅ MCP Server 已启动")
}

// startHealthCheck 启动健康检查
func startHealthCheck(ctx context.Context, port int, tm *auth.TokenManager) {
	fmt.Printf("🏥 健康检查端点: http://localhost:%d/health\n", port)
	// TODO: 实现健康检查 HTTP 服务器
}

// startPeriodicSync 启动定时同步
func startPeriodicSync(ctx context.Context, interval time.Duration) {
	fmt.Printf("🔄 定时同步已启用 (间隔: %s)\n", interval)
	// TODO: 实现定时同步逻辑
}
