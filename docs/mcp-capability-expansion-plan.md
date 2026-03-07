# mcp-capability-expansion-plan

## 1. 目标与背景

基于现有配置结构 [`Config`](../pkg/config/config.go) 与 [`MCPConfig`](../pkg/config/config.go)，当前系统已经具备：

- 多传输（stdio/sse/streamable）
- 智能治理（逾期、长期任务、拆分、成就）
- Provider 集成与同步引擎

下一阶段目标是补齐“可运营、可观测、可扩展、可控风险”的 MCP 能力，使其从“功能可用”升级为“工程可持续”。

---

## 2. 现状分析（基于配置层）

在 [`pkg/config/config.go`](../pkg/config/config.go) 中，MCP 相关配置当前聚焦：

- 基础服务启停与传输参数（`enabled`/`transport`/`port`）
- 智能治理参数（`intelligence.*`）

缺口主要在：

1. **安全治理缺失**：缺少鉴权、来源限制、速率限制。
2. **可观测性缺失**：缺少指标、审计日志、追踪配置。
3. **工具治理缺失**：缺少按工具开关、分组、灰度发布。
4. **可靠性策略不完整**：缺少熔断、超时预算、退避策略（MCP 调用层）。
5. **多租户/多工作区能力不足**：缺少租户隔离和配额。
6. **缓存与性能治理不足**：缺少结果缓存与失效策略。

---

## 3. 新增 MCP 功能设计

### 3.1 功能一：MCP 安全与访问控制

新增配置：

- `mcp.security.enabled`
- `mcp.security.auth_mode`（none/token/mutual_tls）
- `mcp.security.tokens`（静态 token 列表或引用）
- `mcp.security.allowed_origins`
- `mcp.security.ip_allowlist`
- `mcp.security.audit_mask_fields`

能力说明：

- 对 MCP 调用入口增加鉴权拦截器。
- 支持 token 轮换（从环境变量读取，配置仅保存 key 引用）。
- 审计日志自动脱敏（如 `apikey`、`clientsecret`）。

---

### 3.2 功能二：MCP 工具注册治理（Tool Governance）

新增配置：

- `mcp.tools.enabled`
- `mcp.tools.default_enabled`
- `mcp.tools.allow_list`
- `mcp.tools.deny_list`
- `mcp.tools.experimental_enabled`
- `mcp.tools.groups.<group_name>`

能力说明：

- 对 [`internal/mcp/handlers.go`](../internal/mcp/handlers.go) 中工具注册流程做统一准入判断。
- 支持“默认关闭 + 白名单启用”模式，降低误暴露风险。
- 支持实验工具灰度（仅指定组可见）。

---

### 3.3 功能三：可观测性（Metrics / Audit / Trace）

新增配置：

- `mcp.observability.metrics.enabled`
- `mcp.observability.metrics.path`
- `mcp.observability.audit.enabled`
- `mcp.observability.audit.output`（stdout/file）
- `mcp.observability.audit.file_path`
- `mcp.observability.trace.enabled`
- `mcp.observability.trace.sample_rate`

能力说明：

- 记录每个工具调用的延迟、成功率、错误码分布。
- 审计日志保留：调用者、工具名、参数摘要、执行时长、结果状态。
- tracing 支持与后续 OpenTelemetry 对接。

---

### 3.4 功能四：可靠性治理（Timeout/Retry/Circuit Breaker）

新增配置：

- `mcp.reliability.default_timeout`
- `mcp.reliability.max_timeout`
- `mcp.reliability.retry.enabled`
- `mcp.reliability.retry.max_attempts`
- `mcp.reliability.retry.backoff`
- `mcp.reliability.circuit_breaker.enabled`
- `mcp.reliability.circuit_breaker.failure_threshold`
- `mcp.reliability.circuit_breaker.half_open_after`

能力说明：

- 在 handler 执行层统一包裹超时控制。
- 对外部 Provider 依赖调用引入重试与熔断。
- 避免单个 Provider 异常拖垮整个 MCP 服务。

---

### 3.5 功能五：缓存与性能治理

新增配置：

- `mcp.cache.enabled`
- `mcp.cache.backend`（memory/file/redis[未来]）
- `mcp.cache.default_ttl`
- `mcp.cache.max_entries`
- `mcp.cache.cacheable_tools`

能力说明：

- 对只读分析类工具（统计、摘要、列表）支持短期缓存。
- 缓存 key 由“工具名 + 参数哈希 + 租户”组成。
- 提供手动失效工具（如 `mcp.cache.invalidate`）。

---

### 3.6 功能六：多租户与配额治理

新增配置：

- `mcp.tenant.enabled`
- `mcp.tenant.default_tenant`
- `mcp.tenant.header_key`
- `mcp.tenant.quotas.<tenant>.qps`
- `mcp.tenant.quotas.<tenant>.daily_calls`
- `mcp.tenant.quotas.<tenant>.max_parallel`

能力说明：

- 适配团队/组织场景的租户隔离。
- 按租户统计调用配额并限流。
- 与审计日志联动，支持按租户追踪问题。

---

## 4. 配置结构建议（Go 结构草案）

建议扩展 [`MCPConfig`](../pkg/config/config.go) 为：

- `Security SecurityConfig`
- `Tools ToolGovernanceConfig`
- `Observability ObservabilityConfig`
- `Reliability ReliabilityConfig`
- `Cache CacheConfig`
- `Tenant TenantConfig`

并在 [`DefaultConfig()`](../pkg/config/config.go) 与 [`setDefaults()`](../pkg/config/config.go) 中补齐默认值，保证向后兼容（旧配置不受影响）。

---

## 5. 分层落地设计

### 5.1 配置层

变更点：[`pkg/config/config.go`](../pkg/config/config.go)

- 新增结构体定义。
- 更新默认值与 `viper` 默认键。
- 增加配置验证入口（建议新增 `Validate()`）。

### 5.2 命令层

候选变更：[`cmd/mcp.go`](../cmd/mcp.go), [`cmd/serve.go`](../cmd/serve.go)

- 新增启动时配置校验输出。
- 增加 `mcp doctor` 子命令（校验 token、端口冲突、配置冲突）。

### 5.3 服务层

候选变更：[`internal/mcp/server.go`](../internal/mcp/server.go)

- 注入中间件链：鉴权 → 限流 → 超时 → 审计 → handler。
- 按配置启用/禁用模块。

### 5.4 Handler 层

候选变更：[`internal/mcp/handlers.go`](../internal/mcp/handlers.go)

- 工具注册前做治理判断。
- 对高成本工具标记 `experimental` 与 `cacheable`。

---

## 6. 兼容性与风险控制

1. **向后兼容**：所有新增项默认关闭或保守值。
2. **渐进启用**：支持按功能模块独立开关。
3. **失败降级**：观测组件失败不影响主流程。
4. **安全兜底**：当 transport=sse/streamable 且 security 未开启时输出高危告警。

---

## 7. 里程碑实施计划

### M1（配置与骨架）

- 扩展配置结构与默认值。
- 增加配置校验与 `mcp doctor`。
- 输出配置迁移示例文档。

当前状态（2026-03-07）：

- 已补齐 `mcp.security/tools/observability/reliability/cache/tenant` 配置骨架与默认值
- 已新增 `pkg/config.Validate()` 与 `NormalizeTransport()`
- 已新增 `taskbridge mcp doctor`
- 已统一 transport 术语为 `stdio/sse/streamable`，旧值 `tcp` 作为兼容别名映射到 `sse`

### M2（安全与可靠性）

- 完成鉴权、限流、超时、重试、熔断中间件。
- 增加关键路径单元测试。

### M3（可观测与缓存）

- 落地 metrics/audit/trace 基础实现。
- 实现只读工具缓存与失效机制。

### M4（多租户与治理完善）

- 完成租户配额与审计联动。
- 完成工具分组和灰度发布策略。

---

## 8. 验收标准

- 配置可正确加载、默认值生效、旧配置无破坏。
- MCP 工具可按白名单精确启停。
- SSE/Streamable 模式下鉴权与限流可观测。
- 关键工具调用具备延迟与错误率指标。
- 外部 Provider 故障下服务具备熔断与恢复能力。

---

## 9. 推荐优先级

1. 安全治理（最高）
2. 可靠性治理
3. 可观测性
4. 工具治理
5. 缓存优化
6. 多租户配额
