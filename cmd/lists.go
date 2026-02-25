package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/yeisme/taskbridge/internal/model"
	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/internal/storage"
	"github.com/yeisme/taskbridge/internal/storage/filestore"
	"github.com/yeisme/taskbridge/pkg/ui"
)

var (
	listsSource  string
	listsFormat  string
	listsSyncNow bool
)

type listSummary struct {
	Provider       string `json:"provider"`
	ListID         string `json:"list_id"`
	ListName       string `json:"list_name"`
	TaskCountLocal int    `json:"task_count_local"`
}

var listsCmd = &cobra.Command{
	Use:   "lists",
	Short: "列出任务清单",
	Long: `列出本地可用的任务清单（便于获取 list_id）。

示例:
  taskbridge lists
  taskbridge lists --source ms
  taskbridge lists --sync-now --source microsoft
  taskbridge lists --format json`,
	Run: runLists,
}

func init() {
	rootCmd.AddCommand(listsCmd)

	listsCmd.Flags().StringVarP(&listsSource, "source", "s", "", "按来源筛选（支持简写，如 ms/g/tick/todo）")
	listsCmd.Flags().StringVarP(&listsFormat, "format", "f", "table", "输出格式（table, json）")
	listsCmd.Flags().BoolVar(&listsSyncNow, "sync-now", false, "查询前先同步远程任务到本地")
}

func runLists(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	resolvedSource := ""
	if listsSource != "" {
		resolvedSource = provider.ResolveProviderName(listsSource)
		if !provider.IsValidProvider(resolvedSource) {
			fmt.Printf("❌ 不支持的来源: %s\n", listsSource)
			fmt.Println("支持的来源: google (g), microsoft (ms), feishu, ticktick (tick), todoist (todo)")
			os.Exit(1)
		}
	}

	if listsSyncNow {
		if err := syncNowForList(ctx, resolvedSource); err != nil {
			fmt.Printf("❌ 同步失败: %v\n", err)
			os.Exit(1)
		}
	}

	store, err := filestore.New(cfg.Storage.Path, cfg.Storage.File.Format)
	if err != nil {
		fmt.Printf("❌ 创建存储失败: %v\n", err)
		os.Exit(1)
	}

	lists, err := store.ListTaskLists(ctx)
	if err != nil {
		fmt.Printf("❌ 查询清单失败: %v\n", err)
		os.Exit(1)
	}

	if resolvedSource != "" {
		filtered := make([]model.TaskList, 0, len(lists))
		for _, list := range lists {
			if list.Source == model.TaskSource(resolvedSource) {
				filtered = append(filtered, list)
			}
		}
		lists = filtered
	}

	if len(lists) == 0 {
		fmt.Println("📭 没有找到清单")
		if !listsSyncNow {
			fmt.Println("💡 可尝试: taskbridge lists --sync-now")
		}
		return
	}

	taskCounts, err := buildTaskCountByList(ctx, store, resolvedSource)
	if err != nil {
		fmt.Printf("❌ 统计任务数量失败: %v\n", err)
		os.Exit(1)
	}

	summaries := make([]listSummary, 0, len(lists))
	for _, list := range lists {
		summaries = append(summaries, listSummary{
			Provider:       string(list.Source),
			ListID:         list.ID,
			ListName:       list.Name,
			TaskCountLocal: taskCounts[list.ID],
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Provider == summaries[j].Provider {
			return summaries[i].ListName < summaries[j].ListName
		}
		return summaries[i].Provider < summaries[j].Provider
	})

	switch listsFormat {
	case "json":
		data, err := json.MarshalIndent(summaries, "", "  ")
		if err != nil {
			fmt.Printf("❌ 序列化失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	default:
		printListsTable(summaries)
	}
}

func buildTaskCountByList(ctx context.Context, store storage.Storage, source string) (map[string]int, error) {
	opts := storage.ListOptions{}
	if source != "" {
		opts.Source = model.TaskSource(source)
	}

	tasks, err := store.ListTasks(ctx, opts)
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int, len(tasks))
	for _, task := range tasks {
		if task.ListID == "" {
			continue
		}
		counts[task.ListID]++
	}
	return counts, nil
}

func printListsTable(lists []listSummary) {
	table := ui.NewSimpleTable(
		ui.Column{Header: "Provider", Width: 10, AlignLeft: true},
		ui.Column{Header: "ListID", Width: 30, AlignLeft: true},
		ui.Column{Header: "ListName", Width: 26, AlignLeft: true},
		ui.Column{Header: "TaskCount", Width: 9, AlignRight: true},
	)

	for _, list := range lists {
		table.AddRow(
			list.Provider,
			truncateDisplay(list.ListID, 30),
			truncateDisplay(list.ListName, 26),
			fmt.Sprintf("%d", list.TaskCountLocal),
		)
	}

	fmt.Println()
	fmt.Println(table.Render())
	fmt.Println()
	fmt.Printf("共 %d 个清单\n", len(lists))
}
