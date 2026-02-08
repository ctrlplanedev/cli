package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type filterModel struct {
	input  textinput.Model
	active bool
}

type filterSubmitMsg struct {
	value string
}

type filterCancelMsg struct{}

type filterChangedMsg struct {
	value string
}

func newFilterModel() filterModel {
	ti := textinput.New()
	ti.Prompt = "/"
	ti.CharLimit = 128
	return filterModel{input: ti}
}

func (m *filterModel) activate() {
	m.active = true
	m.input.SetValue("")
	m.input.Focus()
}

func (m *filterModel) deactivate() {
	m.active = false
	m.input.Blur()
}

func (m filterModel) Update(msg tea.Msg) (filterModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			val := m.input.Value()
			m.deactivate()
			return m, func() tea.Msg { return filterSubmitMsg{value: val} }
		case "esc":
			m.deactivate()
			return m, func() tea.Msg { return filterCancelMsg{} }
		}
	}

	prevVal := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	// Live filtering as user types
	if m.input.Value() != prevVal {
		val := m.input.Value()
		return m, tea.Batch(cmd, func() tea.Msg { return filterChangedMsg{value: val} })
	}

	return m, cmd
}

func (m filterModel) View() string {
	if !m.active {
		return ""
	}
	return inputBarStyle.Render(m.input.View())
}
