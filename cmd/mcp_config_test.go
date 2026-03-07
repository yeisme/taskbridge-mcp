package cmd

import (
	"bytes"
	"net"
	"strings"
	"testing"

	pkgconfig "github.com/yeisme/taskbridge/pkg/config"
	"github.com/yeisme/taskbridge/pkg/paths"
	"github.com/yeisme/taskbridge/pkg/tokenstore"
)

func TestWriteValidationReportAllowsStreamableAndShowsWarnings(t *testing.T) {
	cfg := pkgconfig.DefaultConfig()
	cfg.MCP.Transport = "tcp"

	var out bytes.Buffer
	exitCode := writeValidationReport(&out, cfg.Validate())
	if exitCode != 0 {
		t.Fatalf("expected exitCode=0, got %d, output=%s", exitCode, out.String())
	}
	if !strings.Contains(out.String(), "配置验证通过") {
		t.Fatalf("expected success output: %s", out.String())
	}
	if !strings.Contains(out.String(), "tcp 已废弃") {
		t.Fatalf("expected tcp warning: %s", out.String())
	}
}

func TestWriteValidationReportAcceptsStreamable(t *testing.T) {
	cfg := pkgconfig.DefaultConfig()
	cfg.MCP.Transport = "streamable"
	cfg.MCP.Port = findFreePort(t)
	cfg.MCP.Security.Enabled = true

	var out bytes.Buffer
	exitCode := writeValidationReport(&out, cfg.Validate())
	if exitCode != 0 {
		t.Fatalf("expected exitCode=0 for streamable, got %d, output=%s", exitCode, out.String())
	}
}

func TestBuildDoctorReportDetectsPortConflict(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	cfg := pkgconfig.DefaultConfig()
	cfg.MCP.Transport = "sse"
	cfg.MCP.Port = listener.Addr().(*net.TCPAddr).Port
	cfg.MCP.Security.Enabled = true

	report := buildDoctorReport(cfg)
	if report.Errors == 0 {
		t.Fatalf("expected port conflict error: %#v", report)
	}

	var out bytes.Buffer
	exitCode := writeDoctorReport(&out, report)
	if exitCode != 1 {
		t.Fatalf("expected exitCode=1, got %d, output=%s", exitCode, out.String())
	}
	if !strings.Contains(out.String(), "端口") {
		t.Fatalf("expected port error output: %s", out.String())
	}
}

func TestBuildDoctorReportWarnsOnNetworkTransportWithoutSecurity(t *testing.T) {
	port := findFreePort(t)
	cfg := pkgconfig.DefaultConfig()
	cfg.MCP.Transport = "streamable"
	cfg.MCP.Port = port
	cfg.MCP.Security.Enabled = false

	report := buildDoctorReport(cfg)
	if report.Warnings == 0 {
		t.Fatalf("expected warnings: %#v", report)
	}

	var out bytes.Buffer
	exitCode := writeDoctorReport(&out, report)
	if exitCode != 0 {
		t.Fatalf("expected exitCode=0, got %d, output=%s", exitCode, out.String())
	}
	if !strings.Contains(out.String(), "高风险") {
		t.Fatalf("expected high-risk warning: %s", out.String())
	}
}

func TestBuildDoctorReportChecksProviderTokens(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TASKBRIDGE_HOME", tmpDir)
	if err := tokenstore.Save(paths.GetTokenPath("google"), "google", map[string]string{"access_token": "test"}); err != nil {
		t.Fatalf("save token: %v", err)
	}

	cfg := pkgconfig.DefaultConfig()
	cfg.Providers.Google.Enabled = true
	cfg.Providers.Todoist.Enabled = true

	report := buildDoctorReport(cfg)

	var out bytes.Buffer
	_ = writeDoctorReport(&out, report)
	output := out.String()
	if !strings.Contains(output, "Google token 已就绪") {
		t.Fatalf("expected google token ready: %s", output)
	}
	if !strings.Contains(output, "Todoist 未发现本地 token") {
		t.Fatalf("expected todoist warning: %s", output)
	}
}

func findFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
