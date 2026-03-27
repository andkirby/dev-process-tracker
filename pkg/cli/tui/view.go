package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m *topModel) View() tea.View {
	if m.err != nil {
		return tea.NewView(fmt.Sprintf("Error: %v\nPress 'q' to quit\n", m.err))
	}

	width := m.width
	if width <= 0 {
		width = 120
	}
	if m.height <= 0 {
		m.height = 24
	}

	content := m.baseViewContent(width)
	if m.modal != nil {
		content = overlayModal(content, m.activeModalOverlay(width), width)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m *topModel) baseViewContent(width int) string {
	var b strings.Builder
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)

	switch m.mode {
	case viewModeLogs:
		b.WriteString(headerStyle.Render(m.logsHeaderView()))
		b.WriteString("\n")
	case viewModeLogsDebug:
		b.WriteString(headerStyle.Render("Viewport Debug Mode (b back, q quit)"))
		b.WriteString("\n")
	default:
		b.WriteString(headerStyle.Render("Dev Process Tracker - Health Monitor (q quit, D for debug)"))
	}

	switch m.mode {
	case viewModeTable, viewModeCommand, viewModeSearch:
		b.WriteString("\n")
		b.WriteString(m.renderContext(width))
		b.WriteString("\n")
	}

	switch m.mode {
	case viewModeLogs:
		b.WriteString(m.renderLogs(width))
		b.WriteString("\n")
	case viewModeLogsDebug:
		b.WriteString(m.renderLogsDebug(width))
		b.WriteString("\n")
	case viewModeTable, viewModeSearch:
		b.WriteString(m.table.Render(m, width))
		b.WriteString("\n")
	}

	if m.mode == viewModeCommand {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(fitLine(":"+m.cmdInput, width)))
		b.WriteString("\n")
		hint := `Example: add my-app ~/projects/my-app "npm run dev" 3000`
		if strings.HasPrefix(strings.TrimSpace(m.cmdInput), "add") {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fitLine(hint, width)))
			b.WriteString("\n")
		}
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fitLine("Esc to go back", width)))
		b.WriteString("\n")
	}
	if m.mode == viewModeTable || m.mode == viewModeSearch {
		if sl := m.renderStatusLine(width); sl != "" {
			b.WriteString(sl)
			b.WriteString("\n")
		}
		b.WriteString(m.renderFooter(width))
		b.WriteString("\n")
	} else {
		var footer string
		var statusLine string

		if m.cmdStatus != "" {
			statusLine = m.cmdStatus
		}

		if m.mode == viewModeLogs && len(m.highlightMatches) > 0 {
			matchCounter := fmt.Sprintf("Match %d/%d", m.highlightIndex+1, len(m.highlightMatches))
			footer = fmt.Sprintf("%s | b back | f follow:%t | n/N next/prev highlight", matchCounter, m.followLogs)
		} else if m.mode == viewModeLogs {
			footer = fmt.Sprintf("b back | f follow:%t | ↑↓ scroll | Page Up/Down", m.followLogs)
		} else if m.mode == viewModeLogsDebug {
			footer = "b back | q quit | ↑↓ scroll | Page Up/Down"
		} else {
			footer = fmt.Sprintf("Last updated: %s | Services: %d | Tab switch | Enter logs/start | x remove managed | / filter | ^L clear filter | s sort | ? help | ^A add ^R restart ^E stop | D debug", m.lastUpdate.Format("15:04:05"), m.countVisible())
		}
		footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)

		if statusLine != "" {
			statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
			b.WriteString(statusStyle.Render(fitLine(statusLine, width)))
			b.WriteString("\n")
		}

		b.WriteString(footerStyle.Render(fitLine(footer, width)))
		b.WriteString("\n")
	}

	return b.String()
}

func (m *topModel) renderLogs(width int) string {
	headerLines := renderedLineCount(m.logsHeaderView())
	footerLines := renderedLineCount(m.logsFooterView())
	availableHeight := m.height - headerLines - footerLines
	if availableHeight < 5 {
		availableHeight = 5
	}

	m.viewport.SetWidth(width)
	m.viewport.SetHeight(availableHeight)

	if m.viewportNeedsTop {
		m.viewport.GotoTop()
		m.viewportNeedsTop = false
	}

	return m.viewport.View()
}

func (m *topModel) initDebugViewport() {
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, fmt.Sprintf("Debug Line %d: This is test content for viewport scrolling. Use arrow keys, page up/down, or mouse wheel to scroll. Press 'b' to exit debug mode.", i))
	}
	content := strings.Join(lines, "\n")
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
}

func (m *topModel) renderLogsDebug(width int) string {
	headerHeight := renderedLineCount("Viewport Debug Mode (b back, q quit)")
	footerHeight := renderedLineCount("b back | q quit | ↑↓ scroll | Page Up/Down")
	m.viewport.SetWidth(width)
	height := m.height - headerHeight - footerHeight
	if height < 5 {
		height = 5
	}
	m.viewport.SetHeight(height)
	return m.viewport.View()
}

func (m *topModel) logsHeaderView() string {
	name := "-"
	port := "-"
	pid := "-"
	if m.logSvc != nil {
		name = m.logSvc.Name
		for _, srv := range m.servers {
			if srv.ManagedService != nil && srv.ManagedService.Name == m.logSvc.Name && srv.ProcessRecord != nil {
				if srv.ProcessRecord.Port > 0 {
					port = fmt.Sprintf("%d", srv.ProcessRecord.Port)
				}
				if srv.ProcessRecord.PID > 0 {
					pid = fmt.Sprintf("%d", srv.ProcessRecord.PID)
				}
				break
			}
		}
		if port == "-" && len(m.logSvc.Ports) > 0 && m.logSvc.Ports[0] > 0 {
			port = fmt.Sprintf("%d", m.logSvc.Ports[0])
		}
	} else if m.logPID > 0 {
		pid = fmt.Sprintf("%d", m.logPID)
		for _, srv := range m.servers {
			if srv.ProcessRecord != nil && srv.ProcessRecord.PID == m.logPID {
				if srv.ProcessRecord.Port > 0 {
					port = fmt.Sprintf("%d", srv.ProcessRecord.Port)
				}
				if srv.ManagedService != nil && srv.ManagedService.Name != "" {
					name = srv.ManagedService.Name
				}
				break
			}
		}
		if name == "-" {
			name = fmt.Sprintf("pid:%d", m.logPID)
		}
	}
	return fmt.Sprintf("Logs: %s | Port: %s | PID: %s", name, port, pid)
}

func (m *topModel) logsFooterView() string {
	if len(m.highlightMatches) > 0 {
		matchCounter := fmt.Sprintf("Match %d/%d", m.highlightIndex+1, len(m.highlightMatches))
		return fmt.Sprintf("%s | b back | f follow:%t | n/N next/prev highlight", matchCounter, m.followLogs)
	}
	return fmt.Sprintf("b back | f follow:%t | ↑↓ scroll | Page Up/Down", m.followLogs)
}

func renderedLineCount(s string) int {
	if s == "" {
		return 0
	}
	return 1 + strings.Count(s, "\n")
}
