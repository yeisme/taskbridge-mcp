// Package microsoft provides Microsoft To Do provider implementation
package microsoft

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yeisme/taskbridge/pkg/tokenstore"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

// OAuthConfig OAuth2 配置
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	TenantID     string
	Scopes       []string
	TokenFile    string
}

// OAuth2Client OAuth2 客户端
type OAuth2Client struct {
	config       *oauth2.Config
	tenantID     string
	token        *oauth2.Token
	tokenFile    string
	codeVerifier string
	mu           sync.RWMutex
}

// DefaultScopes 默认权限范围
var DefaultScopes = []string{
	"https://graph.microsoft.com/Tasks.ReadWrite",
	"https://graph.microsoft.com/User.Read",
	"offline_access",
}

// NewOAuth2Client 创建 OAuth2 客户端
func NewOAuth2Client(cfg *OAuthConfig) *OAuth2Client {
	tenantID := cfg.TenantID
	if tenantID == "" {
		tenantID = "common" // 使用 common 支持所有账户类型
	}

	// 默认重定向 URL
	redirectURL := cfg.RedirectURL
	if redirectURL == "" {
		redirectURL = "http://localhost:8080/callback"
	}

	// 合并权限范围
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = DefaultScopes
	}

	// 创建 OAuth2 配置
	oauthConfig := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  redirectURL,
		Scopes:       scopes,
		Endpoint:     microsoft.AzureADEndpoint(tenantID),
	}

	return &OAuth2Client{
		config:    oauthConfig,
		tenantID:  tenantID,
		tokenFile: cfg.TokenFile,
	}
}

// AuthURL 生成授权 URL（带 PKCE）
func (c *OAuth2Client) AuthURL() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 生成 PKCE code_verifier
	c.codeVerifier = generateCodeVerifier()

	// 生成 code_challenge
	codeChallenge := generateCodeChallenge(c.codeVerifier)

	// 构建授权 URL
	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("prompt", "select_account"),
		oauth2.SetAuthURLParam("response_mode", "query"),
	}

	return c.config.AuthCodeURL("state", opts...)
}

// SetTokenFile 设置 token 文件路径
func (c *OAuth2Client) SetTokenFile(path string) {
	c.tokenFile = path
}

// ExchangeCode 交换授权码获取 token
func (c *OAuth2Client) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	c.mu.RLock()
	verifier := c.codeVerifier
	c.mu.RUnlock()

	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_verifier", verifier),
	}

	token, err := c.config.Exchange(ctx, code, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	c.mu.Lock()
	c.token = token
	c.mu.Unlock()

	// 保存 token
	if err := c.SaveToken(); err != nil {
		// 记录错误但不中断流程
		fmt.Fprintf(os.Stderr, "Warning: failed to save token: %v\n", err)
	}

	return token, nil
}

// StartAuthServer 启动本地认证服务器
func (c *OAuth2Client) StartAuthServer(ctx context.Context, port int) (*oauth2.Token, error) {
	// 解析重定向 URL 获取端口
	redirectURL, err := url.Parse(c.config.RedirectURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redirect URL: %w", err)
	}

	if port > 0 {
		// 更新端口
		redirectURL.Host = fmt.Sprintf("localhost:%d", port)
		c.config.RedirectURL = redirectURL.String()
	}

	// 创建监听器
	listener, err := net.Listen("tcp", redirectURL.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to start listener: %w", err)
	}
	defer func() {
		_ = listener.Close()
	}()

	// 生成授权 URL
	authURL := c.AuthURL()

	// 打印授权 URL
	fmt.Printf("\n请在浏览器中打开以下链接进行授权:\n\n%s\n\n", authURL)
	fmt.Println("等待授权回调...")

	// 创建回调处理
	resultChan := make(chan *oauth2.Token, 1)
	errChan := make(chan error, 1)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 处理回调
			if r.URL.Path != redirectURL.Path {
				http.NotFound(w, r)
				return
			}

			// 检查错误
			if errParam := r.URL.Query().Get("error"); errParam != "" {
				errDesc := r.URL.Query().Get("error_description")
				errChan <- fmt.Errorf("%s: %s", errParam, errDesc)
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, "授权失败: %s - %s", errParam, errDesc)
				return
			}

			// 获取授权码
			code := r.URL.Query().Get("code")
			if code == "" {
				errChan <- fmt.Errorf("no authorization code in callback")
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, "授权失败: 未收到授权码")
				return
			}

			// 交换 token
			token, err := c.ExchangeCode(ctx, code)
			if err != nil {
				errChan <- err
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "授权失败: %v", err)
				return
			}

			// 返回成功
			resultChan <- token
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `
<!DOCTYPE html>
<html>
<head>
    <title>授权成功</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
        .success { color: #4CAF50; font-size: 24px; }
    </style>
</head>
<body>
    <div class="success">✓ 授权成功！</div>
    <p>您可以关闭此窗口并返回终端。</p>
    <script>setTimeout(function() { window.close(); }, 3000);</script>
</body>
</html>
`)
		}),
	}

	// 启动服务器
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// 等待结果
	select {
	case token := <-resultChan:
		// 关闭服务器
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return token, nil
	case err := <-errChan:
		_ = server.Shutdown(ctx)
		return nil, err
	case <-ctx.Done():
		_ = server.Shutdown(ctx)
		return nil, ctx.Err()
	}
}

// ValidToken 获取有效的 token（必要时刷新）
func (c *OAuth2Client) ValidToken(ctx context.Context) (*oauth2.Token, error) {
	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()

	if token == nil {
		// 尝试从文件加载
		if err := c.LoadToken(); err != nil {
			return nil, fmt.Errorf("no token available: %w", err)
		}
		c.mu.RLock()
		token = c.token
		c.mu.RUnlock()
	}

	// 检查 token 是否有效
	if token.Valid() {
		return token, nil
	}

	// 刷新 token
	return c.RefreshToken(ctx)
}

// RefreshToken 刷新 token
func (c *OAuth2Client) RefreshToken(ctx context.Context) (*oauth2.Token, error) {
	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()

	if token == nil {
		return nil, fmt.Errorf("no token to refresh")
	}

	// 使用 refresh token 获取新 token
	newToken, err := c.config.TokenSource(ctx, token).Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	c.mu.Lock()
	c.token = newToken
	c.mu.Unlock()

	// 保存新 token
	if err := c.SaveToken(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save refreshed token: %v\n", err)
	}

	return newToken, nil
}

// LoadToken 从文件加载 token
func (c *OAuth2Client) LoadToken() error {
	if c.tokenFile == "" {
		return fmt.Errorf("token file not specified")
	}

	var token oauth2.Token
	if err := tokenstore.Load(c.tokenFile, "microsoft", &token); err != nil {
		return fmt.Errorf("failed to load token: %w", err)
	}

	c.mu.Lock()
	c.token = &token
	c.mu.Unlock()

	return nil
}

// SaveToken 保存 token 到文件
func (c *OAuth2Client) SaveToken() error {
	if c.tokenFile == "" {
		return nil
	}

	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()

	if token == nil {
		return fmt.Errorf("no token to save")
	}

	if err := tokenstore.Save(c.tokenFile, "microsoft", token); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	return nil
}

// HTTPClient 获取配置好的 HTTP 客户端
func (c *OAuth2Client) HTTPClient(ctx context.Context) (*http.Client, error) {
	token, err := c.ValidToken(ctx)
	if err != nil {
		return nil, err
	}

	return c.config.Client(ctx, token), nil
}

// IsAuthenticated 检查是否已认证
func (c *OAuth2Client) IsAuthenticated() bool {
	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()

	if token == nil {
		// 尝试加载
		if err := c.LoadToken(); err != nil {
			return false
		}
		c.mu.RLock()
		token = c.token
		c.mu.RUnlock()
	}

	return token != nil && token.Valid()
}

// RevokeToken 撤销 token（删除本地文件）
func (c *OAuth2Client) RevokeToken() error {
	c.mu.Lock()
	c.token = nil
	c.mu.Unlock()

	if c.tokenFile != "" {
		if err := tokenstore.Delete(c.tokenFile, "microsoft"); err != nil {
			return fmt.Errorf("failed to remove token: %w", err)
		}
	}

	return nil
}

// GetTokenInfo 获取 token 信息
func (c *OAuth2Client) GetTokenInfo() map[string]interface{} {
	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()

	if token == nil {
		return map[string]interface{}{
			"authenticated": false,
		}
	}

	return map[string]interface{}{
		"authenticated": true,
		"token_type":    token.TokenType,
		"expiry":        token.Expiry,
		"has_refresh":   token.RefreshToken != "",
	}
}

// ================ 辅助函数 ================

// generateCodeVerifier 生成 PKCE code_verifier
func generateCodeVerifier() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// 如果随机数生成失败，使用时间戳作为后备
		b = []byte(fmt.Sprintf("%d", time.Now().UnixNano()))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// generateCodeChallenge 生成 PKCE code_challenge
func generateCodeChallenge(verifier string) string {
	h := sha256.New()
	h.Write([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// LoadCredentials 从 JSON 文件加载凭据
func LoadCredentials(credentialsFile string) (*OAuth2Client, error) {
	data, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	var creds struct {
		ClientID     string   `json:"client_id"`
		ClientSecret string   `json:"client_secret"`
		TenantID     string   `json:"tenant_id"`
		RedirectURL  string   `json:"redirect_url"`
		Scopes       []string `json:"scopes"`
		TokenFile    string   `json:"token_file"`
	}

	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	return NewOAuth2Client(&OAuthConfig{
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		TenantID:     creds.TenantID,
		RedirectURL:  creds.RedirectURL,
		Scopes:       creds.Scopes,
		TokenFile:    creds.TokenFile,
	}), nil
}

// ParseScopes 解析权限范围字符串
func ParseScopes(scopeStr string) []string {
	if scopeStr == "" {
		return DefaultScopes
	}

	scopes := strings.Split(scopeStr, " ")
	result := make([]string, 0, len(scopes))
	for _, s := range scopes {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}

	if len(result) == 0 {
		return DefaultScopes
	}

	return result
}

// ReadPassword 从终端读取密码（隐藏输入）
func ReadPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	defer fmt.Println()

	// 尝试从 /dev/tty 读取（Unix）
	if tty, err := os.Open("/dev/tty"); err == nil {
		defer tty.Close()
		data, err := io.ReadAll(tty)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	}

	// 后备方案：从 stdin 读取
	var input string
	if _, err := fmt.Scanln(&input); err != nil {
		return "", err
	}
	return input, nil
}
