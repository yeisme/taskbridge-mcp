// Package sync 提供任务同步功能
package sync

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/internal/storage"
)

// MockProvider 模拟 Provider
type MockProvider struct {
	name          string
	authenticated bool
	taskLists     []model.TaskList
	tasks         map[string][]model.Task // key: listID
	mu            sync.Mutex
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) DisplayName() string {
	return m.name
}

func (m *MockProvider) Authenticate(ctx context.Context, config map[string]interface{}) error {
	m.authenticated = true
	return nil
}

func (m *MockProvider) IsAuthenticated() bool {
	return m.authenticated
}

func (m *MockProvider) RefreshToken(ctx context.Context) error {
	return nil
}

func (m *MockProvider) ListTaskLists(ctx context.Context) ([]model.TaskList, error) {
	return m.taskLists, nil
}

func (m *MockProvider) CreateTaskList(ctx context.Context, name string) (*model.TaskList, error) {
	list := &model.TaskList{
		ID:     "list_" + name,
		Name:   name,
		Source: model.TaskSource(m.name),
	}
	m.taskLists = append(m.taskLists, *list)
	return list, nil
}

func (m *MockProvider) DeleteTaskList(ctx context.Context, listID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, list := range m.taskLists {
		if list.ID == listID {
			m.taskLists = append(m.taskLists[:i], m.taskLists[i+1:]...)
			break
		}
	}
	return nil
}

func (m *MockProvider) ListTasks(ctx context.Context, listID string, opts provider.ListOptions) ([]model.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if tasks, ok := m.tasks[listID]; ok {
		return tasks, nil
	}
	return []model.Task{}, nil
}

func (m *MockProvider) GetTask(ctx context.Context, listID, taskID string) (*model.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if tasks, ok := m.tasks[listID]; ok {
		for _, task := range tasks {
			if task.ID == taskID {
				return &task, nil
			}
		}
	}
	return nil, errors.New("task not found")
}

func (m *MockProvider) SearchTasks(ctx context.Context, query string) ([]model.Task, error) {
	return []model.Task{}, nil
}

func (m *MockProvider) CreateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tasks == nil {
		m.tasks = make(map[string][]model.Task)
	}
	task.ID = "task_" + task.Title
	task.SourceRawID = task.ID
	task.Source = model.TaskSource(m.name)
	m.tasks[listID] = append(m.tasks[listID], *task)
	return task, nil
}

func (m *MockProvider) UpdateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if tasks, ok := m.tasks[listID]; ok {
		for i, t := range tasks {
			if t.ID == task.ID {
				m.tasks[listID][i] = *task
				return task, nil
			}
		}
	}
	return nil, errors.New("task not found")
}

func (m *MockProvider) DeleteTask(ctx context.Context, listID, taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if tasks, ok := m.tasks[listID]; ok {
		for i, t := range tasks {
			if t.ID == taskID {
				m.tasks[listID] = append(tasks[:i], tasks[i+1:]...)
				break
			}
		}
	}
	return nil
}

func (m *MockProvider) BatchCreate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error) {
	result := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		created, err := m.CreateTask(ctx, listID, task)
		if err != nil {
			continue
		}
		result = append(result, *created)
	}
	return result, nil
}

func (m *MockProvider) BatchUpdate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error) {
	result := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		updated, err := m.UpdateTask(ctx, listID, task)
		if err != nil {
			continue
		}
		result = append(result, *updated)
	}
	return result, nil
}

func (m *MockProvider) GetChanges(ctx context.Context, since time.Time) (*provider.SyncChanges, error) {
	return &provider.SyncChanges{}, nil
}

func (m *MockProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsDueDate:   true,
		SupportsPriority:  true,
		SupportsBatch:     true,
		SupportsDeltaSync: true,
	}
}

func (m *MockProvider) GetTokenInfo() *provider.TokenInfo {
	return &provider.TokenInfo{
		Provider: m.name,
		HasToken: m.authenticated,
		IsValid:  m.authenticated,
	}
}

// MockStorage 模拟 Storage
type MockStorage struct {
	taskLists    map[string]*model.TaskList
	tasks        map[string]*model.Task
	lastSyncTime map[string]time.Time
	mu           sync.RWMutex
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		taskLists:    make(map[string]*model.TaskList),
		tasks:        make(map[string]*model.Task),
		lastSyncTime: make(map[string]time.Time),
	}
}

func (s *MockStorage) SaveTaskList(ctx context.Context, list *model.TaskList) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskLists[list.ID] = list
	return nil
}

func (s *MockStorage) GetTaskList(ctx context.Context, id string) (*model.TaskList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if list, ok := s.taskLists[id]; ok {
		return list, nil
	}
	return nil, errors.New("not found")
}

func (s *MockStorage) ListTaskLists(ctx context.Context) ([]model.TaskList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.TaskList, 0, len(s.taskLists))
	for _, list := range s.taskLists {
		result = append(result, *list)
	}
	return result, nil
}

func (s *MockStorage) DeleteTaskList(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.taskLists, id)
	return nil
}

func (s *MockStorage) SaveTask(ctx context.Context, task *model.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
	return nil
}

func (s *MockStorage) GetTask(ctx context.Context, id string) (*model.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if task, ok := s.tasks[id]; ok {
		return task, nil
	}
	return nil, errors.New("not found")
}

func (s *MockStorage) ListTasks(ctx context.Context, opts storage.ListOptions) ([]model.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		result = append(result, *task)
	}
	return result, nil
}

func (s *MockStorage) DeleteTask(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tasks, id)
	return nil
}

func (s *MockStorage) SetLastSyncTime(ctx context.Context, source model.TaskSource, t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSyncTime[string(source)] = t
	return nil
}

func (s *MockStorage) GetLastSyncTime(ctx context.Context, source model.TaskSource) (*time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if t, ok := s.lastSyncTime[string(source)]; ok {
		return &t, nil
	}
	return nil, nil
}

func (s *MockStorage) SaveTasks(ctx context.Context, tasks []*model.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, task := range tasks {
		s.tasks[task.ID] = task
	}
	return nil
}

func (s *MockStorage) QueryTasks(ctx context.Context, query storage.Query) ([]model.Task, error) {
	return s.ListTasks(ctx, storage.ListOptions{})
}

func (s *MockStorage) ExportToJSON(ctx context.Context, opts storage.ExportOptions) ([]byte, error) {
	return []byte("[]"), nil
}

func (s *MockStorage) ExportToMarkdown(ctx context.Context, opts storage.ExportOptions) ([]byte, error) {
	return []byte(""), nil
}

// TestNewEngine 测试创建同步引擎
func TestNewEngine(t *testing.T) {
	providers := map[string]provider.Provider{
		"mock": &MockProvider{name: "mock"},
	}
	store := NewMockStorage()

	engine := NewEngine(providers, store)

	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}
}

// TestSyncPull 测试拉取同步
func TestSyncPull(t *testing.T) {
	mockProvider := &MockProvider{
		name:          "mock",
		authenticated: true,
		taskLists: []model.TaskList{
			{ID: "list1", Name: "List 1", Source: "mock"},
		},
		tasks: map[string][]model.Task{
			"list1": {
				{ID: "task1", Title: "Task 1", Status: model.StatusTodo, Source: "mock"},
				{ID: "task2", Title: "Task 2", Status: model.StatusInProgress, Source: "mock"},
			},
		},
	}

	providers := map[string]provider.Provider{
		"mock": mockProvider,
	}
	store := NewMockStorage()

	engine := NewEngine(providers, store)

	opts := Options{
		Direction: DirectionPull,
		Provider:  "mock",
	}

	result, err := engine.Sync(context.Background(), opts)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Pulled != 2 {
		t.Errorf("Expected 2 pulled tasks, got %d", result.Pulled)
	}

	// 验证任务已保存到存储
	tasks, _ := store.ListTasks(context.Background(), storage.ListOptions{})
	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks in storage, got %d", len(tasks))
	}
}

// TestSyncPullSkipUnchanged 测试重复拉取时跳过未变更任务
func TestSyncPullSkipUnchanged(t *testing.T) {
	mockProvider := &MockProvider{
		name:          "mock",
		authenticated: true,
		taskLists: []model.TaskList{
			{ID: "list1", Name: "List 1", Source: "mock"},
		},
		tasks: map[string][]model.Task{
			"list1": {
				{
					ID:          "task1",
					SourceRawID: "task1",
					Title:       "Task 1",
					Status:      model.StatusTodo,
					Source:      "mock",
					ListID:      "list1",
					ListName:    "List 1",
				},
				{
					ID:          "task2",
					SourceRawID: "task2",
					Title:       "Task 2",
					Status:      model.StatusInProgress,
					Source:      "mock",
					ListID:      "list1",
					ListName:    "List 1",
				},
			},
		},
	}

	providers := map[string]provider.Provider{
		"mock": mockProvider,
	}
	store := NewMockStorage()
	engine := NewEngine(providers, store)

	opts := Options{
		Direction: DirectionPull,
		Provider:  "mock",
	}

	first, err := engine.Sync(context.Background(), opts)
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}
	if first.Pulled != 2 {
		t.Fatalf("expected first pulled=2, got %d", first.Pulled)
	}

	second, err := engine.Sync(context.Background(), opts)
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}
	if second.Pulled != 0 {
		t.Fatalf("expected second pulled=0, got %d", second.Pulled)
	}
	if second.Skipped != 2 {
		t.Fatalf("expected second skipped=2, got %d", second.Skipped)
	}
}

// TestSyncPush 测试推送同步
func TestSyncPush(t *testing.T) {
	mockProvider := &MockProvider{
		name:          "mock",
		authenticated: true,
		taskLists: []model.TaskList{
			{ID: "list1", Name: "我的任务", Source: "mock"},
		},
		tasks: make(map[string][]model.Task),
	}

	providers := map[string]provider.Provider{
		"mock": mockProvider,
	}
	store := NewMockStorage()

	// 添加本地任务
	if err := store.SaveTask(context.Background(), &model.Task{
		ID:        "local1",
		Title:     "Local Task 1",
		Status:    model.StatusTodo,
		Source:    "local",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("failed to seed local task: %v", err)
	}

	engine := NewEngine(providers, store)

	opts := Options{
		Direction: DirectionPush,
		Provider:  "mock",
	}

	result, err := engine.Sync(context.Background(), opts)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Pushed != 1 {
		t.Errorf("Expected 1 pushed task, got %d", result.Pushed)
	}
}

// TestSyncBidirectional 测试双向同步
func TestSyncBidirectional(t *testing.T) {
	mockProvider := &MockProvider{
		name:          "mock",
		authenticated: true,
		taskLists: []model.TaskList{
			{ID: "list1", Name: "我的任务", Source: "mock"},
		},
		tasks: map[string][]model.Task{
			"list1": {
				{ID: "remote1", Title: "Remote Task", Status: model.StatusTodo, Source: "mock"},
			},
		},
	}

	providers := map[string]provider.Provider{
		"mock": mockProvider,
	}
	store := NewMockStorage()

	// 添加本地任务
	if err := store.SaveTask(context.Background(), &model.Task{
		ID:        "local1",
		Title:     "Local Task",
		Status:    model.StatusTodo,
		Source:    "local",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("failed to seed local task: %v", err)
	}

	engine := NewEngine(providers, store)

	opts := Options{
		Direction: DirectionBidirectional,
		Provider:  "mock",
	}

	result, err := engine.Sync(context.Background(), opts)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// 双向同步应该拉取和推送
	if result.Pulled != 1 {
		t.Errorf("Expected 1 pulled task, got %d", result.Pulled)
	}

	// 新行为：pull 阶段会跳过未变化任务，push 阶段可能不再重复推送。
	if result.Pushed != 0 {
		t.Errorf("Expected 0 pushed task, got %d", result.Pushed)
	}
}

// TestSyncNotAuthenticated 测试未认证时的同步
func TestSyncNotAuthenticated(t *testing.T) {
	mockProvider := &MockProvider{
		name:          "mock",
		authenticated: false,
	}

	providers := map[string]provider.Provider{
		"mock": mockProvider,
	}
	store := NewMockStorage()

	engine := NewEngine(providers, store)

	opts := Options{
		Direction: DirectionPull,
		Provider:  "mock",
	}

	_, err := engine.Sync(context.Background(), opts)
	if err == nil {
		t.Error("Expected error for unauthenticated provider")
	}
}

// TestSyncProviderNotFound 测试 Provider 不存在
func TestSyncProviderNotFound(t *testing.T) {
	providers := map[string]provider.Provider{}
	store := NewMockStorage()

	engine := NewEngine(providers, store)

	opts := Options{
		Direction: DirectionPull,
		Provider:  "nonexistent",
	}

	_, err := engine.Sync(context.Background(), opts)
	if err == nil {
		t.Error("Expected error for non-existent provider")
	}
}

// TestSyncAll 测试同步所有 Provider
func TestSyncAll(t *testing.T) {
	providers := map[string]provider.Provider{
		"mock1": &MockProvider{
			name:          "mock1",
			authenticated: true,
			taskLists:     []model.TaskList{{ID: "list1", Name: "List 1"}},
			tasks: map[string][]model.Task{
				"list1": {{ID: "task1", Title: "Task 1"}},
			},
		},
		"mock2": &MockProvider{
			name:          "mock2",
			authenticated: true,
			taskLists:     []model.TaskList{{ID: "list2", Name: "List 2"}},
			tasks: map[string][]model.Task{
				"list2": {{ID: "task2", Title: "Task 2"}},
			},
		},
		"not_auth": &MockProvider{
			name:          "not_auth",
			authenticated: false,
		},
	}
	store := NewMockStorage()

	engine := NewEngine(providers, store)

	opts := Options{
		Direction: DirectionPull,
	}

	results, err := engine.SyncAll(context.Background(), opts)
	if err != nil {
		t.Fatalf("SyncAll failed: %v", err)
	}

	// 只有认证的 Provider 应该被同步
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

// TestGetStatus 测试获取同步状态
func TestGetStatus(t *testing.T) {
	mockProvider := &MockProvider{
		name:          "mock",
		authenticated: true,
	}

	providers := map[string]provider.Provider{
		"mock": mockProvider,
	}
	store := NewMockStorage()
	if err := store.SetLastSyncTime(context.Background(), "mock", time.Now()); err != nil {
		t.Fatalf("failed to set last sync time: %v", err)
	}

	engine := NewEngine(providers, store)

	status, err := engine.GetStatus(context.Background(), "mock")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if status.Provider != "mock" {
		t.Errorf("Expected provider 'mock', got '%s'", status.Provider)
	}

	if !status.Authenticated {
		t.Error("Expected Authenticated to be true")
	}

	if status.LastSyncTime.IsZero() {
		t.Error("Expected LastSyncTime to be set")
	}
}

// TestDryRun 测试 DryRun 模式
func TestDryRun(t *testing.T) {
	mockProvider := &MockProvider{
		name:          "mock",
		authenticated: true,
		taskLists: []model.TaskList{
			{ID: "list1", Name: "List 1"},
		},
		tasks: map[string][]model.Task{
			"list1": {{ID: "task1", Title: "Task 1"}},
		},
	}

	providers := map[string]provider.Provider{
		"mock": mockProvider,
	}
	store := NewMockStorage()

	engine := NewEngine(providers, store)

	opts := Options{
		Direction: DirectionPull,
		Provider:  "mock",
		DryRun:    true,
	}

	result, err := engine.Sync(context.Background(), opts)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if result.Pulled != 1 {
		t.Errorf("Expected 1 pulled in dry run, got %d", result.Pulled)
	}

	// DryRun 不应该实际保存任务
	tasks, _ := store.ListTasks(context.Background(), storage.ListOptions{})
	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks in storage (dry run), got %d", len(tasks))
	}
}

// TestResult 测试同步结果
func TestResult(t *testing.T) {
	result := &Result{
		Provider:     "test",
		Direction:    DirectionPull,
		Pulled:       10,
		Pushed:       5,
		Updated:      3,
		Deleted:      2,
		Skipped:      1,
		Duration:     100 * time.Millisecond,
		LastSyncTime: time.Now(),
		Errors: []Error{
			{TaskID: "task1", Operation: "create", Error: "some error"},
		},
	}

	if result.Provider != "test" {
		t.Errorf("Expected provider 'test', got '%s'", result.Provider)
	}

	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}
}

// TestOptions 测试同步选项
func TestOptions(t *testing.T) {
	opts := Options{
		Direction:       DirectionBidirectional,
		Provider:        "test",
		DryRun:          true,
		Force:           true,
		ConflictResolve: "local",
		DeleteRemote:    true,
	}

	if opts.Direction != DirectionBidirectional {
		t.Errorf("Expected Direction '%s', got '%s'", DirectionBidirectional, opts.Direction)
	}

	if !opts.DryRun {
		t.Error("Expected DryRun to be true")
	}

	if !opts.Force {
		t.Error("Expected Force to be true")
	}

	if opts.ConflictResolve != "local" {
		t.Errorf("Expected ConflictResolve 'local', got '%s'", opts.ConflictResolve)
	}
}

// TestDirection 测试同步方向
func TestDirection(t *testing.T) {
	if DirectionPull != "pull" {
		t.Errorf("Expected DirectionPull 'pull', got '%s'", DirectionPull)
	}

	if DirectionPush != "push" {
		t.Errorf("Expected DirectionPush 'push', got '%s'", DirectionPush)
	}

	if DirectionBidirectional != "bidirectional" {
		t.Errorf("Expected DirectionBidirectional 'bidirectional', got '%s'", DirectionBidirectional)
	}
}
