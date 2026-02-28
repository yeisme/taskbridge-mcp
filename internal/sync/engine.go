// Package sync 提供任务同步功能
package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/internal/storage"
)

// Direction 同步方向
type Direction string

const (
	// DirectionPull 从远程拉取到本地
	DirectionPull Direction = "pull"
	// DirectionPush 从本地推送到远程
	DirectionPush Direction = "push"
	// DirectionBidirectional 双向同步
	DirectionBidirectional Direction = "bidirectional"
)

// Result 同步结果
type Result struct {
	// Provider Provider 名称
	Provider string `json:"provider"`
	// Direction 同步方向
	Direction Direction `json:"direction"`
	// Pulled 拉取的任务数
	Pulled int `json:"pulled"`
	// Pushed 推送的任务数
	Pushed int `json:"pushed"`
	// Updated 更新的任务数
	Updated int `json:"updated"`
	// Deleted 删除的任务数
	Deleted int `json:"deleted"`
	// Skipped 跳过的任务数
	Skipped int `json:"skipped"`
	// Errors 错误列表
	Errors []Error `json:"errors,omitempty"`
	// Duration 同步耗时
	Duration time.Duration `json:"duration"`
	// LastSyncTime 最后同步时间
	LastSyncTime time.Time `json:"last_sync_time"`
}

// Error 同步错误
type Error struct {
	// TaskID 任务 ID
	TaskID string `json:"task_id,omitempty"`
	// Operation 操作类型
	Operation string `json:"operation"`
	// Error 错误信息
	Error string `json:"error"`
}

// Options 同步选项
type Options struct {
	// Direction 同步方向
	Direction Direction
	// Provider 指定的 Provider 名称
	Provider string
	// DryRun 是否为模拟运行（不实际修改）
	DryRun bool
	// Force 是否强制同步（忽略冲突）
	Force bool
	// ConflictResolve 冲突解决策略："local", "remote", "newer"
	ConflictResolve string
	// Since 增量同步起始时间
	Since time.Time
	// DeleteRemote 是否删除远程存在但本地不存在的任务
	DeleteRemote bool
}

// Engine 同步引擎
type Engine struct {
	// providers Provider 映射表
	providers map[string]provider.Provider
	// storage 存储接口
	storage storage.Storage
}

// NewEngine 创建同步引擎
func NewEngine(providers map[string]provider.Provider, store storage.Storage) *Engine {
	return &Engine{
		providers: providers,
		storage:   store,
	}
}

// Sync 执行同步
func (e *Engine) Sync(ctx context.Context, opts Options) (*Result, error) {
	startTime := time.Now()
	result := &Result{
		Direction:    opts.Direction,
		LastSyncTime: startTime,
	}

	// 获取指定的 Provider
	p, ok := e.providers[opts.Provider]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", opts.Provider)
	}
	result.Provider = opts.Provider

	// 检查 Provider 是否已认证
	if !p.IsAuthenticated() {
		return nil, fmt.Errorf("provider %s is not authenticated", opts.Provider)
	}

	switch opts.Direction {
	case DirectionPull:
		err := e.pull(ctx, p, result, opts)
		if err != nil {
			return result, err
		}
	case DirectionPush:
		err := e.push(ctx, p, result, opts)
		if err != nil {
			return result, err
		}
	case DirectionBidirectional:
		err := e.bidirectional(ctx, p, result, opts)
		if err != nil {
			return result, err
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// pull 从远程拉取任务
func (e *Engine) pull(ctx context.Context, p provider.Provider, result *Result, opts Options) error {
	log.Info().Str("provider", p.Name()).Msg("开始拉取任务")

	// 从远程获取所有任务列表
	taskLists, err := p.ListTaskLists(ctx)
	if err != nil {
		return fmt.Errorf("获取任务列表失败: %w", err)
	}

	// 保存任务列表到本地
	for _, list := range taskLists {
		if opts.DryRun {
			log.Info().Str("list", list.Name).Msg("[DryRun] 将拉取任务列表")
			continue
		}
		_ = e.storage.SaveTaskList(ctx, &list)
	}

	// 获取本地该 Provider 的所有任务，用于匹配 local_id 和清理幽灵任务
	source := model.TaskSource(opts.Provider)
	localTasks, err := e.storage.ListTasks(ctx, storage.ListOptions{Source: source})
	if err != nil {
		log.Warn().Err(err).Msg("获取本地任务失败，将直接保存远程任务")
		localTasks = []model.Task{}
	}

	// 创建 local_id 到本地任务的映射
	localTaskMap := make(map[string]*model.Task)
	localByID := make(map[string]*model.Task)
	localBySourceRawID := make(map[string]*model.Task)
	for i := range localTasks {
		localByID[localTasks[i].ID] = &localTasks[i]
		if localTasks[i].SourceRawID != "" {
			localBySourceRawID[localTasks[i].SourceRawID] = &localTasks[i]
		}
		if localTasks[i].Metadata != nil && localTasks[i].Metadata.LocalID != "" {
			localTaskMap[localTasks[i].Metadata.LocalID] = &localTasks[i]
		}
	}

	// 收集远程任务的所有 ID，用于检测幽灵任务
	remoteTaskIDs := make(map[string]bool)

	// 从每个列表拉取任务
	for _, list := range taskLists {
		// 从远程获取任务
		tasks, err := p.ListTasks(ctx, list.ID, provider.ListOptions{})
		if err != nil {
			result.Errors = append(result.Errors, Error{
				Operation: "list_tasks",
				Error:     fmt.Sprintf("获取列表 %s 的任务失败: %v", list.Name, err),
			})
			continue
		}

		for _, task := range tasks {
			// 兜底写入任务所属列表信息，保证 list 输出可见来源列表。
			if task.ListID == "" {
				task.ListID = list.ID
			}
			if task.ListName == "" {
				task.ListName = list.Name
			}

			// 记录远程任务 ID（使用 SourceRawID 或 ID）
			taskID := task.SourceRawID
			if taskID == "" {
				taskID = task.ID
				task.SourceRawID = task.ID
			}
			remoteTaskIDs[taskID] = true

			if opts.DryRun {
				log.Info().Str("task", task.Title).Msg("[DryRun] 将拉取任务")
				result.Pulled++
				continue
			}

			existingTask := e.findExistingLocalTask(&task, localTaskMap, localByID, localBySourceRawID)
			if existingTask != nil {
				// 保留本地主键，避免重复写入新 ID。
				task.ID = existingTask.ID
			}

			// 检查是否有匹配的本地任务（通过 metadata.local_id）
			if task.Metadata != nil && task.Metadata.LocalID != "" {
				if localTask, ok := localTaskMap[task.Metadata.LocalID]; ok {
					// 找到匹配的本地任务，更新它
					log.Info().Str("local_id", task.Metadata.LocalID).Msg("找到匹配的本地任务，进行合并")
					// 保留原始本地 ID
					task.ID = localTask.ID
					// 合并元数据
					if task.Metadata == nil {
						task.Metadata = localTask.Metadata
					}
				}
			}
			if existingTask != nil && e.sameTaskContent(existingTask, &task) && !opts.Force {
				result.Skipped++
				continue
			}

			// 保存到本地存储
			err := e.storage.SaveTask(ctx, &task)
			if err != nil {
				result.Errors = append(result.Errors, Error{
					TaskID:    task.ID,
					Operation: "save_task",
					Error:     err.Error(),
				})
				continue
			}
			result.Pulled++
		}
	}

	// 清理幽灵任务：删除本地存在但远程不存在的任务
	if !opts.DryRun {
		ghostCount := 0
		for _, localTask := range localTasks {
			// 获取本地任务的远程 ID
			taskID := localTask.SourceRawID
			if taskID == "" {
				// 如果没有 SourceRawID，尝试从 ID 中提取（ID格式为 "provider-listID-rawID"）
				taskID = localTask.ID
			}
			// 检查该任务是否在远程存在
			if !remoteTaskIDs[taskID] {
				log.Info().Str("task", localTask.Title).Str("id", localTask.ID).Msg("检测到幽灵任务，正在删除")
				if err := e.storage.DeleteTask(ctx, localTask.ID); err != nil {
					result.Errors = append(result.Errors, Error{
						TaskID:    localTask.ID,
						Operation: "delete_ghost_task",
						Error:     fmt.Sprintf("删除幽灵任务失败: %v", err),
					})
					continue
				}
				ghostCount++
			}
		}
		if ghostCount > 0 {
			log.Info().Int("ghost_count", ghostCount).Msg("已清理幽灵任务")
		}
	}

	// 更新同步时间
	_ = e.storage.SetLastSyncTime(ctx, source, time.Now())

	log.Info().Int("pulled", result.Pulled).Msg("拉取完成")
	return nil
}

func (e *Engine) findExistingLocalTask(
	remoteTask *model.Task,
	localByMeta map[string]*model.Task,
	localByID map[string]*model.Task,
	localBySourceRawID map[string]*model.Task,
) *model.Task {
	if remoteTask == nil {
		return nil
	}
	if remoteTask.Metadata != nil && remoteTask.Metadata.LocalID != "" {
		if localTask, ok := localByMeta[remoteTask.Metadata.LocalID]; ok {
			return localTask
		}
	}
	if localTask, ok := localByID[remoteTask.ID]; ok {
		return localTask
	}
	if remoteTask.SourceRawID != "" {
		if localTask, ok := localBySourceRawID[remoteTask.SourceRawID]; ok {
			return localTask
		}
	}
	return nil
}

func (e *Engine) sameTaskContent(localTask, remoteTask *model.Task) bool {
	if localTask == nil || remoteTask == nil {
		return false
	}
	if localTask.Title != remoteTask.Title ||
		localTask.Description != remoteTask.Description ||
		localTask.Status != remoteTask.Status ||
		localTask.ListID != remoteTask.ListID ||
		localTask.ListName != remoteTask.ListName ||
		localTask.Source != remoteTask.Source ||
		localTask.SourceRawID != remoteTask.SourceRawID ||
		localTask.Priority != remoteTask.Priority ||
		localTask.Quadrant != remoteTask.Quadrant ||
		!sameStringPtr(localTask.ParentID, remoteTask.ParentID) ||
		!sameMetadataCustomFields(localTask.Metadata, remoteTask.Metadata) {
		return false
	}
	if !sameTimePtr(localTask.DueDate, remoteTask.DueDate) ||
		!sameTimePtr(localTask.CompletedAt, remoteTask.CompletedAt) ||
		!sameTimePtr(localTask.Reminder, remoteTask.Reminder) ||
		!sameStringSlice(localTask.Tags, remoteTask.Tags) {
		return false
	}
	return true
}

func sameTimePtr(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
	return true
}

func sameStringPtr(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func sameMetadataCustomFields(a, b *model.TaskMetadata) bool {
	var aFields map[string]any
	var bFields map[string]any
	if a != nil {
		aFields = a.CustomFields
	}
	if b != nil {
		bFields = b.CustomFields
	}
	if len(aFields) == 0 && len(bFields) == 0 {
		return true
	}
	aJSON, err := json.Marshal(aFields)
	if err != nil {
		return false
	}
	bJSON, err := json.Marshal(bFields)
	if err != nil {
		return false
	}
	return string(aJSON) == string(bJSON)
}

// push 推送任务到远程
func (e *Engine) push(ctx context.Context, p provider.Provider, result *Result, opts Options) error {
	log.Info().Str("provider", p.Name()).Msg("开始推送任务")

	// 从本地存储获取所有任务（包括本地创建的任务）
	localTasks, err := e.storage.ListTasks(ctx, storage.ListOptions{})
	if err != nil {
		return fmt.Errorf("获取本地任务失败: %w", err)
	}

	// 获取远程任务列表，找到默认列表
	taskLists, err := p.ListTaskLists(ctx)
	if err != nil {
		return fmt.Errorf("获取远程任务列表失败: %w", err)
	}

	// 查找默认列表
	defaultListID := e.findDefaultListID(taskLists)
	if defaultListID == "" {
		return fmt.Errorf("未找到可用的任务列表")
	}

	log.Info().Str("default_list", defaultListID).Msg("使用默认任务列表")

	// 创建本地任务的 SourceRawID 集合，用于后续比对
	localSourceRawIDs := e.buildLocalSourceRawIDs(localTasks)

	// 推送本地任务
	source := model.TaskSource(opts.Provider)
	e.pushLocalTasks(ctx, p, localTasks, defaultListID, source, opts, result)

	// 双向比对：删除远程存在但本地不存在的任务
	if opts.DeleteRemote {
		e.deleteRemoteTasks(ctx, p, taskLists, localSourceRawIDs, opts.DryRun, result)
	}

	log.Info().Int("pushed", result.Pushed).Int("updated", result.Updated).Int("deleted", result.Deleted).Msg("推送完成")
	return nil
}

// findDefaultListID 查找默认任务列表ID
func (e *Engine) findDefaultListID(taskLists []model.TaskList) string {
	for _, list := range taskLists {
		// Google Tasks 的默认列表名称通常是 "我的任务" 或 "My Tasks"
		if list.Name == "我的任务" || list.Name == "My Tasks" || list.ID == "@default" {
			return list.ID
		}
	}
	// 如果没找到默认列表，使用第一个列表
	if len(taskLists) > 0 {
		return taskLists[0].ID
	}
	return ""
}

// buildLocalSourceRawIDs 构建本地任务的 SourceRawID 集合
func (e *Engine) buildLocalSourceRawIDs(tasks []model.Task) map[string]bool {
	ids := make(map[string]bool)
	for _, task := range tasks {
		if task.SourceRawID != "" {
			ids[task.SourceRawID] = true
		}
	}
	return ids
}

// pushLocalTasks 推送本地任务到远程
func (e *Engine) pushLocalTasks(ctx context.Context, p provider.Provider, tasks []model.Task, defaultListID string, source model.TaskSource, opts Options, result *Result) {
	for _, task := range tasks {
		// 跳过已经从该 Provider 同步的任务
		if task.Source != "" && task.Source != source && task.Source != "local" {
			continue
		}

		// 如果任务已经有 SourceRawID，说明已经同步过
		if task.SourceRawID != "" {
			// 检查远程是否存在该任务
			existingTask, err := p.GetTask(ctx, task.ListID, task.SourceRawID)
			if err == nil && existingTask != nil {
				// 任务存在，检查是否需要更新
				if existingTask.UpdatedAt.Before(task.UpdatedAt) || opts.Force {
					_, err := p.UpdateTask(ctx, task.ListID, &task)
					if err != nil {
						result.Errors = append(result.Errors, Error{
							TaskID:    task.ID,
							Operation: "update_task",
							Error:     err.Error(),
						})
						continue
					}
					result.Updated++
				} else {
					result.Skipped++
				}
				continue
			}
		}

		if opts.DryRun {
			log.Info().Str("task", task.Title).Msg("[DryRun] 将推送任务")
			result.Pushed++
			continue
		}

		// 使用默认列表ID
		listID := task.ListID
		if listID == "" {
			listID = defaultListID
		}

		// 创建新任务
		createdTask, err := p.CreateTask(ctx, listID, &task)
		if err != nil {
			result.Errors = append(result.Errors, Error{
				TaskID:    task.ID,
				Operation: "create_task",
				Error:     err.Error(),
			})
			continue
		}
		// 更新本地任务的 SourceRawID 和 ListID
		task.SourceRawID = createdTask.SourceRawID
		task.ListID = listID
		task.Source = source
		_ = e.storage.SaveTask(ctx, &task)
		result.Pushed++
	}
}

// deleteRemoteTasks 删除远程存在但本地不存在的任务
func (e *Engine) deleteRemoteTasks(ctx context.Context, p provider.Provider, taskLists []model.TaskList, localSourceRawIDs map[string]bool, dryRun bool, result *Result) {
	log.Info().Msg("开始比对远程任务，查找需要删除的任务")
	for _, list := range taskLists {
		// 获取远程任务
		remoteTasks, err := p.ListTasks(ctx, list.ID, provider.ListOptions{})
		if err != nil {
			result.Errors = append(result.Errors, Error{
				Operation: "list_remote_tasks",
				Error:     fmt.Sprintf("获取远程列表 %s 的任务失败: %v", list.Name, err),
			})
			continue
		}

		for _, remoteTask := range remoteTasks {
			// 检查远程任务是否在本地存在
			if !localSourceRawIDs[remoteTask.SourceRawID] {
				// 远程任务在本地不存在，需要删除
				if dryRun {
					log.Info().Str("task", remoteTask.Title).Str("id", remoteTask.SourceRawID).Msg("[DryRun] 将删除远程任务")
					result.Deleted++
					continue
				}

				err := p.DeleteTask(ctx, list.ID, remoteTask.SourceRawID)
				if err != nil {
					result.Errors = append(result.Errors, Error{
						TaskID:    remoteTask.SourceRawID,
						Operation: "delete_remote_task",
						Error:     fmt.Sprintf("删除远程任务失败: %v", err),
					})
					continue
				}
				log.Info().Str("task", remoteTask.Title).Str("id", remoteTask.SourceRawID).Msg("已删除远程任务")
				result.Deleted++
			}
		}
	}
}

// bidirectional 双向同步
func (e *Engine) bidirectional(ctx context.Context, p provider.Provider, result *Result, opts Options) error {
	log.Info().Str("provider", p.Name()).Msg("开始双向同步")

	// 先拉取
	err := e.pull(ctx, p, result, opts)
	if err != nil {
		return fmt.Errorf("拉取失败: %w", err)
	}

	// 再推送
	err = e.push(ctx, p, result, opts)
	if err != nil {
		return fmt.Errorf("推送失败: %w", err)
	}

	log.Info().Msg("双向同步完成")
	return nil
}

// SyncAll 同步所有 Provider
func (e *Engine) SyncAll(ctx context.Context, opts Options) (map[string]*Result, error) {
	results := make(map[string]*Result)

	for name, p := range e.providers {
		if !p.IsAuthenticated() {
			log.Warn().Str("provider", name).Msg("Provider 未认证，跳过")
			continue
		}

		providerOpts := opts
		providerOpts.Provider = name

		result, err := e.Sync(ctx, providerOpts)
		if err != nil {
			log.Error().Err(err).Str("provider", name).Msg("同步失败")
			results[name] = &Result{
				Provider: name,
				Errors: []Error{
					{Operation: "sync", Error: err.Error()},
				},
			}
			continue
		}

		results[name] = result
	}

	return results, nil
}

// Watch 持续监听同步
func (e *Engine) Watch(ctx context.Context, opts Options, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Info().Dur("interval", interval).Msg("开始持续同步")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("停止持续同步")
			return ctx.Err()
		case <-ticker.C:
			result, err := e.Sync(ctx, opts)
			if err != nil {
				log.Error().Err(err).Msg("同步失败")
				continue
			}
			log.Info().
				Str("provider", result.Provider).
				Int("pulled", result.Pulled).
				Int("pushed", result.Pushed).
				Int("updated", result.Updated).
				Msg("同步完成")
		}
	}
}

// GetStatus 获取同步状态
func (e *Engine) GetStatus(ctx context.Context, providerName string) (*Status, error) {
	// 从存储获取最后同步时间
	source := model.TaskSource(providerName)
	lastSync, err := e.storage.GetLastSyncTime(ctx, source)
	if err != nil {
		return nil, err
	}

	p, ok := e.providers[providerName]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", providerName)
	}

	var lastSyncTime time.Time
	if lastSync != nil {
		lastSyncTime = *lastSync
	}

	localTasks, err := e.storage.ListTasks(ctx, storage.ListOptions{})
	if err != nil {
		return nil, err
	}

	pendingChanges := 0
	for _, task := range localTasks {
		if task.Source != "" && task.Source != source && task.Source != model.SourceLocal {
			continue
		}

		if task.SourceRawID == "" {
			pendingChanges++
			continue
		}

		if !lastSyncTime.IsZero() && task.UpdatedAt.After(lastSyncTime) {
			pendingChanges++
		}
	}

	return &Status{
		Provider:       providerName,
		Authenticated:  p.IsAuthenticated(),
		LastSyncTime:   lastSyncTime,
		PendingChanges: pendingChanges,
	}, nil
}

// Status 同步状态
type Status struct {
	Provider       string    `json:"provider"`
	Authenticated  bool      `json:"authenticated"`
	LastSyncTime   time.Time `json:"last_sync_time"`
	PendingChanges int       `json:"pending_changes"`
}
