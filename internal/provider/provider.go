// Package provider 定义了 Todo 软件适配器的接口
package provider

import (
	"context"
	"time"

	"github.com/yeisme/taskbridge/internal/model"
)

// Provider Todo 软件适配器接口
type Provider interface {
	// 基础信息
	Name() string
	DisplayName() string

	// 认证
	Authenticate(ctx context.Context, config map[string]interface{}) error
	IsAuthenticated() bool
	RefreshToken(ctx context.Context) error

	// 任务列表操作
	ListTaskLists(ctx context.Context) ([]model.TaskList, error)
	CreateTaskList(ctx context.Context, name string) (*model.TaskList, error)
	DeleteTaskList(ctx context.Context, listID string) error

	// 任务操作 - 读取
	ListTasks(ctx context.Context, listID string, opts ListOptions) ([]model.Task, error)
	GetTask(ctx context.Context, listID, taskID string) (*model.Task, error)
	SearchTasks(ctx context.Context, query string) ([]model.Task, error)

	// 任务操作 - 写入
	CreateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error)
	UpdateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error)
	DeleteTask(ctx context.Context, listID, taskID string) error

	// 批量操作
	BatchCreate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error)
	BatchUpdate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error)

	// 同步支持
	GetChanges(ctx context.Context, since time.Time) (*SyncChanges, error)

	// 能力查询
	Capabilities() Capabilities

	// Token 管理
	GetTokenInfo() *TokenInfo
}

// TokenInfo Token 信息
type TokenInfo struct {
	// Provider Provider 名称
	Provider string `json:"provider"`
	// HasToken 是否有 Token
	HasToken bool `json:"has_token"`
	// IsValid Token 是否有效
	IsValid bool `json:"is_valid"`
	// ExpiresAt Token 过期时间
	ExpiresAt time.Time `json:"expires_at"`
	// Refreshable 是否可刷新
	Refreshable bool `json:"refreshable"`
	// TimeUntilExpiry 距离过期的时间描述
	TimeUntilExpiry string `json:"time_until_expiry"`
	// NeedsRefresh 是否需要刷新
	NeedsRefresh bool `json:"needs_refresh"`
}

// ListOptions 列表查询选项
type ListOptions struct {
	// PageSize 每页数量
	PageSize int
	// PageToken 分页令牌
	PageToken string
	// Completed 是否已完成（nil 表示全部）
	Completed *bool
	// DueBefore 截止日期在此之前
	DueBefore *time.Time
	// DueAfter 截止日期在此之后
	DueAfter *time.Time
	// UpdatedAfter 更新时间在此之后
	UpdatedAfter *time.Time
}

// Capabilities Provider 能力描述
type Capabilities struct {
	// SupportsSubtasks 是否支持子任务
	SupportsSubtasks bool `json:"supports_subtasks"`
	// SupportsTags 是否支持标签
	SupportsTags bool `json:"supports_tags"`
	// SupportsCategories 是否支持分类
	SupportsCategories bool `json:"supports_categories"`
	// SupportsReminder 是否支持提醒
	SupportsReminder bool `json:"supports_reminder"`
	// SupportsDueDate 是否支持截止日期
	SupportsDueDate bool `json:"supports_due_date"`
	// SupportsStartDate 是否支持开始日期
	SupportsStartDate bool `json:"supports_start_date"`
	// SupportsProgress 是否支持进度
	SupportsProgress bool `json:"supports_progress"`
	// SupportsPriority 是否支持优先级
	SupportsPriority bool `json:"supports_priority"`
	// SupportsSearch 是否支持搜索
	SupportsSearch bool `json:"supports_search"`
	// SupportsBatch 是否支持批量操作
	SupportsBatch bool `json:"supports_batch"`
	// SupportsDeltaSync 是否支持增量同步
	SupportsDeltaSync bool `json:"supports_delta_sync"`
	// MaxTaskLength 任务标题最大长度
	MaxTaskLength int `json:"max_task_length"`
	// MaxDescriptionLength 描述最大长度
	MaxDescriptionLength int `json:"max_description_length"`
}

// SyncChanges 同步变更
type SyncChanges struct {
	// Tasks 变更的任务列表
	Tasks []model.Task `json:"tasks"`
	// DeletedIDs 已删除的任务 ID 列表
	DeletedIDs []string `json:"deleted_ids"`
	// NextToken 下一页令牌
	NextToken string `json:"next_token"`
	// HasMore 是否还有更多
	HasMore bool `json:"has_more"`
}

// Conflict 同步冲突
type Conflict struct {
	// LocalTask 本地任务
	LocalTask *model.Task `json:"local_task"`
	// RemoteTask 远程任务
	RemoteTask *model.Task `json:"remote_task"`
	Field      string      `json:"field"`
}

// SyncError 同步错误
type SyncError struct {
	TaskID    string `json:"task_id"`
	Operation string `json:"operation"`
	Error     string `json:"error"`
}
