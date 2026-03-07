package cmd

import (
	"fmt"
	"io"
	"net"
	"strings"

	pkgconfig "github.com/yeisme/taskbridge/pkg/config"
	"github.com/yeisme/taskbridge/pkg/paths"
	"github.com/yeisme/taskbridge/pkg/tokenstore"
)

type doctorFinding struct {
	Level   string
	Message string
}

type doctorReport struct {
	Configuration []doctorFinding
	Transport     []doctorFinding
	Providers     []doctorFinding
	Errors        int
	Warnings      int
}

func resolveTransportForDisplay(value string) (string, []string, error) {
	canonical, deprecated, err := pkgconfig.NormalizeTransport(value)
	if err != nil {
		return "", nil, err
	}

	warnings := make([]string, 0, 1)
	if deprecated {
		warnings = append(warnings, "transport=tcp 已废弃，已按兼容别名映射为 sse")
	}

	return canonical, warnings, nil
}

func buildDoctorReport(cfg *pkgconfig.Config) doctorReport {
	report := doctorReport{
		Configuration: make([]doctorFinding, 0),
		Transport:     make([]doctorFinding, 0),
		Providers:     make([]doctorFinding, 0),
	}

	addFinding := func(target *[]doctorFinding, level, message string) {
		*target = append(*target, doctorFinding{Level: level, Message: message})
		switch level {
		case pkgconfig.ValidationLevelError:
			report.Errors++
		case pkgconfig.ValidationLevelWarning:
			report.Warnings++
		}
	}

	for _, issue := range cfg.Validate() {
		addFinding(&report.Configuration, issue.Level, fmt.Sprintf("[%s] %s", issue.Field, issue.Message))
	}

	transport, transportWarnings, err := resolveTransportForDisplay(cfg.MCP.Transport)
	if err != nil {
		addFinding(&report.Transport, pkgconfig.ValidationLevelError, fmt.Sprintf("无法解析 transport: %v", err))
	} else {
		addFinding(&report.Transport, "info", fmt.Sprintf("当前 transport: %s", transport))
		for _, warning := range transportWarnings {
			addFinding(&report.Transport, pkgconfig.ValidationLevelWarning, warning)
		}

		if transport == "sse" || transport == "streamable" {
			if cfg.MCP.Port < 1 || cfg.MCP.Port > 65535 {
				addFinding(&report.Transport, pkgconfig.ValidationLevelError, fmt.Sprintf("端口 %d 非法，需为 1-65535", cfg.MCP.Port))
			} else if isPortAvailable(cfg.MCP.Port) {
				addFinding(&report.Transport, "info", fmt.Sprintf("端口 %d 可用", cfg.MCP.Port))
			} else {
				addFinding(&report.Transport, pkgconfig.ValidationLevelError, fmt.Sprintf("端口 %d 已被占用", cfg.MCP.Port))
			}
			if !cfg.MCP.Security.Enabled {
				addFinding(&report.Transport, pkgconfig.ValidationLevelWarning, "高风险：网络传输模式未启用 security")
			}
		}
	}

	if strings.EqualFold(strings.TrimSpace(cfg.MCP.Observability.Audit.Output), "file") &&
		strings.TrimSpace(cfg.MCP.Observability.Audit.FilePath) == "" {
		addFinding(&report.Configuration, pkgconfig.ValidationLevelError, "[mcp.observability.audit.file_path] audit output=file 时不能为空")
	}

	if strings.EqualFold(strings.TrimSpace(cfg.MCP.Cache.Backend), "file") {
		addFinding(&report.Configuration, pkgconfig.ValidationLevelWarning, "[mcp.cache.backend] M1 仅配置预留，运行时尚未启用 file cache")
	}

	for _, finding := range providerDoctorFindings(cfg) {
		addFinding(&report.Providers, finding.Level, finding.Message)
	}

	return report
}

func isPortAvailable(port int) bool {
	if port < 1 || port > 65535 {
		return false
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

func providerDoctorFindings(cfg *pkgconfig.Config) []doctorFinding {
	findings := make([]doctorFinding, 0)

	checkTokenProvider := func(enabled bool, providerName, displayName string) {
		if !enabled {
			return
		}
		hasToken, err := tokenstore.Has(paths.GetTokenPath(providerName), providerName)
		switch {
		case err != nil:
			findings = append(findings, doctorFinding{
				Level:   pkgconfig.ValidationLevelWarning,
				Message: fmt.Sprintf("%s token 检查失败: %v", displayName, err),
			})
		case !hasToken:
			findings = append(findings, doctorFinding{
				Level:   pkgconfig.ValidationLevelWarning,
				Message: fmt.Sprintf("%s 未发现本地 token", displayName),
			})
		default:
			findings = append(findings, doctorFinding{
				Level:   "info",
				Message: fmt.Sprintf("%s token 已就绪", displayName),
			})
		}
	}

	checkTokenProvider(cfg.Providers.Google.Enabled, "google", "Google")
	checkTokenProvider(cfg.Providers.Microsoft.Enabled, "microsoft", "Microsoft")
	checkTokenProvider(cfg.Providers.Todoist.Enabled, "todoist", "Todoist")
	checkTokenProvider(cfg.Providers.TickTick.Enabled, "ticktick", "TickTick")
	checkTokenProvider(cfg.Providers.Dida.Enabled, "dida", "Dida")

	if cfg.Providers.Feishu.Enabled {
		if strings.TrimSpace(cfg.Providers.Feishu.AppID) == "" || strings.TrimSpace(cfg.Providers.Feishu.AppSecret) == "" {
			findings = append(findings, doctorFinding{
				Level:   pkgconfig.ValidationLevelWarning,
				Message: "Feishu 已启用但缺少 appid/appsecret",
			})
		} else {
			findings = append(findings, doctorFinding{
				Level:   "info",
				Message: "Feishu appid/appsecret 已配置",
			})
		}
	}

	return findings
}

func writeDoctorReport(out io.Writer, report doctorReport) int {
	writeSection := func(title string, findings []doctorFinding) {
		fmt.Fprintf(out, "%s\n", title)
		if len(findings) == 0 {
			fmt.Fprintln(out, "  - OK")
			return
		}
		for _, finding := range findings {
			prefix := "INFO"
			switch finding.Level {
			case pkgconfig.ValidationLevelError:
				prefix = "ERROR"
			case pkgconfig.ValidationLevelWarning:
				prefix = "WARN"
			}
			fmt.Fprintf(out, "  - [%s] %s\n", prefix, finding.Message)
		}
	}

	writeSection("Configuration", report.Configuration)
	writeSection("Transport", report.Transport)
	writeSection("Providers", report.Providers)
	fmt.Fprintln(out, "Summary")
	fmt.Fprintf(out, "  - errors: %d\n", report.Errors)
	fmt.Fprintf(out, "  - warnings: %d\n", report.Warnings)

	if report.Errors > 0 {
		return 1
	}
	return 0
}
