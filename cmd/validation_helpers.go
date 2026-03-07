package cmd

import (
	"fmt"
	"io"

	pkgconfig "github.com/yeisme/taskbridge/pkg/config"
)

func writeValidationReport(out io.Writer, issues []pkgconfig.ValidationIssue) int {
	errorCount := 0
	warningCount := 0

	for _, issue := range issues {
		switch issue.Level {
		case pkgconfig.ValidationLevelError:
			errorCount++
		case pkgconfig.ValidationLevelWarning:
			warningCount++
		}
	}

	if errorCount > 0 {
		fmt.Fprintln(out, "❌ 配置验证失败:")
	} else {
		fmt.Fprintln(out, "✅ 配置验证通过")
	}

	if warningCount > 0 {
		fmt.Fprintln(out, "Warnings:")
		for _, issue := range issues {
			if issue.Level != pkgconfig.ValidationLevelWarning {
				continue
			}
			fmt.Fprintf(out, "  - [%s] %s\n", issue.Field, issue.Message)
		}
	}

	if errorCount > 0 {
		fmt.Fprintln(out, "Errors:")
		for _, issue := range issues {
			if issue.Level != pkgconfig.ValidationLevelError {
				continue
			}
			fmt.Fprintf(out, "  - [%s] %s\n", issue.Field, issue.Message)
		}
		return 1
	}

	return 0
}
