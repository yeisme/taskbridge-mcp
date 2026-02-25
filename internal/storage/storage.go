// Package storage 提供存储功能
package storage

import (
	"context"
	"time"

	"github.com/yeisme/taskbridge/internal/model"
)

// Storage 存储接口
type Storage interface {
	// 任务存储
	SaveTask(ctx context.Context, task *model.Task) error
	GetTask(ctx context.Context, id string) (*model.Task, error)
	ListTasks(ctx context.Context, opts ListOptions) ([]model.Task, error)
	DeleteTask(ctx context.Context, id string) error

	// 批量操作
	SaveTasks(ctx context.Context, tasks []*model.Task) error

	// 查询
	QueryTasks(ctx context.Context, query Query) ([]model.Task, error)

	// 任务列表
	SaveTaskList(ctx context.Context, list *model.TaskList) error
	GetTaskList(ctx context.Context, id string) (*model.TaskList, error)
	ListTaskLists(ctx context.Context) ([]model.TaskList, error)
	DeleteTaskList(ctx context.Context, id string) error

	// 导出
	ExportToJSON(ctx context.Context, opts ExportOptions) ([]byte, error)
	ExportToMarkdown(ctx context.Context, opts ExportOptions) ([]byte, error)

	// 同步状态
	GetLastSyncTime(ctx context.Context, source model.TaskSource) (*time.Time, error)
	SetLastSyncTime(ctx context.Context, source model.TaskSource, t time.Time) error
}

// ListOptions 列表选项
type ListOptions struct {
	PageSize  int
	PageToken string
	Source    model.TaskSource
	ListID    string
}

// Query 查询条件
type Query struct {
	Sources    []model.TaskSource
	Statuses   []model.TaskStatus
	Quadrants  []model.Quadrant
	Priorities []model.Priority
	Tags       []string
	ListIDs    []string
	ListNames  []string
	TaskIDs    []string
	DueBefore  *time.Time
	DueAfter   *time.Time
	FullText   string
	QueryText  string
	OrderBy    string // due_date, priority, created_at, updated_at
	OrderDesc  bool
	Limit      int
	Offset     int
}

// ExportOptions 导出选项
type ExportOptions struct {
	Format      string // json, markdown
	Template    string // 自定义模板路径
	IncludeMeta bool   // 是否包含元数据
	Pretty      bool   // 是否格式化输出
	Query       Query  // 筛选条件
}
