# 目标导向拆分 MVP 说明

更新时间：2026-02-25

## 1. 功能概述

本功能用于把自然语言目标（例如“我希望学习 openclaw”“我希望去上海旅游”）转成可执行任务，并通过 MCP 完成以下闭环：

1. 创建项目草稿
2. 生成拆分建议（不落库任务）
3. 用户确认后批量创建任务
4. 按 `project_id` 精准同步该项目任务到指定 provider

默认策略：

- `planning_mode = suggestion_then_confirm`
- `horizon_days = 14`
- 子任务粒度为 30-180 分钟
- 目标类型仅三类：`learning` / `travel` / `generic`

---

## 2. 架构与数据

### 2.1 模块位置

- 目标拆分引擎：`internal/projectplanner/`
- 项目存储：`internal/project/`
- MCP 对接：`internal/mcp/handlers.go`、`internal/mcp/server.go`

### 2.2 存储文件

- 项目与拆分建议：`<storage.path>/projects.json`
- 任务：`<storage.path>/tasks.json`

默认 `storage.path` 来自 `~/.taskbridge/config.yaml`。

### 2.3 任务关联元数据

确认项目后创建的任务会写入以下 metadata 字段：

- `tb_project_id`
- `tb_plan_id`
- `tb_goal_type`
- `tb_phase`
- `tb_step_index`

---

## 3. 意图识别规则

### 3.1 learning

命中关键词：`学习/了解/熟悉/精通/掌握`（含英文 learn/study/master）

### 3.2 travel

命中关键词：`去/旅游/出行/行程/攻略`（含英文 travel/trip/itinerary）

### 3.3 generic

未命中上述关键词时进入通用模板。

---

## 4. MCP 工具说明

## 4.1 `create_project`

用途：创建项目草稿。

输入（新增可选）：

- `name` (required)
- `description`
- `parent_id`
- `goal_text`
- `horizon_days`
- `list_id`
- `source`（支持 provider 简写）

返回重点：

- `id`
- `goal_type`
- `status`（初始为 `draft`）
- `latest_plan_id`

## 4.2 `list_projects`

用途：列出项目，可按状态过滤。

输入：

- `status`（可选：`draft/split_suggested/confirmed/synced`）

返回重点：

- `id,name,status,goal_type,created_at,updated_at,latest_plan_id,latest_plan_summary`

## 4.3 `split_project`

用途：生成拆分建议，不创建任务。

输入：

- `project_id` (required)
- `ai_hint`
- `goal_text`（临时覆盖）
- `horizon_days`
- `max_tasks`
- `constraints`（结构化硬约束，可选）

`constraints` 字段示例：

```json
{
  "require_deliverable": true,
  "min_estimate_minutes": 30,
  "max_estimate_minutes": 180,
  "min_tasks": 8,
  "max_tasks": 12,
  "min_practice_tasks": 2
}
```

返回重点：

- `project_id`
- `plan_id`
- `status=split_suggested`
- `confidence`
- `constraints`
- `phases`
- `tasks_preview`
- `warnings`

## 4.4 `confirm_project`

用途：确认拆分并落库任务（可关闭写入）。

输入：

- `project_id` (required)
- `plan_id`（默认使用最新计划）
- `write_tasks`（默认 `true`）

返回重点：

- `project_id`
- `status=confirmed`
- `created_task_ids`
- `count`

幂等行为：

- 同一 `project_id + plan_id` 已确认过时，不重复创建任务。

## 4.5 `sync_project`

用途：仅同步指定项目任务到 provider。

输入：

- `project_id` (required)
- `provider` (required)

同步筛选语义：

- 仅同步 `Metadata.CustomFields.tb_project_id == project_id` 的任务。

返回重点：

- `project_id`
- `provider`
- `status=synced`
- `pushed/updated/errors`

---

## 5. 端到端调用示例

## 5.1 示例 A：学习 OpenClaw

### Step 1 创建项目

```json
{
  "tool": "create_project",
  "arguments": {
    "name": "学习 OpenClaw",
    "goal_text": "我希望学习 openclaw",
    "horizon_days": 14
  }
}
```

### Step 2 生成拆分建议

```json
{
  "tool": "split_project",
  "arguments": {
    "project_id": "proj_xxx",
    "constraints": {
      "require_deliverable": true,
      "min_estimate_minutes": 30,
      "max_estimate_minutes": 180,
      "min_tasks": 8,
      "max_tasks": 12,
      "min_practice_tasks": 2
    }
  }
}
```

### Step 3 用户确认并创建任务

```json
{
  "tool": "confirm_project",
  "arguments": {
    "project_id": "proj_xxx",
    "plan_id": "plan_xxx",
    "write_tasks": true
  }
}
```

### Step 4 同步该项目任务

```json
{
  "tool": "sync_project",
  "arguments": {
    "project_id": "proj_xxx",
    "provider": "google"
  }
}
```

## 5.2 示例 B：去上海旅游

```json
{
  "tool": "create_project",
  "arguments": {
    "name": "上海旅游准备",
    "goal_text": "我希望去上海旅游",
    "horizon_days": 10
  }
}
```

后续流程同上：`split_project -> confirm_project -> sync_project`。

---

## 6. 常见问题

## 6.1 `split_project` 提示 `project storage not available`

确认 MCP 启动已注入 `WithProjectStore(...)`，并且 `storage.path` 可写。

## 6.2 `confirm_project` 成功但无任务创建

检查：

- `write_tasks` 是否被设置为 `false`
- `plan_id` 是否对应存在的拆分建议
- `tasks_preview` 是否为空

## 6.3 `sync_project` 结果为 0 条

说明未找到带 `tb_project_id=<project_id>` 的任务。先执行 `confirm_project`，再同步。

---

## 7. 验收建议

最小验收链路：

1. `create_project` 返回 `draft`
2. `split_project` 返回 `plan_id` 和 `tasks_preview`
3. `confirm_project` 返回 `count > 0`
4. `list_tasks` 能检索到带 `tb_project_id` 的任务
5. `sync_project` 仅同步该项目任务
