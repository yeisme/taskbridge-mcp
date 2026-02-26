// Package filestore 提供基于文件的存储实现
package filestore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yeisme/taskbridge/internal/filter"
	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/storage"
)

// MultiProviderStorage 多 Provider 存储实现
type MultiProviderStorage struct {
	mu sync.RWMutex

	// basePath 存储基础路径
	basePath string
	// format 存储格式
	format string

	// 全局文件路径
	manifestFile  string
	syncStateFile string
	mappingsFile  string

	// 全局数据
	manifest  *model.Manifest
	syncState *model.SyncState
	mappings  *model.MappingDatabase

	// Provider 数据缓存
	providerData map[string]*ProviderStorage
}

// ProviderStorage 单个 Provider 的存储
type ProviderStorage struct {
	mu sync.RWMutex

	provider string
	basePath string

	tasksFile string
	listsFile string
	metaFile  string

	tasks     map[string]*model.Task
	taskLists map[string]*model.TaskList
	meta      *model.ProviderData
}

// NewMultiProviderStorage 创建多 Provider 存储实例
func NewMultiProviderStorage(basePath, format string) (*MultiProviderStorage, error) {
	mps := &MultiProviderStorage{
		basePath:      basePath,
		format:        format,
		manifestFile:  filepath.Join(basePath, "manifest.json"),
		syncStateFile: filepath.Join(basePath, "sync-state.json"),
		mappingsFile:  filepath.Join(basePath, "mappings.json"),
		providerData:  make(map[string]*ProviderStorage),
	}

	// 确保目录存在
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// 确保providers目录存在
	providersDir := filepath.Join(basePath, "providers")
	if err := os.MkdirAll(providersDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create providers directory: %w", err)
	}

	// 加载全局数据
	if err := mps.loadGlobalData(); err != nil {
		return nil, fmt.Errorf("failed to load global data: %w", err)
	}

	return mps, nil
}

// loadGlobalData 加载全局数据
func (mps *MultiProviderStorage) loadGlobalData() error {
	// 加载清单
	if data, err := os.ReadFile(mps.manifestFile); err == nil {
		var manifest model.Manifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return fmt.Errorf("failed to unmarshal manifest: %w", err)
		}
		mps.manifest = &manifest
	} else if os.IsNotExist(err) {
		mps.manifest = model.NewManifest()
	} else {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	// 加载同步状态
	if data, err := os.ReadFile(mps.syncStateFile); err == nil {
		var syncState model.SyncState
		if err := json.Unmarshal(data, &syncState); err != nil {
			return fmt.Errorf("failed to unmarshal sync state: %w", err)
		}
		mps.syncState = &syncState
	} else if os.IsNotExist(err) {
		mps.syncState = model.NewSyncState()
	} else {
		return fmt.Errorf("failed to read sync state: %w", err)
	}

	// 加载映射
	if data, err := os.ReadFile(mps.mappingsFile); err == nil {
		var mappings model.MappingDatabase
		if err := json.Unmarshal(data, &mappings); err != nil {
			return fmt.Errorf("failed to unmarshal mappings: %w", err)
		}
		mps.mappings = &mappings
	} else if os.IsNotExist(err) {
		mps.mappings = model.NewMappingDatabase()
	} else {
		return fmt.Errorf("failed to read mappings: %w", err)
	}

	return nil
}

// saveGlobalData 保存全局数据
func (mps *MultiProviderStorage) saveGlobalData() error {
	// 保存清单
	mps.manifest.UpdatedAt = time.Now()
	manifestData, err := json.MarshalIndent(mps.manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	if err := os.WriteFile(mps.manifestFile, manifestData, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// 保存同步状态
	syncStateData, err := json.MarshalIndent(mps.syncState, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sync state: %w", err)
	}
	if err := os.WriteFile(mps.syncStateFile, syncStateData, 0644); err != nil {
		return fmt.Errorf("failed to write sync state: %w", err)
	}

	// 保存映射
	mappingsData, err := json.MarshalIndent(mps.mappings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal mappings: %w", err)
	}
	if err := os.WriteFile(mps.mappingsFile, mappingsData, 0644); err != nil {
		return fmt.Errorf("failed to write mappings: %w", err)
	}

	return nil
}

// GetProviderStorage 获取指定 Provider 的存储
func (mps *MultiProviderStorage) GetProviderStorage(provider string) (*ProviderStorage, error) {
	mps.mu.Lock()
	defer mps.mu.Unlock()

	// 检查缓存
	if ps, ok := mps.providerData[provider]; ok {
		return ps, nil
	}

	// 创建新的 Provider 存储
	ps, err := NewProviderStorage(provider, filepath.Join(mps.basePath, "providers", provider))
	if err != nil {
		return nil, err
	}

	mps.providerData[provider] = ps
	return ps, nil
}

// GetManifest 获取清单
func (mps *MultiProviderStorage) GetManifest() *model.Manifest {
	mps.mu.RLock()
	defer mps.mu.RUnlock()
	return mps.manifest
}

// UpdateManifest 更新清单
func (mps *MultiProviderStorage) UpdateManifest(fn func(*model.Manifest)) error {
	mps.mu.Lock()
	defer mps.mu.Unlock()

	fn(mps.manifest)
	return mps.saveGlobalData()
}

// GetSyncState 获取同步状态
func (mps *MultiProviderStorage) GetSyncState() *model.SyncState {
	mps.mu.RLock()
	defer mps.mu.RUnlock()
	return mps.syncState
}

// AddSyncSession 添加同步会话
func (mps *MultiProviderStorage) AddSyncSession(session model.SyncSession) error {
	mps.mu.Lock()
	defer mps.mu.Unlock()

	mps.syncState.SyncSessions = append(mps.syncState.SyncSessions, session)
	// 只保留最近100个会话
	if len(mps.syncState.SyncSessions) > 100 {
		mps.syncState.SyncSessions = mps.syncState.SyncSessions[len(mps.syncState.SyncSessions)-100:]
	}
	return mps.saveGlobalData()
}

// AddPendingOperation 添加待处理操作
func (mps *MultiProviderStorage) AddPendingOperation(op model.PendingOp) error {
	mps.mu.Lock()
	defer mps.mu.Unlock()

	mps.syncState.PendingOperations = append(mps.syncState.PendingOperations, op)
	return mps.saveGlobalData()
}

// GetMappings 获取映射数据库
func (mps *MultiProviderStorage) GetMappings() *model.MappingDatabase {
	mps.mu.RLock()
	defer mps.mu.RUnlock()
	return mps.mappings
}

// FindMappingByProviderID 根据 Provider 任务ID 查找映射
func (mps *MultiProviderStorage) FindMappingByProviderID(provider, taskID string) *model.TaskMapping {
	mps.mu.RLock()
	defer mps.mu.RUnlock()

	for i := range mps.mappings.Mappings {
		if ref, ok := mps.mappings.Mappings[i].Providers[provider]; ok {
			if ref.ID == taskID {
				return &mps.mappings.Mappings[i]
			}
		}
	}
	return nil
}

// UpdateMapping 更新任务映射
func (mps *MultiProviderStorage) UpdateMapping(mapping model.TaskMapping) error {
	mps.mu.Lock()
	defer mps.mu.Unlock()

	// 查找现有映射
	for i, m := range mps.mappings.Mappings {
		if m.LocalID == mapping.LocalID {
			mps.mappings.Mappings[i] = mapping
			return mps.saveGlobalData()
		}
	}

	// 添加新映射
	mps.mappings.Mappings = append(mps.mappings.Mappings, mapping)
	return mps.saveGlobalData()
}

// RemoveMapping 删除任务映射
func (mps *MultiProviderStorage) RemoveMapping(localID string) error {
	mps.mu.Lock()
	defer mps.mu.Unlock()

	for i, m := range mps.mappings.Mappings {
		if m.LocalID == localID {
			mps.mappings.Mappings = append(
				mps.mappings.Mappings[:i],
				mps.mappings.Mappings[i+1:]...,
			)
			return mps.saveGlobalData()
		}
	}
	return nil
}

// --- 实现 storage.Storage 接口 ---

// SaveTask 保存任务
func (mps *MultiProviderStorage) SaveTask(ctx context.Context, task *model.Task) error {
	ps, err := mps.GetProviderStorage(string(task.Source))
	if err != nil {
		return err
	}
	return ps.SaveTask(ctx, task)
}

// GetTask 获取任务
func (mps *MultiProviderStorage) GetTask(ctx context.Context, id string) (*model.Task, error) {
	// 从映射中查找任务所属的 Provider
	mps.mu.RLock()
	for _, m := range mps.mappings.Mappings {
		if m.LocalID == id {
			for provider := range m.Providers {
				mps.mu.RUnlock()
				ps, err := mps.GetProviderStorage(provider)
				if err != nil {
					return nil, err
				}
				return ps.GetTask(ctx, id)
			}
		}
	}
	mps.mu.RUnlock()
	return nil, fmt.Errorf("task not found: %s", id)
}

// ListTasks 列出任务
func (mps *MultiProviderStorage) ListTasks(ctx context.Context, opts storage.ListOptions) ([]model.Task, error) {
	if opts.Source != "" {
		ps, err := mps.GetProviderStorage(string(opts.Source))
		if err != nil {
			return nil, err
		}
		return ps.ListTasks(ctx, opts)
	}

	// 汇总所有 Provider 的任务
	var result []model.Task
	providers := []string{"google", "microsoft", "feishu", "ticktick", "dida", "todoist"}
	for _, p := range providers {
		ps, err := mps.GetProviderStorage(p)
		if err != nil {
			continue
		}
		tasks, err := ps.ListTasks(ctx, opts)
		if err != nil {
			continue
		}
		result = append(result, tasks...)
	}
	return result, nil
}

// DeleteTask 删除任务
func (mps *MultiProviderStorage) DeleteTask(ctx context.Context, id string) error {
	// 从映射中查找任务所属的 Provider
	mps.mu.RLock()
	for _, m := range mps.mappings.Mappings {
		if m.LocalID == id {
			for provider := range m.Providers {
				mps.mu.RUnlock()
				ps, err := mps.GetProviderStorage(provider)
				if err != nil {
					return err
				}
				return ps.DeleteTask(ctx, id)
			}
		}
	}
	mps.mu.RUnlock()
	return fmt.Errorf("task not found: %s", id)
}

// SaveTasks 批量保存任务
func (mps *MultiProviderStorage) SaveTasks(ctx context.Context, tasks []*model.Task) error {
	// 按 Provider 分组
	byProvider := make(map[model.TaskSource][]*model.Task)
	for _, task := range tasks {
		byProvider[task.Source] = append(byProvider[task.Source], task)
	}

	// 分别保存
	for source, providerTasks := range byProvider {
		ps, err := mps.GetProviderStorage(string(source))
		if err != nil {
			return err
		}
		if err := ps.SaveTasks(ctx, providerTasks); err != nil {
			return err
		}
	}
	return nil
}

// QueryTasks 查询任务
func (mps *MultiProviderStorage) QueryTasks(ctx context.Context, query storage.Query) ([]model.Task, error) {
	var result []model.Task

	providers := []string{"google", "microsoft", "feishu", "ticktick", "dida", "todoist"}
	if len(query.Sources) > 0 {
		providers = make([]string, len(query.Sources))
		for i, s := range query.Sources {
			providers[i] = string(s)
		}
	}

	for _, p := range providers {
		ps, err := mps.GetProviderStorage(p)
		if err != nil {
			continue
		}
		// Provider 级查询时取消分页和排序，最后做统一排序/分页。
		providerQuery := query
		providerQuery.Limit = 0
		providerQuery.Offset = 0
		providerQuery.OrderBy = ""
		providerQuery.OrderDesc = false

		tasks, err := ps.QueryTasks(ctx, providerQuery)
		if err != nil {
			continue
		}
		result = append(result, tasks...)
	}

	if query.OrderBy != "" {
		sortTasks(result, query.OrderBy, query.OrderDesc)
	}

	if query.Offset > 0 || query.Limit > 0 {
		start := query.Offset
		if start > len(result) {
			return []model.Task{}, nil
		}
		end := len(result)
		if query.Limit > 0 && start+query.Limit < end {
			end = start + query.Limit
		}
		result = result[start:end]
	}

	return result, nil
}

// SaveTaskList 保存任务列表
func (mps *MultiProviderStorage) SaveTaskList(ctx context.Context, list *model.TaskList) error {
	ps, err := mps.GetProviderStorage(string(list.Source))
	if err != nil {
		return err
	}
	return ps.SaveTaskList(ctx, list)
}

// GetTaskList 获取任务列表
func (mps *MultiProviderStorage) GetTaskList(ctx context.Context, id string) (*model.TaskList, error) {
	providers := []string{"google", "microsoft", "feishu", "ticktick", "dida", "todoist"}
	for _, p := range providers {
		ps, err := mps.GetProviderStorage(p)
		if err != nil {
			continue
		}
		list, err := ps.GetTaskList(ctx, id)
		if err == nil {
			return list, nil
		}
	}
	return nil, fmt.Errorf("task list not found: %s", id)
}

// ListTaskLists 列出任务列表
func (mps *MultiProviderStorage) ListTaskLists(ctx context.Context) ([]model.TaskList, error) {
	var result []model.TaskList
	providers := []string{"google", "microsoft", "feishu", "ticktick", "dida", "todoist"}
	for _, p := range providers {
		ps, err := mps.GetProviderStorage(p)
		if err != nil {
			continue
		}
		lists, err := ps.ListTaskLists(ctx)
		if err != nil {
			continue
		}
		result = append(result, lists...)
	}
	return result, nil
}

// DeleteTaskList 删除任务列表
func (mps *MultiProviderStorage) DeleteTaskList(ctx context.Context, id string) error {
	providers := []string{"google", "microsoft", "feishu", "ticktick", "dida", "todoist"}
	for _, p := range providers {
		ps, err := mps.GetProviderStorage(p)
		if err != nil {
			continue
		}
		if err := ps.DeleteTaskList(ctx, id); err == nil {
			return nil
		}
	}
	return fmt.Errorf("task list not found: %s", id)
}

// ExportToJSON 导出为 JSON
func (mps *MultiProviderStorage) ExportToJSON(ctx context.Context, opts storage.ExportOptions) ([]byte, error) {
	tasks, err := mps.QueryTasks(ctx, opts.Query)
	if err != nil {
		return nil, err
	}

	if opts.Pretty {
		return json.MarshalIndent(tasks, "", "  ")
	}
	return json.Marshal(tasks)
}

// ExportToMarkdown 导出为 Markdown
func (mps *MultiProviderStorage) ExportToMarkdown(ctx context.Context, opts storage.ExportOptions) ([]byte, error) {
	// 复用 FileStorage 的实现
	fs := &FileStorage{}
	return fs.ExportToMarkdown(ctx, opts)
}

// GetLastSyncTime 获取上次同步时间
func (mps *MultiProviderStorage) GetLastSyncTime(ctx context.Context, source model.TaskSource) (*time.Time, error) {
	mps.mu.RLock()
	defer mps.mu.RUnlock()

	if meta, ok := mps.manifest.Providers[string(source)]; ok {
		if !meta.LastSync.IsZero() {
			return &meta.LastSync, nil
		}
	}
	return nil, nil
}

// SetLastSyncTime 设置上次同步时间
func (mps *MultiProviderStorage) SetLastSyncTime(ctx context.Context, source model.TaskSource, t time.Time) error {
	return mps.UpdateManifest(func(m *model.Manifest) {
		meta := m.Providers[string(source)]
		meta.LastSync = t
		m.Providers[string(source)] = meta
	})
}

// --- ProviderStorage 方法 ---

// NewProviderStorage 创建 Provider 存储
func NewProviderStorage(provider, basePath string) (*ProviderStorage, error) {
	ps := &ProviderStorage{
		provider:  provider,
		basePath:  basePath,
		tasksFile: filepath.Join(basePath, "tasks.json"),
		listsFile: filepath.Join(basePath, "lists.json"),
		metaFile:  filepath.Join(basePath, "meta.json"),
		tasks:     make(map[string]*model.Task),
		taskLists: make(map[string]*model.TaskList),
	}

	// 确保目录存在
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create provider directory: %w", err)
	}

	// 加载数据
	if err := ps.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load provider data: %w", err)
	}

	return ps, nil
}

// load 加载 Provider 数据
func (ps *ProviderStorage) load() error {
	// 加载任务
	if data, err := os.ReadFile(ps.tasksFile); err == nil {
		var tasks []*model.Task
		if err := json.Unmarshal(data, &tasks); err != nil {
			return fmt.Errorf("failed to unmarshal tasks: %w", err)
		}
		for _, task := range tasks {
			ps.tasks[task.ID] = task
		}
	}

	// 加载列表
	if data, err := os.ReadFile(ps.listsFile); err == nil {
		var lists []*model.TaskList
		if err := json.Unmarshal(data, &lists); err != nil {
			return fmt.Errorf("failed to unmarshal lists: %w", err)
		}
		for _, list := range lists {
			ps.taskLists[list.ID] = list
		}
	}

	// 加载元数据
	if data, err := os.ReadFile(ps.metaFile); err == nil {
		var meta model.ProviderData
		if err := json.Unmarshal(data, &meta); err != nil {
			return fmt.Errorf("failed to unmarshal meta: %w", err)
		}
		ps.meta = &meta
	} else if os.IsNotExist(err) {
		ps.meta = &model.ProviderData{
			Provider:     ps.provider,
			Capabilities: model.Capabilities{},
		}
	}

	return nil
}

// save 保存 Provider 数据
func (ps *ProviderStorage) save() error {
	// 保存任务
	tasks := make([]*model.Task, 0, len(ps.tasks))
	for _, task := range ps.tasks {
		tasks = append(tasks, task)
	}
	tasksData, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}
	if err := os.WriteFile(ps.tasksFile, tasksData, 0644); err != nil {
		return fmt.Errorf("failed to write tasks file: %w", err)
	}

	// 保存列表
	lists := make([]*model.TaskList, 0, len(ps.taskLists))
	for _, list := range ps.taskLists {
		lists = append(lists, list)
	}
	listsData, err := json.MarshalIndent(lists, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal lists: %w", err)
	}
	if err := os.WriteFile(ps.listsFile, listsData, 0644); err != nil {
		return fmt.Errorf("failed to write lists file: %w", err)
	}

	// 保存元数据
	metaData, err := json.MarshalIndent(ps.meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal meta: %w", err)
	}
	if err := os.WriteFile(ps.metaFile, metaData, 0644); err != nil {
		return fmt.Errorf("failed to write meta file: %w", err)
	}

	return nil
}

// SaveTask 保存任务
func (ps *ProviderStorage) SaveTask(_ context.Context, task *model.Task) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	task.UpdatedAt = time.Now()
	ps.tasks[task.ID] = task

	return ps.save()
}

// GetTask 获取任务
func (ps *ProviderStorage) GetTask(_ context.Context, id string) (*model.Task, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	task, ok := ps.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	return task, nil
}

// ListTasks 列出任务
func (ps *ProviderStorage) ListTasks(_ context.Context, opts storage.ListOptions) ([]model.Task, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	var result []model.Task
	for _, task := range ps.tasks {
		if opts.ListID != "" && task.ListID != opts.ListID {
			continue
		}
		result = append(result, *task)
	}
	return result, nil
}

// DeleteTask 删除任务
func (ps *ProviderStorage) DeleteTask(_ context.Context, id string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	delete(ps.tasks, id)
	return ps.save()
}

// SaveTasks 批量保存任务
func (ps *ProviderStorage) SaveTasks(_ context.Context, tasks []*model.Task) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	now := time.Now()
	for _, task := range tasks {
		task.UpdatedAt = now
		ps.tasks[task.ID] = task
	}

	return ps.save()
}

// QueryTasks 查询任务
func (ps *ProviderStorage) QueryTasks(_ context.Context, query storage.Query) ([]model.Task, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	queryText := normalizedQueryText(query)

	var result []model.Task
	for _, task := range ps.tasks {
		if !taskMatchesQuery(task, query, queryText) {
			continue
		}

		result = append(result, *task)
	}

	if query.OrderBy != "" {
		sortTasks(result, query.OrderBy, query.OrderDesc)
	}
	if query.Offset > 0 || query.Limit > 0 {
		start := query.Offset
		if start > len(result) {
			return []model.Task{}, nil
		}
		end := len(result)
		if query.Limit > 0 && start+query.Limit < end {
			end = start + query.Limit
		}
		result = result[start:end]
	}

	return result, nil
}

func normalizedQueryText(query storage.Query) string {
	if query.QueryText != "" {
		return query.QueryText
	}
	return query.FullText
}

func taskMatchesQuery(task *model.Task, query storage.Query, queryText string) bool {
	return matchTaskID(query.TaskIDs, task.ID) &&
		matchSource(query.Sources, task.Source) &&
		matchListID(query.ListIDs, task.ListID) &&
		matchListName(query.ListNames, task.ListName) &&
		matchStatus(query.Statuses, task.Status) &&
		matchQuadrant(query.Quadrants, task.Quadrant) &&
		matchPriority(query.Priorities, task.Priority) &&
		matchTags(query.Tags, task.Tags) &&
		matchDueDate(query.DueBefore, query.DueAfter, task.DueDate) &&
		matchQueryText(queryText, task)
}

func matchTaskID(ids []string, taskID string) bool {
	return len(ids) == 0 || containsStringCI(ids, taskID)
}

func matchSource(sources []model.TaskSource, source model.TaskSource) bool {
	if len(sources) == 0 {
		return true
	}
	for _, s := range sources {
		if source == s {
			return true
		}
	}
	return false
}

func matchListID(ids []string, listID string) bool {
	return len(ids) == 0 || containsStringCI(ids, listID)
}

func matchListName(listNames []string, listName string) bool {
	if len(listNames) == 0 {
		return true
	}
	for _, name := range listNames {
		if filter.MatchListNameExactNormalized(name, listName) {
			return true
		}
	}
	return false
}

func matchStatus(statuses []model.TaskStatus, status model.TaskStatus) bool {
	if len(statuses) == 0 {
		return true
	}
	for _, s := range statuses {
		if status == s {
			return true
		}
	}
	return false
}

func matchQuadrant(quadrants []model.Quadrant, quadrant model.Quadrant) bool {
	if len(quadrants) == 0 {
		return true
	}
	for _, q := range quadrants {
		if quadrant == q {
			return true
		}
	}
	return false
}

func matchPriority(priorities []model.Priority, priority model.Priority) bool {
	if len(priorities) == 0 {
		return true
	}
	for _, p := range priorities {
		if priority == p {
			return true
		}
	}
	return false
}

func matchTags(queryTags []string, taskTags []string) bool {
	if len(queryTags) == 0 {
		return true
	}
	for _, tag := range queryTags {
		found := false
		for _, taskTag := range taskTags {
			if strings.EqualFold(strings.TrimSpace(tag), strings.TrimSpace(taskTag)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func matchDueDate(dueBefore, dueAfter, dueDate *time.Time) bool {
	if dueBefore != nil && dueDate != nil && dueDate.After(*dueBefore) {
		return false
	}
	if dueAfter != nil && dueDate != nil && dueDate.Before(*dueAfter) {
		return false
	}
	return true
}

func matchQueryText(queryText string, task *model.Task) bool {
	if queryText == "" {
		return true
	}
	return filter.MatchQueryText(task, queryText)
}

func sortTasks(tasks []model.Task, orderBy string, orderDesc bool) {
	sort.Slice(tasks, func(i, j int) bool {
		a, b := tasks[i], tasks[j]
		cmp := compareTaskByOrder(a, b, orderBy)
		if orderDesc {
			return cmp > 0
		}
		return cmp < 0
	})
}

func compareTaskByOrder(a, b model.Task, orderBy string) int {
	switch orderBy {
	case "due_date":
		return compareTimePointers(a.DueDate, b.DueDate)
	case "priority":
		return int(a.Priority) - int(b.Priority)
	case "created_at":
		return compareTimes(a.CreatedAt, b.CreatedAt)
	case "updated_at":
		return compareTimes(a.UpdatedAt, b.UpdatedAt)
	default:
		return compareTimes(a.UpdatedAt, b.UpdatedAt)
	}
}

func compareTimePointers(a, b *time.Time) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return 1
	case b == nil:
		return -1
	default:
		return compareTimes(*a, *b)
	}
}

func compareTimes(a, b time.Time) int {
	if a.Before(b) {
		return -1
	}
	if a.After(b) {
		return 1
	}
	return 0
}

func containsStringCI(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, v := range values {
		if strings.EqualFold(strings.TrimSpace(v), target) {
			return true
		}
	}
	return false
}

// SaveTaskList 保存任务列表
func (ps *ProviderStorage) SaveTaskList(_ context.Context, list *model.TaskList) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	list.UpdatedAt = time.Now()
	ps.taskLists[list.ID] = list

	return ps.save()
}

// GetTaskList 获取任务列表
func (ps *ProviderStorage) GetTaskList(_ context.Context, id string) (*model.TaskList, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	list, ok := ps.taskLists[id]
	if !ok {
		return nil, fmt.Errorf("task list not found: %s", id)
	}
	return list, nil
}

// ListTaskLists 列出任务列表
func (ps *ProviderStorage) ListTaskLists(_ context.Context) ([]model.TaskList, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	var result []model.TaskList
	for _, list := range ps.taskLists {
		result = append(result, *list)
	}
	return result, nil
}

// DeleteTaskList 删除任务列表
func (ps *ProviderStorage) DeleteTaskList(_ context.Context, id string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	delete(ps.taskLists, id)
	return ps.save()
}

// GetMeta 获取元数据
func (ps *ProviderStorage) GetMeta() *model.ProviderData {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.meta
}

// UpdateMeta 更新元数据
func (ps *ProviderStorage) UpdateMeta(fn func(*model.ProviderData)) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	fn(ps.meta)
	return ps.save()
}
