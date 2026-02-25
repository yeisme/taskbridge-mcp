package project

import "context"

// Store 项目与拆分建议存储接口。
type Store interface {
	SaveProject(ctx context.Context, project *Project) error
	GetProject(ctx context.Context, projectID string) (*Project, error)
	ListProjects(ctx context.Context, status string) ([]Project, error)

	SavePlan(ctx context.Context, plan *PlanSuggestion) error
	GetPlan(ctx context.Context, projectID, planID string) (*PlanSuggestion, error)
	GetLatestPlan(ctx context.Context, projectID string) (*PlanSuggestion, error)
}
