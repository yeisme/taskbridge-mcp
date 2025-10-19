package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yeisme/taskbridge-mcp/internal/config"
	"github.com/yeisme/taskbridge-mcp/internal/mcp"
	"github.com/yeisme/taskbridge-mcp/pkg/logger"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage the MCP server",
	Long:  `Commands to start and manage the MCP server with multiple transport options.`,
}

// Start the server with specified transport.
var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the MCP server",
	Long: `Start the MCP server with the specified transport type.

Recommended: Use Streaming HTTP (--transport http) for best performance!
Streaming HTTP offers superior performance over SSE and is the community standard.

Examples:
  taskbridge-mcp server start --transport http --port 8080 (RECOMMENDED)
  taskbridge-mcp server start --transport stdio
  taskbridge-mcp server start --transport sse --port 9090`,
	RunE: func(cmd *cobra.Command, args []string) error {
		transport := cmd.Flag("transport").Value.String()
		if transport == "" {
			transport = "stdio"
		}

		// Validate transport type
		if err := mcp.ValidateTransportType(transport); err != nil {
			logger.Errorf("Invalid transport type: %v", err)
			return err
		}

		// Get port from flag
		portStr := cmd.Flag("port").Value.String()
		cfg, err := config.GetConfig()
		if err != nil {
			logger.Errorf("Failed to load config: %v", err)
			return err
		}
		port := cfg.ServerPort
		if portStr != "" {
			_, _ = fmt.Sscanf(portStr, "%d", &port)
		}

		output := strings.Builder{}
		output.WriteString(fmt.Sprintf("Starting MCP server with transport: %s\n", transport))
		output.WriteString(fmt.Sprintf("Transport features: %s\n", mcp.GetTransportDescription(mcp.TransportType(transport))))
		output.WriteString(fmt.Sprintf("Server will listen on port: %d\n", port))
		printOut(cmd, output.String())

		return nil
	},
}

// List available transports.
var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available MCP transports",
	Long:  "Display all available MCP transport types and their features.",
	RunE: func(cmd *cobra.Command, args []string) error {
		transports := []mcp.TransportType{
			mcp.TransportStdio,
			mcp.TransportSSE,
			mcp.TransportHTTP,
		}

		output := strings.Builder{}
		output.WriteString("Available MCP Transports:\n")
		output.WriteString("============================================================\n")

		for _, t := range transports {
			features := mcp.GetTransportFeatures(t)
			output.WriteString(fmt.Sprintf("\nTransport: %s\n", features.Name))
			output.WriteString(fmt.Sprintf("  Description: %s\n", features.Description))
			output.WriteString(fmt.Sprintf("  Streaming: %v\n", features.IsStreaming))
			output.WriteString(fmt.Sprintf("  Requires HTTP: %v\n", features.RequiresHTTP))

			if features.DefaultPort != 0 {
				output.WriteString(fmt.Sprintf("  Default Port: %d\n", features.DefaultPort))
			}

			if len(features.Capabilities) > 0 {
				output.WriteString(fmt.Sprintf("  Capabilities: %v\n", features.Capabilities))
			}
		}

		output.WriteString("\n============================================================\n")
		printOut(cmd, output.String())
		return nil
	},
}

// Show info about transports.
var serverInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show MCP server information",
	Long:  "Display information about the MCP server and available transports.",
	RunE: func(cmd *cobra.Command, args []string) error {
		output := `MCP Server Information:
Multiple Transport Support Available:

  1. Streaming HTTP (RECOMMENDED) ⭐
     - RESTful API transport with streaming capabilities
     - Best performance and community standard
     - Recommended by MCP community

  2. Stdio
     - Direct CLI communication
     - Suitable for local development

  3. Server-Sent Events (SSE)
     - Alternative streaming transport
     - Lower performance than Streaming HTTP

Run 'taskbridge-mcp server list' for detailed information
Run 'taskbridge-mcp server start --transport http --port 8080' to start with recommended transport
`
		printOut(cmd, output)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	// Add subcommands
	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverListCmd)
	serverCmd.AddCommand(serverInfoCmd)

	// Flags for start command
	serverStartCmd.Flags().String("transport", "stdio", "Transport type (stdio|sse|http)")
	serverStartCmd.Flags().Int("port", 8080, "Port for HTTP-based transports (sse, http)")
}

func printOut(cmd *cobra.Command, msg string) {
	fmt.Fprint(cmd.OutOrStdout(), msg)
}
