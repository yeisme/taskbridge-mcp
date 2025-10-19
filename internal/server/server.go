package server

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yeisme/taskbridge-mcp/internal/config"
	"github.com/yeisme/taskbridge-mcp/pkg/info"
)

// NewServer creates a new Server instance.
func NewServer(cfg *config.Config) *mcp.Server {
	// Create a server.
	server := mcp.NewServer(&mcp.Implementation{Name: info.AppName, Version: info.Version}, nil)

	return server
}
