# Provider 连接指南

本文档详细介绍如何配置和连接各个 Todo 平台 provider。

## 目录

- [Google Tasks](#google-tasks)
- [Microsoft Todo](#microsoft-todo)
- [飞书任务](#飞书任务)
- [TickTick](#ticktick)
- [滴答清单 (Dida365)](#滴答清单-dida365)
- [Todoist](#todoist)

---

## Google Tasks

### 前置要求

- Google Cloud Platform 账号
- 已启用的 Google Tasks API

### 步骤 1: 创建 Google Cloud 项目

1. 访问 [Google Cloud Console](https://console.cloud.google.com/)
2. 创建新项目或选择现有项目
3. 记录项目 ID

### 步骤 2: 启用 Google Tasks API

1. 在左侧菜单中选择 **API 和服务** > **库**
2. 搜索 "Tasks API"
3. 点击 **启用**

### 步骤 3: 配置 OAuth 同意屏幕

1. 转到 **API 和服务** > **OAuth 同意屏幕**
2. 选择用户类型（外部/内部）
3. 填写应用名称、支持邮箱等信息
4. 添加以下作用域：
   - `https://www.googleapis.com/auth/tasks`
   - `https://www.googleapis.com/auth/tasks.readonly`

### 步骤 4: 创建 OAuth 客户端凭证

1. 转到 **API 和服务** > **凭证**
2. 点击 **创建凭证** > **OAuth 客户端 ID**
3. 应用类型选择 **桌面应用**
4. 记录 **客户端 ID** 和 **客户端密钥**

### 步骤 5: 保存凭证文件

创建凭证文件 `~/.taskbridge/credentials/google.json`：

```json
{
  "client_id": "你的客户端ID.apps.googleusercontent.com",
  "client_secret": "你的客户端密钥",
  "redirect_url": "http://127.0.0.1:8080/callback"
}
```

### 步骤 6: 登录认证

```bash
taskbridge auth login google
```

系统会自动打开浏览器进行 OAuth 授权，完成后 token 将保存到 `~/.taskbridge/tokens/google.json`。

---

## Microsoft Todo

### 前置要求

- Microsoft Azure 账号
- Microsoft 365 订阅（个人或工作账号）

### 步骤 1: 注册 Azure AD 应用

1. 访问 [Azure Portal](https://portal.azure.com/)
2. 转到 **Azure Active Directory** > **应用注册**
3. 点击 **新注册**
4. 填写应用名称，选择支持的账户类型
5. 记录 **应用程序(客户端) ID**

### 步骤 2: 配置身份验证

1. 在应用页面，点击 **身份验证**
2. 添加平台 > **Web**
3. 添加重定向 URI：`http://127.0.0.1:8080/callback`
4. 勾选 **访问令牌** 和 **ID 令牌**

### 步骤 3: 创建客户端密钥

1. 点击 **证书和密码**
2. 点击 **新客户端密码**
3. 记录生成的 **密钥值**（只显示一次）

### 步骤 4: 配置 API 权限

1. 点击 **API 权限**
2. 添加权限 > **Microsoft Graph**
3. 选择 **委托的权限**
4. 添加以下权限：
   - `Tasks.Read`
   - `Tasks.ReadWrite`
   - `Tasks.Read.Shared`
   - `Tasks.ReadWrite.Shared`

### 步骤 5: 保存凭证文件

创建凭证文件 `~/.taskbridge/credentials/microsoft.json`：

```json
{
  "client_id": "你的应用程序ID",
  "client_secret": "你的客户端密钥",
  "redirect_url": "http://127.0.0.1:8080/callback",
  "tenant_id": "common"
}
```

### 步骤 6: 登录认证

```bash
taskbridge auth login microsoft
```

---

## 飞书任务

### 前置要求

- 飞书开发者账号
- 已创建的自建应用

### 步骤 1: 创建飞书应用

1. 访问 [飞书开放平台](https://open.feishu.cn/)
2. 点击 **创建企业自建应用**
3. 填写应用名称和描述
4. 记录 **App ID** 和 **App Secret**

### 步骤 2: 配置应用权限

1. 进入应用 > **权限管理**
2. 申请以下权限：
   - `task:tasklist:read` - 获取任务列表
   - `task:tasklist:write` - 创建和更新任务列表
   - `task:task:read` - 获取任务详情
   - `task:task:write` - 创建和更新任务

### 步骤 3: 配置重定向 URL

1. 进入 **安全设置**
2. 添加重定向 URL：`http://127.0.0.1:3456/callback`

### 步骤 4: 发布应用版本

1. 进入 **版本管理与发布**
2. 创建版本并提交审核
3. 审核通过后发布

### 步骤 5: 保存凭证文件

创建凭证文件 `~/.taskbridge/credentials/feishu.json`：

```json
{
  "app_id": "cli_xxxxxxxxxxxx",
  "app_secret": "xxxxxxxxxxxxxxxx",
  "redirect_url": "http://127.0.0.1:3456/callback",
  "scopes": [
    "task:tasklist:read",
    "task:tasklist:write",
    "task:task:read",
    "task:task:write"
  ]
}
```

### 步骤 6: 登录认证

```bash
taskbridge auth login feishu
```

---

## TickTick

TickTick 使用 API Token 认证，无需创建开发者应用。

### 步骤 1: 获取 API Token

1. 打开 [TickTick](https://ticktick.com) 并登录
2. 打开浏览器开发者工具（F12）
3. 切换到 **网络** 标签
4. 刷新页面或进行任意操作
5. 在请求中找到 `Cookie` 头，复制 `t=` 后面的 token 值
6. Token 通常以 `tp_` 开头

### 步骤 2: 登录认证

```bash
taskbridge auth login ticktick
```

按提示输入 API Token，认证成功后 token 将保存到 `~/.taskbridge/tokens/ticktick.json`。

### 注意事项

- TickTick 使用静态 Token，无需刷新
- Token 有效期较长，但建议定期检查

---

## 滴答清单 (Dida365)

滴答清单是 TickTick 的国内版本，使用相同的认证方式。

### 步骤 1: 获取 API Token

1. 打开 [滴答清单](https://dida365.com) 并登录
2. 打开浏览器开发者工具（F12）
3. 切换到 **网络** 标签
4. 刷新页面或进行任意操作
5. 在请求中找到 `Cookie` 头，复制 `t=` 后面的 token 值
6. Token 通常以 `dp_` 开头

### 步骤 2: 登录认证

```bash
taskbridge auth login dida
```

### 别名支持

滴答清单支持以下别名：

- `dida` - 推荐使用
- `ticktick_cn` - TickTick 国内版
- `tick-cn` - 简写形式

---

## Todoist

Todoist 使用 API Token 认证。

### 步骤 1: 获取 API Token

1. 登录 [Todoist](https://todoist.com)
2. 点击右上角头像 > **设置**
3. 选择 **集成** 选项卡
4. 找到 **API Token** 部分
5. 复制显示的 Token

### 步骤 2: 登录认证

```bash
taskbridge auth login todoist
```

按提示输入 API Token。

---

## 常用命令

### 查看认证状态

```bash
# 查看所有 provider 状态
taskbridge auth status

# 查看特定 provider 状态
taskbridge auth status google
taskbridge auth status microsoft
taskbridge auth status feishu
taskbridge auth status ticktick
taskbridge auth status dida
taskbridge auth status todoist
```

### 刷新 Token

```bash
taskbridge auth refresh google
taskbridge auth refresh microsoft
taskbridge auth refresh feishu
taskbridge auth refresh ticktick  # 静态 token，仅校验
taskbridge auth refresh dida      # 静态 token，仅校验
taskbridge auth refresh todoist
```

### 登出

```bash
taskbridge auth logout google
```

---

## 故障排除

### Token 过期

如果遇到 token 过期错误：

```bash
# 刷新 token
taskbridge auth refresh <provider>

# 或重新登录
taskbridge auth login <provider>
```

### 凭证文件未找到

确保凭证文件位于正确的位置：

```
~/.taskbridge/
├── credentials/
│   ├── google.json
│   ├── microsoft.json
│   └── feishu.json
└── tokens/
    ├── google.json
    ├── microsoft.json
    ├── feishu.json
    ├── ticktick.json
    ├── dida.json
    └── todoist.json
```

### 端口被占用

如果 OAuth 回调端口被占用，可以修改凭证文件中的 `redirect_url` 使用其他端口。

### 权限不足

确保在各个平台配置了正确的 API 权限/作用域。

---

## 配置文件示例

完整的配置文件 `~/.taskbridge/config.yaml`：

```yaml
mcp:
  enabled: true
  transport: stdio
  port: 14940

providers:
  microsoft:
    enabled: false
  google:
    enabled: false
  feishu:
    enabled: false
  ticktick:
    enabled: false
  dida:
    enabled: false
  todoist:
    enabled: false

storage:
  type: file
  path: ~/.taskbridge/data

sync:
  auto: false
  interval: 5m
```

启用需要使用的 provider 后即可开始同步任务。
