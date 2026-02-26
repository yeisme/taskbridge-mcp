// Package microsoft provides Microsoft To Do provider implementation
package microsoft

import "time"

// ================ OAuth2 相关类型 ================

// TokenResponse OAuth2 token 响应
type TokenResponse struct {
	// AccessToken 访问令牌
	AccessToken string `json:"access_token"`
	// TokenType 令牌类型
	TokenType string `json:"token_type"`
	// ExpiresIn 过期时间（秒）
	ExpiresIn int `json:"expires_in"`
	// RefreshToken 刷新令牌
	RefreshToken string `json:"refresh_token"`
	// Scope 授权范围
	Scope string `json:"scope"`
}

// ================ Microsoft Graph API 类型 ================

// TodoTaskList Microsoft To Do 任务列表
// https://learn.microsoft.com/en-us/graph/api/resources/todotasklist
type TodoTaskList struct {
	// ID 任务列表唯一标识
	ID string `json:"id"`
	// DisplayName 显示名称
	DisplayName string `json:"displayName"`
	// IsOwner 是否为所有者
	IsOwner bool `json:"isOwner"`
	// IsShared 是否共享
	IsShared bool `json:"isShared"`
	// WellknownName 预定义列表名称
	WellknownName string `json:"wellknownListName"`
	// CreatedDateTime 创建时间
	CreatedDateTime time.Time `json:"createdDateTime"`
	// LastModified 最后修改时间
	LastModified time.Time `json:"lastModifiedDateTime"`
	// Tasks 任务列表
	Tasks []TodoTask `json:"tasks,omitempty"`
}

// TodoTask Microsoft To Do 任务
// https://learn.microsoft.com/en-us/graph/api/resources/todotask
type TodoTask struct {
	// ID 任务唯一标识
	ID string `json:"id"`
	// Title 任务标题
	Title string `json:"title"`
	// Status 任务状态
	Status TaskStatus `json:"status"`
	// Importance 重要性
	Importance Importance `json:"importance"`
	// Body 任务内容
	Body *ItemBody `json:"body,omitempty"`
	// DueDateTime 截止时间
	DueDateTime *DateTimeTimeZone `json:"dueDateTime,omitempty"`
	// StartDateTime 开始时间
	StartDateTime *DateTimeTimeZone `json:"startDateTime,omitempty"`
	// CompletedDateTime 完成时间
	CompletedDateTime *DateTimeTimeZone `json:"completedDateTime,omitempty"`
	// ReminderDateTime 提醒时间
	ReminderDateTime *DateTimeTimeZone `json:"reminderDateTime,omitempty"`
	// CreatedDateTime 创建时间
	CreatedDateTime time.Time `json:"createdDateTime,omitempty"`
	// LastModifiedDateTime 最后修改时间
	LastModifiedDateTime time.Time `json:"lastModifiedDateTime,omitempty"`
	// IsReminderOn 是否开启提醒
	IsReminderOn bool `json:"isReminderOn"`
	// HasAttachments 是否有附件
	HasAttachments bool `json:"hasAttachments"`
	// Imported 是否导入
	Imported bool `json:"imported"`
	// Recurrence 重复规则
	Recurrence *PatternedRecurrence `json:"recurrence,omitempty"`
	// LinkedResources 关联资源
	LinkedResources []LinkedResource `json:"linkedResources,omitempty"`
	// ChecklistItems 检查项列表
	ChecklistItems []ChecklistItem `json:"checklistItems,omitempty"`
	// Categories 分类列表
	Categories []string `json:"categories,omitempty"`
	// Attachments 附件列表
	Attachments []TaskAttachment `json:"attachments,omitempty"`
}

// TaskStatus 任务状态
type TaskStatus string

const (
	StatusNotStarted      TaskStatus = "notStarted"
	StatusInProgress      TaskStatus = "inProgress"
	StatusCompleted       TaskStatus = "completed"
	StatusWaitingOnOthers TaskStatus = "waitingOnOthers"
	StatusDeferred        TaskStatus = "deferred"
)

// Importance 重要性
type Importance string

const (
	ImportanceLow    Importance = "low"
	ImportanceNormal Importance = "normal"
	ImportanceHigh   Importance = "high"
)

// ItemBody 内容体
type ItemBody struct {
	Content     string      `json:"content"`
	ContentType ContentType `json:"contentType"`
}

// ContentType 内容类型
type ContentType string

const (
	ContentTypeText ContentType = "text"
	ContentTypeHTML ContentType = "html"
)

// DateTimeTimeZone 带时区的日期时间
// Microsoft Graph 特有格式
type DateTimeTimeZone struct {
	// DateTime 日期时间字符串
	DateTime string `json:"dateTime"`
	// TimeZone 时区
	TimeZone string `json:"timeZone"`
}

// PatternedRecurrence 重复模式
type PatternedRecurrence struct {
	// Pattern 重复规则
	Pattern RecurrencePattern `json:"pattern"`
	// Range 重复范围
	Range RecurrenceRange `json:"range"`
}

// RecurrencePattern 重复规则
type RecurrencePattern struct {
	// Type 重复类型
	Type RecurrencePatternType `json:"type"`
	// Interval 间隔
	Interval int `json:"interval"`
	// Month 月份
	Month int `json:"month,omitempty"`
	// DayOfMonth 每月的第几天
	DayOfMonth int `json:"dayOfMonth,omitempty"`
	// DaysOfWeek 星期几
	DaysOfWeek []DayOfWeek `json:"daysOfWeek,omitempty"`
	// FirstDayOfWeek 每周的第一天
	FirstDayOfWeek DayOfWeek `json:"firstDayOfWeek,omitempty"`
	// Index 周索引
	Index WeekIndex `json:"index,omitempty"`
}

// RecurrencePatternType 重复类型
type RecurrencePatternType string

const (
	RecurrenceDaily           RecurrencePatternType = "daily"
	RecurrenceWeekly          RecurrencePatternType = "weekly"
	RecurrenceAbsoluteMonthly RecurrencePatternType = "absoluteMonthly"
	RecurrenceRelativeMonthly RecurrencePatternType = "relativeMonthly"
	RecurrenceAbsoluteYearly  RecurrencePatternType = "absoluteYearly"
	RecurrenceRelativeYearly  RecurrencePatternType = "relativeYearly"
)

// RecurrenceRange 重复范围
type RecurrenceRange struct {
	// Type 范围类型
	Type RecurrenceRangeType `json:"type"`
	// StartDate 开始日期
	StartDate string `json:"startDate"`
	// EndDate 结束日期
	EndDate string `json:"endDate,omitempty"`
	// NumberOfOccurrences 重复次数
	NumberOfOccurrences int `json:"numberOfOccurrences,omitempty"`
	// RecurrenceTimeZone 时区
	RecurrenceTimeZone string `json:"recurrenceTimeZone,omitempty"`
}

// RecurrenceRangeType 重复范围类型
type RecurrenceRangeType string

const (
	RangeEndDate  RecurrenceRangeType = "endDate"
	RangeNoEnd    RecurrenceRangeType = "noEnd"
	RangeNumbered RecurrenceRangeType = "numbered"
)

// DayOfWeek 星期
type DayOfWeek string

const (
	Sunday    DayOfWeek = "sunday"
	Monday    DayOfWeek = "monday"
	Tuesday   DayOfWeek = "tuesday"
	Wednesday DayOfWeek = "wednesday"
	Thursday  DayOfWeek = "thursday"
	Friday    DayOfWeek = "friday"
	Saturday  DayOfWeek = "saturday"
)

// WeekIndex 周索引
type WeekIndex string

const (
	First  WeekIndex = "first"
	Second WeekIndex = "second"
	Third  WeekIndex = "third"
	Fourth WeekIndex = "fourth"
	Last   WeekIndex = "last"
)

// LinkedResource 关联资源
type LinkedResource struct {
	// ID 资源 ID
	ID string `json:"id"`
	// WebURL网页 URL
	WebURL string `json:"webUrl"`
	// ApplicationName 应用名称
	ApplicationName string `json:"applicationName"`
	// DisplayName 显示名称
	DisplayName string `json:"displayName"`
	// ExternalContext 外部上下文
	ExternalContext string `json:"externalContext,omitempty"`
	// ExternalID 外部 ID
	ExternalID string `json:"externalId,omitempty"`
}

// ChecklistItem 检查项（子任务）
type ChecklistItem struct {
	// ID 检查项 ID
	ID string `json:"id"`
	// DisplayName 显示名称
	DisplayName string `json:"displayName"`
	// IsChecked 是否已勾选
	IsChecked bool `json:"isChecked"`
	// CreatedDateTime 创建时间
	CreatedDateTime time.Time `json:"createdDateTime,omitempty"`
	// LastModifiedDateTime 最后修改时间
	LastModifiedDateTime time.Time `json:"lastModifiedDateTime,omitempty"`
}

// TaskAttachment 任务附件
type TaskAttachment struct {
	// ID 附件 ID
	ID string `json:"id"`
	// ContentType 内容类型
	ContentType string `json:"contentType"`
	// LastModifiedDateTime 最后修改时间
	LastModifiedDateTime time.Time `json:"lastModifiedDateTime"`
	// Name 文件名
	Name string `json:"name"`
	// Size 文件大小
	Size int64 `json:"size"`
}

// ================ API 响应包装 ================

// TodoTaskListResponse 任务列表响应
type TodoTaskListResponse struct {
	// Value 任务列表
	Value []TodoTaskList `json:"value"`
	// OdataNextLink 下一页链接
	OdataNextLink string `json:"@odata.nextLink,omitempty"`
}

// TodoTaskResponse 任务响应
type TodoTaskResponse struct {
	// Value 任务列表
	Value []TodoTask `json:"value"`
	// OdataNextLink 下一页链接
	OdataNextLink string `json:"@odata.nextLink,omitempty"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	// Error 错误详情
	Error struct {
		// Code 错误码
		Code string `json:"code"`
		// Message 错误消息
		Message string `json:"message"`
		// Details 错误详情列表
		Details []struct {
			// Code 错误码
			Code string `json:"code"`
			// Message 错误消息
			Message string `json:"message"`
			// Target 目标
			Target string `json:"target,omitempty"`
		} `json:"details,omitempty"`
		// InnerError 内部错误
		InnerError *InnerError `json:"innerError,omitempty"`
	} `json:"error"`
}

// InnerError 内部错误
type InnerError struct {
	// Code 错误码
	Code string `json:"code,omitempty"`
	// RequestID 请求 ID
	RequestID string `json:"request-id,omitempty"`
	// Date 日期
	Date string `json:"date,omitempty"`
}

// DeltaResponse 增量同步响应
type DeltaResponse struct {
	// Value 任务列表
	Value []TodoTask `json:"value"`
	// DeltaLink 增量同步链接
	DeltaLink string `json:"@odata.deltaLink,omitempty"`
	// NextLink 下一页链接
	NextLink string `json:"@odata.nextLink,omitempty"`
	// DeltaToken 增量同步令牌
	DeltaToken string `json:"@odata.delta.token,omitempty"`
}

// ================ 辅助函数 ================

// ToDateTimeTimeZone 将 time.Time 转换为 Microsoft DateTimeTimeZone 格式
func ToDateTimeTimeZone(t *time.Time, timezone string) *DateTimeTimeZone {
	if t == nil {
		return nil
	}
	tz := timezone
	if tz == "" {
		tz = "UTC"
	}
	return &DateTimeTimeZone{
		DateTime: t.Format("2006-01-02T15:04:05"),
		TimeZone: tz,
	}
}

// FromDateTimeTimeZone 将 Microsoft DateTimeTimeZone 转换为 time.Time
func FromDateTimeTimeZone(dtz *DateTimeTimeZone) *time.Time {
	if dtz == nil {
		return nil
	}

	// 解析时间字符串
	// 格式可能是 "2024-01-15T00:00:00" 或 "2024-01-15"
	var t time.Time
	var err error

	loc := time.Local
	if dtz.TimeZone != "" {
		loc, err = time.LoadLocation(dtz.TimeZone)
		if err != nil {
			loc = time.Local
		}
	}

	// 尝试解析带时间的格式
	t, err = time.ParseInLocation("2006-01-02T15:04:05", dtz.DateTime, loc)
	if err != nil {
		// 尝试只解析日期
		t, err = time.ParseInLocation("2006-01-02", dtz.DateTime, loc)
		if err != nil {
			return nil
		}
	}

	return &t
}

// ImportanceToPriority 将 Microsoft Importance 转换为 TaskBridge Priority
func ImportanceToPriority(importance Importance) int {
	switch importance {
	case ImportanceHigh:
		return 1
	case ImportanceNormal:
		return 2
	case ImportanceLow:
		return 3
	default:
		return 2
	}
}

// PriorityToImportance 将 TaskBridge Priority 转换为 Microsoft Importance
func PriorityToImportance(priority int) Importance {
	switch priority {
	case 1:
		return ImportanceHigh
	case 2:
		return ImportanceNormal
	case 3, 4:
		return ImportanceLow
	default:
		return ImportanceNormal
	}
}

// StatusToTaskStatus 将 TaskBridge Status 转换为 Microsoft TaskStatus
func StatusToTaskStatus(status string) TaskStatus {
	switch status {
	case "todo", "pending":
		return StatusNotStarted
	case "in_progress":
		return StatusInProgress
	case "completed":
		return StatusCompleted
	case "waiting":
		return StatusWaitingOnOthers
	case "deferred", "cancelled":
		return StatusDeferred
	default:
		return StatusNotStarted
	}
}

// TaskStatusToStatus 将 Microsoft TaskStatus 转换为 TaskBridge Status
func TaskStatusToStatus(status TaskStatus) string {
	switch status {
	case StatusNotStarted:
		return "todo"
	case StatusInProgress:
		return "in_progress"
	case StatusCompleted:
		return "completed"
	case StatusWaitingOnOthers:
		return "waiting"
	case StatusDeferred:
		return "deferred"
	default:
		return "todo"
	}
}
