// Package google provides Google Tasks API client implementation
package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/yeisme/taskbridge/internal/model"
)

const (
	// BaseURL Google Tasks API 基础 URL
	BaseURL = "https://tasks.googleapis.com/tasks/v1"
	// DefaultTimeout 默认请求超时
	DefaultTimeout = 30 * time.Second
)

// Client Google Tasks API 客户端
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string // OAuth2 access token
}

// NewClient 创建新的 Google Tasks 客户端
func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		baseURL: BaseURL,
		token:   token,
	}
}

// SetHTTPClient 设置自定义 HTTP 客户端
func (c *Client) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}

// TaskList Google任务列表
// https://developers.google.com/workspace/tasks/reference/rest/v1/tasklists#resource-tasklist
type TaskList struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Updated  string `json:"updated"`
	SelfLink string `json:"selfLink"`
}

// TaskListCollection 任务列表集合
type TaskListCollection struct {
	Items         []TaskList `json:"items"`
	NextPageToken string     `json:"nextPageToken"`
}

// Task Google 任务
// https://developers.google.com/workspace/tasks/reference/rest/v1/tasks#resource-task
type Task struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Notes     string `json:"notes,omitempty"`
	Status    string `json:"status"`              // needsAction, completed
	Due       string `json:"due,omitempty"`       // RFC 3339 timestamp
	Completed string `json:"completed,omitempty"` // RFC 3339 timestamp
	Deleted   bool   `json:"deleted"`
	Hidden    bool   `json:"hidden"`
	Parent    string `json:"parent,omitempty"`
	Position  string `json:"position,omitempty"`
	SelfLink  string `json:"selfLink"`
	Updated   string `json:"updated"`
	Links     []Link `json:"links,omitempty"`
}

// Link 任务链接
type Link struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Link        string `json:"link"`
}

// TaskCollection 任务集合
type TaskCollection struct {
	Items         []Task `json:"items"`
	NextPageToken string `json:"nextPageToken"`
}

// ListTaskLists 获取任务列表
// https://developers.google.com/workspace/tasks/reference/rest/v1/tasklists/list
func (c *Client) ListTaskLists(ctx context.Context, pageToken string, maxResults int64) (*TaskListCollection, error) {
	u := fmt.Sprintf("%s/users/@me/lists", c.baseURL)

	params := url.Values{}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}
	if maxResults > 0 {
		params.Set("maxResults", fmt.Sprintf("%d", maxResults))
	}
	if len(params) > 0 {
		u = u + "?" + params.Encode()
	}

	var result TaskListCollection
	if err := c.doRequest(ctx, http.MethodGet, u, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTaskList 获取单个任务列表
// https://developers.google.com/workspace/tasks/reference/rest/v1/tasklists/get
func (c *Client) GetTaskList(ctx context.Context, tasklistID string) (*TaskList, error) {
	u := fmt.Sprintf("%s/users/@me/lists/%s", c.baseURL, tasklistID)

	var result TaskList
	if err := c.doRequest(ctx, http.MethodGet, u, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateTaskList 创建任务列表
// https://developers.google.com/workspace/tasks/reference/rest/v1/tasklists/insert
func (c *Client) CreateTaskList(ctx context.Context, title string) (*TaskList, error) {
	u := fmt.Sprintf("%s/users/@me/lists", c.baseURL)

	body := map[string]string{"title": title}
	var result TaskList
	if err := c.doRequest(ctx, http.MethodPost, u, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateTaskList 更新任务列表
// https://developers.google.com/workspace/tasks/reference/rest/v1/tasklists/update
func (c *Client) UpdateTaskList(ctx context.Context, tasklistID, title string) (*TaskList, error) {
	u := fmt.Sprintf("%s/users/@me/lists/%s", c.baseURL, tasklistID)

	body := map[string]string{
		"id":    tasklistID,
		"title": title,
	}
	var result TaskList
	if err := c.doRequest(ctx, http.MethodPut, u, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteTaskList 删除任务列表
// https://developers.google.com/workspace/tasks/reference/rest/v1/tasklists/delete
func (c *Client) DeleteTaskList(ctx context.Context, tasklistID string) error {
	u := fmt.Sprintf("%s/users/@me/lists/%s", c.baseURL, tasklistID)
	return c.doRequest(ctx, http.MethodDelete, u, nil, nil)
}

// ListTasks 获取任务列表中的任务
// https://developers.google.com/workspace/tasks/reference/rest/v1/tasks/list
func (c *Client) ListTasks(ctx context.Context, tasklistID string, opts ListTasksOptions) (*TaskCollection, error) {
	u := fmt.Sprintf("%s/lists/%s/tasks", c.baseURL, tasklistID)

	params := url.Values{}
	if opts.PageToken != "" {
		params.Set("pageToken", opts.PageToken)
	}
	if opts.MaxResults > 0 {
		params.Set("maxResults", fmt.Sprintf("%d", opts.MaxResults))
	}
	if opts.CompletedMax != "" {
		params.Set("completedMax", opts.CompletedMax)
	}
	if opts.CompletedMin != "" {
		params.Set("completedMin", opts.CompletedMin)
	}
	if opts.DueMax != "" {
		params.Set("dueMax", opts.DueMax)
	}
	if opts.DueMin != "" {
		params.Set("dueMin", opts.DueMin)
	}
	if opts.ShowCompleted {
		params.Set("showCompleted", "true")
	}
	if opts.ShowDeleted {
		params.Set("showDeleted", "true")
	}
	if opts.ShowHidden {
		params.Set("showHidden", "true")
	}
	if opts.UpdatedMin != "" {
		params.Set("updatedMin", opts.UpdatedMin)
	}
	if len(params) > 0 {
		u = u + "?" + params.Encode()
	}

	var result TaskCollection
	if err := c.doRequest(ctx, http.MethodGet, u, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListTasksOptions 任务列表选项
type ListTasksOptions struct {
	PageToken     string
	MaxResults    int64
	CompletedMax  string
	CompletedMin  string
	DueMax        string
	DueMin        string
	ShowCompleted bool
	ShowDeleted   bool
	ShowHidden    bool
	UpdatedMin    string
}

// GetTask 获取单个任务
// https://developers.google.com/workspace/tasks/reference/rest/v1/tasks/get
func (c *Client) GetTask(ctx context.Context, tasklistID, taskID string) (*Task, error) {
	u := fmt.Sprintf("%s/lists/%s/tasks/%s", c.baseURL, tasklistID, taskID)

	var result Task
	if err := c.doRequest(ctx, http.MethodGet, u, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateTask 创建任务
// https://developers.google.com/workspace/tasks/reference/rest/v1/tasks/insert
func (c *Client) CreateTask(ctx context.Context, tasklistID string, task *Task) (*Task, error) {
	u := fmt.Sprintf("%s/lists/%s/tasks", c.baseURL, tasklistID)

	var result Task
	if err := c.doRequest(ctx, http.MethodPost, u, task, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateTask 更新任务
// https://developers.google.com/workspace/tasks/reference/rest/v1/tasks/update
func (c *Client) UpdateTask(ctx context.Context, tasklistID string, task *Task) (*Task, error) {
	u := fmt.Sprintf("%s/lists/%s/tasks/%s", c.baseURL, tasklistID, task.ID)

	var result Task
	if err := c.doRequest(ctx, http.MethodPut, u, task, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteTask 删除任务
// https://developers.google.com/workspace/tasks/reference/rest/v1/tasks/delete
func (c *Client) DeleteTask(ctx context.Context, tasklistID, taskID string) error {
	u := fmt.Sprintf("%s/lists/%s/tasks/%s", c.baseURL, tasklistID, taskID)
	return c.doRequest(ctx, http.MethodDelete, u, nil, nil)
}

// ClearTasks 清除已完成任务
// https://developers.google.com/workspace/tasks/reference/rest/v1/tasks/clear
func (c *Client) ClearTasks(ctx context.Context, tasklistID string) error {
	u := fmt.Sprintf("%s/lists/%s/clear", c.baseURL, tasklistID)
	return c.doRequest(ctx, http.MethodPost, u, nil, nil)
}

// MoveTask 移动任务位置
// https://developers.google.com/workspace/tasks/reference/rest/v1/tasks/move
func (c *Client) MoveTask(ctx context.Context, tasklistID, taskID string, opts MoveTaskOptions) (*Task, error) {
	u := fmt.Sprintf("%s/lists/%s/tasks/%s/move", c.baseURL, tasklistID, taskID)

	params := url.Values{}
	if opts.Parent != "" {
		params.Set("parent", opts.Parent)
	}
	if opts.Previous != "" {
		params.Set("previous", opts.Previous)
	}
	if len(params) > 0 {
		u = u + "?" + params.Encode()
	}

	var result Task
	if err := c.doRequest(ctx, http.MethodPost, u, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// MoveTaskOptions 移动任务选项
type MoveTaskOptions struct {
	Parent   string
	Previous string
}

// doRequest 执行 HTTP 请求
func (c *Client) doRequest(ctx context.Context, method, url string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err != nil {
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		}
		apiErr.StatusCode = resp.StatusCode
		return &apiErr
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// APIError Google API 错误
type APIError struct {
	StatusCode int
	ErrorInfo  struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Google API error (status %d): %s", e.StatusCode, e.ErrorInfo.Message)
}

// ToModelTask 将 Google Task 转换为统一任务模型
func (t *Task) ToModelTask(listID, listName string) *model.Task {
	task := &model.Task{
		Source:      model.SourceGoogle,
		SourceRawID: t.ID,
		ID:          fmt.Sprintf("google-%s-%s", listID, t.ID),
		Title:       t.Title,
		Description: t.Notes,
		ListID:      listID,
		ListName:    listName,
	}

	// 状态转换
	switch t.Status {
	case "completed":
		task.Status = model.StatusCompleted
		if t.Completed != "" {
			if completed, err := time.Parse(time.RFC3339, t.Completed); err == nil {
				task.CompletedAt = &completed
			}
		}
	default:
		task.Status = model.StatusTodo
	}

	// 截止日期
	if t.Due != "" {
		if due, err := time.Parse(time.RFC3339, t.Due); err == nil {
			task.DueDate = &due
		}
	}

	// 更新时间
	if t.Updated != "" {
		if updated, err := time.Parse(time.RFC3339, t.Updated); err == nil {
			task.UpdatedAt = updated
		}
	}

	// 父任务
	if t.Parent != "" {
		parentID := fmt.Sprintf("google-%s-%s", listID, t.Parent)
		task.ParentID = &parentID
	}

	// 从备注中提取元数据
	if t.Notes != "" {
		cleanDesc, metadata, _ := model.ExtractMetadata(t.Notes)
		if metadata != nil {
			task.Description = cleanDesc
			task.Metadata = metadata
			metadata.ApplyToTask(task)
		}
	}

	return task
}

// FromModelTask 从统一任务模型创建 Google Task
func FromModelTask(task *model.Task) *Task {
	gtask := &Task{
		ID:     task.SourceRawID,
		Title:  task.Title,
		Status: "needsAction",
	}

	// 状态转换
	switch task.Status {
	case model.StatusCompleted:
		gtask.Status = "completed"
		if task.CompletedAt != nil {
			gtask.Completed = task.CompletedAt.Format(time.RFC3339)
		}
	}

	// 截止日期
	if task.DueDate != nil {
		gtask.Due = task.DueDate.Format(time.RFC3339)
	}

	// 父任务
	if task.ParentID != nil && *task.ParentID != "" {
		// 提取原始 Google Task ID
		gtask.Parent = task.SourceRawID
	}

	// 描述（包含元数据）
	description := task.Description
	if task.Metadata != nil || task.Quadrant != 0 || task.Priority != 0 {
		metadata := task.Metadata
		if metadata == nil {
			metadata = model.MetadataFromTask(task)
		}
		descWithMeta, err := model.EmbedMetadata(description, metadata)
		if err == nil {
			description = descWithMeta
		}
	}
	gtask.Notes = description

	return gtask
}
