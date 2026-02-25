// Package model 定义了 TaskBridge 的核心数据模型
package model

import (
	"time"
)

// Task 统一任务模型 - 抽象所有 Todo 软件的任务
type Task struct {
	// ID 任务唯一标识
	ID string `json:"id"`
	// Title 任务标题
	Title string `json:"title"`
	// Description 任务描述
	Description string `json:"description,omitempty"`
	// Status 任务状态
	Status TaskStatus `json:"status"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
	// CompletedAt 完成时间
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// DueDate 截止日期
	DueDate *time.Time `json:"due_date,omitempty"`
	// StartDate 开始日期
	StartDate *time.Time `json:"start_date,omitempty"`
	// Reminder 提醒时间
	Reminder *time.Time `json:"reminder,omitempty"`

	// ListID 所属列表 ID
	ListID string `json:"list_id,omitempty"`
	// ListName 所属列表名称
	ListName string `json:"list_name,omitempty"`
	// Tags 标签列表
	Tags []string `json:"tags,omitempty"`
	// Categories 分类列表
	Categories []string `json:"categories,omitempty"`

	// Quadrant 四象限分类
	Quadrant Quadrant `json:"quadrant"`
	// Urgency 紧急程度
	Urgency UrgencyLevel `json:"urgency"`
	// Importance 重要程度
	Importance ImportanceLevel `json:"importance"`

	// Priority 优先级（1-4）
	Priority Priority `json:"priority"`
	// PriorityScore AI 计算的优先级分数
	PriorityScore int `json:"priority_score"`

	// Progress 进度（0-100）
	Progress int `json:"progress"`
	// EstimatedMinutes 预估时间（分钟）
	EstimatedMinutes int `json:"estimated_minutes,omitempty"`
	// ActualMinutes 实际时间（分钟）
	ActualMinutes int `json:"actual_minutes,omitempty"`

	// ParentID 父任务 ID
	ParentID *string `json:"parent_id,omitempty"`
	// SubtaskIDs 子任务 ID 列表
	SubtaskIDs []string `json:"subtask_ids,omitempty"`

	// Metadata 元数据，用于存储扩展信息
	Metadata *TaskMetadata `json:"metadata,omitempty"`

	// Source 任务来源平台
	Source TaskSource `json:"source"`
	// SourceRawID 原始平台的任务 ID
	SourceRawID string `json:"source_raw_id"`
	// ETag 用于并发控制的实体标签
	ETag string `json:"etag,omitempty"`
}

// TaskStatus 任务状态
type TaskStatus string

const (
	// StatusTodo 待办状态
	StatusTodo TaskStatus = "todo"
	// StatusInProgress 进行中状态
	StatusInProgress TaskStatus = "in_progress"
	// StatusCompleted 已完成状态
	StatusCompleted TaskStatus = "completed"
	// StatusCancelled 已取消状态
	StatusCancelled TaskStatus = "cancelled"
	// StatusDeferred 已延期状态
	StatusDeferred TaskStatus = "deferred"
)

// TaskSource 任务来源
type TaskSource string

const (
	// SourceMicrosoft Microsoft 来源
	SourceMicrosoft TaskSource = "microsoft"
	// SourceGoogle Google 来源
	SourceGoogle TaskSource = "google"
	// SourceFeishu 飞书来源
	SourceFeishu TaskSource = "feishu"
	// SourceTickTick 滴答清单来源
	SourceTickTick TaskSource = "ticktick"
	// SourceTodoist Todoist 来源
	SourceTodoist TaskSource = "todoist"
	// SourceOmniFocus OmniFocus 来源
	SourceOmniFocus TaskSource = "omnifocus"
	// SourceApple Apple 来源
	SourceApple TaskSource = "apple"
	// SourceLocal 本地来源
	SourceLocal TaskSource = "local"
)

// TaskList 任务列表
type TaskList struct {
	// ID 列表唯一标识
	ID string `json:"id"`
	// Name 列表名称
	Name string `json:"name"`
	// Description 列表描述
	Description string `json:"description,omitempty"`
	// Source 来源平台
	Source TaskSource `json:"source"`
	// SourceRawID 原始平台的列表 ID
	SourceRawID string `json:"source_raw_id"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
	// TaskCount 任务数量
	TaskCount int `json:"task_count,omitempty"`
}

// IsCompleted 检查任务是否已完成
func (t *Task) IsCompleted() bool {
	return t.Status == StatusCompleted
}

// IsOverdue 检查任务是否已过期
func (t *Task) IsOverdue() bool {
	if t.DueDate == nil || t.IsCompleted() {
		return false
	}
	return t.DueDate.Before(time.Now())
}

// DaysUntilDue 计算距离截止日期的天数
func (t *Task) DaysUntilDue() int {
	if t.DueDate == nil {
		return 0
	}
	delta := time.Until(*t.DueDate)
	return int(delta.Hours() / 24)
}

// CalculatePriorityScore 计算优先级分数
func (t *Task) CalculatePriorityScore() int {
	score := 0

	// 基于优先级
	score += int(t.Priority) * 20

	// 基于象限
	switch t.Quadrant {
	case QuadrantUrgentImportant:
		score += 40
	case QuadrantNotUrgentImportant:
		score += 30
	case QuadrantUrgentNotImportant:
		score += 20
	case QuadrantNotUrgentNotImportant:
		score += 10
	}

	// 基于截止日期
	if t.DueDate != nil {
		days := t.DaysUntilDue()
		switch {
		case days < 0:
			score += 30 // 已过期
		case days == 0:
			score += 25 // 今天截止
		case days <= 1:
			score += 20 // 明天截止
		case days <= 3:
			score += 15 // 3天内
		case days <= 7:
			score += 10 // 一周内
		}
	}

	// 基于进度
	if t.Progress > 0 {
		score += t.Progress / 10
	}

	t.PriorityScore = score
	return score
}
