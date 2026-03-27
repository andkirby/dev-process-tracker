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

func (m *topModel) renderConfirmModal(width int) string {
	if m.confirm == nil {
		return ""
	}

	boxWidth := width - 8
	if boxWidth > 72 {
		boxWidth = 72
	}
	if boxWidth < 24 {
		boxWidth = width
	}

	bodyWidth := boxWidth - 4
	if bodyWidth < 8 {
		bodyWidth = boxWidth
	}

	content := strings.Join([]string{
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Render("Confirm"),
		fitLine(m.confirm.prompt, bodyWidth),
		lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fitLine("Enter/y confirm, n/Esc cancel", bodyWidth)),
	}, "\n")

	return lipgloss.NewStyle().
		Width(boxWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("11")).
		Padding(0, 1).
		Render(content)
}

func overlayConfirmModal(background, overlay string, width int) string {
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

func (m *topModel) confirmModalBounds(width int) modalBounds {
	background := m.baseViewContent(width)
	bgLines := strings.Split(strings.TrimRight(background, "\n"), "\n")
	ovLines := strings.Split(m.renderConfirmModal(width), "\n")
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
