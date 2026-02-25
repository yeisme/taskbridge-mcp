// Package feishu provides Feishu (Lark) Task provider implementation
package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	// APIVersion 飞书 API 版本
	APIVersion = "v1"
	// TaskAPIPath 任务 API 路径
	TaskAPIPath = "/task/v1"
	// DefaultPageSize 默认分页大小
	DefaultPageSize = 50
	// MaxPageSize 最大分页大小
	MaxPageSize = 100
)

// Client 飞书 API 客户端
type Client struct {
	httpClient *http.Client
	baseURL    string
	appID      string
	appSecret  string
}

// NewClient 创建 API 客户端
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultAPIBaseURL
	}
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
	}
}

// SetHTTPClient 设置 HTTP 客户端（用于 OAuth2）
func (c *Client) SetHTTPClient(httpClient *http.Client) {
	c.httpClient = httpClient
}

// SetCredentials 设置应用凭据
func (c *Client) SetCredentials(appID, appSecret string) {
	c.appID = appID
	c.appSecret = appSecret
}

// ================ 任务列表操作 ================

// ListTaskLists 获取所有任务列表
// https://open.feishu.cn/document/server-docs/task/v1/tasklist/query
func (c *Client) ListTaskLists(ctx context.Context, pageToken string, pageSize int) (*TaskListResponse, error) {
	path := fmt.Sprintf("%s/tasklists", TaskAPIPath)

	// 构建查询参数
	params := url.Values{}
	if pageSize > 0 {
		if pageSize > MaxPageSize {
			pageSize = MaxPageSize
		}
		params.Set("page_size", fmt.Sprintf("%d", pageSize))
	}
	if pageToken != "" {
		params.Set("page_token", pageToken)
	}

	if len(params) > 0 {
		path = path + "?" + params.Encode()
	}

	var resp TaskListResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("API error: %d - %s", resp.Code, resp.Msg)
	}

	return &resp, nil
}

// GetTaskList 获取单个任务列表
// https://open.feishu.cn/document/server-docs/task/v1/tasklist/get
func (c *Client) GetTaskList(ctx context.Context, listID string) (*TaskList, error) {
	var resp TaskListDetailResponse
	if err := c.get(ctx, fmt.Sprintf("%s/tasklists/%s", TaskAPIPath, listID), &resp); err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("API error: %d - %s", resp.Code, resp.Msg)
	}

	return &resp.Data.Tasklist, nil
}

// CreateTaskList 创建任务列表
// https://open.feishu.cn/document/server-docs/task/v1/tasklist/create
func (c *Client) CreateTaskList(ctx context.Context, req *CreateTaskListRequest) (*TaskList, error) {
	var resp CreateTaskListResponse
	if err := c.post(ctx, fmt.Sprintf("%s/tasklists", TaskAPIPath), req, &resp); err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("API error: %d - %s", resp.Code, resp.Msg)
	}

	return &resp.Data.Tasklist, nil
}

// UpdateTaskList 更新任务列表
// https://open.feishu.cn/document/server-docs/task/v1/tasklist/patch
func (c *Client) UpdateTaskList(ctx context.Context, listID string, req map[string]interface{}) (*TaskList, error) {
	var resp TaskListDetailResponse
	if err := c.patch(ctx, fmt.Sprintf("%s/tasklists/%s", TaskAPIPath, listID), req, &resp); err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("API error: %d - %s", resp.Code, resp.Msg)
	}

	return &resp.Data.Tasklist, nil
}

// DeleteTaskList 删除任务列表
// https://open.feishu.cn/document/server-docs/task/v1/tasklist/delete
func (c *Client) DeleteTaskList(ctx context.Context, listID string) error {
	var resp APIResponse
	if err := c.delete(ctx, fmt.Sprintf("%s/tasklists/%s", TaskAPIPath, listID), &resp); err != nil {
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("API error: %d - %s", resp.Code, resp.Msg)
	}

	return nil
}

// ================ 任务操作 ================

// ListTasks 获取任务列表中的所有任务
// https://open.feishu.cn/document/server-docs/task/v1/task/query
func (c *Client) ListTasks(ctx context.Context, listID string, opts *ListTasksOptions) ([]Task, string, error) {
	path := fmt.Sprintf("%s/tasklists/%s/tasks", TaskAPIPath, listID)

	// 构建查询参数
	params := url.Values{}
	if opts != nil {
		if opts.PageSize > 0 {
			if opts.PageSize > MaxPageSize {
				opts.PageSize = MaxPageSize
			}
			params.Set("page_size", fmt.Sprintf("%d", opts.PageSize))
		}
		if opts.PageToken != "" {
			params.Set("page_token", opts.PageToken)
		}
		if opts.Completed != nil {
			if *opts.Completed {
				params.Set("completed", "true")
			} else {
				params.Set("completed", "false")
			}
		}
		if opts.StartDueTime > 0 {
			params.Set("start_due_time", fmt.Sprintf("%d", opts.StartDueTime))
		}
		if opts.EndDueTime > 0 {
			params.Set("end_due_time", fmt.Sprintf("%d", opts.EndDueTime))
		}
	}

	if len(params) > 0 {
		path = path + "?" + params.Encode()
	}

	var resp TasksResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, "", err
	}

	if resp.Code != 0 {
		return nil, "", fmt.Errorf("API error: %d - %s", resp.Code, resp.Msg)
	}

	return resp.Data.Tasks, resp.Data.PageToken, nil
}

// GetTask 获取单个任务
// https://open.feishu.cn/document/server-docs/task/v1/task/get
func (c *Client) GetTask(ctx context.Context, taskID string) (*Task, error) {
	var resp TaskResponse
	if err := c.get(ctx, fmt.Sprintf("%s/tasks/%s", TaskAPIPath, taskID), &resp); err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("API error: %d - %s", resp.Code, resp.Msg)
	}

	return &resp.Data.Task, nil
}

// CreateTask 创建任务
// https://open.feishu.cn/document/server-docs/task/v1/task/create
func (c *Client) CreateTask(ctx context.Context, req *CreateTaskRequest) (*Task, error) {
	var resp CreateTaskResponse
	if err := c.post(ctx, fmt.Sprintf("%s/tasks", TaskAPIPath), req, &resp); err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("API error: %d - %s", resp.Code, resp.Msg)
	}

	return &resp.Data.Task, nil
}

// UpdateTask 更新任务
// https://open.feishu.cn/document/server-docs/task/v1/task/patch
func (c *Client) UpdateTask(ctx context.Context, taskID string, req *UpdateTaskRequest) (*Task, error) {
	var resp TaskResponse
	if err := c.patch(ctx, fmt.Sprintf("%s/tasks/%s", TaskAPIPath, taskID), req, &resp); err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("API error: %d - %s", resp.Code, resp.Msg)
	}

	return &resp.Data.Task, nil
}

// DeleteTask 删除任务
// https://open.feishu.cn/document/server-docs/task/v1/task/delete
func (c *Client) DeleteTask(ctx context.Context, taskID string) error {
	var resp APIResponse
	if err := c.delete(ctx, fmt.Sprintf("%s/tasks/%s", TaskAPIPath, taskID), &resp); err != nil {
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("API error: %d - %s", resp.Code, resp.Msg)
	}

	return nil
}

// CompleteTask 完成任务
func (c *Client) CompleteTask(ctx context.Context, taskID string) (*Task, error) {
	req := &UpdateTaskRequest{
		Task: Task{
			Status:        StatusDone,
			CompletedTime: time.Now().UnixMilli(),
		},
		UpdateFields: []string{"status", "completed_time"},
	}
	return c.UpdateTask(ctx, taskID, req)
}

// UncompleteTask 取消完成任务
func (c *Client) UncompleteTask(ctx context.Context, taskID string) (*Task, error) {
	req := &UpdateTaskRequest{
		Task: Task{
			Status: StatusTodo,
		},
		UpdateFields: []string{"status"},
	}
	return c.UpdateTask(ctx, taskID, req)
}

// ================ 子任务操作 ================

// ListSubtasks 获取任务的子任务
// https://open.feishu.cn/document/server-docs/task/v1/subtask/list
func (c *Client) ListSubtasks(ctx context.Context, taskID string) ([]Subtask, error) {
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Subtasks []Subtask `json:"subtasks"`
		} `json:"data"`
	}

	if err := c.get(ctx, fmt.Sprintf("%s/tasks/%s/subtasks", TaskAPIPath, taskID), &resp); err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("API error: %d - %s", resp.Code, resp.Msg)
	}

	return resp.Data.Subtasks, nil
}

// CreateSubtask 创建子任务
func (c *Client) CreateSubtask(ctx context.Context, taskID string, title string) (*Subtask, error) {
	req := map[string]string{
		"title": title,
	}

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Subtask Subtask `json:"subtask"`
		} `json:"data"`
	}

	if err := c.post(ctx, fmt.Sprintf("%s/tasks/%s/subtasks", TaskAPIPath, taskID), req, &resp); err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("API error: %d - %s", resp.Code, resp.Msg)
	}

	return &resp.Data.Subtask, nil
}

// CompleteSubtask 完成子任务
func (c *Client) CompleteSubtask(ctx context.Context, taskID, subtaskID string) error {
	req := map[string]interface{}{
		"is_completed": true,
	}

	var resp APIResponse
	if err := c.patch(ctx, fmt.Sprintf("%s/tasks/%s/subtasks/%s", TaskAPIPath, taskID, subtaskID), req, &resp); err != nil {
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("API error: %d - %s", resp.Code, resp.Msg)
	}

	return nil
}

// DeleteSubtask 删除子任务
func (c *Client) DeleteSubtask(ctx context.Context, taskID, subtaskID string) error {
	var resp APIResponse
	if err := c.delete(ctx, fmt.Sprintf("%s/tasks/%s/subtasks/%s", TaskAPIPath, taskID, subtaskID), &resp); err != nil {
		return err
	}

	if resp.Code != 0 {
		return fmt.Errorf("API error: %d - %s", resp.Code, resp.Msg)
	}

	return nil
}

// ================ 增量同步 ================

// GetChanges 获取增量变更
// https://open.feishu.cn/document/server-docs/task/v1/task-changes/get
func (c *Client) GetChanges(ctx context.Context, deltaToken string) (*SyncChangesResponse, error) {
	path := fmt.Sprintf("%s/task_changes", TaskAPIPath)

	params := url.Values{}
	if deltaToken != "" {
		params.Set("delta_token", deltaToken)
	}

	if len(params) > 0 {
		path = path + "?" + params.Encode()
	}

	var resp SyncChangesResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("API error: %d - %s", resp.Code, resp.Msg)
	}

	return &resp, nil
}

// ================ 批量操作 ================

// BatchCreateTasks 批量创建任务
func (c *Client) BatchCreateTasks(ctx context.Context, listID string, tasks []*CreateTaskRequest) ([]Task, error) {
	const maxBatchSize = 20 // 批量限制

	var results []Task

	for i := 0; i < len(tasks); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(tasks) {
			end = len(tasks)
		}

		for _, task := range tasks[i:end] {
			task.TasklistIDs = []string{listID}
			createdTask, err := c.CreateTask(ctx, task)
			if err != nil {
				log.Warn().Err(err).Str("title", task.Title).Msg("Failed to create task in batch")
				continue
			}
			results = append(results, *createdTask)
		}
	}

	return results, nil
}

// BatchUpdateTasks 批量更新任务
func (c *Client) BatchUpdateTasks(ctx context.Context, tasks []*UpdateTaskRequest) ([]Task, error) {
	const maxBatchSize = 20 // 批量限制

	var results []Task

	for i := 0; i < len(tasks); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(tasks) {
			end = len(tasks)
		}

		for _, task := range tasks[i:end] {
			updatedTask, err := c.UpdateTask(ctx, task.TaskID, task)
			if err != nil {
				log.Warn().Err(err).Str("task_id", task.TaskID).Msg("Failed to update task in batch")
				continue
			}
			results = append(results, *updatedTask)
		}
	}

	return results, nil
}

// ================ HTTP 请求方法 ================

// get 发送 GET 请求
func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	return c.doRequest(ctx, http.MethodGet, path, nil, result)
}

// post 发送 POST 请求
func (c *Client) post(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPost, path, body, result)
}

// patch 发送 PATCH 请求
func (c *Client) patch(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, http.MethodPatch, path, body, result)
}

// delete 发送 DELETE 请求
func (c *Client) delete(ctx context.Context, path string, result interface{}) error {
	return c.doRequest(ctx, http.MethodDelete, path, nil, result)
}

// doRequest 执行 HTTP 请求
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	// 构建完整 URL
	fullURL := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	log.Debug().
		Str("method", method).
		Str("url", fullURL).
		Msg("Feishu API request")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	log.Debug().
		Int("status", resp.StatusCode).
		Str("url", fullURL).
		Msg("Feishu API response")

	// 检查错误状态码
	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Msg != "" {
			return fmt.Errorf("API error: %d - %s", errResp.Code, errResp.Msg)
		}
		return fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// ================ 分页处理 ================

// ListTasksOptions 任务列表查询选项
type ListTasksOptions struct {
	PageSize     int
	PageToken    string
	Completed    *bool
	StartDueTime int64 // 毫秒时间戳
	EndDueTime   int64 // 毫秒时间戳
}

// GetAllTaskLists 获取所有任务列表（自动处理分页）
func (c *Client) GetAllTaskLists(ctx context.Context) ([]TaskList, error) {
	var allLists []TaskList
	pageToken := ""

	for {
		resp, err := c.ListTaskLists(ctx, pageToken, DefaultPageSize)
		if err != nil {
			return nil, err
		}

		allLists = append(allLists, resp.Data.Tasklists...)

		if !resp.Data.HasMore || resp.Data.PageToken == "" {
			break
		}

		pageToken = resp.Data.PageToken
	}

	return allLists, nil
}

// GetAllTasks 获取所有任务（自动处理分页）
func (c *Client) GetAllTasks(ctx context.Context, listID string) ([]Task, error) {
	var allTasks []Task
	pageToken := ""

	for {
		tasks, nextToken, err := c.ListTasks(ctx, listID, &ListTasksOptions{
			PageSize:  DefaultPageSize,
			PageToken: pageToken,
		})
		if err != nil {
			return nil, err
		}

		allTasks = append(allTasks, tasks...)

		if nextToken == "" {
			break
		}

		pageToken = nextToken
	}

	return allTasks, nil
}

// ================ 搜索功能 ================

// SearchTasks 搜索任务
// 飞书 API 本身不支持搜索，这里通过获取所有任务后本地过滤实现
func (c *Client) SearchTasks(ctx context.Context, query string) ([]Task, error) {
	// 获取所有任务列表
	lists, err := c.GetAllTaskLists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get task lists: %w", err)
	}

	var results []Task

	// 遍历所有任务列表
	for _, list := range lists {
		tasks, err := c.GetAllTasks(ctx, list.TaskListID)
		if err != nil {
			log.Warn().Err(err).Str("list_id", list.TaskListID).Msg("Failed to get tasks from list")
			continue
		}

		// 本地过滤
		for _, task := range tasks {
			if containsIgnoreCase(task.Title, query) || containsIgnoreCase(task.Description, query) {
				results = append(results, task)
			}
		}
	}

	return results, nil
}

// containsIgnoreCase 检查字符串是否包含子串（忽略大小写）
func containsIgnoreCase(s, substr string) bool {
	sLower := make([]byte, len(s))
	substrLower := make([]byte, len(substr))

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		sLower[i] = c
	}

	for i := 0; i < len(substr); i++ {
		c := substr[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		substrLower[i] = c
	}

	return contains(string(sLower), string(substrLower))
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
