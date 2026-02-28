package todoist

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/pkg/paths"
)

// Provider Todoist Provider。
type Provider struct {
	client       *Client
	config       Config
	capabilities provider.Capabilities
}

// Config Todoist Provider 配置。
type Config struct {
	APIToken  string
	TokenFile string
}

// NewProvider 创建 Todoist Provider。
func NewProvider(cfg Config) (*Provider, error) {
	p := &Provider{
		config: cfg,
		capabilities: provider.Capabilities{
			SupportsSubtasks:     true,
			SupportsTags:         true,
			SupportsCategories:   false,
			SupportsReminder:     false,
			SupportsDueDate:      true,
			SupportsStartDate:    false,
			SupportsProgress:     false,
			SupportsPriority:     true,
			SupportsSearch:       true,
			SupportsBatch:        true,
			SupportsDeltaSync:    false,
			MaxTaskLength:        500,
			MaxDescriptionLength: 16384,
		},
	}

	token := strings.TrimSpace(cfg.APIToken)
	if token == "" && cfg.TokenFile != "" {
		loaded, err := loadAPITokenFromFile(cfg.TokenFile)
		if err == nil {
			token = loaded
		}
	}

	p.client = NewClient(token)
	return p, nil
}

// NewProviderFromHome 从 HOME 目录加载凭证创建 Provider。
func NewProviderFromHome() (*Provider, error) {
	tokenPath := paths.GetTokenPath("todoist")
	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("token file not found at %s", tokenPath)
	}

	return NewProvider(Config{TokenFile: tokenPath})
}

func (p *Provider) Name() string {
	return "todoist"
}

func (p *Provider) DisplayName() string {
	return "Todoist"
}

func (p *Provider) Authenticate(ctx context.Context, config map[string]interface{}) error {
	if config != nil {
		if token, ok := config["api_token"].(string); ok && strings.TrimSpace(token) != "" {
			p.client.SetAPIToken(strings.TrimSpace(token))
			if p.config.TokenFile != "" {
				_ = saveAPITokenToFile(p.config.TokenFile, p.client.APIToken())
			}
		}
	}

	if !p.IsAuthenticated() {
		return fmt.Errorf("authentication required: please run 'taskbridge auth login todoist'")
	}

	_, err := p.client.ListProjects(ctx)
	if err != nil {
		return fmt.Errorf("todoist authentication failed: %w", err)
	}
	return nil
}

func (p *Provider) IsAuthenticated() bool {
	return strings.TrimSpace(p.client.APIToken()) != ""
}

func (p *Provider) RefreshToken(ctx context.Context) error {
	return nil
}

func (p *Provider) ListTaskLists(ctx context.Context) ([]model.TaskList, error) {
	projects, err := p.client.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	lists := make([]model.TaskList, 0, len(projects))
	for i := range projects {
		lists = append(lists, *toModelTaskList(&projects[i]))
	}
	return lists, nil
}

func (p *Provider) CreateTaskList(ctx context.Context, name string) (*model.TaskList, error) {
	project, err := p.client.CreateProject(ctx, name)
	if err != nil {
		return nil, err
	}
	return toModelTaskList(project), nil
}

func (p *Provider) DeleteTaskList(ctx context.Context, listID string) error {
	return p.client.DeleteProject(ctx, listID)
}

func (p *Provider) ListTasks(ctx context.Context, listID string, opts provider.ListOptions) ([]model.Task, error) {
	tasks, err := p.client.ListTasks(ctx, listID)
	if err != nil {
		return nil, err
	}
	sectionNames, err := p.listSectionNames(ctx, listID)
	if err != nil {
		return nil, err
	}

	result := make([]model.Task, 0, len(tasks))
	for i := range tasks {
		mTask := toModelTaskWithSection(&tasks[i], sectionNames[tasks[i].SectionID.String()])
		if mTask == nil {
			continue
		}

		if opts.Completed != nil && mTask.Status == model.StatusCompleted != *opts.Completed {
			continue
		}
		if opts.DueAfter != nil && mTask.DueDate != nil && mTask.DueDate.Before(*opts.DueAfter) {
			continue
		}
		if opts.DueBefore != nil && mTask.DueDate != nil && mTask.DueDate.After(*opts.DueBefore) {
			continue
		}
		if opts.UpdatedAfter != nil && mTask.UpdatedAt.Before(*opts.UpdatedAfter) {
			continue
		}

		result = append(result, *mTask)
	}
	return result, nil
}

func (p *Provider) GetTask(ctx context.Context, listID, taskID string) (*model.Task, error) {
	task, err := p.client.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	sectionNames, err := p.listSectionNames(ctx, listID)
	if err != nil {
		return nil, err
	}
	mTask := toModelTaskWithSection(task, sectionNames[task.SectionID.String()])
	if mTask == nil {
		return nil, fmt.Errorf("task is nil")
	}
	if mTask.ListID == "" {
		mTask.ListID = listID
	}
	return mTask, nil
}

func (p *Provider) SearchTasks(ctx context.Context, query string) ([]model.Task, error) {
	projects, err := p.client.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	var matched []model.Task
	for _, project := range projects {
		sectionNames, err := p.listSectionNames(ctx, project.ID.String())
		if err != nil {
			continue
		}
		tasks, err := p.client.ListTasks(ctx, project.ID.String())
		if err != nil {
			continue
		}
		for i := range tasks {
			if containsIgnoreCase(tasks[i].Content, query) || containsIgnoreCase(tasks[i].Description, query) {
				mTask := toModelTaskWithSection(&tasks[i], sectionNames[tasks[i].SectionID.String()])
				if mTask == nil {
					continue
				}
				mTask.ListID = project.ID.String()
				mTask.ListName = project.Name
				matched = append(matched, *mTask)
			}
		}
	}

	return matched, nil
}

func (p *Provider) CreateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error) {
	req := toCreateTaskRequest(task, listID)
	created, err := p.client.CreateTask(ctx, req)
	if err != nil {
		return nil, err
	}
	return toModelTask(created), nil
}

func (p *Provider) UpdateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error) {
	taskID := task.SourceRawID
	if taskID == "" {
		taskID = task.ID
	}
	if taskID == "" {
		return nil, fmt.Errorf("task id is empty")
	}
	updated, err := p.client.UpdateTask(ctx, taskID, toUpdateTaskRequest(task))
	if err != nil {
		return nil, err
	}
	mTask := toModelTask(updated)
	if mTask != nil && mTask.ListID == "" {
		mTask.ListID = listID
	}
	return mTask, nil
}

func (p *Provider) DeleteTask(ctx context.Context, listID, taskID string) error {
	return p.client.DeleteTask(ctx, taskID)
}

func (p *Provider) BatchCreate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error) {
	result := make([]model.Task, 0, len(tasks))
	for _, t := range tasks {
		created, err := p.CreateTask(ctx, listID, t)
		if err != nil {
			return result, err
		}
		if created != nil {
			result = append(result, *created)
		}
	}
	return result, nil
}

func (p *Provider) BatchUpdate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error) {
	result := make([]model.Task, 0, len(tasks))
	for _, t := range tasks {
		updated, err := p.UpdateTask(ctx, listID, t)
		if err != nil {
			return result, err
		}
		if updated != nil {
			result = append(result, *updated)
		}
	}
	return result, nil
}

func (p *Provider) GetChanges(ctx context.Context, since time.Time) (*provider.SyncChanges, error) {
	return nil, fmt.Errorf("delta sync not supported by Todoist REST API")
}

func (p *Provider) Capabilities() provider.Capabilities {
	return p.capabilities
}

func (p *Provider) GetTokenInfo() *provider.TokenInfo {
	hasToken := p.IsAuthenticated()
	return &provider.TokenInfo{
		Provider:    p.Name(),
		HasToken:    hasToken,
		IsValid:     hasToken,
		Refreshable: false,
	}
}

func (p *Provider) listSectionNames(ctx context.Context, listID string) (map[string]string, error) {
	sections, err := p.client.ListSections(ctx, listID)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(sections))
	for i := range sections {
		id := sections[i].ID.String()
		if id == "" {
			continue
		}
		result[id] = strings.TrimSpace(sections[i].Name)
	}
	return result, nil
}

func loadAPITokenFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return "", fmt.Errorf("token file is empty")
	}

	var tokenFile TokenFile
	if err := json.Unmarshal(data, &tokenFile); err == nil && strings.TrimSpace(tokenFile.APIToken) != "" {
		return strings.TrimSpace(tokenFile.APIToken), nil
	}

	return trimmed, nil
}

func saveAPITokenToFile(path, token string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	payload := TokenFile{
		APIToken:  token,
		Provider:  "todoist",
		UpdatedAt: time.Now(),
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
