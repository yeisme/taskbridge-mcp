# CLI 输出美化计划

## 概述

使用 `charmbracelet/lipgloss` 库美化 TaskBridge CLI 的所有命令输出，解决中文字符对齐问题，提升用户体验。

## 当前问题

### 1. 表格对齐问题

```
名称                简写       状态      认证方式                描述
────              ────     ────    ────────            ────
Google Tasks      google   ❌ 未启用   OAuth2              Google 任务管理服务
Microsoft To Do   ms       ❌ 未启用   OAuth2              微软任务管理服务
飞书任务              feishu   ❌ 未启用   App ID/Secret       飞书任务管理
```

问题：中文字符（如"飞书任务"）的显示宽度与英文字符不同，导致 tabwriter无法正确对齐。

### 2. 样式不统一

- 不同命令使用不同的输出风格
- 缺乏统一的颜色主题
- 错误、成功、警告消息样式不一致

## 解决方案

### 1. 创建 UI 样式模块

创建 `pkg/ui/` 目录，包含：

```
pkg/ui/
├── styles.go      # lipgloss 样式定义
├── table.go       # 表格渲染（支持中文宽度）
├── components.go  # 可复用 UI 组件
└── icons.go       # 图标和符号定义
```

### 2. 样式定义 (styles.go)

```go
package ui

import "github.com/charmbracelet/lipgloss"

// 颜色主题
var (
    // 主色调
    PrimaryColor   = lipgloss.Color("#7C3AED")  // 紫色
    SecondaryColor = lipgloss.Color("#3B82F6")  // 蓝色

    // 状态颜色
    SuccessColor = lipgloss.Color("#10B981")  // 绿色
    ErrorColor   = lipgloss.Color("#EF4444")  // 红色
    WarningColor = lipgloss.Color("#F59E0B")  // 黄色
    InfoColor    = lipgloss.Color("#6B7280")  // 灰色
)

// 基础样式
var (
    TitleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(PrimaryColor).
        MarginBottom(1)

    HeaderStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#FFFFFF")).
        Background(PrimaryColor).
        Padding(0, 1)

    RowStyle = lipgloss.NewStyle().
        Padding(0, 1)

    SelectedStyle = lipgloss.NewStyle().
        Background(lipgloss.Color("#374151")).
        Foreground(lipgloss.Color("#FFFFFF"))
)

// 状态样式
var (
    SuccessStyle = lipgloss.NewStyle().
        Foreground(SuccessColor).
        Bold(true)

    ErrorStyle = lipgloss.NewStyle().
        Foreground(ErrorColor).
        Bold(true)

    WarningStyle = lipgloss.NewStyle().
        Foreground(WarningColor)

    EnabledStyle = SuccessStyle
    DisabledStyle = lipgloss.NewStyle().
        Foreground(InfoColor)
)
```

### 3. 表格组件 (table.go)

```go
package ui

import (
    "github.com/charmbracelet/lipgloss"
    "github.com/charmbracelet/lipgloss/table"
    "github.com/mattn/go-runewidth"
)

// Table 表格组件
type Table struct {
    headers []string
    rows    [][]string
    style   table.Style
}

// NewTable 创建表格
func NewTable(headers ...string) *Table {
    return &Table{
        headers: headers,
        style:   DefaultTableStyle(),
    }
}

// AddRow 添加行
func (t *Table) AddRow(cells ...string) *Table {
    t.rows = append(t.rows, cells)
    return t
}

// Render 渲染表格
func (t *Table) Render() string {
    // 使用 lipgloss/table 正确处理中文宽度
}

// StringWidth 计算字符串显示宽度（支持中文）
func StringWidth(s string) int {
    return runewidth.StringWidth(s)
}
```

### 4. 需要修改的命令

#### 4.1 provider list

**当前输出：**

```
名称                简写       状态      认证方式                描述
────              ────     ────    ────────            ────
Google Tasks      google   ❌ 未启用   OAuth2              Google 任务管理服务
```

**目标输出：**

```
┌──────────────────┬─────────┬──────────┬───────────────┬────────────────────┐
│ 名称             │ 简写    │ 状态     │ 认证方式      │ 描述               │
├──────────────────┼─────────┼──────────┼───────────────┼────────────────────┤
│ Google Tasks     │ google  │ ❌ 未启用 │ OAuth2        │ Google 任务管理服务│
│ Microsoft To Do  │ ms      │ ❌ 未启用 │ OAuth2        │ 微软任务管理服务   │
│ 飞书任务         │ feishu  │ ❌ 未启用 │ App ID/Secret │ 飞书任务管理       │
│ TickTick         │ tick    │ ❌ 未启用 │ User/Pass     │ TickTick 任务管理  │
│ Todoist          │ todo    │ ❌ 未启用 │ API Token     │ Todoist 任务管理   │
└──────────────────┴─────────┴──────────┴───────────────┴────────────────────┘
```

#### 4.2 auth status

**当前输出：**

```
┌─────────────────────────────────────────────────────────────────┐
│ Provider        │ Status             │ User        │ Expires  │
├─────────────────────────────────────────────────────────────────┤
│ Google Tasks    │ ❌ Not configured   │ -           │ -        │
│ Microsoft Todo  │ ❌ Not configured   │ -           │ -        │
│ 飞书任务            │ ❌ Not configured   │ -           │ -        │
└─────────────────────────────────────────────────────────────────┘
```

**目标输出：**

```
╭────────────────────┬──────────────────┬───────────┬──────────╮
│ Provider           │ Status           │ User      │ Expires  │
├────────────────────┼──────────────────┼───────────┼──────────┤
│ Google Tasks       │ ❌ Not configured │ -         │ -        │
│ Microsoft To Do    │ ❌ Not configured │ -         │ -        │
│ 飞书任务           │ ❌ Not configured │ -         │ -        │
│ TickTick           │ ❌ Not configured │ -         │ -        │
│ Todoist            │ ❌ Not configured │ -         │ -        │
╰────────────────────┴──────────────────┴───────────┴──────────╯
```

#### 4.3 provider info

**目标输出：**

```
╭──────────────────────────────────────╮
│           📋 Google Tasks            │
├──────────────────────────────────────┤
│ 名称       Google                    │
│ 描述       Google 任务管理服务        │
│ 认证方式   OAuth2                    │
│ 状态       ❌ 未启用                  │
├──────────────────────────────────────┤
│ 支持的功能                           │
│   ✅ 截止日期                        │
│   ✅ 任务列表                        │
│   ✅ 子任务（有限支持）               │
│   ❌ 优先级（不支持）                │
│   ❌ 标签（不支持）                  │
│   ❌ 增量同步（不支持）              │
╰──────────────────────────────────────╯
```

#### 4.4 auth whoami

**目标输出：**

```
╭──────────────────────────────────────╮
│           📌 Google Tasks            │
├──────────────────────────────────────┤
│ 状态       ✅ 已认证                  │
│ Token类型  Bearer                    │
│ 过期时间   2026-02-22 23:48:56       │
│ 有效性     ✅ 有效                    │
╰──────────────────────────────────────╯
```

## 实施步骤

### 阶段 1：创建 UI 基础模块

1. 创建 `pkg/ui/styles.go` - 定义所有样式
2. 创建 `pkg/ui/table.go` - 表格组件
3. 创建 `pkg/ui/components.go` - 通用组件
4. 添加必要的依赖（如 `github.com/mattn/go-runewidth`）

### 阶段 2：更新命令输出

1. 更新 `cmd/provider.go`
   - `runProviderList()` - 使用新表格组件
   - `runProviderInfo()` - 使用卡片样式

2. 更新 `cmd/auth.go`
   - `printAuthStatusTable()` - 使用新表格组件
   - `showProviderUserInfo()` - 使用卡片样式

3. 更新 `cmd/task.go`
   - 任务列表输出

4. 更新 `cmd/list.go`
   - 列表输出

5. 更新 `cmd/sync.go`
   - 同步状态输出

### 阶段 3：测试和优化

1. 测试所有命令的输出
2. 确保中文字符正确对齐
3. 测试不同终端宽度下的显示效果
4. 优化颜色和样式

## 依赖

```go
require (
    github.com/charmbracelet/lipgloss v1.1.0      // 已有
    github.com/mattn/go-runewidth v0.0.20        // 已有（间接依赖）
)
```

## 参考资源

- [lipgloss 文档](https://github.com/charmbracelet/lipgloss)
- [lipgloss/table](https://github.com/charmbracelet/lipgloss/tree/main/table)
- [go-runewidth](https://github.com/mattn/go-runewidth)
