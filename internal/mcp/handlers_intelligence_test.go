package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/internal/storage/filestore"
	pkgconfig "github.com/yeisme/taskbridge/pkg/config"
)

type capabilityProvider struct {
	name string
	caps provider.Capabilities
}

func (p *capabilityProvider) Name() string        { return p.name }
func (p *capabilityProvider) DisplayName() string { return p.name }
func (p *capabilityProvider) Authenticate(ctx context.Context, config map[string]interface{}) error {
	_ = ctx
	_ = config
	return nil
}
func (p *capabilityProvider) IsAuthenticated() bool                  { return true }
func (p *capabilityProvider) RefreshToken(ctx context.Context) error { _ = ctx; return nil }
func (p *capabilityProvider) ListTaskLists(ctx context.Context) ([]model.TaskList, error) {
	_ = ctx
	return []model.TaskList{{ID: "@default", Name: "My Tasks", Source: model.TaskSource(p.name)}}, nil
}
func (p *capabilityProvider) CreateTaskList(ctx context.Context, name string) (*model.TaskList, error) {
	_ = ctx
	_ = name
	return nil, fmt.Errorf("not implemented")
}
func (p *capabilityProvider) DeleteTaskList(ctx context.Context, listID string) error {
	_ = ctx
	_ = listID
	return nil
}
func (p *capabilityProvider) ListTasks(ctx context.Context, listID string, opts provider.ListOptions) ([]model.Task, error) {
	_ = ctx
	_ = listID
	_ = opts
	return []model.Task{}, nil
}
func (p *capabilityProvider) GetTask(ctx context.Context, listID, taskID string) (*model.Task, error) {
	_ = ctx
	_ = listID
	_ = taskID
	return nil, fmt.Errorf("not found")
}
func (p *capabilityProvider) SearchTasks(ctx context.Context, query string) ([]model.Task, error) {
	_ = ctx
	_ = query
	return []model.Task{}, nil
}
func (p *capabilityProvider) CreateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error) {
	_ = ctx
	_ = listID
	cp := *task
	return &cp, nil
}
func (p *capabilityProvider) UpdateTask(ctx context.Context, listID string, task *model.Task) (*model.Task, error) {
	_ = ctx
	_ = listID
	cp := *task
	return &cp, nil
}
func (p *capabilityProvider) DeleteTask(ctx context.Context, listID, taskID string) error {
	_ = ctx
	_ = listID
	_ = taskID
	return nil
}
func (p *capabilityProvider) BatchCreate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error) {
	_ = ctx
	_ = listID
	out := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, *task)
	}
	return out, nil
}
func (p *capabilityProvider) BatchUpdate(ctx context.Context, listID string, tasks []*model.Task) ([]model.Task, error) {
	return p.BatchCreate(ctx, listID, tasks)
}
func (p *capabilityProvider) GetChanges(ctx context.Context, since time.Time) (*provider.SyncChanges, error) {
	_ = ctx
	_ = since
	return &provider.SyncChanges{}, nil
}
func (p *capabilityProvider) Capabilities() provider.Capabilities { return p.caps }
func (p *capabilityProvider) GetTokenInfo() *provider.TokenInfo {
	return &provider.TokenInfo{Provider: p.name, HasToken: true, IsValid: true}
}

func newIntelligenceTestServer(t *testing.T, cfg pkgconfig.IntelligenceConfig, providers map[string]provider.Provider) (*Server, *filestore.FileStorage, context.Context) {
	t.Helper()
	tmp := t.TempDir()
	store, err := filestore.New(tmp, "json")
	if err != nil {
		t.Fatalf("new task store: %v", err)
	}
	ctx := context.Background()
	if providers == nil {
		providers = map[string]provider.Provider{}
	}
	server := &Server{
		taskStore:          store,
		providers:          providers,
		intelligenceConfig: &cfg,
	}
	return server, store, ctx
}

func TestHandleAnalyzeOverdueHealth(t *testing.T) {
	cfg := pkgconfig.DefaultConfig().MCP.Intelligence
	cfg.Overdue.WarningThreshold = 0
	cfg.Overdue.OverloadThreshold = 0

	s, store, ctx := newIntelligenceTestServer(t, cfg, nil)

	now := time.Now()
	due := now.AddDate(0, 0, -2)
	if err := store.SaveTask(ctx, &model.Task{
		ID:        "task-overdue",
		Title:     "逾期任务",
		Status:    model.StatusTodo,
		CreatedAt: now.AddDate(0, 0, -5),
		UpdatedAt: now,
		DueDate:   &due,
		Source:    model.SourceLocal,
		Priority:  model.PriorityHigh,
		Quadrant:  model.QuadrantUrgentImportant,
	}); err != nil {
		t.Fatalf("save task: %v", err)
	}

	res, err := s.handleAnalyzeOverdueHealth(ctx, buildCallToolRequest(t, map[string]interface{}{
		"include_suggestions": true,
	}))
	if err != nil {
		t.Fatalf("analyze overdue: %v", err)
	}

	payload := parseJSONResult(t, res)
	summary, ok := payload["summary"].(map[string]interface{})
	if !ok {
		t.Fatalf("summary missing: %#v", payload)
	}
	if int(summary["overdue_count"].(float64)) != 1 {
		t.Fatalf("unexpected overdue_count: %#v", summary)
	}
	if summary["is_warning"].(bool) {
		t.Fatalf("expected warning=false with default threshold fallback")
	}
	if summary["is_overload"].(bool) {
		t.Fatalf("expected overload=false with default threshold fallback")
	}

	configApplied, ok := payload["config_applied"].(map[string]interface{})
	if !ok {
		t.Fatalf("config_applied missing: %#v", payload)
	}
	if int(configApplied["warning_threshold"].(float64)) != 3 {
		t.Fatalf("expected warning_threshold=3 fallback when configured 0: %#v", configApplied)
	}
	if int(configApplied["overload_threshold"].(float64)) != 10 {
		t.Fatalf("expected overload_threshold=10 fallback when configured 0: %#v", configApplied)
	}

	if _, ok := payload["actions"].([]interface{}); !ok {
		t.Fatalf("actions missing: %#v", payload)
	}
	if _, ok := payload["questions"].([]interface{}); !ok {
		t.Fatalf("questions missing: %#v", payload)
	}
}

func TestHandleResolveOverdueTasksDeleteNeedsConfirm(t *testing.T) {
	cfg := pkgconfig.DefaultConfig().MCP.Intelligence
	cfg.Overdue.AskBeforeDelete = true

	s, store, ctx := newIntelligenceTestServer(t, cfg, nil)

	now := time.Now()
	if err := store.SaveTask(ctx, &model.Task{
		ID:        "delete-me",
		Title:     "待删除逾期",
		Status:    model.StatusTodo,
		CreatedAt: now.AddDate(0, 0, -10),
		UpdatedAt: now,
		Source:    model.SourceLocal,
	}); err != nil {
		t.Fatalf("save task: %v", err)
	}

	res, err := s.handleResolveOverdueTasks(ctx, buildCallToolRequest(t, map[string]interface{}{
		"actions": []map[string]interface{}{{
			"task_id": "delete-me",
			"type":    "delete",
		}},
	}))
	if err != nil {
		t.Fatalf("resolve overdue: %v", err)
	}
	payload := parseJSONResult(t, res)
	if int(payload["deleted"].(float64)) != 0 {
		t.Fatalf("expected deleted=0 without confirm: %#v", payload)
	}
	if _, err := store.GetTask(ctx, "delete-me"); err != nil {
		t.Fatalf("task should still exist: %v", err)
	}

	res, err = s.handleResolveOverdueTasks(ctx, buildCallToolRequest(t, map[string]interface{}{
		"confirm_token": "confirm-delete",
		"actions": []map[string]interface{}{{
			"task_id": "delete-me",
			"type":    "delete",
		}},
	}))
	if err != nil {
		t.Fatalf("resolve overdue with confirm: %v", err)
	}
	payload = parseJSONResult(t, res)
	if int(payload["deleted"].(float64)) != 1 {
		t.Fatalf("expected deleted=1 with confirm: %#v", payload)
	}
	if _, err := store.GetTask(ctx, "delete-me"); err == nil {
		t.Fatalf("task should be deleted")
	}
}

func TestHandleRebalanceLongTermTasksPromoteWhenShortage(t *testing.T) {
	cfg := pkgconfig.DefaultConfig().MCP.Intelligence
	cfg.LongTerm.MinAgeDays = 1
	cfg.LongTerm.ShortTermMin = 5
	cfg.LongTerm.ShortTermMax = 10
	cfg.LongTerm.PromoteCountWhenShortage = 2

	s, store, ctx := newIntelligenceTestServer(t, cfg, nil)

	now := time.Now()
	dueTomorrow := now.AddDate(0, 0, 1)
	_ = store.SaveTask(ctx, &model.Task{ID: "short-1", Title: "短期任务", Status: model.StatusTodo, CreatedAt: now.AddDate(0, 0, -1), UpdatedAt: now, DueDate: &dueTomorrow, Source: model.SourceLocal})
	_ = store.SaveTask(ctx, &model.Task{ID: "long-1", Title: "长期任务 1", Status: model.StatusTodo, CreatedAt: now.AddDate(0, 0, -10), UpdatedAt: now.AddDate(0, 0, -1), Source: model.SourceLocal, Priority: model.PriorityHigh, Quadrant: model.QuadrantNotUrgentImportant})
	_ = store.SaveTask(ctx, &model.Task{ID: "long-2", Title: "长期任务 2", Status: model.StatusTodo, CreatedAt: now.AddDate(0, 0, -8), UpdatedAt: now.AddDate(0, 0, -2), Source: model.SourceLocal, Priority: model.PriorityMedium, Quadrant: model.QuadrantNotUrgentImportant})

	res, err := s.handleRebalanceLongTermTasks(ctx, buildCallToolRequest(t, map[string]interface{}{"dry_run": false}))
	if err != nil {
		t.Fatalf("rebalance longterm: %v", err)
	}
	payload := parseJSONResult(t, res)
	if payload["mode"].(string) != "shortage" {
		t.Fatalf("unexpected mode: %#v", payload)
	}
	promoted := payload["promoted_tasks"].([]interface{})
	if len(promoted) != 2 {
		t.Fatalf("expected 2 promoted tasks, got %d", len(promoted))
	}

	long1, err := store.GetTask(ctx, "long-1")
	if err != nil || long1.DueDate == nil {
		t.Fatalf("long-1 should have due date after rebalance")
	}
	long2, err := store.GetTask(ctx, "long-2")
	if err != nil || long2.DueDate == nil {
		t.Fatalf("long-2 should have due date after rebalance")
	}
}

func TestHandleDetectDecompositionCandidates(t *testing.T) {
	cfg := pkgconfig.DefaultConfig().MCP.Intelligence
	cfg.Decompose.ComplexityThreshold = 20
	cfg.Decompose.AbstractKeywords = []string{"优化", "整理"}

	providers := map[string]provider.Provider{
		"google": &capabilityProvider{name: "google", caps: provider.Capabilities{SupportsSubtasks: true}},
	}
	s, store, ctx := newIntelligenceTestServer(t, cfg, providers)

	now := time.Now()
	if err := store.SaveTask(ctx, &model.Task{
		ID:        "candidate-1",
		Title:     "优化系统并整理实现方案",
		Status:    model.StatusTodo,
		CreatedAt: now.AddDate(0, 0, -9),
		UpdatedAt: now.AddDate(0, 0, -3),
		Source:    model.SourceLocal,
		Priority:  model.PriorityHigh,
	}); err != nil {
		t.Fatalf("save candidate task: %v", err)
	}

	res, err := s.handleDetectDecompositionCandidates(ctx, buildCallToolRequest(t, map[string]interface{}{"limit": 10}))
	if err != nil {
		t.Fatalf("detect candidates: %v", err)
	}
	payload := parseJSONResult(t, res)
	summary := payload["summary"].(map[string]interface{})
	if int(summary["candidate_count"].(float64)) == 0 {
		t.Fatalf("expected at least one candidate: %#v", payload)
	}
	candidates := payload["candidates"].([]interface{})
	candidate := candidates[0].(map[string]interface{})
	if candidate["task_id"].(string) != "candidate-1" {
		t.Fatalf("unexpected candidate task id: %#v", candidate)
	}
	if candidate["recommended_provider"].(string) != "google" {
		t.Fatalf("expected recommended provider google: %#v", candidate)
	}
	if !candidate["provider_supports_subtasks"].(bool) {
		t.Fatalf("expected provider_supports_subtasks=true: %#v", candidate)
	}
}

func TestHandleDecomposeTaskWithProviderWriteTasks(t *testing.T) {
	cfg := pkgconfig.DefaultConfig().MCP.Intelligence

	providers := map[string]provider.Provider{
		"google": &capabilityProvider{name: "google", caps: provider.Capabilities{SupportsSubtasks: true}},
	}
	s, store, ctx := newIntelligenceTestServer(t, cfg, providers)

	now := time.Now()
	if err := store.SaveTask(ctx, &model.Task{
		ID:        "parent-task",
		Title:     "推进产品上线计划",
		Status:    model.StatusTodo,
		CreatedAt: now.AddDate(0, 0, -4),
		UpdatedAt: now,
		Source:    model.SourceLocal,
		Priority:  model.PriorityHigh,
		Quadrant:  model.QuadrantNotUrgentImportant,
	}); err != nil {
		t.Fatalf("save parent task: %v", err)
	}

	res, err := s.handleDecomposeTaskWithProvider(ctx, buildCallToolRequest(t, map[string]interface{}{
		"task_id":     "parent-task",
		"provider":    "google",
		"strategy":    "project_split",
		"write_tasks": true,
	}))
	if err != nil {
		t.Fatalf("decompose task: %v", err)
	}
	payload := parseJSONResult(t, res)
	createdIDs := payload["created_task_ids"].([]interface{})
	if len(createdIDs) == 0 {
		t.Fatalf("expected created tasks: %#v", payload)
	}

	for _, rawID := range createdIDs {
		taskID := rawID.(string)
		task, err := store.GetTask(ctx, taskID)
		if err != nil {
			t.Fatalf("created task not found: %v", err)
		}
		if task.Metadata == nil || task.Metadata.CustomFields == nil {
			t.Fatalf("created task metadata missing: %s", taskID)
		}
		if task.Metadata.CustomFields["tb_parent_task_id"] != "parent-task" {
			t.Fatalf("parent linkage missing: %#v", task.Metadata.CustomFields)
		}
	}
}

func TestHandleAnalyzeAchievement(t *testing.T) {
	cfg := pkgconfig.DefaultConfig().MCP.Intelligence
	cfg.Achievement.BadgeEnabled = true
	cfg.Achievement.NarrativeEnabled = true
	cfg.Achievement.StreakGoalPerDay = 1

	s, store, ctx := newIntelligenceTestServer(t, cfg, nil)

	now := time.Now()
	for i := 0; i < 7; i++ {
		completedAt := now.AddDate(0, 0, -i)
		due := completedAt.Add(12 * time.Hour)
		task := &model.Task{
			ID:          fmt.Sprintf("done-%d", i),
			Title:       fmt.Sprintf("完成任务 %d", i),
			Status:      model.StatusCompleted,
			CreatedAt:   completedAt.Add(-6 * time.Hour),
			UpdatedAt:   completedAt,
			CompletedAt: &completedAt,
			DueDate:     &due,
			Source:      model.SourceLocal,
			Quadrant:    model.QuadrantNotUrgentImportant,
		}
		if err := store.SaveTask(ctx, task); err != nil {
			t.Fatalf("save completed task: %v", err)
		}
	}

	res, err := s.handleAnalyzeAchievement(ctx, buildCallToolRequest(t, map[string]interface{}{
		"window_days":      30,
		"compare_previous": false,
	}))
	if err != nil {
		t.Fatalf("analyze achievement: %v", err)
	}
	payload := parseJSONResult(t, res)
	metrics := payload["metrics"].(map[string]interface{})
	if int(metrics["completed_count"].(float64)) != 7 {
		t.Fatalf("unexpected completed_count: %#v", metrics)
	}
	if int(metrics["streak_days"].(float64)) < 7 {
		t.Fatalf("expected streak>=7: %#v", metrics)
	}
	badges := payload["badges"].([]interface{})
	hasSteady7 := false
	for _, badge := range badges {
		if badge.(string) == "steady-7" {
			hasSteady7 = true
			break
		}
	}
	if !hasSteady7 {
		t.Fatalf("expected steady-7 badge: %#v", badges)
	}
	if strings.TrimSpace(payload["narrative"].(string)) == "" {
		t.Fatalf("expected non-empty narrative")
	}
}
