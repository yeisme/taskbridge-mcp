package filestore

import (
	"context"
	"testing"
	"time"

	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/storage"
)

func TestQueryTasksExtendedFilters(t *testing.T) {
	dir := t.TempDir()
	fs, err := New(dir, "json")
	if err != nil {
		t.Fatalf("failed to create filestore: %v", err)
	}

	ctx := context.Background()
	now := time.Now()
	tasks := []*model.Task{
		{
			ID:        "task-1",
			Title:     "学习 Kubernetes",
			Status:    model.StatusTodo,
			CreatedAt: now,
			UpdatedAt: now,
			Source:    model.SourceMicrosoft,
			ListID:    "list-a",
			ListName:  "🏗 学习与成长",
			Tags:      []string{"k8s"},
			Priority:  model.PriorityMedium,
			Quadrant:  model.QuadrantNotUrgentNotImportant,
		},
		{
			ID:        "task-2",
			Title:     "项目周报",
			Status:    model.StatusCompleted,
			CreatedAt: now,
			UpdatedAt: now,
			Source:    model.SourceMicrosoft,
			ListID:    "list-b",
			ListName:  "🏢 项目与工作",
			Tags:      []string{"work"},
			Priority:  model.PriorityHigh,
			Quadrant:  model.QuadrantUrgentImportant,
		},
	}

	for _, task := range tasks {
		if err := fs.SaveTask(ctx, task); err != nil {
			t.Fatalf("failed to save task %s: %v", task.ID, err)
		}
	}

	t.Run("filter by list_id", func(t *testing.T) {
		result, err := fs.QueryTasks(ctx, storage.Query{
			ListIDs: []string{"list-a"},
		})
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if len(result) != 1 || result[0].ID != "task-1" {
			t.Fatalf("unexpected result: %+v", result)
		}
	})

	t.Run("filter by normalized list_name", func(t *testing.T) {
		result, err := fs.QueryTasks(ctx, storage.Query{
			ListNames: []string{"学习与成长"},
		})
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if len(result) != 1 || result[0].ID != "task-1" {
			t.Fatalf("unexpected result: %+v", result)
		}
	})

	t.Run("filter by task_ids", func(t *testing.T) {
		result, err := fs.QueryTasks(ctx, storage.Query{
			TaskIDs: []string{"task-2"},
		})
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if len(result) != 1 || result[0].ID != "task-2" {
			t.Fatalf("unexpected result: %+v", result)
		}
	})

	t.Run("filter by query text", func(t *testing.T) {
		result, err := fs.QueryTasks(ctx, storage.Query{
			QueryText: "kubernetes 学习与成长",
		})
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if len(result) != 1 || result[0].ID != "task-1" {
			t.Fatalf("unexpected result: %+v", result)
		}
	})

	t.Run("all filters use AND", func(t *testing.T) {
		result, err := fs.QueryTasks(ctx, storage.Query{
			ListNames: []string{"学习与成长"},
			Statuses:  []model.TaskStatus{model.StatusCompleted},
		})
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if len(result) != 0 {
			t.Fatalf("expected empty result, got %+v", result)
		}
	})
}
