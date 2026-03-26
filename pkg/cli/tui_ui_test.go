package cli

import (
	"strings"
	"testing"

	"github.com/devports/devpt/pkg/models"
	"github.com/stretchr/testify/assert"
)

// Phase 1: Escape Sequence Verification Tests

func TestView_EscapeSequences(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	model := newTopModel(app)
	model.width = 100
	model.height = 40

	t.Run("screen clear sequence present", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "\x1b[H\x1b[2J", "View should clear screen with ANSI escape sequence")
	})

	t.Run("contains escape sequences", func(t *testing.T) {
		output := model.View()
		// Check for any ANSI escape sequence (starts with ESC)
		assert.Contains(t, output, "\x1b[", "View should contain ANSI escape codes")
	})
}

func TestView_HeaderContent(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	model := newTopModel(app)
	model.width = 100
	model.mode = viewModeTable

	t.Run("header text is present", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "Dev Process Tracker", "Should show app title")
		assert.Contains(t, output, "Health Monitor", "Should show subtitle")
	})

	t.Run("header contains quit hint", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "q quit", "Should show quit hint in header")
	})
}

func TestView_StatusBar(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	model := newTopModel(app)
	model.width = 120

	t.Run("footer contains keybinding hints", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "Tab switch", "Should show Tab hint")
		assert.Contains(t, output, "Enter logs/start", "Should show Enter hint")
		assert.Contains(t, output, "/ filter", "Should show filter hint")
		assert.Contains(t, output, "? help", "Should show help hint")
	})

	t.Run("footer shows service count", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "Services:", "Should show service count")
	})

	t.Run("footer shows debug shortcut", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "D debug", "Should show debug hint")
	})
}

func TestView_CommandMode(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	model := newTopModel(app)
	model.width = 100
	model.mode = viewModeCommand

	t.Run("command prompt shows colon", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, ":", "Should show command prompt with colon")
	})

	t.Run("command mode shows hint", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "Esc to go back", "Should show back hint")
	})

	t.Run("command mode shows example", func(t *testing.T) {
		model.cmdInput = "add"
		output := model.View()
		assert.Contains(t, output, "Example:", "Should show command example")
	})
}

func TestView_ConfirmDialog(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	model := newTopModel(app)
	model.width = 100
	model.mode = viewModeConfirm
	model.confirm = &confirmState{
		kind:   confirmStopPID,
		prompt: "Stop PID 123?",
		pid:    123,
	}

	t.Run("confirm prompt includes [y/N]", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "[y/N]", "Should show confirmation options")
	})

	t.Run("confirm shows prompt text", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "Stop PID 123?", "Should show confirm prompt")
	})
}

// Phase 2: Layout & Structure Tests

func TestView_TableStructure(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	model := newTopModel(app)
	model.width = 120
	model.mode = viewModeTable

	t.Run("table has all required column headers", func(t *testing.T) {
		output := model.View()
		lines := strings.Split(output, "\n")
		headerLine := findLineContaining(lines, "Name")

		assert.NotEmpty(t, headerLine, "Should find header line with 'Name'")
		assert.Contains(t, headerLine, "Name", "Should have Name column")
		assert.Contains(t, headerLine, "Port", "Should have Port column")
		assert.Contains(t, headerLine, "PID", "Should have PID column")
		assert.Contains(t, headerLine, "Project", "Should have Project column")
		assert.Contains(t, headerLine, "Command", "Should have Command column")
		assert.Contains(t, headerLine, "Health", "Should have Health column")
	})

	t.Run("table has divider line", func(t *testing.T) {
		output := model.View()
		// Divider uses em-dash characters
		assert.Contains(t, output, "─", "Should have divider line")
	})
}

func TestView_ManagedServicesSection(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	model := newTopModel(app)
	model.width = 120
	model.mode = viewModeTable

	// In viewModeTable, managed services are shown in the unified table with a context line
	// The "Managed Services" section header is only shown in non-table modes (command, search, confirm)
	t.Run("context line shows focus state", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "Focus:", "Should show focus indicator")
	})

	t.Run("tab switch hint in footer", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "Tab switch", "Should show Tab switch hint in footer")
	})
}

func TestView_ContextLine(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	model := newTopModel(app)
	model.width = 100
	model.mode = viewModeTable

	t.Run("context line shows focus", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "Focus:", "Should show focus indicator")
		assert.Contains(t, output, "Sort:", "Should show sort mode")
		assert.Contains(t, output, "Filter:", "Should show filter status")
	})

	t.Run("context line shows 'running' focus by default", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "Focus: running", "Default focus should be running")
	})
}

func TestView_LogsMode(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	model := newTopModel(app)
	model.width = 100
	model.mode = viewModeLogs
	model.logPID = 1234

	t.Run("logs header shows service name", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "Logs:", "Should show logs header")
		assert.Contains(t, output, "pid:1234", "Should show PID for unmanaged service")
	})

	t.Run("logs header shows follow status", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "follow:", "Should show follow status")
	})

	t.Run("logs header shows back hint", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "b back", "Should show back hint")
	})
}

func TestView_HelpMode(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	model := newTopModel(app)
	model.width = 100
	model.mode = viewModeHelp

	t.Run("help shows keymap header", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "Keymap", "Should show keymap section")
	})

	t.Run("help shows keybindings", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "q quit", "Should show quit keybinding")
		assert.Contains(t, output, "Tab switch", "Should show Tab keybinding")
		assert.Contains(t, output, "/ filter", "Should show filter keybinding")
	})

	t.Run("help shows command hints", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "Commands:", "Should show commands section")
		assert.Contains(t, output, "add", "Should show add command")
		assert.Contains(t, output, "start", "Should show start command")
		assert.Contains(t, output, "stop", "Should show stop command")
	})
}

func TestView_SearchMode(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	model := newTopModel(app)
	model.width = 100
	model.mode = viewModeSearch
	model.searchQuery = "node"

	t.Run("search prompt shows query", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "/node", "Should show search prompt with query")
	})

	t.Run("empty search shows slash", func(t *testing.T) {
		model.searchQuery = ""
		output := model.View()
		assert.Contains(t, output, "/", "Should show search prompt")
	})
}

func TestView_SelectedRow(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	model := newTopModel(app)
	model.width = 120
	model.mode = viewModeTable
	model.selected = 0

	t.Run("view renders without error", func(t *testing.T) {
		assert.NotPanics(t, func() {
			_ = model.View()
		}, "View should not panic with selected row")
	})

	t.Run("output is not empty", func(t *testing.T) {
		output := model.View()
		assert.NotEmpty(t, output, "View output should not be empty")
	})
}

func TestView_ManagedServiceSelection(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	model := newTopModel(app)
	model.width = 120
	model.mode = viewModeTable
	model.focus = focusManaged

	t.Run("managed focus shows in context", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "Focus: managed", "Context should show managed focus")
	})

	t.Run("tab switch hint available for focus change", func(t *testing.T) {
		output := model.View()
		assert.Contains(t, output, "Tab switch", "Should show Tab switch for changing focus")
	})
}

// Phase 3: Responsive Layout Tests

func TestView_ResponsiveWidth(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		shouldPanic bool
	}{
		{"narrow terminal 80", 80, false},
		{"standard terminal 100", 100, false},
		{"wide terminal 120", 120, false},
		{"very wide 200", 200, false},
		{"edge case zero", 0, false},
		{"edge case small", 40, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, err := NewApp()
			if err != nil {
				t.Fatalf("Failed to create app: %v", err)
			}
			model := newTopModel(app)
			model.width = tt.width
			model.height = 40

			if tt.shouldPanic {
				assert.Panics(t, func() { model.View() }, "Should panic at width %d", tt.width)
			} else {
				assert.NotPanics(t, func() { output := model.View(); assert.NotEmpty(t, output) },
					"Should not panic at width %d", tt.width)
			}
		})
	}
}

func TestView_ResponsiveHeight(t *testing.T) {
	tests := []struct {
		name  string
		height int
	}{
		{"short terminal 10", 10},
		{"standard terminal 24", 24},
		{"tall terminal 40", 40},
		{"very tall 100", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, err := NewApp()
			if err != nil {
				t.Fatalf("Failed to create app: %v", err)
			}
			model := newTopModel(app)
			model.width = 100
			model.height = tt.height

			assert.NotPanics(t, func() {
				output := model.View()
				assert.NotEmpty(t, output)
			}, "Should not panic at height %d", tt.height)
		})
	}
}

func TestView_TextWrapping(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	model := newTopModel(app)
	model.width = 80

	t.Run("long footer wraps to width", func(t *testing.T) {
		output := model.View()
		lines := strings.Split(output, "\n")

		// Find footer lines (those after "Last updated")
		for _, line := range lines {
			if strings.Contains(line, "Last updated") {
				// Line should not exceed terminal width significantly
				// (accounting for ANSI codes which are invisible)
				visibleWidth := calculateVisibleWidth(line)
				assert.LessOrEqual(t, visibleWidth, model.width+10,
					"Footer line should wrap to fit width")
			}
		}
	})
}

func TestView_EmptyStates(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	t.Run("empty servers list shows message", func(t *testing.T) {
		model := newTopModel(app)
		model.servers = []*models.ServerInfo{}
		model.width = 100
		output := model.View()

		assert.Contains(t, output, "(no matching servers", "Should show empty state message")
	})

	t.Run("empty filter shows message", func(t *testing.T) {
		model := newTopModel(app)
		model.servers = []*models.ServerInfo{}
		model.searchQuery = "nonexistent"
		model.width = 100
		output := model.View()

		assert.Contains(t, output, "(no matching servers for filter", "Should show filter empty message")
	})
}

func TestView_ModeTransitions(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	model := newTopModel(app)
	model.width = 100
	model.height = 40

	t.Run("table mode renders", func(t *testing.T) {
		model.mode = viewModeTable
		output := model.View()
		assert.NotEmpty(t, output)
		assert.Contains(t, output, "Dev Process Tracker")
	})

	t.Run("logs mode renders", func(t *testing.T) {
		model.mode = viewModeLogs
		output := model.View()
		assert.NotEmpty(t, output)
		assert.Contains(t, output, "Logs:")
	})

	t.Run("command mode renders", func(t *testing.T) {
		model.mode = viewModeCommand
		output := model.View()
		assert.NotEmpty(t, output)
		assert.Contains(t, output, ":")
	})

	t.Run("search mode renders", func(t *testing.T) {
		model.mode = viewModeSearch
		output := model.View()
		assert.NotEmpty(t, output)
		assert.Contains(t, output, "/")
	})

	t.Run("help mode renders", func(t *testing.T) {
		model.mode = viewModeHelp
		output := model.View()
		assert.NotEmpty(t, output)
		assert.Contains(t, output, "Keymap")
	})
}

func TestView_StatusMessage(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	model := newTopModel(app)
	model.width = 100

	t.Run("status message appears", func(t *testing.T) {
		model.cmdStatus = "Service started"
		output := model.View()
		assert.Contains(t, output, "Service started", "Should show status message")
	})

	t.Run("empty status does not appear", func(t *testing.T) {
		model.cmdStatus = ""
		output := model.View()
		// Output should still be valid, just without status message
		assert.NotEmpty(t, output, "View should still render without status")
	})
}

func TestView_SortModeDisplay(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}
	model := newTopModel(app)
	model.width = 100

	tests := []struct {
		name     string
		sortMode sortMode
		label    string
	}{
		{"sort by recent", sortRecent, "recent"},
		{"sort by name", sortName, "name"},
		{"sort by project", sortProject, "project"},
		{"sort by port", sortPort, "port"},
		{"sort by health", sortHealth, "health"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model.sortBy = tt.sortMode
			output := model.View()
			assert.Contains(t, output, "Sort: "+tt.label, "Should show sort mode")
		})
	}
}

// Helper functions

// findLineContaining finds the first line containing the specified pattern
func findLineContaining(lines []string, pattern string) string {
	for _, line := range lines {
		if strings.Contains(line, pattern) {
			return line
		}
	}
	return ""
}

// calculateVisibleWidth calculates the visible width of a string excluding ANSI escape codes
func calculateVisibleWidth(s string) int {
	inEscape := false
	visible := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1b { // ESC character
			inEscape = true
		} else if inEscape {
			// ANSI sequences end with letters (a-zA-Z)
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				inEscape = false
			}
		} else {
			visible++
		}
	}
	return visible
}
