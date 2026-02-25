# Token 自动刷新机制设计

## 概述

设计一套完整的 Token 自动刷新机制，确保所有 Provider 的 OAuth2 Token 不会过期导致服务中断。

## 问题分析

### OAuth2 Token 生命周期

```
┌─────────────────────────────────────────────────────────────────┐
│                    OAuth2 Token 生命周期                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  获取Token ──→ 有效期(通常1小时) ──→ 过期 ──→ 需要刷新           │
│       │                              │                          │
│       │                              ▼                          │
│       │                    ┌─────────────────┐                  │
│       │                    │  自动刷新机制   │                  │
│       │                    │  (Refresh Token)│                  │
│       │                    └────────┬────────┘                  │
│       │                             │                           │
│       └─────────────────────────────┘                           │
│                    循环保持有效                                   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 当前问题

1. **Token 过期无感知** - 用户不知道 Token 何时过期
2. **被动刷新** - 只有在调用 API 失败时才刷新
3. **无后台服务** - 没有定时刷新机制
4. **MCP 无法感知** - AI 无法知道 Token 状态

## 解决方案

### 架构设计

```
┌────────────────────────────────────────────────────────────────────┐
│                        Token 管理架构                               │
├────────────────────────────────────────────────────────────────────┤
│                                                                    │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐         │
│  │ MCP Tools    │    │ CLI 命令     │    │ 后台服务     │         │
│  │              │    │              │    │              │         │
│  │ token_status │    │ auth status  │    │ 定时刷新     │         │
│  │ token_refresh│    │ auth refresh │    │ 健康检查     │         │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘         │
│         │                   │                   │                 │
│         └───────────────────┼───────────────────┘                 │
│                             │                                     │
│                             ▼                                     │
│                  ┌─────────────────────┐                          │
│                  │   Token Manager     │                          │
│                  │                     │                          │
│                  │ - 状态监控          │                          │
│                  │ - 自动刷新          │                          │
│                  │ - 过期预警          │                          │
│                  │ - 刷新策略          │                          │
│                  └──────────┬──────────┘                          │
│                             │                                     │
│         ┌───────────────────┼───────────────────┐                 │
│         │                   │                   │                 │
│         ▼                   ▼                   ▼                 │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐           │
│  │ Google      │    │ Microsoft   │    │ 其他        │           │
│  │ OAuth2      │    │ OAuth2      │    │ Provider    │           │
│  └─────────────┘    └─────────────┘    └─────────────┘           │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

### 核心组件

#### 1. Token Manager（internal/auth/token_manager.go）

```go
// TokenManager Token 管理器
type TokenManager struct {
    providers   map[string]TokenProvider
    refreshChan chan string
    stopChan    chan struct{}
    config      TokenManagerConfig
}

// TokenManagerConfig 配置
type TokenManagerConfig struct {
    // 刷新提前量（默认 5 分钟）
    RefreshBuffer time.Duration
    // 检查间隔（默认 1 分钟）
    CheckInterval time.Duration
    // 最大重试次数
    MaxRetries int
    // 重试间隔
    RetryInterval time.Duration
}

// TokenInfo Token 信息
type TokenInfo struct {
    Provider      string    `json:"provider"`
    HasToken      bool      `json:"has_token"`
    IsValid       bool      `json:"is_valid"`
    ExpiresAt     time.Time `json:"expires_at"`
    Refreshable   bool      `json:"refreshable"`
    TimeUntilExpiry string  `json:"time_until_expiry"`
    NeedsRefresh  bool      `json:"needs_refresh"`
}
```

#### 2. 后台服务模式（cmd/serve.go）

```go
// ServeCmd 后台服务命令
var ServeCmd = &cobra.Command{
    Use:   "serve",
    Short: "启动后台服务",
    Long: `启动 TaskBridge 后台服务，提供以下功能：

- Token 自动刷新
- 定时同步
- MCP Server
- 健康检查 API`,
}

// ServeConfig 服务配置
type ServeConfig struct {
    // MCP 服务
    EnableMCP bool
    MCPPort   int

    // Token 刷新
    EnableTokenRefresh bool
    TokenCheckInterval time.Duration

    // 定时同步
    EnableSync bool
    SyncInterval time.Duration

    // 健康检查
    EnableHealthCheck bool
    HealthCheckPort int
}
```

#### 3. MCP Token 工具（internal/mcp/token_tools.go）

```go
// MCP 工具定义

// token_status - 查看所有 Provider 的 Token 状态
// token_refresh - 刷新指定 Provider 的 Token
// token_auto_refresh_enable - 启用自动刷新
// token_auto_refresh_disable - 禁用自动刷新
```

### 刷新策略

#### 主动刷新策略

```go
// RefreshStrategy 刷新策略
type RefreshStrategy struct {
    // 提前刷新时间（Token 过期前多久开始刷新）
    RefreshBuffer time.Duration // 默认 5 分钟

    // 刷新触发条件
    Triggers struct {
        // 时间触发：距离过期时间 < RefreshBuffer
        TimeBased bool
        // API 错误触发：收到 401 错误时
        ErrorBased bool
        // 启动时触发：服务启动时检查
        StartupCheck bool
    }
}
```

#### 刷新流程

```
┌─────────────────────────────────────────────────────────────┐
│                    Token 自动刷新流程                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────┐                                               │
│  │ 定时检查 │ ◄─── 每 1 分钟                                 │
│  └────┬─────┘                                               │
│       │                                                     │
│       ▼                                                     │
│  ┌──────────────────┐                                       │
│  │ 遍历所有Provider │                                       │
│  └────────┬─────────┘                                       │
│           │                                                 │
│           ▼                                                 │
│  ┌──────────────────────┐                                   │
│  │ 检查 Token 是否存在  │                                   │
│  └──────────┬───────────┘                                   │
│             │                                               │
│       ┌─────┴─────┐                                         │
│       │           │                                         │
│       ▼           ▼                                         │
│    [不存在]    [存在]                                        │
│       │           │                                         │
│       │           ▼                                         │
│       │    ┌─────────────────────┐                          │
│       │    │ 计算距离过期时间     │                          │
│       │    └──────────┬──────────┘                          │
│       │               │                                     │
│       │         ┌─────┴─────┐                               │
│       │         │           │                               │
│       │         ▼           ▼                               │
│       │   [ < 5分钟 ]   [ > 5分钟 ]                         │
│       │         │           │                               │
│       │         ▼           │                               │
│       │    ┌─────────┐      │                               │
│       │    │需要刷新 │      │                               │
│       │    └────┬────┘      │                               │
│       │         │           │                               │
│       └─────────┼───────────┘                               │
│                 │                                           │
│                 ▼                                           │
│         ┌───────────────┐                                   │
│         │  执行刷新     │                                   │
│         └───────┬───────┘                                   │
│                 │                                           │
│           ┌─────┴─────┐                                     │
│           │           │                                     │
│           ▼           ▼                                     │
│        [成功]      [失败]                                    │
│           │           │                                     │
│           ▼           ▼                                     │
│      保存新Token   重试/告警                                 │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 实现计划

#### Phase 1: Token Manager 核心实现

1. 创建 `internal/auth/token_manager.go`
2. 实现 Token 状态监控
3. 实现自动刷新逻辑
4. 添加配置支持

#### Phase 2: 后台服务模式

1. 创建 `cmd/serve.go`
2. 集成 Token Manager
3. 添加健康检查端点
4. 支持优雅关闭

#### Phase 3: MCP 工具集成

1. 添加 `token_status` 工具
2. 添加 `token_refresh` 工具
3. 添加自动刷新控制工具
4. 添加 Token 状态资源

#### Phase 4: CLI 增强

1. 增强 `auth status` 显示更多详情
2. 添加 `auth auto-refresh` 命令
3. 添加配置文件支持

### 使用示例

#### CLI 使用

```bash
# 查看所有 Token 状态
taskbridge auth status

# 输出示例：
#┌────────────┬─────────┬─────────────────────┬──────────────┐
#│ Provider   │ 状态    │ 过期时间            │ 剩余时间     │
#├────────────┼─────────┼─────────────────────┼──────────────┤
#│ google     │ ✅ 有效 │ 2024-01-15 10:30:00 │ 45 分钟      │
#│ microsoft  │ ⚠️ 即将过期│ 2024-01-15 09:35:00 │ 5 分钟      │
#│ feishu     │ ❌ 未认证│ -                   │ -            │
#└────────────┴─────────┴─────────────────────┴──────────────┘

# 启动后台服务（包含自动刷新）
taskbridge serve

# 手动刷新
taskbridge auth refresh microsoft

# 启用/禁用自动刷新
taskbridge auth auto-refresh enable
taskbridge auth auto-refresh disable
```

#### MCP 工具使用

```json
// 查看Token状态
{
  "tool": "token_status",
  "arguments": {}
}

// 返回
{
  "providers": [
    {
      "provider": "google",
      "is_valid": true,
      "expires_at": "2024-01-15T10:30:00Z",
      "time_until_expiry": "45m"
    },
    {
      "provider": "microsoft",
      "is_valid": true,
      "needs_refresh": true,
      "expires_at": "2024-01-15T09:35:00Z",
      "time_until_expiry": "5m"
    }
  ]
}

// 刷新Token
{
  "tool": "token_refresh",
  "arguments": {
    "provider": "microsoft"
  }
}
```

### 配置文件

```yaml
# ~/.taskbridge/config.yaml

token:
  # 自动刷新配置
  auto_refresh:
    enabled: true
    check_interval: 1m
    refresh_buffer: 5m

  # 重试配置
  retry:
    max_attempts: 3
    interval: 30s

  # 告警配置
  alert:
    # Token 即将过期时告警（提前多少时间）
    expiry_warning: 24h
    # 刷新失败时告警
    on_refresh_failure: true

serve:
  # 后台服务配置
  mcp:
    enabled: true
    port: 8080

  health_check:
    enabled: true
    port: 8081

  sync:
    enabled: true
    interval: 5m
```

### 错误处理

1. **刷新失败重试** - 最多重试 3 次，间隔 30 秒
2. **网络错误** - 指数退避重试
3. **Refresh Token 过期** - 提示用户重新登录
4. **并发刷新** - 使用锁防止重复刷新

### 监控与日志

```go
// 日志示例
log.Info().
    Str("provider", "microsoft").
    Time("expires_at", token.Expiry).
    Dur("time_until_expiry", timeUntilExpiry).
    Msg("Token status checked")

log.Warn().
    Str("provider", "microsoft").
    Msg("Token will expire soon, refreshing...")

log.Error().
    Str("provider", "microsoft").
    Err(err).
    Msg("Failed to refresh token")
```

## 文件结构

```
internal/
├── auth/
│   ├── token_manager.go     # Token 管理器
│   ├── token_manager_test.go
│   └── config.go            # 配置
├── provider/
│   ├── provider.go          # 添加 TokenProvider 接口
│   └── ...
cmd/
├── serve.go                 # 后台服务命令
└── auth.go                  # 增强认证命令
```

## 下一步行动

1. **立即实现**：Token Manager 核心逻辑
2. **短期实现**：后台服务模式
3. **中期实现**：MCP 工具集成
4. **长期优化**：监控告警系统
