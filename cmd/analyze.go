package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/storage"
	"github.com/yeisme/taskbridge/internal/storage/filestore"
)

var (
	analyzeFormat string
)

// analyzeCmd 分析命令
var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "任务分析",
	Long: `分析任务数据，提供四象限视图、优先级分析、时间分布等报告。

子命令:
  quadrant   四象限分析（艾森豪威尔矩阵）
  priority   优先级分析
  time       时间分布分析
  trend      趋势分析
  report     生成综合报告

示例:
  taskbridge analyze quadrant
  taskbridge analyze priority --format json
  taskbridge analyze report`,
}

// analyzeQuadrantCmd 四象限分析
var analyzeQuadrantCmd = &cobra.Command{
	Use:   "quadrant",
	Short: "四象限分析（艾森豪威尔矩阵）",
	Long: `按照艾森豪威尔矩阵分析任务分布：

  Q1 紧急且重要   - 立即处理
  Q2 重要不紧急   - 计划安排
  Q3 紧急不重要   - 委托他人
  Q4 不紧急不重要 - 考虑删除`,
	Run: runAnalyzeQuadrant,
}

// analyzePriorityCmd 优先级分析
var analyzePriorityCmd = &cobra.Command{
	Use:   "priority",
	Short: "优先级分析",
	Long:  `按优先级分布分析任务`,
	Run:   runAnalyzePriority,
}

// analyzeTimeCmd 时间分析
var analyzeTimeCmd = &cobra.Command{
	Use:   "time",
	Short: "时间分布分析",
	Long:  `按截止日期和创建时间分析任务分布`,
	Run:   runAnalyzeTime,
}

// analyzeTrendCmd 趋势分析
var analyzeTrendCmd = &cobra.Command{
	Use:   "trend",
	Short: "趋势分析",
	Long:  `分析任务完成趋势`,
	Run:   runAnalyzeTrend,
}

// analyzeReportCmd 综合报告
var analyzeReportCmd = &cobra.Command{
	Use:   "report",
	Short: "生成综合报告",
	Long:  `生成包含所有分析的综合报告`,
	Run:   runAnalyzeReport,
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
	analyzeCmd.AddCommand(analyzeQuadrantCmd)
	analyzeCmd.AddCommand(analyzePriorityCmd)
	analyzeCmd.AddCommand(analyzeTimeCmd)
	analyzeCmd.AddCommand(analyzeTrendCmd)
	analyzeCmd.AddCommand(analyzeReportCmd)

	// 通用选项
	for _, cmd := range []*cobra.Command{analyzeQuadrantCmd, analyzePriorityCmd, analyzeTimeCmd, analyzeTrendCmd, analyzeReportCmd} {
		cmd.Flags().StringVarP(&analyzeFormat, "format", "f", "text", "输出格式 (text, json)")
	}
}

// getTasksForAnalysis 获取用于分析的任务
func getTasksForAnalysis() ([]model.Task, error) {
	ctx := context.Background()
	store, err := filestore.New(cfg.Storage.Path, cfg.Storage.File.Format)
	if err != nil {
		return nil, fmt.Errorf("创建存储失败: %w", err)
	}

	// 获取所有任务
	tasks, err := store.ListTasks(ctx, storage.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取任务失败: %w", err)
	}

	return tasks, nil
}

// QuadrantAnalysis 四象限分析结果
type QuadrantAnalysis struct {
	Q1 QuadrantData `json:"q1"`
	Q2 QuadrantData `json:"q2"`
	Q3 QuadrantData `json:"q3"`
	Q4 QuadrantData `json:"q4"`
}

// QuadrantData 象限数据
type QuadrantData struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Count       int      `json:"count"`
	Percentage  float64  `json:"percentage"`
	Tasks       []string `json:"tasks,omitempty"`
}

func runAnalyzeQuadrant(_ *cobra.Command, _ []string) {
	tasks, err := getTasksForAnalysis()
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}

	analysis := QuadrantAnalysis{
		Q1: QuadrantData{
			Name:        "Q1 紧急且重要",
			Description: "立即处理",
			Tasks:       []string{},
		},
		Q2: QuadrantData{
			Name:        "Q2 重要不紧急",
			Description: "计划安排",
			Tasks:       []string{},
		},
		Q3: QuadrantData{
			Name:        "Q3 紧急不重要",
			Description: "委托他人",
			Tasks:       []string{},
		},
		Q4: QuadrantData{
			Name:        "Q4 不紧急不重要",
			Description: "考虑删除",
			Tasks:       []string{},
		},
	}

	total := len(tasks)
	for _, t := range tasks {
		if t.Status == model.StatusCompleted {
			continue
		}
		switch t.Quadrant {
		case model.QuadrantUrgentImportant:
			analysis.Q1.Count++
			analysis.Q1.Tasks = append(analysis.Q1.Tasks, t.Title)
		case model.QuadrantNotUrgentImportant:
			analysis.Q2.Count++
			analysis.Q2.Tasks = append(analysis.Q2.Tasks, t.Title)
		case model.QuadrantUrgentNotImportant:
			analysis.Q3.Count++
			analysis.Q3.Tasks = append(analysis.Q3.Tasks, t.Title)
		case model.QuadrantNotUrgentNotImportant:
			analysis.Q4.Count++
			analysis.Q4.Tasks = append(analysis.Q4.Tasks, t.Title)
		}
	}

	// 计算百分比
	activeTotal := analysis.Q1.Count + analysis.Q2.Count + analysis.Q3.Count + analysis.Q4.Count
	if activeTotal > 0 {
		analysis.Q1.Percentage = float64(analysis.Q1.Count) / float64(activeTotal) * 100
		analysis.Q2.Percentage = float64(analysis.Q2.Count) / float64(activeTotal) * 100
		analysis.Q3.Percentage = float64(analysis.Q3.Count) / float64(activeTotal) * 100
		analysis.Q4.Percentage = float64(analysis.Q4.Count) / float64(activeTotal) * 100
	}

	if analyzeFormat == "json" {
		data, _ := json.MarshalIndent(analysis, "", "  ")
		fmt.Println(string(data))
		return
	}

	// 文本格式
	fmt.Println()
	fmt.Println("📊 四象限分析（艾森豪威尔矩阵）")
	fmt.Println()
	fmt.Println("┌─────────────────────────────────────────────────────────────────┐")
	fmt.Printf("│ %-15s │ %-4d (%5.1f%%) │ 建议: 立即处理      │\n",
		"🔥 Q1 紧急且重要", analysis.Q1.Count, analysis.Q1.Percentage)
	fmt.Println("├─────────────────────────────────────────────────────────────────┤")
	fmt.Printf("│ %-15s │ %-4d (%5.1f%%) │ 建议: 计划安排      │\n",
		"📋 Q2 重要不紧急", analysis.Q2.Count, analysis.Q2.Percentage)
	fmt.Println("├─────────────────────────────────────────────────────────────────┤")
	fmt.Printf("│ %-15s │ %-4d (%5.1f%%) │ 建议: 委托他人      │\n",
		"⚡ Q3 紧急不重要", analysis.Q3.Count, analysis.Q3.Percentage)
	fmt.Println("├─────────────────────────────────────────────────────────────────┤")
	fmt.Printf("│ %-15s │ %-4d (%5.1f%%) │ 建议: 考虑删除      │\n",
		"🗑️ Q4 不紧急不重要", analysis.Q4.Count, analysis.Q4.Percentage)
	fmt.Println("└─────────────────────────────────────────────────────────────────┘")
	fmt.Println()
	fmt.Printf("总计: %d 个任务 (已完成: %d)\n", total, total-activeTotal)
}

// PriorityAnalysis 优先级分析结果
type PriorityAnalysis struct {
	Urgent  PriorityData `json:"urgent"`
	High    PriorityData `json:"high"`
	Medium  PriorityData `json:"medium"`
	Low     PriorityData `json:"low"`
	None    PriorityData `json:"none"`
	Summary SummaryData  `json:"summary"`
}

// PriorityData 优先级数据
type PriorityData struct {
	Count      int      `json:"count"`
	Percentage float64  `json:"percentage"`
	Tasks      []string `json:"tasks,omitempty"`
}

// SummaryData 汇总数据
type SummaryData struct {
	Total            int     `json:"total"`
	Active           int     `json:"active"`
	Completed        int     `json:"completed"`
	AvgPriorityScore float64 `json:"avg_priority_score"`
}

func runAnalyzePriority(_ *cobra.Command, _ []string) {
	tasks, err := getTasksForAnalysis()
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}

	analysis := PriorityAnalysis{
		Urgent: PriorityData{Tasks: []string{}},
		High:   PriorityData{Tasks: []string{}},
		Medium: PriorityData{Tasks: []string{}},
		Low:    PriorityData{Tasks: []string{}},
		None:   PriorityData{Tasks: []string{}},
	}

	var totalScore int
	var scoreCount int

	for _, t := range tasks {
		if t.Status == model.StatusCompleted {
			analysis.Summary.Completed++
			continue
		}
		analysis.Summary.Active++

		switch t.Priority {
		case model.PriorityUrgent:
			analysis.Urgent.Count++
			analysis.Urgent.Tasks = append(analysis.Urgent.Tasks, t.Title)
		case model.PriorityHigh:
			analysis.High.Count++
			analysis.High.Tasks = append(analysis.High.Tasks, t.Title)
		case model.PriorityMedium:
			analysis.Medium.Count++
			analysis.Medium.Tasks = append(analysis.Medium.Tasks, t.Title)
		case model.PriorityLow:
			analysis.Low.Count++
			analysis.Low.Tasks = append(analysis.Low.Tasks, t.Title)
		default:
			analysis.None.Count++
			analysis.None.Tasks = append(analysis.None.Tasks, t.Title)
		}

		if t.PriorityScore > 0 {
			totalScore += t.PriorityScore
			scoreCount++
		}
	}

	analysis.Summary.Total = len(tasks)
	if scoreCount > 0 {
		analysis.Summary.AvgPriorityScore = float64(totalScore) / float64(scoreCount)
	}

	// 计算百分比
	if analysis.Summary.Active > 0 {
		analysis.Urgent.Percentage = float64(analysis.Urgent.Count) / float64(analysis.Summary.Active) * 100
		analysis.High.Percentage = float64(analysis.High.Count) / float64(analysis.Summary.Active) * 100
		analysis.Medium.Percentage = float64(analysis.Medium.Count) / float64(analysis.Summary.Active) * 100
		analysis.Low.Percentage = float64(analysis.Low.Count) / float64(analysis.Summary.Active) * 100
		analysis.None.Percentage = float64(analysis.None.Count) / float64(analysis.Summary.Active) * 100
	}

	if analyzeFormat == "json" {
		data, _ := json.MarshalIndent(analysis, "", "  ")
		fmt.Println(string(data))
		return
	}

	// 文本格式
	fmt.Println()
	fmt.Println("📊 优先级分析")
	fmt.Println()
	fmt.Println("┌────────────────────────────────────────────────────────┐")
	fmt.Printf("│ 🔴 紧急 (P0)    │ %-4d (%5.1f%%)                 │\n",
		analysis.Urgent.Count, analysis.Urgent.Percentage)
	fmt.Printf("│ 🟠 高   (P1)    │ %-4d (%5.1f%%)                 │\n",
		analysis.High.Count, analysis.High.Percentage)
	fmt.Printf("│ 🟡 中   (P2)    │ %-4d (%5.1f%%)                 │\n",
		analysis.Medium.Count, analysis.Medium.Percentage)
	fmt.Printf("│ 🔵 低   (P3)    │ %-4d (%5.1f%%)                 │\n",
		analysis.Low.Count, analysis.Low.Percentage)
	fmt.Printf("│ ⚪ 无优先级      │ %-4d (%5.1f%%)                 │\n",
		analysis.None.Count, analysis.None.Percentage)
	fmt.Println("└────────────────────────────────────────────────────────┘")
	fmt.Println()
	fmt.Printf("总计: %d 个任务 | 活跃: %d | 已完成: %d\n",
		analysis.Summary.Total, analysis.Summary.Active, analysis.Summary.Completed)
	if analysis.Summary.AvgPriorityScore > 0 {
		fmt.Printf("平均优先级分数: %.1f\n", analysis.Summary.AvgPriorityScore)
	}
}

// TimeAnalysis 时间分析结果
type TimeAnalysis struct {
	Overdue   TimeData `json:"overdue"`
	Today     TimeData `json:"today"`
	Tomorrow  TimeData `json:"tomorrow"`
	ThisWeek  TimeData `json:"this_week"`
	NextWeek  TimeData `json:"next_week"`
	ThisMonth TimeData `json:"this_month"`
	Future    TimeData `json:"future"`
	NoDueDate TimeData `json:"no_due_date"`
}

// TimeData 时间数据
type TimeData struct {
	Description string   `json:"description"`
	Count       int      `json:"count"`
	Tasks       []string `json:"tasks,omitempty"`
}

func runAnalyzeTime(_ *cobra.Command, _ []string) {
	tasks, err := getTasksForAnalysis()
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := today.AddDate(0, 0, 1)
	thisWeekEnd := today.AddDate(0, 0, 7-int(today.Weekday()))
	nextWeekEnd := thisWeekEnd.AddDate(0, 0, 7)
	thisMonthEnd := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()).Add(-time.Second)

	analysis := TimeAnalysis{
		Overdue:   TimeData{Description: "已过期", Tasks: []string{}},
		Today:     TimeData{Description: "今天", Tasks: []string{}},
		Tomorrow:  TimeData{Description: "明天", Tasks: []string{}},
		ThisWeek:  TimeData{Description: "本周", Tasks: []string{}},
		NextWeek:  TimeData{Description: "下周", Tasks: []string{}},
		ThisMonth: TimeData{Description: "本月", Tasks: []string{}},
		Future:    TimeData{Description: "更远", Tasks: []string{}},
		NoDueDate: TimeData{Description: "无截止日期", Tasks: []string{}},
	}

	for _, t := range tasks {
		if t.Status == model.StatusCompleted {
			continue
		}

		if t.DueDate == nil {
			analysis.NoDueDate.Count++
			analysis.NoDueDate.Tasks = append(analysis.NoDueDate.Tasks, t.Title)
			continue
		}

		due := time.Date(t.DueDate.Year(), t.DueDate.Month(), t.DueDate.Day(), 0, 0, 0, 0, t.DueDate.Location())

		switch {
		case due.Before(today):
			analysis.Overdue.Count++
			analysis.Overdue.Tasks = append(analysis.Overdue.Tasks, t.Title)
		case due.Equal(today):
			analysis.Today.Count++
			analysis.Today.Tasks = append(analysis.Today.Tasks, t.Title)
		case due.Equal(tomorrow):
			analysis.Tomorrow.Count++
			analysis.Tomorrow.Tasks = append(analysis.Tomorrow.Tasks, t.Title)
		case due.Before(thisWeekEnd) || due.Equal(thisWeekEnd):
			analysis.ThisWeek.Count++
			analysis.ThisWeek.Tasks = append(analysis.ThisWeek.Tasks, t.Title)
		case due.Before(nextWeekEnd) || due.Equal(nextWeekEnd):
			analysis.NextWeek.Count++
			analysis.NextWeek.Tasks = append(analysis.NextWeek.Tasks, t.Title)
		case due.Before(thisMonthEnd) || due.Equal(thisMonthEnd):
			analysis.ThisMonth.Count++
			analysis.ThisMonth.Tasks = append(analysis.ThisMonth.Tasks, t.Title)
		default:
			analysis.Future.Count++
			analysis.Future.Tasks = append(analysis.Future.Tasks, t.Title)
		}
	}

	if analyzeFormat == "json" {
		data, _ := json.MarshalIndent(analysis, "", "  ")
		fmt.Println(string(data))
		return
	}

	// 文本格式
	fmt.Println()
	fmt.Println("📊 时间分布分析")
	fmt.Println()
	fmt.Println("┌────────────────────────────────────────────────────────┐")
	fmt.Printf("│ ⚠️  已过期       │ %-4d 个任务                      │\n", analysis.Overdue.Count)
	fmt.Printf("│ 🔥 今天         │ %-4d 个任务                      │\n", analysis.Today.Count)
	fmt.Printf("│ 📅 明天         │ %-4d 个任务                      │\n", analysis.Tomorrow.Count)
	fmt.Printf("│ 📆 本周         │ %-4d 个任务                      │\n", analysis.ThisWeek.Count)
	fmt.Printf("│ 📋 下周         │ %-4d 个任务                      │\n", analysis.NextWeek.Count)
	fmt.Printf("│ 🗓️  本月         │ %-4d 个任务                      │\n", analysis.ThisMonth.Count)
	fmt.Printf("│ 📁 更远         │ %-4d 个任务                      │\n", analysis.Future.Count)
	fmt.Printf("│ ❓ 无截止日期    │ %-4d 个任务                      │\n", analysis.NoDueDate.Count)
	fmt.Println("└────────────────────────────────────────────────────────┘")
}

// TrendAnalysis 趋势分析结果
type TrendAnalysis struct {
	DailyCompletions []DayData `json:"daily_completions"`
	WeeklyAverage    float64   `json:"weekly_average"`
	TotalCompleted   int       `json:"total_completed"`
}

// DayData 每日数据
type DayData struct {
	Date      string `json:"date"`
	Completed int    `json:"completed"`
}

func runAnalyzeTrend(cmd *cobra.Command, args []string) {
	tasks, err := getTasksForAnalysis()
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}

	// 统计过去7天的完成情况
	now := time.Now()
	dailyCompletions := make(map[string]int)

	for i := 0; i < 7; i++ {
		date := now.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		dailyCompletions[dateStr] = 0
	}

	var totalCompleted int
	for _, t := range tasks {
		if t.Status == model.StatusCompleted && t.CompletedAt != nil {
			dateStr := t.CompletedAt.Format("2006-01-02")
			if _, ok := dailyCompletions[dateStr]; ok {
				dailyCompletions[dateStr]++
				totalCompleted++
			}
		}
	}

	// 转换为有序列表
	var trendData []DayData
	var dates []string
	for d := range dailyCompletions {
		dates = append(dates, d)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	for _, d := range dates {
		trendData = append(trendData, DayData{
			Date:      d,
			Completed: dailyCompletions[d],
		})
	}

	weeklyAvg := float64(totalCompleted) / 7.0

	analysis := TrendAnalysis{
		DailyCompletions: trendData,
		WeeklyAverage:    weeklyAvg,
		TotalCompleted:   totalCompleted,
	}

	if analyzeFormat == "json" {
		data, _ := json.MarshalIndent(analysis, "", "  ")
		fmt.Println(string(data))
		return
	}

	// 文本格式
	fmt.Println()
	fmt.Println("📊 趋势分析（过去7天）")
	fmt.Println()
	fmt.Println("┌────────────────────────────────────────────────────────┐")
	for _, d := range trendData {
		bar := strings.Repeat("█", d.Completed)
		if d.Completed == 0 {
			bar = "░"
		}
		fmt.Printf("│ %s │ %-2d │ %-30s│\n", d.Date, d.Completed, bar)
	}
	fmt.Println("└────────────────────────────────────────────────────────┘")
	fmt.Println()
	fmt.Printf("本周完成: %d 个任务 | 日均: %.1f 个\n", totalCompleted, weeklyAvg)
}

func runAnalyzeReport(cmd *cobra.Command, args []string) {
	tasks, err := getTasksForAnalysis()
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║              📊 TaskBridge 综合分析报告                      ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 汇总
	var active, completed int
	for _, t := range tasks {
		if t.Status == model.StatusCompleted {
			completed++
		} else {
			active++
		}
	}

	fmt.Println("## 📋 任务汇总")
	fmt.Println()
	fmt.Printf("  总任务数: %d | 活跃: %d | 已完成: %d\n", len(tasks), active, completed)
	fmt.Println()

	// 四象限分布
	fmt.Println("## 🎯 四象限分布")
	fmt.Println()
	q1, q2, q3, q4 := 0, 0, 0, 0
	for _, t := range tasks {
		if t.Status == model.StatusCompleted {
			continue
		}
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
	fmt.Printf("  Q1 紧急且重要: %d | Q2 重要不紧急: %d | Q3 紧急不重要: %d | Q4 不紧急不重要: %d\n", q1, q2, q3, q4)
	fmt.Println()

	// 优先级分布
	fmt.Println("## 🔴 优先级分布")
	fmt.Println()
	urgent, high, medium, low := 0, 0, 0, 0
	for _, t := range tasks {
		if t.Status == model.StatusCompleted {
			continue
		}
		switch t.Priority {
		case model.PriorityUrgent:
			urgent++
		case model.PriorityHigh:
			high++
		case model.PriorityMedium:
			medium++
		case model.PriorityLow:
			low++
		}
	}
	fmt.Printf("  紧急: %d | 高: %d | 中: %d | 低: %d\n", urgent, high, medium, low)
	fmt.Println()

	// 时间分布
	fmt.Println("## ⏰ 时间分布")
	fmt.Println()
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var overdue, todayTasks, thisWeek int
	for _, t := range tasks {
		if t.Status == model.StatusCompleted || t.DueDate == nil {
			continue
		}
		if t.DueDate.Before(today) {
			overdue++
		} else if t.DueDate.Format("2006-01-02") == today.Format("2006-01-02") {
			todayTasks++
		} else if t.DueDate.Before(today.AddDate(0, 0, 7)) {
			thisWeek++
		}
	}
	fmt.Printf("  已过期: %d | 今天: %d | 本周: %d\n", overdue, todayTasks, thisWeek)
	fmt.Println()

	// 建议
	fmt.Println("## 💡 建议")
	fmt.Println()
	if q1 > 3 {
		fmt.Println("  ⚠️  Q1 任务过多，考虑重新评估优先级或委托他人")
	}
	if overdue > 0 {
		fmt.Printf("  ⚠️  有 %d 个任务已过期，请尽快处理\n", overdue)
	}
	if q2 > 0 {
		fmt.Println("  ✅ Q2 任务是长期目标的关键，建议安排固定时间处理")
	}
	if q4 > 5 {
		fmt.Println("  🗑️  Q4 任务较多，考虑删除或归档")
	}
	fmt.Println()

	fmt.Println("══════════════════════════════════════════════════════════════")
	fmt.Printf("报告生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
}
