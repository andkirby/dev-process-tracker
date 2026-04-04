package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"github.com/devports/devpt/pkg/health"
	"github.com/devports/devpt/pkg/models"
)

type processTable struct {
	runningVP        viewport.Model
	managedListVP    viewport.Model
	managedDetailsVP viewport.Model

	lastRunningHeight int
	lastManagedHeight int
	lastListWidth     int
	lastDetailsWidth  int
}

func newProcessTable() processTable {
	return processTable{
		runningVP:        viewport.New(),
		managedListVP:    viewport.New(),
		managedDetailsVP: viewport.New(),
	}
}

func (t *processTable) heightFor(termHeight, aboveLines, belowLines int) int {
	h := termHeight - aboveLines - belowLines
	if h < 3 {
		h = 3
	}
	return h
}

func (t *processTable) Render(m *topModel, width int) string {
	topLines := m.tableTopLines(width)
	bottomLines := m.tableBottomLines(width)
	totalHeight := t.heightFor(m.height, topLines, bottomLines)
	runningContent := m.renderRunningTable(width)
	managedHeader := m.renderManagedHeader(width)
	listContent := m.renderManagedList(width / 2)
	detailsContent := m.renderManagedDetails(width - width/2)
	runningLines := 1 + strings.Count(runningContent, "\n")
	listLines := 1 + strings.Count(listContent, "\n")
	detailsLines := 1 + strings.Count(detailsContent, "\n")
	managedLines := max(listLines, detailsLines)
	runningHeight, managedHeight := t.sectionHeights(totalHeight, runningLines, managedLines)

	t.lastRunningHeight = runningHeight
	t.lastManagedHeight = managedHeight
	t.lastListWidth = width / 2
	t.lastDetailsWidth = width - width/2

	t.runningVP.SetWidth(width)
	t.runningVP.SetHeight(runningHeight)
	t.runningVP.SetContent(runningContent)

	t.managedListVP.SetWidth(width / 2)
	t.managedListVP.SetHeight(managedHeight)
	t.managedListVP.SetContent(listContent)

	t.managedDetailsVP.SetWidth(width - width/2)
	t.managedDetailsVP.SetHeight(managedHeight)
	t.managedDetailsVP.SetContent(detailsContent)

	if m.tableFollowSelection {
		t.scrollToSelection(m)
	}

	listView := t.managedListVP.View()
	detailsView := t.managedDetailsVP.View()

	return t.runningVP.View() + "\n" + managedHeader + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, listView, detailsView)
}

func (m *topModel) tableTopLines(width int) int {
	// Header line + blank line before the table content.
	return 2
}

func (m *topModel) tableBottomLines(width int) int {
	lines := renderedLineCount(m.renderFooter(width))
	if sl := m.renderStatusLine(width); sl != "" {
		lines += renderedLineCount(sl)
	}
	return lines
}

func (m *topModel) hasStatusLine() bool {
	if m.cmdStatus != "" {
		return true
	}
	// With split view, details pane shows service context - no need for status line
	return false
}

func (m *topModel) renderStatusLine(width int) string {
	text := ""
	if m.cmdStatus != "" {
		text = m.cmdStatus
	}
	// With split view, the details pane shows service state - no duplication in status line
	if text == "" {
		return ""
	}
	s := lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	return s.Render(fitLine(text, width))
}

func (m *topModel) renderFooter(width int) string {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	h := m.help
	h.SetWidth(width)
	return strings.TrimRight(s.Render(h.View(m.footerKeyMap())), "\n")
}

func (m *topModel) footerKeyMap() keyMap {
	k := m.keys
	k.Search = key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", m.footerFilterLabel()),
	)
	if m.groupHighlightNamespace != nil {
		green := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true).Render("group mode")
		k.GroupToggle = key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", green),
		)
	}
	return k
}

func (m *topModel) footerFilterLabel() string {
	switch {
	case m.mode == viewModeSearch:
		inputWidth := runewidth.StringWidth(m.searchInput.Value()) + 1
		if inputWidth < 1 {
			inputWidth = 1
		}
		if inputWidth > 24 {
			inputWidth = 24
		}
		m.searchInput.SetWidth(inputWidth)
		return m.searchInput.View()
	case strings.TrimSpace(m.searchQuery) != "":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(m.searchQuery)
	default:
		return "filter"
	}
}

func (t *processTable) sectionHeights(totalHeight, runningLines, managedLines int) (int, int) {
	if totalHeight < 3 {
		return 1, 1
	}

	separator := 1
	minManaged := 3
	maxRunning := totalHeight - separator - minManaged
	if maxRunning < 1 {
		maxRunning = 1
	}

	runningHeight := runningLines
	if runningHeight > maxRunning {
		runningHeight = maxRunning
	}
	if runningHeight < 1 {
		runningHeight = 1
	}

	managedHeight := totalHeight - separator - runningHeight
	if managedHeight < 1 {
		managedHeight = 1
	}
	if managedLines > 0 && managedHeight > managedLines {
		managedHeight = managedLines
	}

	return runningHeight, managedHeight
}

func (t *processTable) scrollToSelection(m *topModel) {
	visible := m.visibleServers()
	managed := m.managedServices()

	if m.focus == focusRunning && m.selected >= 0 && m.selected < len(visible) {
		selectedLine := 2 + m.selected
		t.scrollViewportToLine(&t.runningVP, selectedLine)
	} else if m.focus == focusManaged && m.managedSel >= 0 && m.managedSel < len(managed) {
		selectedLine := m.managedSel
		t.scrollViewportToLine(&t.managedListVP, selectedLine)
	}
}

func (t *processTable) scrollViewportToLine(vp *viewport.Model, selectedLine int) {
	totalLines := vp.TotalLineCount()
	visibleLines := vp.VisibleLineCount()
	currentOffset := vp.YOffset()

	if selectedLine < currentOffset || selectedLine >= currentOffset+visibleLines {
		desired := selectedLine - visibleLines/3
		if desired < 0 {
			desired = 0
		}
		if desired > totalLines-visibleLines {
			desired = totalLines - visibleLines
		}
		if desired < 0 {
			desired = 0
		}
		vp.SetYOffset(desired)
	}
}

func (m *topModel) renderRunningTable(width int) string {
	visible := m.visibleServers()
	displayNames := m.displayNames(visible)
	headerStyle := lipgloss.NewStyle()
	yellowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)  // yellow for ascending
	orangeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true) // orange for reverse

	nameW, portW, pidW, projectW, healthW := 14, 6, 7, 14, 7
	sep := 2
	used := nameW + sep + portW + sep + pidW + sep + projectW + sep + healthW + sep
	cmdW := width - used
	if cmdW < 12 {
		cmdW = 12
	}

	nameHeader := headerStyle.Render(fixedCell(fmt.Sprintf("Name (%d)", len(visible)), nameW))
	portHeader := headerStyle.Render(fixedCell("Port", portW))
	pidHeader := headerStyle.Render(fixedCell("PID", pidW))
	projectHeader := headerStyle.Render(fixedCell("Project", projectW))
	commandHeader := headerStyle.Render(fixedCell("Command", cmdW))
	healthHeader := headerStyle.Render(fixedCell("Health", healthW))

	// Apply color based on sort state
	switch m.sortBy {
	case sortName:
		if m.sortReverse {
			nameHeader = orangeStyle.Render(fixedCell(fmt.Sprintf("Name (%d)", len(visible)), nameW))
		} else {
			nameHeader = yellowStyle.Render(fixedCell(fmt.Sprintf("Name (%d)", len(visible)), nameW))
		}
	case sortPort:
		if m.sortReverse {
			portHeader = orangeStyle.Render(fixedCell("Port", portW))
		} else {
			portHeader = yellowStyle.Render(fixedCell("Port", portW))
		}
	case sortProject:
		if m.sortReverse {
			projectHeader = orangeStyle.Render(fixedCell("Project", projectW))
		} else {
			projectHeader = yellowStyle.Render(fixedCell("Project", projectW))
		}
	case sortHealth:
		if m.sortReverse {
			healthHeader = orangeStyle.Render(fixedCell("Health", healthW))
		} else {
			healthHeader = yellowStyle.Render(fixedCell("Health", healthW))
		}
	}

	header := fmt.Sprintf("%s%s%s%s%s%s%s%s%s%s%s",
		nameHeader, pad(sep),
		portHeader, pad(sep),
		pidHeader, pad(sep),
		projectHeader, pad(sep),
		commandHeader, pad(sep),
		healthHeader,
	)
	divider := fmt.Sprintf("%s%s%s%s%s%s%s%s%s%s%s",
		fixedCell(strings.Repeat("─", nameW), nameW), pad(sep),
		fixedCell(strings.Repeat("─", portW), portW), pad(sep),
		fixedCell(strings.Repeat("─", pidW), pidW), pad(sep),
		fixedCell(strings.Repeat("─", projectW), projectW), pad(sep),
		fixedCell(strings.Repeat("─", cmdW), cmdW), pad(sep),
		fixedCell(strings.Repeat("─", healthW), healthW),
	)

	if len(visible) == 0 {
		if m.searchQuery != "" {
			return fitLine("(no matching servers for filter)", width)
		}
		return fitLine("(no matching servers)", width)
	}

	var lines []string
	lines = append(lines, fitAnsiLine(header, width))
	lines = append(lines, fitLine(divider, width))

	rowIndices := make([]int, len(visible))
	for i, srv := range visible {
		rowIndices[i] = len(lines)

		project := projectOf(srv)
		port := "-"
		pid := 0
		cmd := "-"
		icon := "…"
		if srv.ProcessRecord != nil {
			pid = srv.ProcessRecord.PID
			cmd = srv.ProcessRecord.Command
			if srv.ProcessRecord.Port > 0 {
				port = fmt.Sprintf("%d", srv.ProcessRecord.Port)
				if cached := m.health[srv.ProcessRecord.Port]; cached != "" {
					icon = cached
				}
			}
		}

		truncatedCmd := cmd
		if runewidth.StringWidth(cmd) > cmdW {
			truncatedCmd = runewidth.Truncate(cmd, cmdW, "...")
		}

		line := fmt.Sprintf("%s%s%s%s%s%s%s%s%s%s%s",
			fixedCell(displayNames[i], nameW), pad(sep),
			fixedCell(port, portW), pad(sep),
			fixedCell(fmt.Sprintf("%d", pid), pidW), pad(sep),
			fixedCell(project, projectW), pad(sep),
			fixedCell(truncatedCmd, cmdW), pad(sep),
			fixedCell(icon, healthW),
		)
		lines = append(lines, fitLine(line, width))
	}

	// Inject OSC 8 hyperlinks into port cells after fitLine (width calc done).
	for i, srv := range visible {
		if srv.ProcessRecord != nil && srv.ProcessRecord.Port > 0 {
			port := fmt.Sprintf("%d", srv.ProcessRecord.Port)
			old := fixedCell(port, portW)
			linked := osc8Link(port, "http://localhost:"+port) + strings.Repeat(" ", portW-len(port))
			lines[rowIndices[i]] = strings.Replace(lines[rowIndices[i]], old, linked, 1)
		}
	}

	// Apply visual group selection highlight when group toggle is active (before selection highlight)
	if m.groupHighlightNamespace != nil {
		groupStyle := lipgloss.NewStyle().Background(lipgloss.Color("61")).Width(width)
		for i, srv := range visible {
			if i == m.selected {
				continue // active row keeps normal selection color
			}
			name := m.serviceNameFor(srv)
			if extractNamespace(name) == *m.groupHighlightNamespace {
				idx := rowIndices[i]
				lines[idx] = groupStyle.Render(lines[idx])
			}
		}
	}

	if m.selected >= 0 && m.selected < len(visible) {
		idx := rowIndices[m.selected]
		bg := "8"
		if m.focus == focusRunning {
			bg = "57"
		}
		lines[idx] = lipgloss.NewStyle().Background(lipgloss.Color(bg)).Foreground(lipgloss.Color("15")).Render(lines[idx])
	}

	out := strings.Join(lines, "\n")
	if m.showHealthDetail && m.selected >= 0 && m.selected < len(visible) {
		port := 0
		if visible[m.selected].ProcessRecord != nil {
			port = visible[m.selected].ProcessRecord.Port
		}
		if d := m.healthDetails[port]; d != nil {
			out += "\n" + fitLine(fmt.Sprintf("Health detail: %s %dms %s", health.StatusIcon(d.Status), d.ResponseMs, d.Message), width)
		}
	}

	return out
}

func (m *topModel) renderManagedHeader(width int) string {
	text := fmt.Sprintf("Managed Services (%d) ", len(m.managedServices()))
	fillW := width - runewidth.StringWidth(text)
	if fillW < 0 {
		fillW = 0
	}
	header := text + strings.Repeat("─", fillW)
	return lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render(fitLine(header, width))
}

// renderManagedSection is no longer used — list and details are rendered into
// independent viewports (managedListVP, managedDetailsVP) in Render().

func (m *topModel) renderManagedList(width int) string {
	managed := m.managedServices()
	if len(managed) == 0 {
		return fitLine(`No managed services yet. Use ^A then: add myapp /path/to/app "npm run dev" 3000`, width)
	}

	portOwners := make(map[int]int)
	for _, svc := range managed {
		for _, p := range svc.Ports {
			portOwners[p]++
		}
	}

	var lines []string
	for i, svc := range managed {
		state := m.serviceStatus(svc.Name)
		if state == "stopped" {
			if _, ok := m.starting[svc.Name]; ok {
				state = "starting"
			}
		}

		// Build plain text first, then apply styling
		symbolChar := managedStatusSymbol(state)
		symbolColor := managedStatusColor(state)
		plainLine := fmt.Sprintf("%s %s [%s]", symbolChar, svc.Name, state)

		conflicting := false
		for _, p := range svc.Ports {
			if portOwners[p] > 1 {
				conflicting = true
				break
			}
		}
		if conflicting {
			plainLine = fmt.Sprintf("%s (port conflict)", plainLine)
		} else if len(svc.Ports) > 1 {
			plainLine = fmt.Sprintf("%s (ports: %v)", plainLine, svc.Ports)
		}

		// Determine background for this row
		var rowBg string
		var rowFg string
		switch {
		case i == m.managedSel && m.focus == focusManaged:
			rowBg = "57"
			rowFg = "15"
		case m.groupHighlightNamespace != nil && extractNamespace(svc.Name) == *m.groupHighlightNamespace:
			rowBg = "61"
		case i == m.managedSel:
			rowBg = "8"
			rowFg = "15"
		}

		var line string
		if rowBg != "" {
			// Single render path for any row with background — no strings.Replace, no ANSI breakage.
			style := lipgloss.NewStyle().Background(lipgloss.Color(rowBg)).Width(width)
			if rowFg != "" {
				style = style.Foreground(lipgloss.Color(rowFg))
			}
			line = style.Render(fitLine(plainLine, width))
		} else {
			// No background — safe to color symbol separately.
			symbolStyled := lipgloss.NewStyle().Foreground(lipgloss.Color(symbolColor)).Bold(true).Render(symbolChar)
			line = strings.Replace(plainLine, symbolChar, symbolStyled, 1)
			line = fitAnsiLine(line, width)
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m *topModel) renderManagedDetails(width int) string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	header := headerStyle.Render("Selected service details")

	managed := m.managedServices()
	if m.managedSel < 0 || m.managedSel >= len(managed) {
		placeholder := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Select a managed service to inspect status")
		return header + "\n" + fitLine(placeholder, width)
	}

	svc := managed[m.managedSel]
	state := m.serviceStatus(svc.Name)
	if state == "stopped" {
		if _, ok := m.starting[svc.Name]; ok {
			state = "starting"
		}
	}

	symbol := lipgloss.NewStyle().Foreground(lipgloss.Color(managedStatusColor(state))).Bold(true).Render(managedStatusSymbol(state))

	var lines []string
	lines = append(lines, fitLine(header, width))
	lines = append(lines, fitLine(fmt.Sprintf(" %s %s [%s]", symbol, svc.Name, state), width))

	if srv := m.serverInfoForService(svc.Name); srv != nil && srv.Source != "" {
		lines = append(lines, fitLine(fmt.Sprintf(" Source: %s", srv.Source), width))
	}

	// Service metadata: CWD, ports, command (rendered after source, before crash context)
	if svc.CWD != "" {
		lines = append(lines, fitLine(fmt.Sprintf(" Dir: %s", svc.CWD), width))
	}
	if len(svc.Ports) > 0 {
		lines = append(lines, fitLine(fmt.Sprintf(" Port: %s", formatPorts(svc.Ports)), width))
	}
	if svc.Command != "" {
		lines = append(lines, fitLine(fmt.Sprintf(" Cmd: %s", svc.Command), width))
	}

	if state == "crashed" {
		if reason := m.crashReasonForService(svc.Name); reason != "" {
			lines = append(lines, fitLine(fmt.Sprintf(" Headline: %s", reason), width))
		}
		if logPath, err := m.app.LatestServiceLogPath(svc.Name); err == nil && strings.TrimSpace(logPath) != "" {
			lines = append(lines, fitLine(fmt.Sprintf(" Log: %s", logPath), width))
		}
		if srv := m.serverInfoForService(svc.Name); srv != nil {
			for _, logLine := range nonEmptyTail(srv.CrashLogTail, 3) {
				lines = append(lines, fitLine(" "+strings.TrimSpace(logLine), width))
			}
		}
	}

	return strings.Join(lines, "\n")
}

func (t *processTable) updateFocusedViewport(focus viewFocus, msg tea.Msg) tea.Cmd {
	if focus == focusManaged {
		var cmd tea.Cmd
		t.managedListVP, cmd = t.managedListVP.Update(msg)
		return cmd
	}
	var cmd tea.Cmd
	t.runningVP, cmd = t.runningVP.Update(msg)
	return cmd
}

func (t *processTable) updateViewportForTableY(viewportY int, viewportX int, msg tea.Msg) tea.Cmd {
	if viewportY < 0 {
		return nil
	}
	if viewportY < t.lastRunningHeight {
		var cmd tea.Cmd
		t.runningVP, cmd = t.runningVP.Update(msg)
		return cmd
	}
	if viewportY == t.lastRunningHeight {
		return nil
	}

	localManagedY := viewportY - t.lastRunningHeight - 1
	if localManagedY >= 0 && localManagedY < t.lastManagedHeight {
		// Route scroll to list or details viewport based on X position
		if viewportX < t.lastListWidth {
			var cmd tea.Cmd
			t.managedListVP, cmd = t.managedListVP.Update(msg)
			return cmd
		}
		var cmd tea.Cmd
		t.managedDetailsVP, cmd = t.managedDetailsVP.Update(msg)
		return cmd
	}
	return nil
}

func (t *processTable) runningYOffset() int {
	return t.runningVP.YOffset()
}

func (t *processTable) managedYOffset() int {
	return t.managedListVP.YOffset()
}

func pad(n int) string {
	return strings.Repeat(" ", n)
}

// portCell renders a port value as a fixed-width cell.
// When the port is a number, it wraps it in an OSC 8 hyperlink to http://localhost:<port>.
// When the port is "-" (no port), it renders as plain text.
// Uses ansi.StringWidth for correct width calculation with escape sequences.
func portCell(port string, width int) string {
	if port == "-" {
		return fixedCell(port, width)
	}
	return fixedHyperlinkCell(port, "http://localhost:"+port, width)
}

func (m topModel) displayNames(servers []*models.ServerInfo) []string {
	base := make([]string, len(servers))
	projectToSvc := make(map[string]string)
	for _, svc := range m.app.ListServices() {
		cwd := strings.TrimRight(strings.TrimSpace(svc.CWD), "/")
		if cwd != "" {
			projectToSvc[cwd] = svc.Name
		}
	}
	for i, srv := range servers {
		base[i] = m.serviceNameFor(srv)
		if base[i] == "-" && srv.ProcessRecord != nil {
			root := strings.TrimRight(strings.TrimSpace(srv.ProcessRecord.ProjectRoot), "/")
			cwd := strings.TrimRight(strings.TrimSpace(srv.ProcessRecord.CWD), "/")
			if mapped := projectToSvc[root]; mapped != "" {
				base[i] = mapped
			} else if mapped := projectToSvc[cwd]; mapped != "" {
				base[i] = mapped
			}
		}
	}

	count := make(map[string]int)
	for _, n := range base {
		count[n]++
	}
	type row struct{ idx, pid int }
	group := make(map[string][]row)
	for i, n := range base {
		group[n] = append(group[n], row{idx: i, pid: pidOf(servers[i])})
	}
	out := make([]string, len(base))
	for name, rows := range group {
		if count[name] <= 1 || name == "-" {
			for _, r := range rows {
				out[r.idx] = name
			}
			continue
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].pid < rows[j].pid })
		for i, r := range rows {
			out[r.idx] = fmt.Sprintf("%s~%d", name, i+1)
		}
	}
	return out
}
