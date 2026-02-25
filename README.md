# TaskBridge MCP

<div align="center">

**连接 AI 与 Todo 软件的桥梁**

[English](#english) | [简体中文](#简体中文)

</div>

---

## 简体中文

### 项目简介

TaskBridge 是一个 MCP (Model Context Protocol) 工具，旨在连接各种 Todo 软件与 AI，让 AI 能够：

- 📋 **理解任务** - 读取和解析来自不同 Todo 软件的任务
- 🔄 **双向同步** - 支持从 Todo 软件读取和反向写入
- 🎯 **智能分析** - 提供四象限分析、优先级计算等高级功能
- 🤖 **AI 增强** - 为 AI 提供任务上下文，帮助 AI 更好地为用户规划

### 支持的平台

| 平台            | 状态      | 特点       |
| --------------- | --------- | ---------- |
| Microsoft Todo  | ✅ 已完成 | 完整支持   |
| Google Tasks    | ✅ 已完成 | 基础支持   |
| 飞书任务        | 🚧 开发中 | 完整支持   |
| TickTick        | 📋 计划中 | 原生四象限 |
| Todoist         | 📋 计划中 | 完整支持   |
| OmniFocus       | 📋 计划中 | macOS 专用 |
| Apple Reminders | 📋 计划中 | macOS/iOS  |

### 核心功能

#### 1. 统一任务模型

将不同 Todo 软件的任务抽象为统一的数据模型，包括：

- 基础字段（标题、描述、状态、时间）
- 四象限属性（紧急/重要程度）
- 优先级系统
- 元数据存储

#### 2. 四象限视图

基于艾森豪威尔矩阵的任务分类：

```
┌─────────────────────┬─────────────────────┐
│   🔥 Q1 紧急且重要   │   ⚡ Q3 紧急不重要   │
│   立即做             │   授权做             │
├─────────────────────┼─────────────────────┤
│   📋 Q2 重要不紧急   │   🗑️ Q4 不紧急不重要 │
│   计划做             │   删除/延后          │
└─────────────────────┴─────────────────────┘
```

#### 3. MCP 集成

提供 MCP Tools 供 AI 调用：

- `list_tasks` - 列出任务（支持 source/list/status/priority/query 等复杂过滤）
- `list_task_lists` - 列出清单（含 `list_id` 与本地任务计数）
- `create_task` - 创建任务
- `update_task` - 更新任务
- `delete_task` - 删除任务
- `sync_pull` / `sync_push` - 同步任务
- `get_prompt` - 获取提示词模板（含 `json_query_commands`）

### 快速开始

#### 安装

```bash
# 克隆仓库
git clone https://github.com/yeisme/taskbridge-mcp.git
cd taskbridge-mcp

# 安装依赖
go mod tidy

# 编译
go build -o taskbridge
```

#### 配置

```bash
# 复制配置文件
cp configs/config.yaml ~/.taskbridge/config.yaml

# 编辑配置
vim ~/.taskbridge/config.yaml
```

#### 使用

```bash
# 列出任务
./taskbridge list

# 按来源 + 清单过滤
./taskbridge list --source ms --list 学习与成长

# 按清单 ID 过滤
./taskbridge list --source ms --list-id <list_id>

# 同步后再查询
./taskbridge list --sync-now --source microsoft

# 列出清单（用于获取 list_id）
./taskbridge lists --source ms --format json

# 同步任务
./taskbridge sync

# 分析任务
./taskbridge analyze

# 启动后台服务
./taskbridge serve
```

### 项目结构

```
taskbridge-mcp/
├── cmd/                    # CLI 命令
├── internal/
│   ├── model/              # 核心数据模型
│   ├── provider/           # Todo 软件适配器
│   ├── storage/            # 存储层
│   ├── sync/               # 同步引擎
│   └── mcp/                # MCP 服务
├── pkg/
│   ├── config/             # 配置管理
│   └── logger/             # 日志
├── configs/                # 配置文件
└── templates/              # 输出模板
```

### 开发计划

- [x] Phase 1 - 基础框架
  - [x] 核心数据模型
  - [x] CLI 框架
  - [x] 配置管理
  - [x] 文件存储

- [x] Phase 2 - Provider 实现（核心）
  - [x] Microsoft Todo Provider
  - [x] Google Tasks Provider
  - [ ] 飞书 Provider

- [ ] Phase 3 - Provider 实现（扩展）
  - [ ] TickTick Provider
  - [ ] Todoist Provider

- [x] Phase 4 - 同步引擎
  - [x] 同步引擎核心
  - [x] 冲突解决机制
  - [ ] 定时调度器

- [x] Phase 5 - MCP 服务
  - [x] MCP Server 实现
  - [x] Tools 定义
  - [x] Resources 定义

- [x] Phase 6 - 高级功能
  - [x] 四象限分析
  - [x] 优先级计算
  - [x] AI 建议生成

### 技术栈

- **语言**: Go 1.21+
- **CLI**: Cobra
- **配置**: Viper
- **MCP SDK**: github.com/modelcontextprotocol/go-sdk
- **存储**: 文件存储 / MongoDB（可选）

### 贡献

欢迎贡献代码！请查看 [CONTRIBUTING.md](CONTRIBUTING.md) 了解详情。

### 许可证

MIT License

---

## English

### Overview

TaskBridge is an MCP (Model Context Protocol) tool that connects various Todo applications with AI, enabling AI to:

- 📋 **Understand Tasks** - Read and parse tasks from different Todo apps
- 🔄 **Two-way Sync** - Support reading from and writing to Todo apps
- 🎯 **Smart Analysis** - Provide quadrant analysis, priority calculation, etc.
- 🤖 **AI Enhancement** - Provide task context for AI to better plan for users

### Quick Start

```bash
# Clone the repository
git clone https://github.com/yeisme/taskbridge-mcp.git
cd taskbridge-mcp

# Install dependencies
go mod tidy

# Build
go build -o taskbridge

# Run
./taskbridge --help
```

### License

MIT License
