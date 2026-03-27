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
	if m.mode == viewModeConfirm && m.confirm != nil {
		content = overlayConfirmModal(content, m.renderConfirmModal(width), width)
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
	case viewModeLogsDebug:
		b.WriteString(headerStyle.Render("Viewport Debug Mode (b back, q quit)"))
	default:
		b.WriteString(headerStyle.Render("Dev Process Tracker - Health Monitor (q quit, D for debug)"))
	}

	switch m.mode {
	case viewModeTable, viewModeCommand, viewModeSearch, viewModeConfirm:
		b.WriteString("\n")
		b.WriteString(m.renderContext(width))
		b.WriteString("\n")
	}

	switch m.mode {
	case viewModeHelp:
		b.WriteString(m.renderHelp(width))
		b.WriteString("\n")
	case viewModeLogs:
		b.WriteString(m.renderLogs(width))
		b.WriteString("\n")
	case viewModeLogsDebug:
		b.WriteString(m.renderLogsDebug(width))
		b.WriteString("\n")
	case viewModeTable, viewModeConfirm:
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
	if m.mode == viewModeSearch {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(fitLine("/"+m.searchQuery, width)))
		b.WriteString("\n")
	}
	if m.mode == viewModeTable || m.mode == viewModeConfirm {
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
	headerText := m.logsHeaderView()
	headerLines := 1 + strings.Count(headerText, "\n")
	footerLines := 3
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
	headerHeight := 4
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(m.height - headerHeight - 4)
	return m.viewport.View()
}

func (m *topModel) logsHeaderView() string {
	name := "-"
	if m.logSvc != nil {
		name = m.logSvc.Name
	} else if m.logPID > 0 {
		name = fmt.Sprintf("pid:%d", m.logPID)
	}
	return fmt.Sprintf("Logs: %s (b back, f follow:%t)", name, m.followLogs)
}

func (m topModel) renderHelp(width int) string {
	lines := []string{
		"Keymap",
		"q quit, Tab switch list, Enter logs/start, / filter, Ctrl+L clear filter, s sort, h health detail, ? help",
		"Ctrl+A add command, Ctrl+R restart selected, Ctrl+E stop selected",
		"Logs: b back, f toggle follow",
		"Managed list: x remove selected service",
		"Commands: add, start, stop, remove, restore, list, help",
	}
	var out []string
	for _, l := range lines {
		out = append(out, fitLine(l, width))
	}
	return strings.Join(out, "\n")
}
