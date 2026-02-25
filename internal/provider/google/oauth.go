// Package google provides Google OAuth2 authentication
package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/yeisme/taskbridge/pkg/paths"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// OAuth2 Scopes
const (
	// ScopeTasks Google Tasks API scope
	ScopeTasks = "https://www.googleapis.com/auth/tasks"
	// ScopeTasksReadOnly Google Tasks API read-only scope
	ScopeTasksReadOnly = "https://www.googleapis.com/auth/tasks.readonly"
)

// OAuthConfig OAuth2 配置
type OAuthConfig struct {
	// ClientID OAuth2 客户端 ID
	ClientID string
	// ClientSecret OAuth2 客户端密钥
	ClientSecret string
	// RedirectURL 重定向 URL
	RedirectURL string
	// Scopes 授权范围
	Scopes []string
	// TokenFile Token 存储文件路径
	TokenFile string
}

// OAuth2Client OAuth2 客户端
type OAuth2Client struct {
	// config OAuth2 配置
	config *oauth2.Config
	// tokenFile Token 存储文件路径
	tokenFile string
	// token 当前 Token
	token *oauth2.Token
}

// NewOAuth2Client 创建 OAuth2 客户端
func NewOAuth2Client(cfg *OAuthConfig) *OAuth2Client {
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{ScopeTasks}
	}

	return &OAuth2Client{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
			Endpoint:     google.Endpoint,
		},
		tokenFile: cfg.TokenFile,
	}
}

// GetAuthURL 获取授权 URL
func (c *OAuth2Client) GetAuthURL(state string) string {
	return c.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// Exchange 使用授权码交换 token
func (c *OAuth2Client) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := c.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %w", err)
	}

	c.token = token
	return token, nil
}

// SetToken 设置 token
func (c *OAuth2Client) SetToken(token *oauth2.Token) {
	c.token = token
}

// GetToken 获取当前 token
func (c *OAuth2Client) GetToken() *oauth2.Token {
	return c.token
}

// LoadToken 从文件加载 token
func (c *OAuth2Client) LoadToken() (*oauth2.Token, error) {
	if c.tokenFile == "" {
		return nil, fmt.Errorf("token file not specified")
	}

	data, err := os.ReadFile(c.tokenFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	c.token = &token
	return &token, nil
}

// SetTokenFile 设置 token 文件路径
func (c *OAuth2Client) SetTokenFile(path string) {
	c.tokenFile = path
}

// SaveToken 保存 token 到文件
func (c *OAuth2Client) SaveToken(token *oauth2.Token) error {
	if c.tokenFile == "" {
		return fmt.Errorf("token file not specified")
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

	c.token = token
	return nil
}

// TokenSource 获取 token source（自动刷新）
func (c *OAuth2Client) TokenSource(ctx context.Context) oauth2.TokenSource {
	if c.token == nil {
		return nil
	}
	return c.config.TokenSource(ctx, c.token)
}

// RefreshToken 刷新 token
func (c *OAuth2Client) RefreshToken(ctx context.Context) (*oauth2.Token, error) {
	if c.token == nil {
		return nil, fmt.Errorf("no token to refresh")
	}

	tokenSource := c.config.TokenSource(ctx, c.token)
	token, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	c.token = token
	return token, nil
}

// ValidToken 获取有效的 token（必要时刷新）
func (c *OAuth2Client) ValidToken(ctx context.Context) (*oauth2.Token, error) {
	if c.token == nil {
		// 尝试从文件加载
		if _, err := c.LoadToken(); err != nil {
			return nil, fmt.Errorf("no token available: %w", err)
		}
	}

	// 检查 token 是否过期
	if c.token.Valid() {
		return c.token, nil
	}

	// 刷新 token
	return c.RefreshToken(ctx)
}

// HTTPClient 获取配置了认证的 HTTP 客户端
func (c *OAuth2Client) HTTPClient(ctx context.Context) (*http.Client, error) {
	token, err := c.ValidToken(ctx)
	if err != nil {
		return nil, err
	}

	return c.config.Client(ctx, token), nil
}

// IsExpired 检查 token 是否过期
func (c *OAuth2Client) IsExpired() bool {
	if c.token == nil {
		return true
	}
	return !c.token.Valid()
}

// ExpiresIn 返回 token 过期时间
func (c *OAuth2Client) ExpiresIn() time.Duration {
	if c.token == nil || c.token.Expiry.IsZero() {
		return 0
	}
	return time.Until(c.token.Expiry)
}

// LoadCredentials 从 Google credentials 文件加载配置
func LoadCredentials(credentialsFile string) (*OAuth2Client, error) {
	data, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	// 尝试解析为 Web/Installed 类型的凭证文件
	var rawCreds struct {
		Web struct {
			ClientID                string   `json:"client_id"`
			ClientSecret            string   `json:"client_secret"`
			AuthURI                 string   `json:"auth_uri"`
			TokenURI                string   `json:"token_uri"`
			AuthProviderX509CertURL string   `json:"auth_provider_x509_cert_url"`
			RedirectURIs            []string `json:"redirect_uris"`
			ProjectID               string   `json:"project_id"`
		} `json:"web"`
		Installed struct {
			ClientID                string   `json:"client_id"`
			ClientSecret            string   `json:"client_secret"`
			AuthURI                 string   `json:"auth_uri"`
			TokenURI                string   `json:"token_uri"`
			AuthProviderX509CertURL string   `json:"auth_provider_x509_cert_url"`
			RedirectURIs            []string `json:"redirect_uris"`
			ProjectID               string   `json:"project_id"`
		} `json:"installed"`
	}

	if err := json.Unmarshal(data, &rawCreds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	var config *oauth2.Config
	if rawCreds.Web.ClientID != "" {
		redirectURL := "http://localhost:8080/callback"
		if len(rawCreds.Web.RedirectURIs) > 0 {
			redirectURL = rawCreds.Web.RedirectURIs[0]
		}
		config = &oauth2.Config{
			ClientID:     rawCreds.Web.ClientID,
			ClientSecret: rawCreds.Web.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  rawCreds.Web.AuthURI,
				TokenURL: rawCreds.Web.TokenURI,
			},
			RedirectURL: redirectURL,
			Scopes:      []string{ScopeTasks},
		}
	} else if rawCreds.Installed.ClientID != "" {
		redirectURL := "http://localhost:8080/callback"
		if len(rawCreds.Installed.RedirectURIs) > 0 {
			redirectURL = rawCreds.Installed.RedirectURIs[0]
		}
		config = &oauth2.Config{
			ClientID:     rawCreds.Installed.ClientID,
			ClientSecret: rawCreds.Installed.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  rawCreds.Installed.AuthURI,
				TokenURL: rawCreds.Installed.TokenURI,
			},
			RedirectURL: redirectURL,
			Scopes:      []string{ScopeTasks},
		}
	} else {
		// 尝试使用 google.ConfigFromJSON
		config, err = google.ConfigFromJSON(data, ScopeTasks)
		if err != nil {
			return nil, fmt.Errorf("failed to parse credentials: %w", err)
		}
	}

	// 确保 Scopes 已设置
	if len(config.Scopes) == 0 {
		config.Scopes = []string{ScopeTasks}
	}

	return &OAuth2Client{
		config: config,
	}, nil
}

// TokenInfo Token 信息（用于调试）
type TokenInfo struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry"`
	ExpiresIn    int       `json:"expires_in"`
	Valid        bool      `json:"valid"`
}

// GetTokenInfo 获取 token 信息
func (c *OAuth2Client) GetTokenInfo() *TokenInfo {
	if c.token == nil {
		return nil
	}

	return &TokenInfo{
		AccessToken:  maskToken(c.token.AccessToken),
		TokenType:    c.token.TokenType,
		RefreshToken: maskToken(c.token.RefreshToken),
		Expiry:       c.token.Expiry,
		ExpiresIn:    int(time.Until(c.token.Expiry).Seconds()),
		Valid:        c.token.Valid(),
	}
}

// maskToken 隐藏 token 的大部分内容
func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}

// NewOAuth2ClientFromHome 从 HOME 目录加载凭证创建 OAuth2 客户端
// 凭证文件路径: ~/.taskbridge/credentials/google_credentials.json
// Token 文件路径: ~/.taskbridge/credentials/google_token.json
func NewOAuth2ClientFromHome() (*OAuth2Client, error) {
	// 确保凭证目录存在
	if err := paths.EnsureCredentialsDir(); err != nil {
		return nil, fmt.Errorf("failed to create credentials directory: %w", err)
	}

	// 获取凭证文件路径
	credentialsPath := paths.GetCredentialsPath("google")

	// 检查凭证文件是否存在
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("credentials file not found at %s, please run 'taskbridge auth login google' first", credentialsPath)
	}

	// 加载凭证
	client, err := LoadCredentials(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials: %w", err)
	}

	// 设置 token 文件路径
	client.SetTokenFile(paths.GetTokenPath("google"))

	// 尝试加载已有的 token（忽略错误，可能是首次登录）
	_, _ = client.LoadToken()

	return client, nil
}

// GetCredentialsPath 获取 Google 凭证文件路径
func GetCredentialsPath() string {
	return paths.GetCredentialsPath("google")
}

// GetTokenPath 获取 Google Token 文件路径
func GetTokenPath() string {
	return paths.GetTokenPath("google")
}

// EnsureCredentialsDir 确保 Google 凭证目录存在
func EnsureCredentialsDir() error {
	return paths.EnsureCredentialsDir()
}
