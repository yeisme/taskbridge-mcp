// Package feishu provides Feishu (Lark) Task provider implementation
package feishu

import (
	"fmt"
	"time"

	"github.com/yeisme/taskbridge/internal/model"
)

// ================ 任务列表转换 ================

// ToModelTaskList 将飞书 TaskList 转换为统一模型
func ToModelTaskList(list *TaskList) *model.TaskList {
	if list == nil {
		return nil
	}

	return &model.TaskList{
		ID:          list.TaskListID,
		Name:        list.Name,
		Source:      model.SourceFeishu,
		SourceRawID: list.TaskListID,
		CreatedAt:   MillisecondsToTime(list.CreatedTime),
		UpdatedAt:   MillisecondsToTime(list.CreatedTime), // 飞书没有单独的更新时间
	}
}

// ToFeishuTaskList 将统一模型转换为飞书 TaskList
func ToFeishuTaskList(list *model.TaskList) *TaskList {
	if list == nil {
		return nil
	}

	return &TaskList{
		TaskListID: list.ID,
		Name:       list.Name,
	}
}

// ================ 任务转换 ================

// ToModelTask 将飞书 Task 转换为统一模型
func ToModelTask(task *Task) *model.Task {
	if task == nil {
		return nil
	}

	// 基础字段
	mTask := &model.Task{
		ID:          task.TaskID,
		Title:       task.Title,
		Description: task.Description,
		Status:      ToModelStatus(task.Status),
		Source:      model.SourceFeishu,
		CreatedAt:   MillisecondsToTime(task.CreatedTime),
		UpdatedAt:   MillisecondsToTime(task.UpdatedTime),
	}

	// 优先级
	mTask.Priority = ToModelPriority(task.Priority)

	// 截止日期
	if task.DueTime > 0 {
		mTask.DueDate = MillisecondsToTimePtr(task.DueTime)
	}

	// 开始日期
	if task.StartTime > 0 {
		mTask.StartDate = MillisecondsToTimePtr(task.StartTime)
	}

	// 完成时间
	if task.CompletedTime > 0 {
		mTask.CompletedAt = MillisecondsToTimePtr(task.CompletedTime)
	}

	// 元数据
	mTask.Metadata = &model.TaskMetadata{
		Version:    "1.0",
		LastSyncAt: time.Now(),
		SyncSource: "feishu",
	}

	// 提醒
	if len(task.Reminders) > 0 {
		for _, reminder := range task.Reminders {
			if reminder.Enabled && reminder.AbsoluteTime > 0 {
				mTask.Reminder = MillisecondsToTimePtr(reminder.AbsoluteTime)
				break
			}
		}
	}

	// 子任务 ID
	if len(task.SubtaskIDs) > 0 {
		mTask.SubtaskIDs = task.SubtaskIDs
	}
	if task.ParentTaskID != "" {
		parentID := task.ParentTaskID
		mTask.ParentID = &parentID
	}

	// 标签转分类
	if len(task.Tags) > 0 {
		mTask.Categories = make([]string, 0, len(task.Tags))
		for _, tag := range task.Tags {
			mTask.Categories = append(mTask.Categories, tag.Name)
		}
	}

	// 重复规则
	if task.Repeat != nil && mTask.Metadata.CustomFields == nil {
		mTask.Metadata.CustomFields = make(map[string]interface{})
		mTask.Metadata.CustomFields["repeat"] = map[string]interface{}{
			"type":         task.Repeat.Type,
			"interval":     task.Repeat.Interval,
			"end_time":     task.Repeat.EndTime,
			"count":        task.Repeat.Count,
			"days_of_week": task.Repeat.DaysOfWeek,
			"day_of_month": task.Repeat.DayOfMonth,
		}
	}

	// 协作者
	if len(task.CollaboratorIDs) > 0 && mTask.Metadata.CustomFields == nil {
		mTask.Metadata.CustomFields = make(map[string]interface{})
		mTask.Metadata.CustomFields["collaborator_ids"] = task.CollaboratorIDs
	}

	// 附件标记
	if len(task.Attachments) > 0 && mTask.Metadata.CustomFields == nil {
		mTask.Metadata.CustomFields = make(map[string]interface{})
		mTask.Metadata.CustomFields["has_attachments"] = true
		mTask.Metadata.CustomFields["attachment_count"] = len(task.Attachments)
	}

	// 自定义列
	if len(task.CustomColumns) > 0 && mTask.Metadata.CustomFields == nil {
		mTask.Metadata.CustomFields = make(map[string]interface{})
		customFields := make(map[string]interface{})
		for _, col := range task.CustomColumns {
			customFields[col.ColumnID] = col.Value
		}
		mTask.Metadata.CustomFields["custom_columns"] = customFields
	}

	// 所属任务列表
	if len(task.TasklistIDs) > 0 && mTask.Metadata.CustomFields == nil {
		mTask.Metadata.CustomFields = make(map[string]interface{})
		mTask.Metadata.CustomFields["tasklist_ids"] = task.TasklistIDs
	}

	// 计算象限
	mTask.Quadrant = CalculateQuadrantFromFeishuTask(task)

	return mTask
}

// ToFeishuTask 将统一模型转换为飞书 Task
func ToFeishuTask(task *model.Task) *Task {
	if task == nil {
		return nil
	}

	fTask := &Task{
		TaskID:   task.ID,
		Title:    task.Title,
		Status:   ToFeishuStatus(task.Status),
		Priority: ToFeishuPriority(task.Priority),
	}

	// 描述
	if task.Description != "" {
		fTask.Description = task.Description
	}

	// 截止日期
	if task.DueDate != nil {
		fTask.DueTime = TimePtrToMilliseconds(task.DueDate)
	}

	// 开始日期
	if task.StartDate != nil {
		fTask.StartTime = TimePtrToMilliseconds(task.StartDate)
	}

	// 完成时间
	if task.Status == model.StatusCompleted && task.CompletedAt != nil {
		fTask.CompletedTime = TimePtrToMilliseconds(task.CompletedAt)
	}

	// 提醒
	if task.Reminder != nil {
		fTask.Reminders = []Reminder{
			{
				Type:         ReminderTypeAbsolute,
				AbsoluteTime: TimePtrToMilliseconds(task.Reminder),
				Enabled:      true,
			},
		}
	}

	// 分类转标签
	if len(task.Categories) > 0 {
		fTask.Tags = make([]Tag, 0, len(task.Categories))
		for i, cat := range task.Categories {
			fTask.Tags = append(fTask.Tags, Tag{
				TagID: fmt.Sprintf("tag_%d", i),
				Name:  cat,
			})
		}
	}

	// 从元数据恢复其他字段
	if task.Metadata != nil && task.Metadata.CustomFields != nil {
		// 恢复重复规则
		if repeat, ok := task.Metadata.CustomFields["repeat"].(map[string]interface{}); ok {
			fTask.Repeat = &RepeatRule{}
			if t, ok := repeat["type"].(RepeatType); ok {
				fTask.Repeat.Type = t
			}
			if interval, ok := repeat["interval"].(int); ok {
				fTask.Repeat.Interval = interval
			}
			if endTime, ok := repeat["end_time"].(int64); ok {
				fTask.Repeat.EndTime = endTime
			}
			if count, ok := repeat["count"].(int); ok {
				fTask.Repeat.Count = count
			}
			if daysOfWeek, ok := repeat["days_of_week"].([]int); ok {
				fTask.Repeat.DaysOfWeek = daysOfWeek
			}
			if dayOfMonth, ok := repeat["day_of_month"].(int); ok {
				fTask.Repeat.DayOfMonth = dayOfMonth
			}
		}

		// 恢复协作者
		if collaboratorIDs, ok := task.Metadata.CustomFields["collaborator_ids"].([]string); ok {
			fTask.CollaboratorIDs = collaboratorIDs
		}

		// 恢复任务列表 ID
		if tasklistIDs, ok := task.Metadata.CustomFields["tasklist_ids"].([]string); ok {
			fTask.TasklistIDs = tasklistIDs
		}
	}

	return fTask
}

// ToFeishuCreateRequest 将统一模型转换为飞书创建任务请求
func ToFeishuCreateRequest(task *model.Task, listID string) *CreateTaskRequest {
	fTask := ToFeishuTask(task)
	return &CreateTaskRequest{
		Task:        *fTask,
		TasklistIDs: []string{listID},
	}
}

// ToFeishuUpdateRequest 将统一模型转换为飞书更新任务请求
func ToFeishuUpdateRequest(task *model.Task) *UpdateTaskRequest {
	fTask := ToFeishuTask(task)
	return &UpdateTaskRequest{
		Task:         *fTask,
		UpdateFields: []string{"title", "description", "status", "priority", "due_time", "start_time", "reminders"},
	}
}

// ================ 状态转换 ================

// ToModelStatus 将飞书 TaskStatus 转换为 model.TaskStatus
func ToModelStatus(status TaskStatus) model.TaskStatus {
	switch status {
	case StatusTodo:
		return model.StatusTodo
	case StatusInProgress:
		return model.StatusInProgress
	case StatusDone:
		return model.StatusCompleted
	default:
		return model.StatusTodo
	}
}

// ToFeishuStatus 将 model.TaskStatus 转换为飞书 TaskStatus
func ToFeishuStatus(status model.TaskStatus) TaskStatus {
	switch status {
	case model.StatusTodo:
		return StatusTodo
	case model.StatusInProgress:
		return StatusInProgress
	case model.StatusCompleted:
		return StatusDone
	case model.StatusDeferred, model.StatusCancelled:
		return StatusTodo
	default:
		return StatusTodo
	}
}

// ================ 优先级转换 ================

// ToModelPriority 将飞书 TaskPriority 转换为 model.Priority
func ToModelPriority(priority TaskPriority) model.Priority {
	switch priority {
	case PriorityUrgent:
		return model.PriorityUrgent
	case PriorityHigh:
		return model.PriorityHigh
	case PriorityMedium:
		return model.PriorityMedium
	case PriorityLow:
		return model.PriorityLow
	default:
		return model.PriorityNone
	}
}

// ToFeishuPriority 将 model.Priority 转换为飞书 TaskPriority
func ToFeishuPriority(priority model.Priority) TaskPriority {
	switch priority {
	case model.PriorityUrgent:
		return PriorityUrgent
	case model.PriorityHigh:
		return PriorityHigh
	case model.PriorityMedium:
		return PriorityMedium
	case model.PriorityLow:
		return PriorityLow
	default:
		return PriorityNone
	}
}

// ================ 批量转换 ================

// ToModelTasks 批量转换为统一模型
func ToModelTasks(tasks []Task) []model.Task {
	if tasks == nil {
		return nil
	}

	result := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, *ToModelTask(&task))
	}
	return result
}

// ToFeishuTasks 批量转换为飞书任务
func ToFeishuTasks(tasks []model.Task) []Task {
	if tasks == nil {
		return nil
	}

	result := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, *ToFeishuTask(&task))
	}
	return result
}

// ToModelTaskLists 批量转换为统一任务列表
func ToModelTaskLists(lists []TaskList) []model.TaskList {
	if lists == nil {
		return nil
	}

	result := make([]model.TaskList, 0, len(lists))
	for _, list := range lists {
		result = append(result, *ToModelTaskList(&list))
	}
	return result
}

// ================ 四象限映射 ================

// CalculateQuadrantFromFeishuTask 根据任务属性计算四象限
func CalculateQuadrantFromFeishuTask(task *Task) model.Quadrant {
	if task == nil {
		return model.QuadrantNotUrgentNotImportant
	}

	// 基于优先级和截止日期计算
	isImportant := task.Priority == PriorityUrgent || task.Priority == PriorityHigh
	isUrgent := false

	// 检查是否紧急（截止日期在 3 天内）
	if task.DueTime > 0 {
		dueDate := MillisecondsToTime(task.DueTime)
		daysUntilDue := time.Until(dueDate).Hours() / 24
		isUrgent = daysUntilDue <= 3 && daysUntilDue >= 0
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

// ================ 时间处理辅助函数 ================

// ParseFeishuTime 解析飞书时间格式（毫秒时间戳）
func ParseFeishuTime(ms int64) time.Time {
	return MillisecondsToTime(ms)
}

// FormatFeishuTime 格式化为飞书时间格式（毫秒时间戳）
func FormatFeishuTime(t time.Time) int64 {
	return TimeToMilliseconds(t)
}

// ================ 变更类型转换 ================

// ToModelChangeType 将飞书变更类型转换为 model 变更类型
func ToModelChangeType(changeType ChangeType) string {
	switch changeType {
	case ChangeTypeCreated:
		return "created"
	case ChangeTypeUpdated:
		return "updated"
	case ChangeTypeDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}

// ================ 子任务转换 ================

// ToModelSubtask 将飞书子任务转换为 model 子任务信息
func ToModelSubtask(subtask *Subtask) map[string]interface{} {
	if subtask == nil {
		return nil
	}

	return map[string]interface{}{
		"id":             subtask.SubtaskID,
		"title":          subtask.Title,
		"is_completed":   subtask.IsCompleted,
		"completed_time": subtask.CompletedTime,
		"created_time":   subtask.CreatedTime,
		"creator_id":     subtask.CreatorID,
	}
}

// ================ 标签转换 ================

// ToModelTag 将飞书标签转换为 model 标签
func ToModelTag(tag *Tag) map[string]interface{} {
	if tag == nil {
		return nil
	}

	return map[string]interface{}{
		"id":    tag.TagID,
		"name":  tag.Name,
		"color": tag.Color,
	}
}

// ================ 附件转换 ================

// ToModelAttachment 将飞书附件转换为 model 附件
func ToModelAttachment(attachment *Attachment) map[string]interface{} {
	if attachment == nil {
		return nil
	}

	return map[string]interface{}{
		"id":           attachment.AttachmentID,
		"name":         attachment.Name,
		"size":         attachment.Size,
		"mime_type":    attachment.MimeType,
		"url":          attachment.URL,
		"created_time": attachment.CreatedTime,
	}
}
