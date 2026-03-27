package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/devports/devpt/pkg/process"
)

func (m *topModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		m.lastInput = time.Now()

		if m.mode == viewModeCommand {
			switch msg.String() {
			case "esc":
				m.mode = viewModeTable
				m.cmdInput = ""
				return m, nil
			case "enter":
				m.cmdStatus = m.runCommand(strings.TrimSpace(m.cmdInput))
				m.cmdInput = ""
				m.mode = viewModeTable
				m.refresh()
				return m, nil
			case "backspace":
				if len(m.cmdInput) > 0 {
					m.cmdInput = m.cmdInput[:len(m.cmdInput)-1]
				}
				return m, nil
			}
			for _, r := range []rune(msg.Text) {
				if r >= 32 && r != 127 {
					m.cmdInput += string(r)
				}
			}
			return m, nil
		}

		if m.mode == viewModeSearch {
			switch msg.String() {
			case "esc":
				m.mode = viewModeTable
				m.searchQuery = ""
				return m, nil
			case "enter":
				m.mode = viewModeTable
				return m, nil
			case "backspace":
				if len(m.searchQuery) > 0 {
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				}
				return m, nil
			}
			for _, r := range []rune(msg.Text) {
				if r >= 32 && r != 127 {
					m.searchQuery += string(r)
				}
			}
			return m, nil
		}

		if m.mode == viewModeLogs {
			switch {
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit
			case key.Matches(msg, m.keys.Back):
				m.clearLogsView()
				return m, nil
			case key.Matches(msg, m.keys.Follow):
				m.followLogs = !m.followLogs
				return m, nil
			case key.Matches(msg, m.keys.NextMatch):
				if len(m.highlightMatches) > 0 {
					m.highlightIndex = (m.highlightIndex + 1) % len(m.highlightMatches)
				}
				return m, nil
			case key.Matches(msg, m.keys.PrevMatch):
				if len(m.highlightMatches) > 0 {
					m.highlightIndex = (m.highlightIndex - 1 + len(m.highlightMatches)) % len(m.highlightMatches)
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		}

		if m.mode == viewModeLogsDebug {
			switch {
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit
			case key.Matches(msg, m.keys.Back):
				m.mode = viewModeTable
				return m, nil
			default:
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case m.modal != nil && key.Matches(msg, m.keys.Help):
			m.closeModal()
			return m, nil
		case key.Matches(msg, m.keys.Tab):
			if m.focus == focusRunning {
				m.focus = focusManaged
				m.tableFollowSelection = true
				managed := m.managedServices()
				if m.managedSel < 0 && len(managed) > 0 {
					m.managedSel = 0
				}
			} else {
				m.focus = focusRunning
				m.tableFollowSelection = true
				visible := m.visibleServers()
				if m.selected < 0 && len(visible) > 0 {
					m.selected = 0
				}
			}
			return m, nil
		case key.Matches(msg, m.keys.Help):
			m.openHelpModal()
			return m, nil
		case key.Matches(msg, m.keys.Search):
			m.mode = viewModeSearch
			return m, nil
		case key.Matches(msg, m.keys.ClearFilter):
			m.searchQuery = ""
			m.cmdStatus = "Filter cleared"
			return m, nil
		case key.Matches(msg, m.keys.Sort):
			m.sortBy = (m.sortBy + 1) % sortModeCount
			return m, nil
		case key.Matches(msg, m.keys.Health):
			m.showHealthDetail = !m.showHealthDetail
			return m, nil
		case key.Matches(msg, m.keys.Debug):
			m.mode = viewModeLogsDebug
			m.initDebugViewport()
			return m, nil
		case key.Matches(msg, m.keys.Add):
			m.mode = viewModeCommand
			m.cmdInput = "add "
			return m, nil
		case key.Matches(msg, m.keys.Restart):
			m.cmdStatus = m.restartSelected()
			m.refresh()
			return m, nil
		case key.Matches(msg, m.keys.Stop):
			m.prepareStopConfirm()
			return m, nil
		case key.Matches(msg, m.keys.Remove):
			if m.focus == focusManaged {
				managed := m.managedServices()
				if m.managedSel >= 0 && m.managedSel < len(managed) {
					name := managed[m.managedSel].Name
					m.openConfirmModal(&confirmState{
						kind:   confirmRemoveService,
						prompt: fmt.Sprintf("Remove %q from registry?", name),
						name:   name,
					})
				} else {
					m.cmdStatus = "No managed service selected"
				}
			}
			return m, nil
		case msg.String() == ":" || msg.String() == "shift+;" || msg.String() == ";" || msg.String() == "c":
			m.mode = viewModeCommand
			m.cmdInput = ""
			return m, nil
		case msg.String() == "esc":
			if m.modal != nil {
				m.closeModal()
				return m, nil
			}
			switch m.mode {
			case viewModeTable:
				return m, tea.Quit
			case viewModeLogs:
				m.clearLogsView()
			}
			return m, nil
		case msg.String() == "b":
			if m.mode == viewModeLogs {
				m.clearLogsView()
			}
			return m, nil
		case msg.String() == "backspace":
			return m, nil
		case key.Matches(msg, m.keys.Up):
			if m.focus == focusRunning && m.selected > 0 {
				m.selected--
				m.tableFollowSelection = true
			}
			if m.focus == focusManaged && m.managedSel > 0 {
				m.managedSel--
				m.tableFollowSelection = true
			}
			return m, nil
		case key.Matches(msg, m.keys.Down):
			if m.focus == focusRunning {
				if m.selected < len(m.visibleServers())-1 {
					m.selected++
					m.tableFollowSelection = true
				}
			}
			if m.focus == focusManaged {
				if m.managedSel < len(m.managedServices())-1 {
					m.managedSel++
					m.tableFollowSelection = true
				}
			}
			return m, nil
		case key.Matches(msg, m.keys.Confirm):
			if m.activeModalKind() == modalConfirm {
				cmd := m.executeConfirm(true)
				return m, cmd
			}
			return m, nil
		case key.Matches(msg, m.keys.Cancel):
			if m.activeModalKind() == modalConfirm {
				cmd := m.executeConfirm(false)
				return m, cmd
			}
			if m.mode == viewModeLogs && len(m.highlightMatches) > 0 {
				m.highlightIndex = (m.highlightIndex + 1) % len(m.highlightMatches)
			}
			return m, nil
		case msg.String() == "pgup" || msg.String() == "pgdown" || msg.String() == "home" || msg.String() == "end":
			m.tableFollowSelection = false
			cmd := m.table.updateFocusedViewport(m.focus, msg)
			return m, cmd
		case key.Matches(msg, m.keys.Enter):
			switch m.mode {
			case viewModeTable:
				if m.activeModalKind() == modalConfirm {
					cmd := m.executeConfirm(true)
					return m, cmd
				}
				return m.handleEnterKey()
			}
			return m, nil
		default:
			return m, nil
		}
	case tea.MouseMsg:
		mouse := msg.Mouse()
		if m.modal != nil {
			if _, ok := msg.(tea.MouseClickMsg); ok && mouse.Button == tea.MouseLeft {
				bounds := m.activeModalBounds(m.width, m.baseViewContent(m.width))
				if !bounds.contains(mouse.X, mouse.Y) {
					if m.activeModalKind() == modalConfirm {
						cmd := m.executeConfirm(false)
						return m, cmd
					}
					m.closeModal()
					return m, nil
				}
				return m, nil
			}
			return m, nil
		}
		if m.mode == viewModeTable {
			if _, ok := msg.(tea.MouseClickMsg); ok && mouse.Button == tea.MouseLeft {
				return m.handleTableMouseClick(msg)
			}
			m.tableFollowSelection = false
			viewportY := mouse.Y - 2
			cmd := m.table.updateViewportForTableY(viewportY, msg)
			return m, cmd
		}
		if m.mode == viewModeLogs {
			if _, ok := msg.(tea.MouseClickMsg); ok {
				return m.handleMouseClick(msg)
			}
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		if m.mode == viewModeLogsDebug {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.SetWidth(msg.Width)
	case tickMsg:
		m.refresh()
		if m.mode == viewModeLogs && m.followLogs {
			return m, m.tailLogsCmd()
		}
		if m.mode == viewModeTable && !m.healthBusy && time.Since(m.healthLast) > 2*time.Second && time.Since(m.lastInput) > 900*time.Millisecond {
			m.healthBusy = true
			return m, m.healthCmd()
		}
		return m, tickCmd()
	case logMsg:
		oldYOffset := m.viewport.YOffset()
		totalLines := m.viewport.TotalLineCount()
		visibleLines := m.viewport.VisibleLineCount()
		wasAtBottom := (oldYOffset+visibleLines >= totalLines) || totalLines == 0

		m.logLines = msg.lines
		m.logErr = msg.err
		if m.logErr != nil {
			var content string
			if errors.Is(m.logErr, process.ErrNoLogs) {
				content = "No devpt logs for this service yet.\nLogs are only captured when started by devpt.\n"
			} else if errors.Is(m.logErr, process.ErrNoProcessLogs) {
				content = "No accessible logs for this process.\nIf it writes only to a terminal, there may be nothing to tail here.\n"
			} else {
				content = fmt.Sprintf("Error: %v\n", m.logErr)
			}
			m.viewport.SetContent(content)
			m.viewport.GotoTop()
		} else if len(m.logLines) == 0 {
			m.viewport.SetContent("(no logs yet)\n")
			m.viewport.GotoTop()
		} else {
			content := strings.Join(m.logLines, "\n")
			m.viewport.SetContent(content)
			if m.followLogs || wasAtBottom {
				newTotalLines := m.viewport.TotalLineCount()
				newVisibleLines := m.viewport.VisibleLineCount()
				if newTotalLines > newVisibleLines {
					m.viewport.SetYOffset(newTotalLines - newVisibleLines)
				}
			} else {
				m.viewport.SetYOffset(oldYOffset)
			}
		}
		return m, tickCmd()
	case healthMsg:
		m.healthBusy = false
		if msg.err == nil {
			m.health = msg.icons
			m.healthDetails = msg.details
			m.healthLast = time.Now()
		}
		return m, tickCmd()
	}

	if m.mode == viewModeLogs || m.mode == viewModeLogsDebug {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		if cmd != nil {
			return m, cmd
		}
	}

	return m, nil
}

func (m *topModel) clearLogsView() {
	m.mode = viewModeTable
	m.logLines = nil
	m.logErr = nil
	m.logSvc = nil
	m.logPID = 0
}
