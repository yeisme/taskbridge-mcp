package project

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
)

type persistData struct {
	Projects []*Project        `json:"projects"`
	Plans    []*PlanSuggestion `json:"plans"`
}

// FileStore 项目文件存储。
type FileStore struct {
	mu       sync.RWMutex
	filePath string
	projects map[string]*Project
	plans    map[string]*PlanSuggestion
}

// NewFileStore 创建项目存储。
func NewFileStore(basePath string) (*FileStore, error) {
	store := &FileStore{
		filePath: filepath.Join(basePath, "projects.json"),
		projects: make(map[string]*Project),
		plans:    make(map[string]*PlanSuggestion),
	}

	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create project store dir: %w", err)
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FileStore) SaveProject(_ context.Context, project *Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if project.CreatedAt.IsZero() {
		project.CreatedAt = now
	}
	project.UpdatedAt = now
	s.projects[project.ID] = cloneProject(project)
	return s.save()
}

func (s *FileStore) GetProject(_ context.Context, projectID string) (*Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.projects[projectID]
	if !ok {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}
	return cloneProject(p), nil
}

func (s *FileStore) ListProjects(_ context.Context, status string) ([]Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	normalizedStatus := strings.TrimSpace(strings.ToLower(status))
	items := make([]Project, 0, len(s.projects))
	for _, p := range s.projects {
		if normalizedStatus != "" && strings.ToLower(string(p.Status)) != normalizedStatus {
			continue
		}
		items = append(items, *cloneProject(p))
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items, nil
}

func (s *FileStore) SavePlan(_ context.Context, plan *PlanSuggestion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = time.Now()
	}
	s.plans[planKey(plan.ProjectID, plan.PlanID)] = clonePlan(plan)
	return s.save()
}

func (s *FileStore) GetPlan(_ context.Context, projectID, planID string) (*PlanSuggestion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.plans[planKey(projectID, planID)]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s/%s", projectID, planID)
	}
	return clonePlan(p), nil
}

func (s *FileStore) GetLatestPlan(_ context.Context, projectID string) (*PlanSuggestion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	candidates := make([]*PlanSuggestion, 0)
	for _, p := range s.plans {
		if p.ProjectID == projectID {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("plan not found for project: %s", projectID)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].PlanID < candidates[j].PlanID
		}
		return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
	})
	return clonePlan(candidates[len(candidates)-1]), nil
}

func (s *FileStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read project store: %w", err)
	}

	var payload persistData
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal project store: %w", err)
	}

	for _, p := range payload.Projects {
		s.projects[p.ID] = p
	}
	for _, p := range payload.Plans {
		s.plans[planKey(p.ProjectID, p.PlanID)] = p
	}
	return nil
}

func (s *FileStore) save() error {
	projects := make([]*Project, 0, len(s.projects))
	for _, p := range s.projects {
		projects = append(projects, p)
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].CreatedAt.Before(projects[j].CreatedAt)
	})

	plans := make([]*PlanSuggestion, 0, len(s.plans))
	for _, p := range s.plans {
		plans = append(plans, p)
	}
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].ProjectID == plans[j].ProjectID {
			if plans[i].CreatedAt.Equal(plans[j].CreatedAt) {
				return plans[i].PlanID < plans[j].PlanID
			}
			return plans[i].CreatedAt.Before(plans[j].CreatedAt)
		}
		return plans[i].ProjectID < plans[j].ProjectID
	})

	payload := persistData{Projects: projects, Plans: plans}
	bytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project store: %w", err)
	}
	if err := os.WriteFile(s.filePath, bytes, 0o644); err != nil {
		return fmt.Errorf("failed to write project store: %w", err)
	}
	return nil
}

func planKey(projectID, planID string) string {
	return projectID + "::" + planID
}

func cloneProject(p *Project) *Project {
	cp := *p
	return &cp
}

func clonePlan(p *PlanSuggestion) *PlanSuggestion {
	cp := *p
	cp.Phases = append([]string(nil), p.Phases...)
	cp.TasksPreview = append([]PlanTask(nil), p.TasksPreview...)
	cp.Warnings = append([]string(nil), p.Warnings...)
	cp.ConfirmedTaskIDs = append([]string(nil), p.ConfirmedTaskIDs...)
	return &cp
}
