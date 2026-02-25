package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/internal/storage"
	"github.com/yeisme/taskbridge/internal/storage/filestore"
	"github.com/yeisme/taskbridge/internal/sync"
	"github.com/yeisme/taskbridge/pkg/ui"
)

var (
	listSource   string
	listStatus   string
	listFormat   string
	listQuadrant int
	listPriority int
	listTag      string
	listNames    []string
	listIDs      []string
	listTaskIDs  []string
	listQuery    string
	listAll      bool
	listSyncNow  bool
)

// listCmd 列出任务命令
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "列出任务",
	Long: `列出所有任务，支持按来源、状态、象限等条件筛选。

输出格式:
  - table: 表格格式（默认）
  - json: JSON 格式
  - markdown: Markdown 格式

示例:
  taskbridge list
  taskbridge list --format json
  taskbridge list --source google --status todo
  taskbridge list --quadrant 1
  taskbridge list --all`,
	Run: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVarP(&listSource, "source", "s", "", "按来源筛选（google, microsoft, feishu, ticktick, todoist）")
	listCmd.Flags().StringVarP(&listStatus, "status", "t", "", "按状态筛选（todo, in_progress, completed, cancelled）")
	listCmd.Flags().StringVarP(&listFormat, "format", "f", "table", "输出格式（table, json, markdown）")
	listCmd.Flags().IntVarP(&listQuadrant, "quadrant", "q", 0, "按象限筛选（1-4）")
	listCmd.Flags().IntVarP(&listPriority, "priority", "p", 0, "按优先级筛选（1-4）")
	listCmd.Flags().StringVar(&listTag, "tag", "", "按标签筛选")
	listCmd.Flags().StringArrayVar(&listNames, "list", nil, "按清单名称筛选（可重复指定）")
	listCmd.Flags().StringArrayVar(&listIDs, "list-id", nil, "按清单 ID 筛选（可重复指定）")
	listCmd.Flags().StringArrayVar(&listTaskIDs, "id", nil, "按任务 ID 筛选（可重复指定）")
	listCmd.Flags().StringVar(&listQuery, "query", "", "按关键词/自然语言文本过滤（本地匹配）")
	listCmd.Flags().BoolVarP(&listAll, "all", "a", false, "显示所有任务（包括已完成）")
	listCmd.Flags().BoolVar(&listSyncNow, "sync-now", false, "查询前先同步远程任务到本地")
}

func runList(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	// 先解析 source（支持简写）
	resolvedSource := ""
	if listSource != "" {
		resolvedSource = provider.ResolveProviderName(listSource)
		if !provider.IsValidProvider(resolvedSource) {
			fmt.Printf("❌ 不支持的来源: %s\n", listSource)
			fmt.Println("支持的来源: google (g), microsoft (ms), feishu, ticktick (tick), todoist (todo)")
			os.Exit(1)
		}
	}

	if listSyncNow {
		if err := syncNowForList(ctx, resolvedSource); err != nil {
			fmt.Printf("❌ 同步失败: %v\n", err)
			os.Exit(1)
		}
	}

	// 创建存储
	store, err := filestore.New(cfg.Storage.Path, cfg.Storage.File.Format)
	if err != nil {
		fmt.Printf("❌ 创建存储失败: %v\n", err)
		os.Exit(1)
	}

	// 构建查询
	query := storage.Query{}
	if resolvedSource != "" {
		query.Sources = []model.TaskSource{model.TaskSource(resolvedSource)}
	}
	query.ListIDs = sanitizeStringSlice(listIDs)
	query.ListNames = sanitizeStringSlice(listNames)
	query.TaskIDs = sanitizeStringSlice(listTaskIDs)
	query.QueryText = strings.TrimSpace(listQuery)

	statusChanged := cmd.Flags().Lookup("status") != nil && cmd.Flags().Lookup("status").Changed
	if statusChanged && listStatus != "" {
		for _, status := range splitCSV(listStatus) {
			query.Statuses = append(query.Statuses, model.TaskStatus(status))
		}
	}
	if listQuadrant > 0 && listQuadrant <= 4 {
		query.Quadrants = []model.Quadrant{model.Quadrant(listQuadrant)}
	}
	if listPriority > 0 && listPriority <= 4 {
		query.Priorities = []model.Priority{model.Priority(listPriority)}
	}
	if listTag != "" {
		query.Tags = []string{listTag}
	}
	// 当用户显式设置了 --status 时，严格尊重参数，不应用默认状态过滤。
	if !statusChanged && !listAll {
		// 默认只显示未完成任务
		query.Statuses = []model.TaskStatus{model.StatusTodo, model.StatusInProgress}
	}

	// 查询任务
	tasks, err := store.QueryTasks(ctx, query)
	if err != nil {
		fmt.Printf("❌ 查询任务失败: %v\n", err)
		os.Exit(1)
	}

	// 如果没有任务，显示提示
	if len(tasks) == 0 {
		fmt.Println("📭 没有找到任务")
		if !listSyncNow {
			fmt.Println("💡 可尝试: taskbridge list --sync-now")
		}
		return
	}

	// 按格式输出
	switch listFormat {
	case "json":
		printJSON(tasks)
	case "markdown":
		printMarkdown(tasks)
	default:
		printTable(tasks)
	}
}

// TaskOutput 任务输出格式
type TaskOutput struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Status        string   `json:"status"`
	Quadrant      string   `json:"quadrant,omitempty"`
	Priority      string   `json:"priority,omitempty"`
	DueDate       string   `json:"due_date,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	Source        string   `json:"source"`
	ListID        string   `json:"list_id,omitempty"`
	ListName      string   `json:"list_name,omitempty"`
	Progress      int      `json:"progress,omitempty"`
	PriorityScore int      `json:"priority_score,omitempty"`
}

// toOutput 转换为输出格式
func toOutput(t model.Task) TaskOutput {
	quadrantNames := map[model.Quadrant]string{
		model.QuadrantUrgentImportant:       "Q1-紧急重要",
		model.QuadrantNotUrgentImportant:    "Q2-重要不紧急",
		model.QuadrantUrgentNotImportant:    "Q3-紧急不重要",
		model.QuadrantNotUrgentNotImportant: "Q4-不紧急不重要",
	}

	priorityNames := map[model.Priority]string{
		model.PriorityUrgent: "P0-紧急",
		model.PriorityHigh:   "P1-高",
		model.PriorityMedium: "P2-中",
		model.PriorityLow:    "P3-低",
		model.PriorityNone:   "无",
	}

	var dueDate string
	if t.DueDate != nil {
		dueDate = t.DueDate.Format("2006-01-02")
	}

	return TaskOutput{
		ID:            t.ID,
		Title:         t.Title,
		Status:        string(t.Status),
		Quadrant:      quadrantNames[t.Quadrant],
		Priority:      priorityNames[t.Priority],
		DueDate:       dueDate,
		Tags:          t.Tags,
		Source:        string(t.Source),
		ListID:        t.ListID,
		ListName:      t.ListName,
		Progress:      t.Progress,
		PriorityScore: t.PriorityScore,
	}
}

func printJSON(tasks []model.Task) {
	output := make([]TaskOutput, len(tasks))
	for i, t := range tasks {
		output[i] = toOutput(t)
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Printf("❌ 序列化失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

func printTable(tasks []model.Task) {
	termWidth := detectTerminalWidth()
	// 固定列 + 动态标题/列表列，尽量吃满终端宽度。
	idW := 5
	statusW := 8
	quadrantW := 14
	priorityW := 7
	dueW := 10
	providerW := 10
	gapW := 2
	colCount := 8

	minTitleW := 28
	minListW := 14
	maxTitleW := 160
	maxListW := 60

	fixedW := idW + statusW + quadrantW + priorityW + dueW + providerW + (colCount-1)*gapW
	flexibleW := termWidth - fixedW
	if flexibleW < minTitleW+minListW {
		flexibleW = minTitleW + minListW
	}

	titleW := clampInt((flexibleW*2)/3, minTitleW, maxTitleW)
	listW := flexibleW - titleW
	if listW < minListW {
		deficit := minListW - listW
		listW = minListW
		titleW -= deficit
	}
	if titleW < minTitleW {
		titleW = minTitleW
	}
	listW = flexibleW - titleW
	if listW > maxListW {
		extra := listW - maxListW
		listW = maxListW
		titleW += extra
	}

	table := ui.NewSimpleTable(
		ui.Column{Header: "ID", Width: idW, AlignLeft: true},
		ui.Column{Header: "标题", Width: titleW, AlignLeft: true},
		ui.Column{Header: "状态", Width: statusW, AlignLeft: true},
		ui.Column{Header: "象限", Width: quadrantW, AlignLeft: true},
		ui.Column{Header: "优先级", Width: priorityW, AlignLeft: true},
		ui.Column{Header: "截止日期", Width: dueW, AlignLeft: true},
		ui.Column{Header: "Provider", Width: providerW, AlignLeft: true},
		ui.Column{Header: "List", Width: listW, AlignLeft: true},
	)

	fmt.Println()

	for _, t := range tasks {
		quadrant := quadrantShort(t.Quadrant)
		priority := priorityShort(t.Priority)
		status := statusShort(t.Status)
		dueDate := "-"
		if t.DueDate != nil {
			dueDate = t.DueDate.Format("01-02")
			if t.DueDate.Before(time.Now()) && t.Status != model.StatusCompleted {
				dueDate = "!" + dueDate
			}
		}

		title := truncateDisplay(t.Title, titleW)
		if t.Status == model.StatusCompleted {
			title = "✓ " + title
		}

		listName := "-"
		if t.ListName != "" {
			listName = truncateDisplay(t.ListName, listW)
		}

		table.AddRow(
			truncateDisplay(t.ID, idW),
			title,
			truncateDisplay(status, statusW),
			truncateDisplay(quadrant, quadrantW),
			truncateDisplay(priority, priorityW),
			truncateDisplay(dueDate, dueW),
			truncateDisplay(string(t.Source), providerW),
			listName,
		)
	}

	fmt.Println(table.Render())
	fmt.Println()
	fmt.Printf("共 %d 个任务\n", len(tasks))
}

func syncNowForList(ctx context.Context, source string) error {
	// 指定来源时，只同步该 Provider
	if source != "" {
		engine, err := getSyncEngineForProvider(source)
		if err != nil {
			return err
		}
		_, err = engine.Sync(ctx, sync.Options{
			Direction: sync.DirectionPull,
			Provider:  source,
		})
		return err
	}

	// 未指定来源时，尽量同步已认证 Provider
	providers := []string{"google", "microsoft", "feishu", "ticktick", "todoist"}
	var synced int
	for _, p := range providers {
		engine, err := getSyncEngineForProvider(p)
		if err != nil {
			continue
		}
		if _, err := engine.Sync(ctx, sync.Options{
			Direction: sync.DirectionPull,
			Provider:  p,
		}); err == nil {
			synced++
		}
	}
	if synced == 0 {
		return fmt.Errorf("未找到可同步的已认证 Provider")
	}
	return nil
}

func printMarkdown(tasks []model.Task) {
	fmt.Println("# 📋 任务列表")

	// 按象限分组
	quadrants := map[model.Quadrant][]model.Task{}
	for _, t := range tasks {
		quadrants[t.Quadrant] = append(quadrants[t.Quadrant], t)
	}

	quadrantNames := map[model.Quadrant]string{
		model.QuadrantUrgentImportant:       "🔥 紧急且重要 (Q1)",
		model.QuadrantNotUrgentImportant:    "📋 重要不紧急 (Q2)",
		model.QuadrantUrgentNotImportant:    "⚡ 紧急不重要 (Q3)",
		model.QuadrantNotUrgentNotImportant: "🗑️ 不紧急不重要 (Q4)",
	}

	quadrantOrder := []model.Quadrant{
		model.QuadrantUrgentImportant,
		model.QuadrantNotUrgentImportant,
		model.QuadrantUrgentNotImportant,
		model.QuadrantNotUrgentNotImportant,
	}

	for _, q := range quadrantOrder {
		qtasks := quadrants[q]
		if len(qtasks) > 0 {
			fmt.Printf("## %s\n\n", quadrantNames[q])
			for _, t := range qtasks {
				status := " "
				if t.Status == model.StatusCompleted {
					status = "x"
				}
				due := ""
				if t.DueDate != nil {
					due = fmt.Sprintf(" 📅 %s", t.DueDate.Format("2006-01-02"))
				}
				fmt.Printf("- [%s] %s%s\n", status, t.Title, due)
			}
			fmt.Println()
		}
	}

	// 统计
	fmt.Println("---")
	fmt.Printf("**总计**: %d 个任务\n", len(tasks))
}

func quadrantShort(q model.Quadrant) string {
	switch q {
	case model.QuadrantUrgentImportant:
		return "Q1-紧急重要"
	case model.QuadrantNotUrgentImportant:
		return "Q2-重要不紧急"
	case model.QuadrantUrgentNotImportant:
		return "Q3-紧急不重要"
	case model.QuadrantNotUrgentNotImportant:
		return "Q4-不紧急不重要"
	default:
		return "-"
	}
}

func priorityShort(p model.Priority) string {
	switch p {
	case model.PriorityUrgent:
		return "P0-紧急"
	case model.PriorityHigh:
		return "P1-高"
	case model.PriorityMedium:
		return "P2-中"
	case model.PriorityLow:
		return "P3-低"
	default:
		return "-"
	}
}

func statusShort(s model.TaskStatus) string {
	switch s {
	case model.StatusTodo:
		return "待办"
	case model.StatusInProgress:
		return "进行中"
	case model.StatusCompleted:
		return "已完成"
	case model.StatusCancelled:
		return "已取消"
	case model.StatusDeferred:
		return "已延期"
	default:
		return string(s)
	}
}

func truncate(s string, maxLen int) string {
	return truncateDisplay(s, maxLen)
}

func detectTerminalWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 40 {
		return w
	}

	if v := os.Getenv("COLUMNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 40 {
			return n
		}
	}
	// 保守默认宽度，避免过窄换行。
	return 140
}

func truncateDisplay(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if ui.StringWidth(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return strings.Repeat(".", maxWidth)
	}

	target := maxWidth - 3
	var b strings.Builder
	cur := 0
	for _, r := range s {
		rw := ui.StringWidth(string(r))
		if cur+rw > target {
			break
		}
		b.WriteRune(r)
		cur += rw
	}
	b.WriteString("...")
	return b.String()
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func sanitizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		v := strings.TrimSpace(value)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, v)
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		v := strings.TrimSpace(part)
		if v == "" {
			continue
		}
		result = append(result, v)
	}
	return result
}
