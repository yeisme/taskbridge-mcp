# TaskBridge CLI 子命令设计

## 命令结构概览

```
taskbridge [command] [subcommand] [flags]

Commands:
  auth      认证管理
  sync      同步任务
  list      列出任务
  lists     列出清单
  analyze   分析任务
  mcp       MCP 后台服务
  tui       交互式终端界面
  task      任务管理
  config    配置管理
  provider  Provider 管理
  version   版本信息
```

---

## 1. auth - 认证管理

```
taskbridge auth [subcommand]

Subcommands:
  login <provider>    登录指定 Provider
  logout <provider>   登出指定 Provider
  status              查看所有 Provider 的认证状态
  refresh <provider>  刷新指定 Provider 的 token
  whoami              显示当前用户信息

Examples:
  taskbridge auth login google
  taskbridge auth login microsoft
  taskbridge auth status
  taskbridge auth logout google
  taskbridge auth whoami
```

### auth status 输出示例

```
┌─────────────────────────────────────────────────────────────┐
│Provider         │Status    │User                │Expires   │
├─────────────────────────────────────────────────────────────┤
│Google Tasks     │✓ Connected│user@gmail.com     │23h 45m   │
│Microsoft Todo   │✗ Not authenticated│-          │-         │
│Feishu           │✗ Not configured  │-          │-         │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. sync - 同步任务

```
taskbridge sync [flags]

Flags:
  -s, --source string      指定同步源 (google, microsoft, feishu, all)
  -m, --mode string        同步模式 (pull, push, bidirectional)
  -f, --force              强制同步，忽略缓存
  --dry-run                模拟运行，不实际执行
  --watch                  持续监听并同步

Examples:
  taskbridge sync                          # 同步所有已配置的 Provider
  taskbridge sync --source google          # 只同步 Google
  taskbridge sync --mode pull              # 只拉取
  taskbridge sync --watch                  # 持续同步
  taskbridge sync --dry-run                # 预览变更
```

---

## 3. list - 列出任务

```
taskbridge list [flags]

Flags:
  -f, --format string      输出格式 (table, json, markdown)
  -q, --quadrant int       按象限筛选 (1-4)
  -p, --priority int       按优先级筛选 (1-4)
  -t, --status string      按状态筛选 (todo, in_progress, completed, cancelled, deferred)
      --tag string         按标签筛选
      --list stringArray   按清单名称筛选（可重复）
      --list-id stringArray按清单 ID 筛选（可重复）
      --id stringArray     按任务 ID 筛选（可重复）
      --query string       关键词/自然语言文本过滤（本地匹配）
      --sync-now           查询前先执行 pull 同步
  -a, --all                显示所有状态（未显式传 -t 时生效）

Examples:
  taskbridge list
  taskbridge list --source ms --list 学习与成长
  taskbridge list --source microsoft --list-id <list_id>
  taskbridge list --source ms --list 学习与成长 --id <task_id>
  taskbridge list --sync-now --source google
```

### 3.1 lists - 列出清单

```
taskbridge lists [flags]

Flags:
  -s, --source string   数据源（支持简写：ms/g/tick/todo）
  -f, --format string   输出格式 (table, json)
      --sync-now        查询前先执行 pull 同步

Examples:
  taskbridge lists
  taskbridge lists --source ms
  taskbridge lists --source ms --format json
```

---

## 4. analyze - 分析任务

```
taskbridge analyze [subcommand] [flags]

Subcommands:
  quadrant    四象限分析
  priority    优先级分布
  time        时间分析
  trend       趋势分析
  report      生成完整报告

Flags:
  -p, --period string      分析周期 (today, week, month, all)
  -f, --format string      输出格式 (text, json, chart, html)
  -o, --output string      输出到文件

Examples:
  taskbridge analyze quadrant              # 四象限分析
  taskbridge analyze priority --period week# 本周优先级分布
  taskbridge analyze time                  # 时间分布分析
  taskbridge analyze trend                 # 趋势分析
  taskbridge analyze report -f html        # 生成 HTML 报告
```

### analyze quadrant 输出示例

```
╔══════════════════════════════════════════════════════════════╗
║                    艾森豪威尔四象限视图                        ║
╠══════════════════════════════════════════════════════════════╣
║                                                              ║
║    ┌─────────────────────┬─────────────────────┐             ║
║    │   🔥 Q1 紧急且重要   │   ⚡ Q3 紧急不重要   │             ║
║    │   立即做 (3)         │   授权做 (1)         │             ║
║    ├─────────────────────┼─────────────────────┤             ║
║    │   📋 Q2 重要不紧急   │   🗑️ Q4 不紧急不重要 │             ║
║    │   计划做 (5)         │   删除/延后 (2)      │             ║
║    └─────────────────────┴─────────────────────┘             ║
║                                                              ║
╠══════════════════════════════════════════════════════════════╣
║  💡 AI 建议:                                                 ║
║  - Q1 任务较多，建议优先处理"完成项目报告"                    ║
║  - Q2 任务健康，继续保持                                      ║
║  - Q4 任务可考虑删除或委托                                    ║
╚══════════════════════════════════════════════════════════════╝
```

---

## 5. mcp - MCP 后台服务

```
taskbridge mcp [subcommand] [flags]

Subcommands:
  start      启动 MCP 服务
  stop       停止 MCP 服务
  status     查看服务状态
  logs       查看服务日志

Flags:
  -t, --transport string   传输方式 (stdio, tcp, websocket)
  -p, --port int           TCP/WebSocket 端口 (默认: 8080)
  -d, --daemon             后台运行
      --sync-interval      同步间隔 (默认: 5m)

Examples:
  taskbridge mcp start                     # 前台启动
  taskbridge mcp start --daemon            # 后台启动
  taskbridge mcp start --transport tcp     # TCP 模式
  taskbridge mcp status                    # 查看状态
  taskbridge mcp logs                      # 查看日志
  taskbridge mcp stop                      # 停止服务

常用 MCP 工具（节选）:
  - list_tasks      支持 source/list_id/list_name/task_id/status/priority/query 等过滤
  - list_task_lists 列出清单及本地任务计数
  - get_prompt      获取提示词，含 json_query_commands（生成 jq/rg + fallback 命令模板）
```

---

## 6. tui - 交互式终端界面

```
taskbridge tui [flags]

Flags:
  -v, --view string        初始视图 (tasks, quadrant, calendar, timeline)

交互快捷键:
  Tab/Shift+Tab           切换面板
  ↑/↓                     导航任务列表
  Enter                   查看任务详情
  e                       编辑任务
  d                       删除任务
  1-4                     设置象限
  p                       设置优先级
  s                       同步
  /                       搜索
  ?                       帮助
  q/Ctrl+C                退出

TUI 界面布局:
┌─────────────────────────────────────────────────────────────┐
│ TaskBridge - 任务管理                        同步: 2分钟前  │
├───────────────────┬─────────────────────────────────────────┤
│                   │                                         │
│   🔥 Q1 (3)       │   任务列表                              │
│   📋 Q2 (5)       │   ─────────────────────────────────────│
│   ⚡ Q3 (1)       │   [ ] 完成项目报告         🔴 今天      │
│   🗑️ Q4 (2)       │   [x] 回复客户邮件         ✅ 已完成    │
│                   │   [ ] 学习新技术           🟡 明天      │
│   ─────────────── │   [ ] 整理文档             🔵 本周      │
│   总计: 11        │                                         │
│   已完成: 3       │   ─────────────────────────────────────│
│   过期: 1         │   详情: 完成项目报告                     │
│                   │   截止: 2024-02-21                      │
│                   │   优先级: 紧急                          │
│                   │   来源: Google Tasks                    │
├───────────────────┴─────────────────────────────────────────┤
│ 按 ? 查看帮助 │ Tab 切换面板 │ Enter 查看 │ q 退出         │
└─────────────────────────────────────────────────────────────┘
```

---

## 7. task - 任务管理

```
taskbridge task [subcommand] [flags]

Subcommands:
  add         添加任务
  edit        编辑任务
  delete      删除任务
  done        完成任务
  show        查看任务详情
  move        移动任务到其他列表

Flags for add:
  -t, --title string       任务标题
  -d, --description string 任务描述
      --due string         截止日期 (today, tomorrow, YYYY-MM-DD)
  -q, --quadrant int       象限 (1-4)
  -p, --priority int       优先级 (1-4)
      --tag strings        标签
  -s, --source string      目标 Provider

Examples:
  taskbridge task add "完成报告" --due today --priority 4
  taskbridge task add "学习新技术" --quadrant 2 --tag learning
  taskbridge task edit <task-id> --priority 3
  taskbridge task done <task-id>
  taskbridge task delete <task-id>
  taskbridge task show <task-id>
  taskbridge task move <task-id> --list "工作"
```

---

## 8. config - 配置管理

```
taskbridge config [subcommand] [flags]

Subcommands:
  show        显示当前配置
  set         设置配置项
  get         获取配置项
  init        初始化配置文件
  validate    验证配置

Examples:
  taskbridge config show
  taskbridge config set sync.interval 10m
  taskbridge config get providers.google.enabled
  taskbridge config init
  taskbridge config validate
```

---

## 9. provider - Provider 管理

```
taskbridge provider [subcommand] [flags]

Subcommands:
  list        列出所有 Provider
  enable      启用 Provider
  disable     禁用 Provider
  configure   配置 Provider
  test        测试 Provider 连接

Examples:
  taskbridge provider list
  taskbridge provider enable google
  taskbridge provider disable microsoft
  taskbridge provider configure google --client-id xxx --client-secret xxx
  taskbridge provider test google
```

### provider list 输出示例

```
┌─────────────────────────────────────────────────────────────┐
│ Provider         │Enabled   │Authenticated│Capabilities    │
├─────────────────────────────────────────────────────────────┤
│ Google Tasks     │✓ Yes     │✓ Yes        │due_date,subtask│
│ Microsoft Todo   │✓ Yes     │✗ No         │full            │
│ Feishu           │✗ No      │-            │-               │
│ TickTick         │✗ No      │-            │-               │
│ Todoist          │✗ No      │-            │-               │
└─────────────────────────────────────────────────────────────┘
```

---

## 10. version - 版本信息

```
taskbridge version [flags]

Flags:
  -v, --verbose    显示详细信息

Examples:
  taskbridge version
  taskbridge version --verbose

输出示例:
TaskBridge v1.0.0
  Go version: go1.22.0
  Platform: windows/amd64
  Build time: 2024-02-21T12:00:00Z
  Git commit: abc1234
```

---

## 依赖库

### Bubbletea + Lipgloss (TUI)

```go
// go.mod
require (
    github.com/charmbracelet/bubbletea v0.25.0
    github.com/charmbracelet/lipgloss v0.9.1
    github.com/charmbracelet/bubbles v0.17.1
)
```

### 主要组件

| 组件                | 用途       |
| ------------------- | ---------- |
| `bubbletea`         | TUI 框架   |
| `lipgloss`          | 样式和布局 |
| `bubbles/table`     | 表格组件   |
| `bubbles/list`      | 列表组件   |
| `bubbles/textinput` | 输入框     |
| `bubbles/viewport`  | 滚动视图   |
| `bubbles/spinner`   | 加载动画   |
| `bubbles/progress`  | 进度条     |

---

## 目录结构更新

```
cmd/
├── root.go
├── auth.go              # auth 命令
├── auth_login.go        # auth login 子命令
├── auth_logout.go       # auth logout 子命令
├── auth_status.go       # auth status 子命令
├── sync.go              # sync 命令
├── list.go              # list 命令
├── analyze.go           # analyze 命令
├── analyze_quadrant.go  # analyze quadrant 子命令
├── analyze_priority.go  # analyze priority 子命令
├── mcp.go               # mcp 命令
├── tui.go               # tui 命令
├── task.go              # task 命令
├── task_add.go          # task add 子命令
├── task_edit.go         # task edit 子命令
├── config.go            # config 命令
├── provider.go          # provider 命令
└── version.go           # version 命令

internal/
├── tui/                 # TUI 组件
│   ├── app.go           # 主应用
│   ├── views/           # 视图
│   │   ├── tasks.go     # 任务列表视图
│   │   ├── quadrant.go  # 四象限视图
│   │   ├── calendar.go  # 日历视图
│   │   └── detail.go    # 任务详情视图
│   ├── components/      # 组件
│   │   ├── tasklist.go  # 任务列表组件
│   │   ├── sidebar.go   # 侧边栏
│   │   └── statusbar.go # 状态栏
│   └── styles/          # 样式
│       └── styles.go    # Lipgloss 样式定义
├── output/              # 输出格式化
│   ├── json.go
│   ├── yaml.go
│   ├── table.go
│   ├── markdown.go
│   └── chart.go
```
