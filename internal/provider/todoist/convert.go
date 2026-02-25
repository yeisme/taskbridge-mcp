package todoist

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/yeisme/taskbridge/internal/model"
)

func toModelTaskList(project *Project) *model.TaskList {
	if project == nil {
		return nil
	}
	return &model.TaskList{
		ID:          project.ID.String(),
		Name:        project.Name,
		Source:      model.SourceTodoist,
		SourceRawID: project.ID.String(),
	}
}

func toModelTask(task *Task) *model.Task {
	if task == nil {
		return nil
	}

	mTask := &model.Task{
		ID:          task.ID.String(),
		Title:       task.Content,
		Description: task.Description,
		Source:      model.SourceTodoist,
		SourceRawID: task.ID.String(),
		ListID:      task.ProjectID.String(),
		Tags:        append([]string{}, task.Labels...),
		Priority:    model.PriorityFromInt(task.Priority),
		Status:      model.StatusTodo,
	}

	if task.Checked {
		mTask.Status = model.StatusCompleted
		if task.CompletedAt != "" {
			if completed, err := time.Parse(time.RFC3339, task.CompletedAt); err == nil {
				mTask.CompletedAt = &completed
			}
		}
		if mTask.CompletedAt == nil {
			now := time.Now()
			mTask.CompletedAt = &now
		}
	}

	if task.ParentID.String() != "" {
		parent := task.ParentID.String()
		mTask.ParentID = &parent
	}

	if task.AddedAt != "" {
		if created, err := time.Parse(time.RFC3339, task.AddedAt); err == nil {
			mTask.CreatedAt = created
		}
	}
	if task.UpdatedAt != "" {
		if updated, err := time.Parse(time.RFC3339, task.UpdatedAt); err == nil {
			mTask.UpdatedAt = updated
		}
	}

	if task.Due != nil {
		due := parseDue(task.Due)
		mTask.DueDate = due
	}

	if mTask.CreatedAt.IsZero() {
		mTask.CreatedAt = time.Now()
	}
	if mTask.UpdatedAt.IsZero() {
		mTask.UpdatedAt = mTask.CreatedAt
	}

	return mTask
}

func toCreateTaskRequest(task *model.Task, listID string) *CreateTaskRequest {
	if task == nil {
		return nil
	}

	req := &CreateTaskRequest{
		Content:     task.Title,
		Description: task.Description,
		Priority:    int(task.Priority),
		Labels:      append([]string{}, task.Tags...),
	}
	if req.Priority < 1 {
		req.Priority = 1
	}
	if req.Priority > 4 {
		req.Priority = 4
	}

	if id, ok := parseIntID(listID); ok {
		req.ProjectID = &id
	}

	if task.ParentID != nil {
		if id, ok := parseIntID(*task.ParentID); ok {
			req.ParentID = &id
		}
	}

	if task.DueDate != nil {
		req.DueDate = task.DueDate.Format("2006-01-02")
	}

	return req
}

func toUpdateTaskRequest(task *model.Task) *UpdateTaskRequest {
	if task == nil {
		return nil
	}

	req := &UpdateTaskRequest{
		Content:     task.Title,
		Description: task.Description,
		Priority:    int(task.Priority),
		Labels:      append([]string{}, task.Tags...),
	}
	if req.Priority < 1 {
		req.Priority = 1
	}
	if req.Priority > 4 {
		req.Priority = 4
	}

	if task.ParentID != nil {
		if id, ok := parseIntID(*task.ParentID); ok {
			req.ParentID = &id
		}
	}

	if task.DueDate != nil {
		req.DueDate = task.DueDate.Format("2006-01-02")
	}

	return req
}

func parseDue(due *Due) *time.Time {
	if due == nil {
		return nil
	}

	candidates := []string{due.Datetime, due.Date}
	for _, value := range candidates {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, value); err == nil {
			return &t
		}
		if t, err := time.Parse("2006-01-02", value); err == nil {
			return &t
		}
	}
	return nil
}

func parseIntID(id string) (int64, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0, false
	}
	v, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func containsIgnoreCase(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "已过期"
	}
	if d < time.Minute {
		return fmt.Sprintf("%d秒", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d分钟", int(d.Minutes()))
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if minutes == 0 {
		return fmt.Sprintf("%d小时", hours)
	}
	return fmt.Sprintf("%d小时%d分钟", hours, minutes)
}
