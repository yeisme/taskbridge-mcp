package mcp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yeisme/taskbridge-mcp/internal/config"
	"github.com/yeisme/taskbridge-mcp/pkg/info"
	"github.com/yeisme/taskbridge-mcp/pkg/logger"
)

// TransportType defines the transport type for MCP server.
type TransportType string

const (
	// TransportStdio uses standard input/output for communication.
	TransportStdio TransportType = "stdio"
	// TransportSSE uses Server-Sent Events for communication.
	TransportSSE TransportType = "sse"
	// TransportHTTP uses HTTP for communication.
	TransportHTTP TransportType = "http"
)

// Server represents the MCP server with multi-transport support.
type Server struct {
	mcpServer     *mcp.Server
	config        *config.Config
	transportType TransportType
}

// NewServer creates a new Server instance.
func NewServer(cfg *config.Config, transportType TransportType) *Server {
	// Create MCP server with implementation details
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    info.AppName,
		Version: info.Version,
	}, nil)

	// Register MCP handlers
	registerHandlers(mcpServer)

	return &Server{
		mcpServer:     mcpServer,
		config:        cfg,
		transportType: transportType,
	}
}

// registerHandlers registers MCP protocol handlers.
func registerHandlers(s *mcp.Server) {
	// Register list tasks tool
	taskListTool := &mcp.Tool{
		Name:        "list_tasks",
		Description: "List all available tasks",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of tasks to return",
				},
			},
		},
	}

	mcp.AddTool(s, taskListTool, func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		logger.Infof("Tool 'list_tasks' called with args: %v", args)

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Available tasks from taskbridge-mcp"},
			},
		}, nil, nil
	})

	// Register get task tool
	getTaskTool := &mcp.Tool{
		Name:        "get_task",
		Description: "Get details of a specific task",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{
					"type":        "string",
					"description": "Task ID",
				},
			},
			"required": []string{"task_id"},
		},
	}

	mcp.AddTool(s, getTaskTool, func(ctx context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		logger.Infof("Tool 'get_task' called with args: %v", args)

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Task details"},
			},
		}, nil, nil
	})
}

// Run starts the MCP server with the specified transport type.
func (s *Server) Run(ctx context.Context) error {
	switch s.transportType {
	case TransportStdio:
		return s.runStdio(ctx)
	case TransportSSE:
		return s.runSSE(ctx)
	case TransportHTTP:
		return s.runHTTP(ctx)
	default:
		return fmt.Errorf("unsupported transport type: %s", s.transportType)
	}
}

// runStdio starts the MCP server using stdio transport.
func (s *Server) runStdio(ctx context.Context) error {
	logger.Infof("Starting MCP server with stdio transport")

	// Create stdio transport
	_, serverTransport := mcp.NewInMemoryTransports()

	// Connect server to transport
	session, err := s.mcpServer.Connect(ctx, serverTransport, nil)
	if err != nil {
		logger.Errorf("Failed to connect server to stdio transport: %v", err)
		return err
	}

	// Wait for session to complete
	if err := session.Wait(); err != nil {
		logger.Errorf("Stdio transport error: %v", err)
		return err
	}

	logger.Infof("Stdio transport stopped")

	return nil
}

// runHTTPTransport is a helper function for HTTP-based transports (SSE and HTTP).
func (s *Server) runHTTPTransport(ctx context.Context, handler http.Handler, transportName string) error {
	port := s.config.ServerPort
	addr := fmt.Sprintf(":%d", port)

	logger.Infof("Starting MCP server with %s transport on port %d", transportName, port)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Listen for shutdown signal
	go func() {
		<-ctx.Done()

		if err := httpServer.Shutdown(context.Background()); err != nil {
			logger.Errorf("Error shutting down server: %v", err)
		}
	}()

	logger.Infof("%s transport listening on http://localhost%s", transportName, addr)

	// Start HTTP server
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Errorf("%s transport error: %v", transportName, err)
		return err
	}

	logger.Infof("%s transport stopped", transportName)

	return nil
}

// runSSE starts the MCP server using Server-Sent Events transport.
func (s *Server) runSSE(ctx context.Context) error {
	// Create SSE handler
	handler := mcp.NewSSEHandler(func(request *http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)

	return s.runHTTPTransport(ctx, handler, "SSE")
}

// runHTTP starts the MCP server using HTTP transport.
func (s *Server) runHTTP(ctx context.Context) error {
	// Create streamable HTTP handler
	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)

	return s.runHTTPTransport(ctx, handler, "HTTP")
}
