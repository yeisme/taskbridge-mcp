package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/internal/storage"
	pkgconfig "github.com/yeisme/taskbridge/pkg/config"
)

type overdueCandidate struct {
	TaskID        string     `json:"task_id"`
	Title         string     `json:"title"`
	Status        string     `json:"status"`
	Source        string     `json:"source"`
	ListID        string     `json:"list_id,omitempty"`
	Priority      int        `json:"priority,omitempty"`
	Quadrant      int        `json:"quadrant,omitempty"`
	DueDate       *time.Time `json:"due_date,omitempty"`
	DaysOverdue   int        `json:"days_overdue"`
	SevereOverdue bool       `json:"severe_overdue"`
}

type overdueActionItem struct {
	TaskID  string `json:"task_id"`
	Type    string `json:"type"`
	DueDate string `json:"due_date"`
}

func (s *Server) handleAnalyzeOverdueHealth(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.taskStore == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: `{"summary":{"overdue_count":0,"severe_overdue_count":0,"is_warning":false,"is_overload":false},"candidates":[]}`}},
		}, nil
	}

	cfg := s.effectiveIntelligenceConfig()

	var rawArgs map[string]json.RawMessage
	if req != nil && req.Params != nil && len(req.Params.Arguments) > 0 {
		if err := json.Unmarshal(req.Params.Arguments, &rawArgs); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	includeSuggestions := true
	if v, ok := getBool(rawArgs, "include_suggestions"); ok {
		includeSuggestions = v
	}

	query := storage.Query{
		Statuses: []model.TaskStatus{model.StatusTodo, model.StatusInProgress},
		ListIDs:  getStringSlice(rawArgs, "list_id"),
	}
	if source := getString(rawArgs, "source"); source != "" {
		resolvedSource, err := resolveProviderNameStrict(source)
		if err != nil {
			return nil, err
		}
		query.Sources = []model.TaskSource{model.TaskSource(resolvedSource)}
	}

	tasks, err := s.taskStore.QueryTasks(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}

	now := time.Now()
	candidates := make([]overdueCandidate, 0)
	overdueCount := 0
	severeCount := 0
	for _, task := range tasks {
		if task.DueDate == nil {
			continue
		}
		days := calcOverdueDays(task.DueDate, now)
		if days <= 0 {
			continue
		}
		overdueCount++
		severe := days >= cfg.Overdue.SevereDays
		if severe {
			severeCount++
		}
		if len(candidates) < cfg.Overdue.MaxCandidates {
			candidates = append(candidates, overdueCandidate{
				TaskID:        task.ID,
				Title:         task.Title,
				Status:        string(task.Status),
				Source:        string(task.Source),
				ListID:        task.ListID,
				Priority:      int(task.Priority),
				Quadrant:      int(task.Quadrant),
				DueDate:       task.DueDate,
				DaysOverdue:   days,
				SevereOverdue: severe,
			})
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].DaysOverdue == candidates[j].DaysOverdue {
			return candidates[i].Priority > candidates[j].Priority
		}
		return candidates[i].DaysOverdue > candidates[j].DaysOverdue
	})

	isWarning := overdueCount > cfg.Overdue.WarningThreshold
	isOverload := overdueCount > cfg.Overdue.OverloadThreshold

	result := map[string]interface{}{
		"summary": map[string]interface{}{
			"overdue_count":        overdueCount,
			"severe_overdue_count": severeCount,
			"is_warning":           isWarning,
			"is_overload":          isOverload,
		},
		"candidates": candidates,
		"config_applied": map[string]interface{}{
			"warning_threshold":  cfg.Overdue.WarningThreshold,
			"overload_threshold": cfg.Overdue.OverloadThreshold,
			"severe_days":        cfg.Overdue.SevereDays,
			"max_candidates":     cfg.Overdue.MaxCandidates,
		},
	}

	if includeSuggestions {
		result["actions"] = []string{"defer", "reschedule", "delete", "split_then_schedule"}
		result["questions"] = buildOverdueQuestions(overdueCount, isOverload)
	}

	jsonResult, err := toJSON(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: jsonResult}},
	}, nil
}

func (s *Server) handleResolveOverdueTasks(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.taskStore == nil {
		return nil, fmt.Errorf("task storage not available")
	}

	var params struct {
		Actions      []overdueActionItem `json:"actions"`
		DryRun       bool                `json:"dry_run"`
		ConfirmToken string              `json:"confirm_token"`
	}
	if req == nil || req.Params == nil || len(req.Params.Arguments) == 0 {
		return nil, fmt.Errorf("actions is required")
	}
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	if len(params.Actions) == 0 {
		return nil, fmt.Errorf("actions is required")
	}

	cfg := s.effectiveIntelligenceConfig()
	allowDelete := !cfg.Overdue.AskBeforeDelete || strings.TrimSpace(params.ConfirmToken) == "confirm-delete"
	if params.DryRun {
		allowDelete = true
	}

	result := map[string]interface{}{
		"total":               len(params.Actions),
		"updated":             0,
		"deferred":            0,
		"rescheduled":         0,
		"deleted":             0,
		"split_suggested":     0,
		"skipped":             0,
		"errors":              []string{},
		"dry_run":             params.DryRun,
		"requires_confirm":    cfg.Overdue.AskBeforeDelete,
		"confirm_token_match": allowDelete,
	}

	appendErr := func(msg string) {
		result["errors"] = append(result["errors"].([]string), msg)
	}

	now := time.Now()
	for _, action := range params.Actions {
		taskID := strings.TrimSpace(action.TaskID)
		actionType := strings.ToLower(strings.TrimSpace(action.Type))
		if taskID == "" || actionType == "" {
			result["skipped"] = result["skipped"].(int) + 1
			appendErr("invalid action: task_id and type are required")
			continue
		}

		task, err := s.taskStore.GetTask(ctx, taskID)
		if err != nil {
			result["skipped"] = result["skipped"].(int) + 1
			appendErr(fmt.Sprintf("task not found: %s", taskID))
			continue
		}

		switch actionType {
		case "defer":
			task.Status = model.StatusDeferred
			task.UpdatedAt = now
			if !params.DryRun {
				if err := s.taskStore.SaveTask(ctx, task); err != nil {
					result["skipped"] = result["skipped"].(int) + 1
					appendErr(fmt.Sprintf("defer %s failed: %v", taskID, err))
					continue
				}
			}
			result["deferred"] = result["deferred"].(int) + 1
			result["updated"] = result["updated"].(int) + 1
		case "reschedule":
			dueDate, err := time.Parse("2006-01-02", strings.TrimSpace(action.DueDate))
			if err != nil {
				result["skipped"] = result["skipped"].(int) + 1
				appendErr(fmt.Sprintf("invalid due_date for %s: %s", taskID, action.DueDate))
				continue
			}
			task.DueDate = &dueDate
			task.UpdatedAt = now
			if !params.DryRun {
				if err := s.taskStore.SaveTask(ctx, task); err != nil {
					result["skipped"] = result["skipped"].(int) + 1
					appendErr(fmt.Sprintf("reschedule %s failed: %v", taskID, err))
					continue
				}
			}
			result["rescheduled"] = result["rescheduled"].(int) + 1
			result["updated"] = result["updated"].(int) + 1
		case "delete":
			if !allowDelete {
				result["skipped"] = result["skipped"].(int) + 1
				appendErr(fmt.Sprintf("delete %s blocked: confirm_token=confirm-delete required", taskID))
				continue
			}
			if !params.DryRun {
				if err := s.taskStore.DeleteTask(ctx, taskID); err != nil {
					result["skipped"] = result["skipped"].(int) + 1
					appendErr(fmt.Sprintf("delete %s failed: %v", taskID, err))
					continue
				}
			}
			result["deleted"] = result["deleted"].(int) + 1
		case "split_then_schedule":
			if task.Metadata == nil {
				task.Metadata = &model.TaskMetadata{Version: "1.0", CustomFields: map[string]interface{}{}}
			}
			if task.Metadata.CustomFields == nil {
				task.Metadata.CustomFields = map[string]interface{}{}
			}
			task.Metadata.CustomFields["tb_split_suggested"] = true
			task.Metadata.CustomFields["tb_split_suggested_at"] = now.Format(time.RFC3339)
			task.UpdatedAt = now
			if !params.DryRun {
				if err := s.taskStore.SaveTask(ctx, task); err != nil {
					result["skipped"] = result["skipped"].(int) + 1
					appendErr(fmt.Sprintf("split_then_schedule %s failed: %v", taskID, err))
					continue
				}
			}
			result["split_suggested"] = result["split_suggested"].(int) + 1
			result["updated"] = result["updated"].(int) + 1
		default:
			result["skipped"] = result["skipped"].(int) + 1
			appendErr(fmt.Sprintf("unsupported action type for %s: %s", taskID, actionType))
		}
	}

	jsonResult, err := toJSON(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: jsonResult}}}, nil
}

func (s *Server) handleRebalanceLongTermTasks(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.taskStore == nil {
		return nil, fmt.Errorf("task storage not available")
	}

	cfg := s.effectiveIntelligenceConfig()

	var rawArgs map[string]json.RawMessage
	if req != nil && req.Params != nil && len(req.Params.Arguments) > 0 {
		if err := json.Unmarshal(req.Params.Arguments, &rawArgs); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	dryRun, _ := getBool(rawArgs, "dry_run")
	listIDs := getStringSlice(rawArgs, "list_id")

	query := storage.Query{
		Statuses: []model.TaskStatus{model.StatusTodo, model.StatusInProgress},
		ListIDs:  listIDs,
	}
	if source := getString(rawArgs, "source"); source != "" {
		resolvedSource, err := resolveProviderNameStrict(source)
		if err != nil {
			return nil, err
		}
		query.Sources = []model.TaskSource{model.TaskSource(resolvedSource)}
	}

	tasks, err := s.taskStore.QueryTasks(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}

	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	windowEnd := startOfToday.AddDate(0, 0, cfg.LongTerm.ShortTermWindowDays)

	shortTerm := make([]model.Task, 0)
	longTerm := make([]model.Task, 0)
	for _, task := range tasks {
		if task.DueDate != nil {
			if !task.DueDate.Before(startOfToday) && !task.DueDate.After(windowEnd) {
				shortTerm = append(shortTerm, task)
			}
			continue
		}
		if calcAgeDays(task.CreatedAt, now) >= cfg.LongTerm.MinAgeDays {
			longTerm = append(longTerm, task)
		}
	}

	sort.SliceStable(longTerm, func(i, j int) bool {
		scoreI := scoreLongTermTask(longTerm[i], now)
		scoreJ := scoreLongTermTask(longTerm[j], now)
		if scoreI == scoreJ {
			return longTerm[i].UpdatedAt.After(longTerm[j].UpdatedAt)
		}
		return scoreI > scoreJ
	})

	promotedIDs := make([]string, 0)
	retainedIDs := make([]string, 0)
	adjustedIDs := make([]string, 0)

	mode := "balanced"
	if len(shortTerm) < cfg.LongTerm.ShortTermMin {
		mode = "shortage"
		promoteCount := cfg.LongTerm.PromoteCountWhenShortage
		if promoteCount <= 0 {
			promoteCount = 1
		}
		if promoteCount > len(longTerm) {
			promoteCount = len(longTerm)
		}
		for i := 0; i < promoteCount; i++ {
			task := longTerm[i]
			due := startOfToday.AddDate(0, 0, i+1)
			task.DueDate = &due
			task.UpdatedAt = now
			if task.Status == model.StatusDeferred {
				task.Status = model.StatusTodo
			}
			if !dryRun {
				if err := s.taskStore.SaveTask(ctx, &task); err != nil {
					continue
				}
			}
			promotedIDs = append(promotedIDs, task.ID)
		}
	}

	if len(shortTerm) > cfg.LongTerm.ShortTermMax {
		mode = "overflow"
		retainCount := cfg.LongTerm.RetainCountWhenOverflow
		if retainCount <= 0 {
			retainCount = 1
		}
		if retainCount > len(longTerm) {
			retainCount = len(longTerm)
		}
		for i := 0; i < retainCount; i++ {
			retainedIDs = append(retainedIDs, longTerm[i].ID)
		}
		for i := retainCount; i < len(longTerm); i++ {
			task := longTerm[i]
			if strings.EqualFold(strings.TrimSpace(cfg.LongTerm.OverflowStrategy), "backlog_tag") {
				if !containsFold(task.Tags, "backlog") {
					task.Tags = append(task.Tags, "backlog")
				}
			} else {
				task.Status = model.StatusDeferred
			}
			task.UpdatedAt = now
			if !dryRun {
				if err := s.taskStore.SaveTask(ctx, &task); err != nil {
					continue
				}
			}
			adjustedIDs = append(adjustedIDs, task.ID)
		}
	}

	shortTermAfter := len(shortTerm)
	if mode == "shortage" {
		shortTermAfter += len(promotedIDs)
	}

	result := map[string]interface{}{
		"mode":               mode,
		"short_term_before":  len(shortTerm),
		"short_term_after":   shortTermAfter,
		"long_term_pool":     len(longTerm),
		"promoted_tasks":     promotedIDs,
		"retained_long_term": retainedIDs,
		"adjusted_long_term": adjustedIDs,
		"dry_run":            dryRun,
		"config_applied": map[string]interface{}{
			"short_term_min":              cfg.LongTerm.ShortTermMin,
			"short_term_max":              cfg.LongTerm.ShortTermMax,
			"promote_count_when_shortage": cfg.LongTerm.PromoteCountWhenShortage,
			"retain_count_when_overflow":  cfg.LongTerm.RetainCountWhenOverflow,
			"overflow_strategy":           cfg.LongTerm.OverflowStrategy,
		},
	}

	jsonResult, err := toJSON(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: jsonResult}}}, nil
}

func (s *Server) handleDetectDecompositionCandidates(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.taskStore == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: `{"summary":{"total_scanned":0,"candidate_count":0},"candidates":[]}`}},
		}, nil
	}

	cfg := s.effectiveIntelligenceConfig()

	var rawArgs map[string]json.RawMessage
	if req != nil && req.Params != nil && len(req.Params.Arguments) > 0 {
		if err := json.Unmarshal(req.Params.Arguments, &rawArgs); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	limit, ok := getInt(rawArgs, "limit")
	if !ok || limit <= 0 {
		limit = 20
	}

	query := storage.Query{Statuses: []model.TaskStatus{model.StatusTodo, model.StatusInProgress}}
	if source := getString(rawArgs, "source"); source != "" {
		resolvedSource, err := resolveProviderNameStrict(source)
		if err != nil {
			return nil, err
		}
		query.Sources = []model.TaskSource{model.TaskSource(resolvedSource)}
	}

	tasks, err := s.taskStore.QueryTasks(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}

	type candidateItem struct {
		TaskID                   string   `json:"task_id"`
		Title                    string   `json:"title"`
		ComplexityScore          int      `json:"complexity_score"`
		ReasonCodes              []string `json:"reason_codes"`
		HasSubtasks              bool     `json:"has_subtasks"`
		RecommendedProvider      string   `json:"recommended_provider"`
		RecommendedStrategy      string   `json:"recommended_strategy"`
		ProviderSupportsSubtasks bool     `json:"provider_supports_subtasks"`
	}

	candidates := make([]candidateItem, 0)
	for _, task := range tasks {
		hasSubtasks := len(task.SubtaskIDs) > 0
		score, reasons := computeTaskComplexity(task, cfg.Decompose)
		if score < cfg.Decompose.ComplexityThreshold || hasSubtasks {
			continue
		}
		recommendedProvider, supportsSubtasks := s.recommendProviderForTask(task, "")
		strategy := strings.TrimSpace(cfg.Decompose.PreferredStrategy)
		if strategy == "" {
			strategy = "project_split"
		}
		candidates = append(candidates, candidateItem{
			TaskID:                   task.ID,
			Title:                    task.Title,
			ComplexityScore:          score,
			ReasonCodes:              reasons,
			HasSubtasks:              hasSubtasks,
			RecommendedProvider:      recommendedProvider,
			RecommendedStrategy:      strategy,
			ProviderSupportsSubtasks: supportsSubtasks,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].ComplexityScore == candidates[j].ComplexityScore {
			return candidates[i].TaskID < candidates[j].TaskID
		}
		return candidates[i].ComplexityScore > candidates[j].ComplexityScore
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	result := map[string]interface{}{
		"summary": map[string]interface{}{
			"total_scanned":   len(tasks),
			"candidate_count": len(candidates),
			"threshold":       cfg.Decompose.ComplexityThreshold,
			"limit":           limit,
		},
		"candidates": candidates,
	}

	jsonResult, err := toJSON(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: jsonResult}}}, nil
}

func (s *Server) handleDecomposeTaskWithProvider(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.taskStore == nil {
		return nil, fmt.Errorf("task storage not available")
	}

	var params struct {
		TaskID     string `json:"task_id"`
		Provider   string `json:"provider"`
		Strategy   string `json:"strategy"`
		WriteTasks bool   `json:"write_tasks"`
	}
	if req == nil || req.Params == nil || len(req.Params.Arguments) == 0 {
		return nil, fmt.Errorf("task_id is required")
	}
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}
	params.TaskID = strings.TrimSpace(params.TaskID)
	if params.TaskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	task, err := s.taskStore.GetTask(ctx, params.TaskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	cfg := s.effectiveIntelligenceConfig()
	providerName, supportsSubtasks := s.recommendProviderForTask(*task, params.Provider)
	strategy := strings.TrimSpace(params.Strategy)
	if strategy == "" {
		strategy = strings.TrimSpace(cfg.Decompose.PreferredStrategy)
	}
	if strategy == "" {
		strategy = "project_split"
	}

	preview := buildDecomposePreview(*task, supportsSubtasks)
	createdIDs := make([]string, 0)
	planID := fmt.Sprintf("decomp_%d", time.Now().UnixNano())

	if params.WriteTasks {
		createdIDs, err = s.writeDecomposePreviewTasks(ctx, *task, providerName, strategy, planID, preview)
		if err != nil {
			return nil, err
		}
	}

	warnings := make([]string, 0)
	if !supportsSubtasks {
		warnings = append(warnings, "目标 provider 不支持子任务，已建议使用扁平任务与阶段标签")
	}

	result := map[string]interface{}{
		"task_id":                  task.ID,
		"plan_id":                  planID,
		"provider":                 providerName,
		"strategy":                 strategy,
		"provider_capability_used": map[string]interface{}{"supports_subtasks": supportsSubtasks},
		"tasks_preview":            preview,
		"created_task_ids":         createdIDs,
		"write_tasks":              params.WriteTasks,
		"warnings":                 warnings,
	}

	jsonResult, err := toJSON(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: jsonResult}}}, nil
}

func (s *Server) handleAnalyzeAchievement(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.taskStore == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: `{"metrics":{"completed_count":0,"active_count":0},"badges":[],"narrative":"暂无数据"}`}},
		}, nil
	}

	cfg := s.effectiveIntelligenceConfig()

	windowDays := 30
	comparePrevious := cfg.Achievement.ComparePreviousPeriod
	var rawArgs map[string]json.RawMessage
	if req != nil && req.Params != nil && len(req.Params.Arguments) > 0 {
		if err := json.Unmarshal(req.Params.Arguments, &rawArgs); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
		if v, ok := getInt(rawArgs, "window_days"); ok && v > 0 {
			windowDays = v
		}
		if v, ok := getBool(rawArgs, "compare_previous"); ok {
			comparePrevious = v
		}
	}
	if windowDays < 7 {
		windowDays = 7
	}

	tasks, err := s.taskStore.ListTasks(ctx, storage.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	loc := resolveLocation(cfg.Timezone)
	now := time.Now().In(loc)
	start := now.AddDate(0, 0, -windowDays)
	prevStart := start.AddDate(0, 0, -windowDays)

	completedCount := 0
	activeCount := 0
	onTimeCount := 0
	overdueFixedCount := 0
	completedByQuadrant := map[string]int{"q1": 0, "q2": 0, "q3": 0, "q4": 0}
	previousCompleted := 0
	daily := make(map[string]int)

	for _, task := range tasks {
		if task.Status != model.StatusCompleted {
			activeCount++
		}
		completedAt := completionTime(task)
		if completedAt == nil {
			continue
		}
		completedTime := completedAt.In(loc)

		if !completedTime.Before(start) {
			completedCount++
			dayKey := completedTime.Format("2006-01-02")
			daily[dayKey]++
			quadrantKey := fmt.Sprintf("q%d", int(task.Quadrant))
			if _, ok := completedByQuadrant[quadrantKey]; !ok {
				quadrantKey = "q4"
			}
			completedByQuadrant[quadrantKey]++
			if task.DueDate != nil {
				if !completedTime.After(task.DueDate.In(loc)) {
					onTimeCount++
				} else {
					overdueFixedCount++
				}
			}
		}

		if comparePrevious && !completedTime.Before(prevStart) && completedTime.Before(start) {
			previousCompleted++
		}
	}

	streakGoal := cfg.Achievement.StreakGoalPerDay
	if streakGoal <= 0 {
		streakGoal = 1
	}
	streak := calcCompletionStreak(daily, now, streakGoal)

	onTimeRate := 0.0
	if completedCount > 0 {
		onTimeRate = float64(onTimeCount) / float64(completedCount)
	}
	avgPerDay := float64(completedCount) / float64(windowDays)

	delta := completedCount - previousCompleted
	trendText := "持平"
	if delta > 0 {
		trendText = fmt.Sprintf("上升 %+d", delta)
	} else if delta < 0 {
		trendText = fmt.Sprintf("下降 %d", delta)
	}

	badges := make([]string, 0)
	if cfg.Achievement.BadgeEnabled {
		if streak >= 7 {
			badges = append(badges, "steady-7")
		}
		if overdueFixedCount >= 5 {
			badges = append(badges, "overdue-cleaner")
		}
		if completedCount > 0 {
			q2Ratio := float64(completedByQuadrant["q2"]) / float64(completedCount)
			if q2Ratio >= 0.4 {
				badges = append(badges, "q2-builder")
			}
		}
	}

	narrative := ""
	if cfg.Achievement.NarrativeEnabled {
		narrative = fmt.Sprintf("过去 %d 天你完成了 %d 项任务，按时完成率 %.1f%%，当前连续完成 %d 天。趋势：%s。", windowDays, completedCount, onTimeRate*100, streak, trendText)
	}

	nextActions := make([]string, 0)
	if onTimeRate < 0.6 {
		nextActions = append(nextActions, "按时完成率偏低：建议先清理逾期项，并减少本周承诺任务数")
	}
	if streak < 3 {
		nextActions = append(nextActions, "连续完成天数较低：建议设置“每天至少完成 1 个任务”的目标")
	}
	if completedCount > 0 && completedByQuadrant["q2"]*3 < completedCount {
		nextActions = append(nextActions, "Q2（重要不紧急）占比偏低：建议每周固定投入 2~3 个 Q2 任务")
	}
	if len(nextActions) == 0 {
		nextActions = append(nextActions, "保持当前节奏，逐步提升任务拆分质量与计划稳定性")
	}

	result := map[string]interface{}{
		"metrics": map[string]interface{}{
			"window_days":           windowDays,
			"completed_count":       completedCount,
			"active_count":          activeCount,
			"on_time_rate":          onTimeRate,
			"streak_days":           streak,
			"avg_completed_per_day": avgPerDay,
			"overdue_fixed_count":   overdueFixedCount,
			"quadrant_completed":    completedByQuadrant,
			"compare_previous":      comparePrevious,
			"previous_completed":    previousCompleted,
			"delta_completed":       delta,
			"trend":                 trendText,
		},
		"badges":       badges,
		"narrative":    narrative,
		"next_actions": nextActions,
	}

	jsonResult, err := toJSON(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: jsonResult}}}, nil
}

func (s *Server) writeDecomposePreviewTasks(
	ctx context.Context,
	parentTask model.Task,
	providerName, strategy, planID string,
	preview []map[string]interface{},
) ([]string, error) {
	now := time.Now()
	createdIDs := make([]string, 0, len(preview))
	previewToLocal := make(map[string]string, len(preview))

	for idx, item := range preview {
		title := strings.TrimSpace(fmt.Sprint(item["title"]))
		if title == "" {
			title = fmt.Sprintf("拆分任务 %d", idx+1)
		}
		desc := strings.TrimSpace(fmt.Sprint(item["description"]))
		phase := strings.TrimSpace(fmt.Sprint(item["phase"]))
		previewID := strings.TrimSpace(fmt.Sprint(item["id"]))
		parentPreviewID := strings.TrimSpace(fmt.Sprint(item["parent_preview_id"]))
		dueOffset := anyToInt(item["due_offset_days"])
		if dueOffset <= 0 {
			dueOffset = idx + 1
		}
		priority := clampPriority(anyToInt(item["priority"]))
		quadrant := clampQuadrant(anyToInt(item["quadrant"]))

		task := &model.Task{
			ID:          generateID(),
			Title:       title,
			Description: desc,
			Status:      model.StatusTodo,
			CreatedAt:   now,
			UpdatedAt:   now,
			Source:      model.SourceLocal,
			Priority:    priority,
			Quadrant:    quadrant,
		}
		due := now.AddDate(0, 0, dueOffset)
		task.DueDate = &due

		if parentPreviewID != "" {
			if localParentID := strings.TrimSpace(previewToLocal[parentPreviewID]); localParentID != "" {
				parentID := localParentID
				task.ParentID = &parentID
			}
		}

		task.Metadata = &model.TaskMetadata{
			Version:    "1.0",
			Quadrant:   int(task.Quadrant),
			Priority:   int(task.Priority),
			LocalID:    task.ID,
			SyncSource: "local",
			CustomFields: map[string]interface{}{
				"tb_parent_task_id":     parentTask.ID,
				"tb_decompose_plan_id":  planID,
				"tb_decompose_provider": providerName,
				"tb_decompose_strategy": strategy,
				"tb_phase":              phase,
				"tb_preview_id":         previewID,
			},
		}

		if err := s.taskStore.SaveTask(ctx, task); err != nil {
			return nil, fmt.Errorf("failed to save decomposed task: %w", err)
		}
		createdIDs = append(createdIDs, task.ID)
		if previewID != "" {
			previewToLocal[previewID] = task.ID
		}
	}

	return createdIDs, nil
}

func buildDecomposePreview(task model.Task, supportsSubtasks bool) []map[string]interface{} {
	baseTitle := strings.TrimSpace(task.Title)
	if baseTitle == "" {
		baseTitle = "任务"
	}

	steps := []struct {
		Title    string
		Phase    string
		Offset   int
		Priority int
		Quadrant int
	}{
		{Title: "明确范围：" + baseTitle, Phase: "规划", Offset: 1, Priority: maxInt(2, int(task.Priority)), Quadrant: 2},
		{Title: "执行核心工作：" + baseTitle, Phase: "执行", Offset: 2, Priority: maxInt(2, int(task.Priority)), Quadrant: int(task.Quadrant)},
		{Title: "验证与修正：" + baseTitle, Phase: "验证", Offset: 3, Priority: 2, Quadrant: 2},
		{Title: "总结与归档：" + baseTitle, Phase: "收尾", Offset: 4, Priority: 1, Quadrant: 2},
	}

	result := make([]map[string]interface{}, 0, len(steps))
	for i, step := range steps {
		item := map[string]interface{}{
			"id":              fmt.Sprintf("preview_%d", i+1),
			"title":           step.Title,
			"description":     fmt.Sprintf("由任务“%s”拆分而来", baseTitle),
			"phase":           step.Phase,
			"due_offset_days": step.Offset,
			"priority":        clampPriority(step.Priority),
			"quadrant":        clampQuadrant(step.Quadrant),
		}
		if supportsSubtasks && i > 0 {
			item["parent_preview_id"] = "preview_1"
		}
		result = append(result, item)
	}
	return result
}

func computeTaskComplexity(task model.Task, cfg pkgconfig.DecomposeConfig) (int, []string) {
	score := 0
	reasons := make([]string, 0)

	title := strings.TrimSpace(task.Title)
	description := strings.TrimSpace(task.Description)
	fullText := strings.ToLower(title + " " + description)

	if len([]rune(title)) >= 20 {
		score += 10
		reasons = append(reasons, "long_title")
	}
	if len([]rune(title)) >= 36 {
		score += 10
		reasons = append(reasons, "very_long_title")
	}
	if len([]rune(description)) >= 80 {
		score += 10
		reasons = append(reasons, "large_description")
	}

	if cfg.DetectAbstractKeywords {
		matchedKeywords := make([]string, 0)
		for _, keyword := range cfg.AbstractKeywords {
			k := strings.ToLower(strings.TrimSpace(keyword))
			if k == "" {
				continue
			}
			if strings.Contains(fullText, k) {
				matchedKeywords = append(matchedKeywords, keyword)
			}
		}
		if len(matchedKeywords) > 0 {
			score += minInt(30, 10+len(matchedKeywords)*5)
			reasons = append(reasons, "abstract_keyword")
		}
	}

	if task.DueDate == nil {
		score += 10
		reasons = append(reasons, "no_due_date")
	}
	if task.EstimatedMinutes <= 0 {
		score += 5
		reasons = append(reasons, "no_estimate")
	}
	if task.ParentID == nil && len(task.SubtaskIDs) == 0 {
		score += 15
		reasons = append(reasons, "no_subtasks")
	}

	if overdueCount := taskCustomInt(task, "tb_overdue_count"); overdueCount > 0 {
		score += minInt(15, overdueCount*3)
		reasons = append(reasons, "historical_overdue")
	}

	if score > 100 {
		score = 100
	}
	return score, reasons
}

func (s *Server) recommendProviderForTask(task model.Task, preferred string) (string, bool) {
	preferred = strings.TrimSpace(preferred)
	if preferred != "" {
		resolved := provider.ResolveProviderName(preferred)
		if p, ok := s.providers[resolved]; ok {
			return resolved, p.Capabilities().SupportsSubtasks
		}
		if provider.IsValidProvider(resolved) {
			return resolved, false
		}
	}

	taskSource := strings.TrimSpace(string(task.Source))
	if provider.IsValidProvider(taskSource) {
		if p, ok := s.providers[taskSource]; ok {
			return taskSource, p.Capabilities().SupportsSubtasks
		}
		return taskSource, false
	}

	names := make([]string, 0, len(s.providers))
	for name := range s.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if s.providers[name].Capabilities().SupportsSubtasks {
			return name, true
		}
	}
	if len(names) > 0 {
		return names[0], s.providers[names[0]].Capabilities().SupportsSubtasks
	}

	return "local", false
}

func (s *Server) effectiveIntelligenceConfig() pkgconfig.IntelligenceConfig {
	defaults := pkgconfig.DefaultConfig().MCP.Intelligence
	if s.intelligenceConfig == nil {
		return defaults
	}

	out := *s.intelligenceConfig
	if strings.TrimSpace(out.Timezone) == "" {
		out.Timezone = defaults.Timezone
	}

	if out.Overdue.WarningThreshold == 0 {
		out.Overdue.WarningThreshold = defaults.Overdue.WarningThreshold
	}
	if out.Overdue.OverloadThreshold == 0 {
		out.Overdue.OverloadThreshold = defaults.Overdue.OverloadThreshold
	}
	if out.Overdue.SevereDays == 0 {
		out.Overdue.SevereDays = defaults.Overdue.SevereDays
	}
	if out.Overdue.MaxCandidates == 0 {
		out.Overdue.MaxCandidates = defaults.Overdue.MaxCandidates
	}

	if out.LongTerm.MinAgeDays == 0 {
		out.LongTerm.MinAgeDays = defaults.LongTerm.MinAgeDays
	}
	if out.LongTerm.ShortTermWindowDays == 0 {
		out.LongTerm.ShortTermWindowDays = defaults.LongTerm.ShortTermWindowDays
	}
	if out.LongTerm.ShortTermMin == 0 {
		out.LongTerm.ShortTermMin = defaults.LongTerm.ShortTermMin
	}
	if out.LongTerm.ShortTermMax == 0 {
		out.LongTerm.ShortTermMax = defaults.LongTerm.ShortTermMax
	}
	if out.LongTerm.PromoteCountWhenShortage == 0 {
		out.LongTerm.PromoteCountWhenShortage = defaults.LongTerm.PromoteCountWhenShortage
	}
	if out.LongTerm.RetainCountWhenOverflow == 0 {
		out.LongTerm.RetainCountWhenOverflow = defaults.LongTerm.RetainCountWhenOverflow
	}
	if strings.TrimSpace(out.LongTerm.OverflowStrategy) == "" {
		out.LongTerm.OverflowStrategy = defaults.LongTerm.OverflowStrategy
	}

	if out.Decompose.ComplexityThreshold == 0 {
		out.Decompose.ComplexityThreshold = defaults.Decompose.ComplexityThreshold
	}
	if strings.TrimSpace(out.Decompose.PreferredStrategy) == "" {
		out.Decompose.PreferredStrategy = defaults.Decompose.PreferredStrategy
	}
	if len(out.Decompose.AbstractKeywords) == 0 {
		out.Decompose.AbstractKeywords = append([]string(nil), defaults.Decompose.AbstractKeywords...)
	}

	if strings.TrimSpace(out.Achievement.SnapshotGranularity) == "" {
		out.Achievement.SnapshotGranularity = defaults.Achievement.SnapshotGranularity
	}
	if out.Achievement.StreakGoalPerDay == 0 {
		out.Achievement.StreakGoalPerDay = defaults.Achievement.StreakGoalPerDay
	}

	return out
}

func buildOverdueQuestions(overdueCount int, overload bool) []string {
	questions := []string{}
	if overdueCount == 0 {
		questions = append(questions, "当前无逾期任务，是否要提前规划下周重点任务？")
		return questions
	}
	questions = append(questions,
		"这些逾期任务是否都属于延期任务？",
		"请确认需要延期的任务 ID 列表（可批量）。",
	)
	if overload {
		questions = append(questions,
			"当前逾期数量较多，是否删除低价值且长期逾期的任务？",
			"是否将复杂逾期任务先拆分为子任务再重排日期？",
		)
	}
	return questions
}

func calcOverdueDays(due *time.Time, now time.Time) int {
	if due == nil || !due.Before(now) {
		return 0
	}
	days := int(now.Sub(*due).Hours() / 24)
	if days <= 0 {
		return 1
	}
	return days
}

func calcAgeDays(createdAt, now time.Time) int {
	if createdAt.IsZero() {
		return 0
	}
	days := int(now.Sub(createdAt).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

func scoreLongTermTask(task model.Task, now time.Time) int {
	score := int(task.Priority) * 100
	switch task.Quadrant {
	case model.QuadrantUrgentImportant:
		score += 40
	case model.QuadrantNotUrgentImportant:
		score += 30
	case model.QuadrantUrgentNotImportant:
		score += 20
	case model.QuadrantNotUrgentNotImportant:
		score += 10
	}
	updatedGapDays := int(now.Sub(task.UpdatedAt).Hours() / 24)
	if updatedGapDays < 0 {
		updatedGapDays = 0
	}
	freshnessBonus := 30 - updatedGapDays
	if freshnessBonus < 0 {
		freshnessBonus = 0
	}
	score += freshnessBonus
	if looksActionable(task.Title) {
		score += 5
	}
	return score
}

func looksActionable(title string) bool {
	t := strings.ToLower(strings.TrimSpace(title))
	if t == "" {
		return false
	}
	verbs := []string{"完成", "编写", "修复", "整理", "实现", "学习", "review", "write", "fix", "plan", "build"}
	for _, verb := range verbs {
		if strings.HasPrefix(t, strings.ToLower(verb)) {
			return true
		}
	}
	return false
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func taskCustomInt(task model.Task, key string) int {
	if task.Metadata == nil || task.Metadata.CustomFields == nil {
		return 0
	}
	raw, ok := task.Metadata.CustomFields[key]
	if !ok {
		return 0
	}
	return anyToInt(raw)
}

func anyToInt(raw interface{}) int {
	switch v := raw.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func resolveLocation(timezone string) *time.Location {
	tz := strings.TrimSpace(timezone)
	if tz == "" {
		return time.Local
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.Local
	}
	return loc
}

func completionTime(task model.Task) *time.Time {
	if task.CompletedAt != nil {
		return task.CompletedAt
	}
	if task.Status == model.StatusCompleted {
		t := task.UpdatedAt
		if t.IsZero() {
			t = task.CreatedAt
		}
		return &t
	}
	return nil
}

func calcCompletionStreak(daily map[string]int, now time.Time, goalPerDay int) int {
	if goalPerDay <= 0 {
		goalPerDay = 1
	}
	streak := 0
	for i := 0; i < 3650; i++ {
		day := now.AddDate(0, 0, -i).Format("2006-01-02")
		if daily[day] >= goalPerDay {
			streak++
			continue
		}
		break
	}
	return streak
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
