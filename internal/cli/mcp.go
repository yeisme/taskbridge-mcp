package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/yeisme/taskbridge-mcp/internal/config"
	"github.com/yeisme/taskbridge-mcp/internal/mcp"
	"github.com/yeisme/taskbridge-mcp/pkg/logger"
)

var (
	// Transport type flag.
	transportType string

	mcpCmd = &cobra.Command{
		Use:   "mcp",
		Short: "Manage the MCP server",
		Long:  `Commands to start and stop the MCP server with support for multiple transport types.`,
	}

	// Start subcommand.
	startCmd = &cobra.Command{
		Use:   "start",
		Short: "Start the MCP server",
		Long: `Start the MCP server with the specified transport type.
		
Supported transport types:
  - stdio: Standard input/output (default for CLI)
  - sse: Server-Sent Events (HTTP-based)
  - http: HTTP JSON-RPC`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.GetConfig()
			if err != nil {
				logger.Errorf("Failed to load config: %v", err)
				return err
			}

			// Validate transport type
			if transportType == "" {
				transportType = "stdio"
			}

			transport := mcp.TransportType(transportType)

			logger.Infof("Starting MCP server with transport: %s", transportType)

			// Create and run server
			server := mcp.NewServer(cfg, transport)

			// Create context with cancel
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Handle shutdown gracefully (Ctrl+C)
			sigChan := make(chan struct{})
			go func() {
				select {
				case <-cmd.Context().Done():
					cancel()
					sigChan <- struct{}{}
				}
			}()

			if err := server.Run(ctx); err != nil {
				logger.Errorf("Server error: %v", err)
				return err
			}

			return nil
		},
	}

	// List subcommand.
	listCmd = &cobra.Command{
		Use:   "transports",
		Short: "List available MCP transports",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Infof("Available MCP transports:")
			logger.Infof("  - stdio: Standard input/output")
			logger.Infof("  - sse: Server-Sent Events")
			logger.Infof("  - http: HTTP JSON-RPC")
			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(mcpCmd)

	// Add subcommands
	mcpCmd.AddCommand(startCmd)
	mcpCmd.AddCommand(listCmd)

	// Flags for start command
	startCmd.Flags().StringVar(&transportType, "transport", "stdio", "Transport type (stdio|sse|http)")
}
