package project

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFileStoreSaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()

	store, err := NewFileStore(tmp)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	project := &Project{
		ID:       "proj_1",
		Name:     "学习 openclaw",
		GoalText: "我希望学习 openclaw",
		GoalType: GoalTypeLearning,
		Status:   StatusDraft,
	}
	if err := store.SaveProject(ctx, project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	plan := &PlanSuggestion{
		PlanID:       "plan_1",
		ProjectID:    "proj_1",
		GoalType:     GoalTypeLearning,
		Status:       StatusSplitSuggested,
		Phases:       []string{"目标澄清"},
		TasksPreview: []PlanTask{{Title: "任务1", EstimateMinutes: 60, DueOffsetDays: 1, Priority: 3, Quadrant: 2}},
	}
	if err := store.SavePlan(ctx, plan); err != nil {
		t.Fatalf("SavePlan: %v", err)
	}

	reloaded, err := NewFileStore(filepath.Dir(filepath.Join(tmp, "projects.json")))
	if err != nil {
		t.Fatalf("reload NewFileStore: %v", err)
	}
	loadedProject, err := reloaded.GetProject(ctx, "proj_1")
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if loadedProject.Name != "学习 openclaw" {
		t.Fatalf("unexpected project name: %s", loadedProject.Name)
	}
	loadedPlan, err := reloaded.GetLatestPlan(ctx, "proj_1")
	if err != nil {
		t.Fatalf("GetLatestPlan: %v", err)
	}
	if loadedPlan.PlanID != "plan_1" {
		t.Fatalf("unexpected plan id: %s", loadedPlan.PlanID)
	}
}
