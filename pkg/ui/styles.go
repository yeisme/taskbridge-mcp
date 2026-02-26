// Package ui provides styled CLI output components using lipgloss
package ui

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	// Primary colors
	PrimaryColor   = lipgloss.Color("#7C3AED") // Purple
	SecondaryColor = lipgloss.Color("#3B82F6") // Blue

	// Status colors
	SuccessColor = lipgloss.Color("#10B981") // Green
	ErrorColor   = lipgloss.Color("#EF4444") // Red
	WarningColor = lipgloss.Color("#F59E0B") // Yellow/Amber
	InfoColor    = lipgloss.Color("#6B7280") // Gray

	// UI colors
	BorderColor   = lipgloss.Color("#374151") // Dark gray
	HeaderBgColor = lipgloss.Color("#1F2937") // Darker gray
	AltRowColor   = lipgloss.Color("#1A1A2E") // Alternate row background
)

// Base styles
var (
	// TitleStyle for main titles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor).
			MarginBottom(1).
			Padding(0, 1)

	// HeaderStyle for table headers
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(HeaderBgColor).
			Padding(0, 1)

	// RowStyle for table rows
	RowStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// AltRowStyle for alternating table rows
	AltRowStyle = lipgloss.NewStyle().
			Background(AltRowColor).
			Padding(0, 1)

	// SelectedStyle for selected items
	SelectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#374151")).
			Foreground(lipgloss.Color("#FFFFFF"))

	// CardStyle for card-like containers
	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(1, 2).
			Margin(1, 0)
)

// Status styles
var (
	// SuccessStyle for success messages
	SuccessStyle = lipgloss.NewStyle().
			Foreground(SuccessColor).
			Bold(true)

	// ErrorStyle for error messages
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true)

	// WarningStyle for warning messages
	WarningStyle = lipgloss.NewStyle().
			Foreground(WarningColor)

	// InfoStyle for informational messages
	InfoStyle = lipgloss.NewStyle().
			Foreground(InfoColor)

	// EnabledStyle for enabled status
	EnabledStyle = SuccessStyle

	// DisabledStyle for disabled status
	DisabledStyle = lipgloss.NewStyle().
			Foreground(InfoColor)
)

// Label and value styles
var (
	// LabelStyle for field labels
	LabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Bold(true)

	// ValueStyle for field values
	ValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB"))
)

// Status text helpers
var (
	StatusEnabled       = SuccessStyle.Render("✅ 已启用")
	StatusDisabled      = DisabledStyle.Render("❌ 未启用")
	StatusConnected     = SuccessStyle.Render("✅ 已连接")
	StatusNotConfigured = DisabledStyle.Render("❌ 未配置")
	StatusExpired       = WarningStyle.Render("⚠️ 已过期")
)

// Success prints a success message
func Success(msg string) string {
	return SuccessStyle.Render("✓ " + msg)
}

// Error prints an error message
func Error(msg string) string {
	return ErrorStyle.Render("✗ " + msg)
}

// Warning prints a warning message
func Warning(msg string) string {
	return WarningStyle.Render("⚠ " + msg)
}

// Info prints an informational message
func Info(msg string) string {
	return InfoStyle.Render("ℹ " + msg)
}

// LabelValue formats a label-value pair
func LabelValue(label, value string) string {
	return LabelStyle.Render(label+": ") + ValueStyle.Render(value)
}

// Bold returns bold text
func Bold(text string) string {
	return lipgloss.NewStyle().Bold(true).Render(text)
}

// Dim returns dimmed text
func Dim(text string) string {
	return lipgloss.NewStyle().Faint(true).Render(text)
}

// Highlight returns highlighted text
func Highlight(text string) string {
	return lipgloss.NewStyle().
		Foreground(PrimaryColor).
		Bold(true).
		Render(text)
}
