package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/version"
	"github.com/ctrlplanedev/cli/internal/api"
)

// tickMsg is emitted by the auto-refresh ticker
type tickMsg time.Time

// viewFrame represents one level in the navigation stack
type viewFrame struct {
	title          string       // e.g. "Deployments", "my-app > Jobs"
	resource       resourceType // what top-level resource this relates to
	table          tableModel   // the table for this frame
	drillKind      string       // "" for top-level, "deployment-jobs", "deployment-versions"
	drill          *drillContext
	statusColIndex int // which column (0-indexed) holds status, -1 if none
}

// Model is the root Bubble Tea model for the ctrlplane TUI
type Model struct {
	// API
	client      *api.ClientWithResponses
	workspaceID string

	// Display info
	workspaceName string
	apiURL        string

	// Navigation stack
	stack []viewFrame

	// Detail overlay (triggered by 'd')
	detail     detailModel
	showDetail bool

	// Input overlays
	command commandModel
	filter  filterModel

	// Help
	showHelp bool

	// Auto-refresh
	refreshInterval time.Duration

	// Terminal size
	width  int
	height int
}

// NewModel creates the root model
func NewModel(client *api.ClientWithResponses, workspaceID string, refreshInterval time.Duration, startView resourceType, workspaceName string, apiURL string) Model {
	frame := newTopLevelFrame(startView)
	return Model{
		client:          client,
		workspaceID:     workspaceID,
		workspaceName:   workspaceName,
		apiURL:          apiURL,
		stack:           []viewFrame{frame},
		command:         newCommandModel(),
		filter:          newFilterModel(),
		refreshInterval: refreshInterval,
		width:           80,
		height:          24,
	}
}

func newTopLevelFrame(rt resourceType) viewFrame {
	cols := columnsForResource(rt)
	statusCol := -1
	switch rt {
	case resourceTypeJobs:
		statusCol = 1
	case resourceTypeVersions:
		statusCol = 2
	}
	return viewFrame{
		title:          rt.String(),
		resource:       rt,
		table:          newTableModel(cols),
		statusColIndex: statusCol,
	}
}

func (m Model) currentFrame() *viewFrame {
	return &m.stack[len(m.stack)-1]
}

// Init returns the initial command
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		fetchData(m.client, m.workspaceID, m.currentFrame().resource),
	}
	if m.refreshInterval > 0 {
		cmds = append(cmds, tea.Tick(m.refreshInterval, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}))
	}
	return tea.Batch(cmds...)
}

// Update handles all messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		for i := range m.stack {
			m.stack[i].table.width = msg.Width
			m.stack[i].table.height = msg.Height
		}
		if m.showDetail {
			m.detail.setSize(msg.Width, msg.Height)
		}
		return m, nil

	case tickMsg:
		cmds = append(cmds, m.refreshCurrentFrame())
		cmds = append(cmds, tea.Tick(m.refreshInterval, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}))
		return m, tea.Batch(cmds...)

	case dataMsg:
		frame := m.currentFrame()
		if msg.err != nil {
			frame.table.setError(msg.err)
		} else {
			frame.table.setData(msg.rows, msg.total)
		}
		return m, nil

	case commandSubmitMsg:
		m.navigateToTopLevel(msg.resource)
		saveLastView(msg.resource)
		return m, fetchData(m.client, m.workspaceID, msg.resource)

	case commandCancelMsg:
		return m, nil

	case filterSubmitMsg:
		frame := m.currentFrame()
		frame.table.filterText = msg.value
		if frame.resource == resourceTypeResources && frame.drillKind == "" {
			frame.table.loading = true
			return m, fetchResourcesFiltered(m.client, m.workspaceID, msg.value)
		}
		frame.table.applyFilter()
		return m, nil

	case filterCancelMsg:
		frame := m.currentFrame()
		frame.table.filterText = ""
		if frame.resource == resourceTypeResources && frame.drillKind == "" {
			frame.table.loading = true
			return m, fetchResourcesFiltered(m.client, m.workspaceID, "")
		}
		frame.table.applyFilter()
		return m, nil

	case filterChangedMsg:
		frame := m.currentFrame()
		frame.table.filterText = msg.value
		if frame.resource == resourceTypeResources && frame.drillKind == "" {
			frame.table.loading = true
			return m, fetchResourcesFiltered(m.client, m.workspaceID, msg.value)
		}
		frame.table.applyFilter()
		return m, nil

	case tea.KeyMsg:
		if m.command.active {
			var cmd tea.Cmd
			m.command, cmd = m.command.Update(msg)
			return m, cmd
		}

		if m.filter.active {
			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(msg)
			return m, cmd
		}

		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		if m.showDetail {
			return m.updateDetailOverlay(msg)
		}

		return m.updateTableView(msg)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) updateDetailOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Back):
		m.showDetail = false
		return m, nil
	default:
		var cmd tea.Cmd
		m.detail.viewport, cmd = m.detail.viewport.Update(msg)
		return m, cmd
	}
}

func (m Model) updateTableView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	frame := m.currentFrame()

	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Help):
		m.showHelp = true
		return m, nil
	case key.Matches(msg, keys.Command):
		m.command.activate()
		return m, m.command.input.Focus()
	case key.Matches(msg, keys.Filter):
		m.filter.activate()
		return m, m.filter.input.Focus()
	case key.Matches(msg, keys.Back):
		if len(m.stack) > 1 {
			m.stack = m.stack[:len(m.stack)-1]
			parent := m.currentFrame()
			if parent.table.filterText != "" {
				parent.table.filterText = ""
				// For resources, need to re-fetch from server since filter was server-side
				if parent.resource == resourceTypeResources && parent.drillKind == "" {
					parent.table.loading = true
					return m, fetchResourcesFiltered(m.client, m.workspaceID, "")
				}
				parent.table.applyFilter()
			}
		}
		return m, nil
	case key.Matches(msg, keys.Refresh):
		frame.table.loading = true
		return m, m.refreshCurrentFrame()
	case key.Matches(msg, keys.Describe):
		row := frame.table.selectedRow()
		if row != nil {
			title := ""
			if len(row.cols) > 0 {
				title = row.cols[0]
			}
			m.detail = newDetailModel(title, row.rawItem, m.width, m.height)
			m.showDetail = true
		}
		return m, nil
	case key.Matches(msg, keys.Up):
		frame.table.moveUp()
		return m, nil
	case key.Matches(msg, keys.Down):
		frame.table.moveDown()
		return m, nil
	case key.Matches(msg, keys.Enter):
		return m.handleDrillDown()
	}

	return m, nil
}

func (m Model) handleDrillDown() (tea.Model, tea.Cmd) {
	frame := m.currentFrame()
	row := frame.table.selectedRow()
	if row == nil {
		return m, nil
	}

	switch frame.resource {
	case resourceTypeDeployments:
		if frame.drillKind == "" {
			depItem, ok := row.rawItem.(api.DeploymentAndSystems)
			if !ok {
				return m, nil
			}
			jobFrame := viewFrame{
				title:     depItem.Deployment.Name + " > Jobs",
				resource:  resourceTypeDeployments,
				table:     newTableModel(columnsForDrillDown("deployment-jobs")),
				drillKind: "deployment-jobs",
				drill: &drillContext{
					deploymentID:   depItem.Deployment.Id,
					deploymentName: depItem.Deployment.Name,
				},
				statusColIndex: 1,
			}
			jobFrame.table.width = m.width
			jobFrame.table.height = m.height
			m.stack = append(m.stack, jobFrame)
			return m, fetchJobsForDeployment(m.client, m.workspaceID, depItem.Deployment.Id)
		}

	case resourceTypeResources:
		if frame.drillKind == "" {
			resItem, ok := row.rawItem.(api.Resource)
			if !ok {
				return m, nil
			}
			depFrame := viewFrame{
				title:     resItem.Name + " > Deployments",
				resource:  resourceTypeResources,
				table:     newTableModel(columnsForDrillDown("resource-deployments")),
				drillKind: "resource-deployments",
				drill: &drillContext{
					resourceIdentifier: resItem.Identifier,
					resourceName:       resItem.Name,
				},
				statusColIndex: 1,
			}
			depFrame.table.width = m.width
			depFrame.table.height = m.height
			m.stack = append(m.stack, depFrame)
			return m, fetchDeploymentsForResource(m.client, m.workspaceID, resItem.Identifier)
		}
	}

	return m, nil
}

func (m *Model) navigateToTopLevel(rt resourceType) {
	frame := newTopLevelFrame(rt)
	frame.table.width = m.width
	frame.table.height = m.height
	m.stack = []viewFrame{frame}
}

func (m Model) refreshCurrentFrame() tea.Cmd {
	frame := m.currentFrame()
	if frame.drillKind == "" {
		return fetchData(m.client, m.workspaceID, frame.resource)
	}
	if frame.drill != nil {
		switch frame.drillKind {
		case "deployment-jobs":
			return fetchJobsForDeployment(m.client, m.workspaceID, frame.drill.deploymentID)
		case "deployment-versions":
			return fetchVersionsForDeployment(m.client, m.workspaceID, frame.drill.deploymentID)
		case "resource-deployments":
			return fetchDeploymentsForResource(m.client, m.workspaceID, frame.drill.resourceIdentifier)
		}
	}
	return nil
}

// ──────────────────── View ────────────────────

func (m Model) View() string {
	if m.showHelp {
		return m.renderHelp()
	}
	if m.showDetail {
		return m.detail.View()
	}

	var b strings.Builder
	frame := m.currentFrame()

	// ── Header: info (left) + shortcuts (right) ──
	b.WriteString(m.renderHeader())

	// ── Bordered table ──
	b.WriteString(m.renderBorderedTable(frame))

	// ── Bottom resource indicator ──
	resName := strings.ToLower(frame.title)
	if m.filter.active {
		b.WriteString(m.filter.View())
	} else if m.command.active {
		b.WriteString(m.command.View())
	} else {
		b.WriteString(resourceIndicatorStyle.Render(fmt.Sprintf(" <%s>", resName)))
	}

	return b.String()
}

func (m Model) renderHeader() string {
	// Left info column
	labelW := 12
	info := []struct{ label, value string }{
		{"Workspace:", m.workspaceName},
		{"URL:", m.apiURL},
		{"ctrlc Rev:", "v" + version.Version},
	}

	var leftLines []string
	for _, item := range info {
		line := infoLabelStyle.Width(labelW).Render(item.label) + " " + infoValueStyle.Render(item.value)
		leftLines = append(leftLines, line)
	}
	leftBlock := strings.Join(leftLines, "\n")

	// Right shortcuts (two columns)
	type shortcut struct{ key, desc string }
	col1 := []shortcut{
		{"<d>", "Describe"},
		{"<enter>", "Drill Down"},
		{"<esc>", "Back"},
		{"<?>", "Help"},
	}
	col2 := []shortcut{
		{"</>", "Search"},
		{"<:>", "Command"},
		{"<r>", "Refresh"},
		{"<q>", "Quit"},
	}

	shortW := 10
	descW := 12
	var rightLines []string
	for i := 0; i < max(len(col1), len(col2)); i++ {
		var line string
		if i < len(col1) {
			line += shortcutKeyStyle.Width(shortW).Render(col1[i].key)
			line += shortcutDescStyle.Width(descW).Render(col1[i].desc)
		} else {
			line += strings.Repeat(" ", shortW+descW)
		}
		line += " "
		if i < len(col2) {
			line += shortcutKeyStyle.Width(shortW).Render(col2[i].key)
			line += shortcutDescStyle.Width(descW).Render(col2[i].desc)
		}
		rightLines = append(rightLines, line)
	}
	rightBlock := strings.Join(rightLines, "\n")

	// Lay out left + right
	leftWidth := lipgloss.Width(leftBlock)
	rightWidth := lipgloss.Width(rightBlock)
	gap := m.width - leftWidth - rightWidth
	if gap < 2 {
		gap = 2
	}

	leftSplit := strings.Split(leftBlock, "\n")
	rightSplit := strings.Split(rightBlock, "\n")
	maxLines := max(len(leftSplit), len(rightSplit))

	var headerLines []string
	for i := 0; i < maxLines; i++ {
		l := ""
		if i < len(leftSplit) {
			l = leftSplit[i]
		}
		r := ""
		if i < len(rightSplit) {
			r = rightSplit[i]
		}
		lPad := leftWidth - lipgloss.Width(l)
		if lPad < 0 {
			lPad = 0
		}
		headerLines = append(headerLines, " "+l+strings.Repeat(" ", lPad+gap)+r)
	}

	return strings.Join(headerLines, "\n") + "\n"
}

func (m Model) renderBorderedTable(frame *viewFrame) string {
	// Build inner content (header + rows, no border)
	innerContent := frame.table.renderRows(frame.statusColIndex)

	// Compute inner width: table width minus border (2 chars for │ on each side + padding)
	innerWidth := m.width - 4
	if innerWidth < 20 {
		innerWidth = 20
	}

	// Table title: resource(workspace)[count]
	count := len(frame.table.rows)
	var titleText string
	if frame.drillKind != "" {
		// Drill-down: show breadcrumb
		var crumbs []string
		for _, f := range m.stack {
			crumbs = append(crumbs, f.title)
		}
		titleText = fmt.Sprintf(" %s[%d] ", strings.Join(crumbs, " > "), count)
	} else {
		titleText = fmt.Sprintf(" %s(%s)[%d] ", strings.ToLower(frame.resource.String()), m.workspaceName, count)
	}

	// Filter indicator inside the table
	filterLine := ""
	if frame.table.filterText != "" {
		filterLine = lipgloss.NewStyle().Foreground(warningColor).Render(
			fmt.Sprintf("/%s (%d matches)", frame.table.filterText, len(frame.table.rows))) + "\n"
	}

	content := filterLine + innerContent

	// Compute available height for the border box
	// Total: header(4-5 lines) + border(2 top+bottom) + bottom indicator(1)
	headerHeight := len(strings.Split(m.renderHeader(), "\n"))
	borderBoxHeight := m.height - headerHeight - 1
	if borderBoxHeight < 5 {
		borderBoxHeight = 5
	}

	// Build the bordered box
	box := tableBorderStyle.
		Width(innerWidth).
		Height(borderBoxHeight - 2). // subtract for border lines
		Render(content)

	// Inject the title into the top border line
	boxLines := strings.Split(box, "\n")
	if len(boxLines) > 0 {
		topBorder := boxLines[0]
		styledTitle := tableTitleStyle.Render(titleText)
		topBorder = injectTitle(topBorder, styledTitle, titleText)
		boxLines[0] = topBorder
	}

	return strings.Join(boxLines, "\n") + "\n"
}

// injectTitle replaces the middle of the top border with the title
func injectTitle(border string, styledTitle string, rawTitle string) string {
	borderRunes := []rune(border)
	titleLen := len([]rune(rawTitle))

	if len(borderRunes) < titleLen+4 {
		return border
	}

	// Find center position
	center := len(borderRunes) / 2
	start := center - titleLen/2

	// Replace border runes with spaces where title goes, then overlay
	prefix := string(borderRunes[:start])
	suffix := string(borderRunes[start+titleLen:])

	return prefix + styledTitle + suffix
}

func (m Model) renderHelp() string {
	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	helpItems := []struct{ key, desc string }{
		{"↑ / k", "Move cursor up"},
		{"↓ / j", "Move cursor down"},
		{"Enter", "Drill down into resource"},
		{"d", "Describe (show YAML)"},
		{"Esc", "Go back / close overlay"},
		{"/", "Search / filter current table"},
		{":", "Command bar (switch resource type)"},
		{"r", "Refresh current view"},
		{"q", "Quit"},
		{"?", "Toggle this help"},
	}

	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Width(12)
	descStyle := lipgloss.NewStyle().Foreground(textColor)

	for _, item := range helpItems {
		b.WriteString("  ")
		b.WriteString(keyStyle.Render(item.key))
		b.WriteString(descStyle.Render(item.desc))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	cmdTitle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Render("  Commands:")
	b.WriteString(cmdTitle)
	b.WriteString("\n")

	for _, item := range commandItems {
		b.WriteString("  ")
		label := ":" + item.name
		if len(item.aliases) > 0 {
			label += " (" + strings.Join(item.aliases, ", ") + ")"
		}
		b.WriteString(keyStyle.Render(label))
		b.WriteString(descStyle.Render("Switch to " + item.name + " view"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  Press any key to dismiss"))

	return b.String()
}
