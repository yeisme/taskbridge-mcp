# TaskBridge 版本与发布流程

当前稳定版本：`v1.0.1`（截至 2026-02-28）

## 目标

- 统一版本来源（CLI 与 MCP 返回同一版本号）
- 标准化发布步骤（tag + GoReleaser）
- 让 AI/Agent 可以通过 MCP 工具读取当前版本与能力
- 提供更多发行产物，降低不同平台用户使用门槛

## MCP 版本能力查询

新增 MCP 工具：`get_server_info`

返回内容包括：

- `name` / `version` / `transport`
- `capabilities`（按功能分组的工具清单）
- `tools`（当前 MCP 可用工具）
- `prompts`（当前可用提示词）
- `resources`（当前可用资源 URI）

建议 Agent 在会话开始阶段先调用一次 `get_server_info`，再决定调用策略。

## 版本号来源

统一在 `pkg/buildinfo/buildinfo.go`：

- `Version`
- `GitCommit`
- `BuildDate`

发布构建由 `.goreleaser.yaml` 通过 `ldflags` 注入真实值。

## 当前打包矩阵

- 二进制归档：
  - Linux: `amd64` / `arm64`（`.tar.gz`）
  - macOS: `amd64` / `arm64`（`.tar.gz`）
  - Windows: `amd64` / `arm64`（`.zip`）
- 源码包：
  - `taskbridge-mcp_source.tar.gz`
- Linux 包管理格式（随 Release 上传）：
  - `.deb` / `.rpm` / `.apk`（`amd64` / `arm64`）

说明：`386` 与 `armv7` 因第三方依赖不兼容已排除，避免发布失败。

## 发布前检查

1. 工作区干净（或确认仅包含本次发布相关变更）
2. 测试通过：`go test ./...`
3. 版本与文档一致（例如 `v1.0.1`）

## 发布步骤（GitHub Release）

1. 提交代码

```bash
git add .
git commit -m "release: v1.0.1"
```

2. 打标签

```bash
git tag -a v1.0.1 -m "Release v1.0.1"
```

3. 推送代码与标签

```bash
git push origin main
git push origin v1.0.1
```

4. 运行 GoReleaser

```bash
goreleaser release --clean
```

说明：

- `release` 会创建/更新 GitHub Release 并上传构建产物。
- 需要提前配置 `GITHUB_TOKEN`（具备 repo 发布权限）。

## 常见问题

- `goreleaser release` 报 `git is in a dirty state`
  - 先提交或清理非本次变更。

- `GitHub token missing`
  - 设置环境变量：`GITHUB_TOKEN=<token>`

- 版本不一致（CLI 与 MCP 显示不同）
  - 检查是否都使用 `pkg/buildinfo`，并确认 ldflags 注入成功。
