package todoist

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.todoist.com/api/v1"
	defaultTimeout = 30 * time.Second
)

// TokenFile Todoist token 存储格式。
type TokenFile struct {
	APIToken  string    `json:"api_token"`
	Provider  string    `json:"provider,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// ID 兼容 Todoist 数字或字符串 ID。
type ID string

func (id *ID) UnmarshalJSON(data []byte) error {
	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		*id = ID(strings.TrimSpace(asString))
		return nil
	}

	var asNumber int64
	if err := json.Unmarshal(data, &asNumber); err == nil {
		*id = ID(strconv.FormatInt(asNumber, 10))
		return nil
	}

	return fmt.Errorf("invalid id format: %s", string(data))
}

func (id ID) String() string {
	return string(id)
}

// Project Todoist 项目。
type Project struct {
	ID   ID     `json:"id"`
	Name string `json:"name"`
}

// Due Todoist 截止时间。
type Due struct {
	Date     string `json:"date"`
	Datetime string `json:"datetime"`
	Timezone string `json:"timezone"`
	String   string `json:"string"`
}

// Task Todoist 任务。
type Task struct {
	ID          ID       `json:"id"`
	ProjectID   ID       `json:"project_id"`
	Content     string   `json:"content"`
	Description string   `json:"description"`
	Checked     bool     `json:"checked"`
	AddedAt     string   `json:"added_at"`
	UpdatedAt   string   `json:"updated_at"`
	CompletedAt string   `json:"completed_at"`
	Due         *Due     `json:"due"`
	Priority    int      `json:"priority"`
	Labels      []string `json:"labels"`
	ParentID    ID       `json:"parent_id"`
	URL         string   `json:"url"`
}

// CreateTaskRequest 创建任务请求。
type CreateTaskRequest struct {
	Content     string   `json:"content"`
	Description string   `json:"description,omitempty"`
	ProjectID   *int64   `json:"project_id,omitempty"`
	Priority    int      `json:"priority,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	ParentID    *int64   `json:"parent_id,omitempty"`
	DueDate     string   `json:"due_date,omitempty"`
	DueString   string   `json:"due_string,omitempty"`
}

// UpdateTaskRequest 更新任务请求。
type UpdateTaskRequest struct {
	Content     string   `json:"content,omitempty"`
	Description string   `json:"description,omitempty"`
	Priority    int      `json:"priority,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	ParentID    *int64   `json:"parent_id,omitempty"`
	DueDate     string   `json:"due_date,omitempty"`
	DueString   string   `json:"due_string,omitempty"`
}

type pagedProjectsResponse struct {
	Results    []Project `json:"results"`
	NextCursor string    `json:"next_cursor"`
}

type pagedTasksResponse struct {
	Results    []Task `json:"results"`
	NextCursor string `json:"next_cursor"`
}
