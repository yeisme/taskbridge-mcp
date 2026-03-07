package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	taskbridgeMCP "github.com/yeisme/taskbridge/internal/mcp"
	"github.com/yeisme/taskbridge/internal/project"
	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/internal/provider/google"
	"github.com/yeisme/taskbridge/internal/provider/microsoft"
	"github.com/yeisme/taskbridge/internal/provider/ticktick"
	"github.com/yeisme/taskbridge/internal/provider/todoist"
	"github.com/yeisme/taskbridge/internal/storage/filestore"
	"github.com/yeisme/taskbridge/pkg/buildinfo"
)

var (
	mcpTransport string
	mcpPort      int
	mcpToolsJSON bool
)

// MCP 命令样式定义（使用不同的名称避免与 tui.go 冲突）
var (
	mcpSuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true)

	mcpSubTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")).
				Bold(true)

	mcpHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Italic(true)
)

// mcpCmd MCP 服务命令
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP 服务",
	Long: `启动 Model Context Protocol (MCP) 服务，让 AI 可以通过 MCP 协议访问任务数据。

MCP 是一个标准协议，允许 AI 模型与外部工具进行交互。
TaskBridge 作为 MCP 服务器，提供以下功能:
  - 读取任务列表
  - 创建/更新/删除任务
  - 分析任务（四象限、优先级等）
  - 同步任务到各个平台

子命令:
  start   启动 MCP 服务
  status  查看服务状态
  tools   列出可用的 MCP 工具
  doctor  诊断配置与运行风险

示例:
  taskbridge mcp start
  taskbridge mcp start --transport sse --port 14940`,
}

// mcpStartCmd 启动 MCP 服务
var mcpStartCmd = &cobra.Command{
	Use:   "start",
	Short: "启动 MCP 服务",
	Long: `启动 MCP 服务。

传输方式:
  - stdio: 通过标准输入/输出通信（默认）
  - sse: 通过 SSE 通信
  - streamable: 通过 HTTP MCP 端点通信

示例:
  taskbridge mcp start
  taskbridge mcp start --transport sse --port 14940`,
	Run: runMCPStart,
}

// mcpStatusCmd 查看服务状态
var mcpStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看服务状态",
	Long:  `查看 MCP 服务的当前状态`,
	Run:   runMCPStatus,
}

// mcpToolsCmd 列出可用工具
var mcpToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "列出可用的 MCP 工具",
	Long:  `列出 TaskBridge 提供的所有 MCP 工具`,
	Run:   runMCPTools,
}

// mcpDoctorCmd MCP 配置诊断
var mcpDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "诊断 MCP 配置与运行风险",
	Long:  `诊断 MCP 配置、端口占用与 Provider 凭证就绪情况`,
	Run:   runMCPDoctor,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpStartCmd)
	mcpCmd.AddCommand(mcpStatusCmd)
	mcpCmd.AddCommand(mcpToolsCmd)
	mcpCmd.AddCommand(mcpDoctorCmd)

	mcpStartCmd.Flags().StringVar(&mcpTransport, "transport", "stdio", "传输方式 (stdio, sse, streamable)")
	mcpStartCmd.Flags().IntVarP(&mcpPort, "port", "p", 14940, "HTTP 端口（用于 sse/streamable 模式）")
	mcpToolsCmd.Flags().BoolVar(&mcpToolsJSON, "json", false, "以 JSON 格式输出工具列表")
}

// MCPTool MCP 工具定义
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// getMCPTools 获取可用的 MCP 工具列表
func getMCPTools() []MCPTool {
	return []MCPTool{
		{
			Name:        "list_tasks",
			Description: "列出任务，支持来源/清单/状态/优先级/query 等复杂过滤",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source": map[string]interface{}{
						"type":        "string",
						"description": "按来源筛选（支持简写）",
					},
					"list_id": map[string]interface{}{
						"description": "按清单 ID 筛选（string 或 string[]）",
					},
					"list_name": map[string]interface{}{
						"description": "按清单名称筛选（string 或 string[]）",
					},
					"task_id": map[string]interface{}{
						"description": "按任务 ID 筛选（string 或 string[]）",
					},
					"status": map[string]interface{}{
						"description": "按状态筛选（string 或 string[]）",
					},
					"quadrant": map[string]interface{}{
						"description": "按象限筛选（integer 或 integer[]）",
					},
					"priority": map[string]interface{}{
						"description": "按优先级筛选（integer 或 integer[]）",
					},
					"tag": map[string]interface{}{
						"description": "按标签筛选（string 或 string[]）",
					},
					"due_before": map[string]interface{}{
						"type":        "string",
						"description": "截止日期上限 YYYY-MM-DD",
					},
					"due_after": map[string]interface{}{
						"type":        "string",
						"description": "截止日期下限 YYYY-MM-DD",
					},
					"query": map[string]interface{}{
						"type":        "string",
						"description": "关键词/自然语言查询",
					},
					"detail": map[string]interface{}{
						"type":        "string",
						"description": "compact 或 full，默认 compact",
					},
					"include_meta": map[string]interface{}{
						"type":        "boolean",
						"description": "是否返回 meta 信息",
					},
				},
			},
		},
		{
			Name:        "list_task_lists",
			Description: "列出任务清单，包含 provider/list_id/list_name/task_count_local",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source": map[string]interface{}{
						"type":        "string",
						"description": "按来源筛选（支持简写）",
					},
				},
			},
		},
		{
			Name:        "create_project",
			Description: "创建新项目（草稿状态）",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "项目名称",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "项目描述",
					},
					"parent_id": map[string]interface{}{
						"type":        "string",
						"description": "父项目 ID（可选）",
					},
					"goal_text": map[string]interface{}{
						"type":        "string",
						"description": "自然语言目标",
					},
					"horizon_days": map[string]interface{}{
						"type":        "integer",
						"description": "规划周期天数（默认 14）",
					},
					"list_id": map[string]interface{}{
						"type":        "string",
						"description": "任务默认写入清单 ID（可选）",
					},
					"source": map[string]interface{}{
						"type":        "string",
						"description": "目标来源（支持简写）",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "list_projects",
			Description: "列出项目",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "split_project",
			Description: "AI 辅助拆分项目",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project_id": map[string]interface{}{
						"type":        "string",
						"description": "项目 ID",
					},
					"ai_hint": map[string]interface{}{
						"type":        "string",
						"description": "拆分提示（可选）",
					},
					"goal_text": map[string]interface{}{
						"type":        "string",
						"description": "临时覆盖项目目标文本（可选）",
					},
					"horizon_days": map[string]interface{}{
						"type":        "integer",
						"description": "规划周期天数（默认 14）",
					},
					"max_tasks": map[string]interface{}{
						"type":        "integer",
						"description": "最大拆分任务数（默认 12）",
					},
					"constraints": map[string]interface{}{
						"type":        "object",
						"description": "结构化拆分约束",
						"properties": map[string]interface{}{
							"require_deliverable": map[string]interface{}{
								"type":        "boolean",
								"description": "是否强制每个子任务包含交付物",
							},
							"min_estimate_minutes": map[string]interface{}{
								"type":        "integer",
								"description": "最小时长（分钟）",
							},
							"max_estimate_minutes": map[string]interface{}{
								"type":        "integer",
								"description": "最大时长（分钟）",
							},
							"min_tasks": map[string]interface{}{
								"type":        "integer",
								"description": "最少任务数",
							},
							"max_tasks": map[string]interface{}{
								"type":        "integer",
								"description": "最多任务数",
							},
							"min_practice_tasks": map[string]interface{}{
								"type":        "integer",
								"description": "最少实战任务数（按 practice 标签）",
							},
						},
					},
				},
				"required": []string{"project_id"},
			},
		},
		{
			Name:        "confirm_project",
			Description: "确认项目",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project_id": map[string]interface{}{
						"type":        "string",
						"description": "项目 ID",
					},
					"plan_id": map[string]interface{}{
						"type":        "string",
						"description": "指定确认的计划 ID（默认最新）",
					},
					"write_tasks": map[string]interface{}{
						"type":        "boolean",
						"description": "是否写入任务（默认 true）",
					},
				},
				"required": []string{"project_id"},
			},
		},
		{
			Name:        "sync_project",
			Description: "同步项目到指定平台",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"project_id": map[string]interface{}{
						"type":        "string",
						"description": "项目 ID",
					},
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "目标平台（支持简写）",
					},
				},
				"required": []string{"project_id", "provider"},
			},
		},
		{
			Name:        "get_prompt",
			Description: "获取内置提示词模板（含 json_query_commands）",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "提示词名称",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "sync_push",
			Description: "推送本地任务到远程（支持 delete/dry_run）",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "目标 provider（支持简写）",
					},
					"delete": map[string]interface{}{
						"type":        "boolean",
						"description": "是否删除远程多余任务",
					},
					"dry_run": map[string]interface{}{
						"type":        "boolean",
						"description": "模拟执行",
					},
				},
				"required": []string{"provider"},
			},
		},
		{
			Name:        "sync_pull",
			Description: "从远程拉取任务到本地",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "来源 provider（支持简写）",
					},
				},
				"required": []string{"provider"},
			},
		},
		{
			Name:        "list_providers",
			Description: "列出 Provider 状态与能力",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "get_provider_info",
			Description: "获取单个 Provider 详情",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "provider 名称或简写",
					},
				},
				"required": []string{"provider"},
			},
		},
		{
			Name:        "get_provider_config_template",
			Description: "获取 Provider 配置模板",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "provider 名称或简写",
					},
				},
				"required": []string{"provider"},
			},
		},
		{
			Name:        "get_server_info",
			Description: "获取 MCP 服务版本、能力、工具与提示词清单",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "create_task",
			Description: "创建新任务",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "任务标题",
					},
					"due_date": map[string]interface{}{
						"type":        "string",
						"description": "截止日期 (YYYY-MM-DD)",
					},
					"priority": map[string]interface{}{
						"type":        "integer",
						"description": "优先级 (1-4)",
					},
					"quadrant": map[string]interface{}{
						"type":        "integer",
						"description": "象限 (1-4)",
					},
					"parent_id": map[string]interface{}{
						"type":        "string",
						"description": "父任务 ID（用于创建子任务）",
					},
				},
				"required": []string{"title"},
			},
		},
		{
			Name:        "update_task",
			Description: "更新现有任务",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "任务 ID",
					},
					"title": map[string]interface{}{
						"type":        "string",
						"description": "新标题",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "新状态",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "delete_task",
			Description: "删除任务",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "任务 ID",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "complete_task",
			Description: "将任务标记为已完成",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "任务 ID",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			Name:        "analyze_quadrant",
			Description: "按四象限（艾森豪威尔矩阵）分析任务分布",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "analyze_priority",
			Description: "按优先级分析任务分布",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "analyze_overdue_health",
			Description: "分析逾期任务健康度，输出过载风险与建议动作",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source": map[string]interface{}{
						"type":        "string",
						"description": "按来源筛选（支持简写）",
					},
					"list_id": map[string]interface{}{
						"description": "按清单 ID 筛选（string 或 string[]）",
					},
					"include_suggestions": map[string]interface{}{
						"type":        "boolean",
						"description": "是否返回建议动作与提问",
					},
				},
			},
		},
		{
			Name:        "resolve_overdue_tasks",
			Description: "批量处理逾期任务（defer/reschedule/delete/split_then_schedule）",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"actions": map[string]interface{}{
						"type":        "array",
						"description": "处理动作列表",
					},
					"dry_run": map[string]interface{}{
						"type":        "boolean",
						"description": "模拟执行",
					},
					"confirm_token": map[string]interface{}{
						"type":        "string",
						"description": "删除确认 token（confirm-delete）",
					},
				},
				"required": []string{"actions"},
			},
		},
		{
			Name:        "rebalance_longterm_tasks",
			Description: "根据短期任务负载自动调配长期无排期任务",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source": map[string]interface{}{
						"type":        "string",
						"description": "按来源筛选（支持简写）",
					},
					"list_id": map[string]interface{}{
						"description": "按清单 ID 筛选（string 或 string[]）",
					},
					"dry_run": map[string]interface{}{
						"type":        "boolean",
						"description": "模拟执行",
					},
				},
			},
		},
		{
			Name:        "detect_decomposition_candidates",
			Description: "识别复杂/抽象且缺少子任务的候选任务",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"source": map[string]interface{}{
						"type":        "string",
						"description": "按来源筛选（支持简写）",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "返回条数（默认 20）",
					},
				},
			},
		},
		{
			Name:        "decompose_task_with_provider",
			Description: "基于 provider 能力拆分任务（可选落地写入）",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "待拆分任务 ID",
					},
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "目标 provider（支持简写）",
					},
					"strategy": map[string]interface{}{
						"type":        "string",
						"description": "策略（project_split/markdown_split）",
					},
					"write_tasks": map[string]interface{}{
						"type":        "boolean",
						"description": "是否落地写入",
					},
				},
				"required": []string{"task_id"},
			},
		},
		{
			Name:        "analyze_achievement",
			Description: "分析完成情况并输出成就反馈",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"window_days": map[string]interface{}{
						"type":        "integer",
						"description": "统计窗口（默认 30）",
					},
					"compare_previous": map[string]interface{}{
						"type":        "boolean",
						"description": "是否对比上一周期",
					},
				},
			},
		},
	}
}

// printToStderr 在 stdio 模式下将信息输出到 stderr，避免污染 JSON-RPC 通信
func printToStderr(message string) {
	fmt.Fprint(os.Stderr, message)
}

func runMCPStart(cmd *cobra.Command, args []string) {
	transport, warnings, err := resolveMCPStartTransport(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 不支持的传输方式: %s\n", mcpTransport)
		fmt.Fprintf(os.Stderr, "支持的传输方式: stdio, sse, streamable\n")
		os.Exit(1)
	}
	port := resolveMCPStartPort(cmd)

	// 在 stdio 模式下，所有日志信息必须输出到 stderr，因为 stdout 用于 JSON-RPC 通信
	// 在 sse/streamable 模式下，也输出到 stderr 避免干扰 HTTP 服务
	for _, warning := range warnings {
		printToStderr(fmt.Sprintf("⚠️ %s\n", warning))
	}
	printToStderr(titleStyle.Render("🚀 启动 TaskBridge MCP 服务"))
	printToStderr("\n")
	printToStderr(statusBarStyle.Render(fmt.Sprintf("传输方式: %s", transport)))
	printToStderr("\n")
	if transport == "sse" || transport == "streamable" {
		printToStderr(statusBarStyle.Render(fmt.Sprintf("端口: %d", port)))
		printToStderr("\n")
		if transport == "sse" {
			printToStderr(statusBarStyle.Render(fmt.Sprintf("SSE 端点: http://localhost:%d/sse", port)))
			printToStderr("\n")
		} else {
			printToStderr(statusBarStyle.Render(fmt.Sprintf("HTTP 端点: http://localhost:%d/mcp", port)))
			printToStderr("\n")
		}
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		printToStderr("\n⏹️ 正在停止服务...\n")
		cancel()
	}()

	// 创建存储
	store, err := filestore.New(cfg.Storage.Path, cfg.Storage.File.Format)
	if err != nil {
		printToStderr(fmt.Sprintf("❌ 初始化存储失败: %v\n", err))
		os.Exit(1)
	}
	projectStore, err := project.NewFileStore(cfg.Storage.Path)
	if err != nil {
		printToStderr(fmt.Sprintf("❌ 初始化项目存储失败: %v\n", err))
		os.Exit(1)
	}

	// 初始化 Provider 映射
	providers := make(map[string]provider.Provider)

	// 初始化 Google Provider
	googleProvider, err := google.NewProviderFromHome()
	if err == nil && googleProvider.IsAuthenticated() {
		providers["google"] = googleProvider
	}

	// 初始化 Microsoft Provider（与 sync/auth 一致：优先从 HOME 凭证加载）
	microsoftProvider, err := microsoft.NewProviderFromHome()
	if err == nil && microsoftProvider.IsAuthenticated() {
		providers["microsoft"] = microsoftProvider
	}
	todoistProvider, err := todoist.NewProviderFromHome()
	if err == nil {
		if authErr := todoistProvider.Authenticate(ctx, nil); authErr == nil {
			providers["todoist"] = todoistProvider
		}
	}
	tickProvider, err := ticktick.NewProviderFromHomeByName("ticktick")
	if err == nil {
		if authErr := tickProvider.Authenticate(ctx, nil); authErr == nil {
			providers["ticktick"] = tickProvider
		}
	}
	didaProvider, err := ticktick.NewProviderFromHomeByName("dida")
	if err == nil {
		if authErr := didaProvider.Authenticate(ctx, nil); authErr == nil {
			providers["dida"] = didaProvider
		}
	}

	if _, ok := providers["microsoft"]; !ok && cfg.Providers.Microsoft.Enabled {
		printToStderr("⚠️ Microsoft Provider 未就绪，请运行 'taskbridge auth login microsoft'\n")
	}
	if _, ok := providers["todoist"]; !ok && cfg.Providers.Todoist.Enabled {
		printToStderr("⚠️ Todoist Provider 未就绪，请运行 'taskbridge auth login todoist'\n")
	}
	if _, ok := providers["ticktick"]; !ok && cfg.Providers.TickTick.Enabled {
		printToStderr("⚠️ TickTick Provider 未就绪，请运行 'taskbridge auth login ticktick'\n")
	}
	if _, ok := providers["dida"]; !ok && cfg.Providers.Dida.Enabled {
		printToStderr("⚠️ Dida Provider 未就绪，请运行 'taskbridge auth login dida'\n")
	}

	// 创建 MCP 服务器
	server := taskbridgeMCP.NewServer(
		taskbridgeMCP.WithTaskStorage(store),
		taskbridgeMCP.WithProjectStore(projectStore),
		taskbridgeMCP.WithConfig(&taskbridgeMCP.ServerConfig{
			Name:      "taskbridge",
			Version:   buildinfo.Version,
			Transport: transport,
			Port:      port,
		}),
		taskbridgeMCP.WithProviders(providers),
		taskbridgeMCP.WithProviderConfig(&cfg.Providers),
		taskbridgeMCP.WithIntelligenceConfig(&cfg.MCP.Intelligence),
	)

	// 显示启动信息（输出到 stderr）
	printToStderr("\n")
	printToStderr(mcpSuccessStyle.Render("✅ MCP 服务已启动"))
	printToStderr("\n\n")
	printToStderr(mcpSubTitleStyle.Render("可用的 MCP 工具:"))
	printToStderr("\n")
	for name := range server.GetTools() {
		printToStderr(fmt.Sprintf("  • %s\n", name))
	}
	printToStderr("\n")
	printToStderr(mcpSubTitleStyle.Render("可用的 MCP 提示词:"))
	printToStderr("\n")
	for name := range server.GetPrompts() {
		printToStderr(fmt.Sprintf("  • %s\n", name))
	}
	printToStderr("\n")
	printToStderr(mcpHelpStyle.Render("按 Ctrl+C 停止服务"))
	printToStderr("\n")

	// 启动服务
	if err := server.Start(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			printToStderr("👋 服务已停止\n")
			return
		}
		printToStderr(fmt.Sprintf("❌ MCP 服务启动失败: %v\n", err))
		os.Exit(1)
	}

	printToStderr("👋 服务已停止\n")
}

func runMCPStatus(cmd *cobra.Command, args []string) {
	fmt.Println()
	fmt.Println("📊 MCP 服务状态")
	fmt.Println("   ─────────────────────────────────")

	if cfg.MCP.Enabled {
		fmt.Println("   状态: ✅ 已启用")
	} else {
		fmt.Println("   状态: ❌ 未启用")
	}

	transport, warnings, err := resolveTransportForDisplay(cfg.MCP.Transport)
	if err != nil {
		transport = cfg.MCP.Transport
	}
	fmt.Printf("   传输方式: %s\n", transport)
	for _, warning := range warnings {
		fmt.Printf("   警告: %s\n", warning)
	}
	if transport == "sse" || transport == "streamable" {
		fmt.Printf("   端口: %d\n", cfg.MCP.Port)
	}
	if err != nil {
		fmt.Printf("   错误: %v\n", err)
	}

	fmt.Println()
	fmt.Println("已注册的工具:")
	for _, tool := range getMCPTools() {
		fmt.Printf("  - %s\n", tool.Name)
	}
	fmt.Println()
}

func runMCPTools(cmd *cobra.Command, args []string) {
	tools := getMCPTools()

	if mcpToolsJSON {
		data, _ := json.MarshalIndent(tools, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Println()
	fmt.Println("📦 可用的 MCP 工具")
	fmt.Println()

	for i, tool := range tools {
		fmt.Printf("%d. %s\n", i+1, tool.Name)
		fmt.Printf("   描述: %s\n", tool.Description)
		if required, ok := tool.InputSchema["required"].([]string); ok && len(required) > 0 {
			fmt.Printf("   必需参数: %v\n", required)
		}
		fmt.Println()
	}
}

func runMCPDoctor(cmd *cobra.Command, args []string) {
	_ = cmd
	_ = args

	exitCode := writeDoctorReport(os.Stdout, buildDoctorReport(cfg))
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func resolveMCPStartTransport(cmd *cobra.Command) (string, []string, error) {
	raw := cfg.MCP.Transport
	if cmd.Flags().Changed("transport") {
		raw = mcpTransport
	}
	return resolveTransportForDisplay(raw)
}

func resolveMCPStartPort(cmd *cobra.Command) int {
	if cmd.Flags().Changed("port") {
		return mcpPort
	}
	return cfg.MCP.Port
}
