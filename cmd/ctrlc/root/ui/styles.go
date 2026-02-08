package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED") // purple
	secondaryColor = lipgloss.Color("#06B6D4") // cyan
	successColor   = lipgloss.Color("#22C55E") // green
	dangerColor    = lipgloss.Color("#EF4444") // red
	warningColor   = lipgloss.Color("#F59E0B") // amber
	mutedColor     = lipgloss.Color("#6B7280") // gray
	textColor      = lipgloss.Color("#E5E7EB") // light gray

	// Header info labels (left side)
	infoLabelStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	infoValueStyle = lipgloss.NewStyle().
			Foreground(textColor)

	// Shortcut keys in header (right side)
	shortcutKeyStyle = lipgloss.NewStyle().
				Foreground(warningColor).
				Bold(true)

	shortcutDescStyle = lipgloss.NewStyle().
				Foreground(textColor)

	// Table border
	tableBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor)

	// Table title (shown in top border)
	tableTitleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	// Selected row in table
	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primaryColor)

	// Header row inside table
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondaryColor)

	// Help text / muted
	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Status badges
	statusSuccessStyle = lipgloss.NewStyle().Foreground(successColor)
	statusDangerStyle  = lipgloss.NewStyle().Foreground(dangerColor)
	statusWarningStyle = lipgloss.NewStyle().Foreground(warningColor)
	statusMutedStyle   = lipgloss.NewStyle().Foreground(mutedColor)

	// Input bar (command/filter)
	inputBarStyle = lipgloss.NewStyle().
			Foreground(textColor)

	// Suggestion in command bar
	suggestionStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)

	suggestionActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(secondaryColor)

	// Detail view
	detailKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondaryColor)

	// Bottom resource indicator
	resourceIndicatorStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)
)
