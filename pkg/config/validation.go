package config

import (
	"fmt"
	"strings"
)

const (
	// ValidationLevelError 表示阻断性配置错误。
	ValidationLevelError = "error"
	// ValidationLevelWarning 表示非阻断性配置警告。
	ValidationLevelWarning = "warning"
)

// ValidationIssue 描述单条配置校验结果。
type ValidationIssue struct {
	Level   string
	Field   string
	Message string
}

// NormalizeTransport 将 transport 统一为规范值；tcp 作为兼容别名映射到 sse。
func NormalizeTransport(value string) (canonical string, deprecated bool, err error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	switch trimmed {
	case "stdio", "sse", "streamable":
		return trimmed, false, nil
	case "tcp":
		return "sse", true, nil
	case "":
		return "", false, fmt.Errorf("transport is empty")
	default:
		return "", false, fmt.Errorf("unsupported transport: %s", value)
	}
}

// Validate 校验配置并返回 error/warning 列表。
func (c *Config) Validate() []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	if c == nil {
		return append(issues, ValidationIssue{
			Level:   ValidationLevelError,
			Field:   "config",
			Message: "配置为空",
		})
	}

	addIssue := func(level, field, message string) {
		issues = append(issues, ValidationIssue{
			Level:   level,
			Field:   field,
			Message: message,
		})
	}

	if strings.TrimSpace(c.Storage.Type) == "" {
		addIssue(ValidationLevelError, "storage.type", "不能为空")
	}
	if c.Storage.Type == "file" && strings.TrimSpace(c.Storage.Path) == "" {
		addIssue(ValidationLevelError, "storage.path", "不能为空（文件存储模式）")
	}

	if strings.TrimSpace(c.Sync.Mode) == "" {
		addIssue(ValidationLevelError, "sync.mode", "不能为空")
	} else {
		switch strings.ToLower(strings.TrimSpace(c.Sync.Mode)) {
		case "once", "interval", "realtime":
		default:
			addIssue(ValidationLevelError, "sync.mode", fmt.Sprintf("无效值: %s", c.Sync.Mode))
		}
	}

	normalizedTransport := ""
	if c.MCP.Enabled {
		if strings.TrimSpace(c.MCP.Transport) == "" {
			addIssue(ValidationLevelError, "mcp.transport", "不能为空（MCP 启用时）")
		} else {
			canonical, deprecated, err := NormalizeTransport(c.MCP.Transport)
			if err != nil {
				addIssue(ValidationLevelError, "mcp.transport", fmt.Sprintf("无效值: %s", c.MCP.Transport))
			} else {
				normalizedTransport = canonical
				if deprecated {
					addIssue(ValidationLevelWarning, "mcp.transport", "tcp 已废弃，将按兼容别名映射为 sse")
				}
			}
		}

		if normalizedTransport == "sse" || normalizedTransport == "streamable" {
			if c.MCP.Port < 1 || c.MCP.Port > 65535 {
				addIssue(ValidationLevelError, "mcp.port", "仅在 sse/streamable 模式下允许 1-65535")
			}
		}

		if normalizedTransport != "" && normalizedTransport != "stdio" && !c.MCP.Security.Enabled {
			addIssue(ValidationLevelWarning, "mcp.security.enabled", "网络传输模式未启用 security，存在暴露风险")
		}
	}

	switch strings.ToLower(strings.TrimSpace(c.MCP.Security.AuthMode)) {
	case "none", "token", "mutual_tls":
	default:
		addIssue(ValidationLevelError, "mcp.security.auth_mode", fmt.Sprintf("无效值: %s", c.MCP.Security.AuthMode))
	}

	switch strings.ToLower(strings.TrimSpace(c.MCP.Observability.Audit.Output)) {
	case "stdout", "file":
	default:
		addIssue(ValidationLevelError, "mcp.observability.audit.output", fmt.Sprintf("无效值: %s", c.MCP.Observability.Audit.Output))
	}

	switch strings.ToLower(strings.TrimSpace(c.MCP.Cache.Backend)) {
	case "memory", "file":
	default:
		addIssue(ValidationLevelError, "mcp.cache.backend", fmt.Sprintf("无效值: %s", c.MCP.Cache.Backend))
	}

	if c.MCP.Observability.Trace.SampleRate < 0 || c.MCP.Observability.Trace.SampleRate > 1 {
		addIssue(ValidationLevelError, "mcp.observability.trace.sample_rate", "必须在 [0,1] 范围内")
	}

	if c.MCP.Reliability.DefaultTimeout > c.MCP.Reliability.MaxTimeout {
		addIssue(ValidationLevelError, "mcp.reliability.default_timeout", "不能大于 mcp.reliability.max_timeout")
	}
	if c.MCP.Reliability.Retry.MaxAttempts < 1 {
		addIssue(ValidationLevelError, "mcp.reliability.retry.max_attempts", "必须大于等于 1")
	}
	if c.MCP.Reliability.CircuitBreaker.FailureThreshold < 1 {
		addIssue(ValidationLevelError, "mcp.reliability.circuit_breaker.failure_threshold", "必须大于等于 1")
	}

	if c.MCP.Tenant.Enabled && strings.TrimSpace(c.MCP.Tenant.DefaultTenant) == "" {
		addIssue(ValidationLevelError, "mcp.tenant.default_tenant", "tenant 启用时不能为空")
	}

	allowMap := make(map[string]struct{}, len(c.MCP.Tools.AllowList))
	for _, name := range c.MCP.Tools.AllowList {
		trimmed := strings.ToLower(strings.TrimSpace(name))
		if trimmed == "" {
			continue
		}
		allowMap[trimmed] = struct{}{}
	}
	for _, name := range c.MCP.Tools.DenyList {
		trimmed := strings.ToLower(strings.TrimSpace(name))
		if trimmed == "" {
			continue
		}
		if _, ok := allowMap[trimmed]; ok {
			addIssue(ValidationLevelError, "mcp.tools.allow_list", fmt.Sprintf("allow_list 与 deny_list 存在冲突项: %s", trimmed))
		}
	}

	return issues
}
