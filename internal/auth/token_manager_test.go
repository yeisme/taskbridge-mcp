// Package auth 提供 Token 管理功能
package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/yeisme/taskbridge/internal/provider"
)

// MockTokenProvider 模拟 TokenProvider
type MockTokenProvider struct {
	name          string
	authenticated bool
	tokenInfo     *provider.TokenInfo
	refreshErr    error
	refreshCount  int
	refreshFunc   func(ctx context.Context) error // 自定义刷新函数
	mu            sync.Mutex
}

func (m *MockTokenProvider) Name() string {
	return m.name
}

func (m *MockTokenProvider) IsAuthenticated() bool {
	return m.authenticated
}

func (m *MockTokenProvider) RefreshToken(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshCount++

	// 如果有自定义刷新函数，使用它
	if m.refreshFunc != nil {
		return m.refreshFunc(ctx)
	}

	if m.refreshErr == nil {
		m.authenticated = true
		if m.tokenInfo != nil {
			m.tokenInfo.IsValid = true
			m.tokenInfo.ExpiresAt = time.Now().Add(1 * time.Hour)
			m.tokenInfo.NeedsRefresh = false
		}
	}
	return m.refreshErr
}

func (m *MockTokenProvider) GetTokenInfo() *provider.TokenInfo {
	return m.tokenInfo
}

func (m *MockTokenProvider) getRefreshCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.refreshCount
}

// TestNewTokenManager 测试创建 TokenManager
func TestNewTokenManager(t *testing.T) {
	config := DefaultTokenManagerConfig()
	tm := NewTokenManager(config)

	if tm == nil {
		t.Fatal("NewTokenManager returned nil")
	}

	if tm.config.RefreshBuffer != 5*time.Minute {
		t.Errorf("Expected RefreshBuffer to be 5 minutes, got %v", tm.config.RefreshBuffer)
	}

	if tm.config.CheckInterval != 1*time.Minute {
		t.Errorf("Expected CheckInterval to be 1 minute, got %v", tm.config.CheckInterval)
	}

	if tm.config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to be 3, got %d", tm.config.MaxRetries)
	}
}

// TestRegisterProvider 测试注册 Provider
func TestRegisterProvider(t *testing.T) {
	tm := NewTokenManager(DefaultTokenManagerConfig())

	mockProvider := &MockTokenProvider{
		name:          "test_provider",
		authenticated: true,
		tokenInfo: &provider.TokenInfo{
			Provider: "test_provider",
			HasToken: true,
			IsValid:  true,
		},
	}

	tm.RegisterProvider(mockProvider)

	providers := tm.ListProviders()
	if len(providers) != 1 {
		t.Errorf("Expected 1 provider, got %d", len(providers))
	}

	if providers[0] != "test_provider" {
		t.Errorf("Expected provider name 'test_provider', got '%s'", providers[0])
	}
}

// TestUnregisterProvider 测试注销 Provider
func TestUnregisterProvider(t *testing.T) {
	tm := NewTokenManager(DefaultTokenManagerConfig())

	mockProvider := &MockTokenProvider{
		name:          "test_provider",
		authenticated: true,
		tokenInfo:     &provider.TokenInfo{Provider: "test_provider"},
	}

	tm.RegisterProvider(mockProvider)
	tm.UnregisterProvider("test_provider")

	providers := tm.ListProviders()
	if len(providers) != 0 {
		t.Errorf("Expected 0 providers after unregister, got %d", len(providers))
	}
}

// TestGetStatus 测试获取状态
func TestGetStatus(t *testing.T) {
	tm := NewTokenManager(DefaultTokenManagerConfig())

	mockProvider := &MockTokenProvider{
		name:          "test_provider",
		authenticated: true,
		tokenInfo: &provider.TokenInfo{
			Provider:  "test_provider",
			HasToken:  true,
			IsValid:   true,
			ExpiresAt: time.Now().Add(1 * time.Hour),
		},
	}

	tm.RegisterProvider(mockProvider)

	status := tm.GetStatus()
	if len(status) != 1 {
		t.Errorf("Expected 1 status entry, got %d", len(status))
	}

	info, ok := status["test_provider"]
	if !ok {
		t.Fatal("Expected status for test_provider")
	}

	if !info.HasToken {
		t.Error("Expected HasToken to be true")
	}

	if !info.IsValid {
		t.Error("Expected IsValid to be true")
	}
}

// TestNeedsRefresh 测试刷新检测
func TestNeedsRefresh(t *testing.T) {
	config := DefaultTokenManagerConfig()
	tm := NewTokenManager(config)

	tests := []struct {
		name      string
		tokenInfo *provider.TokenInfo
		expected  bool
	}{
		{
			name: "没有token",
			tokenInfo: &provider.TokenInfo{
				HasToken: false,
			},
			expected: false,
		},
		{
			name: "过期时间为零值",
			tokenInfo: &provider.TokenInfo{
				HasToken:  true,
				ExpiresAt: time.Time{},
			},
			expected: false,
		},
		{
			name: "即将过期（在刷新缓冲期内）",
			tokenInfo: &provider.TokenInfo{
				HasToken:  true,
				ExpiresAt: time.Now().Add(3 * time.Minute), // 小于 5 分钟缓冲
			},
			expected: true,
		},
		{
			name: "还有很长时间过期",
			tokenInfo: &provider.TokenInfo{
				HasToken:  true,
				ExpiresAt: time.Now().Add(1 * time.Hour),
			},
			expected: false,
		},
		{
			name: "已经过期",
			tokenInfo: &provider.TokenInfo{
				HasToken:  true,
				ExpiresAt: time.Now().Add(-1 * time.Minute),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tm.needsRefresh(tt.tokenInfo)
			if result != tt.expected {
				t.Errorf("needsRefresh() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestRefreshWithRetry 测试带重试的刷新
func TestRefreshWithRetry(t *testing.T) {
	t.Run("刷新成功", func(t *testing.T) {
		tm := NewTokenManager(DefaultTokenManagerConfig())

		mockProvider := &MockTokenProvider{
			name:          "test_provider",
			authenticated: false,
			tokenInfo: &provider.TokenInfo{
				Provider:     "test_provider",
				HasToken:     true,
				NeedsRefresh: true,
			},
			refreshErr: nil,
		}

		err := tm.refreshWithRetry(context.Background(), mockProvider)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if mockProvider.getRefreshCount() != 1 {
			t.Errorf("Expected 1 refresh call, got %d", mockProvider.getRefreshCount())
		}
	})

	t.Run("刷新失败后重试成功", func(t *testing.T) {
		config := DefaultTokenManagerConfig()
		config.MaxRetries = 3
		config.RetryInterval = 10 * time.Millisecond
		tm := NewTokenManager(config)

		callCount := 0
		tokenInfo := &provider.TokenInfo{
			Provider:     "test_provider",
			HasToken:     true,
			NeedsRefresh: true,
		}
		mockProvider := &MockTokenProvider{
			name:          "test_provider",
			authenticated: false,
			tokenInfo:     tokenInfo,
			// 使用自定义刷新逻辑
			refreshFunc: func(ctx context.Context) error {
				callCount++
				if callCount < 3 {
					return errors.New("temporary error")
				}
				tokenInfo.IsValid = true
				return nil
			},
		}

		err := tm.refreshWithRetry(context.Background(), mockProvider)
		if err != nil {
			t.Errorf("Expected no error after retries, got %v", err)
		}

		if mockProvider.getRefreshCount() != 3 {
			t.Errorf("Expected 3 refresh calls, got %d", mockProvider.getRefreshCount())
		}
	})

	t.Run("所有重试都失败", func(t *testing.T) {
		config := DefaultTokenManagerConfig()
		config.MaxRetries = 2
		config.RetryInterval = 10 * time.Millisecond
		tm := NewTokenManager(config)

		mockProvider := &MockTokenProvider{
			name:          "test_provider",
			authenticated: false,
			tokenInfo: &provider.TokenInfo{
				Provider:     "test_provider",
				HasToken:     true,
				NeedsRefresh: true,
			},
			refreshErr: errors.New("permanent error"),
		}

		err := tm.refreshWithRetry(context.Background(), mockProvider)
		if err == nil {
			t.Error("Expected error, got nil")
		}

		if mockProvider.getRefreshCount() != 2 {
			t.Errorf("Expected 2 refresh calls, got %d", mockProvider.getRefreshCount())
		}
	})
}

// TestRefreshNow 测试立即刷新
func TestRefreshNow(t *testing.T) {
	tm := NewTokenManager(DefaultTokenManagerConfig())

	mockProvider := &MockTokenProvider{
		name:          "test_provider",
		authenticated: false,
		tokenInfo: &provider.TokenInfo{
			Provider: "test_provider",
			HasToken: true,
		},
	}

	tm.RegisterProvider(mockProvider)

	err := tm.RefreshNow(context.Background(), "test_provider")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if mockProvider.getRefreshCount() != 1 {
		t.Errorf("Expected 1 refresh call, got %d", mockProvider.getRefreshCount())
	}

	// 测试不存在的 Provider
	err = tm.RefreshNow(context.Background(), "non_existent")
	if err == nil {
		t.Error("Expected error for non-existent provider")
	}
}

// TestRefreshAll 测试刷新所有 Provider
func TestRefreshAll(t *testing.T) {
	tm := NewTokenManager(DefaultTokenManagerConfig())

	// 注册多个 Provider
	for i := 0; i < 3; i++ {
		mockProvider := &MockTokenProvider{
			name:          string(rune('a' + i)),
			authenticated: false,
			tokenInfo: &provider.TokenInfo{
				Provider: string(rune('a' + i)),
				HasToken: true,
			},
		}
		tm.RegisterProvider(mockProvider)
	}

	results := tm.RefreshAll(context.Background())

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	for name, err := range results {
		if err != nil {
			t.Errorf("Provider %s refresh failed: %v", name, err)
		}
	}
}

// TestStartStop 测试启动和停止
func TestStartStop(t *testing.T) {
	config := DefaultTokenManagerConfig()
	config.CheckInterval = 50 * time.Millisecond
	tm := NewTokenManager(config)

	mockProvider := &MockTokenProvider{
		name:          "test_provider",
		authenticated: true,
		tokenInfo: &provider.TokenInfo{
			Provider:     "test_provider",
			HasToken:     true,
			IsValid:      true,
			ExpiresAt:    time.Now().Add(1 * time.Minute), // 即将过期
			NeedsRefresh: true,
		},
	}

	tm.RegisterProvider(mockProvider)

	ctx := context.Background()
	err := tm.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start: %v", err)
	}

	if !tm.IsRunning() {
		t.Error("Expected IsRunning to be true")
	}

	// 重复启动应该失败
	err = tm.Start(ctx)
	if err == nil {
		t.Error("Expected error when starting already running manager")
	}

	// 等待一段时间让刷新执行
	time.Sleep(100 * time.Millisecond)

	tm.Stop()

	if tm.IsRunning() {
		t.Error("Expected IsRunning to be false after stop")
	}
}

// TestOnRefreshCallback 测试刷新回调
func TestOnRefreshCallback(t *testing.T) {
	config := DefaultTokenManagerConfig()
	config.CheckInterval = 50 * time.Millisecond
	tm := NewTokenManager(config)

	var callbackCalled bool
	var callbackProvider string
	var callbackErr error
	var mu sync.Mutex

	tm.SetOnRefreshCallback(func(provider string, err error) {
		mu.Lock()
		defer mu.Unlock()
		callbackCalled = true
		callbackProvider = provider
		callbackErr = err
	})

	mockProvider := &MockTokenProvider{
		name:          "test_provider",
		authenticated: true,
		tokenInfo: &provider.TokenInfo{
			Provider:     "test_provider",
			HasToken:     true,
			IsValid:      true,
			ExpiresAt:    time.Now().Add(1 * time.Minute),
			NeedsRefresh: true,
		},
	}

	tm.RegisterProvider(mockProvider)

	ctx := context.Background()
	_ = tm.Start(ctx)

	// 等待回调被触发
	time.Sleep(150 * time.Millisecond)

	tm.Stop()

	mu.Lock()
	defer mu.Unlock()
	if !callbackCalled {
		t.Error("Expected callback to be called")
	}

	if callbackProvider != "test_provider" {
		t.Errorf("Expected callback provider 'test_provider', got '%s'", callbackProvider)
	}

	if callbackErr != nil {
		t.Errorf("Expected no error in callback, got %v", callbackErr)
	}
}

// TestFormatDuration 测试格式化持续时间
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{-1 * time.Minute, "已过期"},
		{30 * time.Second, "30秒"},
		{90 * time.Second, "1分钟"},
		{30 * time.Minute, "30分钟"},
		{90 * time.Minute, "1小时30分钟"},
		{2 * time.Hour, "2小时"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("FormatDuration(%v) = '%s', expected '%s'", tt.duration, result, tt.expected)
			}
		})
	}
}

// TestGetProviderStatus 测试获取单个 Provider 状态
func TestGetProviderStatus(t *testing.T) {
	tm := NewTokenManager(DefaultTokenManagerConfig())

	mockProvider := &MockTokenProvider{
		name:          "test_provider",
		authenticated: true,
		tokenInfo: &provider.TokenInfo{
			Provider:  "test_provider",
			HasToken:  true,
			IsValid:   true,
			ExpiresAt: time.Now().Add(1 * time.Hour),
		},
	}

	tm.RegisterProvider(mockProvider)

	info, err := tm.GetProviderStatus("test_provider")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !info.HasToken {
		t.Error("Expected HasToken to be true")
	}

	// 测试不存在的 Provider
	_, err = tm.GetProviderStatus("non_existent")
	if err == nil {
		t.Error("Expected error for non-existent provider")
	}
}
