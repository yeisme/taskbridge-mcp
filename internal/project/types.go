package project

import "time"

// ProjectStatus 项目状态。
type ProjectStatus string

const (
	StatusDraft          ProjectStatus = "draft"
	StatusSplitSuggested ProjectStatus = "split_suggested"
	StatusConfirmed      ProjectStatus = "confirmed"
	StatusSynced         ProjectStatus = "synced"
)

// GoalType 目标类型。
type GoalType string

const (
	GoalTypeLearning GoalType = "learning"
	GoalTypeTravel   GoalType = "travel"
	GoalTypeGeneric  GoalType = "generic"
)

// Project 项目实体。
type Project struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	ParentID     string        `json:"parent_id,omitempty"`
	GoalText     string        `json:"goal_text,omitempty"`
	GoalType     GoalType      `json:"goal_type"`
	Status       ProjectStatus `json:"status"`
	ListID       string        `json:"list_id,omitempty"`
	Source       string        `json:"source,omitempty"`
	HorizonDays  int           `json:"horizon_days,omitempty"`
	LatestPlanID string        `json:"latest_plan_id,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// PlanTask 拆分后的任务建议。
type PlanTask struct {
	// ID 是计划内任务 ID（非真实任务 ID），用于稳定映射与追踪。
	ID              string   `json:"id,omitempty"`
	// ParentID 是父计划任务 ID，用于在确认阶段恢复父子关系。
	ParentID        string   `json:"parent_id,omitempty"`
	Title           string   `json:"title"`
	Description     string   `json:"description,omitempty"`
	EstimateMinutes int      `json:"estimate_minutes"`
	DueOffsetDays   int      `json:"due_offset_days"`
	Priority        int      `json:"priority"`
	Quadrant        int      `json:"quadrant"`
	Tags            []string `json:"tags,omitempty"`
	Phase           string   `json:"phase"`
}

// PlanConstraints 拆分约束。
type PlanConstraints struct {
	RequireDeliverable bool `json:"require_deliverable,omitempty"`
	MinEstimateMinutes int  `json:"min_estimate_minutes,omitempty"`
	MaxEstimateMinutes int  `json:"max_estimate_minutes,omitempty"`
	MinTasks           int  `json:"min_tasks,omitempty"`
	MaxTasks           int  `json:"max_tasks,omitempty"`
	MinPracticeTasks   int  `json:"min_practice_tasks,omitempty"`
}

// PlanSuggestion 项目拆分建议。
type PlanSuggestion struct {
	PlanID           string          `json:"plan_id"`
	ProjectID        string          `json:"project_id"`
	GoalText         string          `json:"goal_text,omitempty"`
	GoalType         GoalType        `json:"goal_type"`
	Status           ProjectStatus   `json:"status"`
	Constraints      PlanConstraints `json:"constraints,omitempty"`
	Phases           []string        `json:"phases"`
	TasksPreview     []PlanTask      `json:"tasks_preview"`
	Confidence       float64         `json:"confidence"`
	Warnings         []string        `json:"warnings,omitempty"`
	AIHint           string          `json:"ai_hint,omitempty"`
	ConfirmedTaskIDs []string        `json:"confirmed_task_ids,omitempty"`
	ConfirmedAt      *time.Time      `json:"confirmed_at,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}
