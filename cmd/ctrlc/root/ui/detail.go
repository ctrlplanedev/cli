package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

type detailModel struct {
	title    string
	rawItem  interface{}
	viewport viewport.Model
	content  string
	active   bool
	width    int
	height   int
}

func newDetailModel(title string, rawItem interface{}, width, height int) detailModel {
	content := renderYAML(rawItem)

	vp := viewport.New(width, height-4)
	vp.SetContent(content)

	return detailModel{
		title:    title,
		rawItem:  rawItem,
		viewport: vp,
		content:  content,
		active:   true,
		width:    width,
		height:   height,
	}
}

func (m *detailModel) setSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width
	m.viewport.Height = height - 4
}

func (m detailModel) View() string {
	var b strings.Builder

	// Title bar
	title := tableTitleStyle.Render(fmt.Sprintf(" Describe: %s ", m.title))
	b.WriteString(title)
	b.WriteString("\n")

	// Viewport content
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Status
	info := fmt.Sprintf(" %d%% ", int(m.viewport.ScrollPercent()*100))
	b.WriteString(helpStyle.Render(info))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  ↑/k up  ↓/j down  esc close  q quit"))

	return b.String()
}

func renderYAML(item interface{}) string {
	data, err := yaml.Marshal(item)
	if err != nil {
		return fmt.Sprintf("Error rendering: %v", err)
	}

	// Syntax-highlight the YAML
	lines := strings.Split(string(data), "\n")
	var result []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			result = append(result, "")
			continue
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			key := line[:idx]
			value := line[idx:]
			styled := detailKeyStyle.Render(key) + lipgloss.NewStyle().Foreground(textColor).Render(value)
			result = append(result, styled)
		} else {
			result = append(result, lipgloss.NewStyle().Foreground(textColor).Render(line))
		}
	}
	return strings.Join(result, "\n")
}
