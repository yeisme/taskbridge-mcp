// Package feishu provides Feishu (Lark) Task provider implementation
package feishu

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larktaskv2 "github.com/larksuite/oapi-sdk-go/v3/service/task/v2"
	"github.com/rs/zerolog/log"
)

const (
	// APIVersion 飞书 API 版本
	APIVersion = "v2"
	// DefaultPageSize 默认分页大小
	DefaultPageSize = 50
	// MaxPageSize 最大分页大小
	MaxPageSize = 100
)

// Client 飞书 API 客户端（基于官方 Go SDK）
type Client struct {
	mu              sync.RWMutex
	sdk             *lark.Client
	baseURL         string
	appID           string
	appSecret       string
	userAccessToken string
	httpClient      *http.Client
}

// NewClient 创建 API 客户端
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultAPIBaseURL
	}

	c := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	c.rebuildSDKLocked()
	return c
}

// SetHTTPClient 设置 HTTP 客户端（兼容旧接口）
func (c *Client) SetHTTPClient(httpClient *http.Client) {
	if httpClient == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.httpClient = httpClient
	c.rebuildSDKLocked()
}

// SetCredentials 设置应用凭据
func (c *Client) SetCredentials(appID, appSecret string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.appID = appID
	c.appSecret = appSecret
	c.rebuildSDKLocked()
}

// SetUserAccessToken 设置用户访问令牌
func (c *Client) SetUserAccessToken(accessToken string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.userAccessToken = strings.TrimSpace(accessToken)
}

func (c *Client) rebuildSDKLocked() {
	appID := c.appID
	appSecret := c.appSecret
	if appID == "" {
		appID = "placeholder_app_id"
	}
	if appSecret == "" {
		appSecret = "placeholder_app_secret"
	}

	opts := []lark.ClientOptionFunc{
		lark.WithLogLevel(larkcore.LogLevelError),
		lark.WithHttpClient(c.httpClient),
	}
	if c.baseURL != "" && c.baseURL != DefaultAPIBaseURL {
		opts = append(opts, lark.WithOpenBaseUrl(c.baseURL))
	}

	c.sdk = lark.NewClient(appID, appSecret, opts...)
}

func (c *Client) requestOptions() ([]larkcore.RequestOptionFunc, error) {
	c.mu.RLock()
	token := c.userAccessToken
	c.mu.RUnlock()

	if token == "" {
		return nil, fmt.Errorf("feishu user access token is empty")
	}

	return []larkcore.RequestOptionFunc{larkcore.WithUserAccessToken(token)}, nil
}

func parseMillis(value *string) int64 {
	if value == nil || *value == "" {
		return 0
	}
	v, err := strconv.ParseInt(*value, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func sdkTaskToLocal(task *larktaskv2.Task) Task {
	if task == nil {
		return Task{}
	}

	local := Task{}
	if task.Guid != nil {
		local.TaskID = *task.Guid
	}
	if task.Summary != nil {
		local.Title = *task.Summary
	}
	if task.Description != nil {
		local.Description = *task.Description
	}
	if task.CreatedAt != nil {
		local.CreatedTime = parseMillis(task.CreatedAt)
	}
	if task.UpdatedAt != nil {
		local.UpdatedTime = parseMillis(task.UpdatedAt)
	}
	if task.CompletedAt != nil {
		local.CompletedTime = parseMillis(task.CompletedAt)
	}
	if task.Due != nil {
		local.DueTime = parseMillis(task.Due.Timestamp)
	}
	if task.Start != nil {
		local.StartTime = parseMillis(task.Start.Timestamp)
	}

	if task.Status != nil {
		switch *task.Status {
		case "done":
			local.Status = StatusDone
		default:
			local.Status = StatusTodo
		}
	}

	if len(task.Tasklists) > 0 {
		local.TasklistIDs = make([]string, 0, len(task.Tasklists))
		for _, info := range task.Tasklists {
			if info != nil && info.TasklistGuid != nil {
				local.TasklistIDs = append(local.TasklistIDs, *info.TasklistGuid)
			}
		}
	}

	if len(task.Members) > 0 {
		local.CollaboratorIDs = make([]string, 0, len(task.Members))
		for _, m := range task.Members {
			if m != nil && m.Id != nil {
				local.CollaboratorIDs = append(local.CollaboratorIDs, *m.Id)
			}
		}
	}

	return local
}

func sdkTasklistToLocal(list *larktaskv2.Tasklist) TaskList {
	local := TaskList{}
	if list == nil {
		return local
	}
	if list.Guid != nil {
		local.TaskListID = *list.Guid
	}
	if list.Name != nil {
		local.Name = *list.Name
	}
	if list.CreatedAt != nil {
		local.CreatedTime = parseMillis(list.CreatedAt)
	}
	if list.UpdatedAt != nil {
		local.CompletedTime = parseMillis(list.UpdatedAt)
	}
	return local
}

func localTaskToInputTask(task *Task, listIDs []string) *larktaskv2.InputTask {
	builder := larktaskv2.NewInputTaskBuilder().
		Summary(task.Title).
		Description(task.Description)

	if task.DueTime > 0 {
		due := larktaskv2.NewDueBuilder().Timestamp(strconv.FormatInt(task.DueTime, 10)).Build()
		builder = builder.Due(due)
	}

	if task.StartTime > 0 {
		start := larktaskv2.NewStartBuilder().Timestamp(strconv.FormatInt(task.StartTime, 10)).Build()
		builder = builder.Start(start)
	}

	if len(listIDs) > 0 {
		tasklists := make([]*larktaskv2.TaskInTasklistInfo, 0, len(listIDs))
		for _, id := range listIDs {
			tasklists = append(tasklists, larktaskv2.NewTaskInTasklistInfoBuilder().TasklistGuid(id).Build())
		}
		builder = builder.Tasklists(tasklists)
	}

	// v2 通过 completed_at 表达完成状态
	if task.Status == StatusDone {
		ts := task.CompletedTime
		if ts == 0 {
			ts = time.Now().UnixMilli()
		}
		builder = builder.CompletedAt(strconv.FormatInt(ts, 10))
	}

	return builder.Build()
}

func localUpdateToInputTaskAndFields(req *UpdateTaskRequest) (*larktaskv2.InputTask, []string) {
	input := larktaskv2.NewInputTaskBuilder()
	fields := make([]string, 0, 6)

	if req.Title != "" {
		input = input.Summary(req.Title)
		fields = append(fields, "summary")
	}
	if req.Description != "" {
		input = input.Description(req.Description)
		fields = append(fields, "description")
	}
	if req.DueTime > 0 {
		due := larktaskv2.NewDueBuilder().Timestamp(strconv.FormatInt(req.DueTime, 10)).Build()
		input = input.Due(due)
		fields = append(fields, "due")
	}
	if req.StartTime > 0 {
		start := larktaskv2.NewStartBuilder().Timestamp(strconv.FormatInt(req.StartTime, 10)).Build()
		input = input.Start(start)
		fields = append(fields, "start")
	}

	// 同步完成状态到 completed_at
	if req.Status == StatusDone {
		ts := req.CompletedTime
		if ts == 0 {
			ts = time.Now().UnixMilli()
		}
		input = input.CompletedAt(strconv.FormatInt(ts, 10))
		fields = append(fields, "completed_at")
	} else if req.Status == StatusTodo {
		input = input.CompletedAt("0")
		fields = append(fields, "completed_at")
	}

	if len(fields) == 0 {
		fields = append(fields, "summary")
		input = input.Summary(req.Title)
	}

	return input.Build(), fields
}

// ================ 任务列表操作 ================

// ListTaskLists 获取所有任务列表
func (c *Client) ListTaskLists(ctx context.Context, pageToken string, pageSize int) (*TaskListResponse, error) {
	if pageSize <= 0 || pageSize > MaxPageSize {
		pageSize = DefaultPageSize
	}

	opts, err := c.requestOptions()
	if err != nil {
		return nil, err
	}

	reqBuilder := larktaskv2.NewListTasklistReqBuilder().
		PageSize(pageSize).
		UserIdType(larktaskv2.UserIdTypeOpenId)
	if pageToken != "" {
		reqBuilder = reqBuilder.PageToken(pageToken)
	}

	resp, err := c.sdk.Task.V2.Tasklist.List(ctx, reqBuilder.Build(), opts...)
	if err != nil {
		return nil, fmt.Errorf("list tasklists failed: %w", err)
	}
	if !resp.Success() {
		return nil, fmt.Errorf("list tasklists failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	out := &TaskListResponse{Code: 0, Msg: "ok"}
	if resp.Data == nil {
		return out, nil
	}

	for _, item := range resp.Data.Items {
		local := sdkTasklistToLocal(item)
		out.Data.Tasklists = append(out.Data.Tasklists, local)
	}
	if resp.Data.HasMore != nil {
		out.Data.HasMore = *resp.Data.HasMore
	}
	if resp.Data.PageToken != nil {
		out.Data.PageToken = *resp.Data.PageToken
	}
	return out, nil
}

// GetTaskList 获取单个任务列表
func (c *Client) GetTaskList(ctx context.Context, listID string) (*TaskList, error) {
	opts, err := c.requestOptions()
	if err != nil {
		return nil, err
	}

	resp, err := c.sdk.Task.V2.Tasklist.Get(ctx, larktaskv2.NewGetTasklistReqBuilder().
		TasklistGuid(listID).
		UserIdType(larktaskv2.UserIdTypeOpenId).
		Build(), opts...)
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, fmt.Errorf("get tasklist failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.Tasklist == nil {
		return nil, fmt.Errorf("tasklist not found")
	}

	local := sdkTasklistToLocal(resp.Data.Tasklist)
	return &local, nil
}

// CreateTaskList 创建任务列表
func (c *Client) CreateTaskList(ctx context.Context, req *CreateTaskListRequest) (*TaskList, error) {
	opts, err := c.requestOptions()
	if err != nil {
		return nil, err
	}

	input := larktaskv2.NewInputTasklistBuilder().Name(req.Name).Build()
	resp, err := c.sdk.Task.V2.Tasklist.Create(ctx, larktaskv2.NewCreateTasklistReqBuilder().
		UserIdType(larktaskv2.UserIdTypeOpenId).
		InputTasklist(input).
		Build(), opts...)
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, fmt.Errorf("create tasklist failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.Tasklist == nil {
		return nil, fmt.Errorf("empty tasklist in response")
	}

	local := sdkTasklistToLocal(resp.Data.Tasklist)
	return &local, nil
}

// UpdateTaskList 更新任务列表
func (c *Client) UpdateTaskList(ctx context.Context, listID string, req map[string]interface{}) (*TaskList, error) {
	name, _ := req["name"].(string)
	if strings.TrimSpace(name) == "" {
		return c.GetTaskList(ctx, listID)
	}

	opts, err := c.requestOptions()
	if err != nil {
		return nil, err
	}

	body := larktaskv2.NewPatchTasklistReqBodyBuilder().
		Tasklist(larktaskv2.NewInputTasklistBuilder().Name(name).Build()).
		UpdateFields([]string{"name"}).
		Build()

	resp, err := c.sdk.Task.V2.Tasklist.Patch(ctx, larktaskv2.NewPatchTasklistReqBuilder().
		TasklistGuid(listID).
		Body(body).
		Build(), opts...)
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, fmt.Errorf("update tasklist failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.Tasklist == nil {
		return nil, fmt.Errorf("empty tasklist in response")
	}

	local := sdkTasklistToLocal(resp.Data.Tasklist)
	return &local, nil
}

// DeleteTaskList 删除任务列表
func (c *Client) DeleteTaskList(ctx context.Context, listID string) error {
	opts, err := c.requestOptions()
	if err != nil {
		return err
	}

	resp, err := c.sdk.Task.V2.Tasklist.Delete(ctx, larktaskv2.NewDeleteTasklistReqBuilder().TasklistGuid(listID).Build(), opts...)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("delete tasklist failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// ================ 任务操作 ================

// ListTasks 获取任务列表中的所有任务
func (c *Client) ListTasks(ctx context.Context, listID string, opts *ListTasksOptions) ([]Task, string, error) {
	if opts == nil {
		opts = &ListTasksOptions{}
	}

	pageSize := opts.PageSize
	if pageSize <= 0 || pageSize > MaxPageSize {
		pageSize = DefaultPageSize
	}

	requestOptions, err := c.requestOptions()
	if err != nil {
		return nil, "", err
	}

	reqBuilder := larktaskv2.NewTasksTasklistReqBuilder().
		TasklistGuid(listID).
		PageSize(pageSize).
		UserIdType(larktaskv2.UserIdTypeOpenId)

	if opts.PageToken != "" {
		reqBuilder = reqBuilder.PageToken(opts.PageToken)
	}
	if opts.Completed != nil {
		reqBuilder = reqBuilder.Completed(*opts.Completed)
	}

	summaryResp, err := c.sdk.Task.V2.Tasklist.Tasks(ctx, reqBuilder.Build(), requestOptions...)
	if err != nil {
		return nil, "", err
	}
	if !summaryResp.Success() {
		return nil, "", fmt.Errorf("list tasks failed: code=%d msg=%s", summaryResp.Code, summaryResp.Msg)
	}
	if summaryResp.Data == nil {
		return []Task{}, "", nil
	}

	tasks := make([]Task, 0, len(summaryResp.Data.Items))
	for _, item := range summaryResp.Data.Items {
		if item == nil || item.Guid == nil {
			continue
		}
		task, getErr := c.GetTask(ctx, *item.Guid)
		if getErr != nil {
			log.Warn().Err(getErr).Str("task_guid", *item.Guid).Msg("failed to fetch task details")
			continue
		}
		if task == nil {
			continue
		}

		// 本地补充截止时间过滤
		if opts.StartDueTime > 0 && task.DueTime > 0 && task.DueTime < opts.StartDueTime {
			continue
		}
		if opts.EndDueTime > 0 && task.DueTime > 0 && task.DueTime > opts.EndDueTime {
			continue
		}

		tasks = append(tasks, *task)
	}

	nextToken := ""
	if summaryResp.Data.PageToken != nil {
		nextToken = *summaryResp.Data.PageToken
	}
	return tasks, nextToken, nil
}

// GetTask 获取单个任务
func (c *Client) GetTask(ctx context.Context, taskID string) (*Task, error) {
	opts, err := c.requestOptions()
	if err != nil {
		return nil, err
	}

	resp, err := c.sdk.Task.V2.Task.Get(ctx, larktaskv2.NewGetTaskReqBuilder().
		TaskGuid(taskID).
		UserIdType(larktaskv2.UserIdTypeOpenId).
		Build(), opts...)
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, fmt.Errorf("get task failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.Task == nil {
		return nil, fmt.Errorf("task not found")
	}

	local := sdkTaskToLocal(resp.Data.Task)
	return &local, nil
}

// CreateTask 创建任务
func (c *Client) CreateTask(ctx context.Context, req *CreateTaskRequest) (*Task, error) {
	opts, err := c.requestOptions()
	if err != nil {
		return nil, err
	}

	input := localTaskToInputTask(&req.Task, req.TasklistIDs)
	resp, err := c.sdk.Task.V2.Task.Create(ctx, larktaskv2.NewCreateTaskReqBuilder().
		UserIdType(larktaskv2.UserIdTypeOpenId).
		InputTask(input).
		Build(), opts...)
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, fmt.Errorf("create task failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.Task == nil {
		return nil, fmt.Errorf("empty task in response")
	}

	local := sdkTaskToLocal(resp.Data.Task)
	return &local, nil
}

// UpdateTask 更新任务
func (c *Client) UpdateTask(ctx context.Context, taskID string, req *UpdateTaskRequest) (*Task, error) {
	opts, err := c.requestOptions()
	if err != nil {
		return nil, err
	}

	input, updateFields := localUpdateToInputTaskAndFields(req)
	body := larktaskv2.NewPatchTaskReqBodyBuilder().
		Task(input).
		UpdateFields(updateFields).
		Build()

	resp, err := c.sdk.Task.V2.Task.Patch(ctx, larktaskv2.NewPatchTaskReqBuilder().
		TaskGuid(taskID).
		UserIdType(larktaskv2.UserIdTypeOpenId).
		Body(body).
		Build(), opts...)
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, fmt.Errorf("update task failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.Task == nil {
		return nil, fmt.Errorf("empty task in response")
	}

	local := sdkTaskToLocal(resp.Data.Task)
	return &local, nil
}

// DeleteTask 删除任务
func (c *Client) DeleteTask(ctx context.Context, taskID string) error {
	opts, err := c.requestOptions()
	if err != nil {
		return err
	}

	resp, err := c.sdk.Task.V2.Task.Delete(ctx, larktaskv2.NewDeleteTaskReqBuilder().TaskGuid(taskID).Build(), opts...)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("delete task failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// CompleteTask 完成任务
func (c *Client) CompleteTask(ctx context.Context, taskID string) (*Task, error) {
	update := &UpdateTaskRequest{Task: Task{Status: StatusDone, CompletedTime: time.Now().UnixMilli()}}
	return c.UpdateTask(ctx, taskID, update)
}

// UncompleteTask 取消完成任务
func (c *Client) UncompleteTask(ctx context.Context, taskID string) (*Task, error) {
	update := &UpdateTaskRequest{Task: Task{Status: StatusTodo, CompletedTime: 0}}
	return c.UpdateTask(ctx, taskID, update)
}

// ================ 子任务操作 ================

// ListSubtasks 获取任务的子任务
func (c *Client) ListSubtasks(ctx context.Context, taskID string) ([]Subtask, error) {
	return []Subtask{}, nil
}

// CreateSubtask 创建子任务
func (c *Client) CreateSubtask(ctx context.Context, taskID string, title string) (*Subtask, error) {
	return nil, fmt.Errorf("create subtask is not implemented with current SDK mapping")
}

// CompleteSubtask 完成子任务
func (c *Client) CompleteSubtask(ctx context.Context, taskID, subtaskID string) error {
	return fmt.Errorf("complete subtask is not implemented with current SDK mapping")
}

// DeleteSubtask 删除子任务
func (c *Client) DeleteSubtask(ctx context.Context, taskID, subtaskID string) error {
	return fmt.Errorf("delete subtask is not implemented with current SDK mapping")
}

// ================ 增量同步 ================

// GetChanges 获取增量变更
func (c *Client) GetChanges(ctx context.Context, deltaToken string) (*SyncChangesResponse, error) {
	return nil, fmt.Errorf("delta token sync is not implemented in SDK v2 adapter")
}

// ================ 批量操作 ================

// BatchCreateTasks 批量创建任务
func (c *Client) BatchCreateTasks(ctx context.Context, listID string, tasks []*CreateTaskRequest) ([]Task, error) {
	const maxBatchSize = 20
	results := make([]Task, 0, len(tasks))

	for i := 0; i < len(tasks); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(tasks) {
			end = len(tasks)
		}
		for _, req := range tasks[i:end] {
			req.TasklistIDs = []string{listID}
			createdTask, err := c.CreateTask(ctx, req)
			if err != nil {
				log.Warn().Err(err).Str("title", req.Title).Msg("failed to create task in batch")
				continue
			}
			results = append(results, *createdTask)
		}
	}

	return results, nil
}

// BatchUpdateTasks 批量更新任务
func (c *Client) BatchUpdateTasks(ctx context.Context, tasks []*UpdateTaskRequest) ([]Task, error) {
	const maxBatchSize = 20
	results := make([]Task, 0, len(tasks))

	for i := 0; i < len(tasks); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(tasks) {
			end = len(tasks)
		}
		for _, req := range tasks[i:end] {
			updatedTask, err := c.UpdateTask(ctx, req.TaskID, req)
			if err != nil {
				log.Warn().Err(err).Str("task_id", req.TaskID).Msg("failed to update task in batch")
				continue
			}
			results = append(results, *updatedTask)
		}
	}

	return results, nil
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
// 飞书 API 本身不支持全文搜索，这里通过获取所有任务后本地过滤实现
func (c *Client) SearchTasks(ctx context.Context, query string) ([]Task, error) {
	lists, err := c.GetAllTaskLists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get task lists: %w", err)
	}

	results := make([]Task, 0)
	for _, list := range lists {
		tasks, err := c.GetAllTasks(ctx, list.TaskListID)
		if err != nil {
			log.Warn().Err(err).Str("list_id", list.TaskListID).Msg("failed to get tasks from list")
			continue
		}
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
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
