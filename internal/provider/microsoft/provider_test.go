// Package microsoft provides Microsoft To Do provider implementation
package microsoft

import (
	"testing"
	"time"

	"github.com/yeisme/taskbridge/internal/model"
)

// TestNewProvider 测试创建 Provider
func TestNewProvider(t *testing.T) {
	cfg := Config{
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		TenantID:     "common",
		RedirectURL:  "http://localhost:8080/callback",
	}

	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	if p == nil {
		t.Fatal("NewProvider returned nil")
	}

	if p.Name() != "microsoft" {
		t.Errorf("Expected name 'microsoft', got '%s'", p.Name())
	}

	if p.DisplayName() != "Microsoft To Do" {
		t.Errorf("Expected display name 'Microsoft To Do', got '%s'", p.DisplayName())
	}
}

// TestCapabilities 测试能力查询
func TestCapabilities(t *testing.T) {
	cfg := Config{
		ClientID: "test_client_id",
	}

	p, _ := NewProvider(cfg)
	caps := p.Capabilities()

	// 验证已知的能力
	if !caps.SupportsSubtasks {
		t.Error("Expected SupportsSubtasks to be true")
	}

	if !caps.SupportsDueDate {
		t.Error("Expected SupportsDueDate to be true")
	}

	if !caps.SupportsPriority {
		t.Error("Expected SupportsPriority to be true")
	}

	if !caps.SupportsBatch {
		t.Error("Expected SupportsBatch to be true")
	}

	if !caps.SupportsDeltaSync {
		t.Error("Expected SupportsDeltaSync to be true")
	}
}

// TestStatusConversion 测试状态转换
func TestStatusConversion(t *testing.T) {
	tests := []struct {
		msStatus    TaskStatus
		modelStatus model.TaskStatus
	}{
		{StatusNotStarted, model.StatusTodo},
		{StatusInProgress, model.StatusInProgress},
		{StatusCompleted, model.StatusCompleted},
		{StatusWaitingOnOthers, model.StatusDeferred},
		{StatusDeferred, model.StatusDeferred},
	}

	for _, tt := range tests {
		t.Run(string(tt.msStatus), func(t *testing.T) {
			result := ToModelStatus(tt.msStatus)
			if result != tt.modelStatus {
				t.Errorf("ToModelStatus(%s) = %s, expected %s", tt.msStatus, result, tt.modelStatus)
			}
		})
	}

	// 测试反向转换（单独测试，因为不是一对一映射）
	reverseTests := []struct {
		modelStatus model.TaskStatus
		msStatus    TaskStatus
	}{
		{model.StatusTodo, StatusNotStarted},
		{model.StatusInProgress, StatusInProgress},
		{model.StatusCompleted, StatusCompleted},
		{model.StatusDeferred, StatusDeferred},
		{model.StatusCancelled, StatusDeferred},
	}

	for _, tt := range reverseTests {
		t.Run("reverse_"+string(tt.modelStatus), func(t *testing.T) {
			result := ToMicrosoftStatus(tt.modelStatus)
			if result != tt.msStatus {
				t.Errorf("ToMicrosoftStatus(%s) = %s, expected %s", tt.modelStatus, result, tt.msStatus)
			}
		})
	}
}

// TestPriorityConversion 测试优先级转换
func TestPriorityConversion(t *testing.T) {
	tests := []struct {
		msImportance  Importance
		modelPriority model.Priority
	}{
		{ImportanceHigh, model.PriorityHigh},
		{ImportanceNormal, model.PriorityMedium},
		{ImportanceLow, model.PriorityLow},
	}

	for _, tt := range tests {
		t.Run(string(tt.msImportance), func(t *testing.T) {
			result := ToModelPriority(tt.msImportance)
			if result != tt.modelPriority {
				t.Errorf("ToModelPriority(%s) = %d, expected %d", tt.msImportance, result, tt.modelPriority)
			}

			// 反向转换
			reverse := ToMicrosoftImportance(tt.modelPriority)
			if reverse != tt.msImportance {
				t.Errorf("ToMicrosoftImportance(%d) = %s, expected %s", tt.modelPriority, reverse, tt.msImportance)
			}
		})
	}
}

// TestDateTimeTimeZoneConversion 测试日期时间转换
func TestDateTimeTimeZoneConversion(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		result := FromDateTimeTimeZone(nil)
		if result != nil {
			t.Error("Expected nil result for nil input")
		}
	})

	t.Run("valid datetime", func(t *testing.T) {
		dtz := &DateTimeTimeZone{
			DateTime: "2024-01-15T10:30:00",
			TimeZone: "UTC",
		}

		result := FromDateTimeTimeZone(dtz)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if result.Year() != 2024 {
			t.Errorf("Expected year 2024, got %d", result.Year())
		}

		if result.Month() != time.January {
			t.Errorf("Expected month January, got %s", result.Month())
		}

		if result.Day() != 15 {
			t.Errorf("Expected day 15, got %d", result.Day())
		}
	})

	t.Run("date only", func(t *testing.T) {
		dtz := &DateTimeTimeZone{
			DateTime: "2024-06-20",
			TimeZone: "Asia/Shanghai",
		}

		result := FromDateTimeTimeZone(dtz)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if result.Year() != 2024 {
			t.Errorf("Expected year 2024, got %d", result.Year())
		}
	})
}

// TestToDateTimeTimeZone 测试转换为 Microsoft 日期时间格式
func TestToDateTimeTimeZone(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		result := ToDateTimeTimeZone(nil, "UTC")
		if result != nil {
			t.Error("Expected nil result for nil input")
		}
	})

	t.Run("valid time", func(t *testing.T) {
		tm := time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC)
		result := ToDateTimeTimeZone(&tm, "UTC")

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if result.TimeZone != "UTC" {
			t.Errorf("Expected timezone UTC, got '%s'", result.TimeZone)
		}

		if result.DateTime != "2024-03-15T14:30:00" {
			t.Errorf("Expected datetime '2024-03-15T14:30:00', got '%s'", result.DateTime)
		}
	})

	t.Run("empty timezone defaults to UTC", func(t *testing.T) {
		tm := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		result := ToDateTimeTimeZone(&tm, "")

		if result.TimeZone != "UTC" {
			t.Errorf("Expected timezone to default to UTC, got '%s'", result.TimeZone)
		}
	})
}

// TestTaskConversion 测试任务转换
func TestTaskConversion(t *testing.T) {
	now := time.Now()

	t.Run("to model task", func(t *testing.T) {
		msTask := &TodoTask{
			ID:                   "task123",
			Title:                "Test Task",
			Status:               StatusInProgress,
			Importance:           ImportanceHigh,
			CreatedDateTime:      now,
			LastModifiedDateTime: now,
			Body: &ItemBody{
				Content:     "Test description",
				ContentType: ContentTypeText,
			},
			DueDateTime: &DateTimeTimeZone{
				DateTime: "2024-12-31T23:59:59",
				TimeZone: "UTC",
			},
		}

		result := ToModelTask(msTask)

		if result.ID != "task123" {
			t.Errorf("Expected ID 'task123', got '%s'", result.ID)
		}

		if result.Title != "Test Task" {
			t.Errorf("Expected Title 'Test Task', got '%s'", result.Title)
		}

		if result.Status != model.StatusInProgress {
			t.Errorf("Expected Status '%s', got '%s'", model.StatusInProgress, result.Status)
		}

		if result.Priority != model.PriorityHigh {
			t.Errorf("Expected Priority '%d', got '%d'", model.PriorityHigh, result.Priority)
		}

		if result.Description != "Test description" {
			t.Errorf("Expected Description 'Test description', got '%s'", result.Description)
		}

		if result.Source != model.SourceMicrosoft {
			t.Errorf("Expected Source '%s', got '%s'", model.SourceMicrosoft, result.Source)
		}
	})

	t.Run("to microsoft task", func(t *testing.T) {
		mTask := &model.Task{
			ID:          "task456",
			Title:       "Test Model Task",
			Status:      model.StatusTodo,
			Priority:    model.PriorityMedium,
			Description: "Model description",
			DueDate:     &now,
		}

		result := ToMicrosoftTask(mTask)

		if result.ID != "task456" {
			t.Errorf("Expected ID 'task456', got '%s'", result.ID)
		}

		if result.Title != "Test Model Task" {
			t.Errorf("Expected Title 'Test Model Task', got '%s'", result.Title)
		}

		if result.Status != StatusNotStarted {
			t.Errorf("Expected Status '%s', got '%s'", StatusNotStarted, result.Status)
		}

		if result.Importance != ImportanceNormal {
			t.Errorf("Expected Importance '%s', got '%s'", ImportanceNormal, result.Importance)
		}

		if result.Body == nil || result.Body.Content != "Model description" {
			t.Error("Expected Body with correct content")
		}
	})
}

// TestTaskListConversion 测试任务列表转换
func TestTaskListConversion(t *testing.T) {
	now := time.Now()

	t.Run("to model task list", func(t *testing.T) {
		msList := &TodoTaskList{
			ID:              "list123",
			DisplayName:     "Test List",
			CreatedDateTime: now,
			LastModified:    now,
		}

		result := ToModelTaskList(msList)

		if result.ID != "list123" {
			t.Errorf("Expected ID 'list123', got '%s'", result.ID)
		}

		if result.Name != "Test List" {
			t.Errorf("Expected Name 'Test List', got '%s'", result.Name)
		}

		if result.Source != model.SourceMicrosoft {
			t.Errorf("Expected Source '%s', got '%s'", model.SourceMicrosoft, result.Source)
		}
	})
}

// TestCalculateQuadrantFromMSTask 测试四象限计算
func TestCalculateQuadrantFromMSTask(t *testing.T) {
	tests := []struct {
		name     string
		task     *TodoTask
		quadrant model.Quadrant
	}{
		{
			name: "high importance, urgent due",
			task: &TodoTask{
				Importance: ImportanceHigh,
				DueDateTime: &DateTimeTimeZone{
					DateTime: time.Now().Add(2 * time.Hour).Format("2006-01-02T15:04:05"),
					TimeZone: "UTC",
				},
			},
			quadrant: model.QuadrantUrgentImportant,
		},
		{
			name: "high importance, not urgent",
			task: &TodoTask{
				Importance: ImportanceHigh,
				DueDateTime: &DateTimeTimeZone{
					DateTime: time.Now().Add(7 * 24 * time.Hour).Format("2006-01-02T15:04:05"),
					TimeZone: "UTC",
				},
			},
			quadrant: model.QuadrantNotUrgentImportant,
		},
		{
			name: "normal importance, urgent due",
			task: &TodoTask{
				Importance: ImportanceNormal,
				DueDateTime: &DateTimeTimeZone{
					DateTime: time.Now().Add(2 * time.Hour).Format("2006-01-02T15:04:05"),
					TimeZone: "UTC",
				},
			},
			quadrant: model.QuadrantUrgentNotImportant,
		},
		{
			name: "normal importance, no due date",
			task: &TodoTask{
				Importance: ImportanceNormal,
			},
			quadrant: model.QuadrantNotUrgentNotImportant,
		},
		{
			name:     "nil task",
			task:     nil,
			quadrant: model.QuadrantNotUrgentNotImportant,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateQuadrantFromMSTask(tt.task)
			if result != tt.quadrant {
				t.Errorf("Expected quadrant %d, got %d", tt.quadrant, result)
			}
		})
	}
}

// TestBatchConversion 测试批量转换
func TestBatchConversion(t *testing.T) {
	t.Run("to model tasks", func(t *testing.T) {
		msTasks := []TodoTask{
			{ID: "1", Title: "Task 1", Status: StatusNotStarted},
			{ID: "2", Title: "Task 2", Status: StatusCompleted},
		}

		result := ToModelTasks(msTasks)

		if len(result) != 2 {
			t.Fatalf("Expected 2 tasks, got %d", len(result))
		}

		if result[0].ID != "1" || result[1].ID != "2" {
			t.Error("Task IDs not preserved in batch conversion")
		}
	})

	t.Run("to microsoft tasks", func(t *testing.T) {
		mTasks := []model.Task{
			{ID: "1", Title: "Task 1", Status: model.StatusTodo},
			{ID: "2", Title: "Task 2", Status: model.StatusCompleted},
		}

		result := ToMicrosoftTasks(mTasks)

		if len(result) != 2 {
			t.Fatalf("Expected 2 tasks, got %d", len(result))
		}

		if result[0].ID != "1" || result[1].ID != "2" {
			t.Error("Task IDs not preserved in batch conversion")
		}
	})

	t.Run("nil input", func(t *testing.T) {
		if ToModelTasks(nil) != nil {
			t.Error("Expected nil for nil input")
		}

		if ToMicrosoftTasks(nil) != nil {
			t.Error("Expected nil for nil input")
		}
	})
}

// TestIsAuthenticated 测试认证状态检查
func TestIsAuthenticated(t *testing.T) {
	cfg := Config{
		ClientID: "test_client_id",
	}

	p, _ := NewProvider(cfg)

	// 未认证状态
	if p.IsAuthenticated() {
		t.Error("Expected IsAuthenticated to be false for new provider")
	}
}

// TestGetTokenInfo 测试获取 Token 信息
func TestGetTokenInfo(t *testing.T) {
	cfg := Config{
		ClientID: "test_client_id",
	}

	p, _ := NewProvider(cfg)

	info := p.GetTokenInfo()

	if info == nil {
		t.Fatal("Expected non-nil TokenInfo")
	}

	if info.Provider != "microsoft" {
		t.Errorf("Expected Provider 'microsoft', got '%s'", info.Provider)
	}

	// 未认证状态
	if info.HasToken {
		t.Error("Expected HasToken to be false")
	}
}

// TestMapStatusToProgress 测试状态到进度的映射
func TestMapStatusToProgress(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		progress int
	}{
		{StatusNotStarted, 0},
		{StatusInProgress, 50},
		{StatusWaitingOnOthers, 25},
		{StatusDeferred, 10},
		{StatusCompleted, 100},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			result := MapStatusToProgress(tt.status)
			if result != tt.progress {
				t.Errorf("MapStatusToProgress(%s) = %d, expected %d", tt.status, result, tt.progress)
			}
		})
	}
}

// TestMapProgressToStatus 测试进度到状态的映射
func TestMapProgressToStatus(t *testing.T) {
	tests := []struct {
		progress int
		status   TaskStatus
	}{
		{0, StatusNotStarted},
		{10, StatusInProgress},
		{25, StatusWaitingOnOthers},
		{50, StatusInProgress},
		{75, StatusInProgress},
		{100, StatusCompleted},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := MapProgressToStatus(tt.progress)
			if result != tt.status {
				t.Errorf("MapProgressToStatus(%d) = %s, expected %s", tt.progress, result, tt.status)
			}
		})
	}
}
