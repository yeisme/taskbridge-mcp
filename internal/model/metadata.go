package model

import (
	"encoding/json"
	"strings"
	"time"
)

// MetadataMarker 元数据标记 - 用于在备注中识别元数据块
const MetadataMarker = "<!-- TaskBridge-Metadata:"

// MetadataEnd 元数据结束标记
const MetadataEnd = " -->"

// TaskMetadata 任务元数据 - 存储在原始软件的备注/描述中
type TaskMetadata struct {
	// Version 元数据版本
	Version string `json:"version"`

	// Quadrant 四象限分类（1-4）
	Quadrant int `json:"quadrant"`
	// Urgency 紧急程度（0-4）
	Urgency int `json:"urgency"`
	// Importance 重要程度（0-4）
	Importance int `json:"importance"`

	// Priority 优先级（0-4）
	Priority int `json:"priority"`
	// PriorityScore AI 计算的优先级分数
	PriorityScore int `json:"priority_score"`

	// AISuggestion AI 建议内容
	AISuggestion string `json:"ai_suggestion,omitempty"`
	// AIConfidence AI 建议置信度
	AIConfidence float64 `json:"ai_confidence,omitempty"`

	// EstimatedMinutes 预估时间（分钟）
	EstimatedMinutes int `json:"estimated_minutes,omitempty"`
	// ActualMinutes 实际时间（分钟）
	ActualMinutes int `json:"actual_minutes,omitempty"`
	// PomodoroCount 番茄钟数量
	PomodoroCount int `json:"pomodoro_count,omitempty"`

	// CustomFields 自定义字段
	CustomFields map[string]interface{} `json:"custom_fields,omitempty"`

	// LastSyncAt 最后同步时间
	LastSyncAt time.Time `json:"last_sync_at"`
	// SyncSource 同步来源
	SyncSource string `json:"sync_source"`
	// LocalID 本地任务 ID
	LocalID string `json:"local_id"`
}

// NewTaskMetadata 创建新的元数据
func NewTaskMetadata() *TaskMetadata {
	return &TaskMetadata{
		Version:    "1.0",
		LastSyncAt: time.Now(),
	}
}

// ToJSON 序列化为 JSON 字符串 - 用于嵌入备注
func (m *TaskMetadata) ToJSON() (string, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ParseMetadata 从字符串解析元数据
func ParseMetadata(s string) (*TaskMetadata, error) {
	var m TaskMetadata
	err := json.Unmarshal([]byte(s), &m)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// EmbedMetadata 将元数据嵌入到描述文本中
func EmbedMetadata(description string, metadata *TaskMetadata) (string, error) {
	// 先移除旧的元数据
	cleanDesc := ExtractMetadataOnly(description)

	jsonStr, err := metadata.ToJSON()
	if err != nil {
		return cleanDesc, err
	}
	return cleanDesc + "\n\n" + MetadataMarker + jsonStr + MetadataEnd, nil
}

// ExtractMetadataOnly 从描述文本中提取元数据，返回清理后的描述
func ExtractMetadataOnly(description string) string {
	startIdx := strings.Index(description, MetadataMarker)
	if startIdx == -1 {
		return description
	}

	endIdx := strings.Index(description[startIdx:], MetadataEnd)
	if endIdx == -1 {
		return description
	}

	// 移除元数据块
	return strings.TrimSpace(description[:startIdx] + description[startIdx+endIdx+len(MetadataEnd):])
}

// ExtractMetadata 从描述文本中提取元数据，返回描述和元数据
func ExtractMetadata(description string) (string, *TaskMetadata, error) {
	startIdx := strings.Index(description, MetadataMarker)
	if startIdx == -1 {
		return description, nil, nil
	}

	endIdx := strings.Index(description[startIdx:], MetadataEnd)
	if endIdx == -1 {
		return description, nil, nil
	}

	// 提取 JSON 部分
	jsonStart := startIdx + len(MetadataMarker)
	jsonEnd := startIdx + endIdx
	jsonStr := description[jsonStart:jsonEnd]

	metadata, err := ParseMetadata(jsonStr)
	if err != nil {
		return description, nil, err
	}

	// 移除元数据块，返回清理后的描述
	cleanDesc := strings.TrimSpace(description[:startIdx] + description[jsonEnd+len(MetadataEnd):])
	return cleanDesc, metadata, nil
}

// MetadataFromTask 从任务创建元数据
func MetadataFromTask(task *Task) *TaskMetadata {
	return &TaskMetadata{
		Version:          "1.0",
		Quadrant:         int(task.Quadrant),
		Urgency:          int(task.Urgency),
		Importance:       int(task.Importance),
		Priority:         int(task.Priority),
		PriorityScore:    task.PriorityScore,
		EstimatedMinutes: task.EstimatedMinutes,
		ActualMinutes:    task.ActualMinutes,
		LastSyncAt:       time.Now(),
		SyncSource:       string(task.Source),
		LocalID:          task.ID,
	}
}

// ApplyToTask 将元数据应用到任务
func (m *TaskMetadata) ApplyToTask(task *Task) {
	task.Quadrant = Quadrant(m.Quadrant)
	task.Urgency = UrgencyLevel(m.Urgency)
	task.Importance = ImportanceLevel(m.Importance)
	task.Priority = Priority(m.Priority)
	task.PriorityScore = m.PriorityScore
	task.EstimatedMinutes = m.EstimatedMinutes
	task.ActualMinutes = m.ActualMinutes
}
