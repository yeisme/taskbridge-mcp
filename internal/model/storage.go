// Package model 数据模型定义
package model

import (
	"time"
)

// Manifest 全局清单文件
type Manifest struct {
	Version   string                  `json:"version"`
	CreatedAt time.Time               `json:"created_at"`
	UpdatedAt time.Time               `json:"updated_at"`
	Providers map[string]ProviderMeta `json:"providers"`
	SyncPairs []SyncPair              `json:"sync_pairs"`
}

// ProviderMeta Provider 元数据
type ProviderMeta struct {
	Enabled       bool      `json:"enabled"`
	Authenticated bool      `json:"authenticated"`
	LastSync      time.Time `json:"last_sync"`
	TaskCount     int       `json:"task_count"`
	ListCount     int       `json:"list_count"`
}

// SyncPair 同步对配置
type SyncPair struct {
	Source     string `json:"source"`
	Target     string `json:"target"`
	Enabled    bool   `json:"enabled"`
	Direction  string `json:"direction"`  // bidirectional, pull, push
	Resolution string `json:"resolution"` // latest_wins, source_wins, target_wins
}

// ProviderData Provider 数据元信息
type ProviderData struct {
	Provider      string        `json:"provider"`
	DisplayName   string        `json:"display_name"`
	LastFullSync  time.Time     `json:"last_full_sync"`
	LastDeltaSync time.Time     `json:"last_delta_sync"`
	SyncToken     string        `json:"sync_token"`
	ETag          string        `json:"etag"`
	Capabilities  Capabilities  `json:"capabilities"`
	Stats         ProviderStats `json:"stats"`
}

// Capabilities Provider 能力
type Capabilities struct {
	SupportsDelta     bool `json:"supports_delta"`
	SupportsSubtasks  bool `json:"supports_subtasks"`
	SupportsPriority  bool `json:"supports_priority"`
	SupportsTags      bool `json:"supports_tags"`
	SupportsReminders bool `json:"supports_reminders"`
}

// ProviderStats Provider 统计信息
type ProviderStats struct {
	TotalTasks     int `json:"total_tasks"`
	CompletedTasks int `json:"completed_tasks"`
	PendingTasks   int `json:"pending_tasks"`
	Lists          int `json:"lists"`
}

// SyncState 同步状态数据库
type SyncState struct {
	Version           string        `json:"version"`
	SyncSessions      []SyncSession `json:"sync_sessions"`
	PendingOperations []PendingOp   `json:"pending_operations"`
}

// SyncSession 同步会话
type SyncSession struct {
	ID          string      `json:"id"`
	Direction   string      `json:"direction"` // pull, push, bidirectional
	Source      string      `json:"source"`
	Target      string      `json:"target,omitempty"`
	StartedAt   time.Time   `json:"started_at"`
	CompletedAt time.Time   `json:"completed_at"`
	Status      string      `json:"status"` // running, completed, failed, conflict
	Stats       SyncOpStats `json:"stats"`
	Errors      []SyncError `json:"errors,omitempty"`
}

// SyncOpStats 同步操作统计
type SyncOpStats struct {
	Pulled  int `json:"pulled"`
	Pushed  int `json:"pushed"`
	Updated int `json:"updated"`
	Deleted int `json:"deleted"`
	Skipped int `json:"skipped"`
	Errors  int `json:"errors"`
}

// SyncError 同步错误
type SyncError struct {
	TaskID    string `json:"task_id"`
	Operation string `json:"operation"`
	Message   string `json:"message"`
}

// PendingOp 待处理操作
type PendingOp struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // create, update, delete
	Provider  string    `json:"provider"`
	TaskID    string    `json:"task_id"`
	CreatedAt time.Time `json:"created_at"`
	Retries   int       `json:"retries"`
	Status    string    `json:"status"` // pending, retrying, failed
	LastError string    `json:"last_error,omitempty"`
}

// TaskMapping 跨 Provider 任务映射
type TaskMapping struct {
	LocalID    string                 `json:"local_id"`
	TitleHash  string                 `json:"title_hash"`
	Providers  map[string]ProviderRef `json:"providers"`
	SyncStatus string                 `json:"sync_status"` // synced, pending, conflict
	LastSync   time.Time              `json:"last_sync"`
	Resolution string                 `json:"resolution"`
}

// ProviderRef Provider 引用
type ProviderRef struct {
	ID           string    `json:"id"`
	ListID       string    `json:"list_id"`
	ETag         string    `json:"etag"`
	LastModified time.Time `json:"last_modified"`
	Version      int       `json:"version"`
}

// MappingDatabase 映射数据库
type MappingDatabase struct {
	Version  string        `json:"version"`
	Mappings []TaskMapping `json:"mappings"`
}

// NewManifest 创建新清单
func NewManifest() *Manifest {
	return &Manifest{
		Version:   "1.0",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Providers: make(map[string]ProviderMeta),
		SyncPairs: []SyncPair{},
	}
}

// NewSyncState 创建新同步状态
func NewSyncState() *SyncState {
	return &SyncState{
		Version:           "1.0",
		SyncSessions:      []SyncSession{},
		PendingOperations: []PendingOp{},
	}
}

// NewMappingDatabase 创建新映射数据库
func NewMappingDatabase() *MappingDatabase {
	return &MappingDatabase{
		Version:  "1.0",
		Mappings: []TaskMapping{},
	}
}
