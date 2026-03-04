package buildinfo

// 这些变量会在构建/发布时通过 ldflags 注入。
var (
	Version   = "1.0.2"
	GitCommit = "unknown"
	BuildDate = "unknown"
)
