// Package microsoft provides Microsoft To Do provider implementation
package microsoft

import (
	"time"

	"github.com/yeisme/taskbridge/internal/model"
)

// ================ 任务列表转换 ================

// ToModelTaskList 将 Microsoft TodoTaskList 转换为统一模型
func ToModelTaskList(list *TodoTaskList) *model.TaskList {
	if list == nil {
		return nil
	}

	return &model.TaskList{
		ID:          list.ID,
		Name:        list.DisplayName,
		Source:      model.SourceMicrosoft,
		SourceRawID: list.ID,
		CreatedAt:   list.CreatedDateTime,
		UpdatedAt:   list.LastModified,
	}
}

// ================ 任务转换 ================

// ToModelTask 将 Microsoft TodoTask 转换为统一模型
func ToModelTask(task *TodoTask) *model.Task {
	if task == nil {
		return nil
	}

	// 基础字段
	mTask := &model.Task{
		ID:          task.ID,
		Title:       task.Title,
		Status:      ToModelStatus(task.Status),
		Source:      model.SourceMicrosoft,
		SourceRawID: task.ID,
		CreatedAt:   task.CreatedDateTime,
		UpdatedAt:   task.LastModifiedDateTime,
	}

	// 重要性转优先级
	mTask.Priority = ToModelPriority(task.Importance)

	// 描述
	if task.Body != nil && task.Body.Content != "" {
		mTask.Description = task.Body.Content
	}

	// 截止日期
	if task.DueDateTime != nil {
		mTask.DueDate = FromDateTimeTimeZone(task.DueDateTime)
	}

	// 开始日期
	if task.StartDateTime != nil {
		mTask.StartDate = FromDateTimeTimeZone(task.StartDateTime)
	}

	// 完成时间
	if task.CompletedDateTime != nil {
		completedAt := FromDateTimeTimeZone(task.CompletedDateTime)
		if completedAt != nil {
			mTask.CompletedAt = completedAt
		}
	}

	// 元数据
	mTask.Metadata = &model.TaskMetadata{
		Version:    "1.0",
		LastSyncAt: time.Now(),
		SyncSource: "microsoft",
		LocalID:    task.ID,
	}

	// 提醒
	if task.IsReminderOn && task.ReminderDateTime != nil {
		mTask.Reminder = FromDateTimeTimeZone(task.ReminderDateTime)
	}

	// 子任务 ID（检查项）
	if len(task.ChecklistItems) > 0 {
		mTask.SubtaskIDs = make([]string, 0, len(task.ChecklistItems))
		for _, item := range task.ChecklistItems {
			mTask.SubtaskIDs = append(mTask.SubtaskIDs, item.ID)
		}
	}

	// 分类
	if len(task.Categories) > 0 {
		mTask.Categories = task.Categories
	}

	// 关联资源 - 存储到自定义字段
	if len(task.LinkedResources) > 0 && mTask.Metadata.CustomFields == nil {
		mTask.Metadata.CustomFields = make(map[string]interface{})
		links := make([]map[string]interface{}, 0, len(task.LinkedResources))
		for _, lr := range task.LinkedResources {
			links = append(links, map[string]interface{}{
				"id":               lr.ID,
				"web_url":          lr.WebURL,
				"application_name": lr.ApplicationName,
				"display_name":     lr.DisplayName,
				"external_id":      lr.ExternalID,
			})
		}
		mTask.Metadata.CustomFields["linked_resources"] = links
	}

	// 重复规则
	if task.Recurrence != nil && mTask.Metadata.CustomFields == nil {
		mTask.Metadata.CustomFields = make(map[string]interface{})
		mTask.Metadata.CustomFields["recurrence"] = map[string]interface{}{
			"pattern": task.Recurrence.Pattern,
			"range":   task.Recurrence.Range,
		}
	}

	// 附件标记
	if task.HasAttachments && mTask.Metadata.CustomFields == nil {
		mTask.Metadata.CustomFields = make(map[string]interface{})
		mTask.Metadata.CustomFields["has_attachments"] = true
	}

	// 计算象限
	mTask.Quadrant = CalculateQuadrantFromMSTask(task)

	return mTask
}

// ToMicrosoftTask 将统一模型转换为 Microsoft TodoTask
func ToMicrosoftTask(task *model.Task) *TodoTask {
	if task == nil {
		return nil
	}

	msTask := &TodoTask{
		ID:         task.ID,
		Title:      task.Title,
		Status:     ToMicrosoftStatus(task.Status),
		Importance: ToMicrosoftImportance(task.Priority),
	}

	// 描述
	if task.Description != "" {
		msTask.Body = &ItemBody{
			Content:     task.Description,
			ContentType: ContentTypeText,
		}
	}

	// 截止日期
	if task.DueDate != nil {
		msTask.DueDateTime = ToDateTimeTimeZone(task.DueDate, "")
	}

	// 开始日期
	if task.StartDate != nil {
		msTask.StartDateTime = ToDateTimeTimeZone(task.StartDate, "")
	}

	// 完成时间
	if task.Status == model.StatusCompleted && task.CompletedAt != nil {
		msTask.CompletedDateTime = ToDateTimeTimeZone(task.CompletedAt, "")
	}

	// 提醒
	if task.Reminder != nil {
		msTask.ReminderDateTime = ToDateTimeTimeZone(task.Reminder, "")
		msTask.IsReminderOn = true
	}

	// 分类
	if len(task.Categories) > 0 {
		msTask.Categories = task.Categories
	}

	// 从元数据恢复关联资源
	if task.Metadata != nil && task.Metadata.CustomFields != nil {
		if links, ok := task.Metadata.CustomFields["linked_resources"].([]map[string]interface{}); ok {
			msTask.LinkedResources = make([]LinkedResource, 0, len(links))
			for _, link := range links {
				lr := LinkedResource{}
				if id, ok := link["id"].(string); ok {
					lr.ID = id
				}
				if webURL, ok := link["web_url"].(string); ok {
					lr.WebURL = webURL
				}
				if appName, ok := link["application_name"].(string); ok {
					lr.ApplicationName = appName
				}
				if displayName, ok := link["display_name"].(string); ok {
					lr.DisplayName = displayName
				}
				if externalID, ok := link["external_id"].(string); ok {
					lr.ExternalID = externalID
				}
				msTask.LinkedResources = append(msTask.LinkedResources, lr)
			}
		}
	}

	return msTask
}

// ================ 状态转换 ================

// ToModelStatus 将 Microsoft TaskStatus 转换为 model.TaskStatus
func ToModelStatus(status TaskStatus) model.TaskStatus {
	switch status {
	case StatusNotStarted:
		return model.StatusTodo
	case StatusInProgress:
		return model.StatusInProgress
	case StatusCompleted:
		return model.StatusCompleted
	case StatusWaitingOnOthers:
		return model.StatusDeferred
	case StatusDeferred:
		return model.StatusDeferred
	default:
		return model.StatusTodo
	}
}

// ToMicrosoftStatus 将 model.TaskStatus 转换为 Microsoft TaskStatus
func ToMicrosoftStatus(status model.TaskStatus) TaskStatus {
	switch status {
	case model.StatusTodo:
		return StatusNotStarted
	case model.StatusInProgress:
		return StatusInProgress
	case model.StatusCompleted:
		return StatusCompleted
	case model.StatusDeferred:
		return StatusDeferred
	case model.StatusCancelled:
		return StatusDeferred
	default:
		return StatusNotStarted
	}
}

// ================ 优先级转换 ================

// ToModelPriority 将 Microsoft Importance 转换为 model.Priority
func ToModelPriority(importance Importance) model.Priority {
	switch importance {
	case ImportanceHigh:
		return model.PriorityHigh
	case ImportanceNormal:
		return model.PriorityMedium
	case ImportanceLow:
		return model.PriorityLow
	default:
		return model.PriorityNone
	}
}

// ToMicrosoftImportance 将 model.Priority 转换为 Microsoft Importance
func ToMicrosoftImportance(priority model.Priority) Importance {
	switch priority {
	case model.PriorityUrgent, model.PriorityHigh:
		return ImportanceHigh
	case model.PriorityMedium:
		return ImportanceNormal
	case model.PriorityLow, model.PriorityNone:
		return ImportanceLow
	default:
		return ImportanceNormal
	}
}

// ================ 批量转换 ================

// ToModelTasks 批量转换为统一模型
func ToModelTasks(tasks []TodoTask) []model.Task {
	if tasks == nil {
		return nil
	}

	result := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, *ToModelTask(&task))
	}
	return result
}

// ToMicrosoftTasks 批量转换为 Microsoft 任务
func ToMicrosoftTasks(tasks []model.Task) []TodoTask {
	if tasks == nil {
		return nil
	}

	result := make([]TodoTask, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, *ToMicrosoftTask(&task))
	}
	return result
}

// ToModelTaskLists 批量转换为统一任务列表
func ToModelTaskLists(lists []TodoTaskList) []model.TaskList {
	if lists == nil {
		return nil
	}

	result := make([]model.TaskList, 0, len(lists))
	for _, list := range lists {
		result = append(result, *ToModelTaskList(&list))
	}
	return result
}

// ================ 时间处理辅助函数 ================

// ParseMicrosoftTime 解析 Microsoft 时间格式
func ParseMicrosoftTime(dateTimeStr, timezone string) (time.Time, error) {
	loc := time.Local
	if timezone != "" {
		var err error
		loc, err = time.LoadLocation(timezone)
		if err != nil {
			loc = time.Local
		}
	}

	// 尝试多种格式
	formats := []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05.0000000",
		"2006-01-02",
		time.RFC3339,
		time.RFC3339Nano,
	}

	for _, format := range formats {
		if t, err := time.ParseInLocation(format, dateTimeStr, loc); err == nil {
			return t, nil
		}
	}

	return time.Time{}, nil
}

// FormatMicrosoftTime 格式化为 Microsoft 时间格式
func FormatMicrosoftTime(t time.Time, timezone string) string {
	loc := time.Local
	if timezone != "" {
		var err error
		loc, err = time.LoadLocation(timezone)
		if err != nil {
			loc = time.Local
		}
	}

	return t.In(loc).Format("2006-01-02T15:04:05")
}

// ================ 四象限映射 ================

// CalculateQuadrantFromMSTask 根据任务属性计算四象限
func CalculateQuadrantFromMSTask(task *TodoTask) model.Quadrant {
	if task == nil {
		return model.QuadrantNotUrgentNotImportant
	}

	// 基于重要性和截止日期计算
	isImportant := task.Importance == ImportanceHigh
	isUrgent := false

	// 检查是否紧急（截止日期在 3 天内）
	if task.DueDateTime != nil {
		dueDate := FromDateTimeTimeZone(task.DueDateTime)
		if dueDate != nil {
			daysUntilDue := time.Until(*dueDate).Hours() / 24
			isUrgent = daysUntilDue <= 3 && daysUntilDue >= 0
		}
	}

	// 映射到四象限
	if isImportant && isUrgent {
		return model.QuadrantUrgentImportant // 紧急且重要
	} else if isImportant && !isUrgent {
		return model.QuadrantNotUrgentImportant // 重要不紧急
	} else if !isImportant && isUrgent {
		return model.QuadrantUrgentNotImportant // 紧急不重要
	} else {
		return model.QuadrantNotUrgentNotImportant // 不紧急不重要
	}
}

// ================ 状态映射 ================

// MapStatusToProgress 将任务状态映射为进度百分比
func MapStatusToProgress(status TaskStatus) int {
	switch status {
	case StatusNotStarted:
		return 0
	case StatusInProgress:
		return 50
	case StatusWaitingOnOthers:
		return 25
	case StatusDeferred:
		return 10
	case StatusCompleted:
		return 100
	default:
		return 0
	}
}

// MapProgressToStatus 将进度百分比映射为任务状态
func MapProgressToStatus(progress int) TaskStatus {
	if progress >= 100 {
		return StatusCompleted
	} else if progress >= 50 {
		return StatusInProgress
	} else if progress >= 25 {
		return StatusWaitingOnOthers
	} else if progress > 0 {
		return StatusInProgress
	}
	return StatusNotStarted
}
