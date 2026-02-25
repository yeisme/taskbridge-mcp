# Microsoft To Do 配置指南

本指南将帮助您一步步配置 Microsoft To Do Provider。

## 前提条件

- 拥有 Microsoft 账户（个人账户或工作/学校账户）
- 可以访问 Azure Portal

## 步骤 1：访问 Azure Portal

1. 打开浏览器，访问 [Azure Portal](https://portal.azure.com/)
2. 使用您的 Microsoft 账户登录

## 步骤 2：注册应用程序

1. 在 Azure Portal 首页，搜索并点击 **"Microsoft Entra ID"**（原 Azure Active Directory）
2. 在左侧菜单中，点击 **"应用注册"**
3. 点击顶部的 **"新注册"** 按钮

## 步骤 3：填写应用信息

在注册页面填写以下信息：

| 字段                  | 值                                                                                                             |
| --------------------- | -------------------------------------------------------------------------------------------------------------- |
| **名称**              | `TaskBridge`（或您喜欢的任何名称）                                                                             |
| **受支持的帐户类型**  | 选择 **"任何组织目录(任何 Microsoft Entra ID 租户 - 多租户)中的帐户和个人 Microsoft 帐户(例如，Skype、Xbox)"** |
| **重定向 URI (可选)** | 选择 **Web**，输入 `http://localhost:8080/callback`                                                            |

4. 点击 **"注册"** 按钮

## 步骤 4：获取应用 ID 和租户 ID

注册成功后，您会看到应用的概览页面：

1. 复制 **应用程序(客户端) ID** - 这是您的 `client_id`
2. 复制 **目录(租户) ID** - 这是您的 `tenant_id`（可以使用 `common` 代替）

## 步骤 5：创建客户端密钥

1. 在应用页面左侧菜单中，点击 **"证书和密码"**
2. 点击 **"新客户端密码"** 按钮
3. 输入描述（如 "TaskBridge Key"）
4. 选择过期时间（建议选择 "180天" 或 "365天"）
5. 点击 **"添加"**
6. **立即复制密钥值** - 这是您的 `client_secret`（⚠️密钥只显示一次，请务必保存！）

## 步骤 6：配置 API 权限

1. 在应用页面左侧菜单中，点击 **"API 权限"**
2. 点击 **"添加权限"**
3. 选择 **"Microsoft Graph"**
4. 选择 **"委托的权限"**
5. 搜索并勾选以下权限：
   - `Tasks.ReadWrite` - 读取和写入用户的任务和任务列表
   - `User.Read` - 登录并读取用户配置文件
6. 点击 **"添加权限"**

## 步骤 7：创建凭证文件

在以下路径创建凭证文件：

**Windows:** `C:\Users\<用户名>\.taskbridge\credentials\microsoft_credentials.json`

**macOS/Linux:** `~/.taskbridge/credentials/microsoft_credentials.json`

文件内容如下：

```json
{
  "client_id": "你的应用程序(客户端) ID",
  "client_secret": "你的客户端密钥值",
  "tenant_id": "common",
  "redirect_url": "http://localhost:8080/callback"
}
```

### 示例

```json
{
  "client_id": "12345678-1234-1234-1234-123456789012",
  "client_secret": "abc123~XYZ456-xxx-xxx",
  "tenant_id": "common",
  "redirect_url": "http://localhost:8080/callback"
}
```

## 步骤 8：登录认证

运行以下命令进行认证：

```bash
taskbridge auth login microsoft
```

系统会自动打开浏览器，请：

1. 使用您的 Microsoft 账户登录
2. 授予应用程序所需的权限
3. 等待页面显示 "认证成功"
4. 返回终端查看认证结果

## 故障排除

### 问题：提示 "凭证文件不存在"

**解决方案：** 确保凭证文件路径正确，并且文件扩展名是 `.json`

### 问题：认证失败 "AADSTS50011"

**解决方案：** 检查 Azure Portal 中的重定向 URI 是否与凭证文件中的 `redirect_url` 完全一致

### 问题：提示 "需要管理员同意"

**解决方案：** 如果使用的是工作/学校账户，可能需要管理员批准应用权限。尝试使用个人 Microsoft 账户。

### 问题：Token 过期

**解决方案：** 运行以下命令刷新 Token：

```bash
taskbridge auth refresh microsoft
```

## 验证配置

运行以下命令验证配置是否成功：

```bash
# 查看认证状态
taskbridge auth status

# 测试连接
taskbridge provider test microsoft

# 拉取任务
taskbridge sync pull microsoft
```

## 安全建议

1. **不要分享**您的 `client_secret`
2. **定期轮换**客户端密钥
3. **仅授予必要的权限**
4. **不要**将凭证文件提交到版本控制系统

## 下一步

配置完成后，您可以：

- 同步 Microsoft To Do 任务：`taskbridge sync pull microsoft`
- 双向同步：`taskbridge sync bidirectional microsoft`
- 查看任务列表：`taskbridge list`
