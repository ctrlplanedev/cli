package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// commandItem is a resource the user can navigate to
type commandItem struct {
	name     string
	aliases  []string
	resource resourceType
}

var commandItems = []commandItem{
	{name: "deployments", aliases: []string{"deploy", "dep"}, resource: resourceTypeDeployments},
	{name: "resources", aliases: []string{"resource", "res"}, resource: resourceTypeResources},
	{name: "jobs", aliases: []string{"job"}, resource: resourceTypeJobs},
	{name: "environments", aliases: []string{"environment", "env"}, resource: resourceTypeEnvironments},
	{name: "versions", aliases: []string{"version", "ver"}, resource: resourceTypeVersions},
}

type commandModel struct {
	input       textinput.Model
	active      bool
	suggestions []commandItem
	selected    int
}

type commandSubmitMsg struct {
	resource resourceType
}

type commandCancelMsg struct{}

func newCommandModel() commandModel {
	ti := textinput.New()
	ti.Prompt = ":"
	ti.CharLimit = 64
	return commandModel{input: ti}
}

func (m *commandModel) activate() {
	m.active = true
	m.input.SetValue("")
	m.suggestions = commandItems
	m.selected = 0
	m.input.Focus()
}

func (m *commandModel) deactivate() {
	m.active = false
	m.input.Blur()
	m.suggestions = nil
}

func (m *commandModel) updateSuggestions() {
	query := strings.ToLower(strings.TrimSpace(m.input.Value()))
	if query == "" {
		m.suggestions = commandItems
		m.selected = 0
		return
	}

	var matches []commandItem
	for _, item := range commandItems {
		if strings.HasPrefix(item.name, query) {
			matches = append(matches, item)
			continue
		}
		for _, alias := range item.aliases {
			if strings.HasPrefix(alias, query) {
				matches = append(matches, item)
				break
			}
		}
	}
	m.suggestions = matches
	if m.selected >= len(m.suggestions) {
		m.selected = max(0, len(m.suggestions)-1)
	}
}

func (m commandModel) Update(msg tea.Msg) (commandModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if len(m.suggestions) > 0 && m.selected < len(m.suggestions) {
				rt := m.suggestions[m.selected].resource
				m.deactivate()
				return m, func() tea.Msg { return commandSubmitMsg{resource: rt} }
			}
			// Try exact match
			if rt, ok := resolveCommand(m.input.Value()); ok {
				m.deactivate()
				return m, func() tea.Msg { return commandSubmitMsg{resource: rt} }
			}
			m.deactivate()
			return m, func() tea.Msg { return commandCancelMsg{} }
		case "esc":
			m.deactivate()
			return m, func() tea.Msg { return commandCancelMsg{} }
		case "tab", "down":
			if len(m.suggestions) > 0 {
				m.selected = (m.selected + 1) % len(m.suggestions)
			}
			return m, nil
		case "shift+tab", "up":
			if len(m.suggestions) > 0 {
				m.selected = (m.selected - 1 + len(m.suggestions)) % len(m.suggestions)
			}
			return m, nil
		}
	}

	prevVal := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != prevVal {
		m.updateSuggestions()
	}
	return m, cmd
}

func (m commandModel) View() string {
	if !m.active {
		return ""
	}

	var b strings.Builder
	b.WriteString(inputBarStyle.Render(m.input.View()))

	if len(m.suggestions) > 0 {
		b.WriteString("\n")
		for i, s := range m.suggestions {
			style := suggestionStyle
			if i == m.selected {
				style = suggestionActiveStyle
			}
			label := s.name
			if len(s.aliases) > 0 {
				label += lipgloss.NewStyle().Foreground(mutedColor).Render(" (" + strings.Join(s.aliases, ", ") + ")")
			}
			b.WriteString("  ")
			b.WriteString(style.Render(label))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// resolveCommand maps a command string to a resource type
func resolveCommand(cmd string) (resourceType, bool) {
	cmd = strings.TrimSpace(strings.ToLower(cmd))
	for _, item := range commandItems {
		if item.name == cmd {
			return item.resource, true
		}
		for _, alias := range item.aliases {
			if alias == cmd {
				return item.resource, true
			}
		}
	}
	return 0, false
}
