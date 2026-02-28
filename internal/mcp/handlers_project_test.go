package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/project"
	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/internal/storage"
	"github.com/yeisme/taskbridge/internal/storage/filestore"
)

type mockProvider struct {
	created  []model.Task
	taskList []model.TaskList
}

func (m *mockProvider) Name() string        { return "google" }
func (m *mockProvider) DisplayName() string { return "Google Tasks" }
func (m *mockProvider) Authenticate(ctx context.Context, config map[string]interface{}) error {
	return nil
}
func (m *mockProvider) IsAuthenticated() bool                  { return true }
func (m *mockProvider) RefreshToken(ctx context.Context) error { return nil }
func (m *mockProvider) ListTaskLists(ctx context.Context) ([]model.TaskList, error) {
	if len(m.taskList) == 0 {
		return []model.TaskList{{ID: "@default", Name: "My Tasks", Source: model.SourceGoogle}}, nil
	}
	return m.taskList, nil
}
func (m *mockProvider) CreateTaskList(ctx context.Context, name string) (*model.TaskList, error) {
	return nil, errors.New("not implemented")
}
func (m *mockProvider) DeleteTaskList(ctx context.Context, listID string) error { return nil }
func (m *mockProvider) ListTasks(ctx context.Context, listID string, opts provider.ListOptions) ([]model.Task, error) {
	return []model.Task{}, nil
}
func (m *mockProvider) GetTask(ctx context.Context, listID, taskID string) (*model.Task, error) {
	return nil, errors.New("not found")
}
func (m *mockProvider) SearchTasks(ctx context.Context, query string) ([]model.Task, error) {
	return []model.Task{}, nil
}
func (m *mockProvider) CreateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error) {
	cp := *task
	cp.SourceRawID = "remote_" + task.ID
	m.created = append(m.created, cp)
	return &cp, nil
}
func (m *mockProvider) UpdateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error) {
	cp := *task
	return &cp, nil
}
func (m *mockProvider) DeleteTask(ctx context.Context, listID, taskID string) error { return nil }
func (m *mockProvider) BatchCreate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error) {
	return nil, nil
}
func (m *mockProvider) BatchUpdate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error) {
	return nil, nil
}
func (m *mockProvider) GetChanges(ctx context.Context, since time.Time) (*provider.SyncChanges, error) {
	return &provider.SyncChanges{}, nil
}
func (m *mockProvider) Capabilities() provider.Capabilities { return provider.Capabilities{} }
func (m *mockProvider) GetTokenInfo() *provider.TokenInfo {
	return &provider.TokenInfo{Provider: "google", HasToken: true, IsValid: true}
}

func TestProjectHandlersLifecycle(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()

	taskStore, err := filestore.New(tmp, "json")
	if err != nil {
		t.Fatalf("new task store: %v", err)
	}
	projectStore, err := project.NewFileStore(tmp)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}
	providerMock := &mockProvider{}

	s := &Server{
		taskStore:    taskStore,
		projectStore: projectStore,
		providers: map[string]provider.Provider{
			"google": providerMock,
		},
	}

	createRes, err := s.handleCreateProject(ctx, buildCallToolRequest(t, map[string]interface{}{
		"name":      "学习 openclaw",
		"goal_text": "我希望学习 openclaw",
	}))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	created := parseJSONResult(t, createRes)
	projectID := created["id"].(string)

	splitRes, err := s.handleSplitProject(ctx, buildCallToolRequest(t, map[string]interface{}{
		"project_id": projectID,
		"constraints": map[string]interface{}{
			"require_deliverable":  true,
			"min_estimate_minutes": 40,
			"max_estimate_minutes": 120,
			"min_tasks":            8,
			"max_tasks":            10,
			"min_practice_tasks":   2,
		},
	}))
	if err != nil {
		t.Fatalf("split project: %v", err)
	}
	split := parseJSONResult(t, splitRes)
	planID := split["plan_id"].(string)
	if planID == "" {
		t.Fatalf("empty plan_id")
	}
	constraints, ok := split["constraints"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing constraints in split response")
	}
	if constraints["require_deliverable"] != true {
		t.Fatalf("unexpected constraints: %#v", constraints)
	}

	tasksBeforeConfirm, err := taskStore.ListTasks(ctx, storage.ListOptions{})
	if err != nil {
		t.Fatalf("list tasks before confirm: %v", err)
	}
	if len(tasksBeforeConfirm) != 0 {
		t.Fatalf("expected 0 tasks before confirm, got %d", len(tasksBeforeConfirm))
	}

	confirmRes, err := s.handleConfirmProject(ctx, buildCallToolRequest(t, map[string]interface{}{
		"project_id": projectID,
		"plan_id":    planID,
	}))
	if err != nil {
		t.Fatalf("confirm project: %v", err)
	}
	confirmed := parseJSONResult(t, confirmRes)
	if int(confirmed["count"].(float64)) == 0 {
		t.Fatalf("expected confirmed task count > 0")
	}

	tasks, err := taskStore.ListTasks(ctx, storage.ListOptions{})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatalf("expected tasks after confirm")
	}
	for _, task := range tasks {
		if task.Metadata == nil || task.Metadata.CustomFields == nil {
			t.Fatalf("task metadata missing")
		}
		if task.Metadata.CustomFields["tb_project_id"] != projectID {
			t.Fatalf("task project id mismatch: %v", task.Metadata.CustomFields["tb_project_id"])
		}
	}

	_ = taskStore.SaveTask(ctx, &model.Task{ID: "extra", Title: "extra", Status: model.StatusTodo, Source: model.SourceLocal, CreatedAt: time.Now(), UpdatedAt: time.Now()})

	syncRes, err := s.handleSyncProject(ctx, buildCallToolRequest(t, map[string]interface{}{
		"project_id": projectID,
		"provider":   "google",
	}))
	if err != nil {
		t.Fatalf("sync project: %v", err)
	}
	synced := parseJSONResult(t, syncRes)
	if synced["status"].(string) != "synced" {
		t.Fatalf("unexpected status: %v", synced["status"])
	}
	if len(providerMock.created) != len(tasks) {
		t.Fatalf("expected synced tasks=%d, got %d", len(tasks), len(providerMock.created))
	}

	listRes, err := s.handleListProjects(ctx, buildCallToolRequest(t, map[string]interface{}{"status": "synced"}))
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	list := parseJSONArrayResult(t, listRes)
	if len(list) != 1 {
		t.Fatalf("expected 1 synced project, got %d", len(list))
	}
}

func TestSplitProjectFromMarkdownLifecycle(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()

	taskStore, err := filestore.New(tmp, "json")
	if err != nil {
		t.Fatalf("new task store: %v", err)
	}
	projectStore, err := project.NewFileStore(tmp)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	s := &Server{
		taskStore:    taskStore,
		projectStore: projectStore,
		providers:    map[string]provider.Provider{},
	}

	createRes, err := s.handleCreateProject(ctx, buildCallToolRequest(t, map[string]interface{}{
		"name":      "学习 k8s",
		"goal_text": "学习 k8s",
	}))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	created := parseJSONResult(t, createRes)
	projectID := created["id"].(string)

	markdown := `- 学习k8s
      - 缩进跳级节点
  - 1. 学习 pods
  - 2. 控制器
这里是说明文本
`
	// 覆盖点：缩进跳级 warning + 非列表行 ignored + 叶子任务提取。
	splitRes, err := s.handleSplitProjectFromMarkdown(ctx, buildCallToolRequest(t, map[string]interface{}{
		"project_id": projectID,
		"markdown":   markdown,
		"horizon_days": 10,
	}))
	if err != nil {
		t.Fatalf("split markdown: %v", err)
	}

	split := parseJSONResult(t, splitRes)
	planID := split["plan_id"].(string)
	if planID == "" {
		t.Fatalf("empty plan_id")
	}

	stats, ok := split["stats"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing stats")
	}
	if int(stats["leaf_tasks"].(float64)) != 3 {
		t.Fatalf("unexpected leaf_tasks: %v", stats["leaf_tasks"])
	}
	if int(stats["ignored_lines"].(float64)) == 0 {
		t.Fatalf("expected ignored_lines > 0")
	}

	taskIDs := extractPlanTaskIDs(t, split["tasks_preview"])
	if len(taskIDs) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(taskIDs))
	}
	for _, id := range taskIDs {
		if len(id) == 0 || id[:4] != "ptk_" {
			t.Fatalf("unexpected plan task id: %s", id)
		}
	}

	splitResAgain, err := s.handleSplitProjectFromMarkdown(ctx, buildCallToolRequest(t, map[string]interface{}{
		"project_id": projectID,
		"markdown":   markdown,
		"horizon_days": 10,
	}))
	if err != nil {
		t.Fatalf("split markdown again: %v", err)
	}
	splitAgain := parseJSONResult(t, splitResAgain)
	taskIDsAgain := extractPlanTaskIDs(t, splitAgain["tasks_preview"])
	// 同输入应得到同一组稳定 plan_task_id。
	if !reflect.DeepEqual(taskIDs, taskIDsAgain) {
		t.Fatalf("plan task ids not stable\nfirst:  %#v\nsecond: %#v", taskIDs, taskIDsAgain)
	}

	confirmRes, err := s.handleConfirmProject(ctx, buildCallToolRequest(t, map[string]interface{}{
		"project_id": projectID,
		"plan_id":    planID,
	}))
	if err != nil {
		t.Fatalf("confirm project: %v", err)
	}
	confirmed := parseJSONResult(t, confirmRes)
	if int(confirmed["count"].(float64)) != 4 {
		t.Fatalf("unexpected confirmed task count: %v", confirmed["count"])
	}

	tasks, err := taskStore.ListTasks(ctx, storage.ListOptions{})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	hasChild := false
	for _, task := range tasks {
		if task.Metadata == nil || task.Metadata.CustomFields == nil {
			t.Fatalf("missing task metadata")
		}
		// confirm_project 需要把 plan_task_id 透传进任务 metadata。
		if _, ok := task.Metadata.CustomFields["tb_plan_task_id"]; !ok {
			t.Fatalf("missing tb_plan_task_id in metadata")
		}
		if task.ParentID != nil && *task.ParentID != "" {
			hasChild = true
			if _, ok := task.Metadata.CustomFields["tb_parent_plan_task_id"]; !ok {
				t.Fatalf("missing tb_parent_plan_task_id in metadata for child task")
			}
		}
	}
	if !hasChild {
		t.Fatalf("expected at least one child task")
	}
}

func TestConfirmProjectBackwardCompatibleWithoutPlanTaskID(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()

	taskStore, err := filestore.New(tmp, "json")
	if err != nil {
		t.Fatalf("new task store: %v", err)
	}
	projectStore, err := project.NewFileStore(tmp)
	if err != nil {
		t.Fatalf("new project store: %v", err)
	}

	s := &Server{
		taskStore:    taskStore,
		projectStore: projectStore,
		providers:    map[string]provider.Provider{},
	}

	createRes, err := s.handleCreateProject(ctx, buildCallToolRequest(t, map[string]interface{}{
		"name": "legacy plan",
	}))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	projectID := parseJSONResult(t, createRes)["id"].(string)

	splitRes, err := s.handleSplitProject(ctx, buildCallToolRequest(t, map[string]interface{}{
		"project_id": projectID,
	}))
	if err != nil {
		t.Fatalf("split project: %v", err)
	}
	planID := parseJSONResult(t, splitRes)["plan_id"].(string)

	// 旧 split_project 产物没有 plan_task_id，也应可正常 confirm。
	if _, err := s.handleConfirmProject(ctx, buildCallToolRequest(t, map[string]interface{}{
		"project_id": projectID,
		"plan_id":    planID,
	})); err != nil {
		t.Fatalf("confirm legacy plan: %v", err)
	}
}

func buildCallToolRequest(t *testing.T, payload map[string]interface{}) *sdkmcp.CallToolRequest {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{Arguments: body},
	}
}

func parseJSONResult(t *testing.T, result *sdkmcp.CallToolResult) map[string]interface{} {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatalf("empty result content")
	}
	text, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("result content is not text")
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return out
}

func parseJSONArrayResult(t *testing.T, result *sdkmcp.CallToolResult) []map[string]interface{} {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatalf("empty result content")
	}
	text, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("result content is not text")
	}
	var out []map[string]interface{}
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("unmarshal array result: %v", err)
	}
	return out
}

func extractPlanTaskIDs(t *testing.T, raw interface{}) []string {
	t.Helper()
	items, ok := raw.([]interface{})
	if !ok {
		t.Fatalf("tasks_preview is not an array: %#v", raw)
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("task item is not an object: %#v", item)
		}
		id, _ := m["id"].(string)
		ids = append(ids, id)
	}
	return ids
}
