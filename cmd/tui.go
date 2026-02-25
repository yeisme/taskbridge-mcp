package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/internal/provider/google"
	"github.com/yeisme/taskbridge/internal/storage"
	"github.com/yeisme/taskbridge/internal/storage/filestore"
)

// tuiCmd TUI 命令
var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "交互式终端界面",
	Long: `启动交互式终端界面（TUI）查看 TaskBridge 任务。

使用键盘导航:
  ↑/k  上移      ↓/j  下移
  ←/h  左侧标签  →/l  右侧标签
  q    退出      ?    显示帮助
  r    刷新      /    搜索
  1-4  按象限筛选 a    显示全部
  s    排序切换`,
	Run: runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

// ViewType 视图类型
type ViewType int

const (
	// ViewTasks 任务视图
	ViewTasks ViewType = iota
	// ViewQuadrant 象限视图
	ViewQuadrant
	// ViewProjects 项目视图
	ViewProjects
	// ViewProviders 提供者视图
	ViewProviders
	// ViewAuth 认证视图
	ViewAuth
	// ViewCount 视图计数
	ViewCount
)

// SortType 排序类型
type SortType int

const (
	// SortByDueDate 按截止日期排序
	SortByDueDate SortType = iota
	// SortByPriority 按优先级排序
	SortByPriority
	// SortByCreated 按创建时间排序
	SortByCreated
	// SortByTitle 按标题排序
	SortByTitle
	// SortCount 排序类型计数
	SortCount
)

// InputMode 输入模式
type InputMode int

const (
	// ModeNormal 普通模式
	ModeNormal InputMode = iota
	// ModeSearch 搜索模式
	ModeSearch
)

// 样式定义
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7C3AED")).
			Padding(0, 2).
			MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true)

	taskStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#374151"))

	completedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Strikethrough(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			MarginTop(1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#374151")).
			Padding(0, 1)

	tabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Padding(0, 2)

	activeTabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7C3AED")).
			Padding(0, 2).
			Bold(true)

	quadrantStyles = map[int]lipgloss.Style{
		1: lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")), // Q1 - 红色
		2: lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")), // Q2 - 蓝色
		3: lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")), // Q3 - 橙色
		4: lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")), // Q4 - 灰色
	}

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#374151")).
			Padding(0, 1)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444"))

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true)

	dueDateStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	overdueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Bold(true)

	subtaskStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF"))
)

// Model TUI 模型
type Model struct {
	// 数据
	tasks      []model.Task
	taskLists  []model.TaskList
	providers  map[model.TaskSource]provider.Provider
	store      storage.Storage
	googleProv provider.Provider

	// UI 状态
	currentView ViewType
	filtered    []model.Task
	selected    int
	quadrant    int // 0 = all, 1-4 = specific
	width       int
	height      int
	loading     bool
	err         error
	showHelp    bool
	sortBy      SortType
	inputMode   InputMode
	inputBuffer string
}

// 初始化模型
func initialModel() Model {
	return Model{
		loading:     true,
		quadrant:    0,
		currentView: ViewTasks,
		sortBy:      SortByDueDate,
		inputMode:   ModeNormal,
	}
}

// 消息类型
type loadMsg struct {
	tasks      []model.Task
	taskLists  []model.TaskList
	providers  map[model.TaskSource]provider.Provider
	store      storage.Storage
	googleProv provider.Provider
	err        error
}

// 加载数据
func loadData() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		store, err := filestore.New(cfg.Storage.Path, cfg.Storage.File.Format)
		if err != nil {
			return loadMsg{err: err}
		}

		tasks, err := store.ListTasks(ctx, storage.ListOptions{})
		if err != nil {
			return loadMsg{err: err}
		}

		taskLists, err := store.ListTaskLists(ctx)
		if err != nil {
			taskLists = []model.TaskList{}
		}

		// 获取已注册的 providers
		providers := provider.GlobalRegistry.GetAll()

		// 尝试初始化 Google Provider
		var googleProv provider.Provider
		gp, err := google.NewProviderFromHome()
		if err == nil && gp.IsAuthenticated() {
			googleProv = gp
		}

		return loadMsg{tasks: tasks, taskLists: taskLists, providers: providers, store: store, googleProv: googleProv}
	}
}

// Init 初始化
func (m Model) Init() tea.Cmd {
	return loadData()
}

// Update 更新
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// 搜索模式
		if m.inputMode == ModeSearch {
			return m.handleSearchInput(msg)
		}
		// 正常模式
		return m.handleNormalInput(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case loadMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.tasks = msg.tasks
			m.taskLists = msg.taskLists
			m.providers = msg.providers
			m.store = msg.store
			m.googleProv = msg.googleProv
			m.applyFilter()
		}
	}

	return m, nil
}

// handleSearchInput 处理搜索输入
func (m Model) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inputMode = ModeNormal
		m.inputBuffer = ""
		m.applyFilter()
	case "enter":
		m.inputMode = ModeNormal
		m.applyFilter()
	case "backspace":
		if len(m.inputBuffer) > 0 {
			m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
			m.applyFilter()
		}
	default:
		if len(msg.String()) == 1 {
			m.inputBuffer += msg.String()
			m.applyFilter()
		}
	}
	return m, nil
}

// handleNormalInput 处理正常输入
func (m Model) handleNormalInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.showHelp = !m.showHelp
	case "r":
		m.loading = true
		return m, loadData()
	case "/":
		m.inputMode = ModeSearch
		m.inputBuffer = ""
	case "s":
		// 切换排序
		m.sortBy = (m.sortBy + 1) % SortCount
		m.applyFilter()
	case "a":
		m.quadrant = 0
		m.applyFilter()
		m.selected = 0
	case "1":
		m.quadrant = 1
		m.applyFilter()
		m.selected = 0
	case "2":
		m.quadrant = 2
		m.applyFilter()
		m.selected = 0
	case "3":
		m.quadrant = 3
		m.applyFilter()
		m.selected = 0
	case "4":
		m.quadrant = 4
		m.applyFilter()
		m.selected = 0
	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}
	case "down", "j":
		maxItems := m.getMaxItems()
		if m.selected < maxItems-1 {
			m.selected++
		}
	case "left", "h":
		if m.currentView > 0 {
			m.currentView--
			m.selected = 0
		}
	case "right", "l":
		if m.currentView < ViewCount-1 {
			m.currentView++
			m.selected = 0
		}
	case "tab":
		m.currentView = (m.currentView + 1) % ViewCount
		m.selected = 0
	}
	return m, nil
}

// getMaxItems 获取当前视图的最大项目数
func (m *Model) getMaxItems() int {
	switch m.currentView {
	case ViewTasks, ViewQuadrant:
		return len(m.filtered)
	case ViewProviders:
		return len(m.providers)
	case ViewProjects:
		return len(m.taskLists)
	default:
		return 0
	}
}

// getSortName 获取排序名称
func (m *Model) getSortName() string {
	switch m.sortBy {
	case SortByDueDate:
		return "截止日期"
	case SortByPriority:
		return "优先级"
	case SortByCreated:
		return "创建时间"
	case SortByTitle:
		return "标题"
	default:
		return "未知"
	}
}

// applyFilter 应用筛选和排序
func (m *Model) applyFilter() {
	m.filtered = nil

	for _, t := range m.tasks {
		// 象限筛选
		if m.quadrant > 0 && int(t.Quadrant) != m.quadrant {
			continue
		}

		// 搜索筛选
		if m.inputMode == ModeSearch && m.inputBuffer != "" {
			if !strings.Contains(strings.ToLower(t.Title), strings.ToLower(m.inputBuffer)) {
				continue
			}
		}

		m.filtered = append(m.filtered, t)
	}

	// 排序
	sort.Slice(m.filtered, func(i, j int) bool {
		switch m.sortBy {
		case SortByDueDate:
			// 没有截止日期的排在后面
			if m.filtered[i].DueDate == nil && m.filtered[j].DueDate == nil {
				return false
			}
			if m.filtered[i].DueDate == nil {
				return false
			}
			if m.filtered[j].DueDate == nil {
				return true
			}
			return m.filtered[i].DueDate.Before(*m.filtered[j].DueDate)
		case SortByPriority:
			return m.filtered[i].Priority < m.filtered[j].Priority
		case SortByCreated:
			return m.filtered[i].CreatedAt.After(m.filtered[j].CreatedAt)
		case SortByTitle:
			return m.filtered[i].Title < m.filtered[j].Title
		default:
			return false
		}
	})
}

// View 渲染
func (m Model) View() string {
	if m.loading {
		return "\n  ⏳ 加载中...\n"
	}

	if m.err != nil {
		return fmt.Sprintf("\n  ❌ 加载失败: %v\n", m.err)
	}

	var b strings.Builder

	// 渲染标签栏
	b.WriteString(m.renderTabs())
	b.WriteString("\n")

	// 渲染搜索输入
	if m.inputMode == ModeSearch {
		b.WriteString(m.renderSearchInput())
		b.WriteString("\n")
	}

	// 渲染当前视图内容
	switch m.currentView {
	case ViewTasks:
		b.WriteString(m.renderTasksView())
	case ViewQuadrant:
		b.WriteString(m.renderQuadrantView())
	case ViewProjects:
		b.WriteString(m.renderProjectsView())
	case ViewProviders:
		b.WriteString(m.renderProvidersView())
	case ViewAuth:
		b.WriteString(m.renderAuthView())
	}

	// 帮助信息
	if m.showHelp {
		b.WriteString("\n")
		b.WriteString(helpStyle.Render(`
快捷键:
  ↑/k  上移      ↓/j  下移
  ←/h  左标签    →/l  右标签
  Tab  切换视图  q    退出
  1-4  按象限    a    全部
  /    搜索      r    刷新
  s    切换排序  ?    帮助
`))
	}

	// 状态栏
	b.WriteString("\n")
	status := fmt.Sprintf(" %s | 排序: %s | 按 ? 查看帮助 | q 退出", m.getViewName(), m.getSortName())
	b.WriteString(statusBarStyle.Render(status))

	return b.String()
}

// renderTabs 渲染标签栏
func (m Model) renderTabs() string {
	tabs := []string{"任务", "四象限", "项目", "Provider", "认证"}
	var renderedTabs []string

	for i, tab := range tabs {
		if i == int(m.currentView) {
			renderedTabs = append(renderedTabs, activeTabStyle.Render(tab))
		} else {
			renderedTabs = append(renderedTabs, tabStyle.Render(tab))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
}

// getViewName 获取当前视图名称
func (m Model) getViewName() string {
	names := []string{"任务列表", "四象限视图", "项目列表", "Provider 信息", "认证状态"}
	return names[m.currentView]
}

// renderSearchInput 渲染搜索输入
func (m Model) renderSearchInput() string {
	return inputStyle.Render(fmt.Sprintf("🔍 搜索: %s_", m.inputBuffer))
}

// renderTasksView 渲染任务视图
func (m Model) renderTasksView() string {
	var b strings.Builder

	title := "📋 任务列表"
	if m.quadrant > 0 {
		title = fmt.Sprintf("📋 象限 Q%d 任务", m.quadrant)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")

	if len(m.filtered) == 0 {
		b.WriteString("  📭 没有找到任务\n")
		return b.String()
	}

	for i, t := range m.filtered {
		var prefix string
		if i == m.selected {
			prefix = "▶ "
		} else {
			prefix = "  "
		}

		var taskLine string
		if t.Status == model.StatusCompleted {
			taskLine = completedStyle.Render(prefix + "✓ " + t.Title)
		} else {
			priorityMark := t.Priority.Emoji()
			overdueMark := ""
			dueDateStr := ""

			// 显示截止日期
			if t.DueDate != nil {
				dueDateStr = dueDateStyle.Render(fmt.Sprintf(" [%s]", t.DueDate.Format("01-02")))
				if t.DueDate.Before(time.Now()) {
					overdueMark = overdueStyle.Render(" ⚠️逾期")
				}
			}

			// 显示子任务数量
			subtaskStr := ""
			if len(t.SubtaskIDs) > 0 {
				subtaskStr = subtaskStyle.Render(fmt.Sprintf(" [%d子任务]", len(t.SubtaskIDs)))
			}

			if i == m.selected {
				taskLine = selectedStyle.Render(prefix+priorityMark+" "+t.Title) + dueDateStr + overdueMark + subtaskStr
			} else {
				taskLine = taskStyle.Render(prefix+priorityMark+" "+t.Title) + dueDateStr + overdueMark + subtaskStr
			}
		}
		b.WriteString(taskLine + "\n")
	}

	return b.String()
}

// renderQuadrantView 渲染四象限视图
func (m Model) renderQuadrantView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("📊 四象限分析"))
	b.WriteString("\n\n")

	// 统计各象限任务
	q1, q2, q3, q4 := 0, 0, 0, 0
	for _, t := range m.tasks {
		switch t.Quadrant {
		case model.QuadrantUrgentImportant:
			q1++
		case model.QuadrantNotUrgentImportant:
			q2++
		case model.QuadrantUrgentNotImportant:
			q3++
		case model.QuadrantNotUrgentNotImportant:
			q4++
		}
	}

	// Q1
	b.WriteString(quadrantStyles[1].Render("🔥 Q1 - 紧急且重要 (立即做)"))
	fmt.Fprintf(&b, " [%d个任务]\n", q1)
	b.WriteString(m.renderQuadrantTasks(model.QuadrantUrgentImportant))
	b.WriteString("\n")

	// Q2
	b.WriteString(quadrantStyles[2].Render("📋 Q2 - 重要不紧急 (计划做)"))
	fmt.Fprintf(&b, " [%d个任务]\n", q2)
	b.WriteString(m.renderQuadrantTasks(model.QuadrantNotUrgentImportant))
	b.WriteString("\n")

	// Q3
	b.WriteString(quadrantStyles[3].Render("⚡ Q3 - 紧急不重要 (授权做)"))
	fmt.Fprintf(&b, " [%d个任务]\n", q3)
	b.WriteString(m.renderQuadrantTasks(model.QuadrantUrgentNotImportant))
	b.WriteString("\n")

	// Q4
	b.WriteString(quadrantStyles[4].Render("🗑️ Q4 - 不紧急不重要 (删除/延后)"))
	fmt.Fprintf(&b, " [%d个任务]\n", q4)
	b.WriteString(m.renderQuadrantTasks(model.QuadrantNotUrgentNotImportant))

	return b.String()
}

// renderQuadrantTasks 渲染象限任务
func (m Model) renderQuadrantTasks(q model.Quadrant) string {
	var b strings.Builder
	count := 0
	for _, t := range m.tasks {
		if t.Quadrant == q && count < 5 {
			prefix := "  • "
			dueStr := ""
			if t.DueDate != nil {
				dueStr = dueDateStyle.Render(fmt.Sprintf(" [%s]", t.DueDate.Format("01-02")))
			}
			if t.Status == model.StatusCompleted {
				b.WriteString(completedStyle.Render(prefix+"✓ "+t.Title) + dueStr + "\n")
			} else {
				b.WriteString(taskStyle.Render(prefix+t.Title) + dueStr + "\n")
			}
			count++
		}
	}
	if count == 0 {
		b.WriteString("  (暂无任务)\n")
	} else {
		remaining := 0
		for _, t := range m.tasks {
			if t.Quadrant == q {
				remaining++
			}
		}
		if remaining > 5 {
			fmt.Fprintf(&b, "  ... 还有 %d 个任务\n", remaining-5)
		}
	}
	return b.String()
}

// renderProjectsView 渲染项目视图
func (m Model) renderProjectsView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("📁 项目列表 (任务列表)"))
	b.WriteString("\n\n")

	if len(m.taskLists) == 0 {
		b.WriteString(infoStyle.Render("没有找到项目\n"))
		return b.String()
	}

	for i, list := range m.taskLists {
		prefix := "  "
		if i == m.selected {
			prefix = "▶ "
		}

		// 统计该列表下的任务数
		taskCount := 0
		for _, t := range m.tasks {
			if t.ListID == list.ID || t.ListName == list.Name {
				taskCount++
			}
		}

		if i == m.selected {
			b.WriteString(selectedStyle.Render(fmt.Sprintf("%s📁 %s", prefix, list.Name)))
		} else {
			fmt.Fprintf(&b, "%s📁 %s", prefix, list.Name)
		}
		b.WriteString(infoStyle.Render(fmt.Sprintf(" (%d个任务)\n", taskCount)))
	}

	return b.String()
}

// renderProvidersView 渲染 Provider 视图
func (m Model) renderProvidersView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("🔌 Provider 信息"))
	b.WriteString("\n\n")

	if len(m.providers) == 0 {
		b.WriteString(infoStyle.Render("没有注册的 Provider\n"))
		return b.String()
	}

	i := 0
	for name, p := range m.providers {
		prefix := "  "
		if i == m.selected {
			prefix = "▶ "
		}

		caps := p.Capabilities()
		status := "❌ 未认证"
		if p.IsAuthenticated() {
			status = "✅ 已认证"
		}

		fmt.Fprintf(&b, "%s%s - %s\n", prefix, name, status)
		fmt.Fprintf(&b, "    子任务: %v | 标签: %v | 优先级: %v\n",
			boolToCheck(caps.SupportsSubtasks),
			boolToCheck(caps.SupportsTags),
			boolToCheck(caps.SupportsPriority))
		fmt.Fprintf(&b, "    截止日期: %v | 提醒: %v | 进度: %v\n",
			boolToCheck(caps.SupportsDueDate),
			boolToCheck(caps.SupportsReminder),
			boolToCheck(caps.SupportsProgress))
		b.WriteString("\n")
		i++
	}

	return b.String()
}

// renderAuthView 渲染认证视图
func (m Model) renderAuthView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("🔐 认证状态"))
	b.WriteString("\n\n")

	if len(m.providers) == 0 {
		b.WriteString(infoStyle.Render("没有注册的 Provider\n"))
		return b.String()
	}

	i := 0
	for name, p := range m.providers {
		prefix := "  "
		if i == m.selected {
			prefix = "▶ "
		}

		if p.IsAuthenticated() {
			fmt.Fprintf(&b, "%s%s: %s\n", prefix, name, successStyle.Render("✅ 已认证"))
		} else {
			fmt.Fprintf(&b, "%s%s: %s\n", prefix, name, errorStyle.Render("❌ 未认证"))
			fmt.Fprintf(&b, "    运行 taskbridge auth %s 进行认证\n", name)
		}
		b.WriteString("\n")
		i++
	}

	b.WriteString(infoStyle.Render("提示: 使用 taskbridge auth <provider> 命令进行认证\n"))

	return b.String()
}

// boolToCheck 布尔值转勾选符号
func boolToCheck(b bool) string {
	if b {
		return "✓"
	}
	return "✗"
}

func runTUI(cmd *cobra.Command, args []string) {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("启动 TUI 失败: %v\n", err)
		os.Exit(1)
	}
}
