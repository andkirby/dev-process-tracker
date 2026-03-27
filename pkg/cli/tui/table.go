package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"github.com/devports/devpt/pkg/health"
	"github.com/devports/devpt/pkg/models"
)

type processTable struct {
	runningVP viewport.Model
	managedVP viewport.Model

	aboveLines int
	belowLines int

	lastRunningHeight int
	lastManagedHeight int
}

func newProcessTable() processTable {
	return processTable{
		runningVP:  viewport.New(),
		managedVP:  viewport.New(),
		aboveLines: 2,
		belowLines: 1,
	}
}

func (t *processTable) heightFor(termHeight int, hasStatus bool) int {
	below := t.belowLines
	if hasStatus {
		below++
	}
	h := termHeight - t.aboveLines - below
	if h < 3 {
		h = 3
	}
	return h
}

func (t *processTable) Render(m *topModel, width int) string {
	totalHeight := t.heightFor(m.height, m.hasStatusLine())
	runningContent := m.renderRunningTable(width)
	managedHeader := m.renderManagedHeader(width)
	managedContent := m.renderManagedSection(width)
	runningLines := 1 + strings.Count(runningContent, "\n")
	runningHeight, managedHeight := t.sectionHeights(totalHeight, runningLines)

	t.lastRunningHeight = runningHeight
	t.lastManagedHeight = managedHeight

	t.runningVP.SetWidth(width)
	t.runningVP.SetHeight(runningHeight)
	t.runningVP.SetContent(runningContent)

	t.managedVP.SetWidth(width)
	t.managedVP.SetHeight(managedHeight)
	t.managedVP.SetContent(managedContent)
	if m.tableFollowSelection {
		t.scrollToSelection(m)
	}

	return t.runningVP.View() + "\n" + managedHeader + "\n" + t.managedVP.View()
}

func (m *topModel) hasStatusLine() bool {
	if m.cmdStatus != "" {
		return true
	}
	if m.focus == focusManaged {
		managed := m.managedServices()
		if m.managedSel >= 0 && m.managedSel < len(managed) {
			if m.crashReasonForService(managed[m.managedSel].Name) != "" {
				return true
			}
		}
	}
	return false
}

func (m *topModel) renderContext(width int) string {
	baseStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	appliedFilterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	var filter string
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
		filter = m.searchInput.View()
	case strings.TrimSpace(m.searchQuery) != "":
		filter = appliedFilterStyle.Render(m.searchQuery)
	default:
		filter = "none"
	}

	ctx := strings.Join([]string{
		baseStyle.Render(fmt.Sprintf("Services: %d", m.countVisible())),
		baseStyle.Render(fmt.Sprintf("Sort: %s", sortModeLabel(m.sortBy))),
		baseStyle.Render("Filter: ") + filter,
	}, " | ")
	return fitAnsiLine(ctx, width)
}

func (m *topModel) renderStatusLine(width int) string {
	text := ""
	if m.cmdStatus != "" {
		text = m.cmdStatus
	} else if m.focus == focusManaged {
		managed := m.managedServices()
		if m.managedSel >= 0 && m.managedSel < len(managed) {
			if reason := m.crashReasonForService(managed[m.managedSel].Name); reason != "" {
				text = fmt.Sprintf("Crash: %s", reason)
			}
		}
	}
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
	return s.Render(h.View(m.keys))
}

func (t *processTable) sectionHeights(totalHeight, runningLines int) (int, int) {
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
		t.scrollViewportToLine(&t.managedVP, selectedLine)
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

	nameW, portW, pidW, projectW, healthW := 14, 6, 7, 14, 7
	sep := 2
	used := nameW + sep + portW + sep + pidW + sep + projectW + sep + healthW + sep
	cmdW := width - used
	if cmdW < 12 {
		cmdW = 12
	}

	header := fmt.Sprintf("%s%s%s%s%s%s%s%s%s%s%s",
		fixedCell("Name", nameW), pad(sep),
		fixedCell("Port", portW), pad(sep),
		fixedCell("PID", pidW), pad(sep),
		fixedCell("Project", projectW), pad(sep),
		fixedCell("Command", cmdW), pad(sep),
		fixedCell("Health", healthW),
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
	lines = append(lines, fitLine(header, width))
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
			truncatedCmd = runewidth.Truncate(cmd, cmdW-3, "...")
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
	text := "Managed Services "
	fillW := width - runewidth.StringWidth(text)
	if fillW < 0 {
		fillW = 0
	}
	header := text + strings.Repeat("─", fillW)
	return lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render(fitLine(header, width))
}

func (m *topModel) renderManagedSection(width int) string {
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

	var b strings.Builder
	for i, svc := range managed {
		state := m.serviceStatus(svc.Name)
		if state == "stopped" {
			if _, ok := m.starting[svc.Name]; ok {
				state = "starting"
			}
		}

		line := fmt.Sprintf("%s [%s]", svc.Name, state)

		conflicting := false
		for _, p := range svc.Ports {
			if portOwners[p] > 1 {
				conflicting = true
				break
			}
		}
		if conflicting {
			line = fmt.Sprintf("%s (port conflict)", line)
		} else if len(svc.Ports) > 1 {
			line = fmt.Sprintf("%s (ports: %v)", line, svc.Ports)
		}

		line = fitLine(line, width)
		if i == m.managedSel {
			bg := "8"
			if m.focus == focusManaged {
				bg = "57"
			}
			line = lipgloss.NewStyle().Background(lipgloss.Color(bg)).Foreground(lipgloss.Color("15")).Render(line)
		}
		b.WriteString(line)
		if i < len(managed)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (t *processTable) updateFocusedViewport(focus viewFocus, msg tea.Msg) tea.Cmd {
	if focus == focusManaged {
		var cmd tea.Cmd
		t.managedVP, cmd = t.managedVP.Update(msg)
		return cmd
	}
	var cmd tea.Cmd
	t.runningVP, cmd = t.runningVP.Update(msg)
	return cmd
}

func (t *processTable) updateViewportForTableY(viewportY int, msg tea.Msg) tea.Cmd {
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
		var cmd tea.Cmd
		t.managedVP, cmd = t.managedVP.Update(msg)
		return cmd
	}
	return nil
}

func (t *processTable) runningYOffset() int {
	return t.runningVP.YOffset()
}

func (t *processTable) managedYOffset() int {
	return t.managedVP.YOffset()
}

func pad(n int) string {
	return strings.Repeat(" ", n)
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

func (m topModel) sortServers(servers []*models.ServerInfo) {
	switch m.sortBy {
	case sortName:
		sort.Slice(servers, func(i, j int) bool {
			return strings.ToLower(m.serviceNameFor(servers[i])) < strings.ToLower(m.serviceNameFor(servers[j]))
		})
	case sortProject:
		sort.Slice(servers, func(i, j int) bool {
			return strings.ToLower(projectOf(servers[i])) < strings.ToLower(projectOf(servers[j]))
		})
	case sortPort:
		sort.Slice(servers, func(i, j int) bool { return portOf(servers[i]) < portOf(servers[j]) })
	case sortHealth:
		sort.Slice(servers, func(i, j int) bool {
			return strings.Compare(m.health[portOf(servers[i])], m.health[portOf(servers[j])]) < 0
		})
	default:
		sort.Slice(servers, func(i, j int) bool { return pidOf(servers[i]) > pidOf(servers[j]) })
	}
}
