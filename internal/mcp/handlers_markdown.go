package mcp

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yeisme/taskbridge/internal/project"
)

var (
	// 支持无序列表：-/*/+。
	unorderedListItemPattern = regexp.MustCompile(`^[-*+]\s+(.+)$`)
	// 支持有序列表：1. / 1)。
	orderedListItemPattern   = regexp.MustCompile(`^\d+[.)]\s+(.+)$`)
	// 标题归一化时去掉前缀序号，兼容多重前缀（如 "1. 2. xxx"）。
	orderedTitlePrefix       = regexp.MustCompile(`^\d+[.)]\s+`)
)

// markdownNode 表示解析后的 Markdown 列表节点。
type markdownNode struct {
	Title        string
	Indent       int
	SiblingIndex int
	Children     []*markdownNode
}

// markdownParseStats 用于返回解析统计信息。
type markdownParseStats struct {
	TotalNodes   int `json:"total_nodes"`
	LeafTasks    int `json:"leaf_tasks"`
	IgnoredLines int `json:"ignored_lines"`
}

// handleSplitProjectFromMarkdown 将 Markdown 列表任务树转换为 split_suggested 计划。
func (s *Server) handleSplitProjectFromMarkdown(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.projectStore == nil {
		return nil, fmt.Errorf("project storage not available")
	}

	var params struct {
		ProjectID   string `json:"project_id"`
		Markdown    string `json:"markdown"`
		HorizonDays int    `json:"horizon_days"`
		MaxTasks    int    `json:"max_tasks"`
	}
	if err := json.Unmarshal(req.Params.Arguments, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if strings.TrimSpace(params.ProjectID) == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if strings.TrimSpace(params.Markdown) == "" {
		return nil, fmt.Errorf("markdown is required")
	}

	item, err := s.projectStore.GetProject(ctx, strings.TrimSpace(params.ProjectID))
	if err != nil {
		return nil, err
	}

	// 解析输入 Markdown，并应用 horizon/maxTasks 默认值与边界。
	root, stats, warnings := parseMarkdownTaskTree(params.Markdown)
	horizonDays := pickHorizonDays(params.HorizonDays, item.HorizonDays)
	maxTasks := normalizeMarkdownMaxTasks(params.MaxTasks)

	// 仅把叶子节点映射为 tasks_preview；非叶子用于 phase。
	tasks, phases, buildWarnings := buildPlanTasksFromMarkdown(root, item.ID, horizonDays, maxTasks)
	warnings = append(warnings, buildWarnings...)
	stats.LeafTasks = countLeafNodes(root)
	if stats.LeafTasks == 0 {
		return nil, fmt.Errorf("no valid markdown task leaf nodes found")
	}
	if len(tasks) < stats.LeafTasks {
		warnings = append(warnings, fmt.Sprintf("leaf tasks truncated by max_tasks=%d", maxTasks))
	}

	// 按现有计划结构落库，保持与 split_project 输出结构一致。
	suggestion := &project.PlanSuggestion{
		PlanID:       generatePlanID(),
		ProjectID:    item.ID,
		GoalText:     item.GoalText,
		GoalType:     item.GoalType,
		Status:       project.StatusSplitSuggested,
		Constraints:  project.PlanConstraints{},
		Phases:       phases,
		TasksPreview: tasks,
		Confidence:   0.9,
		Warnings:     warnings,
		CreatedAt:    time.Now(),
	}

	if err := s.projectStore.SavePlan(ctx, suggestion); err != nil {
		return nil, fmt.Errorf("failed to save project plan: %w", err)
	}

	item.Status = project.StatusSplitSuggested
	item.LatestPlanID = suggestion.PlanID
	item.HorizonDays = horizonDays
	if err := s.projectStore.SaveProject(ctx, item); err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}

	response := map[string]interface{}{
		"project_id":    item.ID,
		"plan_id":       suggestion.PlanID,
		"status":        suggestion.Status,
		"confidence":    suggestion.Confidence,
		"tasks_preview": suggestion.TasksPreview,
		"phases":        suggestion.Phases,
		"warnings":      suggestion.Warnings,
		"stats":         stats,
	}
	result, _ := toJSON(response)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: result}},
	}, nil
}

// parseMarkdownTaskTree 按缩进优先规则解析 Markdown 列表树。
func parseMarkdownTaskTree(markdown string) (*markdownNode, markdownParseStats, []string) {
	root := &markdownNode{Title: "__root__", Indent: -1}
	stack := []*markdownNode{root}
	stats := markdownParseStats{}
	warnings := make([]string, 0)
	previousListIndent := -1

	lines := strings.Split(markdown, "\n")
	for i, raw := range lines {
		// 约定：tab 统一按 2 个空格处理，避免编辑器差异导致层级漂移。
		line := strings.ReplaceAll(raw, "\t", "  ")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			stats.IgnoredLines++
			continue
		}

		indent := countLeadingSpaces(line)
		title, ok := parseMarkdownListTitle(strings.TrimSpace(line[indent:]))
		if !ok {
			stats.IgnoredLines++
			warnings = append(warnings, fmt.Sprintf("line %d ignored: not a markdown list item", i+1))
			continue
		}

		// 缩进跳级不报错：挂到最近合法父节点，同时记录 warning。
		if previousListIndent >= 0 && indent > previousListIndent+2 {
			warnings = append(warnings, fmt.Sprintf("line %d indent jump from %d to %d; attached to nearest parent", i+1, previousListIndent, indent))
		}
		previousListIndent = indent

		// 维护一个按缩进回退的栈结构来确定父子关系。
		for len(stack) > 1 && indent <= stack[len(stack)-1].Indent {
			stack = stack[:len(stack)-1]
		}

		parent := stack[len(stack)-1]
		node := &markdownNode{
			Title:        title,
			Indent:       indent,
			SiblingIndex: len(parent.Children) + 1,
		}
		parent.Children = append(parent.Children, node)
		stack = append(stack, node)
		stats.TotalNodes++
	}

	return root, stats, warnings
}

// buildPlanTasksFromMarkdown 将树转换为扁平 tasks_preview，并填充默认字段。
// 注意：这里会保留父子关系（ParentID），用于后续按 provider 落地为步骤/子任务。
func buildPlanTasksFromMarkdown(root *markdownNode, projectID string, horizonDays, maxTasks int) ([]project.PlanTask, []string, []string) {
	tasks := make([]project.PlanTask, 0)
	phases := make([]string, 0)
	phaseSeen := map[string]bool{}
	warnings := make([]string, 0)

	for _, top := range root.Children {
		collectMarkdownTasks(top, projectID, []string{top.Title}, []int{top.SiblingIndex}, top.Title, "", &tasks, &phases, phaseSeen)
	}

	if len(tasks) == 0 {
		return tasks, phases, warnings
	}

	if maxTasks <= 0 {
		maxTasks = 200
	}
	if len(tasks) > maxTasks {
		tasks = tasks[:maxTasks]
	}

	// 截止天数按任务索引在 [1, horizonDays] 内均匀分布。
	for i := range tasks {
		tasks[i].DueOffsetDays = distributeDueOffset(i, len(tasks), horizonDays)
	}
	return tasks, phases, warnings
}

// collectMarkdownTasks 深度优先遍历，所有节点都生成任务，子节点通过 ParentID 关联父节点。
func collectMarkdownTasks(node *markdownNode, projectID string, pathTitles []string, pathIndexes []int, phase, parentPlanTaskID string, tasks *[]project.PlanTask, phases *[]string, phaseSeen map[string]bool) {
	phase = strings.TrimSpace(phase)
	if phase == "" {
		phase = "导入任务"
	}
	if !phaseSeen[phase] {
		phaseSeen[phase] = true
		*phases = append(*phases, phase)
	}

	normalizedTitles := make([]string, 0, len(pathTitles))
	for _, title := range pathTitles {
		normalizedTitles = append(normalizedTitles, normalizePathTitle(title))
	}

	// 稳定 ID：project_id + 规范化路径 + 同级序号路径。
	taskID := stablePlanTaskID(projectID, strings.Join(normalizedTitles, "/"), joinIndexPath(pathIndexes))
	*tasks = append(*tasks, project.PlanTask{
		ID:              taskID,
		ParentID:        parentPlanTaskID,
		Title:           node.Title,
		Description:     "",
		EstimateMinutes: 60,
		DueOffsetDays:   1,
		Priority:        2,
		Quadrant:        2,
		Tags:            []string{"markdown_import"},
		Phase:           phase,
	})

	for _, child := range node.Children {
		// 复制 path 切片，避免递归过程中共享底层数组造成污染。
		nextTitles := append(append([]string{}, pathTitles...), child.Title)
		nextIndexes := append(append([]int{}, pathIndexes...), child.SiblingIndex)
		collectMarkdownTasks(child, projectID, nextTitles, nextIndexes, phase, taskID, tasks, phases, phaseSeen)
	}
}

// parseMarkdownListTitle 从单行文本里提取列表项标题。
func parseMarkdownListTitle(content string) (string, bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", false
	}

	if match := unorderedListItemPattern.FindStringSubmatch(content); len(match) == 2 {
		title := normalizeMarkdownTitle(match[1])
		return title, title != ""
	}
	if match := orderedListItemPattern.FindStringSubmatch(content); len(match) == 2 {
		title := normalizeMarkdownTitle(match[1])
		return title, title != ""
	}
	return "", false
}

// normalizeMarkdownTitle 归一化标题，去掉序号前缀并清理首尾空白。
func normalizeMarkdownTitle(raw string) string {
	title := strings.TrimSpace(raw)
	for {
		next := strings.TrimSpace(orderedTitlePrefix.ReplaceAllString(title, ""))
		if next == title {
			break
		}
		title = next
	}
	return title
}

// normalizePathTitle 用于稳定 ID 的路径归一化：小写 + 压缩空白。
func normalizePathTitle(title string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(title))), " ")
}

// stablePlanTaskID 生成可复现的计划任务 ID（ptk_ + sha1 前 10 位）。
func stablePlanTaskID(projectID, normalizedPathTitles, indexPath string) string {
	base := projectID + "|" + normalizedPathTitles + "|" + indexPath
	sum := sha1.Sum([]byte(base))
	return "ptk_" + hex.EncodeToString(sum[:])[:10]
}

// joinIndexPath 把路径上的 sibling 序号拼接为稳定字符串（如 1.2.1）。
func joinIndexPath(path []int) string {
	parts := make([]string, 0, len(path))
	for _, p := range path {
		parts = append(parts, strconv.Itoa(p))
	}
	return strings.Join(parts, ".")
}

// countLeadingSpaces 计算前导空格，用于缩进层级判断。
func countLeadingSpaces(line string) int {
	count := 0
	for _, ch := range line {
		if ch != ' ' {
			break
		}
		count++
	}
	return count
}

// countLeafNodes 统计树中叶子节点数，用于返回 stats 与截断提示。
func countLeafNodes(root *markdownNode) int {
	total := 0
	var walk func(node *markdownNode)
	walk = func(node *markdownNode) {
		if len(node.Children) == 0 {
			total++
			return
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, child := range root.Children {
		walk(child)
	}
	return total
}

// distributeDueOffset 把任务均匀映射到 [1, horizonDays] 的截止偏移天数。
func distributeDueOffset(index, total, horizonDays int) int {
	if horizonDays <= 0 {
		horizonDays = 14
	}
	if total <= 1 {
		return 1
	}
	span := float64(maxInt(1, horizonDays-1))
	ratio := float64(index) / float64(total-1)
	offset := 1 + int(math.Round(span*ratio))
	if offset < 1 {
		return 1
	}
	if offset > horizonDays {
		return horizonDays
	}
	return offset
}

// normalizeMarkdownMaxTasks 处理 max_tasks 默认值与硬上限。
func normalizeMarkdownMaxTasks(v int) int {
	if v <= 0 {
		return 200
	}
	if v > 500 {
		return 500
	}
	return v
}
