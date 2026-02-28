package todoist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client Todoist REST API 客户端。
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiToken   string
}

// NewClient 创建客户端。
func NewClient(apiToken string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    defaultBaseURL,
		apiToken:   apiToken,
	}
}

// SetAPIToken 设置 API token。
func (c *Client) SetAPIToken(token string) {
	c.apiToken = token
}

// APIToken 返回 API token。
func (c *Client) APIToken() string {
	return c.apiToken
}

// ListProjects 列出项目。
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var all []Project
	cursor := ""
	for {
		path := "/projects"
		v := url.Values{}
		if cursor != "" {
			v.Set("cursor", cursor)
		}
		if len(v) > 0 {
			path += "?" + v.Encode()
		}

		var resp pagedProjectsResponse
		if err := c.doRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
			return nil, err
		}

		all = append(all, resp.Results...)
		if resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}
	return all, nil
}

// CreateProject 创建项目。
func (c *Client) CreateProject(ctx context.Context, name string) (*Project, error) {
	body := map[string]string{"name": name}
	var project Project
	if err := c.doRequest(ctx, http.MethodPost, "/projects", body, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// DeleteProject 删除项目。
func (c *Client) DeleteProject(ctx context.Context, projectID string) error {
	return c.doRequest(ctx, http.MethodDelete, "/projects/"+projectID, nil, nil)
}

// ListSections 列出板块。
func (c *Client) ListSections(ctx context.Context, projectID string) ([]Section, error) {
	var all []Section
	cursor := ""
	for {
		path := "/sections"
		v := url.Values{}
		if projectID != "" {
			v.Set("project_id", projectID)
		}
		if cursor != "" {
			v.Set("cursor", cursor)
		}
		if len(v) > 0 {
			path += "?" + v.Encode()
		}

		var resp pagedSectionsResponse
		if err := c.doRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
			return nil, err
		}

		all = append(all, resp.Results...)
		if resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}
	return all, nil
}

// ListTasks 列出任务。
func (c *Client) ListTasks(ctx context.Context, projectID string) ([]Task, error) {
	var all []Task
	cursor := ""
	for {
		path := "/tasks"
		v := url.Values{}
		if projectID != "" {
			v.Set("project_id", projectID)
		}
		if cursor != "" {
			v.Set("cursor", cursor)
		}
		if len(v) > 0 {
			path += "?" + v.Encode()
		}

		var resp pagedTasksResponse
		if err := c.doRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
			return nil, err
		}

		all = append(all, resp.Results...)
		if resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}
	return all, nil
}

// GetTask 获取任务。
func (c *Client) GetTask(ctx context.Context, taskID string) (*Task, error) {
	var task Task
	if err := c.doRequest(ctx, http.MethodGet, "/tasks/"+taskID, nil, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// CreateTask 创建任务。
func (c *Client) CreateTask(ctx context.Context, req *CreateTaskRequest) (*Task, error) {
	var task Task
	if err := c.doRequest(ctx, http.MethodPost, "/tasks", req, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// UpdateTask 更新任务。
func (c *Client) UpdateTask(ctx context.Context, taskID string, req *UpdateTaskRequest) (*Task, error) {
	if err := c.doRequest(ctx, http.MethodPost, "/tasks/"+taskID, req, nil); err != nil {
		return nil, err
	}
	return c.GetTask(ctx, taskID)
}

// DeleteTask 删除任务。
func (c *Client) DeleteTask(ctx context.Context, taskID string) error {
	return c.doRequest(ctx, http.MethodDelete, "/tasks/"+taskID, nil, nil)
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	if c.apiToken == "" {
		return fmt.Errorf("todoist api token is empty")
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		msg := string(respBytes)
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("todoist api error: status=%d body=%s", resp.StatusCode, msg)
	}

	if out != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, out); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}
	return nil
}
