// Package feishu provides Feishu (Lark) Task provider implementation
package feishu

import "time"

// ================ OAuth2 相关类型 ================

// TokenResponse OAuth2 token 响应
// https://open.feishu.cn/document/common-capabilities/sso/api/get-user-access-token
type TokenResponse struct {
	// AccessToken 访问令牌
	AccessToken string `json:"access_token"`
	// TokenType 令牌类型
	TokenType string `json:"token_type"`
	// ExpiresIn 访问令牌过期时间（秒）
	ExpiresIn int `json:"expires_in"`
	// RefreshToken 刷新令牌
	RefreshToken string `json:"refresh_token"`
	// Scope 授权范围
	Scope string `json:"scope"`
	// RefreshExpiresIn 刷新令牌过期时间（秒）
	RefreshExpiresIn int `json:"refresh_expires_in"`
}

// TokenErrorResponse token 错误响应
type TokenErrorResponse struct {
	// Code 错误码
	Code int `json:"code"`
	// Msg 错误消息
	Msg string `json:"msg"`
}

// UserInfoResponse 用户信息响应
type UserInfoResponse struct {
	// Code 响应码
	Code int `json:"code"`
	// Msg 响应消息
	Msg string `json:"msg"`
	// Data 用户数据
	Data struct {
		// Name 用户名称
		Name string `json:"name"`
		// EnName 英文名称
		EnName string `json:"en_name"`
		// Avatar 头像 URL
		Avatar string `json:"avatar"`
		// Email 邮箱
		Email string `json:"email"`
		// Mobile 手机号
		Mobile string `json:"mobile"`
		// UserID 用户 ID
		UserID string `json:"user_id"`
		// OpenID 用户 Open ID
		OpenID string `json:"open_id"`
	} `json:"data"`
}

// ================ 飞书任务 API 类型 ================

// TaskList 飞书任务列表
// https://open.feishu.cn/document/server-docs/task/v1/tasklist/create
type TaskList struct {
	// TaskListID 任务列表 ID
	TaskListID string `json:"tasklist_id"`
	// Name 任务列表名称
	Name string `json:"name"`
	// Color 颜色，格式为 #RRGGBB
	Color string `json:"color,omitempty"`
	// CreatedTime 创建时间（毫秒时间戳）
	CreatedTime int64 `json:"created_time,omitempty"`
	// CompletedTime 完成时间（毫秒时间戳）
	CompletedTime int64 `json:"completed_time,omitempty"`
	// TaskCount 任务数量
	TaskCount int `json:"task_count,omitempty"`
	// IsDeleted 是否已删除
	IsDeleted bool `json:"is_deleted,omitempty"`
	// Source 来源
	Source string `json:"source,omitempty"`
}

// Task 飞书任务
// https://open.feishu.cn/document/server-docs/task/v1/task/create
type Task struct {
	// TaskID 任务 ID
	TaskID string `json:"task_id"`
	// Title 任务标题
	Title string `json:"title"`
	// Description 任务描述（富文本）
	Description string `json:"description,omitempty"`
	// Source 任务来源
	Source string `json:"source,omitempty"`
	// Status 任务状态
	Status TaskStatus `json:"status,omitempty"`
	// DueTime 截止时间（毫秒时间戳）
	DueTime int64 `json:"due_time,omitempty"`
	// StartTime 开始时间（毫秒时间戳）
	StartTime int64 `json:"start_time,omitempty"`
	// CompletedTime 完成时间（毫秒时间戳）
	CompletedTime int64 `json:"completed_time,omitempty"`
	// CreatedTime 创建时间（毫秒时间戳）
	CreatedTime int64 `json:"created_time,omitempty"`
	// UpdatedTime 更新时间（毫秒时间戳）
	UpdatedTime int64 `json:"updated_time,omitempty"`
	// Reminders 提醒配置
	Reminders []Reminder `json:"reminders,omitempty"`
	// Repeat 重复规则
	Repeat *RepeatRule `json:"repeat,omitempty"`
	// CustomColumns 自定义列值
	CustomColumns []CustomColumnValue `json:"custom_columns,omitempty"`
	// Priority 优先级
	Priority TaskPriority `json:"priority,omitempty"`
	// TasklistIDs 所属任务列表
	TasklistIDs []string `json:"tasklist_ids,omitempty"`
	// SubtaskIDs 子任务 ID 列表
	SubtaskIDs []string `json:"subtask_ids,omitempty"`
	// CollaboratorIDs 协作者 ID 列表
	CollaboratorIDs []string `json:"collaborator_ids,omitempty"`
	// CreatorID 创建者 ID
	CreatorID string `json:"creator_id,omitempty"`
	// Tags 标签列表
	Tags []Tag `json:"tags,omitempty"`
	// Attachments 附件列表
	Attachments []Attachment `json:"attachments,omitempty"`
	// CommentCount 评论数
	CommentCount int `json:"comment_count,omitempty"`
	// HasComment 是否有评论
	HasComment bool `json:"has_comment,omitempty"`
}

// TaskStatus 任务状态
type TaskStatus int

const (
	StatusTodo       TaskStatus = 0 // 未完成
	StatusInProgress TaskStatus = 1 // 进行中
	StatusDone       TaskStatus = 2 // 已完成
)

// TaskPriority 任务优先级
type TaskPriority int

const (
	PriorityNone   TaskPriority = 0 // 无优先级
	PriorityLow    TaskPriority = 1 // 低优先级
	PriorityMedium TaskPriority = 2 // 中优先级
	PriorityHigh   TaskPriority = 3 // 高优先级
	PriorityUrgent TaskPriority = 4 // 紧急
)

// Reminder 提醒配置
type Reminder struct {
	// Type 提醒类型
	Type ReminderType `json:"type"`
	// RelativeTime 相对时间（分钟）
	RelativeTime int `json:"relative_time,omitempty"`
	// AbsoluteTime 绝对时间（毫秒时间戳）
	AbsoluteTime int64 `json:"absolute_time,omitempty"`
	// Enabled 是否启用
	Enabled bool `json:"enabled,omitempty"`
}

// ReminderType 提醒类型
type ReminderType int

const (
	ReminderTypeRelative ReminderType = 0 // 相对时间提醒
	ReminderTypeAbsolute ReminderType = 1 // 绝对时间提醒
)

// RepeatRule 重复规则
type RepeatRule struct {
	// Type 重复类型
	Type RepeatType `json:"type"`
	// Interval 重复间隔
	Interval int `json:"interval,omitempty"`
	// EndTime 结束时间（毫秒时间戳）
	EndTime int64 `json:"end_time,omitempty"`
	// Count 重复次数
	Count int `json:"count,omitempty"`
	// DaysOfWeek 星期几（1-7 表示周一到周日）
	DaysOfWeek []int `json:"days_of_week,omitempty"`
	// DayOfMonth 每月的第几天
	DayOfMonth int `json:"day_of_month,omitempty"`
}

// RepeatType 重复类型
type RepeatType int

const (
	RepeatTypeDaily   RepeatType = 0 // 每天
	RepeatTypeWeekly  RepeatType = 1 // 每周
	RepeatTypeMonthly RepeatType = 2 // 每月
	RepeatTypeYearly  RepeatType = 3 // 每年
)

// CustomColumnValue 自定义列值
type CustomColumnValue struct {
	// ColumnID 列 ID
	ColumnID string `json:"column_id"`
	// Value 列值
	Value interface{} `json:"value"`
}

// Tag 标签
type Tag struct {
	// TagID 标签 ID
	TagID string `json:"tag_id"`
	// Name 标签名称
	Name string `json:"name"`
	// Color 标签颜色
	Color string `json:"color,omitempty"`
}

// Attachment 附件
type Attachment struct {
	// AttachmentID 附件 ID
	AttachmentID string `json:"attachment_id"`
	// Name 文件名
	Name string `json:"name"`
	// Size 文件大小（字节）
	Size int64 `json:"size,omitempty"`
	// MimeType 文件类型
	MimeType string `json:"mime_type,omitempty"`
	// URL 文件 URL
	URL string `json:"url,omitempty"`
	// CreatedTime 创建时间
	CreatedTime int64 `json:"created_time,omitempty"`
}

// Subtask 子任务
type Subtask struct {
	// SubtaskID 子任务 ID
	SubtaskID string `json:"subtask_id"`
	// Title 子任务标题
	Title string `json:"title"`
	// IsCompleted 是否完成
	IsCompleted bool `json:"is_completed"`
	// CompletedTime 完成时间
	CompletedTime int64 `json:"completed_time,omitempty"`
	// CreatedTime 创建时间
	CreatedTime int64 `json:"created_time,omitempty"`
	// CreatorID 创建者 ID
	CreatorID string `json:"creator_id,omitempty"`
}

// ================ API 响应包装 ================

// APIResponse 通用 API 响应
type APIResponse struct {
	// Code 响应码
	Code int `json:"code"`
	// Msg 响应消息
	Msg string `json:"msg"`
}

// TaskListResponse 任务列表响应
type TaskListResponse struct {
	// Code 响应码
	Code int `json:"code"`
	// Msg 响应消息
	Msg string `json:"msg"`
	// Data 任务列表数据
	Data struct {
		// Tasklists 任务列表
		Tasklists []TaskList `json:"tasklists"`
		// HasMore 是否有更多数据
		HasMore bool `json:"has_more"`
		// PageToken 分页令牌
		PageToken string `json:"page_token"`
	} `json:"data"`
}

// TaskResponse 单个任务响应
type TaskResponse struct {
	// Code 响应码
	Code int `json:"code"`
	// Msg 响应消息
	Msg string `json:"msg"`
	// Data 任务数据
	Data struct {
		// Task 任务详情
		Task Task `json:"task"`
	} `json:"data"`
}

// TaskListDetailResponse 任务列表详情响应
type TaskListDetailResponse struct {
	// Code 响应码
	Code int `json:"code"`
	// Msg 响应消息
	Msg string `json:"msg"`
	// Data 任务列表数据
	Data struct {
		// Tasklist 任务列表详情
		Tasklist TaskList `json:"tasklist"`
	} `json:"data"`
}

// TasksResponse 任务列表中的任务响应
type TasksResponse struct {
	// Code 响应码
	Code int `json:"code"`
	// Msg 响应消息
	Msg string `json:"msg"`
	// Data 任务数据
	Data struct {
		// Tasks 任务列表
		Tasks []Task `json:"tasks"`
		// HasMore 是否有更多数据
		HasMore bool `json:"has_more"`
		// PageToken 分页令牌
		PageToken string `json:"page_token"`
		// Total 总数
		Total int `json:"total"`
	} `json:"data"`
}

// CreateTaskResponse 创建任务响应
type CreateTaskResponse struct {
	// Code 响应码
	Code int `json:"code"`
	// Msg 响应消息
	Msg string `json:"msg"`
	// Data 创建的任务数据
	Data struct {
		// Task 创建的任务
		Task Task `json:"task"`
	} `json:"data"`
}

// CreateTaskListResponse 创建任务列表响应
type CreateTaskListResponse struct {
	// Code 响应码
	Code int `json:"code"`
	// Msg 响应消息
	Msg string `json:"msg"`
	// Data 创建的任务列表数据
	Data struct {
		// Tasklist 创建的任务列表
		Tasklist TaskList `json:"tasklist"`
	} `json:"data"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	// Code 错误码
	Code int `json:"code"`
	// Msg 错误消息
	Msg string `json:"msg"`
}

// ================ 请求体 ================

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	// Task 任务信息
	Task
	// TasklistIDs 所属任务列表 ID
	TasklistIDs []string `json:"tasklist_ids"`
}

// UpdateTaskRequest 更新任务请求
type UpdateTaskRequest struct {
	// Task 任务信息
	Task
	// UpdateFields 需要更新的字段列表
	UpdateFields []string `json:"update_fields,omitempty"`
}

// CreateTaskListRequest 创建任务列表请求
type CreateTaskListRequest struct {
	// Name 任务列表名称
	Name string `json:"name"`
	// Color 任务列表颜色
	Color string `json:"color,omitempty"`
}

// ================ 增量同步 ================

// DeltaToken 增量同步令牌
type DeltaToken struct {
	// Token 增量同步令牌值
	Token string `json:"token"`
	// ExpiresAt 过期时间（毫秒时间戳）
	ExpiresAt int64 `json:"expires_at"`
}

// SyncChangesResponse 增量同步响应
type SyncChangesResponse struct {
	// Code 响应码
	Code int `json:"code"`
	// Msg 响应消息
	Msg string `json:"msg"`
	// Data 变更数据
	Data struct {
		// Changes 任务变更列表
		Changes []TaskChange `json:"changes"`
		// HasMore 是否有更多数据
		HasMore bool `json:"has_more"`
		// PageToken 分页令牌
		PageToken string `json:"page_token"`
		// DeltaToken 增量同步令牌
		DeltaToken string `json:"delta_token"`
	} `json:"data"`
}

// TaskChange 任务变更
type TaskChange struct {
	// Task 变更的任务
	Task Task `json:"task"`
	// ChangeType 变更类型
	ChangeType ChangeType `json:"change_type"`
	// ChangedAt 变更时间（毫秒时间戳）
	ChangedAt int64 `json:"changed_at"`
}

// ChangeType 变更类型
type ChangeType string

const (
	ChangeTypeCreated ChangeType = "created"
	ChangeTypeUpdated ChangeType = "updated"
	ChangeTypeDeleted ChangeType = "deleted"
)

// ================ 辅助函数 ================

// MillisecondsToTime 将毫秒时间戳转换为 time.Time
func MillisecondsToTime(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.Unix(0, ms*int64(time.Millisecond))
}

// TimeToMilliseconds 将 time.Time 转换为毫秒时间戳
func TimeToMilliseconds(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano() / int64(time.Millisecond)
}

// TimePtrToMilliseconds 将 *time.Time 转换为毫秒时间戳
func TimePtrToMilliseconds(t *time.Time) int64 {
	if t == nil || t.IsZero() {
		return 0
	}
	return t.UnixNano() / int64(time.Millisecond)
}

// MillisecondsToTimePtr 将毫秒时间戳转换为 *time.Time
func MillisecondsToTimePtr(ms int64) *time.Time {
	if ms == 0 {
		return nil
	}
	t := time.Unix(0, ms*int64(time.Millisecond))
	return &t
}

// PriorityToFeishu 将 TaskBridge Priority 转换为飞书 TaskPriority
func PriorityToFeishu(priority int) TaskPriority {
	switch priority {
	case 1:
		return PriorityUrgent
	case 2:
		return PriorityHigh
	case 3:
		return PriorityMedium
	case 4:
		return PriorityLow
	default:
		return PriorityNone
	}
}

// FeishuToPriority 将飞书 TaskPriority 转换为 TaskBridge Priority
func FeishuToPriority(priority TaskPriority) int {
	switch priority {
	case PriorityUrgent:
		return 1
	case PriorityHigh:
		return 2
	case PriorityMedium:
		return 3
	case PriorityLow:
		return 4
	default:
		return 0
	}
}

// StatusToFeishu 将 TaskBridge Status 转换为飞书 TaskStatus
func StatusToFeishu(status string) TaskStatus {
	switch status {
	case "todo", "pending":
		return StatusTodo
	case "in_progress":
		return StatusInProgress
	case "completed":
		return StatusDone
	default:
		return StatusTodo
	}
}

// FeishuToStatus 将飞书 TaskStatus 转换为 TaskBridge Status
func FeishuToStatus(status TaskStatus) string {
	switch status {
	case StatusTodo:
		return "todo"
	case StatusInProgress:
		return "in_progress"
	case StatusDone:
		return "completed"
	default:
		return "todo"
	}
}
