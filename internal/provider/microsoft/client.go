// Package microsoft provides Microsoft To Do provider implementation
package microsoft

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	// DefaultBaseURL Microsoft Graph API 基础 URL
	DefaultBaseURL = "https://graph.microsoft.com/v1.0"
	// BetaBaseURL Beta API 基础 URL（某些功能需要）
	BetaBaseURL = "https://graph.microsoft.com/beta"
)

// Client Microsoft Graph API 客户端
type Client struct {
	httpClient *http.Client
	baseURL    string
	betaURL    string
	authToken  string // 直接存储 token
}

// NewClient 创建 API 客户端
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
		betaURL: BetaBaseURL,
	}
}

// SetHTTPClient 设置 HTTP 客户端（用于 OAuth2）
func (c *Client) SetHTTPClient(httpClient *http.Client) {
	c.httpClient = httpClient
}

// SetAuthToken 设置认证 token
func (c *Client) SetAuthToken(token string) {
	c.authToken = token
}

// ================ 任务列表操作 ================

// ListTodoLists 获取所有任务列表
func (c *Client) ListTodoLists(ctx context.Context) ([]TodoTaskList, error) {
	var resp TodoTaskListResponse
	if err := c.get(ctx, "/me/todo/lists", &resp); err != nil {
		return nil, err
	}
	return resp.Value, nil
}

// GetTodoList 获取单个任务列表
func (c *Client) GetTodoList(ctx context.Context, listID string) (*TodoTaskList, error) {
	var list TodoTaskList
	if err := c.get(ctx, fmt.Sprintf("/me/todo/lists/%s", listID), &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// CreateTodoList 创建任务列表
func (c *Client) CreateTodoList(ctx context.Context, displayName string) (*TodoTaskList, error) {
	body := map[string]string{
		"displayName": displayName,
	}
	var list TodoTaskList
	if err := c.post(ctx, "/me/todo/lists", body, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// UpdateTodoList 更新任务列表
func (c *Client) UpdateTodoList(ctx context.Context, listID string, displayName string) (*TodoTaskList, error) {
	body := map[string]string{
		"displayName": displayName,
	}
	var list TodoTaskList
	if err := c.patch(ctx, fmt.Sprintf("/me/todo/lists/%s", listID), body, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// DeleteTodoList 删除任务列表
func (c *Client) DeleteTodoList(ctx context.Context, listID string) error {
	return c.delete(ctx, fmt.Sprintf("/me/todo/lists/%s", listID))
}

// ================ 任务操作 ================

// ListTasks 获取任务列表中的所有任务
func (c *Client) ListTasks(ctx context.Context, listID string, opts *ListOptions) ([]TodoTask, error) {
	path := fmt.Sprintf("/me/todo/lists/%s/tasks", listID)

	// 构建查询参数
	if opts != nil {
		params := url.Values{}
		if opts.Filter != "" {
			params.Set("$filter", opts.Filter)
		}
		if opts.OrderBy != "" {
			params.Set("$orderby", opts.OrderBy)
		}
		if opts.Top > 0 {
			params.Set("$top", fmt.Sprintf("%d", opts.Top))
		}
		if opts.Skip > 0 {
			params.Set("$skip", fmt.Sprintf("%d", opts.Skip))
		}
		if len(params) > 0 {
			path = path + "?" + params.Encode()
		}
	}

	// 自动处理分页，确保拉取完整任务集合。
	var allTasks []TodoTask
	nextURL := path
	for nextURL != "" {
		var resp TodoTaskResponse
		if err := c.get(ctx, nextURL, &resp); err != nil {
			return nil, err
		}
		allTasks = append(allTasks, resp.Value...)
		nextURL = resp.OdataNextLink
	}

	return allTasks, nil
}

// GetTask 获取单个任务
func (c *Client) GetTask(ctx context.Context, listID, taskID string) (*TodoTask, error) {
	var task TodoTask
	if err := c.get(ctx, fmt.Sprintf("/me/todo/lists/%s/tasks/%s", listID, taskID), &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// CreateTask 创建任务
func (c *Client) CreateTask(ctx context.Context, listID string, task *TodoTask) (*TodoTask, error) {
	var result TodoTask
	if err := c.post(ctx, fmt.Sprintf("/me/todo/lists/%s/tasks", listID), task, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateTask 更新任务
func (c *Client) UpdateTask(ctx context.Context, listID string, task *TodoTask) (*TodoTask, error) {
	var result TodoTask
	if err := c.patch(ctx, fmt.Sprintf("/me/todo/lists/%s/tasks/%s", listID, task.ID), task, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteTask 删除任务
func (c *Client) DeleteTask(ctx context.Context, listID, taskID string) error {
	return c.delete(ctx, fmt.Sprintf("/me/todo/lists/%s/tasks/%s", listID, taskID))
}

// CompleteTask 完成任务
func (c *Client) CompleteTask(ctx context.Context, listID, taskID string) (*TodoTask, error) {
	now := time.Now()
	task := &TodoTask{
		Status:            StatusCompleted,
		CompletedDateTime: ToDateTimeTimeZone(&now, ""),
	}
	return c.UpdateTask(ctx, listID, task)
}

// ================ 增量同步 ================

// GetDelta 获取增量变更
func (c *Client) GetDelta(ctx context.Context, listID, deltaToken string) (*DeltaResponse, error) {
	path := fmt.Sprintf("/me/todo/lists/%s/tasks/delta", listID)
	if deltaToken != "" {
		path = fmt.Sprintf("%s?$deltatoken=%s", path, deltaToken)
	}

	var resp DeltaResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ================ 检查项（子任务）操作 ================

// ListChecklistItems 获取任务的检查项
func (c *Client) ListChecklistItems(ctx context.Context, listID, taskID string) ([]ChecklistItem, error) {
	var resp struct {
		Value []ChecklistItem `json:"value"`
	}
	if err := c.get(ctx, fmt.Sprintf("/me/todo/lists/%s/tasks/%s/checklistItems", listID, taskID), &resp); err != nil {
		return nil, err
	}
	return resp.Value, nil
}

// CreateChecklistItem 创建检查项
func (c *Client) CreateChecklistItem(ctx context.Context, listID, taskID, displayName string, isChecked bool) (*ChecklistItem, error) {
	body := map[string]interface{}{
		"displayName": displayName,
		"isChecked":   isChecked,
	}
	var result ChecklistItem
	if err := c.post(ctx, fmt.Sprintf("/me/todo/lists/%s/tasks/%s/checklistItems", listID, taskID), body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateChecklistItem 更新检查项
func (c *Client) UpdateChecklistItem(ctx context.Context, listID, taskID, itemID string, isChecked bool) (*ChecklistItem, error) {
	body := map[string]interface{}{
		"isChecked": isChecked,
	}
	var result ChecklistItem
	if err := c.patch(ctx, fmt.Sprintf("/me/todo/lists/%s/tasks/%s/checklistItems/%s", listID, taskID, itemID), body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteChecklistItem 删除检查项
func (c *Client) DeleteChecklistItem(ctx context.Context, listID, taskID, itemID string) error {
	return c.delete(ctx, fmt.Sprintf("/me/todo/lists/%s/tasks/%s/checklistItems/%s", listID, taskID, itemID))
}

// ================ 关联资源操作 ================

// ListLinkedResources 获取任务的关联资源
func (c *Client) ListLinkedResources(ctx context.Context, listID, taskID string) ([]LinkedResource, error) {
	var resp struct {
		Value []LinkedResource `json:"value"`
	}
	if err := c.get(ctx, fmt.Sprintf("/me/todo/lists/%s/tasks/%s/linkedResources", listID, taskID), &resp); err != nil {
		return nil, err
	}
	return resp.Value, nil
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
func (c *Client) delete(ctx context.Context, path string) error {
	return c.doRequest(ctx, http.MethodDelete, path, nil, nil)
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

	// 构建完整 URL（支持 @odata.nextLink 返回的绝对地址）
	fullURL := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		fullURL = c.baseURL + path
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// 直接设置 Authorization header
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	log.Debug().
		Str("method", method).
		Str("url", fullURL).
		Str("auth_header_preview", func() string {
			h := req.Header.Get("Authorization")
			if len(h) > 30 {
				return h[:30] + "..."
			}
			return h
		}()).
		Msg("Microsoft Graph API request")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close response body")
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	log.Debug().
		Int("status", resp.StatusCode).
		Str("url", fullURL).
		Msg("Microsoft Graph API response")

	// 检查错误状态码
	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Code != "" {
			return fmt.Errorf("API error: %s - %s", errResp.Error.Code, errResp.Error.Message)
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

// ListOptions 列表查询选项
type ListOptions struct {
	Filter  string
	OrderBy string
	Top     int
	Skip    int
}

// PagedResult 分页结果
type PagedResult struct {
	NextLink string
	HasMore  bool
}

// GetAllTasks 获取所有任务（自动处理分页）
func (c *Client) GetAllTasks(ctx context.Context, listID string) ([]TodoTask, error) {
	return c.ListTasks(ctx, listID, nil)
}

// ================ 批量操作 ================

// BatchRequest 批量请求
type BatchRequest struct {
	Requests []BatchRequestItem `json:"requests"`
}

// BatchRequestItem 批量请求项
type BatchRequestItem struct {
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    interface{}       `json:"body,omitempty"`
}

// BatchResponse 批量响应
type BatchResponse struct {
	Responses []BatchResponseItem `json:"responses"`
}

// BatchResponseItem 批量响应项
type BatchResponseItem struct {
	ID     string          `json:"id"`
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

// ExecuteBatch 执行批量请求
func (c *Client) ExecuteBatch(ctx context.Context, batch *BatchRequest) (*BatchResponse, error) {
	var result BatchResponse
	if err := c.post(ctx, "/$batch", batch, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// BatchCreateTasks 批量创建任务
func (c *Client) BatchCreateTasks(ctx context.Context, listID string, tasks []*TodoTask) ([]TodoTask, error) {
	const maxBatchSize = 20 // Microsoft Graph API 批量限制

	var results []TodoTask

	for i := 0; i < len(tasks); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(tasks) {
			end = len(tasks)
		}

		batch := &BatchRequest{
			Requests: make([]BatchRequestItem, 0, end-i),
		}

		for j, task := range tasks[i:end] {
			batch.Requests = append(batch.Requests, BatchRequestItem{
				ID:      fmt.Sprintf("%d", i+j),
				Method:  "POST",
				URL:     fmt.Sprintf("/me/todo/lists/%s/tasks", listID),
				Headers: map[string]string{"Content-Type": "application/json"},
				Body:    task,
			})
		}

		resp, err := c.ExecuteBatch(ctx, batch)
		if err != nil {
			return nil, err
		}

		for _, item := range resp.Responses {
			if item.Status >= 400 {
				log.Warn().Str("id", item.ID).Int("status", item.Status).Msg("Batch request failed")
				continue
			}

			var task TodoTask
			if err := json.Unmarshal(item.Body, &task); err != nil {
				log.Warn().Err(err).Str("id", item.ID).Msg("Failed to parse batch response")
				continue
			}
			results = append(results, task)
		}
	}

	return results, nil
}
