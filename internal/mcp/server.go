// Package mcp 提供 MCP (Model Context Protocol) 服务器实现
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yeisme/taskbridge/internal/project"
	"github.com/yeisme/taskbridge/internal/provider"
	"github.com/yeisme/taskbridge/internal/storage"
	pkgconfig "github.com/yeisme/taskbridge/pkg/config"
)

// Server MCP 服务器
type Server struct {
	server         *mcp.Server
	taskStore      storage.Storage
	projectStore   project.Store
	config         *ServerConfig
	providers      map[string]provider.Provider
	providerConfig *pkgconfig.ProvidersConfig
}

// ServerConfig 服务器配置
type ServerConfig struct {
	// Name 服务名称
	Name string
	// Version 服务版本
	Version string
	// Transport 传输方式 (stdio, sse, streamable, inmemory)
	Transport string
	// Port HTTP 端口（用于 sse 和 streamable 模式）
	Port int
}

// ServerOption 服务器选项
type ServerOption func(*Server)

// WithTaskStorage 设置任务存储
func WithTaskStorage(store storage.Storage) ServerOption {
	return func(s *Server) {
		s.taskStore = store
	}
}

// WithProjectStore 设置项目存储
func WithProjectStore(store project.Store) ServerOption {
	return func(s *Server) {
		s.projectStore = store
	}
}

// WithConfig 设置配置
func WithConfig(cfg *ServerConfig) ServerOption {
	return func(s *Server) {
		s.config = cfg
	}
}

// WithProviders 设置 Provider 映射
func WithProviders(providers map[string]provider.Provider) ServerOption {
	return func(s *Server) {
		s.providers = providers
	}
}

// WithProviderConfig 设置 Provider 配置
func WithProviderConfig(cfg *pkgconfig.ProvidersConfig) ServerOption {
	return func(s *Server) {
		s.providerConfig = cfg
	}
}

// NewServer 创建 MCP 服务器
func NewServer(opts ...ServerOption) *Server {
	s := &Server{
		config: &ServerConfig{
			Name:      "taskbridge",
			Version:   "1.0.0",
			Transport: "stdio",
		},
		providers: make(map[string]provider.Provider),
	}

	for _, opt := range opts {
		opt(s)
	}

	// 创建 MCP 服务器实例
	s.server = mcp.NewServer(&mcp.Implementation{
		Name:    s.config.Name,
		Version: s.config.Version,
	}, nil)

	// 注册工具
	s.registerTools()

	// 注册提示词
	s.registerPrompts()

	// 注册资源
	s.registerResources()

	return s
}

// Start 启动 MCP 服务
func (s *Server) Start(ctx context.Context) error {
	switch s.config.Transport {
	case "stdio":
		return s.startStdio(ctx)
	case "sse":
		return s.startSSE(ctx)
	case "streamable":
		return s.startStreamableHTTP(ctx)
	case "inmemory":
		return s.startInMemory(ctx)
	default:
		return fmt.Errorf("unsupported transport: %s", s.config.Transport)
	}
}

// startStdio 启动 stdio 传输
func (s *Server) startStdio(ctx context.Context) error {
	transport := &mcp.StdioTransport{}
	session, err := s.server.Connect(ctx, transport, nil)
	if err != nil {
		return err
	}
	// 等待上下文取消
	<-ctx.Done()
	return session.Close()
}

// startInMemory 启动内存传输（用于测试）
func (s *Server) startInMemory(ctx context.Context) error {
	// 内存传输主要用于测试，这里简单返回
	<-ctx.Done()
	return nil
}

// startSSE 启动 SSE 传输
func (s *Server) startSSE(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.config.Port)

	// 创建 SSE Handler
	sseHandler := mcp.NewSSEHandler(func(_ *http.Request) *mcp.Server {
		return s.server
	}, nil)

	// 设置路由
	mux := http.NewServeMux()
	mux.Handle("/sse", sseHandler)
	mux.Handle("/message", sseHandler)

	// 创建 HTTP 服务器
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// 启动服务器
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("SSE server error: %v\n", err)
		}
	}()

	// 等待上下文取消
	<-ctx.Done()
	// 使用独立超时上下文进行优雅关闭，避免直接传入已取消的 ctx 导致返回 context canceled
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}

// startStreamableHTTP 启动 Streamable HTTP 传输
func (s *Server) startStreamableHTTP(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.config.Port)

	// 创建 Streamable HTTP Handler
	httpHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return s.server
	}, nil)

	// 设置路由
	mux := http.NewServeMux()
	mux.Handle("/mcp", httpHandler)

	// 创建 HTTP 服务器
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// 启动服务器
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Streamable HTTP server error: %v\n", err)
		}
	}()

	// 等待上下文取消
	<-ctx.Done()
	// 使用独立超时上下文进行优雅关闭，避免直接传入已取消的 ctx 导致返回 context canceled
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}

// registerTools 注册所有工具
func (s *Server) registerTools() {
	// 任务管理工具
	s.registerTaskTools()

	// 分析工具
	s.registerAnalysisTools()

	// 项目管理工具
	s.registerProjectTools()

	// 提示词工具
	s.registerPromptTools()

	// 同步工具
	s.registerSyncTools()

	// Provider 工具
	s.registerProviderTools()
}

// registerTaskTools 注册任务管理工具
func (s *Server) registerTaskTools() {
	// 列出任务工具
	s.server.AddTool(&mcp.Tool{
		Name:        "list_tasks",
		Description: "列出任务，支持来源、清单、状态、优先级、时间范围、query 文本等复杂过滤",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"source": {"type": "string", "description": "按来源筛选（支持简写：g/ms/tick/todo）"},
				"list_id": {
					"oneOf": [
						{"type": "string"},
						{"type": "array", "items": {"type": "string"}}
					],
					"description": "按清单 ID 筛选"
				},
				"list_name": {
					"oneOf": [
						{"type": "string"},
						{"type": "array", "items": {"type": "string"}}
					],
					"description": "按清单名称筛选（规范化精确匹配）"
				},
				"task_id": {
					"oneOf": [
						{"type": "string"},
						{"type": "array", "items": {"type": "string"}}
					],
					"description": "按任务 ID 筛选"
				},
				"status": {
					"oneOf": [
						{"type": "string"},
						{"type": "array", "items": {"type": "string"}}
					],
					"description": "任务状态（todo/in_progress/completed/cancelled/deferred）"
				},
				"quadrant": {
					"oneOf": [
						{"type": "integer"},
						{"type": "array", "items": {"type": "integer"}}
					],
					"description": "按象限筛选（1-4）"
				},
				"priority": {
					"oneOf": [
						{"type": "integer"},
						{"type": "array", "items": {"type": "integer"}}
					],
					"description": "按优先级筛选（0-4）"
				},
				"tag": {
					"oneOf": [
						{"type": "string"},
						{"type": "array", "items": {"type": "string"}}
					],
					"description": "按标签筛选"
				},
				"due_before": {"type": "string", "description": "截止日期上限，格式 YYYY-MM-DD"},
				"due_after": {"type": "string", "description": "截止日期下限，格式 YYYY-MM-DD"},
				"query": {"type": "string", "description": "关键词/自然语言文本过滤"},
				"limit": {"type": "integer", "description": "返回条数上限"},
				"offset": {"type": "integer", "description": "分页偏移量"},
				"order_by": {"type": "string", "description": "排序字段：due_date/priority/created_at/updated_at"},
				"order_desc": {"type": "boolean", "description": "是否降序排序"},
				"detail": {"type": "string", "description": "返回字段级别：compact/full，默认 compact"},
				"include_meta": {"type": "boolean", "description": "是否返回 meta 信息（包含过滤条件与统计）"}
			}
		}`),
	}, s.handleListTasks)

	// 列出清单工具
	s.server.AddTool(&mcp.Tool{
		Name:        "list_task_lists",
		Description: "列出任务清单，包含 provider/list_id/list_name/task_count_local",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"source": {"type": "string", "description": "按来源筛选（支持简写：g/ms/tick/todo）"}
			}
		}`),
	}, s.handleListTaskLists)

	// 创建任务工具
	s.server.AddTool(&mcp.Tool{
		Name:        "create_task",
		Description: "创建新任务",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"title": {"type": "string", "description": "任务标题"},
				"due_date": {"type": "string", "description": "截止日期 (YYYY-MM-DD)"},
				"priority": {"type": "integer", "description": "优先级 (1-4)"},
				"quadrant": {"type": "integer", "description": "象限 (1-4)"}
			},
			"required": ["title"]
		}`),
	}, s.handleCreateTask)

	// 更新任务工具
	s.server.AddTool(&mcp.Tool{
		Name:        "update_task",
		Description: "更新现有任务",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {"type": "string", "description": "任务 ID"},
				"title": {"type": "string", "description": "新标题"},
				"status": {"type": "string", "description": "新状态"}
			},
			"required": ["id"]
		}`),
	}, s.handleUpdateTask)

	// 删除任务工具
	s.server.AddTool(&mcp.Tool{
		Name:        "delete_task",
		Description: "删除任务",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {"type": "string", "description": "任务 ID"}
			},
			"required": ["id"]
		}`),
	}, s.handleDeleteTask)

	// 完成任务工具
	s.server.AddTool(&mcp.Tool{
		Name:        "complete_task",
		Description: "将任务标记为已完成",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"id": {"type": "string", "description": "任务 ID"}
			},
			"required": ["id"]
		}`),
	}, s.handleCompleteTask)
}

// registerAnalysisTools 注册分析工具
func (s *Server) registerAnalysisTools() {
	// 四象限分析工具
	s.server.AddTool(&mcp.Tool{
		Name:        "analyze_quadrant",
		Description: "按四象限（艾森豪威尔矩阵）分析任务分布",
		InputSchema: json.RawMessage(`{"type": "object"}`),
	}, s.handleAnalyzeQuadrant)

	// 优先级分析工具
	s.server.AddTool(&mcp.Tool{
		Name:        "analyze_priority",
		Description: "按优先级分析任务分布",
		InputSchema: json.RawMessage(`{"type": "object"}`),
	}, s.handleAnalyzePriority)
}

// registerProjectTools 注册项目管理工具
func (s *Server) registerProjectTools() {
	// 创建项目工具
	s.server.AddTool(&mcp.Tool{
		Name:        "create_project",
		Description: "创建新项目（草稿状态）",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {"type": "string", "description": "项目名称"},
				"description": {"type": "string", "description": "项目描述"},
				"parent_id": {"type": "string", "description": "父项目 ID（可选）"},
				"goal_text": {"type": "string", "description": "自然语言目标"},
				"horizon_days": {"type": "integer", "description": "规划周期天数（默认 14）"},
				"list_id": {"type": "string", "description": "任务默认写入清单 ID（可选）"},
				"source": {"type": "string", "description": "目标来源（支持简写）"}
			},
			"required": ["name"]
		}`),
	}, s.handleCreateProject)

	// 列出项目工具
	s.server.AddTool(&mcp.Tool{
		Name:        "list_projects",
		Description: "列出所有项目",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"status": {"type": "string", "description": "按状态筛选 (draft, split_suggested, confirmed, synced)"}
			}
		}`),
	}, s.handleListProjects)

	// 拆分项目工具
	s.server.AddTool(&mcp.Tool{
		Name:        "split_project",
		Description: "使用 AI 辅助将项目拆分为子任务",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"project_id": {"type": "string", "description": "项目 ID"},
				"ai_hint": {"type": "string", "description": "AI 拆分提示（可选）"},
				"goal_text": {"type": "string", "description": "临时覆盖项目目标文本（可选）"},
				"horizon_days": {"type": "integer", "description": "规划周期天数（默认 14）"},
				"max_tasks": {"type": "integer", "description": "最大拆分任务数（默认 12）"},
				"constraints": {
					"type": "object",
					"description": "结构化拆分约束",
					"properties": {
						"require_deliverable": {"type": "boolean", "description": "是否强制每个子任务包含交付物"},
						"min_estimate_minutes": {"type": "integer", "description": "最小时长（分钟）"},
						"max_estimate_minutes": {"type": "integer", "description": "最大时长（分钟）"},
						"min_tasks": {"type": "integer", "description": "最少任务数"},
						"max_tasks": {"type": "integer", "description": "最多任务数"},
						"min_practice_tasks": {"type": "integer", "description": "最少实战任务数（按 practice 标签）"}
					}
				}
			},
			"required": ["project_id"]
		}`),
	}, s.handleSplitProject)

	// 确认项目工具
	s.server.AddTool(&mcp.Tool{
		Name:        "confirm_project",
		Description: "确认项目，准备同步",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"project_id": {"type": "string", "description": "项目 ID"},
				"plan_id": {"type": "string", "description": "指定确认的计划 ID（默认最新）"},
				"write_tasks": {"type": "boolean", "description": "是否写入任务（默认 true）"}
			},
			"required": ["project_id"]
		}`),
	}, s.handleConfirmProject)

	// 同步项目工具
	s.server.AddTool(&mcp.Tool{
		Name:        "sync_project",
		Description: "同步项目到指定平台",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"project_id": {"type": "string", "description": "项目 ID"},
				"provider": {"type": "string", "description": "目标平台 (google, microsoft)"}
			},
			"required": ["project_id", "provider"]
		}`),
	}, s.handleSyncProject)
}

// registerPromptTools 注册提示词工具
func (s *Server) registerPromptTools() {
	// 获取提示词工具
	s.server.AddTool(&mcp.Tool{
		Name:        "get_prompt",
		Description: "获取内置提示词模板",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"name": {"type": "string", "description": "提示词名称 (quadrant_analysis, task_creation, project_planning, ai_split_guide, json_query_commands)"}
			},
			"required": ["name"]
		}`),
	}, s.handleGetPrompt)
}

// registerSyncTools 注册同步工具
func (s *Server) registerSyncTools() {
	// 推送同步工具
	s.server.AddTool(&mcp.Tool{
		Name:        "sync_push",
		Description: "推送本地任务到远程平台，可选择删除远程多余任务",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"provider": {"type": "string", "description": "目标平台 (google, microsoft, feishu, ticktick, dida, todoist)"},
				"delete": {"type": "boolean", "description": "是否删除远程存在但本地不存在的任务"},
				"dry_run": {"type": "boolean", "description": "模拟执行，不实际修改"}
			},
			"required": ["provider"]
		}`),
	}, s.handleSyncPush)

	// 拉取同步工具
	s.server.AddTool(&mcp.Tool{
		Name:        "sync_pull",
		Description: "从远程平台拉取任务到本地",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"provider": {"type": "string", "description": "来源平台 (google, microsoft, feishu, ticktick, dida, todoist)"}
			},
			"required": ["provider"]
		}`),
	}, s.handleSyncPull)
}

// registerProviderTools 注册 Provider 工具
func (s *Server) registerProviderTools() {
	// 列出 Providers
	s.server.AddTool(&mcp.Tool{
		Name:        "list_providers",
		Description: "列出所有支持的 Provider 及其状态",
		InputSchema: json.RawMessage(`{"type": "object"}`),
	}, s.handleListProviders)

	// 获取 Provider 详情
	s.server.AddTool(&mcp.Tool{
		Name:        "get_provider_info",
		Description: "获取指定 Provider 的详细信息和能力（支持简写：google, ms, feishu, tick, todo）",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"provider": {"type": "string", "description": "Provider 名称或简写"}
			},
			"required": ["provider"]
		}`),
	}, s.handleGetProviderInfo)

	// 获取配置模板
	s.server.AddTool(&mcp.Tool{
		Name:        "get_provider_config_template",
		Description: "获取 Provider 的配置模板，AI agent 可据此生成配置",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"provider": {"type": "string", "description": "Provider 名称或简写"}
			},
			"required": ["provider"]
		}`),
	}, s.handleGetProviderConfigTemplate)
}

// registerPrompts 注册所有提示词
func (s *Server) registerPrompts() {
	// 四象限分析提示词
	s.server.AddPrompt(&mcp.Prompt{
		Name:        "quadrant_analysis",
		Description: "四象限分析提示词 - 说明如何将任务转换为四象限信息",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "context",
				Description: "分析上下文信息",
				Required:    false,
			},
		},
	}, s.handleQuadrantAnalysisPrompt)

	// 任务创建提示词
	s.server.AddPrompt(&mcp.Prompt{
		Name:        "task_creation",
		Description: "任务创建提示词 - 指导如何创建有效任务",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "title",
				Description: "任务标题",
				Required:    true,
			},
		},
	}, s.handleTaskCreationPrompt)

	// 项目规划提示词
	s.server.AddPrompt(&mcp.Prompt{
		Name:        "project_planning",
		Description: "项目规划提示词 - 指导如何规划项目",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "project_name",
				Description: "项目名称",
				Required:    true,
			},
		},
	}, s.handleProjectPlanningPrompt)

	// AI 拆分指导提示词
	s.server.AddPrompt(&mcp.Prompt{
		Name:        "ai_split_guide",
		Description: "AI 拆分指导提示词 - 指导 AI 如何拆分项目为子任务",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "project_name",
				Description: "项目名称",
				Required:    true,
			},
			{
				Name:        "complexity",
				Description: "项目复杂度 (simple, medium, complex)",
				Required:    false,
			},
		},
	}, s.handleAISplitGuidePrompt)

	// JSON 检索命令提示词
	s.server.AddPrompt(&mcp.Prompt{
		Name:        "json_query_commands",
		Description: "生成 jq/rg 与 shell fallback 的 JSON 检索命令模板（PowerShell + Bash）",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "goal",
				Description: "检索目标（例如：查找微软中学习与成长清单的 completed 任务）",
				Required:    false,
			},
		},
	}, s.handleJSONQueryCommandsPrompt)
}

// registerResources 注册所有资源
func (s *Server) registerResources() {
	// 注册任务资源
	s.server.AddResource(&mcp.Resource{
		URI:         "taskbridge://tasks",
		Name:        "任务列表",
		Description: "所有任务的完整列表",
		MIMEType:    "application/json",
	}, s.handleTasksResource)

	// 注册项目资源
	s.server.AddResource(&mcp.Resource{
		URI:         "taskbridge://projects",
		Name:        "项目列表",
		Description: "所有项目的完整列表",
		MIMEType:    "application/json",
	}, s.handleProjectsResource)

	// 注册提示词资源
	s.server.AddResource(&mcp.Resource{
		URI:         "taskbridge://prompts",
		Name:        "提示词列表",
		Description: "所有内置提示词",
		MIMEType:    "application/json",
	}, s.handlePromptsResource)
}

// GetServer 获取底层 MCP 服务器
func (s *Server) GetServer() *mcp.Server {
	return s.server
}

// GetConfig 获取配置
func (s *Server) GetConfig() *ServerConfig {
	return s.config
}

// GetTools 获取所有工具名称
func (s *Server) GetTools() map[string]bool {
	// 返回工具名称集合
	return map[string]bool{
		"list_tasks":                   true,
		"list_task_lists":              true,
		"create_task":                  true,
		"update_task":                  true,
		"delete_task":                  true,
		"complete_task":                true,
		"analyze_quadrant":             true,
		"analyze_priority":             true,
		"create_project":               true,
		"list_projects":                true,
		"split_project":                true,
		"confirm_project":              true,
		"sync_project":                 true,
		"get_prompt":                   true,
		"sync_push":                    true,
		"sync_pull":                    true,
		"list_providers":               true,
		"get_provider_info":            true,
		"get_provider_config_template": true,
	}
}

// GetPrompts 获取所有提示词名称
func (s *Server) GetPrompts() map[string]bool {
	// 返回提示词名称集合
	return map[string]bool{
		"quadrant_analysis":   true,
		"task_creation":       true,
		"project_planning":    true,
		"ai_split_guide":      true,
		"json_query_commands": true,
	}
}

// toJSON 辅助函数 - 转换为 JSON 字符串
func toJSON(v interface{}) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
