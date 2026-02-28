package ticktick

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultV2BaseURL    = "https://api.ticktick.com/api/v2"
	defaultAuthBaseURL  = "https://api.ticktick.com/api/v2"
	didaV2BaseURL       = "https://api.dida365.com/api/v2"
	didaAuthBaseURL     = "https://api.dida365.com/api/v2"
	defaultOpenBaseURL  = "https://api.ticktick.com/open/v1"
	didaOpenBaseURL     = "https://api.dida365.com/open/v1"
	defaultRequestAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) taskbridge/1.0"
	openInboxProjectID  = "inbox"
)

type Client struct {
	httpClient  *http.Client
	baseURL     string
	authBaseURL string
	openBaseURL string
	username    string
	password    string
	token       string
}

func NewClient(baseURL, authBaseURL string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultV2BaseURL
	}
	if strings.TrimSpace(authBaseURL) == "" {
		authBaseURL = defaultAuthBaseURL
	}
	return &Client{
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		baseURL:     strings.TrimRight(baseURL, "/"),
		authBaseURL: strings.TrimRight(authBaseURL, "/"),
		openBaseURL: openBaseURLFor(baseURL),
	}
}

func (c *Client) SetCredentials(username, password string) {
	c.username = strings.TrimSpace(username)
	c.password = password
}

func (c *Client) SetBaseURLs(baseURL, authBaseURL string) {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultV2BaseURL
	}
	if strings.TrimSpace(authBaseURL) == "" {
		authBaseURL = defaultAuthBaseURL
	}
	c.baseURL = strings.TrimRight(baseURL, "/")
	c.authBaseURL = strings.TrimRight(authBaseURL, "/")
	c.openBaseURL = openBaseURLFor(baseURL)
}

func (c *Client) BaseURL() string {
	return c.baseURL
}

func (c *Client) SetToken(token string) {
	c.token = strings.TrimSpace(token)
}

func (c *Client) Token() string {
	return c.token
}

func (c *Client) IsAuthenticated() bool {
	return c.token != ""
}

func (c *Client) SignOn(ctx context.Context) (*SignOnResponse, error) {
	if c.username == "" || c.password == "" {
		return nil, fmt.Errorf("ticktick username/password is required")
	}

	payload := map[string]string{
		"username": c.username,
		"password": c.password,
	}

	var resp SignOnResponse
	if err := c.doRequest(ctx, http.MethodPost, c.authBaseURL+"/user/signon?wc=true&remember=true", payload, false, &resp); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.Token) == "" {
		return nil, fmt.Errorf("ticktick signon succeeded but token is empty")
	}

	c.token = strings.TrimSpace(resp.Token)
	return &resp, nil
}

func (c *Client) UserStatus(ctx context.Context) error {
	var raw map[string]interface{}
	return c.doRequest(ctx, http.MethodGet, c.baseURL+"/user/status", nil, true, &raw)
}

func (c *Client) GetBatch(ctx context.Context) (*BatchResponse, error) {
	var resp BatchResponse
	if err := c.doRequest(ctx, http.MethodGet, c.baseURL+"/batch/check/0", nil, true, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) OpenListProjects(ctx context.Context) ([]OpenProject, error) {
	var resp []OpenProject
	if err := c.doOpenRequest(ctx, http.MethodGet, c.openBaseURL+"/project", nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) OpenCreateProject(ctx context.Context, req *OpenProjectCreateRequest) (*OpenProject, error) {
	var resp OpenProject
	if err := c.doOpenRequest(ctx, http.MethodPost, c.openBaseURL+"/project", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) OpenDeleteProject(ctx context.Context, projectID string) error {
	return c.doOpenRequest(ctx, http.MethodDelete, c.openBaseURL+"/project/"+projectID, nil, nil)
}

func (c *Client) OpenProjectData(ctx context.Context, projectID string) (*OpenProjectData, error) {
	var resp OpenProjectData
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("ticktick project id is empty")
	}
	url := c.openBaseURL + "/project/" + projectID + "/data"
	if strings.EqualFold(projectID, openInboxProjectID) {
		url = c.openBaseURL + "/project/" + openInboxProjectID + "/data"
	}
	if err := c.doOpenRequest(ctx, http.MethodGet, url, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) OpenCreateTask(ctx context.Context, req *OpenTaskCreateRequest) (*OpenTask, error) {
	var resp OpenTask
	if err := c.doOpenRequest(ctx, http.MethodPost, c.openBaseURL+"/task", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) OpenUpdateTask(ctx context.Context, taskID string, req *OpenTaskUpdateRequest) (*OpenTask, error) {
	var resp OpenTask
	if err := c.doOpenRequest(ctx, http.MethodPost, c.openBaseURL+"/task/"+taskID, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) OpenGetTask(ctx context.Context, projectID, taskID string) (*OpenTask, error) {
	var resp OpenTask
	if err := c.doOpenRequest(ctx, http.MethodGet, c.openBaseURL+"/project/"+projectID+"/task/"+taskID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) OpenDeleteTask(ctx context.Context, projectID, taskID string) error {
	return c.doOpenRequest(ctx, http.MethodDelete, c.openBaseURL+"/project/"+projectID+"/task/"+taskID, nil, nil)
}

func (c *Client) BatchProject(ctx context.Context, payload *BatchProjectRequest) (*BatchMutationResponse, error) {
	var resp BatchMutationResponse
	if err := c.doRequest(ctx, http.MethodPost, c.baseURL+"/batch/project", payload, true, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) BatchTask(ctx context.Context, payload *BatchTaskRequest) (*BatchMutationResponse, error) {
	var resp BatchMutationResponse
	if err := c.doRequest(ctx, http.MethodPost, c.baseURL+"/batch/task", payload, true, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) doRequest(ctx context.Context, method, url string, body interface{}, needAuth bool, out interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body failed: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", defaultRequestAgent)
	req.Header.Set("X-Device", `{"platform":"web","version":6430,"id":"taskbridge-client"}`)
	if needAuth {
		if c.token == "" {
			return fmt.Errorf("ticktick token is empty")
		}
		req.AddCookie(&http.Cookie{Name: "t", Value: c.token})
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ticktick api error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respData)))
	}
	if out != nil && len(respData) > 0 {
		if err := json.Unmarshal(respData, out); err != nil {
			return fmt.Errorf("decode response failed: %w body=%s", err, strings.TrimSpace(string(respData)))
		}
	}
	return nil
}

func (c *Client) doOpenRequest(ctx context.Context, method, url string, body interface{}, out interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body failed: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", defaultRequestAgent)
	if c.token == "" {
		return fmt.Errorf("ticktick token is empty")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ticktick openapi error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respData)))
	}
	if out != nil && len(respData) > 0 {
		if err := json.Unmarshal(respData, out); err != nil {
			return fmt.Errorf("decode response failed: %w body=%s", err, strings.TrimSpace(string(respData)))
		}
	}
	return nil
}

func openBaseURLFor(baseURL string) string {
	if strings.Contains(strings.ToLower(baseURL), "dida365.com") {
		return didaOpenBaseURL
	}
	return defaultOpenBaseURL
}
