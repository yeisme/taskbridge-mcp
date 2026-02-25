// Package auth 提供 Token 管理功能
package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/yeisme/taskbridge/internal/provider"
)

// TokenProvider Token 提供者接口
type TokenProvider interface {
	// Name 返回 Provider 名称
	Name() string
	// IsAuthenticated 检查是否已认证
	IsAuthenticated() bool
	// RefreshToken 刷新 Token
	RefreshToken(ctx context.Context) error
	// GetTokenInfo 获取 Token 信息
	GetTokenInfo() *provider.TokenInfo
}

// TokenManagerConfig Token 管理器配置
type TokenManagerConfig struct {
	// RefreshBuffer 刷新提前量（默认 5 分钟）
	RefreshBuffer time.Duration
	// CheckInterval 检查间隔（默认 1 分钟）
	CheckInterval time.Duration
	// MaxRetries 最大重试次数
	MaxRetries int
	// RetryInterval 重试间隔
	RetryInterval time.Duration
}

// DefaultTokenManagerConfig 默认配置
func DefaultTokenManagerConfig() TokenManagerConfig {
	return TokenManagerConfig{
		RefreshBuffer: 5 * time.Minute,
		CheckInterval: 1 * time.Minute,
		MaxRetries:    3,
		RetryInterval: 30 * time.Second,
	}
}

// TokenManager Token 管理器
type TokenManager struct {
	// providers 已注册的 Provider 映射表
	providers map[string]TokenProvider
	// config 管理器配置
	config TokenManagerConfig
	// stopChan 停止信号通道
	stopChan chan struct{}
	// running 是否正在运行
	running bool
	// mu 读写锁
	mu sync.RWMutex
	// onRefresh 刷新回调函数
	onRefresh func(provider string, err error)
}

// NewTokenManager 创建 Token 管理器
func NewTokenManager(config TokenManagerConfig) *TokenManager {
	return &TokenManager{
		providers: make(map[string]TokenProvider),
		config:    config,
		stopChan:  make(chan struct{}),
	}
}

// RegisterProvider 注册 Provider
func (tm *TokenManager) RegisterProvider(provider TokenProvider) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.providers[provider.Name()] = provider
	log.Info().Str("provider", provider.Name()).Msg("Provider registered to TokenManager")
}

// UnregisterProvider 注销 Provider
func (tm *TokenManager) UnregisterProvider(name string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.providers, name)
	log.Info().Str("provider", name).Msg("Provider unregistered from TokenManager")
}

// SetOnRefreshCallback 设置刷新回调
func (tm *TokenManager) SetOnRefreshCallback(callback func(provider string, err error)) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.onRefresh = callback
}

// Start 启动自动刷新
func (tm *TokenManager) Start(ctx context.Context) error {
	tm.mu.Lock()
	if tm.running {
		tm.mu.Unlock()
		return fmt.Errorf("token manager already running")
	}
	tm.running = true
	tm.mu.Unlock()

	log.Info().
		Dur("check_interval", tm.config.CheckInterval).
		Dur("refresh_buffer", tm.config.RefreshBuffer).
		Msg("Token manager started")

	go tm.run(ctx)

	return nil
}

// Stop 停止自动刷新
func (tm *TokenManager) Stop() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if !tm.running {
		return
	}

	close(tm.stopChan)
	tm.running = false
	log.Info().Msg("Token manager stopped")
}

// run 运行刷新循环
func (tm *TokenManager) run(ctx context.Context) {
	// 启动时立即检查一次
	tm.checkAndRefresh(ctx)

	ticker := time.NewTicker(tm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Token manager stopped by context")
			return
		case <-tm.stopChan:
			return
		case <-ticker.C:
			tm.checkAndRefresh(ctx)
		}
	}
}

// checkAndRefresh 检查并刷新所有 Token
func (tm *TokenManager) checkAndRefresh(ctx context.Context) {
	tm.mu.RLock()
	providers := make([]TokenProvider, 0, len(tm.providers))
	for _, p := range tm.providers {
		providers = append(providers, p)
	}
	onRefresh := tm.onRefresh
	tm.mu.RUnlock()

	for _, provider := range providers {
		info := provider.GetTokenInfo()
		if info == nil {
			continue
		}

		// 检查是否需要刷新
		if info.NeedsRefresh || tm.needsRefresh(info) {
			log.Info().
				Str("provider", provider.Name()).
				Bool("needs_refresh", info.NeedsRefresh).
				Msg("Token needs refresh, starting...")

			err := tm.refreshWithRetry(ctx, provider)

			if onRefresh != nil {
				onRefresh(provider.Name(), err)
			}
		}
	}
}

// needsRefresh 检查是否需要刷新
func (tm *TokenManager) needsRefresh(info *provider.TokenInfo) bool {
	if !info.HasToken {
		return false
	}

	if info.ExpiresAt.IsZero() {
		return false
	}

	timeUntilExpiry := time.Until(info.ExpiresAt)
	return timeUntilExpiry <= tm.config.RefreshBuffer
}

// refreshWithRetry 带重试的刷新
func (tm *TokenManager) refreshWithRetry(ctx context.Context, provider TokenProvider) error {
	var lastErr error

	for i := 0; i < tm.config.MaxRetries; i++ {
		if i > 0 {
			log.Warn().
				Str("provider", provider.Name()).
				Int("attempt", i+1).
				Int("max_retries", tm.config.MaxRetries).
				Msg("Retrying token refresh...")

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(tm.config.RetryInterval):
			}
		}

		err := provider.RefreshToken(ctx)
		if err == nil {
			log.Info().
				Str("provider", provider.Name()).
				Msg("Token refreshed successfully")
			return nil
		}

		lastErr = err
		log.Error().
			Err(err).
			Str("provider", provider.Name()).
			Int("attempt", i+1).
			Msg("Failed to refresh token")
	}

	return fmt.Errorf("failed to refresh token after %d attempts: %w", tm.config.MaxRetries, lastErr)
}

// RefreshNow 立即刷新指定 Provider
func (tm *TokenManager) RefreshNow(ctx context.Context, providerName string) error {
	tm.mu.RLock()
	provider, ok := tm.providers[providerName]
	tm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("provider %s not found", providerName)
	}

	return tm.refreshWithRetry(ctx, provider)
}

// RefreshAll 立即刷新所有 Provider
func (tm *TokenManager) RefreshAll(ctx context.Context) map[string]error {
	tm.mu.RLock()
	providers := make([]TokenProvider, 0, len(tm.providers))
	for _, p := range tm.providers {
		providers = append(providers, p)
	}
	tm.mu.RUnlock()

	results := make(map[string]error)
	var wg sync.WaitGroup

	for _, provider := range providers {
		wg.Add(1)
		go func(p TokenProvider) {
			defer wg.Done()
			err := tm.refreshWithRetry(ctx, p)
			results[p.Name()] = err
		}(provider)
	}

	wg.Wait()
	return results
}

// GetStatus 获取所有 Provider 的 Token 状态
func (tm *TokenManager) GetStatus() map[string]*provider.TokenInfo {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	status := make(map[string]*provider.TokenInfo)
	for name, p := range tm.providers {
		info := p.GetTokenInfo()
		if info != nil {
			// 更新 NeedsRefresh 状态
			info.NeedsRefresh = tm.needsRefresh(info)
			status[name] = info
		} else {
			status[name] = &provider.TokenInfo{
				Provider: name,
				HasToken: false,
				IsValid:  false,
			}
		}
	}

	return status
}

// GetProviderStatus 获取指定 Provider 的状态
func (tm *TokenManager) GetProviderStatus(name string) (*provider.TokenInfo, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	p, ok := tm.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	info := p.GetTokenInfo()
	if info != nil {
		info.NeedsRefresh = tm.needsRefresh(info)
	}
	return info, nil
}

// IsRunning 检查是否正在运行
func (tm *TokenManager) IsRunning() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.running
}

// ListProviders 列出所有已注册的 Provider
func (tm *TokenManager) ListProviders() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	names := make([]string, 0, len(tm.providers))
	for name := range tm.providers {
		names = append(names, name)
	}
	return names
}

// FormatDuration 格式化持续时间
func FormatDuration(d time.Duration) string {
	if d < 0 {
		return "已过期"
	}

	if d < time.Minute {
		return fmt.Sprintf("%d秒", int(d.Seconds()))
	}

	if d < time.Hour {
		return fmt.Sprintf("%d分钟", int(d.Minutes()))
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if minutes == 0 {
		return fmt.Sprintf("%d小时", hours)
	}
	return fmt.Sprintf("%d小时%d分钟", hours, minutes)
}
