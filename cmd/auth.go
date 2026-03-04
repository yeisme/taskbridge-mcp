package cmd

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/internal/provider/feishu"
	"github.com/yeisme/taskbridge/internal/provider/google"
	"github.com/yeisme/taskbridge/internal/provider/microsoft"
	"github.com/yeisme/taskbridge/internal/provider/ticktick"
	"github.com/yeisme/taskbridge/internal/provider/todoist"
	"github.com/yeisme/taskbridge/pkg/paths"
	"github.com/yeisme/taskbridge/pkg/tokenstore"
	"github.com/yeisme/taskbridge/pkg/ui"
)

// authCmd 认证命令
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "认证管理",
	Long: `管理 Todo Provider 的认证状态。

支持的 Provider:
  - google: Google Tasks API
  - microsoft: Microsoft Todo
  - feishu: 飞书任务
  - ticktick: TickTick
  - dida: 滴答清单（国内）
  - todoist: Todoist

子命令:
  login <provider>    登录指定 Provider
  logout <provider>   登出指定 Provider
  status              查看所有 Provider 的认证状态
  show <provider>     查看单个 Provider 的认证详情
  refresh <provider>  刷新指定 Provider 的 token

示例:
  taskbridge auth login google
  taskbridge auth status
  taskbridge auth show ms
  taskbridge auth logout google`,
}

// authLoginCmd 登录命令
var authLoginCmd = &cobra.Command{
	Use:   "login <provider>",
	Short: "登录指定 Provider",
	Long: `登录指定的 Todo Provider 进行 OAuth2 认证。

支持的 Provider:
  - google: Google Tasks API
  - microsoft: Microsoft Todo
  - feishu: 飞书任务
  - ticktick: TickTick（国际）
  - dida: 滴答清单（国内）
  - todoist: Todoist

示例:
  taskbridge auth login google
  taskbridge auth login google --manual  # 手动输入授权码`,
	Args: cobra.ExactArgs(1),
	Run:  runAuthLogin,
}

// authLogoutCmd 登出命令
var authLogoutCmd = &cobra.Command{
	Use:   "logout <provider>",
	Short: "登出指定 Provider",
	Long: `登出指定的 Todo Provider，删除本地存储的 token。

示例:
  taskbridge auth logout google`,
	Args: cobra.ExactArgs(1),
	Run:  runAuthLogout,
}

// authStatusCmd 状态命令
var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看所有 Provider 的认证状态",
	Long: `显示所有已配置 Provider 的认证状态。

示例:
  taskbridge auth status`,
	Run: runAuthStatus,
}

// authShowCmd 详情命令
var authShowCmd = &cobra.Command{
	Use:   "show <provider>",
	Short: "查看单个 Provider 的认证详情",
	Long: `显示指定 Provider 的认证详情，支持简写。

示例:
  taskbridge auth show microsoft
  taskbridge auth show ms`,
	Args: cobra.ExactArgs(1),
	Run:  runAuthShow,
}

// authRefreshCmd 刷新命令
var authRefreshCmd = &cobra.Command{
	Use:   "refresh <provider>",
	Short: "刷新指定 Provider 的 token",
	Long: `刷新指定 Provider 的 OAuth2 token。

示例:
  taskbridge auth refresh google`,
	Args: cobra.ExactArgs(1),
	Run:  runAuthRefresh,
}

var (
	// 登录选项
	manualAuth bool
)

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authShowCmd)
	authCmd.AddCommand(authRefreshCmd)

	// 登录命令选项
	authLoginCmd.Flags().BoolVar(&manualAuth, "manual", false, "手动输入授权码（用于无浏览器环境）")
}

// runAuthLogin 执行登录
func runAuthLogin(cmd *cobra.Command, args []string) {
	// 解析 Provider 名称（支持简写）
	providerName := provider.ResolveProviderName(args[0])

	// 检查 Provider 是否有效
	if !provider.IsValidProvider(providerName) {
		fmt.Printf("❌ 不支持的 Provider: %s\n", args[0])
		fmt.Println("支持的 Provider: google (g), microsoft (ms), feishu, ticktick (tick), dida (ticktick_cn), todoist (todo)")
		os.Exit(1)
	}

	switch providerName {
	case "google":
		loginGoogle()
	case "microsoft":
		loginMicrosoft()
	case "feishu":
		loginFeishu()
	case "ticktick":
		loginTickTick()
	case "dida":
		loginDida()
	case "todoist":
		loginTodoist()
	default:
		def, _ := provider.GetProviderDefinition(providerName)
		fmt.Printf("❌ %s 尚未实现登录功能\n", def.DisplayName)
		os.Exit(1)
	}
}

// loginGoogle 登录 Google
func loginGoogle() {
	fmt.Println("🔐 开始 Google Tasks OAuth2 认证...")

	// 确保凭证目录存在
	if err := paths.EnsureCredentialsDir(); err != nil {
		fmt.Printf("❌ 创建凭证目录失败: %v\n", err)
		os.Exit(1)
	}

	// 检查凭证文件
	credentialsPath := paths.GetCredentialsPath("google")
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		fmt.Printf("❌ 凭证文件不存在: %s\n", credentialsPath)
		fmt.Println("\n请按以下步骤操作:")
		fmt.Println("1. 访问 Google Cloud Console: https://console.cloud.google.com/")
		fmt.Println("2. 创建项目并启用 Google Tasks API")
		fmt.Println("3. 配置 OAuth2 同意屏幕")
		fmt.Println("4. 创建 OAuth2 凭证（桌面应用）")
		fmt.Printf("5. 下载凭证文件并保存到: %s\n", credentialsPath)
		os.Exit(1)
	}

	// 加载凭证
	client, err := google.LoadCredentials(credentialsPath)
	if err != nil {
		fmt.Printf("❌ 加载凭证失败: %v\n", err)
		os.Exit(1)
	}

	// 设置 token 文件路径
	tokenPath := paths.GetTokenPath("google")
	client.SetTokenFile(tokenPath)

	if manualAuth {
		// 生成授权 URL
		state := fmt.Sprintf("taskbridge-%d", time.Now().Unix())
		authURL := client.GetAuthURL(state)

		fmt.Println("\n📋 请在浏览器中打开以下链接进行授权:")
		fmt.Println()
		fmt.Printf("   %s\n", authURL)
		fmt.Println()

		// 手动输入授权码模式（支持直接粘贴回调 URL）
		fmt.Print("请输入授权码（或粘贴完整回调 URL）: ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("❌ 读取授权码失败: %v\n", err)
			os.Exit(1)
		}
		code, err := extractGoogleAuthCode(input)
		if err != nil {
			fmt.Printf("❌ 授权码格式错误: %v\n", err)
			fmt.Println("请复制浏览器回调地址里 `code=` 后面的完整值，或直接粘贴完整回调 URL。")
			os.Exit(1)
		}

		// 交换 token
		token, err := client.Exchange(context.Background(), code)
		if err != nil {
			fmt.Printf("❌ 交换 token 失败: %v\n", err)
			os.Exit(1)
		}

		// 保存 token
		if err := client.SaveToken(token); err != nil {
			fmt.Printf("❌ 保存 token 失败: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n✅ Google Tasks 认证成功!")
		fmt.Printf("📁 Token 已保存到: %s\n", tokenPath)
	} else {
		// 自动模式：本地启动回调服务完成认证
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		token, err := client.StartAuthServer(ctx, 0)
		if err != nil {
			fmt.Printf("❌ 自动认证失败: %v\n", err)
			fmt.Println("可改用手动模式: taskbridge auth login google --manual")
			fmt.Println("若你使用 Google Desktop 凭证且 redirect_uri 为 http://localhost，请确保本机 80 端口可监听。")
			os.Exit(1)
		}

		fmt.Println("\n✅ Google Tasks 自动认证成功!")
		fmt.Printf("📁 Token 已保存到: %s\n", tokenPath)
		if !token.Expiry.IsZero() {
			fmt.Printf("⏰ 过期时间: %s\n", token.Expiry.Format("2006-01-02 15:04:05"))
		}
	}
}

func extractGoogleAuthCode(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("输入为空")
	}

	// 支持直接粘贴完整回调 URL。
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		parsedURL, err := url.Parse(trimmed)
		if err != nil {
			return "", fmt.Errorf("无法解析 URL: %w", err)
		}
		code := strings.TrimSpace(parsedURL.Query().Get("code"))
		if code == "" {
			return "", fmt.Errorf("URL 中未找到 code 参数")
		}
		return code, nil
	}

	// 支持粘贴 query 字符串（例如 code=xxx&scope=...）。
	if strings.Contains(trimmed, "code=") {
		query := trimmed
		if idx := strings.Index(trimmed, "?"); idx >= 0 && idx < len(trimmed)-1 {
			query = trimmed[idx+1:]
		}
		values, err := url.ParseQuery(query)
		if err == nil {
			code := strings.TrimSpace(values.Get("code"))
			if code != "" {
				return code, nil
			}
		}
	}

	if strings.HasPrefix(trimmed, "taskbridge-") || looksLikeNumericState(trimmed) {
		return "", fmt.Errorf("看起来输入的是 state，不是授权 code")
	}

	return trimmed, nil
}

func looksLikeNumericState(v string) bool {
	if len(v) < 8 || len(v) > 16 {
		return false
	}
	for _, ch := range v {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// runAuthLogout 执行登出
func runAuthLogout(cmd *cobra.Command, args []string) {
	// 解析 Provider 名称（支持简写）
	providerName := provider.ResolveProviderName(args[0])

	// 检查 Provider 是否有效
	if !provider.IsValidProvider(providerName) {
		fmt.Printf("❌ 不支持的 Provider: %s\n", args[0])
		os.Exit(1)
	}

	tokenPath := paths.GetTokenPath(providerName)
	hasToken, err := tokenstore.Has(tokenPath, providerName)
	if err != nil {
		fmt.Printf("❌ 读取 token 失败: %v\n", err)
		os.Exit(1)
	}
	if !hasToken {
		fmt.Printf("ℹ️ %s 未登录\n", providerName)
		return
	}

	if err := tokenstore.Delete(tokenPath, providerName); err != nil {
		fmt.Printf("❌ 登出失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ %s 已登出\n", providerName)
}

// runAuthStatus 执行状态查询
func runAuthStatus(cmd *cobra.Command, args []string) {
	printAuthStatusTable()
}

// runAuthShow 执行单个 Provider 详情查询
func runAuthShow(cmd *cobra.Command, args []string) {
	providerName := provider.ResolveProviderName(args[0])
	if !provider.IsValidProvider(providerName) {
		fmt.Printf("❌ 不支持的 Provider: %s\n", args[0])
		fmt.Println("支持的 Provider: google (g), microsoft (ms), feishu, ticktick (tick), dida (ticktick_cn), todoist (todo)")
		os.Exit(1)
	}

	snapshot := getProviderAuthSnapshot(providerName)

	authenticated := "否"
	if snapshot.Authenticated {
		authenticated = "是"
	}

	valid := "未知"
	if snapshot.Valid != nil {
		if *snapshot.Valid {
			valid = "是"
		} else {
			valid = "否"
		}
	}

	pairs := map[string]string{
		"Provider": snapshot.Provider,
		"显示名称":     snapshot.DisplayName,
		"简写":       snapshot.ShortName,
		"Token 文件": snapshot.TokenPath,
		"状态":       snapshot.StatusText,
		"已认证":      authenticated,
		"Token 有效": valid,
		"过期时间":     snapshot.ExpiresAt,
		"建议操作":     snapshot.NextAction,
	}

	fmt.Println()
	fmt.Println(ui.KeyValueCard("🔐 Auth Detail", pairs))
	fmt.Println()
}

// runAuthRefresh 执行 token 刷新
func runAuthRefresh(cmd *cobra.Command, args []string) {
	// 解析 Provider 名称（支持简写）
	providerName := provider.ResolveProviderName(args[0])

	// 检查 Provider 是否有效
	if !provider.IsValidProvider(providerName) {
		fmt.Printf("❌ 不支持的 Provider: %s\n", args[0])
		os.Exit(1)
	}

	switch providerName {
	case "google":
		refreshGoogleToken()
	case "microsoft":
		refreshMicrosoftToken()
	case "feishu":
		refreshFeishuToken()
	case "ticktick":
		refreshTickTickToken()
	case "dida":
		refreshDidaToken()
	case "todoist":
		refreshTodoistToken()
	default:
		def, _ := provider.GetProviderDefinition(providerName)
		fmt.Printf("❌ %s 尚未实现 token 刷新功能\n", def.DisplayName)
		os.Exit(1)
	}
}

// refreshGoogleToken 刷新 Google token
func refreshGoogleToken() {
	client, err := google.NewOAuth2ClientFromHome()
	if err != nil {
		fmt.Printf("❌ 加载 Google OAuth2 客户端失败: %v\n", err)
		os.Exit(1)
	}

	token, err := client.RefreshToken(context.Background())
	if err != nil {
		fmt.Printf("❌ 刷新 token 失败: %v\n", err)
		os.Exit(1)
	}

	if err := client.SaveToken(token); err != nil {
		fmt.Printf("❌ 保存 token 失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Google token 已刷新")
}

// loginMicrosoft 登录 Microsoft
func loginMicrosoft() {
	fmt.Println("🔐 开始 Microsoft To Do OAuth2 认证...")

	// 确保凭证目录存在
	if err := paths.EnsureCredentialsDir(); err != nil {
		fmt.Printf("❌ 创建凭证目录失败: %v\n", err)
		os.Exit(1)
	}

	// 检查凭证文件
	credentialsPath := paths.GetCredentialsPath("microsoft")
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		fmt.Printf("❌ 凭证文件不存在: %s\n", credentialsPath)
		fmt.Println("\n请按以下步骤操作:")
		fmt.Println("1. 访问 Azure Portal: https://portal.azure.com/")
		fmt.Println("2. 注册应用程序（Azure Active Directory）")
		fmt.Println("3. 配置重定向 URI: http://localhost:8080/callback")
		fmt.Println("4. 添加 API 权限: Tasks.ReadWrite, User.Read")
		fmt.Println("5. 创建客户端密钥")
		fmt.Printf("6. 创建凭证文件并保存到: %s\n", credentialsPath)
		fmt.Println("\n凭证文件格式:")
		fmt.Println(`{
	 "client_id": "你的应用ID",
	 "client_secret": "你的客户端密钥",
	 "tenant_id": "common",
	 "redirect_url": "http://localhost:8080/callback"
}`)
		os.Exit(1)
	}

	// 加载凭证
	oauthClient, err := microsoft.LoadCredentials(credentialsPath)
	if err != nil {
		fmt.Printf("❌ 加载凭证失败: %v\n", err)
		os.Exit(1)
	}

	// 设置 token 文件路径
	tokenPath := paths.GetTokenPath("microsoft")
	oauthClient.SetTokenFile(tokenPath)

	// 启动认证服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	token, err := oauthClient.StartAuthServer(ctx, 8080)
	if err != nil {
		fmt.Printf("❌ 认证失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Microsoft To Do 认证成功!")
	fmt.Printf("📁 Token 已保存到: %s\n", tokenPath)
	fmt.Printf("🔑 Token 类型: %s\n", token.TokenType)
}

// refreshMicrosoftToken 刷新 Microsoft token
func refreshMicrosoftToken() {
	credentialsPath := paths.GetCredentialsPath("microsoft")
	tokenPath := paths.GetTokenPath("microsoft")

	oauthClient, err := microsoft.LoadCredentials(credentialsPath)
	if err != nil {
		fmt.Printf("❌ 加载 Microsoft OAuth2 客户端失败: %v\n", err)
		os.Exit(1)
	}

	oauthClient.SetTokenFile(tokenPath)

	// 加载现有 token
	if err := oauthClient.LoadToken(); err != nil {
		fmt.Printf("❌ 加载 token 失败: %v\n", err)
		os.Exit(1)
	}

	// 刷新 token
	token, err := oauthClient.RefreshToken(context.Background())
	if err != nil {
		fmt.Printf("❌ 刷新 token 失败: %v\n", err)
		os.Exit(1)
	}

	if err := oauthClient.SaveToken(); err != nil {
		fmt.Printf("❌ 保存 token 失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Microsoft token 已刷新")
	fmt.Printf("🔑 新过期时间: %s\n", token.Expiry.Format("2006-01-02 15:04:05"))
}

// loginFeishu 登录飞书
func loginFeishu() {
	fmt.Println("🔐 开始飞书 Todo OAuth2 认证...")

	if err := paths.EnsureCredentialsDir(); err != nil {
		fmt.Printf("❌ 创建凭证目录失败: %v\n", err)
		os.Exit(1)
	}

	credentialsPath := paths.GetCredentialsPath("feishu")
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		fmt.Printf("❌ 凭证文件不存在: %s\n", credentialsPath)
		fmt.Println("\n请按以下步骤操作:")
		fmt.Println("1. 访问飞书开放平台: https://open.feishu.cn/")
		fmt.Println("2. 创建自建应用并开启 Todo 相关权限")
		fmt.Println("3. 配置重定向 URL（端口需与本地回调监听一致，例如 http://127.0.0.1:3456/callback）")
		fmt.Printf("4. 创建凭证文件并保存到: %s\n", credentialsPath)
		fmt.Println("\n凭证文件格式:")
		fmt.Println(`{
  "app_id": "cli_xxx",
  "app_secret": "xxxx",
  "redirect_url": "http://127.0.0.1:3456/callback",
  "scopes": ["task:tasklist:read","task:tasklist:write","task:task:read","task:task:write"]
}`)
		os.Exit(1)
	}

	oauthClient, err := feishu.LoadCredentials(credentialsPath)
	if err != nil {
		fmt.Printf("❌ 加载凭证失败: %v\n", err)
		os.Exit(1)
	}

	tokenPath := paths.GetTokenPath("feishu")
	oauthClient.SetTokenFile(tokenPath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 端口由凭证中的 redirect_url 决定（不强制使用 8080）
	token, err := oauthClient.StartAuthServer(ctx, 0)
	if err != nil {
		fmt.Printf("❌ 认证失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ 飞书 Todo 认证成功!")
	fmt.Printf("📁 Token 已保存到: %s\n", tokenPath)
	fmt.Printf("🔑 Token 类型: %s\n", token.TokenType)
}

// refreshFeishuToken 刷新飞书 token
func refreshFeishuToken() {
	credentialsPath := paths.GetCredentialsPath("feishu")
	tokenPath := paths.GetTokenPath("feishu")

	oauthClient, err := feishu.LoadCredentials(credentialsPath)
	if err != nil {
		fmt.Printf("❌ 加载飞书 OAuth2 客户端失败: %v\n", err)
		os.Exit(1)
	}

	oauthClient.SetTokenFile(tokenPath)
	if err := oauthClient.LoadToken(); err != nil {
		fmt.Printf("❌ 加载 token 失败: %v\n", err)
		os.Exit(1)
	}

	token, err := oauthClient.RefreshToken(context.Background())
	if err != nil {
		fmt.Printf("❌ 刷新 token 失败: %v\n", err)
		os.Exit(1)
	}

	if err := oauthClient.SaveToken(); err != nil {
		fmt.Printf("❌ 保存 token 失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ 飞书 token 已刷新")
	fmt.Printf("🔑 新过期时间(秒): %d\n", token.ExpiresIn)
}

// loginTickTick 登录 TickTick（API Token）
func loginTickTick() {
	loginTickStyleProvider("ticktick")
}

func loginDida() {
	loginTickStyleProvider("dida")
}

func loginTickStyleProvider(providerName string) {
	displayName := "TickTick"
	tokenHint := "tp_"
	if providerName == "dida" {
		displayName = "Dida365"
		tokenHint = "dp_"
	}

	fmt.Printf("🔐 开始 %s API Token 认证...\n", displayName)

	if err := paths.EnsureCredentialsDir(); err != nil {
		fmt.Printf("❌ 创建凭证目录失败: %v\n", err)
		os.Exit(1)
	}

	tokenPath := paths.GetTokenPath(providerName)
	fmt.Printf("\n请按以下步骤获取 %s API Token:\n", displayName)
	if providerName == "dida" {
		fmt.Println("1. 打开 dida365.com 并登录开发者平台或 OpenAPI 管理页")
	} else {
		fmt.Println("1. 打开 TickTick 开发者平台并登录")
	}
	fmt.Println("2. 创建或查看个人 API Token")
	fmt.Printf("3. 复制 token（通常以 `%s` 开头）\n", tokenHint)
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("请输入 %s API Token: ", displayName)
	apiToken, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("❌ 读取 API Token 失败: %v\n", err)
		os.Exit(1)
	}
	apiToken = strings.TrimSpace(apiToken)
	if apiToken == "" {
		fmt.Println("❌ API Token 不能为空")
		os.Exit(1)
	}

	p, err := ticktick.NewProvider(ticktick.Config{
		ProviderName: providerName,
		Token:        apiToken,
		TokenFile:    tokenPath,
	})
	if err != nil {
		fmt.Printf("❌ 初始化 %s Provider 失败: %v\n", displayName, err)
		os.Exit(1)
	}
	if err := p.Authenticate(context.Background(), map[string]interface{}{
		"token":    apiToken,
		"provider": providerName,
	}); err != nil {
		fmt.Printf("❌ %s 认证失败: %v\n", displayName, err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ %s 认证成功!\n", displayName)
	fmt.Printf("📁 Token 已保存到: %s\n", tokenPath)
}

// refreshTickTickToken 刷新 TickTick token（静态 token）
func refreshTickTickToken() {
	refreshTickStyleProvider("ticktick")
}

func refreshDidaToken() {
	refreshTickStyleProvider("dida")
}

func refreshTickStyleProvider(providerName string) {
	displayName := "TickTick"
	if providerName == "dida" {
		displayName = "Dida365"
	}
	tokenPath := paths.GetTokenPath(providerName)
	hasToken, err := tokenstore.Has(tokenPath, providerName)
	if err != nil {
		fmt.Printf("❌ 读取 %s token 失败: %v\n", displayName, err)
		os.Exit(1)
	}
	if !hasToken {
		fmt.Printf("❌ %s 凭证不存在，请先执行: taskbridge auth login %s\n", displayName, providerName)
		os.Exit(1)
	}

	p, err := ticktick.NewProvider(ticktick.Config{
		ProviderName: providerName,
		TokenFile:    tokenPath,
	})
	if err != nil {
		fmt.Printf("❌ 初始化 %s Provider 失败: %v\n", displayName, err)
		os.Exit(1)
	}
	if err := p.RefreshToken(context.Background()); err != nil {
		fmt.Printf("❌ 刷新 %s token 失败: %v\n", displayName, err)
		os.Exit(1)
	}

	fmt.Printf("✅ %s token 校验通过（静态 token 无需刷新）\n", displayName)
}

// loginTodoist 登录 Todoist（API Token）
func loginTodoist() {
	fmt.Println("🔐 开始 Todoist API Token 认证...")

	if err := paths.EnsureCredentialsDir(); err != nil {
		fmt.Printf("❌ 创建凭证目录失败: %v\n", err)
		os.Exit(1)
	}

	tokenPath := paths.GetTokenPath("todoist")
	fmt.Println("\n请按以下步骤获取 Todoist API Token:")
	fmt.Println("1. 访问 https://todoist.com/app/settings/integrations/developer")
	fmt.Println("2. 复制 API Token")
	fmt.Println()

	fmt.Print("请输入 API Token: ")
	reader := bufio.NewReader(os.Stdin)
	token, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("❌ 读取 API Token 失败: %v\n", err)
		os.Exit(1)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		fmt.Println("❌ API Token 不能为空")
		os.Exit(1)
	}

	p, err := todoist.NewProvider(todoist.Config{
		APIToken:  token,
		TokenFile: tokenPath,
	})
	if err != nil {
		fmt.Printf("❌ 初始化 Todoist Provider 失败: %v\n", err)
		os.Exit(1)
	}
	if err := p.Authenticate(context.Background(), map[string]interface{}{"api_token": token}); err != nil {
		fmt.Printf("❌ Todoist 认证失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Todoist 认证成功!")
	fmt.Printf("📁 Token 已保存到: %s\n", tokenPath)
}

// refreshTodoistToken 刷新 Todoist token（静态 API Token，无需刷新）
func refreshTodoistToken() {
	fmt.Println("ℹ️ Todoist 使用静态 API Token，无需刷新。")
	fmt.Println("若 token 失效，请重新执行: taskbridge auth login todoist")
}

type AuthSnapshot struct {
	Provider      string
	DisplayName   string
	ShortName     string
	TokenPath     string
	Authenticated bool
	Valid         *bool
	StatusText    string
	ExpiresAt     string
	NextAction    string
}

type providerMeta struct {
	Name        string
	DisplayName string
	ShortName   string
}

func getAuthProviderOrder() []string {
	return []string{"google", "microsoft", "feishu", "ticktick", "dida", "todoist"}
}

func getAuthProviderMeta(name string) providerMeta {
	def, ok := provider.GetProviderDefinition(name)
	if !ok {
		return providerMeta{
			Name:        name,
			DisplayName: name,
			ShortName:   name,
		}
	}
	return providerMeta{
		Name:        def.Name,
		DisplayName: def.DisplayName,
		ShortName:   def.ShortName,
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func isProviderEnabled(providerName string) bool {
	switch providerName {
	case "google":
		return cfg.Providers.Google.Enabled
	case "microsoft":
		return cfg.Providers.Microsoft.Enabled
	case "feishu":
		return cfg.Providers.Feishu.Enabled
	case "ticktick":
		return cfg.Providers.TickTick.Enabled
	case "dida":
		return cfg.Providers.Dida.Enabled
	case "todoist":
		return cfg.Providers.Todoist.Enabled
	default:
		return false
	}
}

func getProviderAuthSnapshot(providerName string) AuthSnapshot {
	meta := getAuthProviderMeta(providerName)
	snapshot := AuthSnapshot{
		Provider:      meta.Name,
		DisplayName:   meta.DisplayName,
		ShortName:     meta.ShortName,
		TokenPath:     paths.GetTokenPath(meta.Name),
		Authenticated: false,
		Valid:         nil,
		StatusText:    "❌ Not configured",
		ExpiresAt:     "-",
		NextAction:    fmt.Sprintf("taskbridge auth login %s", meta.Name),
	}

	hasToken, err := tokenstore.Has(snapshot.TokenPath, meta.Name)
	if err != nil {
		snapshot.StatusText = "⚠️ Token error"
		snapshot.ExpiresAt = "读取失败"
		snapshot.NextAction = fmt.Sprintf("检查 token 文件: %s", snapshot.TokenPath)
		return snapshot
	}
	if !hasToken {
		if isProviderEnabled(meta.Name) {
			snapshot.StatusText = "❌ Not authenticated"
		}
		return snapshot
	}

	snapshot.Authenticated = true
	snapshot.StatusText = "✅ Connected"
	snapshot.NextAction = fmt.Sprintf("taskbridge auth logout %s", meta.Name)

	switch meta.Name {
	case "google":
		client, err := google.NewOAuth2ClientFromHome()
		if err != nil {
			// Google 客户端加载失败时，保守认为已连接但无法判定有效性
			snapshot.Valid = nil
			snapshot.ExpiresAt = "有效"
			return snapshot
		}
		info := client.GetTokenInfo()
		if info == nil {
			snapshot.Valid = nil
			snapshot.ExpiresAt = "未知"
			return snapshot
		}
		if info.Valid {
			snapshot.Valid = boolPtr(true)
			snapshot.ExpiresAt = info.Expiry.Format("2006-01-02")
			return snapshot
		}
		snapshot.Valid = boolPtr(false)
		snapshot.StatusText = "⚠️ Expired"
		snapshot.ExpiresAt = "需刷新"
		snapshot.NextAction = "taskbridge auth refresh google"
		return snapshot
	case "microsoft":
		credentialsPath := paths.GetCredentialsPath("microsoft")
		oauthClient, err := microsoft.LoadCredentials(credentialsPath)
		if err != nil {
			snapshot.Valid = nil
			snapshot.ExpiresAt = "有效"
			return snapshot
		}
		oauthClient.SetTokenFile(snapshot.TokenPath)
		if err := oauthClient.LoadToken(); err != nil {
			snapshot.Valid = nil
			snapshot.ExpiresAt = "有效"
			return snapshot
		}
		info := oauthClient.GetTokenInfo()
		if expiry, ok := info["expiry"].(time.Time); ok && !expiry.IsZero() {
			snapshot.ExpiresAt = expiry.Format("2006-01-02")
			now := time.Now()
			if expiry.After(now) {
				snapshot.Valid = boolPtr(true)
			} else {
				snapshot.Valid = boolPtr(false)
				snapshot.StatusText = "⚠️ Expired"
				snapshot.NextAction = "taskbridge auth refresh microsoft"
			}
			return snapshot
		}
		snapshot.Valid = nil
		snapshot.ExpiresAt = "有效"
		return snapshot
	default:
		snapshot.Valid = nil
		snapshot.ExpiresAt = "有效"
		return snapshot
	}
}

// getProviderStatus 获取 Provider 认证状态
// 优先检查 token 文件是否存在，而不是配置中的 Enabled 状态
func getProviderStatus(providerName string) (status, user, expires string) {
	snapshot := getProviderAuthSnapshot(providerName)
	user = "-"
	if snapshot.Authenticated {
		user = "已认证"
	}
	return snapshot.StatusText, user, snapshot.ExpiresAt
}

// printAuthStatusTable 打印认证状态表格
func printAuthStatusTable() {
	// 使用 lipgloss table 组件
	table := ui.NewTable("Provider", "简写", "状态", "Expires")

	for _, p := range getAuthProviderOrder() {
		snapshot := getProviderAuthSnapshot(p)
		table.AddRow(snapshot.DisplayName, snapshot.ShortName, snapshot.StatusText, snapshot.ExpiresAt)
	}

	fmt.Println()
	fmt.Println(table.Render())
	fmt.Println()
}
