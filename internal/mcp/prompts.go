package mcp

// EmbeddedPrompts 内置提示词映射
var EmbeddedPrompts = map[string]string{
	"quadrant_analysis":   QuadrantAnalysisPrompt,
	"task_creation":       TaskCreationPrompt,
	"project_planning":    ProjectPlanningPrompt,
	"ai_split_guide":      AISplitGuidePrompt,
	"json_query_commands": JSONQueryCommandsPrompt,
}

// QuadrantAnalysisPrompt 四象限分析提示词
const QuadrantAnalysisPrompt = `# 四象限分析提示词

## 任务转四象限规则

当分析任务时，请根据以下规则将任务分配到艾森豪威尔矩阵的四个象限：

### 判断标准

**紧急程度 Urgency 判断：**
- 已过期或今天截止 → Critical 紧急 (4)
- 3天内截止 → High 高 (3)
- 1周内截止 → Medium 中 (2)
- 超过1周或无截止日期 → Low 低 (1)

**重要程度 Importance 判断：**
- 与核心目标直接相关 → Critical 关键 (4)
- 影响他人或项目进度 → High 高 (3)
- 有价值但可延后 → Medium 中 (2)
- 可选或低价值 → Low 低 (1)

### 象限分配

| 象限 | 紧急程度 | 重要程度 | 策略 |
|------|----------|----------|------|
| Q1 - 立即做 | >= Medium | >= Medium | 优先处理 |
| Q2 - 计划做 | < Medium | >= Medium | 安排时间 |
| Q3 - 授权做 | >= Medium | < Medium | 委托他人 |
| Q4 - 删除/延后 | < Medium | < Medium | 考虑删除 |

### MCP 工具调用示例

分析任务时，调用 analyze_quadrant 工具：

` + "```json" + `
{
  "tool": "analyze_quadrant",
  "arguments": {}
}
` + "```" + `

创建任务时，指定象限：

` + "```json" + `
{
  "tool": "create_task",
  "arguments": {
    "title": "完成项目报告",
    "due_date": "2026-02-25",
    "priority": 3,
    "quadrant": 1
  }
}
` + "```" + `

### 象限详细说明

**Q1 - 紧急且重要（立即做）**
- 这类任务需要立即处理
- 通常涉及危机、截止日期或关键问题
- 示例：紧急 bug 修复、即将到期的报告、客户投诉

**Q2 - 重要不紧急（计划做）**
- 这类任务对长期目标很重要
- 需要主动安排时间处理
- 示例：学习新技能、战略规划、关系建立、健康锻炼

**Q3 - 紧急不重要（授权做）**
- 这类任务可以委托给他人
- 通常是别人的优先事项
- 示例：某些会议、部分邮件回复、一些电话

**Q4 - 不紧急不重要（删除/延后）**
- 这类任务应该考虑删除或大幅延后
- 通常是时间浪费者
- 示例：无意义的社交媒体浏览、过多的娱乐活动
`

// TaskCreationPrompt 任务创建提示词
const TaskCreationPrompt = `# 任务创建提示词

## 创建有效任务的原则

### 1. 使用动词开头
- ✅ "完成项目报告"
- ✅ "回复客户邮件"
- ❌ "项目报告"
- ❌ "客户邮件"

### 2. 具体且可衡量
- ✅ "编写 500 字的产品介绍"
- ❌ "写点东西"

### 3. 设置合理的截止日期
- 考虑任务的复杂度
- 留出缓冲时间
- 避免过度承诺

### 4. 正确设置优先级和象限

**优先级指南：**
- 4 (紧急)：必须立即处理
- 3 (高)：重要但不紧急
- 2 (中)：普通优先级
- 1 (低)：可延后处理

**象限指南：**
- Q1：紧急且重要 - 立即做
- Q2：重要不紧急 - 计划做
- Q3：紧急不重要 - 授权做
- Q4：不紧急不重要 - 删除/延后

### MCP 工具调用示例

` + "```json" + `
{
  "tool": "create_task",
  "arguments": {
    "title": "完成季度销售报告",
    "due_date": "2026-03-31",
    "priority": 3,
    "quadrant": 2
  }
}
` + "```" + `

### 任务拆分建议

如果一个任务看起来很复杂，考虑拆分为多个子任务：

1. 识别主要交付物
2. 按时间顺序排列步骤
3. 每个子任务应该能在 1-2 小时内完成
4. 为每个子任务设置独立的截止日期
`

// ProjectPlanningPrompt 项目规划提示词
const ProjectPlanningPrompt = `# 项目规划提示词

## 项目规划框架

### 1. 定义项目目标（SMART 原则）
- **S**pecific（具体的）：明确要达成什么
- **M**easurable（可衡量的）：有明确的成功标准
- **A**chievable（可实现的）：在现有资源下可行
- **R**elevant（相关的）：与更大的目标相关联
- **T**ime-bound（有时限的）：有明确的截止日期

### 2. 分解项目为任务

**分解步骤：**
1. 识别主要阶段/里程碑
2. 每个阶段分解为具体任务
3. 确定任务之间的依赖关系
4. 估算每个任务所需时间

### 3. 设置优先级

**优先级矩阵：**
| 优先级 | 描述 | 处理方式 |
|--------|------|----------|
| P0 | 阻塞性任务 | 立即处理 |
| P1 | 关键路径任务 | 优先处理 |
| P2 | 重要任务 | 正常处理 |
| P3 | 可选任务 | 有时间再处理 |

### 4. MCP 工作流

**创建项目：**
` + "```json" + `
{
  "tool": "create_project",
  "arguments": {
    "name": "学习 K8s",
    "description": "系统学习 Kubernetes 容器编排"
  }
}
` + "```" + `

**AI 辅助拆分：**
` + "```json" + `
{
  "tool": "split_project",
  "arguments": {
    "project_id": "proj_xxx",
    "ai_hint": "按照从基础到高级的顺序拆分，每周一个主题"
  }
}
` + "```" + `

**确认并同步：**
` + "```json" + `
{
  "tool": "confirm_project",
  "arguments": {
    "project_id": "proj_xxx"
  }
}
` + "```" + `
`

// AISplitGuidePrompt AI 拆分指导提示词
const AISplitGuidePrompt = `# AI 项目拆分指导

## 项目信息
- 项目名称: %s
- 复杂度: %s

## 拆分原则

### 1. 按时间维度拆分
- 将项目按周/月划分阶段
- 每个阶段有明确的学习目标
- 循序渐进，由浅入深

### 2. 按主题拆分
- 识别项目的主要知识领域
- 每个主题独立成任务组
- 相关任务放在一起

### 3. 按交付物拆分
- 每个任务应该有明确的产出
- 产出可以是文档、代码、演示等
- 便于跟踪进度

### 4. 复杂度考虑

**简单项目 (1-2周)：**
- 3-5 个主要任务
- 每个任务 2-4 小时

**中等项目 (1-2月)：**
- 10-20 个任务
- 分为 4-6 个阶段
- 每周 2-3 个任务

**复杂项目 (3个月+)：**
- 分为多个子项目
- 每个子项目独立管理
- 设置里程碑检查点

## 输出格式

请按以下格式输出拆分建议：

` + "```" + `
## 阶段 1：[阶段名称]
- [ ] 任务1：[任务描述] (预计: X小时)
- [ ] 任务2：[任务描述] (预计: X小时)

## 阶段 2：[阶段名称]
- [ ] 任务1：[任务描述] (预计: X小时)
- [ ] 任务2：[任务描述] (预计: X小时)

...
` + "```" + `

## 注意事项

1. 任务应该是具体的、可执行的
2. 每个任务有明确的时间估算
3. 考虑任务之间的依赖关系
4. 留出复习和缓冲时间
5. 设置检查点来验证进度
`

// JSONQueryCommandsPrompt JSON 检索命令提示词
const JSONQueryCommandsPrompt = `# JSON 检索命令生成提示词

你的任务是：根据用户检索目标，生成可直接运行的命令，优先减少上下文数据量。

## 输出要求

1. 同时输出两套命令：
- PowerShell 版本（Windows）
- Bash 版本（Linux/macOS）

2. 优先使用：
- jq（结构化过滤）
- rg（文本快速检索）

3. 若 jq/rg 不可用，提供 shell fallback：
- PowerShell: ConvertFrom-Json + Where-Object + Select-Object
- Bash: grep/awk/sed

4. 默认数据文件：
- data/tasks.json
- data/lists.json

5. 输出格式：
- 先给简短“思路”
- 再给可复制命令块
- 最后给“如何按需改参数”的 1-2 句说明

## 常见示例模板

### 1) 按 provider + 清单名 + 状态筛选

PowerShell:
` + "```powershell" + `
Get-Content data/tasks.json -Raw |
  jq '.[] | select(.source=="microsoft" and (.list_name|test("学习与成长")) and .status=="completed")'
` + "```" + `

Bash:
` + "```bash" + `
jq '.[] | select(.source=="microsoft" and (.list_name|test("学习与成长")) and .status=="completed")' data/tasks.json
` + "```" + `

### 2) 统计某清单任务数

PowerShell:
` + "```powershell" + `
Get-Content data/tasks.json -Raw |
  jq '[.[] | select(.source=="microsoft" and (.list_name|test("学习与成长")))] | length'
` + "```" + `

Bash:
` + "```bash" + `
jq '[.[] | select(.source=="microsoft" and (.list_name|test("学习与成长")))] | length' data/tasks.json
` + "```" + `

### 3) fallback（无 jq）

PowerShell:
` + "```powershell" + `
Get-Content data/tasks.json -Raw | ConvertFrom-Json |
  Where-Object { $_.source -eq "microsoft" -and $_.list_name -match "学习与成长" -and $_.status -eq "completed" } |
  Select-Object id,title,status,source,list_name
` + "```" + `

Bash:
` + "```bash" + `
grep -n '"source":"microsoft"' data/tasks.json
` + "```" + `
`
