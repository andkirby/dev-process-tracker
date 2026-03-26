package cli

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/devports/devpt/pkg/health"
	"github.com/devports/devpt/pkg/models"
	"github.com/devports/devpt/pkg/process"
)

// TopCmd starts the interactive TUI mode (like 'top')
func (a *App) TopCmd() error {
	model := newTopModel(a)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

type viewMode int
type viewFocus int
type sortMode int
type confirmKind int

const (
	viewModeTable viewMode = iota
	viewModeLogs
	viewModeLogsDebug // Simple viewport test mode
	viewModeCommand
	viewModeSearch
	viewModeHelp
	viewModeConfirm
)

// Use viewport for table rendering
const useViewportForTable = true

const (
	focusRunning viewFocus = iota
	focusManaged
)

const (
	sortRecent sortMode = iota
	sortName
	sortProject
	sortPort
	sortHealth
	sortModeCount
)

const (
	confirmStopPID confirmKind = iota
	confirmRemoveService
	confirmSudoKill
)

type confirmState struct {
	kind        confirmKind
	prompt      string
	pid         int
	name        string
	serviceName string
}

// topModel represents the TUI state.
type topModel struct {
	app        *App
	servers    []*models.ServerInfo
	width      int
	height     int
	lastUpdate time.Time
	lastInput  time.Time
	err        error

	selected   int
	managedSel int
	focus      viewFocus
	mode       viewMode

	logLines   []string
	logErr     error
	logSvc     *models.ManagedService
	logPID     int
	followLogs bool

	cmdInput    string
	searchQuery string
	cmdStatus   string

	health           map[int]string
	healthDetails    map[int]*health.HealthCheck
	showHealthDetail bool
	healthBusy       bool
	healthLast       time.Time
	healthChk        *health.Checker

	sortBy sortMode

	starting map[string]time.Time
	removed  map[string]*models.ManagedService

	confirm *confirmState

	// Viewport state for logs view (M0 - walking skeleton)
	viewport           viewport.Model
	viewportNeedsTop    bool // Flag to reset viewport to top after sizing
	tableContentHash    string // Track table content to avoid unnecessary updates
	selectionChanged   bool  // Track if selection changed for scrolling
	lastSelected       int   // Track last selection to detect changes
	lastManagedSel     int   // Track last managed selection
	highlightIndex     int
	highlightMatches   []int

	// Double-click detection
	lastClickTime time.Time
	lastClickY    int
}

func newTopModel(app *App) *topModel {
	m := &topModel{
		app:           app,
		lastUpdate:    time.Now(),
		lastInput:     time.Now(),
		mode:          viewModeTable,
		focus:         focusRunning,
		followLogs:    false, // Disabled by default to avoid interfering with scrolling
		health:        make(map[int]string),
		healthDetails: make(map[int]*health.HealthCheck),
		healthChk:     health.NewChecker(800 * time.Millisecond),
		sortBy:        sortRecent,
		starting:      make(map[string]time.Time),
		removed:       make(map[string]*models.ManagedService),
		lastSelected:  -1,
		lastManagedSel: -1,
	}
	if servers, err := app.discoverServers(); err == nil {
		m.servers = servers
	}

	// Initialize viewport (M0 - walking skeleton)
	m.viewport = viewport.New(0, 0)
	m.highlightIndex = 0
	m.highlightMatches = []int{}

	return m
}

func (m topModel) Init() tea.Cmd {
	return tickCmd()
}

func (m *topModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.lastInput = time.Now()

		// Command mode - handle input first (from origin/main fix)
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
			for _, r := range msg.Runes {
				if r >= 32 && r != 127 {
					m.cmdInput += string(r)
				}
			}
			return m, nil
		}

		// Search mode - handle input first (from origin/main fix)
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
			for _, r := range msg.Runes {
				if r >= 32 && r != 127 {
					m.searchQuery += string(r)
				}
			}
			return m, nil
		}

		// In logs mode, let viewport handle scrolling keys first (BR-1.6)
		// Only intercept keys we explicitly handle (q, esc, b, f, n, N)
		if m.mode == viewModeLogs {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "esc", "b":
				m.mode = viewModeTable
				m.logLines = nil
				m.logErr = nil
				m.logSvc = nil
				m.logPID = 0
				// Invalidate table content hash to force viewport refresh when returning to table mode
				m.tableContentHash = ""
				return m, nil
			case "f":
				m.followLogs = !m.followLogs
				return m, nil
			case "n":
				if len(m.highlightMatches) > 0 {
					m.highlightIndex = (m.highlightIndex + 1) % len(m.highlightMatches)
				}
				return m, nil
			case "N":
				if len(m.highlightMatches) > 0 {
					m.highlightIndex = (m.highlightIndex - 1 + len(m.highlightMatches)) % len(m.highlightMatches)
				}
				return m, nil
			default:
				// Pass all other keys to viewport for scrolling (arrows, pgup/down, etc.)
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		}

		// Debug mode - simple viewport test
		if m.mode == viewModeLogsDebug {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "b", "esc":
				m.mode = viewModeTable
				// Invalidate table content hash to force viewport refresh when returning to table mode
				m.tableContentHash = ""
				return m, nil
			default:
				// Pass all keys to viewport
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		}

		// Table mode key handling
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			if m.mode == viewModeTable {
				if m.focus == focusRunning {
					m.focus = focusManaged
					// Ensure managed selection is valid
					managed := m.managedServices()
					if m.managedSel < 0 && len(managed) > 0 {
						m.managedSel = 0
					}
				} else {
					m.focus = focusRunning
					// Ensure running selection is valid
					visible := m.visibleServers()
					if m.selected < 0 && len(visible) > 0 {
						m.selected = 0
					}
				}
				m.selectionChanged = true
			}
			return m, nil
		case "?", "f1":
			if m.mode == viewModeTable {
				m.mode = viewModeHelp
			}
			return m, nil
		case "/":
			if m.mode == viewModeTable {
				m.mode = viewModeSearch
			}
			return m, nil
		case "ctrl+l":
			if m.mode == viewModeTable {
				m.searchQuery = ""
				m.cmdStatus = "Filter cleared"
			}
			return m, nil
		case "s":
			if m.mode == viewModeTable {
				m.sortBy = (m.sortBy + 1) % sortModeCount
			}
			return m, nil
		case "h":
			if m.mode == viewModeTable {
				m.showHealthDetail = !m.showHealthDetail
			}
			return m, nil
		case "D":
			if m.mode == viewModeTable {
				m.mode = viewModeLogsDebug
				m.initDebugViewport()
			}
			return m, nil
		case "f":
			if m.mode == viewModeLogs {
				m.followLogs = !m.followLogs
			}
			return m, nil
		case "ctrl+a":
			if m.mode == viewModeTable {
				m.mode = viewModeCommand
				m.cmdInput = "add "
			}
			return m, nil
		case "ctrl+r":
			if m.mode == viewModeTable {
				m.cmdStatus = m.restartSelected()
				m.refresh()
			}
			return m, nil
		case "ctrl+e":
			if m.mode == viewModeTable {
				m.prepareStopConfirm()
			}
			return m, nil
		case "x", "delete", "ctrl+d":
			if m.mode == viewModeTable && m.focus == focusManaged {
				managed := m.managedServices()
				if m.managedSel >= 0 && m.managedSel < len(managed) {
					name := managed[m.managedSel].Name
					m.confirm = &confirmState{
						kind:   confirmRemoveService,
						prompt: fmt.Sprintf("Remove %q from registry?", name),
						name:   name,
					}
					m.mode = viewModeConfirm
				} else {
					m.cmdStatus = "No managed service selected"
				}
			}
			return m, nil
		case ":", "shift+;", ";", "c":
			if m.mode == viewModeTable {
				m.mode = viewModeCommand
				m.cmdInput = ""
			}
			return m, nil
		case "esc":
			switch m.mode {
			case viewModeTable:
				return m, tea.Quit
			case viewModeLogs:
				m.mode = viewModeTable
				m.logLines = nil
				m.logErr = nil
				m.logSvc = nil
				m.logPID = 0
				// Invalidate table content hash to force viewport refresh when returning to table mode
				m.tableContentHash = ""
			case viewModeHelp, viewModeConfirm:
				m.mode = viewModeTable
				m.confirm = nil
			}
			return m, nil
		case "b":
			if m.mode == viewModeLogs {
				m.mode = viewModeTable
				m.logLines = nil
				m.logErr = nil
				m.logSvc = nil
				m.logPID = 0
				// Invalidate table content hash to force viewport refresh when returning to table mode
				m.tableContentHash = ""
				return m, nil
			}
			return m, nil
		case "backspace":
			return m, nil
		case "up", "k":
			if m.mode == viewModeTable {
				if m.focus == focusRunning && m.selected > 0 {
					m.selected--
					m.selectionChanged = true
				}
				if m.focus == focusManaged && m.managedSel > 0 {
					m.managedSel--
					m.selectionChanged = true
				}
			}
			return m, nil
		case "down", "j":
			if m.mode == viewModeTable {
				if m.focus == focusRunning {
					if m.selected < len(m.visibleServers())-1 {
						m.selected++
						m.selectionChanged = true
					}
				}
				if m.focus == focusManaged {
					if m.managedSel < len(m.managedServices())-1 {
						m.managedSel++
						m.selectionChanged = true
					}
				}
			}
			return m, nil
		case "y":
			if m.mode == viewModeConfirm {
				cmd := m.executeConfirm(true)
				return m, cmd
			}
			return m, nil
		case "n":
			if m.mode == viewModeConfirm {
				cmd := m.executeConfirm(false)
				return m, cmd
			}
			// Highlight cycling: 'n' moves to next highlight (BR-1.3)
			if m.mode == viewModeLogs && len(m.highlightMatches) > 0 {
				m.highlightIndex = (m.highlightIndex + 1) % len(m.highlightMatches)
				return m, nil
			}
			return m, nil
		case "N":
			// Highlight cycling: 'N' moves to previous highlight (BR-1.4)
			if m.mode == viewModeLogs && len(m.highlightMatches) > 0 {
				m.highlightIndex = (m.highlightIndex - 1 + len(m.highlightMatches)) % len(m.highlightMatches)
				return m, nil
			}
			return m, nil
		case "pgup", "pgdown", "home", "end":
			// In table mode, pass scrolling keys to viewport
			if m.mode == viewModeTable {
				var cmd tea.Cmd
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
			return m, nil
		case "enter":
			switch m.mode {
			case viewModeConfirm:
				cmd := m.executeConfirm(true)
				return m, cmd
			case viewModeTable:
				return m.handleEnterKey()
			}
			return m, nil
		default:
			return m, nil
		}
	case tea.MouseMsg:
		// Handle mouse click in table mode for selection
		if m.mode == viewModeTable {
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				return m.handleTableMouseClick(msg)
			}
			// Pass scroll/wheel events to viewport
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		// Handle mouse clicks in logs view mode
		if m.mode == viewModeLogs {
			// Click events (button press) are handled by our click handler
			if msg.Action == tea.MouseActionPress {
				return m.handleMouseClick(msg)
			}
			// All other mouse events (wheel, drag, release) go to viewport for scrolling
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		// Debug mode - pass all mouse events to viewport
		if m.mode == viewModeLogsDebug {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Don't return - let viewport receive this event too
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
		// Save current scroll position
		oldYOffset := m.viewport.YOffset
		totalLines := m.viewport.TotalLineCount()
		visibleLines := m.viewport.VisibleLineCount()
		wasAtBottom := (oldYOffset + visibleLines >= totalLines) || totalLines == 0

		m.logLines = msg.lines
		m.logErr = msg.err
		// Update viewport content with new log lines (DEVPT-002)
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

			// Restore scroll position or follow
			if m.followLogs || wasAtBottom {
				// If follow mode is on or we were at bottom, go to bottom
				newTotalLines := m.viewport.TotalLineCount()
				newVisibleLines := m.viewport.VisibleLineCount()
				if newTotalLines > newVisibleLines {
					m.viewport.SetYOffset(newTotalLines - newVisibleLines)
				}
			} else {
				// Otherwise, try to preserve user's scroll position
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

	// Pass events to viewport when in logs mode or debug mode (DEVPT-002)
	if m.mode == viewModeLogs || m.mode == viewModeLogsDebug {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		if cmd != nil {
			return m, cmd
		}
	}

	return m, nil
}

func (m *topModel) refresh() {
	if servers, err := m.app.discoverServers(); err == nil {
		m.servers = servers
		m.lastUpdate = time.Now()
		if m.selected >= len(m.visibleServers()) && len(m.visibleServers()) > 0 {
			m.selected = len(m.visibleServers()) - 1
		}
		if m.managedSel >= len(m.managedServices()) && len(m.managedServices()) > 0 {
			m.managedSel = len(m.managedServices()) - 1
		}
		for name, at := range m.starting {
			if m.isServiceRunning(name) || time.Since(at) > 45*time.Second {
				delete(m.starting, name)
			}
		}
	} else {
		m.err = err
	}
}

func (m *topModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress 'q' to quit\n", m.err)
	}

	width := m.width
	if width <= 0 {
		width = 120
	}

	var b strings.Builder
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)

	// Ensure stale lines are removed when viewport shrinks/resizes.
	b.WriteString("\x1b[H\x1b[2J")
	if m.mode == viewModeLogs {
		name := "-"
		if m.logSvc != nil {
			name = m.logSvc.Name
		} else if m.logPID > 0 {
			name = fmt.Sprintf("pid:%d", m.logPID)
		}
		b.WriteString(headerStyle.Render(fmt.Sprintf("Logs: %s (b back, f follow:%t)", name, m.followLogs)))
	} else if m.mode == viewModeLogsDebug {
		b.WriteString(headerStyle.Render("Viewport Debug Mode (b back, q quit)"))
	} else {
		b.WriteString(headerStyle.Render("Dev Process Tracker - Health Monitor (q quit, D for debug)"))
	}
	if m.mode == viewModeTable || m.mode == viewModeCommand || m.mode == viewModeSearch || m.mode == viewModeConfirm {
		focus := "running"
		if m.focus == focusManaged {
			focus = "managed"
		}
		filter := m.searchQuery
		if strings.TrimSpace(filter) == "" {
			filter = "none"
		}
		ctx := fmt.Sprintf("Focus: %s | Sort: %s | Filter: %s", focus, sortModeLabel(m.sortBy), filter)
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fitLine(ctx, width)))
		b.WriteString("\n")
	}

	switch m.mode {
	case viewModeHelp:
		b.WriteString(m.renderHelp(width))
	case viewModeLogs:
		b.WriteString(m.renderLogs(width))
	case viewModeLogsDebug:
		b.WriteString(m.renderLogsDebug(width))
	case viewModeTable:
		// Use viewport for table rendering
		b.WriteString(m.renderTableWithViewport(width))
		b.WriteString("\n")
	default:
		rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
		b.WriteString(rowStyle.Render(m.renderTable(width)))
		b.WriteString("\n\n")
		b.WriteString(m.renderManaged(width))
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
	if m.mode == viewModeConfirm && m.confirm != nil {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true).Render(fitLine(m.confirm.prompt+" [y/N]", width)))
		b.WriteString("\n")
	}
	var footer string
	var statusLine string

	// Build status line (orange, above footer)
	if m.cmdStatus != "" {
		statusLine = m.cmdStatus
	} else if m.mode == viewModeTable && m.focus == focusManaged {
		// Show crash reason for selected managed service
		managed := m.managedServices()
		if m.managedSel >= 0 && m.managedSel < len(managed) {
			svc := managed[m.managedSel]
			if reason := m.crashReasonForService(svc.Name); reason != "" {
				statusLine = fmt.Sprintf("Crash: %s", reason)
			}
		}
	}

	if m.mode == viewModeLogs && len(m.highlightMatches) > 0 {
		// Show match counter in logs view when highlights are active (BR-1.5)
		matchCounter := fmt.Sprintf("Match %d/%d", m.highlightIndex+1, len(m.highlightMatches))
		footer = fmt.Sprintf("%s | b back | f follow:%t | n/N next/prev highlight", matchCounter, m.followLogs)
	} else if m.mode == viewModeLogs {
		footer = fmt.Sprintf("b back | f follow:%t | ↑↓ scroll | Page Up/Down", m.followLogs)
	} else if m.mode == viewModeLogsDebug {
		footer = "b back | q quit | ↑↓ scroll | Page Up/Down"
	} else if m.mode == viewModeTable {
		footer = fmt.Sprintf("Services: %d | Tab switch | Enter logs/start | Page Up/Down scroll | / filter | ? help | D debug", m.countVisible())
	} else {
		footer = fmt.Sprintf("Last updated: %s | Services: %d | Tab switch | Enter logs/start | x remove managed | / filter | ^L clear filter | s sort | ? help | ^A add ^R restart ^E stop | D debug", m.lastUpdate.Format("15:04:05"), m.countVisible())
	}
	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)

	// Render status line (orange) above footer if present
	if statusLine != "" {
		statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
		b.WriteString(statusStyle.Render(fitLine(statusLine, width)))
		b.WriteString("\n")
	}

	b.WriteString(footerStyle.Render(fitLine(footer, width)))
	b.WriteString("\n")
	return b.String()
}

func (m topModel) renderTable(width int) string {
	visible := m.visibleServers()
	displayNames := m.displayNames(visible)
	nameW, portW, pidW, projectW, healthW := 14, 6, 7, 14, 7
	sep := 2
	used := nameW + sep + portW + sep + pidW + sep + projectW + sep + healthW + sep
	cmdW := width - used
	if cmdW < 12 {
		cmdW = 12
	}

	var lines []string
	header := fmt.Sprintf("%s%s%s%s%s%s%s%s%s%s%s",
		fixedCell("Name", nameW), strings.Repeat(" ", sep),
		fixedCell("Port", portW), strings.Repeat(" ", sep),
		fixedCell("PID", pidW), strings.Repeat(" ", sep),
		fixedCell("Project", projectW), strings.Repeat(" ", sep),
		fixedCell("Command", cmdW), strings.Repeat(" ", sep),
		fixedCell("Health", healthW),
	)
	divider := fmt.Sprintf("%s%s%s%s%s%s%s%s%s%s%s",
		fixedCell(strings.Repeat("─", nameW), nameW), strings.Repeat(" ", sep),
		fixedCell(strings.Repeat("─", portW), portW), strings.Repeat(" ", sep),
		fixedCell(strings.Repeat("─", pidW), pidW), strings.Repeat(" ", sep),
		fixedCell(strings.Repeat("─", projectW), projectW), strings.Repeat(" ", sep),
		fixedCell(strings.Repeat("─", cmdW), cmdW), strings.Repeat(" ", sep),
		fixedCell(strings.Repeat("─", healthW), healthW),
	)
	lines = append(lines, fitLine(header, width))
	lines = append(lines, fitLine(divider, width))

	rowFirstLineIdx := make([]int, len(visible))
	for i, srv := range visible {
		project := "-"
		if srv.ProcessRecord != nil {
			if srv.ProcessRecord.ProjectRoot != "" {
				project = pathBase(srv.ProcessRecord.ProjectRoot)
			} else if srv.ProcessRecord.CWD != "" {
				project = pathBase(srv.ProcessRecord.CWD)
			}
		}
		if project == "-" && srv.ManagedService != nil && srv.ManagedService.CWD != "" {
			project = pathBase(srv.ManagedService.CWD)
		}

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

		// Truncate command to one line with ellipsis
		truncatedCmd := cmd
		if runewidth.StringWidth(cmd) > cmdW {
			truncatedCmd = runewidth.Truncate(cmd, cmdW-3, "...")
		}

		rowFirstLineIdx[i] = len(lines)
		line := fmt.Sprintf("%s%s%s%s%s%s%s%s%s%s%s",
			fixedCell(displayNames[i], nameW), strings.Repeat(" ", sep),
			fixedCell(port, portW), strings.Repeat(" ", sep),
			fixedCell(fmt.Sprintf("%d", pid), pidW), strings.Repeat(" ", sep),
			fixedCell(project, projectW), strings.Repeat(" ", sep),
			fixedCell(truncatedCmd, cmdW), strings.Repeat(" ", sep),
			fixedCell(icon, healthW),
		)
		lines = append(lines, fitLine(line, width))
	}

	if len(visible) == 0 {
		if m.searchQuery != "" {
			return fitLine("(no matching servers for filter)", width)
		}
		return fitLine("(no matching servers)", width)
	}

	// Bounds check: selected index may be out of bounds when filtering reduces visible items
	if m.selected >= 0 && m.selected < len(visible) {
		selectedLine := rowFirstLineIdx[m.selected]
		if selectedLine >= 2 && selectedLine < len(lines) {
			lines[selectedLine] = lipgloss.NewStyle().Background(lipgloss.Color("57")).Foreground(lipgloss.Color("15")).Render(lines[selectedLine])
		}
	}

	out := strings.Join(lines, "\n")
	if m.showHealthDetail {
		if m.selected >= 0 && m.selected < len(visible) {
			port := 0
			if visible[m.selected].ProcessRecord != nil {
				port = visible[m.selected].ProcessRecord.Port
			}
			if d := m.healthDetails[port]; d != nil {
				out += "\n" + fitLine(fmt.Sprintf("Health detail: %s %dms %s", health.StatusIcon(d.Status), d.ResponseMs, d.Message), width)
			}
		}
	}
	return out
}

func fixedCell(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) > width {
		return runewidth.Truncate(s, width, "")
	}
	return s + strings.Repeat(" ", width-runewidth.StringWidth(s))
}

func wrapRunes(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	if s == "" {
		return []string{""}
	}
	var out []string
	rest := s
	for runewidth.StringWidth(rest) > width {
		chunk := runewidth.Truncate(rest, width, "")
		if chunk == "" {
			break
		}
		out = append(out, chunk)
		rest = strings.TrimPrefix(rest, chunk)
	}
	if rest != "" {
		out = append(out, rest)
	}
	return out
}

func wrapWords(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	lines := make([]string, 0, 4)
	cur := words[0]
	for _, w := range words[1:] {
		candidate := cur + " " + w
		if runewidth.StringWidth(candidate) <= width {
			cur = candidate
			continue
		}
		lines = append(lines, cur)
		// If a single word is longer than width, fall back to rune wrapping.
		if runewidth.StringWidth(w) > width {
			chunks := wrapRunes(w, width)
			if len(chunks) > 0 {
				lines = append(lines, chunks[:len(chunks)-1]...)
				cur = chunks[len(chunks)-1]
			} else {
				cur = w
			}
		} else {
			cur = w
		}
	}
	lines = append(lines, cur)
	return lines
}

func (m topModel) renderManaged(width int) string {
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
	// Render header with horizontal line on same line
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	text := "Managed Services (Tab focus, Enter start) "
	textWidth := runewidth.StringWidth(text)
	fillWidth := width - textWidth
	if fillWidth < 0 {
		fillWidth = 0
	}
	fill := strings.Repeat("─", fillWidth)
	line := text + fill
	b.WriteString(headerStyle.Render(fitLine(line, width)))
	b.WriteString("\n")
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
		if m.focus == focusManaged && i == m.managedSel {
			line = lipgloss.NewStyle().Background(lipgloss.Color("57")).Foreground(lipgloss.Color("15")).Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	if m.focus == focusManaged && m.managedSel >= 0 && m.managedSel < len(managed) {
		svc := managed[m.managedSel]
		// Don't show crash reason inline - it makes the list jumpy
		// Reason is shown in status line instead (below)
		_ = svc
		_ = m.crashReasonForService(svc.Name)
	}
	return b.String()
}

func (m *topModel) renderLogs(width int) string {
	// Calculate total space used by header and footer
	headerText := m.logsHeaderView()
	headerLines := 1 + strings.Count(headerText, "\n") // Count actual header lines

	// Footer takes approximately 2-3 lines depending on wrapping
	footerLines := 3

	// Calculate available height for viewport
	availableHeight := m.height - headerLines - footerLines
	if availableHeight < 5 {
		availableHeight = 5 // Minimum viewport height
	}

	m.viewport.Width = width
	m.viewport.Height = availableHeight

	// If we just entered logs mode, reset to top now that viewport is sized
	if m.viewportNeedsTop {
		m.viewport.GotoTop()
		m.viewportNeedsTop = false
	}

	return m.viewport.View()
}

// ensureSelectionVisible scrolls the viewport to show the selected item
func (m *topModel) ensureSelectionVisible() {
	visible := m.visibleServers()
	managed := m.managedServices()

	// Viewport content is renderTableContent() which outputs:
	// - renderTable(): header (line 0) + divider (line 1) + N data rows
	// - "\n\n": 2 blank lines
	// - renderManaged(): header + divider + N managed rows
	var selectedLine int
	if m.focus == focusRunning && m.selected >= 0 && m.selected < len(visible) {
		// Running table: header (0) + divider (1) + data rows starting at line 2
		selectedLine = 2 + m.selected
	} else if m.focus == focusManaged && m.managedSel >= 0 && m.managedSel < len(managed) {
		// After running section: 2 blank lines + managed header + divider + selected row
		runningSectionLines := 2 + len(visible) // header + divider + N rows
		selectedLine = runningSectionLines + 2 + 1 + 1 + m.managedSel // +2 for blank lines, +1 for header, +1 for divider
	} else {
		return
	}

	totalLines := m.viewport.TotalLineCount()
	visibleLines := m.viewport.VisibleLineCount()
	currentOffset := m.viewport.YOffset

	// Calculate desired offset with some padding above/below selection
	desiredOffset := selectedLine - visibleLines/3
	if desiredOffset < 0 {
		desiredOffset = 0
	}
	if desiredOffset > totalLines - visibleLines {
		desiredOffset = totalLines - visibleLines
	}

	// Only scroll if selection is outside visible area
	if selectedLine < currentOffset || selectedLine >= currentOffset + visibleLines {
		m.viewport.SetYOffset(desiredOffset)
	}
}

// renderTableWithViewport renders the table using the viewport component
func (m *topModel) renderTableWithViewport(width int) string {
	// Generate table content
	tableContent := m.renderTableContent(width)

	// Only update viewport content if it actually changed
	contentHash := fmt.Sprintf("%s-%d", tableContent, len(m.servers))
	if m.tableContentHash != contentHash {
		m.viewport.SetContent(tableContent)
		m.tableContentHash = contentHash
	}

	// Calculate available space for viewport
	headerHeight := 3 // Title (1) + newline (1) + context (1)
	footerHeight := 2 // Spacing newline (1) + footer line (1)

	// Calculate if we need space for status line
	hasStatus := false
	if m.cmdStatus != "" {
		hasStatus = true
	} else if m.mode == viewModeTable && m.focus == focusManaged {
		managed := m.managedServices()
		if m.managedSel >= 0 && m.managedSel < len(managed) {
			svc := managed[m.managedSel]
			if m.crashReasonForService(svc.Name) != "" {
				hasStatus = true
			}
		}
	}

	statusHeight := 0
	if hasStatus {
		statusHeight = 1
	}

	availableHeight := m.height - headerHeight - footerHeight - statusHeight
	if availableHeight < 5 {
		availableHeight = 5
	}

	m.viewport.Width = width
	m.viewport.Height = availableHeight

	// Only scroll to selection if it changed
	if m.selectionChanged {
		m.ensureSelectionVisible()
		m.selectionChanged = false
	}

	return m.viewport.View()
}

// renderTableContent generates the table content as a string
func (m *topModel) renderTableContent(width int) string {
	var b strings.Builder

	// Running services section
	b.WriteString(m.renderTable(width))
	b.WriteString("\n\n")

	// Managed services section
	b.WriteString(m.renderManaged(width))

	return b.String()
}

// initDebugViewport initializes the viewport with test content for debug mode
func (m *topModel) initDebugViewport() {
	// Generate 100 lines of test content
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, fmt.Sprintf("Debug Line %d: This is test content for viewport scrolling. Use arrow keys, page up/down, or mouse wheel to scroll. Press 'b' to exit debug mode.", i))
	}
	content := strings.Join(lines, "\n")
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
}

// renderLogsDebug renders the debug viewport mode
func (m *topModel) renderLogsDebug(width int) string {
	// Size viewport to available space
	headerHeight := 4 // Fixed height for debug header
	m.viewport.Width = width
	m.viewport.Height = m.height - headerHeight - 4 // -4 for footer

	return m.viewport.View()
}

// logsHeaderView returns the header string for logs view mode
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

func (m topModel) countVisible() int { return len(m.visibleServers()) }

func (m topModel) visibleServers() []*models.ServerInfo {
	var visible []*models.ServerInfo
	q := strings.ToLower(strings.TrimSpace(m.searchQuery))
	for _, srv := range m.servers {
		if srv == nil || srv.ProcessRecord == nil {
			continue
		}
		if srv.ManagedService == nil {
			if srv.ProcessRecord.Port == 0 || !isRuntimeCommand(srv.ProcessRecord.Command) {
				continue
			}
		}
		if q != "" {
			hay := strings.ToLower(fmt.Sprintf("%s %s %s %d %s %s",
				m.serviceNameFor(srv), projectOf(srv), srv.ProcessRecord.Command, srv.ProcessRecord.Port, srv.ProcessRecord.CWD, srv.ProcessRecord.ProjectRoot))
			if !strings.Contains(hay, q) {
				continue
			}
		}
		visible = append(visible, srv)
	}
	m.sortServers(visible)
	return visible
}

func (m topModel) managedServices() []*models.ManagedService {
	services := m.app.registry.ListServices()
	q := strings.ToLower(strings.TrimSpace(m.searchQuery))
	var filtered []*models.ManagedService
	for _, svc := range services {
		if q == "" || strings.Contains(strings.ToLower(svc.Name+" "+svc.CWD+" "+svc.Command), q) {
			filtered = append(filtered, svc)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name) })
	return filtered
}

func (m topModel) displayNames(servers []*models.ServerInfo) []string {
	base := make([]string, len(servers))
	projectToSvc := make(map[string]string)
	for _, svc := range m.app.registry.ListServices() {
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

func (m topModel) serviceNameFor(srv *models.ServerInfo) string {
	if srv == nil {
		return "-"
	}
	if srv.ManagedService != nil && srv.ManagedService.Name != "" {
		return srv.ManagedService.Name
	}
	if srv.ProcessRecord != nil {
		if srv.ProcessRecord.ProjectRoot != "" {
			return pathBase(srv.ProcessRecord.ProjectRoot)
		}
		if srv.ProcessRecord.CWD != "" {
			return pathBase(srv.ProcessRecord.CWD)
		}
		if srv.ProcessRecord.Command != "" {
			return pathBase(srv.ProcessRecord.Command)
		}
	}
	return "-"
}

func (m topModel) runCommand(input string) string {
	if input == "" {
		return ""
	}
	args, err := parseArgs(input)
	if err != nil || len(args) == 0 {
		return "Invalid command"
	}
	switch args[0] {
	case "help":
		m.mode = viewModeHelp
		return ""
	case "list":
		services := m.app.registry.ListServices()
		if len(services) == 0 {
			return "No managed services"
		}
		names := make([]string, 0, len(services))
		for _, svc := range services {
			names = append(names, svc.Name)
		}
		sort.Strings(names)
		return "Managed services: " + strings.Join(names, ", ")
	case "add":
		if len(args) < 4 {
			return "Usage: add <name> <cwd> \"<cmd>\" [ports...]"
		}
		name, cwd, cmd := args[1], args[2], args[3]
		var ports []int
		for _, p := range args[4:] {
			port, perr := strconv.Atoi(p)
			if perr != nil {
				return "Invalid port: " + p
			}
			ports = append(ports, port)
		}
		if err := m.app.AddCmd(name, cwd, cmd, ports); err != nil {
			return err.Error()
		}
		return fmt.Sprintf("Added %q", name)
	case "remove", "rm":
		if len(args) < 2 {
			return "Usage: remove <name>"
		}
		svc := m.app.registry.GetService(args[1])
		if svc == nil {
			return fmt.Sprintf("service %q not found", args[1])
		}
		m.confirm = &confirmState{kind: confirmRemoveService, prompt: fmt.Sprintf("Remove %q from registry?", svc.Name), name: svc.Name}
		m.mode = viewModeConfirm
		return ""
	case "restore":
		if len(args) < 2 {
			return "Usage: restore <name>"
		}
		svc := m.removed[args[1]]
		if svc == nil {
			return fmt.Sprintf("no removed service %q in this session", args[1])
		}
		if err := m.app.AddCmd(svc.Name, svc.CWD, svc.Command, svc.Ports); err != nil {
			return err.Error()
		}
		delete(m.removed, args[1])
		return fmt.Sprintf("Restored %q", args[1])
	case "start":
		if len(args) < 2 {
			return "Usage: start <name>"
		}
		if err := m.app.StartCmd(args[1]); err != nil {
			return err.Error()
		}
		m.starting[args[1]] = time.Now()
		return fmt.Sprintf("Started %q", args[1])
	case "stop":
		if len(args) < 2 {
			return "Usage: stop <name|--port PORT>"
		}
		if args[1] == "--port" {
			if len(args) < 3 {
				return "Usage: stop --port PORT"
			}
			if err := m.app.StopCmd(args[2]); err != nil {
				return err.Error()
			}
			return fmt.Sprintf("Stopped port %s", args[2])
		}
		if err := m.app.StopCmd(args[1]); err != nil {
			return err.Error()
		}
		return fmt.Sprintf("Stopped %q", args[1])
	default:
		return "Unknown command (type :help)"
	}
}

func (m topModel) startSelected() string {
	visible := m.visibleServers()
	if m.selected < 0 || m.selected >= len(visible) {
		return "No service selected"
	}
	srv := visible[m.selected]
	if srv.ManagedService == nil {
		return "Selected process is not a managed service"
	}
	if err := m.app.StartCmd(srv.ManagedService.Name); err != nil {
		return err.Error()
	}
	m.starting[srv.ManagedService.Name] = time.Now()
	return fmt.Sprintf("Started %q", srv.ManagedService.Name)
}

func (m topModel) restartSelected() string {
	visible := m.visibleServers()
	if m.selected < 0 || m.selected >= len(visible) {
		return "No service selected"
	}
	srv := visible[m.selected]
	if srv.ManagedService == nil {
		return "Selected process is not a managed service"
	}
	if err := m.app.RestartCmd(srv.ManagedService.Name); err != nil {
		return err.Error()
	}
	m.starting[srv.ManagedService.Name] = time.Now()
	return fmt.Sprintf("Restarted %q", srv.ManagedService.Name)
}

func (m *topModel) prepareStopConfirm() {
	visible := m.visibleServers()
	if m.selected < 0 || m.selected >= len(visible) {
		m.cmdStatus = "No service selected"
		return
	}
	srv := visible[m.selected]
	if srv.ProcessRecord == nil || srv.ProcessRecord.PID == 0 {
		m.cmdStatus = "No PID to stop"
		return
	}
	prompt := fmt.Sprintf("Stop PID %d?", srv.ProcessRecord.PID)
	serviceName := ""
	if srv.ManagedService != nil {
		prompt = fmt.Sprintf("Stop %q (PID %d)?", srv.ManagedService.Name, srv.ProcessRecord.PID)
		serviceName = srv.ManagedService.Name
	}
	m.confirm = &confirmState{kind: confirmStopPID, prompt: prompt, pid: srv.ProcessRecord.PID, serviceName: serviceName}
	m.mode = viewModeConfirm
}

func (m *topModel) executeConfirm(yes bool) tea.Cmd {
	if m.confirm == nil {
		m.mode = viewModeTable
		return nil
	}
	c := *m.confirm
	m.confirm = nil
	m.mode = viewModeTable
	if !yes {
		m.cmdStatus = "Cancelled"
		return nil
	}
	switch c.kind {
	case confirmStopPID:
		if err := m.app.processManager.Stop(c.pid, 5*time.Second); err != nil {
			if errors.Is(err, process.ErrNeedSudo) {
				m.confirm = &confirmState{kind: confirmSudoKill, prompt: fmt.Sprintf("Run sudo kill -9 %d now?", c.pid), pid: c.pid}
				m.mode = viewModeConfirm
				return nil
			}
			if isProcessFinishedErr(err) {
				m.cmdStatus = fmt.Sprintf("Process %d already exited", c.pid)
				if c.serviceName != "" {
					_ = m.app.registry.ClearServicePID(c.serviceName)
				}
			} else {
				m.cmdStatus = err.Error()
			}
		} else {
			m.cmdStatus = fmt.Sprintf("Stopped PID %d", c.pid)
			if c.serviceName != "" {
				if clrErr := m.app.registry.ClearServicePID(c.serviceName); clrErr != nil {
					m.cmdStatus = fmt.Sprintf("Stopped PID %d (warning: %v)", c.pid, clrErr)
				}
			}
		}
	case confirmRemoveService:
		svc := m.app.registry.GetService(c.name)
		if svc != nil {
			copySvc := *svc
			m.removed[c.name] = &copySvc
		}
		if err := m.app.RemoveCmd(c.name); err != nil {
			m.cmdStatus = err.Error()
		} else {
			m.cmdStatus = fmt.Sprintf("Removed %q (use :restore %s)", c.name, c.name)
		}
	case confirmSudoKill:
		m.cmdStatus = fmt.Sprintf("Run manually: sudo kill -9 %d", c.pid)
	}
	m.refresh()
	return nil
}

func (m topModel) tailLogsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.logSvc != nil {
			lines, err := m.app.processManager.Tail(m.logSvc.Name, 200)
			return logMsg{lines: lines, err: err}
		}
		if m.logPID > 0 {
			lines, err := m.app.processManager.TailProcess(m.logPID, 200)
			return logMsg{lines: lines, err: err}
		}
		return logMsg{err: fmt.Errorf("no service selected")}
	}
}

func (m topModel) healthCmd() tea.Cmd {
	visible := m.visibleServers()
	return func() tea.Msg {
		icons := make(map[int]string)
		details := make(map[int]*health.HealthCheck)
		for _, srv := range visible {
			if srv.ProcessRecord == nil || srv.ProcessRecord.Port <= 0 {
				continue
			}
			check := m.healthChk.Check(srv.ProcessRecord.Port)
			icons[srv.ProcessRecord.Port] = health.StatusIcon(check.Status)
			details[srv.ProcessRecord.Port] = check
		}
		return healthMsg{icons: icons, details: details}
	}
}

type tickMsg time.Time
type logMsg struct {
	lines []string
	err   error
}
type healthMsg struct {
	icons   map[int]string
	details map[int]*health.HealthCheck
	err     error
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func parseArgs(input string) ([]string, error) {
	var args []string
	var buf strings.Builder
	inQuotes := false
	var quote rune
	escaped := false
	for _, r := range input {
		if escaped {
			buf.WriteRune(r)
			escaped = false
			continue
		}
		switch r {
		case '\\':
			escaped = true
		case '"', '\'':
			if inQuotes && r == quote {
				inQuotes = false
				quote = 0
			} else if !inQuotes {
				inQuotes = true
				quote = r
			} else {
				buf.WriteRune(r)
			}
		case ' ', '\t':
			if inQuotes {
				buf.WriteRune(r)
			} else if buf.Len() > 0 {
				args = append(args, buf.String())
				buf.Reset()
			}
		default:
			buf.WriteRune(r)
		}
	}
	if buf.Len() > 0 {
		args = append(args, buf.String())
	}
	return args, nil
}

func fitLine(line string, width int) string {
	if width <= 0 {
		return line
	}
	lineWidth := runewidth.StringWidth(line)
	if lineWidth == width {
		return line
	}
	if lineWidth > width {
		// Let the terminal wrap long lines to the viewport instead of truncating.
		return line
	}
	return line + strings.Repeat(" ", width-lineWidth)
}

func pathBase(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "-"
	}
	if strings.Contains(raw, " ") {
		raw = strings.Fields(raw)[0]
	}
	raw = strings.TrimRight(raw, "/")
	parts := strings.Split(raw, "/")
	if len(parts) == 0 {
		return "-"
	}
	base := parts[len(parts)-1]
	if base == "" {
		return "-"
	}
	return base
}

func projectOf(srv *models.ServerInfo) string {
	if srv == nil || srv.ProcessRecord == nil {
		return ""
	}
	if srv.ProcessRecord.ProjectRoot != "" {
		return pathBase(srv.ProcessRecord.ProjectRoot)
	}
	return pathBase(srv.ProcessRecord.CWD)
}

func portOf(srv *models.ServerInfo) int {
	if srv == nil || srv.ProcessRecord == nil {
		return 0
	}
	return srv.ProcessRecord.Port
}

func pidOf(srv *models.ServerInfo) int {
	if srv == nil || srv.ProcessRecord == nil {
		return 0
	}
	return srv.ProcessRecord.PID
}

func isRuntimeCommand(raw string) bool {
	base := strings.ToLower(pathBase(raw))
	switch base {
	case "node", "nodejs", "npm", "npx", "pnpm", "yarn", "bun", "bunx", "deno",
		"vite", "webpack", "webpack-dev-server", "next", "next-server", "nuxt", "ts-node", "tsx",
		"python", "python3", "pip", "pipenv", "poetry",
		"ruby", "rails",
		"go",
		"java", "javac", "gradle", "mvn",
		"dotnet",
		"php":
		return true
	default:
		return false
	}
}

func sortModeLabel(s sortMode) string {
	switch s {
	case sortName:
		return "name"
	case sortProject:
		return "project"
	case sortPort:
		return "port"
	case sortHealth:
		return "health"
	default:
		return "recent"
	}
}

func (m topModel) isServiceRunning(name string) bool {
	for _, srv := range m.servers {
		if srv.ManagedService != nil && srv.ManagedService.Name == name && srv.ProcessRecord != nil && srv.ProcessRecord.PID > 0 {
			return true
		}
	}
	return false
}

func (m topModel) serviceStatus(name string) string {
	for _, srv := range m.servers {
		if srv.ManagedService != nil && srv.ManagedService.Name == name {
			if srv.Status != "" {
				return srv.Status
			}
		}
	}
	if m.isServiceRunning(name) {
		return "running"
	}
	return "stopped"
}

func (m topModel) crashReasonForService(name string) string {
	for _, srv := range m.servers {
		if srv.ManagedService != nil && srv.ManagedService.Name == name && srv.Status == "crashed" {
			return srv.CrashReason
		}
	}
	return ""
}

// calculateGutterWidth calculates the gutter width based on total line count.
// The gutter shows line numbers and is used for mouse click navigation.
func (m topModel) calculateGutterWidth() int {
	totalLines := m.viewport.TotalLineCount()
	if totalLines <= 0 {
		return 0
	}
	// Calculate width needed for the largest line number
	width := len(strconv.Itoa(totalLines))
	// Add padding for space after line number
	return width + 1
}

// handleMouseClick processes mouse click events for the logs viewport.
// Gutter clicks (left side) jump to the clicked line.
// Text area clicks (right of gutter) center the clicked line in the viewport.
func (m *topModel) handleMouseClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Only handle button press events (not release or motion)
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}

	// Only handle left mouse button
	if msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	// Check if we have any content
	if len(m.logLines) == 0 {
		return m, nil
	}

	// Calculate gutter width
	gutterWidth := m.calculateGutterWidth()

	// Determine if click is in gutter or text area
	clickedInGutter := msg.X < gutterWidth

	// Calculate which line was clicked (relative to viewport)
	// msg.Y is the row within the viewport
	clickedLine := msg.Y

	// Adjust for viewport's current offset to get absolute line number
	absoluteLine := clickedLine + m.viewport.YOffset

	// Ensure the line is within valid range
	if absoluteLine < 0 || absoluteLine >= len(m.logLines) {
		return m, nil
	}

	if clickedInGutter {
		// Gutter click: jump viewport so clicked line is at top
		m.viewport.GotoTop()
		// Use LineDown to position the clicked line at the top
		m.viewport.LineDown(absoluteLine)
	} else {
		// Text click: center the clicked line in viewport
		visibleLines := m.viewport.VisibleLineCount()
		if visibleLines > 0 {
			// Calculate offset to center the line
			centerOffset := absoluteLine - (visibleLines / 2)
			if centerOffset < 0 {
				centerOffset = 0
			}
			m.viewport.SetYOffset(centerOffset)
		}
	}

	return m, nil
}

// handleEnterKey processes the Enter key action for the current selection.
// For running services: opens logs view
// For managed services: starts the service
func (m *topModel) handleEnterKey() (tea.Model, tea.Cmd) {
	if m.focus == focusManaged {
		managed := m.managedServices()
		if m.managedSel >= 0 && m.managedSel < len(managed) {
			if err := m.app.StartCmd(managed[m.managedSel].Name); err != nil {
				m.cmdStatus = err.Error()
			} else {
				name := managed[m.managedSel].Name
				m.cmdStatus = fmt.Sprintf("Started %q", name)
				m.starting[name] = time.Now()
			}
			m.refresh()
			return m, nil
		}
	}
	if m.focus == focusRunning {
		visible := m.visibleServers()
		if m.selected >= 0 && m.selected < len(visible) {
			srv := visible[m.selected]
			if srv.ManagedService == nil {
				m.mode = viewModeLogs
				m.logSvc = nil
				m.logPID = srv.ProcessRecord.PID
				m.viewportNeedsTop = true
				return m, m.tailLogsCmd()
			}
			m.mode = viewModeLogs
			m.logSvc = srv.ManagedService
			m.logPID = 0
			m.viewportNeedsTop = true
			return m, m.tailLogsCmd()
		}
	}
	return m, nil
}

// handleTableMouseClick processes mouse click events for the table view.
// It determines which row was clicked and updates the selection accordingly.
// Double-click on a running service opens logs (equivalent to pressing Enter).
func (m *topModel) handleTableMouseClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	visible := m.visibleServers()
	managed := m.managedServices()

	// Screen layout before viewport:
	// - Line 0: Title ("Dev Process Tracker - Health Monitor...")
	// - Line 1: Context ("Focus: running | Sort: recent...")
	// - Line 2+: Viewport content starts here
	//
	// msg.Y is screen-relative, so we need to subtract header offset
	// to get viewport-relative Y coordinate.
	headerOffset := 2 // Title (1) + Context (1)

	// Convert screen Y to viewport-relative Y
	viewportY := msg.Y - headerOffset
	if viewportY < 0 {
		return m, nil // Click was in header area
	}

	// Calculate absolute line number within viewport content
	absoluteLine := viewportY + m.viewport.YOffset

	// Table content layout (within viewport):
	// Running section:
	//   - Header line (0)
	//   - Divider line (1)
	//   - Data rows (2 to 2+len(visible)-1)
	//   - Blank lines (2+len(visible), 2+len(visible)+1)
	// Managed section:
	//   - Header line (2+len(visible)+2)
	//   - Data rows starting at (2+len(visible)+3)

	runningDataStart := 2
	runningDataEnd := runningDataStart + len(visible) - 1
	blankLinesEnd := runningDataEnd + 1 // +1 for blank line between sections (the "\n\n" creates 1 visual blank line)
	managedHeaderLine := blankLinesEnd + 1
	managedDataStart := managedHeaderLine + 1

	// Check for double-click (same Y position within 500ms)
	const doubleClickThreshold = 500 * time.Millisecond
	isDoubleClick := !m.lastClickTime.IsZero() &&
		time.Since(m.lastClickTime) < doubleClickThreshold &&
		m.lastClickY == msg.Y

	// Update last click tracking
	m.lastClickTime = time.Now()
	m.lastClickY = msg.Y

	// Check if click is in running services section
	if absoluteLine >= runningDataStart && absoluteLine <= runningDataEnd {
		newSelected := absoluteLine - runningDataStart
		if newSelected >= 0 && newSelected < len(visible) {
			// If double-click on running service, open logs (Enter key behavior)
			if isDoubleClick && m.selected == newSelected {
				m.focus = focusRunning
				m.selectionChanged = true
				m.lastInput = time.Now()
				// Trigger Enter key behavior - open logs for running service
				return m.handleEnterKey()
			}
			m.selected = newSelected
			m.focus = focusRunning
			m.selectionChanged = true
			m.lastInput = time.Now()
		}
		return m, nil
	}

	// Check if click is in managed services section
	if absoluteLine >= managedDataStart {
		newManagedSel := absoluteLine - managedDataStart
		if newManagedSel >= 0 && newManagedSel < len(managed) {
			// If double-click on managed service, open logs (Enter key behavior)
			if isDoubleClick && m.managedSel == newManagedSel {
				m.focus = focusManaged
				m.selectionChanged = true
				m.lastInput = time.Now()
				// Trigger Enter key behavior - open logs for managed service
				return m.handleEnterKey()
			}
			m.managedSel = newManagedSel
			m.focus = focusManaged
			m.selectionChanged = true
			m.lastInput = time.Now()
		}
	}

	return m, nil
}
