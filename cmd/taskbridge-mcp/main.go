package main

import (
	"fmt"
	"os"

	"github.com/yeisme/taskbridge-mcp/internal/cli"
)

func main() {
	// Execute CLI
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
