package ticktick

import (
	"bytes"
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

type Provider struct {
	client       *Client
	config       Config
	capabilities provider.Capabilities
	source       model.TaskSource
	name         string
	displayName  string
}

type Config struct {
	ProviderName string
	Username     string
	Password     string
	Token        string
	TokenFile    string
}

type tokenStore struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
	APIToken string `json:"api_token,omitempty"`
}

func NewProvider(cfg Config) (*Provider, error) {
	name := normalizeProviderName(cfg.ProviderName)
	baseURL, authBaseURL, source, displayName := providerProfile(name)
	cfg.ProviderName = name

	p := &Provider{
		config:      cfg,
		client:      NewClient(baseURL, authBaseURL),
		source:      source,
		name:        name,
		displayName: displayName,
		capabilities: provider.Capabilities{
			SupportsSubtasks:     true,
			SupportsTags:         true,
			SupportsCategories:   false,
			SupportsReminder:     false,
			SupportsDueDate:      true,
			SupportsStartDate:    true,
			SupportsProgress:     true,
			SupportsPriority:     true,
			SupportsSearch:       true,
			SupportsBatch:        true,
			SupportsDeltaSync:    false,
			MaxTaskLength:        5000,
			MaxDescriptionLength: 50000,
		},
	}
	if cfg.TokenFile != "" {
		if s, err := loadTokenStore(cfg.TokenFile); err == nil {
			if strings.TrimSpace(cfg.Username) == "" {
				cfg.Username = s.Username
			}
			if cfg.Password == "" {
				cfg.Password = s.Password
			}
			if strings.TrimSpace(cfg.Token) == "" {
				cfg.Token = s.Token
			}
		}
	}

	p.config = cfg
	p.client.SetCredentials(cfg.Username, cfg.Password)
	p.client.SetToken(cfg.Token)
	return p, nil
}

func NewProviderFromHome() (*Provider, error) {
	return NewProviderFromHomeByName("ticktick")
}

func NewProviderFromHomeByName(providerName string) (*Provider, error) {
	resolvedName := normalizeProviderName(providerName)
	tokenPath := paths.GetTokenPath(resolvedName)
	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("token file not found at %s", tokenPath)
	}
	return NewProvider(Config{
		ProviderName: resolvedName,
		TokenFile:    tokenPath,
	})
}

func (p *Provider) Name() string { return p.name }

func (p *Provider) DisplayName() string { return p.displayName }

func (p *Provider) isDidaOpenAPI() bool {
	return p.name == "dida"
}

func (p *Provider) Authenticate(ctx context.Context, config map[string]interface{}) error {
	if config != nil {
		if v, ok := config["token"].(string); ok && strings.TrimSpace(v) != "" {
			p.config.Token = strings.TrimSpace(v)
		}
		if v, ok := config["api_token"].(string); ok && strings.TrimSpace(v) != "" {
			p.config.Token = strings.TrimSpace(v)
		}
		if v, ok := config["provider"].(string); ok && strings.TrimSpace(v) != "" {
			p.name = normalizeProviderName(v)
			baseURL, authBaseURL, source, displayName := providerProfile(p.name)
			p.client.SetBaseURLs(baseURL, authBaseURL)
			p.source = source
			p.displayName = displayName
			p.config.ProviderName = p.name
		}
		if v, ok := config["username"].(string); ok && strings.TrimSpace(v) != "" {
			p.config.Username = strings.TrimSpace(v)
		}
		if v, ok := config["password"].(string); ok && v != "" {
			p.config.Password = v
		}
	}

	p.client.SetCredentials(p.config.Username, p.config.Password)
	p.client.SetToken(p.config.Token)

	if p.client.IsAuthenticated() {
		var authErr error
		if p.isDidaOpenAPI() {
			_, authErr = p.client.OpenListProjects(ctx)
		} else {
			authErr = p.client.UserStatus(ctx)
		}
		if authErr == nil {
			if p.config.TokenFile != "" {
				_ = saveTokenStore(p.config.TokenFile, &tokenStore{
					Username: p.config.Username,
					Password: p.config.Password,
					Token:    p.client.Token(),
				})
			}
			return nil
		}
	}

	if strings.TrimSpace(p.config.Username) == "" || p.config.Password == "" {
		return fmt.Errorf("%s token 无效，请重新执行 'taskbridge auth login %s' 输入新的 token", p.displayName, p.name)
	}

	if _, err := p.client.SignOn(ctx); err != nil {
		return fmt.Errorf("ticktick signon failed: %w", err)
	}

	p.config.Token = p.client.Token()
	if p.config.TokenFile != "" {
		_ = saveTokenStore(p.config.TokenFile, &tokenStore{
			Username: p.config.Username,
			Password: p.config.Password,
			Token:    p.client.Token(),
		})
	}
	return nil
}

func (p *Provider) IsAuthenticated() bool {
	return p.client.IsAuthenticated()
}

func (p *Provider) RefreshToken(ctx context.Context) error {
	if strings.TrimSpace(p.config.Username) == "" || p.config.Password == "" {
		if p.client.IsAuthenticated() {
			var err error
			if p.isDidaOpenAPI() {
				_, err = p.client.OpenListProjects(ctx)
			} else {
				err = p.client.UserStatus(ctx)
			}
			if err == nil {
				return nil
			}
		}
		return fmt.Errorf("%s 为静态 token 模式，当前 token 无效，请重新执行 'taskbridge auth login %s'", p.displayName, p.name)
	}

	if _, err := p.client.SignOn(ctx); err != nil {
		return err
	}
	p.config.Token = p.client.Token()
	if p.config.TokenFile != "" {
		_ = saveTokenStore(p.config.TokenFile, &tokenStore{
			Username: p.config.Username,
			Password: p.config.Password,
			Token:    p.client.Token(),
		})
	}
	return nil
}

func (p *Provider) ListTaskLists(ctx context.Context) ([]model.TaskList, error) {
	if p.isDidaOpenAPI() {
		projects, err := p.client.OpenListProjects(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]model.TaskList, 0, len(projects))
		for _, proj := range projects {
			if strings.TrimSpace(proj.ID) == "" {
				continue
			}
			result = append(result, model.TaskList{
				ID:          proj.ID,
				Name:        proj.Name,
				Source:      p.source,
				SourceRawID: proj.ID,
			})
		}
		return result, nil
	}

	batch, err := p.client.GetBatch(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]model.TaskList, 0, len(batch.ProjectProfiles)+1)
	if strings.TrimSpace(batch.InboxID) != "" {
		result = append(result, model.TaskList{
			ID:          batch.InboxID,
			Name:        "Inbox",
			Source:      p.source,
			SourceRawID: batch.InboxID,
		})
	}

	for _, list := range batch.ProjectProfiles {
		if strings.TrimSpace(list.ID) == "" {
			continue
		}
		result = append(result, model.TaskList{
			ID:          list.ID,
			Name:        list.Name,
			Source:      p.source,
			SourceRawID: list.ID,
		})
	}
	return result, nil
}

func (p *Provider) CreateTaskList(ctx context.Context, name string) (*model.TaskList, error) {
	if p.isDidaOpenAPI() {
		created, err := p.client.OpenCreateProject(ctx, &OpenProjectCreateRequest{Name: name})
		if err != nil {
			return nil, err
		}
		return &model.TaskList{
			ID:          created.ID,
			Name:        created.Name,
			Source:      p.source,
			SourceRawID: created.ID,
		}, nil
	}

	_, err := p.client.BatchProject(ctx, &BatchProjectRequest{Add: []ProjectCreateV2{{Name: name}}})
	if err != nil {
		return nil, err
	}

	lists, err := p.ListTaskLists(ctx)
	if err != nil {
		return nil, err
	}
	for _, l := range lists {
		if l.Name == name {
			copied := l
			return &copied, nil
		}
	}
	return nil, fmt.Errorf("created project but cannot resolve new list id")
}

func (p *Provider) DeleteTaskList(ctx context.Context, listID string) error {
	if p.isDidaOpenAPI() {
		return p.client.OpenDeleteProject(ctx, listID)
	}
	_, err := p.client.BatchProject(ctx, &BatchProjectRequest{Delete: []string{listID}})
	return err
}

func (p *Provider) ListTasks(ctx context.Context, listID string, opts provider.ListOptions) ([]model.Task, error) {
	if p.isDidaOpenAPI() {
		lists := []string{listID}
		if strings.TrimSpace(listID) == "" {
			taskLists, err := p.ListTaskLists(ctx)
			if err != nil {
				return nil, err
			}
			lists = lists[:0]
			for _, l := range taskLists {
				lists = append(lists, l.ID)
			}
		}
		result := make([]model.Task, 0)
		for _, lid := range lists {
			data, err := p.client.OpenProjectData(ctx, lid)
			if err != nil {
				return nil, err
			}
			listName := data.Project.Name
			for _, t := range data.Tasks {
				mt := toModelOpenTask(t, listName, p.source)
				if opts.Completed != nil && (mt.Status == model.StatusCompleted) != *opts.Completed {
					continue
				}
				if opts.DueAfter != nil && mt.DueDate != nil && mt.DueDate.Before(*opts.DueAfter) {
					continue
				}
				if opts.DueBefore != nil && mt.DueDate != nil && mt.DueDate.After(*opts.DueBefore) {
					continue
				}
				if opts.UpdatedAfter != nil && mt.UpdatedAt.Before(*opts.UpdatedAfter) {
					continue
				}
				result = append(result, *mt)
			}
		}
		return result, nil
	}

	batch, err := p.client.GetBatch(ctx)
	if err != nil {
		return nil, err
	}

	listName := ""
	for _, l := range batch.ProjectProfiles {
		if l.ID == listID {
			listName = l.Name
			break
		}
	}
	if listName == "" && listID == batch.InboxID {
		listName = "Inbox"
	}

	result := make([]model.Task, 0)
	for _, t := range batch.SyncTaskBean.Update {
		if listID != "" && t.ProjectID != listID {
			continue
		}
		mt := toModelTask(t, listName, p.source)
		if opts.Completed != nil && (mt.Status == model.StatusCompleted) != *opts.Completed {
			continue
		}
		if opts.DueAfter != nil && mt.DueDate != nil && mt.DueDate.Before(*opts.DueAfter) {
			continue
		}
		if opts.DueBefore != nil && mt.DueDate != nil && mt.DueDate.After(*opts.DueBefore) {
			continue
		}
		if opts.UpdatedAfter != nil && mt.UpdatedAt.Before(*opts.UpdatedAfter) {
			continue
		}
		result = append(result, *mt)
	}
	return result, nil
}

func (p *Provider) GetTask(ctx context.Context, listID, taskID string) (*model.Task, error) {
	if p.isDidaOpenAPI() {
		if strings.TrimSpace(listID) != "" {
			t, err := p.client.OpenGetTask(ctx, listID, taskID)
			if err == nil {
				return toModelOpenTask(*t, "", p.source), nil
			}
		}
		tasks, err := p.ListTasks(ctx, "", provider.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, t := range tasks {
			if t.ID == taskID {
				copied := t
				return &copied, nil
			}
		}
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	tasks, err := p.ListTasks(ctx, listID, provider.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		if t.ID == taskID {
			copied := t
			return &copied, nil
		}
	}
	return nil, fmt.Errorf("task not found: %s", taskID)
}

func (p *Provider) SearchTasks(ctx context.Context, query string) ([]model.Task, error) {
	if p.isDidaOpenAPI() {
		tasks, err := p.ListTasks(ctx, "", provider.ListOptions{})
		if err != nil {
			return nil, err
		}
		q := strings.ToLower(strings.TrimSpace(query))
		if q == "" {
			return []model.Task{}, nil
		}
		result := make([]model.Task, 0)
		for _, t := range tasks {
			title := strings.ToLower(t.Title)
			content := strings.ToLower(t.Description)
			if strings.Contains(title, q) || strings.Contains(content, q) {
				result = append(result, t)
			}
		}
		return result, nil
	}

	batch, err := p.client.GetBatch(ctx)
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return []model.Task{}, nil
	}

	listMap := make(map[string]string, len(batch.ProjectProfiles)+1)
	if batch.InboxID != "" {
		listMap[batch.InboxID] = "Inbox"
	}
	for _, l := range batch.ProjectProfiles {
		listMap[l.ID] = l.Name
	}

	result := make([]model.Task, 0)
	for _, t := range batch.SyncTaskBean.Update {
		title := strings.ToLower(t.Title)
		content := strings.ToLower(t.Content + " " + t.Desc)
		if strings.Contains(title, q) || strings.Contains(content, q) {
			result = append(result, *toModelTask(t, listMap[t.ProjectID], p.source))
		}
	}
	return result, nil
}

func (p *Provider) CreateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error) {
	if p.isDidaOpenAPI() {
		if strings.TrimSpace(listID) == "" {
			lists, err := p.ListTaskLists(ctx)
			if err != nil {
				return nil, err
			}
			if len(lists) == 0 {
				return nil, fmt.Errorf("no task list found")
			}
			listID = lists[0].ID
		}
		req := OpenTaskCreateRequest{
			ProjectID: listID,
			Title:     task.Title,
			Content:   task.Description,
			Priority:  toTickTickPriority(task.Priority),
			Tags:      task.Tags,
		}
		if task.DueDate != nil {
			req.DueDate = task.DueDate.UTC().Format(time.RFC3339)
		}
		if task.StartDate != nil {
			req.StartDate = task.StartDate.UTC().Format(time.RFC3339)
		}
		created, err := p.client.OpenCreateTask(ctx, &req)
		if err != nil {
			return nil, err
		}
		return toModelOpenTask(*created, task.ListName, p.source), nil
	}

	payload := TaskCreateV2{
		ProjectID: listID,
		Title:     task.Title,
		Content:   task.Description,
		Priority:  toTickTickPriority(task.Priority),
		Tags:      task.Tags,
	}
	if task.DueDate != nil {
		payload.DueDate = task.DueDate.UTC().Format(time.RFC3339)
	}
	if task.StartDate != nil {
		payload.StartDate = task.StartDate.UTC().Format(time.RFC3339)
	}

	resp, err := p.client.BatchTask(ctx, &BatchTaskRequest{Add: []TaskCreateV2{payload}})
	if err != nil {
		return nil, err
	}

	for id := range resp.ID2ETag {
		created, err := p.GetTask(ctx, listID, id)
		if err == nil {
			return created, nil
		}
	}
	return nil, fmt.Errorf("task created but id not returned")
}

func (p *Provider) UpdateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error) {
	if p.isDidaOpenAPI() {
		status := 0
		if task.Status == model.StatusCompleted {
			status = 2
		}
		req := OpenTaskUpdateRequest{
			ProjectID: listID,
			Title:     task.Title,
			Content:   task.Description,
			Priority:  toTickTickPriority(task.Priority),
			Status:    status,
			Tags:      task.Tags,
		}
		if task.DueDate != nil {
			req.DueDate = task.DueDate.UTC().Format(time.RFC3339)
		}
		if task.StartDate != nil {
			req.StartDate = task.StartDate.UTC().Format(time.RFC3339)
		}
		updated, err := p.client.OpenUpdateTask(ctx, task.ID, &req)
		if err != nil {
			return nil, err
		}
		return toModelOpenTask(*updated, task.ListName, p.source), nil
	}

	status := 0
	if task.Status == model.StatusCompleted {
		status = 2
	}
	payload := TaskUpdateV2{
		ID:        task.ID,
		ProjectID: listID,
		Title:     task.Title,
		Content:   task.Description,
		Priority:  toTickTickPriority(task.Priority),
		Status:    status,
		Tags:      task.Tags,
	}
	if task.DueDate != nil {
		payload.DueDate = task.DueDate.UTC().Format(time.RFC3339)
	}
	if task.StartDate != nil {
		payload.StartDate = task.StartDate.UTC().Format(time.RFC3339)
	}

	if _, err := p.client.BatchTask(ctx, &BatchTaskRequest{Update: []TaskUpdateV2{payload}}); err != nil {
		return nil, err
	}

	return p.GetTask(ctx, listID, task.ID)
}

func (p *Provider) DeleteTask(ctx context.Context, listID, taskID string) error {
	if p.isDidaOpenAPI() {
		if strings.TrimSpace(listID) == "" {
			t, err := p.GetTask(ctx, "", taskID)
			if err != nil {
				return err
			}
			listID = t.ListID
		}
		return p.client.OpenDeleteTask(ctx, listID, taskID)
	}
	_, err := p.client.BatchTask(ctx, &BatchTaskRequest{Delete: []TaskDeleteV2{{TaskID: taskID, ProjectID: listID}}})
	return err
}

func (p *Provider) BatchCreate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error) {
	result := make([]model.Task, 0, len(tasks))
	for _, t := range tasks {
		created, err := p.CreateTask(ctx, listID, t)
		if err != nil {
			continue
		}
		result = append(result, *created)
	}
	return result, nil
}

func (p *Provider) BatchUpdate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error) {
	result := make([]model.Task, 0, len(tasks))
	for _, t := range tasks {
		updated, err := p.UpdateTask(ctx, listID, t)
		if err != nil {
			continue
		}
		result = append(result, *updated)
	}
	return result, nil
}

func (p *Provider) GetChanges(ctx context.Context, since time.Time) (*provider.SyncChanges, error) {
	if p.isDidaOpenAPI() {
		tasks, err := p.ListTasks(ctx, "", provider.ListOptions{})
		if err != nil {
			return nil, err
		}
		changes := &provider.SyncChanges{Tasks: make([]model.Task, 0), DeletedIDs: []string{}}
		for _, t := range tasks {
			if t.UpdatedAt.After(since) {
				changes.Tasks = append(changes.Tasks, t)
			}
		}
		return changes, nil
	}

	batch, err := p.client.GetBatch(ctx)
	if err != nil {
		return nil, err
	}
	changes := &provider.SyncChanges{Tasks: make([]model.Task, 0), DeletedIDs: []string{}}
	for _, t := range batch.SyncTaskBean.Update {
		mt := toModelTask(t, "", p.source)
		if mt.UpdatedAt.After(since) {
			changes.Tasks = append(changes.Tasks, *mt)
		}
	}
	return changes, nil
}

func (p *Provider) Capabilities() provider.Capabilities { return p.capabilities }

func (p *Provider) GetTokenInfo() *provider.TokenInfo {
	hasToken := p.client.IsAuthenticated()
	return &provider.TokenInfo{
		Provider:    p.Name(),
		HasToken:    hasToken,
		IsValid:     hasToken,
		Refreshable: false,
	}
}

func parseTickTickTime(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	layouts := []string{time.RFC3339, "2006-01-02T15:04:05.000-0700", "2006-01-02T15:04:05-0700"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			u := t.UTC()
			return &u
		}
	}
	return nil
}

func toModelTask(t TaskV2, listName string, source model.TaskSource) *model.Task {
	created := time.Now()
	if parsed := parseTickTickTime(t.StartDate); parsed != nil {
		created = *parsed
	}
	updated := created
	if parsed := parseTickTickTime(t.ModifiedTime); parsed != nil {
		updated = *parsed
	}

	status := model.StatusTodo
	if t.Status == 2 {
		status = model.StatusCompleted
	}

	m := &model.Task{
		ID:          t.ID,
		Title:       t.Title,
		Description: strings.TrimSpace(strings.TrimSpace(t.Content) + "\n" + strings.TrimSpace(t.Desc)),
		Status:      status,
		CreatedAt:   created,
		UpdatedAt:   updated,
		ListID:      t.ProjectID,
		ListName:    listName,
		Tags:        t.Tags,
		Priority:    toModelPriority(t.Priority),
		Source:      source,
		SourceRawID: t.ID,
	}
	if due := parseTickTickTime(t.DueDate); due != nil {
		m.DueDate = due
	}
	if start := parseTickTickTime(t.StartDate); start != nil {
		m.StartDate = start
	}
	if done := parseTickTickTime(t.CompletedTime); done != nil {
		m.CompletedAt = done
	}
	return m
}

func toModelOpenTask(t OpenTask, listName string, source model.TaskSource) *model.Task {
	now := time.Now().UTC()
	status := model.StatusTodo
	if t.Status == 2 {
		status = model.StatusCompleted
	}

	m := &model.Task{
		ID:          t.ID,
		Title:       t.Title,
		Description: strings.TrimSpace(strings.TrimSpace(t.Content) + "\n" + strings.TrimSpace(t.Desc)),
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
		ListID:      t.ProjectID,
		ListName:    listName,
		Tags:        t.Tags,
		Priority:    toModelPriority(t.Priority),
		Source:      source,
		SourceRawID: t.ID,
		ETag:        t.ETag,
	}
	if due := parseTickTickTime(t.DueDate); due != nil {
		m.DueDate = due
	}
	if start := parseTickTickTime(t.StartDate); start != nil {
		m.StartDate = start
		if m.CreatedAt.IsZero() {
			m.CreatedAt = *start
		}
	}
	if dt := parseTickTickTime(t.DateTime); dt != nil {
		if m.DueDate == nil {
			m.DueDate = dt
		}
	}
	if status == model.StatusCompleted {
		done := now
		m.CompletedAt = &done
	}
	return m
}

func toModelPriority(p int) model.Priority {
	switch p {
	case 5:
		return model.PriorityUrgent
	case 3:
		return model.PriorityHigh
	case 2:
		return model.PriorityMedium
	case 1:
		return model.PriorityLow
	default:
		return model.PriorityNone
	}
}

func toTickTickPriority(p model.Priority) int {
	switch p {
	case model.PriorityUrgent:
		return 5
	case model.PriorityHigh:
		return 3
	case model.PriorityMedium:
		return 2
	case model.PriorityLow:
		return 1
	default:
		return 0
	}
}

func saveTokenStore(path string, store *tokenStore) error {
	if store == nil {
		return nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func loadTokenStore(path string) (*tokenStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	var store tokenStore
	if err := json.Unmarshal(data, &store); err != nil {
		raw := strings.TrimSpace(string(data))
		if raw != "" && !strings.HasPrefix(raw, "{") {
			store.Token = raw
			return &store, nil
		}
		return nil, err
	}
	if strings.TrimSpace(store.Token) == "" && strings.TrimSpace(store.APIToken) != "" {
		store.Token = strings.TrimSpace(store.APIToken)
	}
	return &store, nil
}

func normalizeProviderName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "dida", "ticktick_cn", "tick-cn":
		return "dida"
	default:
		return "ticktick"
	}
}

func providerProfile(providerName string) (baseURL, authBaseURL string, source model.TaskSource, displayName string) {
	switch normalizeProviderName(providerName) {
	case "dida":
		return didaV2BaseURL, didaAuthBaseURL, model.SourceDida, "Dida365"
	default:
		return defaultV2BaseURL, defaultAuthBaseURL, model.SourceTickTick, "TickTick"
	}
}
