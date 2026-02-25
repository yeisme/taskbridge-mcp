# TaskBridge 任务报告

生成时间: {{.GeneratedAt}}

## 概览

- 总任务数: {{.TotalCount}}
- 已完成: {{.CompletedCount}}
- 进行中: {{.InProgressCount}}
- 待办: {{.TodoCount}}

## 四象限视图

### 🔥 Q1 - 紧急且重要 (立即做)

{{range .Quadrant1}}
- [{{if eq .Status "completed"}}x{{else}} {{end}}] {{.Title}}
  {{if .DueDate}}- 截止: {{.DueDate.Format "2006-01-02"}}{{end}}
  {{if .Priority}}- 优先级: {{.Priority.String}}{{end}}
{{else}}
暂无任务
{{end}}

### 📋 Q2 - 重要不紧急 (计划做)

{{range .Quadrant2}}
- [{{if eq .Status "completed"}}x{{else}} {{end}}] {{.Title}}
  {{if .DueDate}}- 截止: {{.DueDate.Format "2006-01-02"}}{{end}}
  {{if .Priority}}- 优先级: {{.Priority.String}}{{end}}
{{else}}
暂无任务
{{end}}

### ⚡ Q3 - 紧急不重要 (授权做)

{{range .Quadrant3}}
- [{{if eq .Status "completed"}}x{{else}} {{end}}] {{.Title}}
  {{if .DueDate}}- 截止: {{.DueDate.Format "2006-01-02"}}{{end}}
{{else}}
暂无任务
{{end}}

### 🗑️ Q4 - 不紧急不重要 (删除/延后)

{{range .Quadrant4}}
- [{{if eq .Status "completed"}}x{{else}} {{end}}] {{.Title}}
{{else}}
暂无任务
{{end}}

## 按来源分布

{{range $source, $count := .BySource}}
- {{ $source }}: {{ $count }} 个任务
{{end}}

## 即将到期

{{range .Upcoming}}
- {{.Title}} - {{.DueDate.Format "2006-01-02"}}
{{else}}
暂无即将到期的任务
{{end}}

## 已过期

{{range .Overdue}}
- ⚠️ {{.Title}} - 已过期 {{.DaysUntilDue}} 天
{{else}}
暂无过期任务
{{end}}
