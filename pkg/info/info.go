package info

import (
	"fmt"
	"runtime"
)

// 编译时通过 -ldflags 注入的变量.
var (
	// GitCommit 是当前 Git 提交哈希.
	GitCommit = "unknown"
	// GitTag 是当前 Git 标签.
	GitTag = "unknown"
	// BuildDate 是编译时间.
	BuildDate = "unknown"
	// Version 是应用版本.
	Version = "dev"
)

const (
	// AppName 应用程序名称.
	AppName = "taskbridge-mcp"
)

// GetBuildInfo 返回格式化的构建信息.
func GetBuildInfo() string {
	osArch := runtime.GOOS + "/" + runtime.GOARCH
	return fmt.Sprintf("%s %s %s %s", Version, GitCommit, BuildDate, osArch)
}
