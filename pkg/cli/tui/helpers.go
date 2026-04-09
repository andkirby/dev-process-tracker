package tui

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/devports/devpt/pkg/models"
)

func fixedCell(s string, width int) string {
	if width <= 0 {
		return ""
	}
	w := runewidth.StringWidth(s)
	if w > width {
		return runewidth.Truncate(s, width, "")
	}
	return s + strings.Repeat(" ", width-w)
}

// osc8Link wraps text in an OSC 8 hyperlink escape sequence.
// Terminals that support OSC 8 will make the text clickable, opening the given URL.
// Unsupported terminals silently display the plain text.
func osc8Link(text, url string) string {
	return ansi.SetHyperlink(url) + text + ansi.ResetHyperlink()
}

// fixedHyperlinkCell wraps text in an OSC 8 hyperlink and pads it to the given
// visible width. Uses ansi.StringWidth which correctly strips escape sequences
// for width calculation (unlike runewidth.StringWidth which does not).
func fixedHyperlinkCell(text, url string, width int) string {
	if width <= 0 {
		return ""
	}
	linked := osc8Link(text, url)
	visibleWidth := ansi.StringWidth(linked)
	if visibleWidth >= width {
		// Text exceeds cell width — truncate the plain text (strip escapes for display)
		truncated := ansi.Truncate(text, width, "")
		return truncated + strings.Repeat(" ", width-ansi.StringWidth(truncated))
	}
	return linked + strings.Repeat(" ", width-visibleWidth)
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
	if lineWidth > width {
		if width <= 3 {
			return runewidth.Truncate(line, width, "")
		}
		return runewidth.Truncate(line, width, "...")
	}
	return line + strings.Repeat(" ", width-lineWidth)
}

func formatPorts(ports []int) string {
	if len(ports) == 0 {
		return ""
	}
	strs := make([]string, len(ports))
	for i, p := range ports {
		strs[i] = strconv.Itoa(p)
	}
	return strings.Join(strs, ", ")
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

func isProcessFinishedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "process already finished") || strings.Contains(msg, "no such process")
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

func (m topModel) serverInfoForService(name string) *models.ServerInfo {
	for _, srv := range m.servers {
		if srv.ManagedService != nil && srv.ManagedService.Name == name {
			return srv
		}
	}
	return nil
}

func (m topModel) selectedManagedService() *models.ManagedService {
	managed := m.managedServices()
	if m.managedSel < 0 || m.managedSel >= len(managed) {
		return nil
	}
	return managed[m.managedSel]
}

func managedStatusSymbol(state string) string {
	switch state {
	case "running":
		return "▶"
	case "crashed":
		return "✘"
	case "starting":
		return "…"
	default:
		return "■"
	}
}

func managedStatusColor(state string) string {
	switch state {
	case "running":
		return "10"
	case "crashed":
		return "9"
	case "starting":
		return "11"
	default:
		return "8"
	}
}

func nonEmptyTail(lines []string, n int) []string {
	if n <= 0 || len(lines) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			filtered = append(filtered, line)
		}
	}
	if len(filtered) <= n {
		return filtered
	}
	return filtered[len(filtered)-n:]
}

func (m topModel) calculateGutterWidth() int {
	totalLines := m.viewport.TotalLineCount()
	if totalLines <= 0 {
		return 0
	}
	width := len(strconv.Itoa(totalLines))
	return width + 1
}

func (m *topModel) handleMouseClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	mouse := msg.Mouse()
	if mouse.Button != tea.MouseLeft {
		return m, nil
	}
	if len(m.logLines) == 0 {
		return m, nil
	}

	gutterWidth := m.calculateGutterWidth()
	clickedInGutter := mouse.X < gutterWidth
	clickedLine := mouse.Y
	absoluteLine := clickedLine + m.viewport.YOffset()

	if absoluteLine < 0 || absoluteLine >= len(m.logLines) {
		return m, nil
	}

	if clickedInGutter {
		m.viewport.SetYOffset(absoluteLine)
	} else {
		visibleLines := m.viewport.VisibleLineCount()
		if visibleLines > 0 {
			centerOffset := absoluteLine - (visibleLines / 2)
			if centerOffset < 0 {
				centerOffset = 0
			}
			m.viewport.SetYOffset(centerOffset)
		}
	}

	return m, nil
}

func (m *topModel) handleEnterKey() (tea.Model, tea.Cmd) {
	if m.focus == focusManaged {
		managed := m.managedServices()
		if m.managedSel >= 0 && m.managedSel < len(managed) {
			if err := m.app.StartCmd(managed[m.managedSel].Name); err != nil {
				m.cmdStatus = err.Error()
			} else {
				name := managed[m.managedSel].Name
				m.cmdStatus = "Started " + strconv.Quote(name)
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
			m.mode = viewModeLogs
			if srv.ManagedService == nil {
				m.logSvc = nil
				m.logPID = srv.ProcessRecord.PID
			} else {
				m.logSvc = srv.ManagedService
				m.logPID = 0
			}
			m.viewportNeedsTop = true
			return m, m.tailLogsCmd()
		}
	}
	return m, nil
}

func (m *topModel) handleTableMouseClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	visible := m.visibleServers()
	managed := m.managedServices()
	mouse := msg.Mouse()

	headerOffset := m.tableTopLines(m.width)
	// Bubble Tea mouse row coordinates are effectively one line below our table math.
	viewportY := mouse.Y - headerOffset + 1
	if viewportY < 0 {
		return m, nil
	}

	// Check if click is on the header row (line 0 in running viewport)
	if viewportY < m.table.lastRunningHeight {
		absoluteLine := viewportY + m.table.runningYOffset()
		if absoluteLine == 0 {
			if col := m.columnAtX(mouse.X); col >= 0 {
				m.cycleSort(col)
				m.lastInput = time.Now()
				return m, nil
			}
		}
	}

	runningDataStart := 2

	const doubleClickThreshold = 500 * time.Millisecond
	isDoubleClick := !m.lastClickTime.IsZero() &&
		time.Since(m.lastClickTime) < doubleClickThreshold &&
		m.lastClickY == mouse.Y

	m.lastClickTime = time.Now()
	m.lastClickY = mouse.Y

	if viewportY < m.table.lastRunningHeight {
		absoluteLine := viewportY + m.table.runningYOffset()
		runningDataEnd := runningDataStart + len(visible) - 1
		if absoluteLine < runningDataStart || absoluteLine > runningDataEnd {
			return m, nil
		}
		newSelected := absoluteLine - runningDataStart
		if newSelected >= 0 && newSelected < len(visible) {
			if isDoubleClick && m.selected == newSelected {
				m.focus = focusRunning
				m.tableFollowSelection = true
				m.lastInput = time.Now()
				return m.handleEnterKey()
			}
			m.focus = focusRunning
			m.selected = newSelected
			m.tableFollowSelection = true
			m.groupHighlightNamespace = nil
			m.lastInput = time.Now()
		}
		return m, nil
	}

	// Managed header sits directly above the managed viewport content.
	if viewportY == m.table.lastRunningHeight {
		return m, nil
	}

	managedViewportY := viewportY - m.table.lastRunningHeight - 1
	if managedViewportY < 0 || managedViewportY >= m.table.lastManagedHeight {
		return m, nil
	}

	absoluteManagedLine := managedViewportY + m.table.managedYOffset()
	newManagedSel := absoluteManagedLine
	if newManagedSel >= 0 && newManagedSel < len(managed) {
		if isDoubleClick && m.managedSel == newManagedSel {
			m.focus = focusManaged
			m.tableFollowSelection = true
			m.lastInput = time.Now()
			if mouse.Mod&tea.ModShift != 0 {
				m.prepareGroupStartConfirm()
				return m, nil
			}
			return m.handleEnterKey()
		}
		m.groupHighlightNamespace = nil
		m.focus = focusManaged
		m.managedSel = newManagedSel
		m.tableFollowSelection = true
		m.lastInput = time.Now()
	}

	return m, nil
}
