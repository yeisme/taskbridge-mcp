package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/project"
	"github.com/yeisme/taskbridge/internal/projectplanner"
	"github.com/yeisme/taskbridge/internal/provider"
	msprovider "github.com/yeisme/taskbridge/internal/provider/microsoft"
	"github.com/yeisme/taskbridge/internal/storage"
)

// ================ 任务工具处理器 ================

// handleListTasks 处理列出任务请求
func (s *Server) handleListTasks(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.taskStore == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "[]"}},
		}, nil
	}

	// 默认返回 compact 以降低上下文消耗。
	detail := "compact"
	includeMeta := false

	query := storage.Query{}
	appliedFilters := map[string]interface{}{}

	var rawArgs map[string]json.RawMessage
	if args := req.Params.Arguments; args != nil {
		if err := json.Unmarshal(args, &rawArgs); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	if source := getString(rawArgs, "source"); source != "" {
		resolvedSource, err := resolveProviderNameStrict(source)
		if err != nil {
			return nil, err
		}
		query.Sources = []model.TaskSource{model.TaskSource(resolvedSource)}
		appliedFilters["source"] = resolvedSource
	}

	if values := getStringSlice(rawArgs, "list_id"); len(values) > 0 {
		query.ListIDs = values
		appliedFilters["list_id"] = values
	}

	if values := getStringSlice(rawArgs, "list_name"); len(values) > 0 {
		query.ListNames = values
		appliedFilters["list_name"] = values
	}

	if values := getStringSlice(rawArgs, "task_id"); len(values) > 0 {
		query.TaskIDs = values
		appliedFilters["task_id"] = values
	}

	if values := getStringSlice(rawArgs, "status"); len(values) > 0 {
		query.Statuses = make([]model.TaskStatus, 0, len(values))
		for _, value := range values {
			query.Statuses = append(query.Statuses, model.TaskStatus(value))
		}
		appliedFilters["status"] = values
	}

	if values := getIntSlice(rawArgs, "quadrant"); len(values) > 0 {
		query.Quadrants = make([]model.Quadrant, 0, len(values))
		for _, value := range values {
			query.Quadrants = append(query.Quadrants, model.Quadrant(value))
		}
		appliedFilters["quadrant"] = values
	}

	if values := getIntSlice(rawArgs, "priority"); len(values) > 0 {
		query.Priorities = make([]model.Priority, 0, len(values))
		for _, value := range values {
			query.Priorities = append(query.Priorities, model.Priority(value))
		}
		appliedFilters["priority"] = values
	}

	if values := getStringSlice(rawArgs, "tag"); len(values) > 0 {
		query.Tags = values
		appliedFilters["tag"] = values
	}

	if v := getString(rawArgs, "due_before"); v != "" {
		if dueBefore, err := time.Parse("2006-01-02", v); err == nil {
			query.DueBefore = &dueBefore
			appliedFilters["due_before"] = v
		}
	}
	if v := getString(rawArgs, "due_after"); v != "" {
		if dueAfter, err := time.Parse("2006-01-02", v); err == nil {
			query.DueAfter = &dueAfter
			appliedFilters["due_after"] = v
		}
	}

	if value := getString(rawArgs, "query"); value != "" {
		query.QueryText = value
		appliedFilters["query"] = value
	}

	if value, ok := getInt(rawArgs, "limit"); ok {
		query.Limit = value
		appliedFilters["limit"] = value
	}
	if value, ok := getInt(rawArgs, "offset"); ok {
		query.Offset = value
		appliedFilters["offset"] = value
	}
	if value := getString(rawArgs, "order_by"); value != "" {
		query.OrderBy = value
		appliedFilters["order_by"] = value
	}
	if value, ok := getBool(rawArgs, "order_desc"); ok {
		query.OrderDesc = value
		appliedFilters["order_desc"] = value
	}

	if value := strings.TrimSpace(getString(rawArgs, "detail")); value != "" {
		detail = strings.ToLower(value)
	}
	if value, ok := getBool(rawArgs, "include_meta"); ok {
		includeMeta = value
	}

	tasks, err := s.taskStore.QueryTasks(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	var payload interface{}
	if detail == "full" {
		payload = tasks
	} else {
		payload = toCompactTasks(tasks)
	}

	if includeMeta {
		total := len(tasks)
		if query.Limit > 0 || query.Offset > 0 {
			countQuery := query
			countQuery.Limit = 0
			countQuery.Offset = 0
			countQuery.OrderBy = ""
			countQuery.OrderDesc = false
			allMatched, err := s.taskStore.QueryTasks(ctx, countQuery)
			if err == nil {
				total = len(allMatched)
			}
		}

		payload = map[string]interface{}{
			"tasks": payload,
			"meta": map[string]interface{}{
				"returned":        len(tasks),
				"total":           total,
				"detail":          detail,
				"applied_filters": appliedFilters,
			},
		}
	}

	result, err := toJSON(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

func (s *Server) handleListTaskLists(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.taskStore == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "[]"}},
		}, nil
	}

	source := ""
	if args := req.Params.Arguments; args != nil {
		var params struct {
			Source string `json:"source"`
		}
		if err := json.Unmarshal(args, &params); err == nil {
			var resolveErr error
			source, resolveErr = resolveProviderNameStrict(strings.TrimSpace(params.Source))
			if resolveErr != nil {
				return nil, resolveErr
			}
		}
	}

	lists, err := s.taskStore.ListTaskLists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list task lists: %w", err)
	}

	opts := storage.ListOptions{}
	if source != "" && provider.IsValidProvider(source) {
		opts.Source = model.TaskSource(source)
	}
	tasks, err := s.taskStore.ListTasks(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks for counts: %w", err)
	}

	counts := make(map[string]int, len(tasks))
	for _, task := range tasks {
		if task.ListID == "" {
			continue
		}
		countKey := buildProviderListKey(string(task.Source), task.ListID)
		counts[countKey]++
	}

	type listItem struct {
		Provider       string `json:"provider"`
		Source         string `json:"source"`
		ListID         string `json:"list_id"`
		ListName       string `json:"list_name"`
		TaskCountLocal int    `json:"task_count_local"`
	}

	result := make([]listItem, 0, len(lists))
	for _, list := range lists {
		if source != "" && provider.IsValidProvider(source) && string(list.Source) != source {
			continue
		}
		countKey := buildProviderListKey(string(list.Source), list.ID)
		result = append(result, listItem{
			Provider:       string(list.Source),
			Source:         string(list.Source),
			ListID:         list.ID,
			ListName:       list.Name,
			TaskCountLocal: counts[countKey],
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Provider == result[j].Provider {
			return result[i].ListName < result[j].ListName
		}
		return result[i].Provider < result[j].Provider
	})

	output, err := toJSON(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal list_task_lists result: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: output}},
	}, nil
}

// handleCreateTask 处理创建任务请求
func (s *Server) handleCreateTask(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.taskStore == nil {
		return nil, fmt.Errorf("task storage not available")
	}

	// 解析参数
	var params struct {
		Title    string `json:"title"`
		DueDate  string `json:"due_date"`
		Priority int    `json:"priority"`
		Quadrant int    `json:"quadrant"`
		ParentID string `json:"parent_id"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Title == "" {
		return nil, fmt.Errorf("title is required")
	}

	// 创建任务
	task := &model.Task{
		ID:        generateID(),
		Title:     params.Title,
		Status:    model.StatusTodo,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Source:    model.SourceLocal,
	}

	// 设置截止日期
	if params.DueDate != "" {
		dueDate, err := time.Parse("2006-01-02", params.DueDate)
		if err == nil {
			task.DueDate = &dueDate
		}
	}

	// 设置优先级
	if params.Priority > 0 && params.Priority <= 4 {
		task.Priority = model.Priority(params.Priority)
	}

	// 设置象限
	if params.Quadrant > 0 && params.Quadrant <= 4 {
		task.Quadrant = model.Quadrant(params.Quadrant)
	} else {
		// 自动计算象限
		task.Quadrant = model.CalculateQuadrantFromTask(task)
	}

	// 设置元数据，包含 local_id 用于后续同步匹配
	task.Metadata = &model.TaskMetadata{
		Version:    "1.0",
		Quadrant:   int(task.Quadrant),
		Priority:   int(task.Priority),
		LocalID:    task.ID,
		SyncSource: "local",
	}
	if strings.TrimSpace(params.ParentID) != "" {
		parentID := strings.TrimSpace(params.ParentID)
		task.ParentID = &parentID
	}

	// 保存任务到本地
	if err := s.taskStore.SaveTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to save task: %w", err)
	}

	// 尝试自动同步到 Google Tasks
	if googleProvider, ok := s.providers["google"]; ok && googleProvider.IsAuthenticated() {
		// 获取默认任务列表
		taskLists, err := googleProvider.ListTaskLists(ctx)
		if err == nil && len(taskLists) > 0 {
			// 查找默认列表
			var defaultListID string
			for _, list := range taskLists {
				if list.Name == "我的任务" || list.Name == "My Tasks" || list.ID == "@default" {
					defaultListID = list.ID
					break
				}
			}
			if defaultListID == "" {
				defaultListID = taskLists[0].ID
			}

			// 推送任务到 Google Tasks
			taskToSync := *task
			if taskToSync.ParentID != nil && strings.TrimSpace(*taskToSync.ParentID) != "" {
				// 若 parent_id 是本地任务 ID，则尝试解析其 Google 远端 ID。
				if parentTask, err := s.taskStore.GetTask(ctx, strings.TrimSpace(*taskToSync.ParentID)); err == nil && parentTask != nil && strings.TrimSpace(parentTask.SourceRawID) != "" {
					parentRemote := strings.TrimSpace(parentTask.SourceRawID)
					taskToSync.ParentID = &parentRemote
				}
			}
			taskToSync = sanitizeTaskForRemote(taskToSync)
			createdTask, err := googleProvider.CreateTask(ctx, defaultListID, &taskToSync)
			if err == nil {
				// 更新本地任务信息
				task.SourceRawID = createdTask.SourceRawID
				task.ListID = defaultListID
				task.Source = model.SourceGoogle
				if task.Metadata != nil {
					task.Metadata.LastSyncAt = time.Now()
				}
				_ = s.taskStore.SaveTask(ctx, task)
			}
		}
	}

	result, _ := toJSON(task)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// handleUpdateTask 处理更新任务请求
func (s *Server) handleUpdateTask(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.taskStore == nil {
		return nil, fmt.Errorf("task storage not available")
	}

	// 解析参数
	var params struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.ID == "" {
		return nil, fmt.Errorf("id is required")
	}

	// 获取现有任务
	task, err := s.taskStore.GetTask(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	// 更新字段
	if params.Title != "" {
		task.Title = params.Title
	}
	if params.Status != "" {
		task.Status = model.TaskStatus(params.Status)
		if task.Status == model.StatusCompleted {
			now := time.Now()
			task.CompletedAt = &now
		}
	}
	task.UpdatedAt = time.Now()

	// 保存任务
	if err := s.taskStore.SaveTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to save task: %w", err)
	}

	result, _ := toJSON(task)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// handleDeleteTask 处理删除任务请求
func (s *Server) handleDeleteTask(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.taskStore == nil {
		return nil, fmt.Errorf("task storage not available")
	}

	// 解析参数
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.ID == "" {
		return nil, fmt.Errorf("id is required")
	}

	// 删除任务
	if err := s.taskStore.DeleteTask(ctx, params.ID); err != nil {
		return nil, fmt.Errorf("failed to delete task: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(`{"success": true, "id": "%s"}`, params.ID)}},
	}, nil
}

// handleCompleteTask 处理完成任务请求
func (s *Server) handleCompleteTask(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.taskStore == nil {
		return nil, fmt.Errorf("task storage not available")
	}

	// 解析参数
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.ID == "" {
		return nil, fmt.Errorf("id is required")
	}

	// 获取现有任务
	task, err := s.taskStore.GetTask(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	// 标记为完成
	task.Status = model.StatusCompleted
	now := time.Now()
	task.CompletedAt = &now
	task.UpdatedAt = now

	// 保存任务
	if err := s.taskStore.SaveTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to save task: %w", err)
	}

	result, _ := toJSON(task)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// ================ 分析工具处理器 ================

// QuadrantAnalysis 四象限分析结果
type QuadrantAnalysis struct {
	GeneratedAt time.Time    `json:"generated_at"`
	Q1          []model.Task `json:"q1"` // 紧急且重要
	Q2          []model.Task `json:"q2"` // 重要不紧急
	Q3          []model.Task `json:"q3"` // 紧急不重要
	Q4          []model.Task `json:"q4"` // 不紧急不重要
	Summary     string       `json:"summary"`
}

// handleAnalyzeQuadrant 处理四象限分析请求
func (s *Server) handleAnalyzeQuadrant(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.taskStore == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: `{"q1": [], "q2": [], "q3": [], "q4": [], "summary": "No tasks available"}`}},
		}, nil
	}

	tasks, err := s.taskStore.ListTasks(ctx, storage.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	analysis := QuadrantAnalysis{
		GeneratedAt: time.Now(),
	}

	for _, task := range tasks {
		// 计算象限
		quadrant := model.CalculateQuadrantFromTask(&task)
		task.Quadrant = quadrant

		switch quadrant {
		case model.QuadrantUrgentImportant:
			analysis.Q1 = append(analysis.Q1, task)
		case model.QuadrantNotUrgentImportant:
			analysis.Q2 = append(analysis.Q2, task)
		case model.QuadrantUrgentNotImportant:
			analysis.Q3 = append(analysis.Q3, task)
		case model.QuadrantNotUrgentNotImportant:
			analysis.Q4 = append(analysis.Q4, task)
		}
	}

	// 生成摘要
	analysis.Summary = fmt.Sprintf(
		"Q1(紧急重要): %d个任务 - 立即处理\n"+
			"Q2(重要不紧急): %d个任务 - 计划安排\n"+
			"Q3(紧急不重要): %d个任务 - 委托他人\n"+
			"Q4(不紧急不重要): %d个任务 - 考虑删除",
		len(analysis.Q1), len(analysis.Q2), len(analysis.Q3), len(analysis.Q4),
	)

	result, _ := toJSON(analysis)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// PriorityAnalysis 优先级分析结果
type PriorityAnalysis struct {
	GeneratedAt time.Time    `json:"generated_at"`
	Urgent      []model.Task `json:"urgent"`
	High        []model.Task `json:"high"`
	Medium      []model.Task `json:"medium"`
	Low         []model.Task `json:"low"`
	None        []model.Task `json:"none"`
	Summary     string       `json:"summary"`
}

// handleAnalyzePriority 处理优先级分析请求
func (s *Server) handleAnalyzePriority(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.taskStore == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: `{"urgent": [], "high": [], "medium": [], "low": [], "none": [], "summary": "No tasks available"}`}},
		}, nil
	}

	tasks, err := s.taskStore.ListTasks(ctx, storage.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	analysis := PriorityAnalysis{
		GeneratedAt: time.Now(),
	}

	for _, task := range tasks {
		switch task.Priority {
		case model.PriorityUrgent:
			analysis.Urgent = append(analysis.Urgent, task)
		case model.PriorityHigh:
			analysis.High = append(analysis.High, task)
		case model.PriorityMedium:
			analysis.Medium = append(analysis.Medium, task)
		case model.PriorityLow:
			analysis.Low = append(analysis.Low, task)
		default:
			analysis.None = append(analysis.None, task)
		}
	}

	analysis.Summary = fmt.Sprintf(
		"紧急: %d | 高: %d | 中: %d | 低: %d | 无: %d",
		len(analysis.Urgent), len(analysis.High), len(analysis.Medium), len(analysis.Low), len(analysis.None),
	)

	result, _ := toJSON(analysis)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// ================ 项目工具处理器 ================

// handleCreateProject 处理创建项目请求
func (s *Server) handleCreateProject(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.projectStore == nil {
		return nil, fmt.Errorf("project storage not available")
	}

	var params struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ParentID    string `json:"parent_id"`
		GoalText    string `json:"goal_text"`
		HorizonDays int    `json:"horizon_days"`
		ListID      string `json:"list_id"`
		Source      string `json:"source"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if strings.TrimSpace(params.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}

	source := strings.TrimSpace(params.Source)
	if source != "" {
		resolvedSource, err := resolveProviderNameStrict(source)
		if err != nil {
			return nil, err
		}
		source = resolvedSource
	}

	goalText := strings.TrimSpace(params.GoalText)
	if goalText == "" {
		goalText = strings.TrimSpace(params.Name)
	}

	now := time.Now()
	item := &project.Project{
		ID:           generateProjectID(),
		Name:         strings.TrimSpace(params.Name),
		Description:  strings.TrimSpace(params.Description),
		ParentID:     strings.TrimSpace(params.ParentID),
		GoalText:     goalText,
		GoalType:     projectplanner.DetectGoalType(goalText),
		Status:       project.StatusDraft,
		ListID:       strings.TrimSpace(params.ListID),
		Source:       source,
		HorizonDays:  params.HorizonDays,
		LatestPlanID: "",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if item.HorizonDays == 0 {
		item.HorizonDays = 14
	}

	if err := s.projectStore.SaveProject(ctx, item); err != nil {
		return nil, fmt.Errorf("failed to save project: %w", err)
	}

	result, _ := toJSON(item)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// handleListProjects 处理列出项目请求
func (s *Server) handleListProjects(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.projectStore == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "[]"}},
		}, nil
	}

	var params struct {
		Status string `json:"status"`
	}
	if len(req.Params.Arguments) > 0 {
		if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	items, err := s.projectStore.ListProjects(ctx, strings.TrimSpace(params.Status))
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	resultItems := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		summary := ""
		if plan, err := s.projectStore.GetLatestPlan(ctx, item.ID); err == nil {
			summary = fmt.Sprintf("%d phases / %d tasks", len(plan.Phases), len(plan.TasksPreview))
		}
		resultItems = append(resultItems, map[string]interface{}{
			"id":                  item.ID,
			"name":                item.Name,
			"status":              item.Status,
			"goal_type":           item.GoalType,
			"created_at":          item.CreatedAt,
			"updated_at":          item.UpdatedAt,
			"latest_plan_id":      item.LatestPlanID,
			"latest_plan_summary": summary,
		})
	}

	result, _ := toJSON(resultItems)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// handleSplitProject 处理拆分项目请求
func (s *Server) handleSplitProject(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.projectStore == nil {
		return nil, fmt.Errorf("project storage not available")
	}

	var params struct {
		ProjectID   string `json:"project_id"`
		AIHint      string `json:"ai_hint"`
		GoalText    string `json:"goal_text"`
		HorizonDays int    `json:"horizon_days"`
		MaxTasks    int    `json:"max_tasks"`
		Constraints struct {
			RequireDeliverable bool `json:"require_deliverable"`
			MinEstimateMinutes int  `json:"min_estimate_minutes"`
			MaxEstimateMinutes int  `json:"max_estimate_minutes"`
			MinTasks           int  `json:"min_tasks"`
			MaxTasks           int  `json:"max_tasks"`
			MinPracticeTasks   int  `json:"min_practice_tasks"`
		} `json:"constraints"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.ProjectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	item, err := s.projectStore.GetProject(ctx, params.ProjectID)
	if err != nil {
		return nil, err
	}

	goalText := strings.TrimSpace(params.GoalText)
	if goalText == "" {
		goalText = item.GoalText
	}

	suggestion := projectplanner.Decompose(projectplanner.DecomposeInput{
		ProjectID:   item.ID,
		ProjectName: item.Name,
		GoalText:    goalText,
		GoalType:    item.GoalType,
		HorizonDays: pickHorizonDays(params.HorizonDays, item.HorizonDays),
		MaxTasks:    params.MaxTasks,
		AIHint:      params.AIHint,
		Constraints: project.PlanConstraints{
			RequireDeliverable: params.Constraints.RequireDeliverable,
			MinEstimateMinutes: params.Constraints.MinEstimateMinutes,
			MaxEstimateMinutes: params.Constraints.MaxEstimateMinutes,
			MinTasks:           params.Constraints.MinTasks,
			MaxTasks:           params.Constraints.MaxTasks,
			MinPracticeTasks:   params.Constraints.MinPracticeTasks,
		},
	})
	suggestion.PlanID = generatePlanID()

	if err := s.projectStore.SavePlan(ctx, suggestion); err != nil {
		return nil, fmt.Errorf("failed to save project plan: %w", err)
	}

	item.GoalText = goalText
	item.GoalType = suggestion.GoalType
	item.Status = project.StatusSplitSuggested
	item.LatestPlanID = suggestion.PlanID
	item.HorizonDays = pickHorizonDays(params.HorizonDays, item.HorizonDays)
	if err := s.projectStore.SaveProject(ctx, item); err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}

	response := map[string]interface{}{
		"project_id":    item.ID,
		"plan_id":       suggestion.PlanID,
		"status":        suggestion.Status,
		"confidence":    suggestion.Confidence,
		"constraints":   suggestion.Constraints,
		"tasks_preview": suggestion.TasksPreview,
		"phases":        suggestion.Phases,
		"warnings":      suggestion.Warnings,
	}
	result, _ := toJSON(response)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// handleConfirmProject 处理确认项目请求
func (s *Server) handleConfirmProject(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.projectStore == nil {
		return nil, fmt.Errorf("project storage not available")
	}

	var params struct {
		ProjectID  string `json:"project_id"`
		PlanID     string `json:"plan_id"`
		WriteTasks *bool  `json:"write_tasks"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.ProjectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	item, err := s.projectStore.GetProject(ctx, params.ProjectID)
	if err != nil {
		return nil, err
	}

	writeTasks := true
	if params.WriteTasks != nil {
		writeTasks = *params.WriteTasks
	}

	var plan *project.PlanSuggestion
	if strings.TrimSpace(params.PlanID) != "" {
		plan, err = s.projectStore.GetPlan(ctx, params.ProjectID, strings.TrimSpace(params.PlanID))
	} else {
		plan, err = s.projectStore.GetLatestPlan(ctx, params.ProjectID)
	}
	if err != nil {
		return nil, err
	}

	if writeTasks && len(plan.ConfirmedTaskIDs) > 0 {
		response := map[string]interface{}{
			"project_id":       params.ProjectID,
			"status":           project.StatusConfirmed,
			"created_task_ids": plan.ConfirmedTaskIDs,
			"count":            len(plan.ConfirmedTaskIDs),
		}
		result, _ := toJSON(response)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: result}},
		}, nil
	}

	createdTaskIDs := make([]string, 0)
	if writeTasks {
		if s.taskStore == nil {
			return nil, fmt.Errorf("task storage not available")
		}

		now := time.Now()
		// 维护 plan_task_id -> 本地任务 ID 映射，用于恢复父子关系。
		planToLocalID := make(map[string]string, len(plan.TasksPreview))
		for idx, planTask := range plan.TasksPreview {
			task := &model.Task{
				ID:          generateID(),
				Title:       planTask.Title,
				Description: planTask.Description,
				Status:      model.StatusTodo,
				CreatedAt:   now,
				UpdatedAt:   now,
				Source:      model.SourceLocal,
				ListID:      item.ListID,
				Priority:    clampPriority(planTask.Priority),
				Quadrant:    clampQuadrant(planTask.Quadrant),
				Tags:        append([]string{}, planTask.Tags...),
			}
			dueDate := now.AddDate(0, 0, maxInt(1, planTask.DueOffsetDays))
			task.DueDate = &dueDate
			task.Metadata = &model.TaskMetadata{
				Version:    "1.0",
				Quadrant:   int(task.Quadrant),
				Priority:   int(task.Priority),
				LocalID:    task.ID,
				SyncSource: "local",
				CustomFields: map[string]interface{}{
					"tb_project_id": item.ID,
					"tb_plan_id":    plan.PlanID,
					"tb_goal_type":  string(item.GoalType),
					"tb_phase":      planTask.Phase,
					"tb_step_index": idx + 1,
				},
			}
			if strings.TrimSpace(planTask.ParentID) != "" {
				if parentLocalID, ok := planToLocalID[strings.TrimSpace(planTask.ParentID)]; ok {
					parentID := parentLocalID
					task.ParentID = &parentID
				}
			}
			if strings.TrimSpace(planTask.ID) != "" {
				// 兼容新旧计划：仅当计划任务含稳定 ID 时才写入 metadata。
				task.Metadata.CustomFields["tb_plan_task_id"] = strings.TrimSpace(planTask.ID)
			}
			if strings.TrimSpace(planTask.ParentID) != "" {
				task.Metadata.CustomFields["tb_parent_plan_task_id"] = strings.TrimSpace(planTask.ParentID)
			}
			if err := s.taskStore.SaveTask(ctx, task); err != nil {
				return nil, fmt.Errorf("failed to save task from plan: %w", err)
			}
			createdTaskIDs = append(createdTaskIDs, task.ID)
			if strings.TrimSpace(planTask.ID) != "" {
				planToLocalID[strings.TrimSpace(planTask.ID)] = task.ID
			}
		}
	}

	now := time.Now()
	plan.Status = project.StatusConfirmed
	plan.ConfirmedTaskIDs = createdTaskIDs
	plan.ConfirmedAt = &now
	if err := s.projectStore.SavePlan(ctx, plan); err != nil {
		return nil, fmt.Errorf("failed to update plan status: %w", err)
	}

	item.Status = project.StatusConfirmed
	item.LatestPlanID = plan.PlanID
	if err := s.projectStore.SaveProject(ctx, item); err != nil {
		return nil, fmt.Errorf("failed to update project status: %w", err)
	}

	response := map[string]interface{}{
		"project_id":       item.ID,
		"status":           item.Status,
		"created_task_ids": createdTaskIDs,
		"count":            len(createdTaskIDs),
	}
	result, _ := toJSON(response)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// handleSyncProject 处理同步项目请求
func (s *Server) handleSyncProject(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.projectStore == nil {
		return nil, fmt.Errorf("project storage not available")
	}
	if s.taskStore == nil {
		return nil, fmt.Errorf("task storage not available")
	}

	var params struct {
		ProjectID string `json:"project_id"`
		Provider  string `json:"provider"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.ProjectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if params.Provider == "" {
		return nil, fmt.Errorf("provider is required")
	}
	resolvedProvider, err := resolveProviderNameStrict(params.Provider)
	if err != nil {
		return nil, err
	}
	if _, err := s.projectStore.GetProject(ctx, params.ProjectID); err != nil {
		return nil, err
	}

	p, ok := s.providers[resolvedProvider]
	if !ok {
		return nil, fmt.Errorf("provider %s not found or not authenticated", resolvedProvider)
	}

	localTasks, err := s.taskStore.ListTasks(ctx, storage.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list local tasks: %w", err)
	}
	projectTasks := make([]model.Task, 0)
	for _, task := range localTasks {
		if taskBelongsToProject(task, params.ProjectID) {
			projectTasks = append(projectTasks, task)
		}
	}

	if len(projectTasks) == 0 {
		response := map[string]interface{}{
			"project_id": params.ProjectID,
			"provider":   resolvedProvider,
			"status":     project.StatusConfirmed,
			"pushed":     0,
			"updated":    0,
			"errors":     []string{},
			"message":    "未找到可同步的项目任务",
		}
		result, _ := toJSON(response)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: result}},
		}, nil
	}

	taskLists, err := p.ListTaskLists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote task lists: %w", err)
	}
	defaultListID := s.findDefaultListID(taskLists)

	resultPayload := &SyncPushResult{Provider: resolvedProvider}
	targetSource := model.TaskSource(resolvedProvider)
	if targetSource == model.SourceGoogle {
		planTaskToLocalID := buildPlanTaskToLocalIDMap(projectTasks)
		parentTasks, childTasks := splitTasksByParentRelation(projectTasks, planTaskToLocalID)
		s.pushLocalTasks(ctx, p, parentTasks, defaultListID, targetSource, false, resultPayload)
		if len(childTasks) > 0 {
			if err := s.pullProviderTasksIntoLocal(ctx, p, resolvedProvider); err != nil {
				resultPayload.Errors = append(resultPayload.Errors, fmt.Sprintf("pull-before-children: %v", err))
			}
			s.pushLocalTasks(ctx, p, childTasks, defaultListID, targetSource, false, resultPayload)
		}
	} else {
		s.pushLocalTasks(ctx, p, projectTasks, defaultListID, targetSource, false, resultPayload)
	}

	item, err := s.projectStore.GetProject(ctx, params.ProjectID)
	if err == nil {
		item.Status = project.StatusSynced
		_ = s.projectStore.SaveProject(ctx, item)
	}

	response := map[string]interface{}{
		"project_id": params.ProjectID,
		"provider":   resolvedProvider,
		"status":     project.StatusSynced,
		"pushed":     resultPayload.Pushed,
		"updated":    resultPayload.Updated,
		"errors":     resultPayload.Errors,
		"message":    fmt.Sprintf("项目已同步到 %s", resolvedProvider),
	}

	result, _ := toJSON(response)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// ================ 提示词工具处理器 ================

// handleGetPrompt 处理获取提示词请求
func (s *Server) handleGetPrompt(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 解析参数
	var params struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	prompt, ok := EmbeddedPrompts[params.Name]
	if !ok {
		return nil, fmt.Errorf("prompt not found: %s", params.Name)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: prompt}},
	}, nil
}

// ================ 提示词处理器 ================

// handleQuadrantAnalysisPrompt 处理四象限分析提示词请求
func (s *Server) handleQuadrantAnalysisPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return &mcp.GetPromptResult{
		Description: "四象限分析提示词",
		Messages: []*mcp.PromptMessage{
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: EmbeddedPrompts["quadrant_analysis"]},
			},
		},
	}, nil
}

// handleTaskCreationPrompt 处理任务创建提示词请求
func (s *Server) handleTaskCreationPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	title := ""
	if args := req.Params.Arguments; args != nil {
		title = args["title"]
	}

	prompt := EmbeddedPrompts["task_creation"]
	if title != "" {
		prompt = fmt.Sprintf("任务标题: %s\n\n%s", title, prompt)
	}

	return &mcp.GetPromptResult{
		Description: "任务创建提示词",
		Messages: []*mcp.PromptMessage{
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: prompt},
			},
		},
	}, nil
}

// handleProjectPlanningPrompt 处理项目规划提示词请求
func (s *Server) handleProjectPlanningPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	projectName := ""
	if args := req.Params.Arguments; args != nil {
		projectName = args["project_name"]
	}

	prompt := EmbeddedPrompts["project_planning"]
	if projectName != "" {
		prompt = fmt.Sprintf("项目名称: %s\n\n%s", projectName, prompt)
	}

	return &mcp.GetPromptResult{
		Description: "项目规划提示词",
		Messages: []*mcp.PromptMessage{
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: prompt},
			},
		},
	}, nil
}

// handleAISplitGuidePrompt 处理 AI 拆分指导提示词请求
func (s *Server) handleAISplitGuidePrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	projectName := ""
	complexity := "medium"
	if args := req.Params.Arguments; args != nil {
		projectName = args["project_name"]
		if c := args["complexity"]; c != "" {
			complexity = c
		}
	}

	prompt := fmt.Sprintf(EmbeddedPrompts["ai_split_guide"], projectName, complexity)

	return &mcp.GetPromptResult{
		Description: "AI 拆分指导提示词",
		Messages: []*mcp.PromptMessage{
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: prompt},
			},
		},
	}, nil
}

// handleJSONQueryCommandsPrompt 处理 JSON 检索命令提示词请求
func (s *Server) handleJSONQueryCommandsPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	goal := ""
	if args := req.Params.Arguments; args != nil {
		goal = strings.TrimSpace(args["goal"])
	}

	prompt := EmbeddedPrompts["json_query_commands"]
	if goal != "" {
		prompt = fmt.Sprintf("检索目标: %s\n\n%s", goal, prompt)
	}

	return &mcp.GetPromptResult{
		Description: "JSON 检索命令提示词",
		Messages: []*mcp.PromptMessage{
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: prompt},
			},
		},
	}, nil
}

// ================ 资源处理器 ================

// handleTasksResource 处理任务资源请求
func (s *Server) handleTasksResource(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if s.taskStore == nil {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: "taskbridge://tasks", Text: "[]"}},
		}, nil
	}

	tasks, err := s.taskStore.ListTasks(ctx, storage.ListOptions{})
	if err != nil {
		return nil, err
	}

	result, _ := toJSON(tasks)
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{URI: "taskbridge://tasks", Text: result}},
	}, nil
}

// handleProjectsResource 处理项目资源请求
func (s *Server) handleProjectsResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if s.projectStore == nil {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: "taskbridge://projects", Text: "[]"}},
		}, nil
	}
	projects, err := s.projectStore.ListProjects(ctx, "")
	if err != nil {
		return nil, err
	}
	output, err := toJSON(projects)
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{URI: "taskbridge://projects", Text: output}},
	}, nil
}

// handlePromptsResource 处理提示词资源请求
func (s *Server) handlePromptsResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	result, _ := toJSON(EmbeddedPrompts)
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{URI: "taskbridge://prompts", Text: result}},
	}, nil
}

// ================ 同步工具处理器 ================

// SyncPushResult 同步推送结果
type SyncPushResult struct {
	Provider string   `json:"provider"`
	Pushed   int      `json:"pushed"`
	Updated  int      `json:"updated"`
	Deleted  int      `json:"deleted"`
	Errors   []string `json:"errors,omitempty"`
	DryRun   bool     `json:"dry_run,omitempty"`
	Message  string   `json:"message,omitempty"`
}

func (s *Server) handleSyncPush(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 解析参数
	var params struct {
		Provider     string `json:"provider"`
		DeleteRemote bool   `json:"delete"`
		DryRun       bool   `json:"dry_run"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Provider == "" {
		return nil, fmt.Errorf("provider is required")
	}
	resolvedProvider, err := resolveProviderNameStrict(params.Provider)
	if err != nil {
		return nil, err
	}
	targetSource := model.TaskSource(resolvedProvider)

	// 检查 Provider 是否存在
	p, ok := s.providers[resolvedProvider]
	if !ok {
		return nil, fmt.Errorf("provider %s not found or not authenticated", resolvedProvider)
	}

	// 获取本地任务
	localTasks, err := s.taskStore.ListTasks(ctx, storage.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list local tasks: %w", err)
	}

	result := &SyncPushResult{
		Provider: resolvedProvider,
	}

	// 创建本地任务的 SourceRawID 集合
	localSourceRawIDs := s.buildLocalSourceRawIDsForProvider(localTasks, targetSource)

	// 获取远程任务列表
	taskLists, err := p.ListTaskLists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote task lists: %w", err)
	}

	// 查找默认列表
	defaultListID := s.findDefaultListID(taskLists)

	// Google 采用两阶段：先推父任务，再 pull，再推子任务，避免子任务落在根层。
	if targetSource == model.SourceGoogle {
		planTaskToLocalID := buildPlanTaskToLocalIDMap(localTasks)
		parentTasks, childTasks := splitTasksByParentRelation(localTasks, planTaskToLocalID)
		s.pushLocalTasks(ctx, p, parentTasks, defaultListID, targetSource, params.DryRun, result)
		if len(childTasks) > 0 {
			if !params.DryRun {
				if err := s.pullProviderTasksIntoLocal(ctx, p, resolvedProvider); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("pull-before-children: %v", err))
				}
			}
			s.pushLocalTasks(ctx, p, childTasks, defaultListID, targetSource, params.DryRun, result)
		}
	} else {
		s.pushLocalTasks(ctx, p, localTasks, defaultListID, targetSource, params.DryRun, result)
	}

	// 删除远程多余任务
	if params.DeleteRemote {
		s.deleteRemoteTasks(ctx, p, taskLists, localSourceRawIDs, params.DryRun, result)
	}

	if params.DryRun {
		result.DryRun = true
		result.Message = "这是模拟执行，未实际修改数据"
	}

	jsonResult, _ := toJSON(result)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: jsonResult}},
	}, nil
}

// buildLocalSourceRawIDsForProvider 构建目标 provider 的 SourceRawID 集合
func (s *Server) buildLocalSourceRawIDsForProvider(tasks []model.Task, source model.TaskSource) map[string]bool {
	ids := make(map[string]bool)
	for _, task := range tasks {
		if task.Source != "" && task.Source != source {
			continue
		}
		if task.SourceRawID != "" {
			ids[task.SourceRawID] = true
		}
	}
	return ids
}

// findDefaultListID 查找默认任务列表ID
func (s *Server) findDefaultListID(taskLists []model.TaskList) string {
	for _, list := range taskLists {
		if list.Name == "我的任务" || list.Name == "My Tasks" || list.ID == "@default" {
			return list.ID
		}
	}
	if len(taskLists) > 0 {
		return taskLists[0].ID
	}
	return ""
}

// pushLocalTasks 推送本地任务到远程
func (s *Server) pushLocalTasks(ctx context.Context, p provider.Provider, tasks []model.Task, defaultListID string, source model.TaskSource, dryRun bool, result *SyncPushResult) {
	planTaskToLocalID := buildPlanTaskToLocalIDMap(tasks)
	ordered := orderTasksByParent(tasks, planTaskToLocalID)
	remoteByLocalID := make(map[string]string, len(ordered))
	for _, task := range ordered {
		// 只同步本地任务或目标 provider 的任务，避免跨 provider 推送污染。
		if task.Source != "" && task.Source != source && task.Source != model.SourceLocal {
			continue
		}
		listID := task.ListID
		if listID == "" {
			listID = defaultListID
		}

		parentRemoteID := ""
		parentLocalID := localParentIDFromTask(task, planTaskToLocalID)
		if parentLocalID != "" {
			parentRemoteID = remoteByLocalID[parentLocalID]
			if parentRemoteID == "" {
				parentRemoteID = s.findRemoteIDByLocalParent(ctx, tasks, parentLocalID, source)
			}
			// 若 parent_id 已经是 google-<listID>-<rawID> 结构，直接提取 rawID。
			if parentRemoteID == "" && source == model.SourceGoogle {
				parentRemoteID = toGoogleParentRawID(parentLocalID, listID)
			}
			// 子任务必须绑定父任务；若父任务映射缺失，跳过本次创建，避免散落到根层。
			if parentRemoteID == "" {
				result.Errors = append(result.Errors, fmt.Sprintf("skip child %s: parent remote id not resolved", task.ID))
				continue
			}
		}

		// Microsoft To Do：子任务要映射为父任务的 checklist step，而非独立任务。
		if source == model.SourceMicrosoft && parentRemoteID != "" {
			if err := s.syncMicrosoftChecklistStep(ctx, p, listID, parentRemoteID, &task, dryRun, result); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("step %s: %v", task.ID, err))
			}
			continue
		}

		taskToSync := sanitizeTaskForRemote(task)
		// Google Tasks：子任务使用 parent 字段创建在父任务下。
		if source == model.SourceGoogle && parentRemoteID != "" {
			taskToSync.ParentID = &parentRemoteID
		}

		if taskToSync.SourceRawID != "" {
			// 任务已存在远程，检查是否需要更新
			existingTask, err := p.GetTask(ctx, listID, taskToSync.SourceRawID)
			if err == nil && existingTask != nil {
				if existingTask.UpdatedAt.Before(taskToSync.UpdatedAt) {
					if !dryRun {
						_, err := p.UpdateTask(ctx, listID, &taskToSync)
						if err != nil {
							result.Errors = append(result.Errors, fmt.Sprintf("update %s: %v", taskToSync.ID, err))
							continue
						}
					}
					result.Updated++
				}
				remoteByLocalID[task.ID] = taskToSync.SourceRawID
				continue
			}
		}

		// 创建新任务
		if !dryRun {
			createdTask, err := p.CreateTask(ctx, listID, &taskToSync)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("create %s: %v", taskToSync.ID, err))
				continue
			}
			// 更新本地任务信息
			task.SourceRawID = createdTask.SourceRawID
			task.ListID = listID
			task.Source = source
			_ = s.taskStore.SaveTask(ctx, &task)
			remoteByLocalID[task.ID] = task.SourceRawID
		} else if task.SourceRawID != "" {
			remoteByLocalID[task.ID] = task.SourceRawID
		}
		result.Pushed++
	}
}

func (s *Server) syncMicrosoftChecklistStep(ctx context.Context, p provider.Provider, listID, parentRemoteID string, task *model.Task, dryRun bool, result *SyncPushResult) error {
	msProvider, ok := p.(*msprovider.Provider)
	if !ok {
		return fmt.Errorf("provider is not microsoft")
	}
	if listID == "" {
		return fmt.Errorf("list id is required for microsoft checklist step")
	}

	stepID := ""
	if task.Metadata != nil && task.Metadata.CustomFields != nil {
		if v, ok := task.Metadata.CustomFields["tb_ms_step_id"]; ok {
			stepID = strings.TrimSpace(fmt.Sprint(v))
		}
	}
	if stepID == "" {
		stepID = parseMicrosoftStepID(task.SourceRawID)
	}
	isChecked := task.Status == model.StatusCompleted

	cleanTitle := sanitizeMarkdownText(task.Title)
	if dryRun {
		if stepID == "" {
			result.Pushed++
		} else {
			result.Updated++
		}
		return nil
	}

	if stepID == "" {
		item, err := msProvider.CreateChecklistItem(ctx, listID, parentRemoteID, cleanTitle, isChecked)
		if err != nil {
			return err
		}
		originalID := task.ID
		canonicalID := buildMicrosoftStepLocalID(parentRemoteID, item.ID)
		if task.Metadata == nil {
			task.Metadata = &model.TaskMetadata{Version: "1.0", CustomFields: map[string]interface{}{}}
		}
		if task.Metadata.CustomFields == nil {
			task.Metadata.CustomFields = map[string]interface{}{}
		}
		task.Metadata.CustomFields["tb_ms_step_id"] = item.ID
		task.Metadata.CustomFields["tb_ms_parent_source_raw_id"] = parentRemoteID
		task.Metadata.LocalID = canonicalID
		task.ID = canonicalID
		task.SourceRawID = "ms_step:" + item.ID
		task.Source = model.SourceMicrosoft
		task.ListID = listID
		_ = s.taskStore.SaveTask(ctx, task)
		if originalID != "" && originalID != canonicalID {
			_ = s.taskStore.DeleteTask(ctx, originalID)
		}
		result.Pushed++
		return nil
	}

	if _, err := msProvider.UpdateChecklistItem(ctx, listID, parentRemoteID, stepID, cleanTitle, isChecked); err != nil {
		return err
	}
	canonicalID := buildMicrosoftStepLocalID(parentRemoteID, stepID)
	if task.Metadata == nil {
		task.Metadata = &model.TaskMetadata{Version: "1.0", CustomFields: map[string]interface{}{}}
	}
	if task.Metadata.CustomFields == nil {
		task.Metadata.CustomFields = map[string]interface{}{}
	}
	task.Metadata.CustomFields["tb_ms_step_id"] = stepID
	task.Metadata.CustomFields["tb_ms_parent_source_raw_id"] = parentRemoteID
	task.Metadata.LocalID = canonicalID
	task.ID = canonicalID
	task.SourceRawID = "ms_step:" + stepID
	task.Source = model.SourceMicrosoft
	task.ListID = listID
	_ = s.taskStore.SaveTask(ctx, task)
	result.Updated++
	return nil
}

func orderTasksByParent(tasks []model.Task, planTaskToLocalID map[string]string) []model.Task {
	byID := make(map[string]model.Task, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}

	visited := make(map[string]bool, len(tasks))
	inStack := make(map[string]bool, len(tasks))
	ordered := make([]model.Task, 0, len(tasks))

	var visit func(id string)
	visit = func(id string) {
		if visited[id] {
			return
		}
		if inStack[id] {
			return
		}
		task, ok := byID[id]
		if !ok {
			return
		}
		inStack[id] = true
		parentLocalID := localParentIDFromTask(task, planTaskToLocalID)
		if parentLocalID != "" {
			visit(parentLocalID)
		}
		inStack[id] = false
		visited[id] = true
		ordered = append(ordered, task)
	}

	for _, task := range tasks {
		visit(task.ID)
	}
	return ordered
}

func (s *Server) findRemoteIDByLocalParent(ctx context.Context, tasks []model.Task, parentLocalID string, source model.TaskSource) string {
	for _, task := range tasks {
		if task.ID != parentLocalID {
			continue
		}
		if task.SourceRawID != "" && (task.Source == source || task.Source == model.SourceLocal || task.Source == "") {
			return task.SourceRawID
		}
		break
	}

	// 第二阶段（仅子任务切片）时，父任务可能不在当前 tasks 切片里，直接回查存储。
	if s.taskStore != nil {
		if parentTask, err := s.taskStore.GetTask(ctx, parentLocalID); err == nil && parentTask != nil {
			if parentTask.SourceRawID != "" && (parentTask.Source == source || parentTask.Source == model.SourceLocal || parentTask.Source == "") {
				return parentTask.SourceRawID
			}
		}
	}

	// 兼容按 plan_task_id 关联的历史数据：父引用可能不是本地 task.ID。
	if s.taskStore != nil {
		all, err := s.taskStore.ListTasks(ctx, storage.ListOptions{})
		if err == nil {
			for _, task := range all {
				if getCustomFieldString(task, "tb_plan_task_id") != parentLocalID {
					continue
				}
				if task.SourceRawID != "" && (task.Source == source || task.Source == model.SourceLocal || task.Source == "") {
					return task.SourceRawID
				}
			}
		}
	}
	return ""
}

func buildPlanTaskToLocalIDMap(tasks []model.Task) map[string]string {
	result := make(map[string]string, len(tasks))
	for _, task := range tasks {
		planTaskID := getCustomFieldString(task, "tb_plan_task_id")
		if planTaskID == "" {
			continue
		}
		result[planTaskID] = task.ID
	}
	return result
}

// localParentIDFromTask 兼容两种父子表示：
// 1. 直接 ParentID（推荐）
// 2. 通过 tb_parent_plan_task_id -> tb_plan_task_id 映射恢复（兼容历史数据）
func localParentIDFromTask(task model.Task, planTaskToLocalID map[string]string) string {
	if task.ParentID != nil {
		if parentID := strings.TrimSpace(*task.ParentID); parentID != "" {
			return parentID
		}
	}
	parentPlanTaskID := getCustomFieldString(task, "tb_parent_plan_task_id")
	if parentPlanTaskID == "" {
		return ""
	}
	return strings.TrimSpace(planTaskToLocalID[parentPlanTaskID])
}

func getCustomFieldString(task model.Task, key string) string {
	if task.Metadata == nil || task.Metadata.CustomFields == nil {
		return ""
	}
	raw, ok := task.Metadata.CustomFields[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}

func toGoogleParentRawID(parentID, listID string) string {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" || listID == "" {
		return ""
	}
	prefix := "google-" + listID + "-"
	if strings.HasPrefix(parentID, prefix) {
		return strings.TrimPrefix(parentID, prefix)
	}
	return ""
}

func splitTasksByParentRelation(tasks []model.Task, planTaskToLocalID map[string]string) ([]model.Task, []model.Task) {
	parents := make([]model.Task, 0, len(tasks))
	children := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		if localParentIDFromTask(task, planTaskToLocalID) == "" {
			parents = append(parents, task)
			continue
		}
		children = append(children, task)
	}
	return parents, children
}

// pullProviderTasksIntoLocal 拉取远端任务到本地，用于在二阶段同步前刷新父任务的远端映射。
func (s *Server) pullProviderTasksIntoLocal(ctx context.Context, p provider.Provider, providerName string) error {
	taskLists, err := p.ListTaskLists(ctx)
	if err != nil {
		return fmt.Errorf("failed to list remote task lists: %w", err)
	}
	for _, list := range taskLists {
		_ = s.taskStore.SaveTaskList(ctx, &list)
		tasks, err := p.ListTasks(ctx, list.ID, provider.ListOptions{})
		if err != nil {
			continue
		}
		for _, task := range tasks {
			if task.ListID == "" {
				task.ListID = list.ID
			}
			if task.ListName == "" {
				task.ListName = list.Name
			}
			if task.Source == "" {
				task.Source = model.TaskSource(providerName)
			}
			_ = s.taskStore.SaveTask(ctx, &task)
		}
	}
	return nil
}

var markdownCheckboxPrefixPattern = regexp.MustCompile(`^\s*\[\s*[xX ]?\s*\]\s*`)

// sanitizeTaskForRemote 在不修改本地任务结构的前提下，清洗远程展示文本中的 Markdown 装饰。
func sanitizeTaskForRemote(task model.Task) model.Task {
	task.Title = sanitizeMarkdownText(task.Title)
	task.Description = sanitizeMarkdownText(task.Description)
	return task
}

// sanitizeMarkdownText 去掉常见 Markdown 装饰（如 [ ]、**），避免远程任务出现原始标记。
func sanitizeMarkdownText(text string) string {
	out := strings.TrimSpace(text)
	if out == "" {
		return out
	}
	out = markdownCheckboxPrefixPattern.ReplaceAllString(out, "")
	out = strings.ReplaceAll(out, "**", "")
	out = strings.ReplaceAll(out, "__", "")
	out = strings.Join(strings.Fields(out), " ")
	return out
}

func parseMicrosoftStepID(sourceRawID string) string {
	raw := strings.TrimSpace(sourceRawID)
	if !strings.HasPrefix(raw, "ms_step:") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(raw, "ms_step:"))
}

func buildMicrosoftStepLocalID(parentRemoteID, stepID string) string {
	parent := strings.TrimSpace(parentRemoteID)
	step := strings.TrimSpace(stepID)
	if parent == "" || step == "" {
		return ""
	}
	return fmt.Sprintf("ms-step-%s-%s", parent, step)
}

// deleteRemoteTasks 删除远程多余任务
func (s *Server) deleteRemoteTasks(ctx context.Context, p provider.Provider, taskLists []model.TaskList, localSourceRawIDs map[string]bool, dryRun bool, result *SyncPushResult) {
	for _, list := range taskLists {
		remoteTasks, err := p.ListTasks(ctx, list.ID, provider.ListOptions{})
		if err != nil {
			continue
		}

		for _, remoteTask := range remoteTasks {
			if !localSourceRawIDs[remoteTask.SourceRawID] {
				if !dryRun {
					err := p.DeleteTask(ctx, list.ID, remoteTask.SourceRawID)
					if err != nil {
						result.Errors = append(result.Errors, fmt.Sprintf("delete %s: %v", remoteTask.SourceRawID, err))
						continue
					}
				}
				result.Deleted++
			}
		}
	}
}

// handleSyncPull 处理拉取同步请求
func (s *Server) handleSyncPull(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 解析参数
	var params struct {
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Provider == "" {
		return nil, fmt.Errorf("provider is required")
	}
	resolvedProvider, err := resolveProviderNameStrict(params.Provider)
	if err != nil {
		return nil, err
	}

	// 检查 Provider 是否存在
	p, ok := s.providers[resolvedProvider]
	if !ok {
		return nil, fmt.Errorf("provider %s not found or not authenticated", resolvedProvider)
	}

	result := map[string]interface{}{
		"provider": resolvedProvider,
		"pulled":   0,
		"errors":   []string{},
	}

	// 获取远程任务列表
	taskLists, err := p.ListTaskLists(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote task lists: %w", err)
	}

	// 从每个列表拉取任务
	for _, list := range taskLists {
		_ = s.taskStore.SaveTaskList(ctx, &list)

		tasks, err := p.ListTasks(ctx, list.ID, provider.ListOptions{})
		if err != nil {
			result["errors"] = append(result["errors"].([]string), fmt.Sprintf("list %s: %v", list.Name, err))
			continue
		}

		for _, task := range tasks {
			if task.ListID == "" {
				task.ListID = list.ID
			}
			if task.ListName == "" {
				task.ListName = list.Name
			}
			if task.Source == "" {
				task.Source = model.TaskSource(resolvedProvider)
			}

			if err := s.taskStore.SaveTask(ctx, &task); err != nil {
				result["errors"] = append(result["errors"].([]string), fmt.Sprintf("save %s: %v", task.ID, err))
				continue
			}
			result["pulled"] = result["pulled"].(int) + 1
		}
	}

	jsonResult, _ := toJSON(result)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: jsonResult}},
	}, nil
}

// ================ Provider 工具处理器 ================

// ProviderInfo Provider 信息
type ProviderInfo struct {
	Name         string   `json:"name"`
	ShortName    string   `json:"short_name"`
	DisplayName  string   `json:"display_name"`
	Description  string   `json:"description"`
	AuthType     string   `json:"auth_type"`
	Enabled      bool     `json:"enabled"`
	Connected    bool     `json:"connected"`
	Capabilities []string `json:"capabilities"`
}

// ServerInfo MCP 服务信息
type ServerInfo struct {
	Name         string              `json:"name"`
	Version      string              `json:"version"`
	Transport    string              `json:"transport"`
	Capabilities map[string][]string `json:"capabilities"`
	Tools        []string            `json:"tools"`
	Prompts      []string            `json:"prompts"`
	Resources    []string            `json:"resources"`
}

// handleGetServerInfo 返回 MCP 版本和能力信息，供 AI 识别当前功能范围
func (s *Server) handleGetServerInfo(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_ = ctx
	_ = req

	toolsMap := s.GetTools()
	tools := make([]string, 0, len(toolsMap))
	for name := range toolsMap {
		tools = append(tools, name)
	}
	sort.Strings(tools)

	promptsMap := s.GetPrompts()
	prompts := make([]string, 0, len(promptsMap))
	for name := range promptsMap {
		prompts = append(prompts, name)
	}
	sort.Strings(prompts)

	info := ServerInfo{
		Name:      s.config.Name,
		Version:   s.config.Version,
		Transport: s.config.Transport,
		Capabilities: map[string][]string{
			"task_management":    {"list_tasks", "list_task_lists", "create_task", "update_task", "delete_task", "complete_task"},
			"analysis":           {"analyze_quadrant", "analyze_priority"},
			"project_management": {"create_project", "list_projects", "split_project", "split_project_from_markdown", "confirm_project", "sync_project"},
			"sync":               {"sync_pull", "sync_push"},
			"provider":           {"list_providers", "get_provider_info", "get_provider_config_template"},
			"prompt":             {"get_prompt"},
			"server_meta":        {"get_server_info"},
		},
		Tools:     tools,
		Prompts:   prompts,
		Resources: []string{"taskbridge://tasks", "taskbridge://projects", "taskbridge://prompts"},
	}

	result, _ := toJSON(info)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// handleListProviders 处理列出 Providers 请求
func (s *Server) handleListProviders(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 从配置读取启用状态
	googleEnabled := false
	microsoftEnabled := false
	feishuEnabled := false
	ticktickEnabled := false
	didaEnabled := false
	todoistEnabled := false

	if s.providerConfig != nil {
		googleEnabled = s.providerConfig.Google.Enabled
		microsoftEnabled = s.providerConfig.Microsoft.Enabled
		feishuEnabled = s.providerConfig.Feishu.Enabled
		ticktickEnabled = s.providerConfig.TickTick.Enabled
		didaEnabled = s.providerConfig.Dida.Enabled
		todoistEnabled = s.providerConfig.Todoist.Enabled
	}

	providers := []ProviderInfo{
		{
			Name:         "google",
			ShortName:    "google",
			DisplayName:  "Google Tasks",
			Description:  "Google 任务管理服务",
			AuthType:     "OAuth2",
			Enabled:      googleEnabled,
			Capabilities: []string{"due_date", "task_lists", "subtasks_limited"},
		},
		{
			Name:         "microsoft",
			ShortName:    "ms",
			DisplayName:  "Microsoft To Do",
			Description:  "微软任务管理服务",
			AuthType:     "OAuth2",
			Enabled:      microsoftEnabled,
			Capabilities: []string{"due_date", "task_lists", "subtasks", "priority", "reminder"},
		},
		{
			Name:         "feishu",
			ShortName:    "feishu",
			DisplayName:  "飞书任务",
			Description:  "飞书任务管理",
			AuthType:     "App ID/Secret",
			Enabled:      feishuEnabled,
			Capabilities: []string{"due_date", "task_lists", "priority", "tags"},
		},
		{
			Name:         "ticktick",
			ShortName:    "tick",
			DisplayName:  "TickTick",
			Description:  "TickTick 任务管理",
			AuthType:     "API Token",
			Enabled:      ticktickEnabled,
			Capabilities: []string{"due_date", "task_lists", "subtasks", "priority", "tags", "reminder"},
		},
		{
			Name:         "dida",
			ShortName:    "tick_cn",
			DisplayName:  "Dida365",
			Description:  "滴答清单（国内）",
			AuthType:     "API Token",
			Enabled:      didaEnabled,
			Capabilities: []string{"due_date", "task_lists", "subtasks", "priority", "tags", "reminder"},
		},
		{
			Name:         "todoist",
			ShortName:    "todo",
			DisplayName:  "Todoist",
			Description:  "Todoist 任务管理",
			AuthType:     "API Token",
			Enabled:      todoistEnabled,
			Capabilities: []string{"due_date", "projects", "subtasks", "priority", "tags"},
		},
	}

	// 检查已认证的 Provider
	for i := range providers {
		if p, ok := s.providers[providers[i].Name]; ok {
			providers[i].Connected = p.IsAuthenticated()
		}
	}

	result, _ := toJSON(providers)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// handleGetProviderInfo 处理获取 Provider 详情请求
func (s *Server) handleGetProviderInfo(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var params struct {
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Provider == "" {
		return nil, fmt.Errorf("provider is required")
	}

	// 支持简写
	providerName := provider.ResolveProviderName(params.Provider)

	providerInfos := map[string]ProviderInfo{
		"google": {
			Name:        "google",
			ShortName:   "google",
			DisplayName: "Google Tasks",
			Description: "Google 任务管理服务",
			AuthType:    "OAuth2",
			Capabilities: []string{
				"✅ 截止日期",
				"✅ 任务列表",
				"✅ 子任务（有限支持）",
				"❌ 优先级（不支持）",
				"❌ 标签（不支持）",
				"❌ 增量同步（不支持）",
			},
		},
		"microsoft": {
			Name:        "microsoft",
			ShortName:   "ms",
			DisplayName: "Microsoft To Do",
			Description: "微软任务管理服务",
			AuthType:    "OAuth2",
			Capabilities: []string{
				"✅ 截止日期",
				"✅ 任务列表",
				"✅ 子任务",
				"✅ 优先级",
				"✅ 提醒",
				"❌ 标签（不支持）",
			},
		},
		"feishu": {
			Name:        "feishu",
			ShortName:   "feishu",
			DisplayName: "飞书任务",
			Description: "飞书任务管理",
			AuthType:    "App ID/Secret",
			Capabilities: []string{
				"✅ 截止日期",
				"✅ 任务列表",
				"✅ 优先级",
				"✅ 标签",
				"❌ 子任务（有限支持）",
			},
		},
		"ticktick": {
			Name:        "ticktick",
			ShortName:   "tick",
			DisplayName: "TickTick",
			Description: "TickTick 任务管理",
			AuthType:    "API Token",
			Capabilities: []string{
				"✅ 截止日期",
				"✅ 任务列表",
				"✅ 子任务",
				"✅ 优先级",
				"✅ 标签",
				"✅ 提醒",
			},
		},
		"dida": {
			Name:        "dida",
			ShortName:   "tick_cn",
			DisplayName: "Dida365",
			Description: "滴答清单（国内）",
			AuthType:    "API Token",
			Capabilities: []string{
				"✅ 截止日期",
				"✅ 任务列表",
				"✅ 子任务",
				"✅ 优先级",
				"✅ 标签",
				"✅ 提醒",
			},
		},
		"todoist": {
			Name:        "todoist",
			ShortName:   "todo",
			DisplayName: "Todoist",
			Description: "Todoist 任务管理",
			AuthType:    "API Token",
			Capabilities: []string{
				"✅ 截止日期",
				"✅ 项目",
				"✅ 子任务",
				"✅ 优先级",
				"✅ 标签",
			},
		},
	}

	info, ok := providerInfos[providerName]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", params.Provider)
	}

	// 检查认证状态
	if p, ok := s.providers[providerName]; ok {
		info.Enabled = true
		info.Connected = p.IsAuthenticated()
	}

	result, _ := toJSON(info)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// handleGetProviderConfigTemplate 处理获取配置模板请求
func (s *Server) handleGetProviderConfigTemplate(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var params struct {
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Provider == "" {
		return nil, fmt.Errorf("provider is required")
	}

	providerName := provider.ResolveProviderName(params.Provider)

	templates := map[string]interface{}{
		"google": map[string]interface{}{
			"description": "Google Tasks 使用 OAuth2 认证",
			"auth_type":   "OAuth2",
			"steps": []string{
				"1. 运行 'taskbridge auth login google'",
				"2. 在浏览器中完成 Google 登录授权",
				"3. 授权完成后自动获取 token",
			},
			"required_fields": []string{},
			"optional_fields": []string{},
		},
		"microsoft": map[string]interface{}{
			"description": "Microsoft To Do 使用 OAuth2 认证",
			"auth_type":   "OAuth2",
			"steps": []string{
				"1. 运行 'taskbridge auth login microsoft'",
				"2. 在浏览器中完成 Microsoft 登录授权",
				"3. 授权完成后自动获取 token",
			},
			"required_fields": []string{},
			"optional_fields": []string{},
		},
		"todoist": map[string]interface{}{
			"description": "Todoist 使用 API Token 认证",
			"auth_type":   "API Token",
			"steps": []string{
				"1. 访问 https://todoist.com/app/settings/integrations/developer",
				"2. 复制 API Token",
			},
			"required_fields": []string{"api_token"},
			"optional_fields": []string{},
			"config_template": map[string]string{
				"api_token": "你的 Todoist API Token",
			},
		},
		"ticktick": map[string]interface{}{
			"description": "TickTick 使用 API Token 认证",
			"auth_type":   "API Token",
			"steps": []string{
				"1. 登录 TickTick 开发者平台",
				"2. 创建或查看个人 API Token",
			},
			"required_fields": []string{"api_token"},
			"optional_fields": []string{},
			"config_template": map[string]string{
				"api_token": "你的 TickTick API Token（通常以 tp_ 开头）",
			},
		},
		"dida": map[string]interface{}{
			"description": "Dida365 使用 API Token 认证",
			"auth_type":   "API Token",
			"steps": []string{
				"1. 登录 Dida365 开发者平台",
				"2. 创建或查看个人 API Token",
			},
			"required_fields": []string{"api_token"},
			"optional_fields": []string{},
			"config_template": map[string]string{
				"api_token": "你的 Dida API Token（通常以 dp_ 开头）",
			},
		},
		"feishu": map[string]interface{}{
			"description": "飞书任务使用 App ID/Secret 认证",
			"auth_type":   "App ID/Secret",
			"steps": []string{
				"1. 访问飞书开放平台 https://open.feishu.cn",
				"2. 创建企业自建应用",
				"3. 获取 App ID 和 App Secret",
				"4. 配置应用权限：任务相关权限",
			},
			"required_fields": []string{"app_id", "app_secret"},
			"optional_fields": []string{},
			"config_template": map[string]string{
				"app_id":     "你的飞书应用 App ID",
				"app_secret": "你的飞书应用 App Secret",
			},
		},
	}

	template, ok := templates[providerName]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", params.Provider)
	}

	result, _ := toJSON(template)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// ================ 辅助函数 ================

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}

func generateProjectID() string {
	return fmt.Sprintf("proj_%d", time.Now().UnixNano())
}

func generatePlanID() string {
	return fmt.Sprintf("plan_%d", time.Now().UnixNano())
}

type compactTask struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Status    string     `json:"status"`
	Source    string     `json:"source"`
	ListID    string     `json:"list_id,omitempty"`
	ListName  string     `json:"list_name,omitempty"`
	Priority  int        `json:"priority,omitempty"`
	Quadrant  int        `json:"quadrant,omitempty"`
	DueDate   *time.Time `json:"due_date,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
	Tags      []string   `json:"tags,omitempty"`
}

func toCompactTasks(tasks []model.Task) []compactTask {
	result := make([]compactTask, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, compactTask{
			ID:        task.ID,
			Title:     task.Title,
			Status:    string(task.Status),
			Source:    string(task.Source),
			ListID:    task.ListID,
			ListName:  task.ListName,
			Priority:  int(task.Priority),
			Quadrant:  int(task.Quadrant),
			DueDate:   task.DueDate,
			UpdatedAt: task.UpdatedAt,
			Tags:      task.Tags,
		})
	}
	return result
}

func getString(raw map[string]json.RawMessage, key string) string {
	if len(raw) == 0 {
		return ""
	}
	value, ok := raw[key]
	if !ok || len(value) == 0 {
		return ""
	}
	var out string
	if err := json.Unmarshal(value, &out); err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func getStringSlice(raw map[string]json.RawMessage, key string) []string {
	if len(raw) == 0 {
		return nil
	}
	value, ok := raw[key]
	if !ok || len(value) == 0 {
		return nil
	}

	var single string
	if err := json.Unmarshal(value, &single); err == nil {
		single = strings.TrimSpace(single)
		if single == "" {
			return nil
		}
		return []string{single}
	}

	var array []string
	if err := json.Unmarshal(value, &array); err == nil {
		result := make([]string, 0, len(array))
		for _, v := range array {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			result = append(result, v)
		}
		if len(result) == 0 {
			return nil
		}
		return result
	}

	return nil
}

func getInt(raw map[string]json.RawMessage, key string) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	value, ok := raw[key]
	if !ok || len(value) == 0 {
		return 0, false
	}

	var out int
	if err := json.Unmarshal(value, &out); err != nil {
		return 0, false
	}
	return out, true
}

func getIntSlice(raw map[string]json.RawMessage, key string) []int {
	if len(raw) == 0 {
		return nil
	}
	value, ok := raw[key]
	if !ok || len(value) == 0 {
		return nil
	}

	var single int
	if err := json.Unmarshal(value, &single); err == nil {
		return []int{single}
	}

	var array []int
	if err := json.Unmarshal(value, &array); err == nil {
		if len(array) == 0 {
			return nil
		}
		return array
	}

	return nil
}

func getBool(raw map[string]json.RawMessage, key string) (bool, bool) {
	if len(raw) == 0 {
		return false, false
	}
	value, ok := raw[key]
	if !ok || len(value) == 0 {
		return false, false
	}

	var out bool
	if err := json.Unmarshal(value, &out); err != nil {
		return false, false
	}
	return out, true
}

func resolveProviderNameStrict(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", nil
	}

	resolved := provider.ResolveProviderName(trimmed)
	if !provider.IsValidProvider(resolved) {
		return "", fmt.Errorf("invalid provider: %s", name)
	}

	return resolved, nil
}

func buildProviderListKey(source, listID string) string {
	return source + "::" + listID
}

func pickHorizonDays(primary, fallback int) int {
	if primary > 0 {
		return primary
	}
	if fallback > 0 {
		return fallback
	}
	return 14
}

func clampPriority(priority int) model.Priority {
	if priority < 1 {
		return model.PriorityLow
	}
	if priority > 4 {
		return model.PriorityUrgent
	}
	return model.Priority(priority)
}

func clampQuadrant(quadrant int) model.Quadrant {
	if quadrant < 1 {
		return model.QuadrantNotUrgentImportant
	}
	if quadrant > 4 {
		return model.QuadrantNotUrgentNotImportant
	}
	return model.Quadrant(quadrant)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func taskBelongsToProject(task model.Task, projectID string) bool {
	if task.Metadata == nil || task.Metadata.CustomFields == nil {
		return false
	}
	raw, ok := task.Metadata.CustomFields["tb_project_id"]
	if !ok {
		return false
	}
	switch v := raw.(type) {
	case string:
		return v == projectID
	case fmt.Stringer:
		return v.String() == projectID
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64) == projectID
	case int:
		return strconv.Itoa(v) == projectID
	default:
		return fmt.Sprint(v) == projectID
	}
}
