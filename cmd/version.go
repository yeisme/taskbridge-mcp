package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	// 版本信息 - 编译时通过 ldflags 注入
	version   = "dev"
	gitCommit = "unknown"
	buildDate = "unknown"
)

// versionCmd 版本命令
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Long: `显示 TaskBridge 的版本信息。

示例:
  taskbridge version
  taskbridge version --json`,
	Run: runVersion,
}

var versionJSON bool

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolVar(&versionJSON, "json", false, "以 JSON 格式输出")
}

func runVersion(_ *cobra.Command, _ []string) {
	if versionJSON {
		fmt.Printf(`{
  "version": "%s",
  "git_commit": "%s",
  "build_date": "%s",
  "go_version": "%s",
  "platform": "%s/%s"
}`, version, gitCommit, buildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		fmt.Println()
		return
	}

	fmt.Println()
	fmt.Println("  TaskBridge - 连接 AI 与 Todo 软件的桥梁")
	fmt.Println()
	fmt.Printf("  版本:       %s\n", version)
	fmt.Printf("  Git 提交:   %s\n", gitCommit)
	fmt.Printf("  构建时间:   %s\n", buildDate)
	fmt.Printf("  Go 版本:    %s\n", runtime.Version())
	fmt.Printf("  平台:       %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()
}
