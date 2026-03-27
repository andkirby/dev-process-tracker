package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type modalBounds struct {
	x      int
	y      int
	width  int
	height int
}

func (m *topModel) openHelpModal() {
	m.modal = &modalState{kind: modalHelp}
}

func (m *topModel) openConfirmModal(confirm *confirmState) {
	m.confirm = confirm
	m.modal = &modalState{kind: modalConfirm}
}

func (m *topModel) closeModal() {
	m.modal = nil
	m.confirm = nil
}

func (m *topModel) activeModalKind() modalKind {
	if m.modal == nil {
		return 0
	}
	return m.modal.kind
}

func renderModal(title, body, hint string, width, maxWidth int, accent string) string {
	boxWidth := width - 8
	if maxWidth > 0 && boxWidth > maxWidth {
		boxWidth = maxWidth
	}
	if boxWidth < 24 {
		boxWidth = width
	}

	bodyWidth := boxWidth - 4
	if bodyWidth < 8 {
		bodyWidth = boxWidth
	}

	lines := []string{
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(accent)).Render(title),
	}
	for _, line := range strings.Split(body, "\n") {
		lines = append(lines, fitAnsiLine(line, bodyWidth))
	}
	if hint != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fitAnsiLine(hint, bodyWidth)))
	}
	content := strings.Join(lines, "\n")

	return lipgloss.NewStyle().
		Width(boxWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(accent)).
		Padding(0, 1).
		Render(content)
}

func (m *topModel) renderConfirmModal(width int) string {
	if m.confirm == nil {
		return ""
	}
	return renderModal("Confirm", m.confirm.prompt, "Enter/y confirm, n/Esc cancel", width, 72, "11")
}

func (m *topModel) renderHelpModal(width int) string {
	h := m.help
	boxWidth := width - 12
	if boxWidth > 96 {
		boxWidth = 96
	}
	if boxWidth < 36 {
		boxWidth = width
	}
	h.ShowAll = true
	h.SetWidth(boxWidth - 4)

	body := strings.Join([]string{
		h.View(m.keys),
		"",
		"Commands: add, start, stop, remove, restore, list, help",
	}, "\n")
	return renderModal("Help", body, "Esc/? closes", width, boxWidth, "12")
}

func (m *topModel) activeModalOverlay(width int) string {
	switch m.activeModalKind() {
	case modalHelp:
		return m.renderHelpModal(width)
	case modalConfirm:
		return m.renderConfirmModal(width)
	default:
		return ""
	}
}

func overlayModal(background, overlay string, width int) string {
	bgLines := strings.Split(strings.TrimRight(background, "\n"), "\n")
	ovLines := strings.Split(overlay, "\n")
	if len(bgLines) == 0 || len(ovLines) == 0 {
		return background
	}

	bounds := calculateModalBounds(bgLines, ovLines, width)

	for i, line := range ovLines {
		targetY := bounds.y + i
		if targetY < 0 || targetY >= len(bgLines) {
			continue
		}
		left := ansi.Cut(bgLines[targetY], 0, bounds.x)
		rightStart := bounds.x + ansi.StringWidth(line)
		right := ""
		if rightStart < width {
			right = ansi.Cut(bgLines[targetY], rightStart, width)
		}
		bgLines[targetY] = padAnsiLine(left, bounds.x) + line + padAnsiLine(right, width-rightStart)
	}

	return strings.Join(bgLines, "\n") + "\n"
}

func (m *topModel) activeModalBounds(width int, background string) modalBounds {
	overlay := m.activeModalOverlay(width)
	bgLines := strings.Split(strings.TrimRight(background, "\n"), "\n")
	ovLines := strings.Split(overlay, "\n")
	return calculateModalBounds(bgLines, ovLines, width)
}

func calculateModalBounds(bgLines, ovLines []string, width int) modalBounds {
	bounds := modalBounds{}
	if len(bgLines) == 0 || len(ovLines) == 0 {
		return bounds
	}

	bounds.height = len(ovLines)
	bounds.y = (len(bgLines) - bounds.height) / 2
	if bounds.y < 0 {
		bounds.y = 0
	}

	for _, line := range ovLines {
		if w := ansi.StringWidth(line); w > bounds.width {
			bounds.width = w
		}
	}

	bounds.x = (width - bounds.width) / 2
	if bounds.x < 0 {
		bounds.x = 0
	}

	return bounds
}

func (b modalBounds) contains(x, y int) bool {
	return x >= b.x && x < b.x+b.width && y >= b.y && y < b.y+b.height
}

func padAnsiLine(line string, targetWidth int) string {
	width := ansi.StringWidth(line)
	if width >= targetWidth {
		return line
	}
	return line + strings.Repeat(" ", targetWidth-width)
}

func fitAnsiLine(line string, targetWidth int) string {
	if targetWidth <= 0 {
		return line
	}
	if ansi.StringWidth(line) > targetWidth {
		return ansi.Truncate(line, targetWidth, "...")
	}
	return padAnsiLine(line, targetWidth)
}
