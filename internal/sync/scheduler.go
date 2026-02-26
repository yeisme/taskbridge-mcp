// Package sync 提供任务同步功能
package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"

	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/internal/storage"
)

// SchedulerConfig 调度器配置
type SchedulerConfig struct {
	// Cron 表达式，如 "0 */5 * * * *" 表示每5分钟执行一次
	CronExpression string `json:"cron_expression"`
	// 同步方向
	Direction Direction `json:"direction"`
	// 要同步的 Provider 列表，为空则同步所有
	Providers []string `json:"providers,omitempty"`
	// 是否启用增量同步
	Incremental bool `json:"incremental"`
	// 错误重试次数
	MaxRetries int `json:"max_retries"`
	// 重试间隔
	RetryInterval time.Duration `json:"retry_interval"`
	// 冲突解决策略
	ConflictResolve string `json:"conflict_resolve"`
	// 是否删除远程多余任务
	DeleteRemote bool `json:"delete_remote"`
}

// SchedulerStats 调度器统计
type SchedulerStats struct {
	// TotalRuns 总运行次数
	TotalRuns int `json:"total_runs"`
	// SuccessRuns 成功运行次数
	SuccessRuns int `json:"success_runs"`
	// FailedRuns 失败运行次数
	FailedRuns int `json:"failed_runs"`
	// LastRunTime 最后一次运行时间
	LastRunTime time.Time `json:"last_run_time"`
	// LastRunStatus 最后一次运行状态（success/failed）
	LastRunStatus string `json:"last_run_status"`
	// LastRunResult 最后一次运行结果
	LastRunResult *Result `json:"last_run_result,omitempty"`
	// TotalPulled 总拉取任务数
	TotalPulled int `json:"total_pulled"`
	// TotalPushed 总推送任务数
	TotalPushed int `json:"total_pushed"`
	// TotalErrors 总错误数
	TotalErrors int `json:"total_errors"`
	// AverageRuntime 平均运行时间
	AverageRuntime time.Duration `json:"average_runtime"`
}

// Scheduler 定时同步调度器
type Scheduler struct {
	// config 调度器配置
	config SchedulerConfig
	// engine 同步引擎
	engine *Engine
	// cron Cron 调度器实例
	cron *cron.Cron
	// storage 存储接口
	storage storage.Storage
	// providers Provider 映射表
	providers map[string]provider.Provider
	// stats 运行统计
	stats SchedulerStats
	// mu 读写锁
	mu sync.RWMutex
	// running 是否正在运行
	running bool
	// entryID Cron 任务 ID
	entryID cron.EntryID
}

// NewScheduler 创建调度器
func NewScheduler(config SchedulerConfig, providers map[string]provider.Provider, store storage.Storage) *Scheduler {
	// 设置默认值
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryInterval == 0 {
		config.RetryInterval = 30 * time.Second
	}
	if config.ConflictResolve == "" {
		config.ConflictResolve = "newer"
	}

	return &Scheduler{
		config:    config,
		engine:    NewEngine(providers, store),
		storage:   store,
		providers: providers,
		cron:      cron.New(cron.WithSeconds(), cron.WithLocation(time.Local)),
	}
}

// Start 启动调度器
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("scheduler is already running")
	}

	// 解析 cron 表达式
	schedule, err := cron.ParseStandard(s.config.CronExpression)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	// 添加定时任务
	s.entryID = s.cron.Schedule(schedule, cron.FuncJob(func() {
		s.runSync(ctx)
	}))

	s.cron.Start()
	s.running = true

	log.Info().
		Str("cron", s.config.CronExpression).
		Bool("incremental", s.config.Incremental).
		Int("max_retries", s.config.MaxRetries).
		Msg("同步调度器已启动")

	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	ctx := s.cron.Stop()
	<-ctx.Done()

	s.running = false
	log.Info().Msg("同步调度器已停止")

	return nil
}

// IsRunning 检查调度器是否运行中
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetStats 获取调度器统计
func (s *Scheduler) GetStats() SchedulerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

// Trigger 手动触发一次同步
func (s *Scheduler) Trigger(ctx context.Context) (*Result, error) {
	return s.runSyncWithRetry(ctx)
}

// runSync 执行同步（定时任务入口）
func (s *Scheduler) runSync(ctx context.Context) {
	result, err := s.runSyncWithRetry(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.stats.TotalRuns++
	s.stats.LastRunTime = time.Now()

	if err != nil {
		s.stats.FailedRuns++
		s.stats.LastRunStatus = "failed"
		log.Error().Err(err).Msg("定时同步失败")
	} else {
		s.stats.SuccessRuns++
		s.stats.LastRunStatus = "success"
		s.stats.TotalPulled += result.Pulled
		s.stats.TotalPushed += result.Pushed
		s.stats.TotalErrors += len(result.Errors)
		s.stats.LastRunResult = result

		log.Info().
			Int("pulled", result.Pulled).
			Int("pushed", result.Pushed).
			Int("updated", result.Updated).
			Int("errors", len(result.Errors)).
			Dur("duration", result.Duration).
			Msg("定时同步完成")
	}

	// 更新平均运行时间
	if s.stats.TotalRuns > 0 {
		totalRuntime := s.stats.AverageRuntime * time.Duration(s.stats.TotalRuns-1)
		if s.stats.LastRunResult != nil {
			totalRuntime += s.stats.LastRunResult.Duration
		}
		s.stats.AverageRuntime = totalRuntime / time.Duration(s.stats.TotalRuns)
	}
}

// runSyncWithRetry 带重试的同步
func (s *Scheduler) runSyncWithRetry(ctx context.Context) (*Result, error) {
	var lastErr error

	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		if attempt > 0 {
			log.Warn().
				Int("attempt", attempt).
				Dur("interval", s.config.RetryInterval).
				Err(lastErr).
				Msg("同步失败，准备重试")

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(s.config.RetryInterval):
			}
		}

		result, err := s.doSync(ctx)
		if err == nil {
			return result, nil
		}
		lastErr = err

		// 检查是否为可重试错误
		if !s.isRetryableError(err) {
			break
		}
	}

	return nil, fmt.Errorf("同步失败，已重试 %d 次: %w", s.config.MaxRetries, lastErr)
}

// doSync 执行实际的同步
func (s *Scheduler) doSync(ctx context.Context) (*Result, error) {
	// 确定要同步的 Provider
	providerNames := s.config.Providers
	if len(providerNames) == 0 {
		for name := range s.providers {
			providerNames = append(providerNames, name)
		}
	}

	// 如果只有一个 Provider，直接同步
	if len(providerNames) == 1 {
		return s.syncProvider(ctx, providerNames[0])
	}

	// 多个 Provider，合并结果
	combinedResult := &Result{
		Direction:    s.config.Direction,
		LastSyncTime: time.Now(),
	}

	for _, name := range providerNames {
		result, err := s.syncProvider(ctx, name)
		if err != nil {
			combinedResult.Errors = append(combinedResult.Errors, Error{
				Operation: "sync_provider",
				Error:     fmt.Sprintf("Provider %s 同步失败: %v", name, err),
			})
			continue
		}

		combinedResult.Pulled += result.Pulled
		combinedResult.Pushed += result.Pushed
		combinedResult.Updated += result.Updated
		combinedResult.Deleted += result.Deleted
		combinedResult.Skipped += result.Skipped
		combinedResult.Errors = append(combinedResult.Errors, result.Errors...)
	}

	combinedResult.Duration = time.Since(combinedResult.LastSyncTime)
	return combinedResult, nil
}

// syncProvider 同步单个 Provider
func (s *Scheduler) syncProvider(ctx context.Context, providerName string) (*Result, error) {
	// 获取最后同步时间（用于增量同步）
	var since time.Time
	if s.config.Incremental {
		lastSync, err := s.storage.GetLastSyncTime(ctx, model.TaskSource(providerName))
		if err == nil && lastSync != nil {
			since = *lastSync
		}
	}

	opts := Options{
		Direction:       s.config.Direction,
		Provider:        providerName,
		ConflictResolve: s.config.ConflictResolve,
		DeleteRemote:    s.config.DeleteRemote,
		Since:           since,
	}

	return s.engine.Sync(ctx, opts)
}

// isRetryableError 检查是否为可重试错误
func (s *Scheduler) isRetryableError(err error) bool {
	// 网络错误、超时错误、服务暂时不可用等可以重试
	errStr := err.Error()
	retryablePatterns := []string{
		"timeout",
		"connection refused",
		"temporary",
		"503",
		"502",
		"429", // rate limit
		"network",
		"EOF",
	}

	for _, pattern := range retryablePatterns {
		if contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// contains 检查字符串是否包含子串（忽略大小写）
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > 0 && containsSubstr(s, substr)))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			subc := substr[j]
			// 简单的小写转换
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if subc >= 'A' && subc <= 'Z' {
				subc += 32
			}
			if sc != subc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// NextRunTime 获取下次运行时间
func (s *Scheduler) NextRunTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running || s.entryID == 0 {
		return time.Time{}
	}

	entry := s.cron.Entry(s.entryID)
	return entry.Next
}

// UpdateConfig 更新调度器配置
func (s *Scheduler) UpdateConfig(config SchedulerConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	wasRunning := s.running

	// 如果正在运行，先停止
	if s.running {
		ctx := s.cron.Stop()
		<-ctx.Done()
		s.running = false
	}

	// 更新配置
	s.config = config

	// 如果之前在运行，重新启动
	if wasRunning {
		s.mu.Unlock()
		err := s.Start(context.Background())
		s.mu.Lock()
		return err
	}

	return nil
}

// ================ 增量同步支持 ================

// IncrementalSyncConfig 增量同步配置
type IncrementalSyncConfig struct {
	// Provider 名称
	Provider string `json:"provider"`
	// 同步方向
	Direction Direction `json:"direction"`
	// 增量同步令牌（由系统维护）
	DeltaToken string `json:"delta_token,omitempty"`
	// 最后同步时间
	LastSyncTime time.Time `json:"last_sync_time"`
	// 是否首次同步
	IsFirstSync bool `json:"is_first_sync"`
}

// IncrementalSyncState 增量同步状态
type IncrementalSyncState struct {
	mu      sync.RWMutex
	configs map[string]*IncrementalSyncConfig // key: provider name
	storage storage.Storage
}

// NewIncrementalSyncState 创建增量同步状态
func NewIncrementalSyncState(store storage.Storage) *IncrementalSyncState {
	return &IncrementalSyncState{
		configs: make(map[string]*IncrementalSyncConfig),
		storage: store,
	}
}

// GetConfig 获取 Provider 的增量同步配置
func (s *IncrementalSyncState) GetConfig(providerName string) *IncrementalSyncConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if config, ok := s.configs[providerName]; ok {
		return config
	}

	// 从存储加载
	config := &IncrementalSyncConfig{
		Provider: providerName,
	}

	lastSync, err := s.storage.GetLastSyncTime(context.Background(), model.TaskSource(providerName))
	if err == nil && lastSync != nil {
		config.LastSyncTime = *lastSync
		config.IsFirstSync = false
	} else {
		config.IsFirstSync = true
	}

	s.configs[providerName] = config
	return config
}

// UpdateConfig 更新增量同步配置
func (s *IncrementalSyncState) UpdateConfig(providerName string, config *IncrementalSyncConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.configs[providerName] = config

	// 持久化最后同步时间
	if !config.LastSyncTime.IsZero() {
		_ = s.storage.SetLastSyncTime(context.Background(), model.TaskSource(providerName), config.LastSyncTime)
	}
}

// SetDeltaToken 设置增量同步令牌
func (s *IncrementalSyncState) SetDeltaToken(providerName, token string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if config, ok := s.configs[providerName]; ok {
		config.DeltaToken = token
		config.LastSyncTime = time.Now()
		config.IsFirstSync = false
	}
}
