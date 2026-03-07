package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfigIncludesMCPExpansionDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MCP.Security.AuthMode != "none" {
		t.Fatalf("unexpected security auth mode: %s", cfg.MCP.Security.AuthMode)
	}
	if !cfg.MCP.Tools.DefaultEnabled {
		t.Fatalf("expected tools.default_enabled=true")
	}
	if cfg.MCP.Observability.Metrics.Path != "/metrics" {
		t.Fatalf("unexpected metrics path: %s", cfg.MCP.Observability.Metrics.Path)
	}
	if cfg.MCP.Reliability.DefaultTimeout != 30*time.Second {
		t.Fatalf("unexpected default timeout: %s", cfg.MCP.Reliability.DefaultTimeout)
	}
	if cfg.MCP.Cache.Backend != "memory" {
		t.Fatalf("unexpected cache backend: %s", cfg.MCP.Cache.Backend)
	}
	if cfg.MCP.Tenant.HeaderKey != "X-TaskBridge-Tenant" {
		t.Fatalf("unexpected tenant header key: %s", cfg.MCP.Tenant.HeaderKey)
	}
	if len(cfg.MCP.Security.AuditMaskFields) == 0 {
		t.Fatalf("expected audit mask defaults")
	}
}

func TestNormalizeTransport(t *testing.T) {
	canonical, deprecated, err := NormalizeTransport("tcp")
	if err != nil {
		t.Fatalf("normalize tcp: %v", err)
	}
	if canonical != "sse" {
		t.Fatalf("expected sse, got %s", canonical)
	}
	if !deprecated {
		t.Fatalf("expected deprecated=true for tcp")
	}
}

func TestValidateCoversExpandedRules(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MCP.Transport = "tcp"
	cfg.MCP.Security.AuthMode = "bad-mode"
	cfg.MCP.Tools.AllowList = []string{"list_tasks"}
	cfg.MCP.Tools.DenyList = []string{"list_tasks"}
	cfg.MCP.Observability.Trace.SampleRate = 2
	cfg.MCP.Reliability.DefaultTimeout = 3 * time.Minute
	cfg.MCP.Reliability.MaxTimeout = 2 * time.Minute
	cfg.MCP.Reliability.Retry.MaxAttempts = 0
	cfg.MCP.Reliability.CircuitBreaker.FailureThreshold = 0

	issues := cfg.Validate()
	if !hasIssue(issues, ValidationLevelWarning, "mcp.transport") {
		t.Fatalf("expected tcp deprecation warning: %#v", issues)
	}
	if !hasIssue(issues, ValidationLevelError, "mcp.security.auth_mode") {
		t.Fatalf("expected invalid auth mode error: %#v", issues)
	}
	if !hasIssue(issues, ValidationLevelError, "mcp.observability.trace.sample_rate") {
		t.Fatalf("expected sample rate error: %#v", issues)
	}
	if !hasIssue(issues, ValidationLevelError, "mcp.tools.allow_list") {
		t.Fatalf("expected allow/deny conflict error: %#v", issues)
	}
}

func TestLoadPreservesDefaultsForMissingNewFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := []byte("mcp:\n  enabled: true\n  intelligence:\n    enabled: true\n")
	if err := os.WriteFile(configPath, content, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.MCP.Security.AuthMode != "none" {
		t.Fatalf("expected default auth_mode, got %s", cfg.MCP.Security.AuthMode)
	}
	if cfg.MCP.Observability.Audit.Output != "stdout" {
		t.Fatalf("expected default audit output, got %s", cfg.MCP.Observability.Audit.Output)
	}
	if cfg.MCP.Cache.MaxEntries != 1000 {
		t.Fatalf("expected default cache max entries, got %d", cfg.MCP.Cache.MaxEntries)
	}
}

func hasIssue(issues []ValidationIssue, level, field string) bool {
	for _, issue := range issues {
		if issue.Level == level && issue.Field == field {
			return true
		}
	}
	return false
}
