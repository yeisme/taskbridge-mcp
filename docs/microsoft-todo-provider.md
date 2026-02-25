# Microsoft Todo Provider 开发计划

## 概述

为 TaskBridge MCP 实现 Microsoft Todo（原 Wunderlist）Provider，允许用户通过 Microsoft Graph API 同步和管理任务。

## 技术背景

### Microsoft Graph API

Microsoft To Do 使用 Microsoft Graph API 进行访问，主要端点：

- `GET /me/todo/lists` - 获取任务列表
- `GET /me/todo/lists/{todoTaskListId}/tasks` - 获取任务
- `POST /me/todo/lists/{todoTaskListId}/tasks` - 创建任务
- `PATCH /me/todo/lists/{todoTaskListId}/tasks/{todoTaskId}` - 更新任务
- `DELETE /me/todo/lists/{todoTaskListId}/tasks/{todoTaskId}` - 删除任务

### OAuth2 认证

使用 Azure AD OAuth2 流程：

- 授权端点: `https://login.microsoftonline.com/common/oauth2/v2.0/authorize`
- Token 端点: `https://login.microsoftonline.com/common/oauth2/v2.0/token`
- 所需权限: `Tasks.ReadWrite`, `User.Read`

## 目录结构

```
internal/provider/microsoft/
├── provider.go      # Provider 接口实现
├── client.go        # Graph API 客户端
├── oauth.go         # OAuth2 认证处理
├── types.go         # API 数据类型定义
└── convert.go       # 数据模型转换
```

## 开发任务

### 1. OAuth2 认证模块 (oauth.go)

```go
// OAuth2Config OAuth2 配置
type OAuth2Config struct {
    ClientID     string
    ClientSecret string
    RedirectURL  string
    TenantID     string // 可选，用于多租户
    Scopes       []string
}

// 实现 PKCE 流程（推荐用于 CLI 应用）
// 1. 生成 code_verifier 和 code_challenge
// 2. 启动本地 HTTP 服务器接收回调
// 3. 交换 authorization code 获取 token
// 4. 持久化 token 到文件
```

### 2. Graph API 客户端 (client.go)

```go
// Client Microsoft Graph API 客户端
type Client struct {
    httpClient *http.Client
    baseURL    string
}

// 主要方法
func (c *Client) ListTodoLists(ctx context.Context) ([]TodoTaskList, error)
func (c *Client) ListTasks(ctx context.Context, listID string, opts ListOptions) ([]TodoTask, error)
func (c *Client) CreateTask(ctx context.Context, listID string, task *TodoTask) (*TodoTask, error)
func (c *Client) UpdateTask(ctx context.Context, listID string, task *TodoTask) (*TodoTask, error)
func (c *Client) DeleteTask(ctx context.Context, listID, taskID string) error
```

### 3. 数据类型定义 (types.go)

```go
// TodoTaskList Microsoft Todo 任务列表
type TodoTaskList struct {
    ID          string     `json:"id"`
    DisplayName string     `json:"displayName"`
    IsOwner     bool       `json:"isOwner"`
    IsShared    bool       `json:"isShared"`
    Wellknown   string     `json:"wellknownListName"`
}

// TodoTask Microsoft Todo 任务
type TodoTask struct {
    ID                 string            `json:"id"`
    Title              string            `json:"title"`
    Status             TaskStatus        `json:"status"`
    Importance         Importance        `json:"importance"`
    Body               ItemBody          `json:"body"`
    DueDateTime        *DateTimeTimeZone `json:"dueDateTime,omitempty"`
    StartDateTime      *DateTimeTimeZone `json:"startDateTime,omitempty"`
    CompletedDateTime  *DateTimeTimeZone `json:"completedDateTime,omitempty"`
    LastModifiedDateTime time.Time       `json:"lastModifiedDateTime"`
    LinkedResources    []LinkedResource  `json:"linkedResources"`
}
```

### 4. Provider 接口实现 (provider.go)

```go
// Provider Microsoft Todo Provider
type Provider struct {
    client       *Client
    oauth        *OAuth2Client
    config       Config
    capabilities provider.Capabilities
}

// 实现所有 provider.Provider 接口方法
```

### 5. 数据转换 (convert.go)

```go
// ToModelTask 将 Microsoft Todo 任务转换为统一模型
func ToModelTask(msTask *TodoTask) *model.Task

// ToMicrosoftTask 将统一模型转换为 Microsoft Todo 任务
func ToMicrosoftTask(task *model.Task) *TodoTask
```

## 能力映射

| Microsoft Todo  | TaskBridge Model | 说明                 |
| --------------- | ---------------- | -------------------- |
| importance      | priority         | 高/中/低 → 1-4       |
| status          | status           | notStarted/completed |
| dueDateTime     | due_date         | 带时区的时间         |
| body.content    | description      | 任务描述             |
| linkedResources | metadata         | 关联资源             |

## CLI 命令扩展

在 `cmd/auth.go` 中添加 Microsoft 认证支持：

```bash
# 登录 Microsoft 账户
taskbridge auth login microsoft --client-id <id> --client-secret <secret>

# 登出
taskbridge auth logout microsoft

# 查看认证状态
taskbridge auth status microsoft
```

## 测试计划

1. **单元测试**
   - OAuth2 流程测试（使用 mock）
   - 数据转换测试
   - API 响应解析测试

2. **集成测试**
   - 实际 API 调用测试（需要测试账户）
   - 完整同步流程测试

## 依赖

```go
// 可能需要的新依赖
golang.org/x/oauth2
github.com/coreos/go-oidc // 可选，用于 OIDC 发现
```

## 风险与注意事项

1. **API 限制**: Graph API 有请求频率限制，需要实现重试机制
2. **时区处理**: Microsoft 使用 DateTimeTimeZone 格式，需要正确转换
3. **增量同步**: 使用 delta link 实现增量同步
4. **权限范围**: 确保请求最小必要权限

## 参考资源

- [Microsoft Graph To Do API 文档](https://learn.microsoft.com/en-us/graph/api/resources/todo-overview)
- [Microsoft 身份平台文档](https://learn.microsoft.com/en-us/azure/active-directory/develop/)
- [OAuth 2.0 PKCE](https://oauth.net/2/pkce/)

## 时间线

```
[1] 创建目录结构和基础文件
[2] 实现 OAuth2 认证流程
[3] 实现 Graph API 客户端
[4] 实现 Provider 接口
[5] CLI 集成和测试
[6] 文档更新
```
