// Package microsoft provides Microsoft To Do provider implementation
package microsoft

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/pkg/paths"
)

// Provider Microsoft To Do Provider
type Provider struct {
	client       *Client
	oauth        *OAuth2Client
	config       Config
	capabilities provider.Capabilities
	mu           sync.RWMutex
}

// Config Microsoft Provider 配置
type Config struct {
	ClientID        string
	ClientSecret    string
	TenantID        string
	RedirectURL     string
	CredentialsFile string
	TokenFile       string
	Scopes          []string
}

// NewProvider 创建 Microsoft To Do Provider
func NewProvider(cfg Config) (*Provider, error) {
	p := &Provider{
		config: cfg,
		capabilities: provider.Capabilities{
			SupportsSubtasks:     true,  // 通过 checklistItems 支持
			SupportsTags:         false, // Microsoft To Do 不支持标签
			SupportsCategories:   true,  // 通过 categories 支持
			SupportsReminder:     true,  // 支持 reminderDateTime
			SupportsDueDate:      true,
			SupportsStartDate:    true,
			SupportsProgress:     false, // 不直接支持进度
			SupportsPriority:     true,  // 通过 importance 支持
			SupportsSearch:       false, // 需要自己实现
			SupportsBatch:        true,  // 支持 $batch
			SupportsDeltaSync:    true,  // 支持 delta 链接
			MaxTaskLength:        10000, // 估计值
			MaxDescriptionLength: 10000, // 估计值
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
	} else if cfg.ClientID != "" {
		p.oauth = NewOAuth2Client(&OAuthConfig{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			TenantID:     cfg.TenantID,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
			TokenFile:    cfg.TokenFile,
		})
	}

	// 初始化客户端
	p.client = NewClient("")

	return p, nil
}

// Name 返回 Provider 名称
func (p *Provider) Name() string {
	return "microsoft"
}

// DisplayName 返回 Provider 显示名称
func (p *Provider) DisplayName() string {
	return "Microsoft To Do"
}

// Authenticate 认证
func (p *Provider) Authenticate(ctx context.Context, config map[string]interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 如果已有有效 token，直接返回
	if p.oauth != nil {
		_, err := p.oauth.ValidToken(ctx)
		if err == nil {
			// 创建客户端
			httpClient, err := p.oauth.HTTPClient(ctx)
			if err != nil {
				return err
			}
			p.client.SetHTTPClient(httpClient)
			return nil
		}

		// 尝试从文件加载 token
		if err := p.oauth.LoadToken(); err == nil {
			_, err := p.oauth.ValidToken(ctx)
			if err == nil {
				httpClient, err := p.oauth.HTTPClient(ctx)
				if err != nil {
					return err
				}
				p.client.SetHTTPClient(httpClient)
				return nil
			}
		}
	}

	// 如果配置中有凭据，使用它们
	if clientID, ok := config["client_id"].(string); ok && clientID != "" {
		clientSecret, _ := config["client_secret"].(string)
		tenantID, _ := config["tenant_id"].(string)
		redirectURL, _ := config["redirect_url"].(string)
		tokenFile, _ := config["token_file"].(string)

		p.oauth = NewOAuth2Client(&OAuthConfig{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			TenantID:     tenantID,
			RedirectURL:  redirectURL,
			TokenFile:    tokenFile,
		})
	}

	// 启动交互式认证流程
	if p.oauth != nil {
		token, err := p.oauth.StartAuthServer(ctx, 0)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		// 设置 HTTP 客户端
		httpClient, err := p.oauth.HTTPClient(ctx)
		if err != nil {
			return err
		}
		p.client.SetHTTPClient(httpClient)

		log.Info().Str("token_type", token.TokenType).Msg("Microsoft authentication successful")
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

	_, err := p.oauth.RefreshToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	// 更新 HTTP 客户端
	httpClient, err := p.oauth.HTTPClient(ctx)
	if err != nil {
		return err
	}
	p.client.SetHTTPClient(httpClient)

	return nil
}

// ================ 任务列表操作 ================

// ListTaskLists 获取所有任务列表
func (p *Provider) ListTaskLists(ctx context.Context) ([]model.TaskList, error) {
	if !p.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated")
	}

	lists, err := p.client.ListTodoLists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list task lists: %w", err)
	}

	result := make([]model.TaskList, 0, len(lists))
	for _, list := range lists {
		result = append(result, *ToModelTaskList(&list))
	}

	return result, nil
}

// CreateTaskList 创建任务列表
func (p *Provider) CreateTaskList(ctx context.Context, name string) (*model.TaskList, error) {
	if !p.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated")
	}

	list, err := p.client.CreateTodoList(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to create task list: %w", err)
	}

	return ToModelTaskList(list), nil
}

// DeleteTaskList 删除任务列表
func (p *Provider) DeleteTaskList(ctx context.Context, listID string) error {
	if !p.IsAuthenticated() {
		return fmt.Errorf("not authenticated")
	}

	return p.client.DeleteTodoList(ctx, listID)
}

// ================ 任务操作 - 读取 ================

// ListTasks 列出任务
func (p *Provider) ListTasks(ctx context.Context, listID string, opts provider.ListOptions) ([]model.Task, error) {
	if !p.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated")
	}

	// 构建查询选项
	clientOpts := &ListOptions{}
	if opts.PageSize > 0 {
		clientOpts.Top = opts.PageSize
	}

	// 构建过滤条件
	var filters []string
	if opts.Completed != nil {
		if *opts.Completed {
			filters = append(filters, "status eq 'completed'")
		} else {
			filters = append(filters, "status ne 'completed'")
		}
	}
	if opts.DueBefore != nil {
		filters = append(filters, fmt.Sprintf("dueDateTime/dateTime lt '%s'", opts.DueBefore.Format("2006-01-02")))
	}
	if opts.DueAfter != nil {
		filters = append(filters, fmt.Sprintf("dueDateTime/dateTime gt '%s'", opts.DueAfter.Format("2006-01-02")))
	}
	if len(filters) > 0 {
		// 注意：Microsoft Graph API 的 filter 语法可能需要调整
		clientOpts.Filter = filters[0]
		for i := 1; i < len(filters); i++ {
			clientOpts.Filter += " and " + filters[i]
		}
	}

	tasks, err := p.client.ListTasks(ctx, listID, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	// 获取列表名称，用于写入统一任务模型的 ListName。
	listName := ""
	if list, err := p.client.GetTodoList(ctx, listID); err == nil && list != nil {
		listName = list.DisplayName
	}

	result := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		modelTask := ToModelTask(&task)
		if modelTask != nil {
			modelTask.ListID = listID
			if listName != "" {
				modelTask.ListName = listName
			}
			result = append(result, *modelTask)

			// Microsoft 的子任务是 checklistItems，这里展开为本地子任务模型。
			for _, item := range task.ChecklistItems {
				parentID := modelTask.ID
				subtask := model.Task{
					ID:          fmt.Sprintf("ms-step-%s-%s", task.ID, item.ID),
					Title:       item.DisplayName,
					Status:      model.StatusTodo,
					Source:      model.SourceMicrosoft,
					SourceRawID: "ms_step:" + item.ID,
					ListID:      listID,
					ListName:    listName,
					ParentID:    &parentID,
					CreatedAt:   item.CreatedDateTime,
					UpdatedAt:   item.LastModifiedDateTime,
					Metadata: &model.TaskMetadata{
						Version:    "1.0",
						LastSyncAt: time.Now(),
						SyncSource: "microsoft",
						LocalID:    fmt.Sprintf("ms-step-%s-%s", task.ID, item.ID),
						CustomFields: map[string]interface{}{
							"tb_ms_step_id":              item.ID,
							"tb_ms_parent_source_raw_id": task.ID,
						},
					},
				}
				if item.IsChecked {
					subtask.Status = model.StatusCompleted
					now := time.Now()
					subtask.CompletedAt = &now
				}
				if subtask.CreatedAt.IsZero() {
					subtask.CreatedAt = modelTask.CreatedAt
				}
				if subtask.UpdatedAt.IsZero() {
					subtask.UpdatedAt = modelTask.UpdatedAt
				}
				result = append(result, subtask)
			}
		}
	}

	return result, nil
}

// GetTask 获取单个任务
func (p *Provider) GetTask(ctx context.Context, listID, taskID string) (*model.Task, error) {
	if !p.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated")
	}

	task, err := p.client.GetTask(ctx, listID, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	return ToModelTask(task), nil
}

// SearchTasks 搜索任务
func (p *Provider) SearchTasks(ctx context.Context, query string) ([]model.Task, error) {
	if !p.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated")
	}

	// Microsoft To Do API 不直接支持搜索，需要获取所有任务后本地过滤
	// 这是一个简化的实现
	lists, err := p.client.ListTodoLists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list task lists: %w", err)
	}

	var results []model.Task
	for _, list := range lists {
		tasks, err := p.client.GetAllTasks(ctx, list.ID)
		if err != nil {
			log.Warn().Err(err).Str("list_id", list.ID).Msg("Failed to get tasks from list")
			continue
		}

		for _, task := range tasks {
			// 简单的标题匹配
			if containsIgnoreCase(task.Title, query) {
				results = append(results, *ToModelTask(&task))
			}
		}
	}

	return results, nil
}

// ================ 任务操作 - 写入 ================

// CreateTask 创建任务
func (p *Provider) CreateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error) {
	if !p.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated")
	}

	msTask := ToMicrosoftTask(task)
	result, err := p.client.CreateTask(ctx, listID, msTask)
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	return ToModelTask(result), nil
}

// UpdateTask 更新任务
func (p *Provider) UpdateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error) {
	if !p.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated")
	}

	msTask := ToMicrosoftTask(task)
	result, err := p.client.UpdateTask(ctx, listID, msTask)
	if err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
	}

	return ToModelTask(result), nil
}

// DeleteTask 删除任务
func (p *Provider) DeleteTask(ctx context.Context, listID, taskID string) error {
	if !p.IsAuthenticated() {
		return fmt.Errorf("not authenticated")
	}

	return p.client.DeleteTask(ctx, listID, taskID)
}

// CreateChecklistItem 在指定任务下创建步骤（Microsoft To Do checklist item）。
func (p *Provider) CreateChecklistItem(ctx context.Context, listID, taskID, displayName string, isChecked bool) (*ChecklistItem, error) {
	if !p.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated")
	}
	return p.client.CreateChecklistItem(ctx, listID, taskID, displayName, isChecked)
}

// UpdateChecklistItem 更新步骤标题/完成状态。
func (p *Provider) UpdateChecklistItem(ctx context.Context, listID, taskID, itemID, displayName string, isChecked bool) (*ChecklistItem, error) {
	if !p.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated")
	}
	return p.client.UpdateChecklistItem(ctx, listID, taskID, itemID, displayName, isChecked)
}

// ================ 批量操作 ================

// BatchCreate 批量创建任务
func (p *Provider) BatchCreate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error) {
	if !p.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated")
	}

	msTasks := make([]*TodoTask, 0, len(tasks))
	for _, task := range tasks {
		msTasks = append(msTasks, ToMicrosoftTask(task))
	}

	results, err := p.client.BatchCreateTasks(ctx, listID, msTasks)
	if err != nil {
		return nil, fmt.Errorf("failed to batch create tasks: %w", err)
	}

	modelTasks := make([]model.Task, 0, len(results))
	for _, result := range results {
		modelTasks = append(modelTasks, *ToModelTask(&result))
	}

	return modelTasks, nil
}

// BatchUpdate 批量更新任务
func (p *Provider) BatchUpdate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error) {
	if !p.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated")
	}

	// Microsoft To Do 批量更新需要使用 $batch API
	// 这里简化实现，逐个更新
	results := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		updated, err := p.UpdateTask(ctx, listID, task)
		if err != nil {
			log.Warn().Err(err).Str("task_id", task.ID).Msg("Failed to update task in batch")
			continue
		}
		results = append(results, *updated)
	}

	return results, nil
}

// ================ 同步支持 ================

// GetChanges 获取增量变更
func (p *Provider) GetChanges(ctx context.Context, since time.Time) (*provider.SyncChanges, error) {
	if !p.IsAuthenticated() {
		return nil, fmt.Errorf("not authenticated")
	}

	// 获取所有任务列表
	lists, err := p.client.ListTodoLists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list task lists: %w", err)
	}

	changes := &provider.SyncChanges{
		Tasks:      []model.Task{},
		DeletedIDs: []string{},
		HasMore:    false,
	}

	// 对每个列表获取增量
	for _, list := range lists {
		// 使用 delta API 获取变更
		delta, err := p.client.GetDelta(ctx, list.ID, "")
		if err != nil {
			log.Warn().Err(err).Str("list_id", list.ID).Msg("Failed to get delta")
			continue
		}

		for _, task := range delta.Value {
			// 过滤出指定时间后的变更
			if task.LastModifiedDateTime.After(since) {
				changes.Tasks = append(changes.Tasks, *ToModelTask(&task))
			}
		}

		if delta.DeltaLink != "" {
			changes.NextToken = delta.DeltaLink
		}
		changes.HasMore = changes.HasMore || delta.NextLink != ""
	}

	return changes, nil
}

// ================ 能力查询 ================

// Capabilities 返回 Provider 能力
func (p *Provider) Capabilities() provider.Capabilities {
	return p.capabilities
}

// GetTokenInfo 获取 Token 信息
func (p *Provider) GetTokenInfo() *provider.TokenInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

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
	hasToken, ok := oauthInfo["authenticated"].(bool)
	info.HasToken = ok && hasToken

	// 检查是否有效
	if info.HasToken {
		info.IsValid = p.oauth.IsAuthenticated()
	}

	// 获取过期时间
	if expiry, ok := oauthInfo["expiry"].(time.Time); ok && !expiry.IsZero() {
		info.ExpiresAt = expiry
		timeUntilExpiry := time.Until(expiry)
		info.TimeUntilExpiry = formatDuration(timeUntilExpiry)
		info.NeedsRefresh = timeUntilExpiry <= 5*time.Minute
	}

	// 检查是否可刷新
	if hasRefresh, ok := oauthInfo["has_refresh"].(bool); ok {
		info.Refreshable = hasRefresh
	}

	return info
}

// formatDuration 格式化持续时间
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

// ================ 辅助函数 ================

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

// GetDefaultListID 获取默认任务列表 ID
func (p *Provider) GetDefaultListID(ctx context.Context) (string, error) {
	lists, err := p.client.ListTodoLists(ctx)
	if err != nil {
		return "", err
	}

	// 查找 "Tasks" 列表（Microsoft To Do 的默认列表）
	for _, list := range lists {
		if list.WellknownName == "defaultList" || list.DisplayName == "Tasks" {
			return list.ID, nil
		}
	}

	// 返回第一个列表
	if len(lists) > 0 {
		return lists[0].ID, nil
	}

	return "", fmt.Errorf("no task lists found")
}

// Logout 登出
func (p *Provider) Logout() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.oauth != nil {
		return p.oauth.RevokeToken()
	}
	return nil
}

// GetAuthInfo 获取认证信息
func (p *Provider) GetAuthInfo() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	info := map[string]interface{}{
		"provider":      p.Name(),
		"authenticated": p.IsAuthenticated(),
	}

	if p.oauth != nil {
		tokenInfo := p.oauth.GetTokenInfo()
		for k, v := range tokenInfo {
			info[k] = v
		}
	}

	return info
}

// WriteTokenToFile 将 token 写入指定文件
func (p *Provider) WriteTokenToFile(filePath string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.oauth == nil {
		return fmt.Errorf("no OAuth client available")
	}

	p.oauth.tokenFile = filePath
	return p.oauth.SaveToken()
}

// LoadTokenFromFile 从指定文件加载 token
func (p *Provider) LoadTokenFromFile(filePath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.oauth == nil {
		return fmt.Errorf("no OAuth client available")
	}

	p.oauth.tokenFile = filePath
	return p.oauth.LoadToken()
}

// SetEnvAuth 从环境变量设置认证
func (p *Provider) SetEnvAuth() error {
	clientID := os.Getenv("MICROSOFT_CLIENT_ID")
	clientSecret := os.Getenv("MICROSOFT_CLIENT_SECRET")
	tenantID := os.Getenv("MICROSOFT_TENANT_ID")

	if clientID == "" {
		return fmt.Errorf("MICROSOFT_CLIENT_ID environment variable not set")
	}

	p.oauth = NewOAuth2Client(&OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TenantID:     tenantID,
		TokenFile:    os.Getenv("MICROSOFT_TOKEN_FILE"),
	})

	return nil
}

// NewProviderFromHome 从 HOME 目录加载凭证创建 Provider
// 凭证文件路径: ~/.taskbridge/credentials/microsoft_credentials.json
// Token 文件路径: ~/.taskbridge/credentials/tokens.json
func NewProviderFromHome() (*Provider, error) {
	credentialsDir := paths.GetCredentialsDir()

	// 确保凭证目录存在
	if err := os.MkdirAll(credentialsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create credentials directory: %w", err)
	}

	// 获取凭证文件路径
	credentialsPath := paths.GetCredentialsPath("microsoft")

	// 检查凭证文件是否存在
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("credentials file not found at %s", credentialsPath)
	}

	// 获取 Token 文件路径
	tokenPath := paths.GetTokenPath("microsoft")

	// 创建 Provider
	p, err := NewProvider(Config{
		CredentialsFile: credentialsPath,
		TokenFile:       tokenPath,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	// 尝试加载已有 token（不启动交互式认证）
	if p.oauth != nil {
		// 尝试从文件加载 token
		if err := p.oauth.LoadToken(); err != nil {
			return nil, fmt.Errorf("no valid token found, please run 'taskbridge auth microsoft': %w", err)
		}

		// 验证 token 是否有效
		ctx := context.Background()
		validToken, err := p.oauth.ValidToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("token validation failed, please run 'taskbridge auth microsoft': %w", err)
		}

		log.Debug().
			Str("token_type", validToken.TokenType).
			Bool("valid", validToken.Valid()).
			Msg("Microsoft token loaded successfully")

		// 直接设置 token 到 client
		p.client.SetAuthToken(validToken.AccessToken)
	}

	return p, nil
}
