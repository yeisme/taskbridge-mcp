package projectplanner

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yeisme/taskbridge/internal/project"
)

// DecomposeInput 拆分输入。
type DecomposeInput struct {
	ProjectID   string
	ProjectName string
	GoalText    string
	GoalType    project.GoalType
	HorizonDays int
	MaxTasks    int
	AIHint      string
	Constraints project.PlanConstraints
}

// Decompose 按目标拆分任务建议。
func Decompose(input DecomposeInput) *project.PlanSuggestion {
	goalText := strings.TrimSpace(input.GoalText)
	if goalText == "" {
		goalText = input.ProjectName
	}

	goalType := input.GoalType
	if goalType == "" {
		goalType = DetectGoalType(goalText)
	}

	horizonDays := input.HorizonDays
	if horizonDays == 0 {
		horizonDays = 14
	}
	if horizonDays < 7 {
		horizonDays = 7
	}
	if horizonDays > 14 {
		horizonDays = 14
	}

	constraints := normalizeConstraints(input.Constraints)
	maxTasks := effectiveMaxTasks(input.MaxTasks, constraints.MaxTasks)

	subject := inferSubject(goalText, input.ProjectName)
	templates := templatesFor(goalType, subject)

	phases := make([]string, 0, len(templates))
	tasks := make([]project.PlanTask, 0, maxTasks)
	warnings := make([]string, 0)

	for _, phase := range templates {
		phases = append(phases, phase.Name)
		for _, task := range phase.Tasks {
			task.Phase = phase.Name
			task.DueOffsetDays = scaleOffset(task.DueOffsetDays, horizonDays)
			task.EstimateMinutes = clampInt(task.EstimateMinutes, constraints.MinEstimateMinutes, constraints.MaxEstimateMinutes)
			if constraints.RequireDeliverable {
				task.Description = ensureDeliverable(task.Title, task.Description)
			}
			tasks = append(tasks, task)
		}
	}

	tasks = ensureMinPracticeTasks(tasks, constraints.MinPracticeTasks, horizonDays, constraints.RequireDeliverable)
	tasks = ensureMinTasks(tasks, constraints.MinTasks, horizonDays, constraints.RequireDeliverable)

	if len(tasks) > maxTasks {
		tasks = tasks[:maxTasks]
		warnings = append(warnings, "任务已按 max_tasks 截断")
	}

	if len(tasks) < constraints.MinTasks {
		warnings = append(warnings, "可生成任务数量低于 min_tasks，已尽可能补齐")
	}

	if goalType == project.GoalTypeGeneric {
		warnings = append(warnings, "未命中学习/旅行关键词，使用 generic 模板")
	}

	confidence := 0.68
	switch goalType {
	case project.GoalTypeLearning, project.GoalTypeTravel:
		confidence = 0.86
	}

	return &project.PlanSuggestion{
		ProjectID:    input.ProjectID,
		GoalText:     goalText,
		GoalType:     goalType,
		Status:       project.StatusSplitSuggested,
		Constraints:  constraints,
		Phases:       phases,
		TasksPreview: tasks,
		Confidence:   confidence,
		Warnings:     warnings,
		AIHint:       strings.TrimSpace(input.AIHint),
		CreatedAt:    time.Now(),
	}
}

func normalizeConstraints(in project.PlanConstraints) project.PlanConstraints {
	out := in
	if out.MinEstimateMinutes == 0 {
		out.MinEstimateMinutes = 30
	}
	if out.MaxEstimateMinutes == 0 {
		out.MaxEstimateMinutes = 180
	}
	if out.MinEstimateMinutes < 15 {
		out.MinEstimateMinutes = 15
	}
	if out.MaxEstimateMinutes < out.MinEstimateMinutes {
		out.MaxEstimateMinutes = out.MinEstimateMinutes
	}
	if out.MinTasks == 0 {
		out.MinTasks = 6
	}
	if out.MaxTasks == 0 {
		out.MaxTasks = 18
	}
	if out.MinTasks < 1 {
		out.MinTasks = 1
	}
	if out.MaxTasks < out.MinTasks {
		out.MaxTasks = out.MinTasks
	}
	if out.MaxTasks > 30 {
		out.MaxTasks = 30
	}
	if out.MinPracticeTasks < 0 {
		out.MinPracticeTasks = 0
	}
	return out
}

func effectiveMaxTasks(inputMaxTasks, constraintMaxTasks int) int {
	maxTasks := inputMaxTasks
	if maxTasks == 0 {
		maxTasks = 12
	}
	if maxTasks < 6 {
		maxTasks = 6
	}
	if maxTasks > 18 {
		maxTasks = 18
	}
	if constraintMaxTasks > 0 && constraintMaxTasks < maxTasks {
		return constraintMaxTasks
	}
	return maxTasks
}

func ensureDeliverable(title, description string) string {
	desc := strings.TrimSpace(description)
	if strings.Contains(desc, "产出：") {
		return desc
	}
	if desc == "" {
		desc = "完成任务并记录结果"
	}
	return desc + "；产出：" + title + " 的结果记录"
}

func ensureMinPracticeTasks(tasks []project.PlanTask, minPracticeTasks, horizonDays int, requireDeliverable bool) []project.PlanTask {
	if minPracticeTasks <= 0 {
		return tasks
	}
	count := 0
	for _, t := range tasks {
		if hasTag(t.Tags, "practice") {
			count++
		}
	}
	if count >= minPracticeTasks {
		return tasks
	}

	nextOffset := 1
	if len(tasks) > 0 {
		nextOffset = minInt(horizonDays, tasks[len(tasks)-1].DueOffsetDays+1)
	}
	for i := count; i < minPracticeTasks; i++ {
		task := project.PlanTask{
			Title:           "补充实战练习 " + strconv.Itoa(i+1),
			Description:     "围绕当前目标做一次可复现实操",
			EstimateMinutes: 120,
			DueOffsetDays:   nextOffset,
			Priority:        3,
			Quadrant:        2,
			Tags:            []string{"practice"},
			Phase:           "基础实践",
		}
		if requireDeliverable {
			task.Description = ensureDeliverable(task.Title, task.Description)
		}
		tasks = append(tasks, task)
		nextOffset = minInt(horizonDays, nextOffset+1)
	}
	return tasks
}

func ensureMinTasks(tasks []project.PlanTask, minTasks, horizonDays int, requireDeliverable bool) []project.PlanTask {
	if len(tasks) >= minTasks {
		return tasks
	}
	nextOffset := 1
	if len(tasks) > 0 {
		nextOffset = minInt(horizonDays, tasks[len(tasks)-1].DueOffsetDays+1)
	}
	for i := len(tasks); i < minTasks; i++ {
		task := project.PlanTask{
			Title:           "执行检查点 " + strconv.Itoa(i+1),
			Description:     "补充任务，确保计划可执行且可追踪",
			EstimateMinutes: 60,
			DueOffsetDays:   nextOffset,
			Priority:        2,
			Quadrant:        2,
			Tags:            []string{"checkpoint"},
			Phase:           "执行检查点",
		}
		if requireDeliverable {
			task.Description = ensureDeliverable(task.Title, task.Description)
		}
		tasks = append(tasks, task)
		nextOffset = minInt(horizonDays, nextOffset+1)
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].DueOffsetDays < tasks[j].DueOffsetDays
	})
	return tasks
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func scaleOffset(offset, horizonDays int) int {
	if offset <= 0 {
		return 1
	}
	ratio := float64(horizonDays) / 14.0
	scaled := int(math.Round(float64(offset) * ratio))
	if scaled < 1 {
		return 1
	}
	if scaled > horizonDays {
		return horizonDays
	}
	return scaled
}

func inferSubject(goalText, projectName string) string {
	clean := strings.TrimSpace(goalText)
	if clean == "" {
		return strings.TrimSpace(projectName)
	}

	replacements := []string{
		"我希望", "我想", "希望", "想要", "学习", "了解", "熟悉", "精通", "掌握", "去", "旅游", "出行", "行程", "攻略",
	}
	for _, r := range replacements {
		clean = strings.TrimSpace(strings.ReplaceAll(clean, r, ""))
	}
	if clean == "" {
		return strings.TrimSpace(projectName)
	}
	return clean
}
