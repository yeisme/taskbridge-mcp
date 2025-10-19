package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yeisme/taskbridge-mcp/internal/config"
	"github.com/yeisme/taskbridge-mcp/pkg/info"
	"github.com/yeisme/taskbridge-mcp/pkg/logger"
	"go.uber.org/zap/zapcore"
)

var (
	// Global flags.
	configFile string
	logLevel   string
	verbose    bool
)

// Execute executes the CLI command.
func Execute() error {
	defer func() {
		_ = logger.Sync()
	}()

	return rootCmd.Execute()
}

// rootCmd represents the base command.
var rootCmd = &cobra.Command{
	Use:   "taskbridge-mcp",
	Short: "TaskBridge MCP - The unified bridge between task management systems and AI assistants",
	Long: `TaskBridge MCP (Model Context Protocol)
A unified command line interface for managing integrations with various task management systems.

Supported Platforms:
  • Microsoft To Do
  • Google Tasks
  • Todoist
  • Notion
  • Feishu

Usage Examples:
  taskbridge-mcp server start         # Start MCP server
  taskbridge-mcp adapter list         # List all adapters`,
	Version: info.Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no subcommand, show help
		if len(args) == 0 {
			return cmd.Help()
		}

		return nil
	},
}

// init initializes the root command.
func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file path (default: $HOME/.taskbridge/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug|info|warn|error)")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "verbose output mode")
}

// initConfig initializes configuration and logger.
func initConfig() {
	// Load application config
	cfg, err := config.GetConfig()
	if err != nil {
		logger.Warnf("Failed to load config: %v", err)
	}

	// Parse log level
	if logLevel == "" && cfg != nil {
		logLevel = cfg.LogLevel
	}

	level := parseLogLevel(logLevel)

	// Initialize logger
	logCfg := logger.DefaultLogConfig()
	logCfg.Level = level

	logCfg.Development = verbose
	if verbose {
		logCfg.AddCaller = true
	}

	if err := logger.Init(logCfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// Log initialization info
	logger.Infof("TaskBridge MCP started with log level: %s", logLevel)
	logger.Debugf("Config file: %s", configFile)
	logger.Debugf("Log directory: %s", logger.GetLogDirectory())
}

// parseLogLevel converts string to zapcore.Level.
func parseLogLevel(levelStr string) zapcore.Level {
	switch strings.ToLower(levelStr) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "dpanic":
		return zapcore.DPanicLevel
	case "panic":
		return zapcore.PanicLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}
