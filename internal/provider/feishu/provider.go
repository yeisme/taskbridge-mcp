// Package feishu provides Feishu (Lark) Task provider implementation
package feishu

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/pkg/paths"
)

// Provider 飞书任务 Provider
type Provider struct {
	client       *Client
	oauth        *OAuth2Client
	config       Config
	capabilities provider.Capabilities
	mu           sync.RWMutex
}

const (
	feishuVirtualMyTasksListID   = "feishu-my-tasks"
	feishuVirtualMyTasksListName = "我的任务"
)

// Config 飞书 Provider 配置
type Config struct {
	AppID           string
	AppSecret       string
	RedirectURL     string
	CredentialsFile string
	TokenFile       string
	Scopes          []string
}

// NewProvider 创建飞书任务 Provider
func NewProvider(cfg Config) (*Provider, error) {
	p := &Provider{
		config: cfg,
		capabilities: provider.Capabilities{
			SupportsSubtasks:     true,  // 飞书支持子任务
			SupportsTags:         true,  // 飞书支持标签
			SupportsCategories:   false, // 使用标签代替
			SupportsReminder:     true,  // 飞书支持提醒
			SupportsDueDate:      true,
			SupportsStartDate:    true,
			SupportsProgress:     false, // 飞书不直接支持进度
			SupportsPriority:     true,  // 飞书支持优先级
			SupportsSearch:       true,  // 通过本地过滤实现
			SupportsBatch:        true,  // 支持批量操作
			SupportsDeltaSync:    true,  // 支持增量同步
			MaxTaskLength:        5000,  // 飞书任务标题限制
			MaxDescriptionLength: 50000, // 飞书描述限制
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
	} else if cfg.AppID != "" {
		p.oauth = NewOAuth2Client(&OAuthConfig{
			AppID:       cfg.AppID,
			AppSecret:   cfg.AppSecret,
			RedirectURL: cfg.RedirectURL,
			Scopes:      cfg.Scopes,
			TokenFile:   cfg.TokenFile,
		})
	}

	// 初始化客户端
	p.client = NewClient("")
	p.client.SetCredentials(cfg.AppID, cfg.AppSecret)

	return p, nil
}

// Name 返回 Provider 名称
func (p *Provider) Name() string {
	return "feishu"
}

// DisplayName 返回 Provider 显示名称
func (p *Provider) DisplayName() string {
	return "飞书任务"
}

// Authenticate 认证
func (p *Provider) Authenticate(ctx context.Context, config map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 如果已有有效 token，直接返回
	if p.oauth != nil {
		token, err := p.oauth.ValidToken(ctx)
		if err == nil {
			p.client.SetUserAccessToken(token.AccessToken)
			if p.oauth.config != nil {
				p.client.SetCredentials(p.oauth.config.AppID, p.oauth.config.AppSecret)
			}
			return nil
		}

		// 尝试从文件加载 token
		if err := p.oauth.LoadToken(); err == nil {
			token, err := p.oauth.ValidToken(ctx)
			if err == nil {
				p.client.SetUserAccessToken(token.AccessToken)
				if p.oauth.config != nil {
					p.client.SetCredentials(p.oauth.config.AppID, p.oauth.config.AppSecret)
				}
				return nil
			}
		}
	}

	// 如果配置中有凭据，使用它们
	if appID, ok := config["app_id"].(string); ok && appID != "" {
		appSecret, _ := config["app_secret"].(string)
		redirectURL, _ := config["redirect_url"].(string)
		tokenFile, _ := config["token_file"].(string)

		p.oauth = NewOAuth2Client(&OAuthConfig{
			AppID:       appID,
			AppSecret:   appSecret,
			RedirectURL: redirectURL,
			TokenFile:   tokenFile,
		})
		p.client.SetCredentials(appID, appSecret)
	}

	// 启动交互式认证流程
	if p.oauth != nil {
		token, err := p.oauth.StartAuthServer(ctx, 0)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		p.client.SetUserAccessToken(token.AccessToken)
		if p.oauth.config != nil {
			p.client.SetCredentials(p.oauth.config.AppID, p.oauth.config.AppSecret)
		}

		log.Info().Str("token_type", token.TokenType).Msg("Feishu authentication successful")
		return nil
	}

	return fmt.Errorf("no OAuth configuration available")
}

// IsAuthenticated 检查是否已认证
func (p *Provider) IsAuthenticated() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.oauth == nil {
		return false
	}

	return p.oauth.IsAuthenticated()
}

// RefreshToken 刷新 token
func (p *Provider) RefreshToken(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.oauth == nil {
		return fmt.Errorf("no OAuth client available")
	}

	token, err := p.oauth.RefreshToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	p.client.SetUserAccessToken(token.AccessToken)
	if p.oauth.config != nil {
		p.client.SetCredentials(p.oauth.config.AppID, p.oauth.config.AppSecret)
	}

	return nil
}

// ================ 任务列表操作 ================

// ListTaskLists 获取所有任务列表
func (p *Provider) ListTaskLists(ctx context.Context) ([]model.TaskList, error) {
	lists, err := p.client.GetAllTaskLists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list task lists: %w", err)
	}
	// 仅保留“我的任务 + 我创建/负责的清单”，避免拉取与当前账号无关的共享清单。
	if p.oauth != nil {
		if userInfo, err := p.oauth.GetUserInfo(ctx); err == nil && userInfo != nil {
			openID := strings.TrimSpace(userInfo.Data.OpenID)
			lists = filterTaskListsByOwnerOrCreator(lists, openID)
		} else if err != nil {
			log.Warn().Err(err).Msg("failed to fetch feishu user info, fallback to unfiltered task lists")
		}
	}

	modelLists := ToModelTaskLists(lists)
	// 增加一个“我的任务”虚拟清单，覆盖飞书「全部任务/我负责」等视图中的任务。
	modelLists = append([]model.TaskList{{
		ID:          feishuVirtualMyTasksListID,
		Name:        feishuVirtualMyTasksListName,
		Source:      model.SourceFeishu,
		SourceRawID: feishuVirtualMyTasksListID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}}, modelLists...)
	return modelLists, nil
}

func filterTaskListsByOwnerOrCreator(lists []TaskList, openID string) []TaskList {
	openID = strings.TrimSpace(openID)
	if openID == "" {
		return lists
	}
	filtered := make([]TaskList, 0, len(lists))
	for _, list := range lists {
		name := strings.TrimSpace(list.Name)
		if strings.EqualFold(name, "我的任务") || strings.EqualFold(name, "my tasks") {
			filtered = append(filtered, list)
			continue
		}
		creatorID := strings.TrimSpace(list.CreatorID)
		ownerID := strings.TrimSpace(list.OwnerID)
		if strings.EqualFold(creatorID, openID) || strings.EqualFold(ownerID, openID) || containsUserID(list.MemberIDs, openID) {
			filtered = append(filtered, list)
		}
	}
	return filtered
}

func containsUserID(ids []string, openID string) bool {
	for _, id := range ids {
		if strings.EqualFold(strings.TrimSpace(id), openID) {
			return true
		}
	}
	return false
}

// CreateTaskList 创建任务列表
func (p *Provider) CreateTaskList(ctx context.Context, name string) (*model.TaskList, error) {
	req := &CreateTaskListRequest{
		Name: name,
	}

	list, err := p.client.CreateTaskList(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create task list: %w", err)
	}

	return ToModelTaskList(list), nil
}

// DeleteTaskList 删除任务列表
func (p *Provider) DeleteTaskList(ctx context.Context, listID string) error {
	return p.client.DeleteTaskList(ctx, listID)
}

// ================ 任务操作 - 读取 ================

// ListTasks 获取任务列表中的任务
func (p *Provider) ListTasks(ctx context.Context, listID string, opts provider.ListOptions) ([]model.Task, error) {
	if strings.TrimSpace(listID) == feishuVirtualMyTasksListID {
		tasks, err := p.client.GetAllMyTasks(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list my tasks: %w", err)
		}
		// 展开子任务：飞书“我的任务”接口通常只返回父任务，子任务需单独拉取。
		allTasks := make([]Task, 0, len(tasks)+8)
		allTasks = append(allTasks, tasks...)
		exists := make(map[string]bool, len(tasks))
		for _, t := range tasks {
			if t.TaskID != "" {
				exists[t.TaskID] = true
			}
		}
		for _, parent := range tasks {
			subtasks, subErr := p.client.ListSubtaskTasks(ctx, parent.TaskID)
			if subErr != nil {
				log.Warn().Err(subErr).Str("parent_task_id", parent.TaskID).Msg("failed to list feishu subtasks")
				continue
			}
			for _, sub := range subtasks {
				if sub.TaskID == "" || exists[sub.TaskID] {
					continue
				}
				exists[sub.TaskID] = true
				allTasks = append(allTasks, sub)
			}
		}

		modelTasks := ToModelTasks(allTasks)
		for i := range modelTasks {
			if modelTasks[i].ListID == "" {
				modelTasks[i].ListID = feishuVirtualMyTasksListID
			}
			if modelTasks[i].ListName == "" {
				modelTasks[i].ListName = feishuVirtualMyTasksListName
			}
		}
		return modelTasks, nil
	}

	// 转换选项
	feishuOpts := &ListTasksOptions{
		PageSize:  opts.PageSize,
		PageToken: opts.PageToken,
	}

	if opts.Completed != nil {
		feishuOpts.Completed = opts.Completed
	}

	if opts.DueBefore != nil {
		feishuOpts.EndDueTime = TimeToMilliseconds(*opts.DueBefore)
	}

	if opts.DueAfter != nil {
		feishuOpts.StartDueTime = TimeToMilliseconds(*opts.DueAfter)
	}

	// 如果指定了分页大小，使用分页
	if opts.PageSize > 0 {
		tasks, _, err := p.client.ListTasks(ctx, listID, feishuOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to list tasks: %w", err)
		}
		return ToModelTasks(tasks), nil
	}

	// 否则获取所有任务
	tasks, err := p.client.GetAllTasks(ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	return ToModelTasks(tasks), nil
}

// GetTask 获取单个任务
func (p *Provider) GetTask(ctx context.Context, listID, taskID string) (*model.Task, error) {
	task, err := p.client.GetTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	return ToModelTask(task), nil
}

// SearchTasks 搜索任务
func (p *Provider) SearchTasks(ctx context.Context, query string) ([]model.Task, error) {
	tasks, err := p.client.SearchTasks(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to search tasks: %w", err)
	}

	return ToModelTasks(tasks), nil
}

// ================ 任务操作 - 写入 ================

// CreateTask 创建任务
func (p *Provider) CreateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error) {
	req := ToFeishuCreateRequest(task, listID)

	createdTask, err := p.client.CreateTask(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	return ToModelTask(createdTask), nil
}

// UpdateTask 更新任务
func (p *Provider) UpdateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error) {
	req := ToFeishuUpdateRequest(task)

	updatedTask, err := p.client.UpdateTask(ctx, task.ID, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
	}

	return ToModelTask(updatedTask), nil
}

// DeleteTask 删除任务
func (p *Provider) DeleteTask(ctx context.Context, listID, taskID string) error {
	return p.client.DeleteTask(ctx, taskID)
}

// ================ 批量操作 ================

// BatchCreate 批量创建任务
func (p *Provider) BatchCreate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error) {
	reqs := make([]*CreateTaskRequest, 0, len(tasks))
	for _, task := range tasks {
		reqs = append(reqs, ToFeishuCreateRequest(task, listID))
	}

	createdTasks, err := p.client.BatchCreateTasks(ctx, listID, reqs)
	if err != nil {
		return nil, fmt.Errorf("failed to batch create tasks: %w", err)
	}

	return ToModelTasks(createdTasks), nil
}

// BatchUpdate 批量更新任务
func (p *Provider) BatchUpdate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error) {
	reqs := make([]*UpdateTaskRequest, 0, len(tasks))
	for _, task := range tasks {
		reqs = append(reqs, ToFeishuUpdateRequest(task))
	}

	updatedTasks, err := p.client.BatchUpdateTasks(ctx, reqs)
	if err != nil {
		return nil, fmt.Errorf("failed to batch update tasks: %w", err)
	}

	return ToModelTasks(updatedTasks), nil
}

// ================ 同步支持 ================

// GetChanges 获取增量变更
func (p *Provider) GetChanges(ctx context.Context, since time.Time) (*provider.SyncChanges, error) {
	// 飞书使用 delta token 进行增量同步
	// 这里简化实现，获取所有任务列表的变更
	changes := &provider.SyncChanges{
		Tasks:      []model.Task{},
		DeletedIDs: []string{},
	}

	// 获取所有任务列表
	lists, err := p.client.GetAllTaskLists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get task lists: %w", err)
	}

	// 获取每个列表中的任务
	for _, list := range lists {
		tasks, err := p.client.GetAllTasks(ctx, list.TaskListID)
		if err != nil {
			log.Warn().Err(err).Str("list_id", list.TaskListID).Msg("Failed to get tasks from list")
			continue
		}

		// 过滤出更新的任务
		for _, task := range tasks {
			updatedTime := MillisecondsToTime(task.UpdatedTime)
			if updatedTime.After(since) {
				changes.Tasks = append(changes.Tasks, *ToModelTask(&task))
			}
		}
	}

	return changes, nil
}

// ================ 能力查询 ================

// Capabilities 返回 Provider 能力
func (p *Provider) Capabilities() provider.Capabilities {
	return p.capabilities
}

// ================ Token 管理 ================

// GetTokenInfo 获取 token 信息
func (p *Provider) GetTokenInfo() *provider.TokenInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.oauth == nil {
		return &provider.TokenInfo{
			Provider: "feishu",
			HasToken: false,
			IsValid:  false,
		}
	}

	tokenInfo := p.oauth.GetTokenInfo()
	hasToken, _ := tokenInfo["authenticated"].(bool)

	info := &provider.TokenInfo{
		Provider:    "feishu",
		HasToken:    hasToken,
		IsValid:     hasToken,
		Refreshable: tokenInfo["has_refresh"].(bool),
	}

	if expiresIn, ok := tokenInfo["expires_in"].(int); ok && expiresIn > 0 {
		info.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
		info.TimeUntilExpiry = fmt.Sprintf("%d seconds", expiresIn)
		info.NeedsRefresh = expiresIn < 300 // 5分钟内过期需要刷新
	}

	return info
}

// ================ 辅助方法 ================

// GetUserInfo 获取用户信息
func (p *Provider) GetUserInfo(ctx context.Context) (*UserInfoResponse, error) {
	return p.oauth.GetUserInfo(ctx)
}

// SetTokenFile 设置 token 文件路径
func (p *Provider) SetTokenFile(path string) {
	if p.oauth != nil {
		p.oauth.SetTokenFile(path)
	}
}

// Logout 登出（撤销 token）
func (p *Provider) Logout() error {
	if p.oauth == nil {
		return nil
	}
	return p.oauth.RevokeToken()
}

// NewProviderFromHome 从 HOME 目录加载凭证创建 Provider
func NewProviderFromHome() (*Provider, error) {
	credentialsPath := paths.GetCredentialsPath("feishu")
	tokenPath := paths.GetTokenPath("feishu")

	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("credentials file not found at %s", credentialsPath)
	}

	p, err := NewProvider(Config{
		CredentialsFile: credentialsPath,
		TokenFile:       tokenPath,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	if err := p.Authenticate(context.Background(), nil); err != nil {
		return nil, fmt.Errorf("feishu provider authentication failed: %w", err)
	}

	return p, nil
}

// ================ 初始化时注册 Provider ================

func init() {
	// 注册飞书 Provider 到全局注册表
	// 这将在程序启动时自动执行
}
