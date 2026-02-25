package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/storage/filestore"
)

var (
	taskListID   string
	taskDueDate  string
	taskPriority int
	taskQuadrant int
	taskFormat   string
)

// taskCmd 任务管理命令
var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "任务管理",
	Long: `管理本地任务。

子命令:
  add      添加任务
  edit     编辑任务
  delete   删除任务
  done     完成任务
  show     显示任务详情
  move     移动任务到其他列表

示例:
  taskbridge task add "完成报告" --due 2024-01-15 --priority 3
  taskbridge task done <task-id>
  taskbridge task show <task-id>`,
}

// taskAddCmd 添加任务
var taskAddCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "添加任务",
	Long: `添加新任务到本地存储。

示例:
  taskbridge task add "完成项目报告"
  taskbridge task add "回复邮件" --due 2024-01-15 --priority 3 --quadrant 1`,
	Args: cobra.ExactArgs(1),
	Run:  runTaskAdd,
}

// taskEditCmd 编辑任务
var taskEditCmd = &cobra.Command{
	Use:   "edit <task-id>",
	Short: "编辑任务",
	Long: `编辑现有任务。

示例:
  taskbridge task edit <task-id> --title "新标题"
  taskbridge task edit <task-id> --due 2024-01-20 --priority 2`,
	Args: cobra.ExactArgs(1),
	Run:  runTaskEdit,
}

// taskDeleteCmd 删除任务
var taskDeleteCmd = &cobra.Command{
	Use:   "delete <task-id>",
	Short: "删除任务",
	Long: `删除指定任务。

示例:
  taskbridge task delete <task-id>
  taskbridge task delete <task-id> --force`,
	Args: cobra.ExactArgs(1),
	Run:  runTaskDelete,
}

// taskDoneCmd 完成任务
var taskDoneCmd = &cobra.Command{
	Use:   "done <task-id>",
	Short: "完成任务",
	Long: `将任务标记为已完成。

示例:
  taskbridge task done <task-id>`,
	Args: cobra.ExactArgs(1),
	Run:  runTaskDone,
}

// taskShowCmd 显示任务详情
var taskShowCmd = &cobra.Command{
	Use:   "show <task-id>",
	Short: "显示任务详情",
	Long: `显示指定任务的详细信息。

示例:
  taskbridge task show <task-id>
  taskbridge task show <task-id> --format json`,
	Args: cobra.ExactArgs(1),
	Run:  runTaskShow,
}

// taskUndoCmd 撤销完成
var taskUndoCmd = &cobra.Command{
	Use:   "undo <task-id>",
	Short: "撤销完成",
	Long: `将已完成的任务恢复为未完成状态。

示例:
  taskbridge task undo <task-id>`,
	Args: cobra.ExactArgs(1),
	Run:  runTaskUndo,
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskAddCmd)
	taskCmd.AddCommand(taskEditCmd)
	taskCmd.AddCommand(taskDeleteCmd)
	taskCmd.AddCommand(taskDoneCmd)
	taskCmd.AddCommand(taskShowCmd)
	taskCmd.AddCommand(taskUndoCmd)

	// add 命令选项
	taskAddCmd.Flags().StringVar(&taskListID, "list", "", "任务列表 ID")
	taskAddCmd.Flags().StringVar(&taskDueDate, "due", "", "截止日期 (YYYY-MM-DD)")
	taskAddCmd.Flags().IntVarP(&taskPriority, "priority", "p", 0, "优先级 (1-4)")
	taskAddCmd.Flags().IntVarP(&taskQuadrant, "quadrant", "q", 0, "象限 (1-4)")

	// edit 命令选项
	taskEditCmd.Flags().String("title", "", "新标题")
	taskEditCmd.Flags().StringVar(&taskDueDate, "due", "", "截止日期 (YYYY-MM-DD)")
	taskEditCmd.Flags().IntVarP(&taskPriority, "priority", "p", 0, "优先级 (1-4)")
	taskEditCmd.Flags().IntVarP(&taskQuadrant, "quadrant", "q", 0, "象限 (1-4)")

	// show 命令选项
	taskShowCmd.Flags().StringVarP(&taskFormat, "format", "f", "text", "输出格式 (text, json)")

	// delete 命令选项
	taskDeleteCmd.Flags().Bool("force", false, "强制删除，不确认")
}

func runTaskAdd(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	title := args[0]

	// 创建存储
	store, err := filestore.New(cfg.Storage.Path, cfg.Storage.File.Format)
	if err != nil {
		fmt.Printf("❌ 创建存储失败: %v\n", err)
		os.Exit(1)
	}

	// 创建任务
	task := &model.Task{
		ID:        generateID(),
		Title:     title,
		Status:    model.StatusTodo,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Source:    model.SourceLocal,
		ListID:    taskListID,
		Priority:  model.Priority(taskPriority),
		Quadrant:  model.Quadrant(taskQuadrant),
	}

	// 解析截止日期
	if taskDueDate != "" {
		due, err := time.Parse("2006-01-02", taskDueDate)
		if err != nil {
			fmt.Printf("❌ 无效的日期格式: %v\n", err)
			os.Exit(1)
		}
		task.DueDate = &due
	}

	// 计算优先级分数
	task.CalculatePriorityScore()

	// 保存任务
	if err := store.SaveTask(ctx, task); err != nil {
		fmt.Printf("❌ 保存任务失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 任务已创建: %s (ID: %s)\n", title, task.ID)
}

func runTaskEdit(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	taskID := args[0]

	// 创建存储
	store, err := filestore.New(cfg.Storage.Path, cfg.Storage.File.Format)
	if err != nil {
		fmt.Printf("❌ 创建存储失败: %v\n", err)
		os.Exit(1)
	}

	// 获取任务
	task, err := store.GetTask(ctx, taskID)
	if err != nil {
		fmt.Printf("❌ 获取任务失败: %v\n", err)
		os.Exit(1)
	}

	// 更新字段
	if title, _ := cmd.Flags().GetString("title"); title != "" {
		task.Title = title
	}
	if taskDueDate != "" {
		due, err := time.Parse("2006-01-02", taskDueDate)
		if err != nil {
			fmt.Printf("❌ 无效的日期格式: %v\n", err)
			os.Exit(1)
		}
		task.DueDate = &due
	}
	if taskPriority > 0 {
		task.Priority = model.Priority(taskPriority)
	}
	if taskQuadrant > 0 {
		task.Quadrant = model.Quadrant(taskQuadrant)
	}

	task.UpdatedAt = time.Now()
	task.CalculatePriorityScore()

	// 保存任务
	if err := store.SaveTask(ctx, task); err != nil {
		fmt.Printf("❌ 保存任务失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 任务已更新: %s\n", task.ID)
}

func runTaskDelete(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	taskID := args[0]

	// 创建存储
	store, err := filestore.New(cfg.Storage.Path, cfg.Storage.File.Format)
	if err != nil {
		fmt.Printf("❌ 创建存储失败: %v\n", err)
		os.Exit(1)
	}

	// 检查任务是否存在
	task, err := store.GetTask(ctx, taskID)
	if err != nil {
		fmt.Printf("❌ 任务不存在: %v\n", err)
		os.Exit(1)
	}

	// 确认删除
	force, _ := cmd.Flags().GetBool("force")
	if !force {
		fmt.Printf("确定要删除任务 \"%s\" 吗? (y/N): ", task.Title)
		var confirm string
		if _, err := fmt.Scanln(&confirm); err != nil {
			fmt.Println("已取消")
			return
		}
		if confirm != "y" && confirm != "Y" {
			fmt.Println("已取消")
			return
		}
	}

	// 删除任务
	if err := store.DeleteTask(ctx, taskID); err != nil {
		fmt.Printf("❌ 删除任务失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 任务已删除: %s\n", taskID)
}

func runTaskDone(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	taskID := args[0]

	// 创建存储
	store, err := filestore.New(cfg.Storage.Path, cfg.Storage.File.Format)
	if err != nil {
		fmt.Printf("❌ 创建存储失败: %v\n", err)
		os.Exit(1)
	}

	// 获取任务
	task, err := store.GetTask(ctx, taskID)
	if err != nil {
		fmt.Printf("❌ 获取任务失败: %v\n", err)
		os.Exit(1)
	}

	// 标记完成
	now := time.Now()
	task.Status = model.StatusCompleted
	task.CompletedAt = &now
	task.UpdatedAt = now

	// 保存任务
	if err := store.SaveTask(ctx, task); err != nil {
		fmt.Printf("❌ 保存任务失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 任务已完成: %s\n", task.Title)
}

func runTaskUndo(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	taskID := args[0]

	// 创建存储
	store, err := filestore.New(cfg.Storage.Path, cfg.Storage.File.Format)
	if err != nil {
		fmt.Printf("❌ 创建存储失败: %v\n", err)
		os.Exit(1)
	}

	// 获取任务
	task, err := store.GetTask(ctx, taskID)
	if err != nil {
		fmt.Printf("❌ 获取任务失败: %v\n", err)
		os.Exit(1)
	}

	// 撤销完成
	task.Status = model.StatusTodo
	task.CompletedAt = nil
	task.UpdatedAt = time.Now()

	// 保存任务
	if err := store.SaveTask(ctx, task); err != nil {
		fmt.Printf("❌ 保存任务失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 任务已恢复: %s\n", task.Title)
}

func runTaskShow(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	taskID := args[0]

	// 创建存储
	store, err := filestore.New(cfg.Storage.Path, cfg.Storage.File.Format)
	if err != nil {
		fmt.Printf("❌ 创建存储失败: %v\n", err)
		os.Exit(1)
	}

	// 获取任务
	task, err := store.GetTask(ctx, taskID)
	if err != nil {
		fmt.Printf("❌ 获取任务失败: %v\n", err)
		os.Exit(1)
	}

	if taskFormat == "json" {
		data, _ := json.MarshalIndent(task, "", "  ")
		fmt.Println(string(data))
		return
	}

	// 文本格式
	fmt.Println()
	fmt.Printf("📋 任务详情\n")
	fmt.Println("   ─────────────────────────────────")
	fmt.Printf("   ID:       %s\n", task.ID)
	fmt.Printf("   标题:     %s\n", task.Title)
	fmt.Printf("   状态:     %s\n", statusShort(task.Status))
	fmt.Printf("   优先级:   %s\n", task.Priority.String())
	fmt.Printf("   象限:     %s\n", task.Quadrant.String())

	if task.DueDate != nil {
		fmt.Printf("   截止日期: %s\n", task.DueDate.Format("2006-01-02"))
	}
	if task.ListName != "" {
		fmt.Printf("   列表:     %s\n", task.ListName)
	}
	if len(task.Tags) > 0 {
		fmt.Printf("   标签:     %v\n", task.Tags)
	}
	if task.Progress > 0 {
		fmt.Printf("   进度:     %d%%\n", task.Progress)
	}
	if task.Description != "" {
		fmt.Printf("   描述:     %s\n", task.Description)
	}

	fmt.Printf("   来源:     %s\n", task.Source)
	fmt.Printf("   创建时间: %s\n", task.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("   更新时间: %s\n", task.UpdatedAt.Format("2006-01-02 15:04:05"))

	if task.CompletedAt != nil {
		fmt.Printf("   完成时间: %s\n", task.CompletedAt.Format("2006-01-02 15:04:05"))
	}

	fmt.Println()
}

// generateID 生成任务 ID
func generateID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}
