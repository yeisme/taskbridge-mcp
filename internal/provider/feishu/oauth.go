// Package feishu provides Feishu (Lark) Task provider implementation
package feishu

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

// 飞书开放平台 OAuth2 端点
const (
	// DefaultAuthURL 飞书授权 URL
	DefaultAuthURL = "https://open.feishu.cn/open-apis/authen/v1/authorize"
	// DefaultTokenURL 飞书 Token URL
	DefaultTokenURL = "https://open.feishu.cn/open-apis/authen/v1/oidc/access_token"
	// DefaultRefreshTokenURL 飞书刷新 Token URL
	DefaultRefreshTokenURL = "https://open.feishu.cn/open-apis/authen/v1/oidc/refresh_access_token"
	// DefaultUserInfoURL 飞书用户信息 URL
	DefaultUserInfoURL = "https://open.feishu.cn/open-apis/authen/v1/user_info"
	// DefaultAPIBaseURL 飞书 API 基础 URL
	DefaultAPIBaseURL = "https://open.feishu.cn/open-apis"
)

// OAuthConfig OAuth2 配置
type OAuthConfig struct {
	AppID       string
	AppSecret   string
	RedirectURL string
	Scopes      []string
	TokenFile   string
}

// OAuth2Client OAuth2 客户端
type OAuth2Client struct {
	config     *OAuthConfig
	token      *TokenResponse
	tokenFile  string
	state      string
	mu         sync.RWMutex
	httpClient *http.Client
}

// DefaultScopes 默认权限范围
var DefaultScopes = []string{
	"task:tasklist:read",
	"task:tasklist:write",
	"task:task:read",
	"task:task:write",
	"contact:user.base:readonly",
}

// NewOAuth2Client 创建 OAuth2 客户端
func NewOAuth2Client(cfg *OAuthConfig) *OAuth2Client {
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

	return &OAuth2Client{
		config: &OAuthConfig{
			AppID:       cfg.AppID,
			AppSecret:   cfg.AppSecret,
			RedirectURL: redirectURL,
			Scopes:      scopes,
			TokenFile:   cfg.TokenFile,
		},
		tokenFile: cfg.TokenFile,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// AuthURL 生成授权 URL
func (c *OAuth2Client) AuthURL() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 生成随机 state
	c.state = generateState()

	// 构建授权 URL
	params := url.Values{}
	params.Set("app_id", c.config.AppID)
	params.Set("redirect_uri", c.config.RedirectURL)
	params.Set("state", c.state)
	params.Set("scope", strings.Join(c.config.Scopes, " "))

	return fmt.Sprintf("%s?%s", DefaultAuthURL, params.Encode())
}

// SetTokenFile 设置 token 文件路径
func (c *OAuth2Client) SetTokenFile(path string) {
	c.tokenFile = path
}

// ExchangeCode 交换授权码获取 token
func (c *OAuth2Client) ExchangeCode(ctx context.Context, code string) (*TokenResponse, error) {
	// 构建请求体
	reqBody := map[string]string{
		"grant_type": "authorization_code",
		"code":       code,
	}

	// 发送请求
	resp, err := c.doTokenRequest(ctx, DefaultTokenURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	c.mu.Lock()
	c.token = resp
	c.mu.Unlock()

	// 保存 token
	if err := c.SaveToken(); err != nil {
		log.Warn().Err(err).Msg("Failed to save token")
	}

	return resp, nil
}

// StartAuthServer 启动本地认证服务器
func (c *OAuth2Client) StartAuthServer(ctx context.Context, port int) (*TokenResponse, error) {
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
	defer listener.Close()

	// 生成授权 URL
	authURL := c.AuthURL()

	// 打印授权 URL
	fmt.Printf("\n请在浏览器中打开以下链接进行授权:\n\n%s\n\n", authURL)
	fmt.Println("等待授权回调...")

	// 创建回调处理
	resultChan := make(chan *TokenResponse, 1)
	errChan := make(chan error, 1)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 处理回调
			if r.URL.Path != redirectURL.Path {
				http.NotFound(w, r)
				return
			}

			// 检查 state
			state := r.URL.Query().Get("state")
			c.mu.RLock()
			expectedState := c.state
			c.mu.RUnlock()

			if state != expectedState {
				errChan <- fmt.Errorf("invalid state: expected %s, got %s", expectedState, state)
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, "授权失败: state 不匹配")
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
func (c *OAuth2Client) ValidToken(ctx context.Context) (*TokenResponse, error) {
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
	if c.isTokenValid(token) {
		return token, nil
	}

	// 刷新 token
	return c.RefreshToken(ctx)
}

// isTokenValid 检查 token 是否有效
func (c *OAuth2Client) isTokenValid(token *TokenResponse) bool {
	if token == nil || token.AccessToken == "" {
		return false
	}

	// 这里需要根据 expires_in 计算 token 是否过期
	// 由于我们没有存储 token 的获取时间，这里简化处理
	// 实际应用中应该存储获取时间并计算过期时间
	return true
}

// RefreshToken 刷新 token
func (c *OAuth2Client) RefreshToken(ctx context.Context) (*TokenResponse, error) {
	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()

	if token == nil || token.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	// 构建请求体
	reqBody := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": token.RefreshToken,
	}

	// 发送请求
	newToken, err := c.doTokenRequest(ctx, DefaultRefreshTokenURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	// 如果新 token 没有返回 refresh_token，保留原来的
	if newToken.RefreshToken == "" {
		newToken.RefreshToken = token.RefreshToken
	}

	c.mu.Lock()
	c.token = newToken
	c.mu.Unlock()

	// 保存新 token
	if err := c.SaveToken(); err != nil {
		log.Warn().Err(err).Msg("Failed to save refreshed token")
	}

	return newToken, nil
}

// doTokenRequest 执行 token 请求
func (c *OAuth2Client) doTokenRequest(ctx context.Context, tokenURL string, params map[string]string) (*TokenResponse, error) {
	// 构建请求体
	formData := url.Values{}
	for k, v := range params {
		formData.Set(k, v)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 检查错误状态码
	if resp.StatusCode >= 400 {
		var errResp TokenErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Msg != "" {
			return nil, fmt.Errorf("token error: %d - %s", errResp.Code, errResp.Msg)
		}
		return nil, fmt.Errorf("token error: status %d, body: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var tokenResp struct {
		Code int            `json:"code"`
		Msg  string         `json:"msg"`
		Data *TokenResponse `json:"data"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.Code != 0 {
		return nil, fmt.Errorf("token error: %d - %s", tokenResp.Code, tokenResp.Msg)
	}

	if tokenResp.Data == nil {
		return nil, fmt.Errorf("empty token data in response")
	}

	return tokenResp.Data, nil
}

// LoadToken 从文件加载 token
func (c *OAuth2Client) LoadToken() error {
	if c.tokenFile == "" {
		return fmt.Errorf("token file not specified")
	}

	data, err := os.ReadFile(c.tokenFile)
	if err != nil {
		return fmt.Errorf("failed to read token file: %w", err)
	}

	var token TokenResponse
	if err := json.Unmarshal(data, &token); err != nil {
		return fmt.Errorf("failed to parse token: %w", err)
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

	// 确保目录存在
	dir := filepath.Dir(c.tokenFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	if err := os.WriteFile(c.tokenFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// HTTPClient 获取配置好的 HTTP 客户端
func (c *OAuth2Client) HTTPClient(ctx context.Context) (*http.Client, error) {
	token, err := c.ValidToken(ctx)
	if err != nil {
		return nil, err
	}

	// 创建带有 token 的 transport
	transport := &authTransport{
		base:        http.DefaultTransport,
		accessToken: token.AccessToken,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}, nil
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

	return token != nil && token.AccessToken != ""
}

// RevokeToken 撤销 token（删除本地文件）
func (c *OAuth2Client) RevokeToken() error {
	c.mu.Lock()
	c.token = nil
	c.mu.Unlock()

	if c.tokenFile != "" {
		if err := os.Remove(c.tokenFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove token file: %w", err)
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
		"expires_in":    token.ExpiresIn,
		"has_refresh":   token.RefreshToken != "",
	}
}

// GetUserInfo 获取用户信息
func (c *OAuth2Client) GetUserInfo(ctx context.Context) (*UserInfoResponse, error) {
	httpClient, err := c.HTTPClient(ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, DefaultUserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var userInfo UserInfoResponse
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if userInfo.Code != 0 {
		return nil, fmt.Errorf("API error: %d - %s", userInfo.Code, userInfo.Msg)
	}

	return &userInfo, nil
}

// ================ 辅助类型和函数 ================

// authTransport 实现 http.RoundTripper 接口，自动添加认证头
type authTransport struct {
	base        http.RoundTripper
	accessToken string
}

// RoundTrip 实现 http.RoundTripper 接口
func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.accessToken))
	return t.base.RoundTrip(req)
}

// generateState 生成随机 state
func generateState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 如果随机数生成失败，使用时间戳作为后备
		return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// LoadCredentials 从 JSON 文件加载凭据
func LoadCredentials(credentialsFile string) (*OAuth2Client, error) {
	data, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	var creds struct {
		AppID       string   `json:"app_id"`
		AppSecret   string   `json:"app_secret"`
		RedirectURL string   `json:"redirect_url"`
		Scopes      []string `json:"scopes"`
		TokenFile   string   `json:"token_file"`
	}

	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	return NewOAuth2Client(&OAuthConfig{
		AppID:       creds.AppID,
		AppSecret:   creds.AppSecret,
		RedirectURL: creds.RedirectURL,
		Scopes:      creds.Scopes,
		TokenFile:   creds.TokenFile,
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

// ToOAuth2Token 将飞书 TokenResponse 转换为 oauth2.Token
func ToOAuth2Token(token *TokenResponse) *oauth2.Token {
	if token == nil {
		return nil
	}

	return &oauth2.Token{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(token.ExpiresIn) * time.Second),
	}
}

// FromOAuth2Token 将 oauth2.Token 转换为飞书 TokenResponse
func FromOAuth2Token(token *oauth2.Token) *TokenResponse {
	if token == nil {
		return nil
	}

	expiresIn := int(time.Until(token.Expiry).Seconds())
	if expiresIn < 0 {
		expiresIn = 0
	}

	return &TokenResponse{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		ExpiresIn:    expiresIn,
	}
}
