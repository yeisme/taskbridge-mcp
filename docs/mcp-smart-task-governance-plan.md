# MCP 智能任务治理与成就分析方案

更新时间：2026-03-04

## 1. 目标

围绕提出的 4 类需求，补齐 TaskBridge 的 MCP 能力：

1. 识别过时/延期任务并触发风险提醒，且在过载时引导用户决策（延期、删除、拆分、保留）。
2. 针对“无时间安排的长期任务”做动态调配，基于短期任务负载自动补位或收敛，并支持配置参数化。
3. 识别复杂/抽象任务，检测是否缺少子任务并提示拆分；拆分执行要根据用户已连接 provider 的能力选择策略。
4. 提供多维完成度分析与成就反馈，让用户能持续感受到进展。

---

## 2. 现状与差距

### 2.1 已有能力（可复用）

- MCP 工具框架、工具注册、提示词与资源机制完整。
- 已有任务查询与分析工具：`list_tasks`、`analyze_quadrant`、`analyze_priority`。
- 已有项目拆分链路：`create_project -> split_project/split_project_from_markdown -> confirm_project -> sync_project`。
- 已有 provider 能力探测接口与别名解析。

### 2.2 缺口

- 缺少“过期任务治理”专用工具与决策闭环（目前只有静态分析，没有策略执行与用户决策引导）。
- 缺少“长期任务动态调配”机制（没有短期/长期平衡规则）。
- 缺少“复杂任务检测 + 子任务缺失检查 + 跨 provider 拆分策略”的统一流程。
- 缺少“成就分析（连续性、趋势、勋章、叙事反馈）”能力。
- 缺少上述规则的统一配置模型（阈值、策略、提示行为等）。

---

## 3. 目标能力设计（对应 4 类需求）

## 3.1 过时任务治理（R1）

### 检测规则

- `overdue`: `due_date < now && status in [todo, in_progress]`
- `severe_overdue`: 逾期天数 >= `severe_days`
- `overload`: 逾期总数 > `overload_threshold`

### 输出分层

- **普通告警**：超过 `warning_threshold` 触发。
- **过载告警**：超过 `overload_threshold` 触发，并附“任务分配不合理”提示。
- **决策问题包**：自动返回可问用户的问题：
  - 这些是延期任务吗？
  - 哪些延期（批量选择）？
  - 是否删除低价值项？
  - 是否先拆分再排期？

### 执行动作

- `defer`：改状态为 `deferred`
- `reschedule`：重设 `due_date`
- `delete`：删除任务（默认需二次确认）
- `split_then_schedule`：转拆分流程

---

## 3.2 长期无排期任务调配（R2）

### 核心定义

- `long_term_unscheduled`: `due_date == nil && status in [todo, in_progress] && age_days >= min_age_days`
- `short_term_active`: `due_date in [today, today+window_days] && status in [todo, in_progress]`

### 平衡规则

- 当 `short_term_active < short_term_min`：
  - 从长期池按优先分拣选 `promote_count_when_shortage` 条补到短期（自动安排 due date）
- 当 `short_term_active > short_term_max`：
  - 长期池只保留 `retain_count_when_overflow` 条“活跃长期任务”，其余转入 backlog/deferred

### 排序依据（候选选择）

优先级从高到低：

1. `priority`
2. `quadrant`
3. 最近更新更近（避免沉没）
4. 标题含明确动词（可执行性）

---

## 3.3 复杂/抽象任务拆分建议（R3）

### 候选识别

复杂度分（0-100）由以下因子加权：

- 标题抽象关键词命中（如“优化/推进/完善/研究/整理”等）
- 标题长度过长、描述含多目标
- 无 `due_date`、无估时、无子任务
- 历史多次延期（可选）

`complexity_score >= complexity_threshold` 且 `无子任务` -> 候选。

### 跨 provider 策略

- 若目标 provider `SupportsSubtasks=true`：优先父子任务拆分。
- 若 `SupportsSubtasks=false`：改为“扁平任务 + 计划标签/阶段字段”（如 `tb_plan_id`、`tb_phase`）。
- 若用户连接多个 provider：
  - 优先当前任务来源 provider
  - 否则按“已认证 + 支持子任务 + 写入能力”自动推荐

### 与现有链路衔接

复用既有项目工具链：

- 将复杂任务映射为临时项目（内部 draft）
- 调用拆分工具生成建议
- 由用户确认后写入任务

---

## 3.4 完成情况与成就反馈（R4）

### 分析维度（至少 5 种）

1. **速度**：近 7/30 天完成量、日均完成量
2. **稳定性**：连续完成天数（streak）
3. **质量**：按时完成率、逾期修复率
4. **结构**：Q1~Q4 完成分布（是否只救火）
5. **趋势**：环比变化（本周 vs 上周、本月 vs 上月）

### 成就反馈

- 徽章（badges）示例：
  - `steady-7`: 连续 7 天有完成
  - `overdue-cleaner`: 7 天内处理逾期 >= N
  - `q2-builder`: Q2 完成占比达阈值
- 文案采用“事实 + 进步 + 下一步建议”三段式，避免空泛夸奖。

---

## 4. 建议新增 MCP 工具

为减少一次性改动，分为“只读诊断”与“可执行动作”两组。

| 工具名                            | 类型     | 作用                                   |
| --------------------------------- | -------- | -------------------------------------- |
| `analyze_overdue_health`          | 只读     | 逾期统计、过载检测、候选动作、提问建议 |
| `resolve_overdue_tasks`           | 写入     | 按用户决策批量延期/删除/改状态         |
| `rebalance_longterm_tasks`        | 可选写入 | 按短期负载规则补位或收敛长期任务       |
| `detect_decomposition_candidates` | 只读     | 识别复杂/抽象任务并给出拆分建议        |
| `decompose_task_with_provider`    | 写入     | 根据 provider 能力执行拆分落地         |
| `analyze_achievement`             | 只读     | 多维完成分析 + 徽章 + 正反馈叙事       |

---

## 5. 配置模型（支持参数可调）

建议新增 `mcp.intelligence` 配置段；默认路径与当前一致：`C:\Users\ye\.taskbridge\config.yaml`。

```yaml
mcp:
  intelligence:
    enabled: true
    timezone: "Asia/Shanghai"

    overdue:
      warning_threshold: 3
      overload_threshold: 10
      severe_days: 7
      ask_before_delete: true
      max_candidates: 30

    long_term:
      min_age_days: 7
      short_term_window_days: 7
      short_term_min: 5
      short_term_max: 10
      promote_count_when_shortage: 3
      retain_count_when_overflow: 1
      overflow_strategy: "defer" # defer | backlog_tag

    decomposition:
      complexity_threshold: 60
      detect_abstract_keywords: true
      ask_before_split: true
      preferred_strategy: "project_split" # project_split | markdown_split
      abstract_keywords:
        - "优化"
        - "推进"
        - "完善"
        - "研究"
        - "整理"

    achievement:
      snapshot_granularity: "daily"
      streak_goal_per_day: 1
      badge_enabled: true
      narrative_enabled: true
      compare_previous_period: true
```

> 说明：若继续坚持“环境变量优先”，则补充一组 `TASKBRIDGE_MCP_INTELLIGENCE_*` 映射即可，不影响结构设计。

---

## 6. 跨 Provider 执行流程（统一策略）

### 6.1 总流程

1. 拉取任务（必要时）
2. 读取候选任务集（按 source/list/status 过滤）
3. 执行规则引擎（逾期/长期/复杂度/成就）
4. 生成 `decision_pack`
5. AI 向用户提问并收集决策
6. 调用执行类工具落地（可 `dry_run`）
7. 输出结果与下一步建议

### 6.2 关键分支

- **Provider 不支持子任务**：自动改为“扁平拆分 + 元数据关联”。
- **Provider 未认证**：返回“可执行建议 + 认证指引”，不中断分析。
- **写操作默认保守**：删除、批量延期等高风险动作要求显式确认。

---

## 7. 数据结构与落库建议

### 7.1 任务元数据扩展（CustomFields）

建议增加（均为可选）：

- `tb_complexity_score`
- `tb_is_long_term`
- `tb_last_overdue_at`
- `tb_overdue_count`
- `tb_achievement_credit`

### 7.2 成就快照存储

新增本地文件（按天滚动）：

- `<storage.path>/insights-history.json`

记录：

- 日期
- 完成量
- 连续天数
- 逾期处理量
- 象限完成分布
- 已达成徽章

---

## 8. 实施路线（先 MVP，再迭代）

## Phase 1（MVP，1~2 周）

- 新增 3 个只读工具：
  - `analyze_overdue_health`
  - `detect_decomposition_candidates`
  - `analyze_achievement`
- 配置读入与默认值落地（先支持核心阈值）
- 返回标准化 `questions` 字段，驱动 AI 与用户交互

## Phase 2（执行闭环，1~2 周）

- 新增执行工具：
  - `resolve_overdue_tasks`
  - `rebalance_longterm_tasks`
  - `decompose_task_with_provider`
- 引入 `dry_run` + 批量操作回执
- 高风险动作加 confirm token（防误删）

## Phase 3（增强体验，1 周）

- 成就徽章与周期报告模板
- 个性化建议（基于用户历史模式）
- TUI/CLI 汇总入口（可选）

---

## 9. 验收标准

1. 逾期任务超过阈值时，能给出清晰告警与可执行决策选项。
2. 短期任务低于 5 能自动补位；高于 10 能执行长期保留策略（阈值可配置）。
3. 对复杂且无子任务项能稳定识别，并按 provider 能力给出可落地拆分路径。
4. 至少提供 5 个维度的完成分析，并生成具“事实依据”的成就反馈。
5. 所有策略参数可通过配置修改并在 MCP 输出中回显当前生效值。

---

## 10. 工具接口草案（MCP）

### 10.1 `analyze_overdue_health`

输入：

```json
{
  "source": "microsoft",
  "list_id": ["xxx"],
  "include_suggestions": true
}
```

输出（节选）：

```json
{
  "summary": {
    "overdue_count": 12,
    "severe_overdue_count": 4,
    "is_overload": true
  },
  "questions": [
    "这些逾期任务是否属于延期任务？",
    "请确认需要延期的任务 ID 列表",
    "是否删除低价值且逾期超过 14 天的任务？"
  ],
  "actions": ["defer", "reschedule", "delete", "split_then_schedule"],
  "config_applied": {
    "warning_threshold": 3,
    "overload_threshold": 10
  }
}
```

### 10.2 `resolve_overdue_tasks`

输入：

```json
{
  "actions": [
    { "task_id": "t1", "type": "reschedule", "due_date": "2026-03-12" },
    { "task_id": "t2", "type": "defer" }
  ],
  "dry_run": false,
  "confirm_token": "optional"
}
```

输出：成功/失败统计、逐条错误、变更摘要。

### 10.3 `rebalance_longterm_tasks`

输入：

```json
{
  "source": "todoist",
  "dry_run": true
}
```

输出：

- `short_term_before/after`
- `promoted_tasks`
- `retained_long_term`
- `deferred_or_backlog_tasks`

### 10.4 `detect_decomposition_candidates`

输入：

```json
{
  "source": "google",
  "limit": 50
}
```

输出：

- `candidates[]`（含 `complexity_score`、`reason_codes`、`has_subtasks`）
- `recommended_provider`
- `recommended_strategy`

### 10.5 `decompose_task_with_provider`

输入：

```json
{
  "task_id": "t123",
  "provider": "ms",
  "strategy": "project_split",
  "write_tasks": false
}
```

输出：

- `plan_id`
- `tasks_preview`
- `provider_capability_used`
- `warnings`

### 10.6 `analyze_achievement`

输入：

```json
{
  "window_days": 30,
  "compare_previous": true
}
```

输出：

- `metrics`（完成量、按时率、streak、趋势）
- `badges`
- `narrative`
- `next_actions`

---

## 11. 影响文件（实施时）

- MCP 工具注册与能力声明
- MCP handlers（建议按领域拆文件：overdue/longterm/decomposition/achievement）
- 配置结构与默认值
- 任务元数据辅助函数
- 单元测试与集成测试（规则边界、provider 分支、dry_run）

> 本文档为架构与实施计划，适合作为下一步编码任务的直接输入。

---

## 12. 实施状态（2026-03-04）

已完成首轮可用实现（MVP+部分执行闭环），包括：

1. 配置模型落地
   - `pkg/config/config.go`
   - 新增 `mcp.intelligence` 及子配置：`overdue`、`long_term`、`decomposition`、`achievement`
   - 已提供默认值并注册到 `setDefaults`

2. MCP 工具注册与能力声明
   - `internal/mcp/server.go`
   - 新增工具注册：
     - `analyze_overdue_health`
     - `resolve_overdue_tasks`
     - `rebalance_longterm_tasks`
     - `detect_decomposition_candidates`
     - `decompose_task_with_provider`
     - `analyze_achievement`
   - 已同步到 `GetTools` 能力集合

3. MCP 启动注入智能配置
   - `cmd/mcp.go`
   - `NewServer(...)` 新增 `WithIntelligenceConfig(&cfg.MCP.Intelligence)`

4. 工具处理器实现
   - `internal/mcp/handlers_intelligence.go`
   - 已实现 6 个工具的处理逻辑（含 `dry_run`、删除确认 token、候选提问、provider 能力分支）

5. 服务能力信息扩展
   - `internal/mcp/handlers.go`
   - `get_server_info` 增补 `analysis/intelligence` 能力分组

6. 测试覆盖
   - `internal/mcp/handlers_intelligence_test.go`
   - 覆盖：逾期分析、删除确认、长期调配、复杂任务候选、拆分写入、成就分析

7. 回归结果
   - 已通过 `go test ./internal/mcp`
   - 已通过 `go test ./...`
