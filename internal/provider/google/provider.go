// Package google provides Google Tasks provider implementation
package google

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/provider"
)

// Provider Google Tasks Provider
type Provider struct {
	client       *Client
	oauth        *OAuth2Client
	config       Config
	capabilities provider.Capabilities
}

// Config Google Provider 配置
type Config struct {
	ClientID        string
	ClientSecret    string
	RedirectURL     string
	CredentialsFile string
	TokenFile       string
}

// NewProvider 创建 Google Tasks Provider
func NewProvider(cfg Config) (*Provider, error) {
	p := &Provider{
		config: cfg,
		capabilities: provider.Capabilities{
			SupportsSubtasks:     false,
			SupportsTags:         false,
			SupportsCategories:   false,
			SupportsReminder:     false,
			SupportsDueDate:      true,
			SupportsStartDate:    false,
			SupportsProgress:     false,
			SupportsPriority:     false,
			SupportsSearch:       false,
			SupportsBatch:        false,
			SupportsDeltaSync:    false,
			MaxTaskLength:        8192,
			MaxDescriptionLength: 8192,
		},
	}

	// 初始化 OAuth2
	if cfg.CredentialsFile != "" {
		oauth, err := LoadCredentials(cfg.CredentialsFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load credentials: %w", err)
		}
		if cfg.TokenFile != "" {
			oauth.tokenFile = cfg.TokenFile
		}
		p.oauth = oauth
	} else if cfg.ClientID != "" && cfg.ClientSecret != "" {
		p.oauth = NewOAuth2Client(&OAuthConfig{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			TokenFile:    cfg.TokenFile,
		})
	}

	return p, nil
}

// Name 返回 Provider 名称
func (p *Provider) Name() string {
	return "google"
}

// DisplayName 返回 Provider 显示名称
func (p *Provider) DisplayName() string {
	return "Google Tasks"
}

// Authenticate 认证
func (p *Provider) Authenticate(ctx context.Context, _ map[string]interface{}) error {
	// 如果已有有效 token，直接返回
	if p.oauth != nil {
		_, err := p.oauth.ValidToken(ctx)
		if err == nil {
			// 创建客户端
			httpClient, err := p.oauth.HTTPClient(ctx)
			if err != nil {
				return err
			}
			p.client = NewClient("")
			p.client.SetHTTPClient(httpClient)
			return nil
		}

		// 尝试从文件加载 token
		if _, err := p.oauth.LoadToken(); err == nil {
			httpClient, err := p.oauth.HTTPClient(ctx)
			if err != nil {
				return err
			}
			p.client = NewClient("")
			p.client.SetHTTPClient(httpClient)
			return nil
		}
	}

	// 需要用户授权
	return fmt.Errorf("authentication required: please run 'taskbridge auth google' to authenticate")
}

// IsAuthenticated 检查是否已认证
func (p *Provider) IsAuthenticated() bool {
	if p.oauth == nil {
		return false
	}
	return !p.oauth.IsExpired()
}

// RefreshToken 刷新 token
func (p *Provider) RefreshToken(ctx context.Context) error {
	if p.oauth == nil {
		return fmt.Errorf("OAuth2 not configured")
	}

	token, err := p.oauth.RefreshToken(ctx)
	if err != nil {
		return err
	}

	// 保存新 token
	if p.oauth.tokenFile != "" {
		if err := p.oauth.SaveToken(token); err != nil {
			return err
		}
	}

	return nil
}

// ListTaskLists 获取任务列表
func (p *Provider) ListTaskLists(ctx context.Context) ([]model.TaskList, error) {
	if err := p.ensureClient(ctx); err != nil {
		return nil, err
	}

	result, err := p.client.ListTaskLists(ctx, "", 100)
	if err != nil {
		return nil, err
	}

	lists := make([]model.TaskList, len(result.Items))
	for i, item := range result.Items {
		lists[i] = model.TaskList{
			ID:          item.ID,
			Name:        item.Title,
			Source:      model.SourceGoogle,
			SourceRawID: item.ID,
		}
		if item.Updated != "" {
			if updated, err := time.Parse(time.RFC3339, item.Updated); err == nil {
				lists[i].UpdatedAt = updated
			}
		}
	}

	return lists, nil
}

// CreateTaskList 创建任务列表
func (p *Provider) CreateTaskList(ctx context.Context, name string) (*model.TaskList, error) {
	if err := p.ensureClient(ctx); err != nil {
		return nil, err
	}

	result, err := p.client.CreateTaskList(ctx, name)
	if err != nil {
		return nil, err
	}

	return &model.TaskList{
		ID:          result.ID,
		Name:        result.Title,
		Source:      model.SourceGoogle,
		SourceRawID: result.ID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

// DeleteTaskList 删除任务列表
func (p *Provider) DeleteTaskList(ctx context.Context, listID string) error {
	if err := p.ensureClient(ctx); err != nil {
		return err
	}

	return p.client.DeleteTaskList(ctx, listID)
}

// ListTasks 获取任务
func (p *Provider) ListTasks(ctx context.Context, listID string, opts provider.ListOptions) ([]model.Task, error) {
	if err := p.ensureClient(ctx); err != nil {
		return nil, err
	}

	// 获取任务列表名称
	lists, err := p.ListTaskLists(ctx)
	if err != nil {
		return nil, err
	}
	var listName string
	for _, list := range lists {
		if list.ID == listID {
			listName = list.Name
			break
		}
	}

	googleOpts := ListTasksOptions{
		ShowCompleted: true,
		ShowDeleted:   false,
		ShowHidden:    false,
	}

	if opts.Completed != nil {
		googleOpts.ShowCompleted = *opts.Completed
	}
	if opts.UpdatedAfter != nil {
		googleOpts.UpdatedMin = opts.UpdatedAfter.Format(time.RFC3339)
	}

	var allTasks []model.Task
	pageToken := opts.PageToken

	for {
		googleOpts.PageToken = pageToken
		if opts.PageSize > 0 {
			googleOpts.MaxResults = int64(opts.PageSize)
		}

		result, err := p.client.ListTasks(ctx, listID, googleOpts)
		if err != nil {
			return nil, err
		}

		for _, item := range result.Items {
			if item.Deleted || item.Hidden {
				continue
			}
			task := item.ToModelTask(listID, listName)
			allTasks = append(allTasks, *task)
		}

		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}

	return allTasks, nil
}

// GetTask 获取单个任务
func (p *Provider) GetTask(ctx context.Context, listID, taskID string) (*model.Task, error) {
	if err := p.ensureClient(ctx); err != nil {
		return nil, err
	}

	// 获取任务列表名称
	lists, err := p.ListTaskLists(ctx)
	if err != nil {
		return nil, err
	}
	var listName string
	for _, list := range lists {
		if list.ID == listID {
			listName = list.Name
			break
		}
	}

	result, err := p.client.GetTask(ctx, listID, taskID)
	if err != nil {
		return nil, err
	}

	return result.ToModelTask(listID, listName), nil
}

// SearchTasks 搜索任务（Google Tasks API 不支持搜索）
func (p *Provider) SearchTasks(ctx context.Context, query string) ([]model.Task, error) {
	// Google Tasks API 不支持搜索功能
	// 需要获取所有任务然后在本地过滤
	return nil, fmt.Errorf("search not supported by Google Tasks API")
}

// CreateTask 创建任务
func (p *Provider) CreateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error) {
	if err := p.ensureClient(ctx); err != nil {
		return nil, err
	}

	gtask := FromModelTask(task)
	result, err := p.client.CreateTask(ctx, listID, gtask)
	if err != nil {
		return nil, err
	}

	// 获取任务列表名称
	lists, err := p.ListTaskLists(ctx)
	if err != nil {
		return nil, err
	}
	var listName string
	for _, list := range lists {
		if list.ID == listID {
			listName = list.Name
			break
		}
	}

	return result.ToModelTask(listID, listName), nil
}

// UpdateTask 更新任务
func (p *Provider) UpdateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error) {
	if err := p.ensureClient(ctx); err != nil {
		return nil, err
	}

	gtask := FromModelTask(task)
	result, err := p.client.UpdateTask(ctx, listID, gtask)
	if err != nil {
		return nil, err
	}

	// 获取任务列表名称
	lists, err := p.ListTaskLists(ctx)
	if err != nil {
		return nil, err
	}
	var listName string
	for _, list := range lists {
		if list.ID == listID {
			listName = list.Name
			break
		}
	}

	return result.ToModelTask(listID, listName), nil
}

// DeleteTask 删除任务
func (p *Provider) DeleteTask(ctx context.Context, listID, taskID string) error {
	if err := p.ensureClient(ctx); err != nil {
		return err
	}

	return p.client.DeleteTask(ctx, listID, taskID)
}

// BatchCreate 批量创建任务
func (p *Provider) BatchCreate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error) {
	// Google Tasks API 不支持批量操作
	// 逐个创建
	result := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		created, err := p.CreateTask(ctx, listID, task)
		if err != nil {
			return result, err
		}
		result = append(result, *created)
	}
	return result, nil
}

// BatchUpdate 批量更新任务
func (p *Provider) BatchUpdate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error) {
	// Google Tasks API 不支持批量操作
	// 逐个更新
	result := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		updated, err := p.UpdateTask(ctx, listID, task)
		if err != nil {
			return result, err
		}
		result = append(result, *updated)
	}
	return result, nil
}

// GetChanges 获取变更（Google Tasks API 不支持增量同步）
func (p *Provider) GetChanges(ctx context.Context, since time.Time) (*provider.SyncChanges, error) {
	// Google Tasks API 不支持增量同步
	// 需要获取所有任务并比较
	return nil, fmt.Errorf("delta sync not supported by Google Tasks API")
}

// Capabilities 返回 Provider 能力
func (p *Provider) Capabilities() provider.Capabilities {
	return p.capabilities
}

// GetTokenInfo 获取 Token 信息
func (p *Provider) GetTokenInfo() *provider.TokenInfo {
	info := &provider.TokenInfo{
		Provider: p.Name(),
		HasToken: false,
		IsValid:  false,
	}

	if p.oauth == nil {
		return info
	}

	// 获取 OAuth 客户端的 token 信息
	oauthInfo := p.oauth.GetTokenInfo()
	if oauthInfo == nil {
		return info
	}

	// 检查是否有 token
	info.HasToken = oauthInfo.AccessToken != ""
	info.IsValid = oauthInfo.Valid

	// 获取过期时间
	if !oauthInfo.Expiry.IsZero() {
		info.ExpiresAt = oauthInfo.Expiry
		timeUntilExpiry := time.Until(oauthInfo.Expiry)
		info.TimeUntilExpiry = formatDurationGoogle(timeUntilExpiry)
		info.NeedsRefresh = timeUntilExpiry <= 5*time.Minute
	}

	// 检查是否可刷新
	info.Refreshable = oauthInfo.RefreshToken != ""

	return info
}

// formatDurationGoogle 格式化持续时间
func formatDurationGoogle(d time.Duration) string {
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

// ensureClient 确保客户端已初始化
func (p *Provider) ensureClient(ctx context.Context) error {
	if p.client != nil {
		return nil
	}

	if p.oauth == nil {
		return fmt.Errorf("OAuth2 not configured")
	}

	httpClient, err := p.oauth.HTTPClient(ctx)
	if err != nil {
		return err
	}

	p.client = NewClient("")
	p.client.SetHTTPClient(httpClient)
	return nil
}

// GetAuthURL 获取授权 URL（用于 CLI 认证流程）
func (p *Provider) GetAuthURL(state string) string {
	return p.oauth.GetAuthURL(state)
}

// CompleteAuth 完成授权（用于 CLI 认证流程）
func (p *Provider) CompleteAuth(ctx context.Context, code string) error {
	token, err := p.oauth.Exchange(ctx, code)
	if err != nil {
		return err
	}

	return p.oauth.SaveToken(token)
}

// NewProviderFromHome 从 HOME 目录加载凭证创建 Provider
// 凭证文件路径: ~/.taskbridge/credentials/google_credentials.json
// Token 文件路径: ~/.taskbridge/credentials/google_token.json
func NewProviderFromHome() (*Provider, error) {
	// 确保凭证目录存在
	if err := EnsureCredentialsDir(); err != nil {
		return nil, fmt.Errorf("failed to create credentials directory: %w", err)
	}

	// 获取凭证文件路径
	credentialsPath := GetCredentialsPath()

	// 检查凭证文件是否存在
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("credentials file not found at %s", credentialsPath)
	}

	// 创建 Provider
	p, err := NewProvider(Config{
		CredentialsFile: credentialsPath,
		TokenFile:       GetTokenPath(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	// 尝试认证（如果已有 token）
	_ = p.Authenticate(context.Background(), nil)

	return p, nil
}
