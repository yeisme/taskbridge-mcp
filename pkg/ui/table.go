package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/mattn/go-runewidth"
)

// Table represents a styled table
type Table struct {
	inner   *table.Table
	headers []string
	rows    [][]string
}

// NewTable creates a new table with headers
func NewTable(headers ...string) *Table {
	t := &Table{
		headers: headers,
		inner: table.New().
			Headers(headers...).
			Border(lipgloss.NormalBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(BorderColor)),
	}
	return t
}

// AddRow adds a row to the table
func (t *Table) AddRow(cells ...string) *Table {
	t.rows = append(t.rows, cells)
	return t
}

// SetWidth sets the table width
func (t *Table) SetWidth(width int) *Table {
	t.inner = t.inner.Width(width)
	return t
}

// Render renders the table to a string
func (t *Table) Render() string {
	// Add all rows
	for _, row := range t.rows {
		t.inner.Row(row...)
	}
	return t.inner.Render()
}

// String implements Stringer interface
func (t *Table) String() string {
	return t.Render()
}

// StringWidth calculates the display width of a string (supporting CJK characters)
func StringWidth(s string) int {
	return runewidth.StringWidth(s)
}

// PadRight pads a string on the right to a given display width
func PadRight(s string, width int) string {
	currentWidth := StringWidth(s)
	if currentWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-currentWidth)
}

// PadLeft pads a string on the left to a given display width
func PadLeft(s string, width int) string {
	currentWidth := StringWidth(s)
	if currentWidth >= width {
		return s
	}
	return strings.Repeat(" ", width-currentWidth) + s
}

// PadCenter centers a string within a given display width
func PadCenter(s string, width int) string {
	currentWidth := StringWidth(s)
	if currentWidth >= width {
		return s
	}
	left := (width - currentWidth) / 2
	right := width - currentWidth - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// SimpleTable creates a simple aligned table without borders
type SimpleTable struct {
	columns    []Column
	rows       [][]string
	showHeader bool
}

// Column defines a table column
type Column struct {
	Header     string
	Width      int
	AlignLeft  bool
	AlignRight bool
}

// NewSimpleTable creates a simple table
func NewSimpleTable(columns ...Column) *SimpleTable {
	return &SimpleTable{
		columns:    columns,
		showHeader: true,
	}
}

// HideHeader hides the table header
func (t *SimpleTable) HideHeader() *SimpleTable {
	t.showHeader = false
	return t
}

// AddRow adds a row to the simple table
func (t *SimpleTable) AddRow(cells ...string) *SimpleTable {
	t.rows = append(t.rows, cells)
	return t
}

// Render renders the simple table
func (t *SimpleTable) Render() string {
	var sb strings.Builder

	// Calculate column widths if not specified
	widths := make([]int, len(t.columns))
	for i, col := range t.columns {
		if col.Width > 0 {
			widths[i] = col.Width
		} else {
			widths[i] = StringWidth(col.Header)
		}
	}

	// Update widths based on content
	for _, row := range t.rows {
		for i, cell := range row {
			if i < len(widths) {
				w := StringWidth(cell)
				if w > widths[i] {
					widths[i] = w
				}
			}
		}
	}

	// Render header
	if t.showHeader {
		for i := range t.columns {
			if i < len(widths) {
				header := t.columns[i].Header
				if t.columns[i].AlignRight {
					sb.WriteString(PadLeft(header, widths[i]))
				} else {
					sb.WriteString(PadRight(header, widths[i]))
				}
				if i < len(t.columns)-1 {
					sb.WriteString("  ") // Column gap
				}
			}
		}
		sb.WriteString("\n")

		// Header separator
		for i := range t.columns {
			if i < len(widths) {
				sb.WriteString(strings.Repeat("─", widths[i]))
				if i < len(t.columns)-1 {
					sb.WriteString("  ")
				}
			}
		}
		sb.WriteString("\n")
	}

	// Render rows
	for _, row := range t.rows {
		for i, cell := range row {
			if i < len(t.columns) && i < len(widths) {
				if t.columns[i].AlignRight {
					sb.WriteString(PadLeft(cell, widths[i]))
				} else {
					sb.WriteString(PadRight(cell, widths[i]))
				}
				if i < len(t.columns)-1 {
					sb.WriteString("  ")
				}
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
