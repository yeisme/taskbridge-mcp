# 🦞 OpenClaw 学习指南

## 项目简介

**OpenClaw** 是一个开源的个人 AI 助手，您可以在自己的设备上运行。它支持多种通信渠道，包括 WhatsApp、Telegram、Slack、Discord、Google Chat、Signal、iMessage、Microsoft Teams、WebChat 等。

- **GitHub**: https://github.com/openclaw/openclaw
- **官网**: https://openclaw.ai
- **文档**: https://docs.openclaw.ai
- **Stars**: 226,578+ ⭐
- **语言**: TypeScript
- **许可证**: MIT

## 核心特性

### 1. 本地优先的 Gateway
- 单一控制平面管理会话、渠道、工具和事件
- 支持 WebSocket 控制平面
- 包含 Control UI 和 Canvas Host

### 2. 多渠道支持
| 渠道 | 描述 |
|------|------|
| WhatsApp | 通过 Baileys 实现 |
| Telegram | 通过 grammY 实现 |
| Slack | 通过 Bolt 实现 |
| Discord | 通过 discord.js 实现 |
| Google Chat | 通过 Chat API 实现 |
| Signal | 通过 signal-cli 实现 |
| BlueBubbles | iMessage（推荐） |
| iMessage | 传统 imsg |
| Microsoft Teams | 扩展支持 |
| Matrix | 扩展支持 |
| Zalo | 扩展支持 |
| WebChat | 内置 |

### 3. 多代理路由
- 将入站渠道/账户/对等点路由到隔离的代理
- 支持工作区和每个代理的会话

### 4. 语音功能
- **Voice Wake**: macOS/iOS/Android 的始终在线语音
- **Talk Mode**: 与 ElevenLabs 集成的对话模式

### 5. Live Canvas
- 代理驱动的可视化工作空间
- 支持 A2UI (Agent-to-UI)

### 6. 模型支持
- OpenAI (ChatGPT/Codex)
- Anthropic (推荐 Claude Opus 4.6)
- 支持 OAuth 和 API 密钥认证
- 模型故障转移和配置文件轮换

## 安装指南

### 前置要求
- **Node.js ≥ 22**
- npm / pnpm / bun

### 推荐安装方式

```bash
# 使用 npm 安装
npm install -g openclaw@latest

# 或使用 pnpm
pnpm add -g openclaw@latest

# 运行入门向导（推荐）
openclaw onboard --install-daemon
```

向导会安装 Gateway 守护进程（launchd/systemd 用户服务），使其保持运行。

### 从源码构建（开发模式）

```bash
# 克隆仓库
git clone https://github.com/openclaw/openclaw.git
cd openclaw

# 安装依赖（推荐使用 pnpm）
pnpm install

# 构建 UI（首次运行时自动安装 UI 依赖）
pnpm ui:build

# 构建项目
pnpm build

# 运行入门向导
pnpm openclaw onboard --install-daemon

# 开发循环（TS 更改时自动重载）
pnpm gateway:watch
```

## 快速入门

### 1. 启动 Gateway

```bash
openclaw gateway --port 18789 --verbose
```

### 2. 发送消息

```bash
openclaw message send --to +1234567890 --message "Hello from OpenClaw"
```

### 3. 与助手对话

```bash
# 基本对话
openclaw agent --message "Ship checklist"

# 带思考模式的对话
openclaw agent --message "Ship checklist" --thinking high
```

## 核心概念

### Session 模型
- `main`: 用于直接聊天
- 支持群组隔离
- 激活模式和队列模式
- 回复功能

### 安全默认设置
- **DM 配对**: 未知发送者会收到配对码
- 批准配对: `openclaw pairing approve <channel> <code>`
- 公开入站 DM 需要显式选择加入

### 工具系统
- 浏览器工具
- Canvas 工具
- Nodes 工具
- Cron 调度
- 会话管理
- Discord/Slack 操作

## 项目结构

```
openclaw/
├── packages/              # 核心包
│   ├── core/             # 核心功能
│   ├── gateway/          # Gateway 服务
│   ├── cli/              # 命令行工具
│   └── ...
├── docs/                  # 文档
├── extensions/            # 扩展
└── templates/             # 模板
```

## 开发渠道

| 渠道 | 描述 |
|------|------|
| **stable** | 标签发布版本 (`vYYYY.M.D`)，npm dist-tag `latest` |
| **beta** | 预发布版本 (`vYYYY.M.D-beta.N`)，npm dist-tag `beta` |
| **dev** | `main` 分支的最新代码，npm dist-tag `dev` |

切换渠道:
```bash
openclaw update --channel stable|beta|dev
```

## 学习资源

### 官方文档
- [入门指南](https://docs.openclaw.ai/start/getting-started)
- [更新指南](https://docs.openclaw.ai/install/updating)
- [展示案例](https://docs.openclaw.ai/start/showcase)
- [常见问题](https://docs.openclaw.ai/help/faq)
- [入门向导](https://docs.openclaw.ai/start/wizard)

### 核心文档
- [Gateway](https://docs.openclaw.ai/gateway) - 控制平面
- [Channels](https://docs.openclaw.ai/channels) - 渠道配置
- [Tools](https://docs.openclaw.ai/tools) - 工具系统
- [Models](https://docs.openclaw.ai/concepts/models) - 模型配置
- [Security](https://docs.openclaw.ai/gateway/security) - 安全指南

### 社区
- [Discord](https://discord.gg/clawd) - 社区讨论
- [GitHub Issues](https://github.com/openclaw/openclaw/issues) - 问题反馈
- [DeepWiki](https://deepwiki.com/openclaw/openclaw) - 深度 Wiki

### 平台特定
- [macOS 应用](https://docs.openclaw.ai/platforms/macos) - 菜单栏应用
- [iOS/Android Nodes](https://docs.openclaw.ai/nodes) - 移动端节点
- [Docker](https://docs.openclaw.ai/install/docker) - Docker 部署
- [Nix](https://github.com/openclaw/nix-openclaw) - Nix 打包

## 常用命令

```bash
# 诊断检查
openclaw doctor

# 查看版本
openclaw version

# 更新
openclaw update

# 配置模型
openclaw models

# 管理配对
openclaw pairing approve <channel> <code>
openclaw pairing list
```

## 推荐配置

### 模型推荐
虽然支持任何模型，但强烈推荐:
- **Anthropic Pro/Max (100/200) + Opus 4.6**
- 原因: 长上下文强度和更好的提示注入抵抗力

### 平台支持
- **macOS**: 完整支持（菜单栏应用 + Canvas）
- **Linux**: 完整支持
- **Windows**: 通过 WSL2（强烈推荐）

## 下一步

1. **安装**: 运行 `openclaw onboard --install-daemon`
2. **配置渠道**: 选择您使用的消息平台进行配置
3. **配置模型**: 设置 AI 模型（推荐 Claude）
4. **测试**: 发送测试消息验证配置
5. **探索**: 查看文档了解更多功能

## 注意事项

- 处理真实的消息界面，将入站 DM 视为**不受信任的输入**
- 默认 DM 策略需要配对才能处理消息
- 运行 `openclaw doctor` 检查风险/错误配置的 DM 策略
- Windows 用户建议使用 WSL2

---

*最后更新: 2026-02-25*
