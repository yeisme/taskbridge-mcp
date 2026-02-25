package projectplanner

import (
	"strings"
	"testing"

	"github.com/yeisme/taskbridge/internal/project"
)

func TestDecomposeLearning(t *testing.T) {
	plan := Decompose(DecomposeInput{
		ProjectID:   "proj_1",
		ProjectName: "学习 openclaw",
		GoalText:    "我希望学习 openclaw",
		HorizonDays: 14,
	})

	if plan.GoalType != project.GoalTypeLearning {
		t.Fatalf("goal type=%s, want learning", plan.GoalType)
	}
	if len(plan.Phases) == 0 || plan.Phases[0] != "目标澄清" {
		t.Fatalf("unexpected phases: %#v", plan.Phases)
	}
	if len(plan.TasksPreview) < 6 {
		t.Fatalf("tasks too few: %d", len(plan.TasksPreview))
	}
}

func TestDecomposeTravel(t *testing.T) {
	plan := Decompose(DecomposeInput{
		ProjectID:   "proj_2",
		ProjectName: "上海旅游",
		GoalText:    "我希望去上海旅游",
		HorizonDays: 10,
	})

	if plan.GoalType != project.GoalTypeTravel {
		t.Fatalf("goal type=%s, want travel", plan.GoalType)
	}
	if len(plan.TasksPreview) < 6 {
		t.Fatalf("tasks too few: %d", len(plan.TasksPreview))
	}
	for _, task := range plan.TasksPreview {
		if task.EstimateMinutes < 30 || task.EstimateMinutes > 180 {
			t.Fatalf("estimate out of range: %d", task.EstimateMinutes)
		}
	}
}

func TestDecomposeWithConstraints(t *testing.T) {
	plan := Decompose(DecomposeInput{
		ProjectID:   "proj_3",
		ProjectName: "学习 openclaw",
		GoalText:    "我希望学习 openclaw",
		Constraints: project.PlanConstraints{
			RequireDeliverable: true,
			MinEstimateMinutes: 40,
			MaxEstimateMinutes: 100,
			MinTasks:           8,
			MaxTasks:           10,
			MinPracticeTasks:   2,
		},
	})

	if len(plan.TasksPreview) < 8 || len(plan.TasksPreview) > 10 {
		t.Fatalf("tasks out of range: %d", len(plan.TasksPreview))
	}
	practiceCount := 0
	for _, task := range plan.TasksPreview {
		if task.EstimateMinutes < 40 || task.EstimateMinutes > 100 {
			t.Fatalf("estimate out of constrained range: %d", task.EstimateMinutes)
		}
		if !strings.Contains(task.Description, "产出：") {
			t.Fatalf("task missing deliverable description: %s", task.Title)
		}
		for _, tag := range task.Tags {
			if strings.EqualFold(tag, "practice") {
				practiceCount++
				break
			}
		}
	}
	if practiceCount < 2 {
		t.Fatalf("practice tasks too few: %d", practiceCount)
	}
}
