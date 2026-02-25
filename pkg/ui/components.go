package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Card creates a card-style container for displaying information
func Card(title string, content []string) string {
	var sb strings.Builder

	// Title style
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		Padding(0, 1)

	// Content style
	contentStyle := lipgloss.NewStyle().
		Padding(0, 2)

	// Card border style
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Padding(1, 2).
		MarginTop(1)

	// Build title
	if title != "" {
		sb.WriteString(titleStyle.Render(title))
		sb.WriteString("\n")
	}

	// Build content
	for _, line := range content {
		sb.WriteString(contentStyle.Render(line))
		sb.WriteString("\n")
	}

	return cardStyle.Render(sb.String())
}

// KeyValueCard creates a card with key-value pairs
func KeyValueCard(title string, pairs map[string]string) string {
	var lines []string
	for label, value := range pairs {
		lines = append(lines, LabelValue(label, value))
	}
	return Card(title, lines)
}

// Section creates a section with a header
func Section(header string, content string) string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		MarginTop(1).
		MarginBottom(1)

	var sb strings.Builder
	sb.WriteString(headerStyle.Render(header))
	sb.WriteString("\n")
	sb.WriteString(content)
	return sb.String()
}

// List creates a bulleted list
func List(items []string) string {
	var sb strings.Builder
	bulletStyle := lipgloss.NewStyle().Foreground(PrimaryColor)
	itemStyle := lipgloss.NewStyle().PaddingLeft(2)

	for _, item := range items {
		sb.WriteString(bulletStyle.Render("• "))
		sb.WriteString(itemStyle.Render(item))
		sb.WriteString("\n")
	}
	return sb.String()
}

// CheckList creates a checklist with check/cross marks
func CheckList(items []CheckItem) string {
	var sb strings.Builder
	for _, item := range items {
		var mark string
		if item.Checked {
			mark = SuccessStyle.Render("✅ ")
		} else {
			mark = DisabledStyle.Render("❌ ")
		}
		sb.WriteString(mark)
		sb.WriteString(item.Text)
		sb.WriteString("\n")
	}
	return sb.String()
}

// CheckItem represents a checklist item
type CheckItem struct {
	Text    string
	Checked bool
}

// Header creates a styled header
func Header(text string) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(PrimaryColor).
		Padding(0, 2).
		MarginBottom(1)
	return style.Render(" " + text + " ")
}

// Divider creates a horizontal divider
func Divider() string {
	style := lipgloss.NewStyle().
		Foreground(BorderColor)
	return style.Render(strings.Repeat("─", 50))
}

// KeyValue creates aligned key-value pairs
func KeyValue(items [][2]string, keyWidth int) string {
	var sb strings.Builder
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF")).
		Width(keyWidth)
	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E7EB"))

	for _, item := range items {
		sb.WriteString(keyStyle.Render(item[0]))
		sb.WriteString(valueStyle.Render(item[1]))
		sb.WriteString("\n")
	}
	return sb.String()
}

// StatusBadge creates a colored status badge
func StatusBadge(status string) string {
	var style lipgloss.Style
	var text string

	switch strings.ToLower(status) {
	case "enabled", "connected", "active", "valid", "success":
		style = lipgloss.NewStyle().
			Foreground(SuccessColor).
			Bold(true)
		text = "✅ " + status
	case "disabled", "inactive", "not configured":
		style = lipgloss.NewStyle().
			Foreground(InfoColor)
		text = "❌ " + status
	case "expired", "warning":
		style = lipgloss.NewStyle().
			Foreground(WarningColor)
		text = "⚠️ " + status
	case "error", "failed":
		style = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true)
		text = "❌ " + status
	default:
		style = lipgloss.NewStyle()
		text = status
	}

	return style.Render(text)
}

// ProviderCard creates a styled provider information card
func ProviderCard(name, displayName, description, authType, status string, capabilities []CheckItem) string {
	var sb strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(PrimaryColor).
		Padding(0, 2).
		MarginBottom(1)
	sb.WriteString(headerStyle.Render(" 📋 " + displayName + " "))
	sb.WriteString("\n\n")

	// Info
	infoStyle := lipgloss.NewStyle().PaddingLeft(2)
	sb.WriteString(infoStyle.Render(LabelValue("名称", name)))
	sb.WriteString("\n")
	sb.WriteString(infoStyle.Render(LabelValue("描述", description)))
	sb.WriteString("\n")
	sb.WriteString(infoStyle.Render(LabelValue("认证方式", authType)))
	sb.WriteString("\n")
	sb.WriteString(infoStyle.Render(LabelValue("状态", StatusBadge(status))))
	sb.WriteString("\n\n")

	// Capabilities header
	capHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(SecondaryColor).
		PaddingLeft(2)
	sb.WriteString(capHeaderStyle.Render("支持的功能:"))
	sb.WriteString("\n")

	// Capabilities list
	for _, cap := range capabilities {
		var mark string
		if cap.Checked {
			mark = SuccessStyle.Render("  ✅ ")
		} else {
			mark = DisabledStyle.Render("  ❌ ")
		}
		sb.WriteString(mark)
		sb.WriteString(cap.Text)
		sb.WriteString("\n")
	}

	// Wrap in card
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Padding(1, 1).
		MarginTop(1)

	return cardStyle.Render(sb.String())
}
