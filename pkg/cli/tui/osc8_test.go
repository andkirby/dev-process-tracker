package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- TEST-osc8-helper: OSC 8 helper function produces correct escape sequences ---

func TestOsc8Link_Format(t *testing.T) {
	link := osc8Link("3000", "http://localhost:3000")

	// Must contain the visible text
	assert.Contains(t, link, "3000")

	// Must start with OSC 8 sequence
	assert.True(t, strings.HasPrefix(link, "\x1b]8;;http://localhost:3000\x07"),
		"link should start with OSC 8 hyperlink escape")

	// Must end with OSC 8 reset sequence
	assert.True(t, strings.HasSuffix(link, "\x1b]8;;\x07"),
		"link should end with OSC 8 reset escape")

	// The visible width must be just the text (4 for "3000")
	assert.Equal(t, 4, ansi.StringWidth(link))
}

func TestOsc8Link_ZeroVisibleWidthForEscapes(t *testing.T) {
	// Verify that the escape sequences themselves have zero visible width
	open := ansi.SetHyperlink("http://localhost:3000")
	close_ := ansi.ResetHyperlink()
	assert.Equal(t, 0, ansi.StringWidth(open))
	assert.Equal(t, 0, ansi.StringWidth(close_))
}

// --- TEST-no-port-plain: Port dash renders as plain text without OSC 8 wrapping ---

func TestPortCell_DashRendersPlain(t *testing.T) {
	cell := portCell("-", 6)

	// Must be plain "-" with padding, no escape sequences
	assert.Equal(t, "-     ", cell)
	assert.Equal(t, 6, ansi.StringWidth(cell))
	assert.Equal(t, 6, len(cell)) // plain ASCII, no escapes
	assert.NotContains(t, cell, "\x1b]")
}

// --- TEST-layout-dimensions: Table column widths and layout remain unchanged ---

func TestFixedHyperlinkCell_Width(t *testing.T) {
	cell := fixedHyperlinkCell("3000", "http://localhost:3000", 6)

	// Visible width must be exactly the requested width
	assert.Equal(t, 6, ansi.StringWidth(cell))

	// Must contain the port number
	assert.Contains(t, cell, "3000")

	// Must contain OSC 8 escape sequences
	assert.Contains(t, cell, "\x1b]8;;")
}

func TestFixedHyperlinkCell_LongText(t *testing.T) {
	// If text exceeds width, it falls back to truncation without hyperlink
	cell := fixedHyperlinkCell("12345678", "http://localhost:12345678", 6)
	assert.Equal(t, 6, ansi.StringWidth(cell))
	// Truncated plain text, no OSC 8 escapes since it overflows
	assert.Equal(t, "123456", cell)
}

func TestFixedHyperlinkCell_ZeroWidth(t *testing.T) {
	cell := fixedHyperlinkCell("3000", "http://localhost:3000", 0)
	assert.Equal(t, "", cell)
}

func TestFixedHyperlinkCell_MatchesFixedCellForPlainText(t *testing.T) {
	// When there's no hyperlink, fixedCell and fixedHyperlinkCell should
	// produce the same visible result for the text portion
	plain := fixedCell("3000", 6)
	linked := fixedHyperlinkCell("3000", "http://localhost:3000", 6)

	// Both should have the same visible width
	assert.Equal(t, ansi.StringWidth(plain), ansi.StringWidth(linked))

	// The linked version should have escapes
	assert.True(t, len(linked) > len(plain))
	assert.Contains(t, linked, "\x1b]8;;")
}

// --- TEST-osc8-port-render: Port cell contains valid OSC 8 escape with correct URI ---

func TestPortCell_NumericPort(t *testing.T) {
	cell := portCell("3000", 6)

	// Visible width must be correct
	assert.Equal(t, 6, ansi.StringWidth(cell))

	// Must contain OSC 8 with correct URL
	assert.Contains(t, cell, "http://localhost:3000")

	// Must contain the visible port number
	assert.Contains(t, cell, "3000")

	// Must have opening and closing OSC 8 sequences
	assert.True(t, strings.Contains(cell, "\x1b]8;;http://localhost:3000\x07"))
	assert.True(t, strings.Contains(cell, "\x1b]8;;\x07"))
}

func TestPortCell_SingleDigitPort(t *testing.T) {
	cell := portCell("8", 6)
	assert.Equal(t, 6, ansi.StringWidth(cell))
	assert.Contains(t, cell, "http://localhost:8")
}

func TestPortCell_FiveDigitPort(t *testing.T) {
	cell := portCell("65535", 6)
	assert.Equal(t, 6, ansi.StringWidth(cell))
	assert.Contains(t, cell, "http://localhost:65535")
}

func TestPortCell_DashNoEscape(t *testing.T) {
	cell := portCell("-", 6)
	// No escape sequences for dash
	assert.Equal(t, "-     ", cell)
	require.Equal(t, 6, len(cell))
	for _, ch := range cell {
		// All characters should be printable ASCII (no escape chars)
		assert.True(t, ch >= 32 && ch <= 126, "unexpected non-printable char: %U", ch)
	}
}

func TestPortCell_HTTPSchemeOnly(t *testing.T) {
	// Verify constraint C-1: only http scheme, only localhost
	cell := portCell("3000", 6)
	assert.Contains(t, cell, "http://localhost:3000")
	assert.NotContains(t, cell, "https://")
}
