package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type tableModel struct {
	columns    []string
	rows       []tableRow
	allRows    []tableRow // unfiltered
	cursor     int
	offset     int // scroll offset for visible window
	total      int
	loading    bool
	err        error
	width      int
	height     int
	filterText string
}

func newTableModel(columns []string) tableModel {
	return tableModel{
		columns: columns,
		loading: true,
	}
}

func (m *tableModel) setData(rows []tableRow, total int) {
	m.allRows = rows
	m.total = total
	m.loading = false
	m.err = nil
	m.applyFilter()
}

func (m *tableModel) setError(err error) {
	m.err = err
	m.loading = false
}

func (m *tableModel) applyFilter() {
	if m.filterText == "" {
		m.rows = m.allRows
	} else {
		filter := strings.ToLower(m.filterText)
		m.rows = nil
		for _, row := range m.allRows {
			for _, col := range row.cols {
				if strings.Contains(strings.ToLower(col), filter) {
					m.rows = append(m.rows, row)
					break
				}
			}
		}
	}
	if m.cursor >= len(m.rows) {
		m.cursor = max(0, len(m.rows)-1)
	}
	m.offset = 0
}

func (m *tableModel) moveUp() {
	if m.cursor > 0 {
		m.cursor--
		if m.cursor < m.offset {
			m.offset = m.cursor
		}
	}
}

func (m *tableModel) moveDown() {
	if m.cursor < len(m.rows)-1 {
		m.cursor++
		visibleRows := m.visibleRowCount()
		if m.cursor >= m.offset+visibleRows {
			m.offset = m.cursor - visibleRows + 1
		}
	}
}

func (m tableModel) visibleRowCount() int {
	// Reserve: info header(~5) + border(2) + table header(1) + bottom indicator(1) + filter(1)
	available := m.height - 10
	if available < 1 {
		available = 1
	}
	return available
}

func (m tableModel) selectedRow() *tableRow {
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		return &m.rows[m.cursor]
	}
	return nil
}

func (m tableModel) renderRows(statusColIndex int) string {
	var b strings.Builder

	if m.loading {
		b.WriteString(lipgloss.NewStyle().Foreground(warningColor).Padding(0, 2).Render("Loading..."))
		b.WriteString("\n")
		return b.String()
	}

	if m.err != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(dangerColor).Padding(0, 2).Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n")
		return b.String()
	}

	// Column widths
	colWidths := m.calculateColumnWidths()

	// Header
	var headerParts []string
	for i, col := range m.columns {
		w := colWidths[i]
		headerParts = append(headerParts, padRight(col, w))
	}
	b.WriteString(headerStyle.Render(strings.Join(headerParts, " ")))
	b.WriteString("\n")

	// Rows
	visibleRows := m.visibleRowCount()
	end := m.offset + visibleRows
	if end > len(m.rows) {
		end = len(m.rows)
	}

	for i := m.offset; i < end; i++ {
		row := m.rows[i]
		var parts []string
		for ci, col := range row.cols {
			w := colWidths[ci]
			padded := padRight(col, w)
			if i == m.cursor {
				parts = append(parts, padded)
			} else if ci == statusColIndex && statusColIndex >= 0 {
				parts = append(parts, colorStatus(padded, col))
			} else {
				parts = append(parts, lipgloss.NewStyle().Foreground(textColor).Render(padded))
			}
		}
		line := strings.Join(parts, " ")

		if i == m.cursor {
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	// Pad remaining space
	for i := end - m.offset; i < visibleRows; i++ {
		b.WriteString("\n")
	}

	return b.String()
}

func colorStatus(display, status string) string {
	switch status {
	case "successful", "ready":
		return statusSuccessStyle.Render(display)
	case "failure", "failed", "rejected":
		return statusDangerStyle.Render(display)
	case "inProgress", "building", "pending":
		return statusWarningStyle.Render(display)
	default:
		return statusMutedStyle.Render(display)
	}
}

func (m tableModel) calculateColumnWidths() []int {
	widths := make([]int, len(m.columns))

	// Start with header widths
	for i, col := range m.columns {
		widths[i] = len(col)
	}

	// Expand based on data
	for _, row := range m.rows {
		for i, col := range row.cols {
			if i < len(widths) && len(col) > widths[i] {
				widths[i] = len(col)
			}
		}
	}

	// Cap each column to a reasonable max
	maxCol := m.width / max(len(m.columns), 1)
	if maxCol < 10 {
		maxCol = 10
	}
	for i := range widths {
		if widths[i] > maxCol {
			widths[i] = maxCol
		}
	}

	return widths
}

func padRight(s string, w int) string {
	if len(s) >= w {
		if w > 3 {
			return s[:w-3] + "..."
		}
		return s[:w]
	}
	return s + strings.Repeat(" ", w-len(s))
}
