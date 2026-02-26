# MCP 多传输方式支持方案

## 背景

当前 TaskBridge MCP 服务只支持 stdio 传输方式，用户希望支持更多传输方式以便于不同场景使用。

## MCP 传输方式

根据 MCP 规范和 go-sdk 支持，有以下传输方式：

### 1. stdio（已支持）

- 通过标准输入/输出通信
- 适用于本地进程间通信
- Claude Desktop、VSCode 等使用此方式

### 2. SSE (Server-Sent Events)

- 通过 HTTP SSE 进行通信
- 客户端通过 HTTP POST 发送请求
- 服务器通过 SSE 推送消息
- 适用于需要远程访问的场景

### 3. Streamable HTTP（新规范）

- MCP 2024-11-05 规范引入的新传输方式
- 单向 HTTP 请求/响应模式
- 更简单的实现

## go-sdk 支持情况

```go
// go-sdk 提供的传输类型
mcp.StdioTransport{}      // stdio 传输
mcp.NewSSEServer()// SSE 服务器
```

## 实现方案

### 1. 更新 internal/mcp/server.go

```go
// Start 启动 MCP 服务
func (s *Server) Start(ctx context.Context) error {
	switch s.config.Transport {
	case "stdio":
		return s.startStdio(ctx)
	case "sse":
		return s.startSSE(ctx)
	case "inmemory":
		return s.startInMemory(ctx)
	default:
		return fmt.Errorf("unsupported transport: %s", s.config.Transport)
	}
}

// startSSE 启动 SSE 传输
func (s *Server) startSSE(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.config.Port)

	// 创建 SSE 服务器
sseServer := mcp.NewSSEServer(s.server)

	// 启动 HTTP 服务器
	http.Handle("/sse", sseServer)
	http.Handle("/message", sseServer)

	server := &http.Server{
		Addr:    addr,
		Handler: http.DefaultServeMux,
	}

	// 启动服务器
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "SSE server error: %v\n", err)
		}
	}()

	// 等待上下文取消
	<-ctx.Done()
	return server.Shutdown(ctx)
}
```

### 2. 更新 cmd/mcp.go

```go
mcpStartCmd.Flags().StringVar(&mcpTransport, "transport", "stdio",
	"传输方式 (stdio, sse)")
mcpStartCmd.Flags().IntVarP(&mcpPort, "port", "p", 8080,
	"SSE/HTTP 端口（仅 sse 模式）")
```

### 3. 传输方式选择指南

| 传输方式 | 使用场景                 | 端口 |
| -------- | ------------------------ | ---- |
| stdio    | Claude Desktop、本地工具 | 无需 |
| sse      | Web 客户端、远程访问     | 8080 |

## 使用示例

### stdio 模式

```bash
taskbridge mcp start
# 或明确指定
taskbridge mcp start --transport stdio
```

### SSE 模式

```bash
taskbridge mcp start --transport sse --port 8080
```

## 实施步骤

1. 更新 [`internal/mcp/server.go`](internal/mcp/server.go) 添加 SSE 传输支持
2. 更新 [`cmd/mcp.go`](cmd/mcp.go) 移除 tcp 限制，添加 sse 支持
3. 更新帮助文档和命令说明
4. 测试两种传输模式
