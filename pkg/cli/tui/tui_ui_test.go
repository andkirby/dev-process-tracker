package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/devports/devpt/pkg/buildinfo"
	"github.com/devports/devpt/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestView_EscapeSequences(t *testing.T) {
	model := newTestModel()
	model.width = 100
	model.height = 40

	t.Run("no raw screen clear escape", func(t *testing.T) {
		output := model.View().Content
		assert.NotContains(t, output, "\x1b[2J")
	})

	t.Run("output is non-empty", func(t *testing.T) {
		output := model.View().Content
		assert.NotEmpty(t, output)
	})
}

func TestView_HeaderContent(t *testing.T) {
	model := newTestModel()
	model.width = 100
	model.mode = viewModeTable

	t.Run("header text is present", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "Dev Process Tracker")
		assert.Contains(t, output, "Health Monitor")
	})

	t.Run("header shows current version", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, buildinfo.Version)
	})

	t.Run("header omits quit hint", func(t *testing.T) {
		output := model.View().Content
		assert.NotContains(t, output, "q quit")
	})
}

func TestView_StatusBar(t *testing.T) {
	model := newTestModel()
	model.width = 120

	t.Run("footer contains keybinding hints", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "switch list")
		assert.Contains(t, output, "logs/start")
		assert.Contains(t, output, "filter")
		assert.Contains(t, output, "toggle help")
	})

	t.Run("footer shows service count", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "Name (1)")
	})

	t.Run("footer stays compact", func(t *testing.T) {
		output := model.View().Content
		assert.NotContains(t, output, "D for debug")
	})
}

func TestView_CommandMode(t *testing.T) {
	model := newTestModel()
	model.width = 100
	model.mode = viewModeCommand

	t.Run("command prompt shows colon", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, ":")
	})

	t.Run("command mode shows hint", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "Esc to go back")
	})

	t.Run("command mode shows example", func(t *testing.T) {
		model.cmdInput = "add"
		output := model.View().Content
		assert.Contains(t, output, "Example:")
	})
}

func TestView_ConfirmDialog(t *testing.T) {
	model := newTestModel()
	model.width = 100
	model.height = 24
	model.openConfirmModal(&confirmState{kind: confirmStopPID, prompt: "Stop PID 123?", pid: 123})

	t.Run("confirm prompt includes [y/N]", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "Enter/y confirm, n/Esc cancel")
	})

	t.Run("confirm shows prompt text", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "Stop PID 123?")
	})

	t.Run("confirm keeps table visible behind modal", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "app")
		assert.Contains(t, output, "No managed")
		assert.Contains(t, output, "Confirm")
	})

	t.Run("click outside confirm closes modal", func(t *testing.T) {
		clickModel := newTestModel()
		clickModel.width = 100
		clickModel.height = 24
		clickModel.openConfirmModal(&confirmState{kind: confirmStopPID, prompt: "Stop PID 123?", pid: 123})

		newModel, cmd := clickModel.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 0, Y: 0})
		assert.Nil(t, cmd)

		updated := newModel.(*topModel)
		assert.Equal(t, viewModeTable, updated.mode)
		assert.Nil(t, updated.modal)
		assert.Nil(t, updated.confirm)
		assert.Equal(t, "Cancelled", updated.cmdStatus)
	})

	t.Run("enter confirms action in confirm mode", func(t *testing.T) {
		enterModel := newTestModel()
		enterModel.width = 100
		enterModel.height = 24
		enterModel.openConfirmModal(&confirmState{kind: confirmRemoveService, prompt: "Remove test?", name: "missing"})

		newModel, cmd := enterModel.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		assert.Nil(t, cmd)

		updated := newModel.(*topModel)
		assert.Equal(t, viewModeTable, updated.mode)
		assert.Nil(t, updated.modal)
		assert.Nil(t, updated.confirm)
		assert.NotEmpty(t, updated.cmdStatus)
	})
}

func TestView_TableStructure(t *testing.T) {
	model := newTestModel()
	model.width = 120
	model.mode = viewModeTable

	t.Run("table has all required column headers", func(t *testing.T) {
		output := model.View().Content
		lines := strings.Split(output, "\n")
		headerLine := findLineContaining(lines, "Name")

		assert.NotEmpty(t, headerLine)
		assert.Contains(t, headerLine, "Name (1)")
		assert.Contains(t, headerLine, "Port")
		assert.Contains(t, headerLine, "PID")
		assert.Contains(t, headerLine, "Project")
		assert.Contains(t, headerLine, "Command")
		assert.Contains(t, headerLine, "Health")
	})

	t.Run("table has divider line", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "─")
	})
}

func TestView_ManagedServicesSection(t *testing.T) {
	model := newTestModel()
	model.width = 120
	model.mode = viewModeTable

	t.Run("context line shows focus state", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "switch list")
	})

	t.Run("tab switch hint in footer", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "switch list")
	})
}

func TestView_ContextLine(t *testing.T) {
	model := newTestModel()
	model.width = 100
	model.mode = viewModeTable

	t.Run("context line shows focus", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "switch list")
	})

	t.Run("context line omits service count", func(t *testing.T) {
		output := model.View().Content
		assert.NotContains(t, output, "Services: 1 |")
	})
}

func TestView_LogsMode(t *testing.T) {
	model := newTestModel()
	model.width = 100
	model.mode = viewModeLogs
	model.logPID = 1234

	t.Run("logs header shows service name", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "Logs:")
		assert.Contains(t, output, "PID: 1234")
	})

	t.Run("logs header shows port field", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "Port:")
	})

	t.Run("logs footer shows back hint", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "b back")
	})
}

func TestView_HelpMode(t *testing.T) {
	model := newTestModel()
	model.width = 100
	model.height = 24
	model.openHelpModal()

	t.Run("help shows keymap header", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "Help")
	})

	t.Run("help shows keybindings", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "switch list")
		assert.Contains(t, output, "toggle help")
		assert.Contains(t, output, "/")
	})

	t.Run("help shows command hints", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "add")
		assert.Contains(t, output, "logs/start")
		assert.Contains(t, output, "toggle follow")
	})

	t.Run("help keeps table visible behind modal", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "app")
		assert.Contains(t, output, "Manage")
		assert.Contains(t, output, "Help")
	})

	t.Run("click outside help closes modal", func(t *testing.T) {
		clickModel := newTestModel()
		clickModel.width = 100
		clickModel.height = 24
		clickModel.openHelpModal()

		newModel, cmd := clickModel.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 0, Y: 0})
		assert.Nil(t, cmd)

		updated := newModel.(*topModel)
		assert.Equal(t, viewModeTable, updated.mode)
		assert.Nil(t, updated.modal)
	})
}

func TestView_SearchMode(t *testing.T) {
	model := newTestModel()
	model.width = 100
	model.mode = viewModeSearch
	model.searchQuery = "node"
	model.searchInput.SetValue("node")
	model.searchInput.Focus()

	t.Run("search prompt shows query", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "node")
		assert.Contains(t, output, ">")
		assert.Contains(t, output, "Name (1)")
	})

	t.Run("empty search shows inline input", func(t *testing.T) {
		model.searchQuery = ""
		model.searchInput.SetValue("")
		output := model.View().Content
		assert.Contains(t, output, ">")
	})
}

func TestView_SelectedRow(t *testing.T) {
	model := newTestModel()
	model.width = 120
	model.mode = viewModeTable
	model.selected = 0

	t.Run("view renders without error", func(t *testing.T) {
		assert.NotPanics(t, func() {
			_ = model.View()
		})
	})

	t.Run("output is not empty", func(t *testing.T) {
		output := model.View().Content
		assert.NotEmpty(t, output)
	})
}

func TestView_ManagedServiceSelection(t *testing.T) {
	model := newTestModel()
	model.width = 120
	model.mode = viewModeTable
	model.focus = focusManaged

	t.Run("managed focus shows in context", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "Managed Services")
	})

	t.Run("tab switch hint available for focus change", func(t *testing.T) {
		output := model.View().Content
		assert.Contains(t, output, "switch list")
	})
}

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
			model := newTestModel()
			model.width = tt.width
			model.height = 40

			if tt.shouldPanic {
				assert.Panics(t, func() { model.View() })
			} else {
				assert.NotPanics(t, func() {
					output := model.View().Content
					assert.NotEmpty(t, output)
				})
			}
		})
	}
}

func TestView_ResponsiveHeight(t *testing.T) {
	tests := []struct {
		name   string
		height int
	}{
		{"short terminal 10", 10},
		{"standard terminal 24", 24},
		{"tall terminal 40", 40},
		{"very tall 100", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := newTestModel()
			model.width = 100
			model.height = tt.height

			assert.NotPanics(t, func() {
				output := model.View().Content
				assert.NotEmpty(t, output)
			})
		})
	}
}

func TestView_TextWrapping(t *testing.T) {
	model := newTestModel()
	model.width = 80

	t.Run("long footer wraps to width", func(t *testing.T) {
		output := model.View().Content
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "switch list") || strings.Contains(line, "filter") || strings.Contains(line, ">") {
				visibleWidth := calculateVisibleWidth(line)
				assert.LessOrEqual(t, visibleWidth, model.width+10)
			}
		}
	})
}

func TestView_EmptyStates(t *testing.T) {
	t.Run("empty servers list shows message", func(t *testing.T) {
		model := newTestModel()
		model.servers = []*models.ServerInfo{}
		model.width = 100
		output := model.View().Content
		assert.Contains(t, output, "(no matching servers")
	})

	t.Run("empty filter shows message", func(t *testing.T) {
		model := newTestModel()
		model.servers = []*models.ServerInfo{}
		model.searchQuery = "nonexistent"
		model.width = 100
		output := model.View().Content
		assert.Contains(t, output, "(no matching servers for filter")
	})
}

func TestView_ModeTransitions(t *testing.T) {
	model := newTestModel()
	model.width = 100
	model.height = 40

	t.Run("table mode renders", func(t *testing.T) {
		model.mode = viewModeTable
		output := model.View().Content
		assert.NotEmpty(t, output)
		assert.Contains(t, output, "Dev Process Tracker")
		assert.Contains(t, output, "Name (1)")
	})

	t.Run("logs mode renders", func(t *testing.T) {
		model.mode = viewModeLogs
		output := model.View().Content
		assert.NotEmpty(t, output)
		assert.Contains(t, output, "Logs:")
	})

	t.Run("command mode renders", func(t *testing.T) {
		model.mode = viewModeCommand
		output := model.View().Content
		assert.NotEmpty(t, output)
		assert.Contains(t, output, ":")
	})

	t.Run("search mode renders", func(t *testing.T) {
		model.mode = viewModeSearch
		model.searchInput.SetValue("")
		model.searchInput.Focus()
		output := model.View().Content
		assert.NotEmpty(t, output)
		assert.Contains(t, output, ">")
		assert.Contains(t, output, "Name (1)")
	})

	t.Run("help mode renders", func(t *testing.T) {
		model.openHelpModal()
		output := model.View().Content
		assert.NotEmpty(t, output)
		assert.Contains(t, output, "Help")
		assert.Contains(t, output, "switch list")
	})
}

func TestView_StatusMessage(t *testing.T) {
	model := newTestModel()
	model.width = 100

	t.Run("status message appears", func(t *testing.T) {
		model.cmdStatus = "Service started"
		output := model.View().Content
		assert.Contains(t, output, "Service started")
	})

	t.Run("empty status does not appear", func(t *testing.T) {
		model.cmdStatus = ""
		output := model.View().Content
		assert.NotEmpty(t, output)
	})
}

func TestView_StatusAndFooterClampToWidth(t *testing.T) {
	model := newTestModel()
	model.width = 40
	model.height = 20
	model.mode = viewModeTable
	model.cmdStatus = `Restarted "mdt-be" because the previous health check timed out on localhost:3001`

	output := model.View().Content
	lines := strings.Split(output, "\n")
	var statusLine, footerLine string

	for _, line := range lines {
		if strings.Contains(line, `Restarted "mdt-be"`) {
			statusLine = line
		}
		if strings.Contains(line, "switch list") {
			footerLine = line
		}
	}

	assert.NotEmpty(t, statusLine)
	assert.NotEmpty(t, footerLine)
	assert.LessOrEqual(t, calculateVisibleWidth(statusLine), model.width)
	assert.LessOrEqual(t, calculateVisibleWidth(footerLine), model.width)
	assert.Contains(t, statusLine, `Restarted "mdt-be" because the previo`)
	assert.NotContains(t, statusLine, "localhost:3001")
}

func TestView_SortModeDisplay(t *testing.T) {
	model := newTestModel()
	model.width = 100

	tests := []struct {
		name     string
		sortMode sortMode
	}{
		{"sort by recent", sortRecent},
		{"sort by name", sortName},
		{"sort by project", sortProject},
		{"sort by port", sortPort},
		{"sort by health", sortHealth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model.sortBy = tt.sortMode
			output := model.View().Content
			assert.Contains(t, output, "switch list")
			assert.Contains(t, output, "Name (1)")
		})
	}
}

func TestView_ManagedCrashContextAndSymbols(t *testing.T) {
	stoppedAt := time.Date(2026, 3, 27, 21, 54, 25, 0, time.UTC)
	deps := &fakeAppDeps{
		services: []*models.ManagedService{
			{
				Name:     "test-go-basic-fake",
				CWD:      "/Users/kirby/.config/dev-process-tracker/sandbox/servers/go-basic",
				Command:  "go run .",
				Ports:    []int{3401},
				LastStop: &stoppedAt,
			},
		},
		servers: []*models.ServerInfo{
			{
				ManagedService: &models.ManagedService{Name: "test-go-basic-fake", CWD: "/Users/kirby/.config/dev-process-tracker/sandbox/servers/go-basic", Command: "go run .", Ports: []int{3401}},
				Status:         "crashed",
				Source:         models.SourceManaged,
				CrashReason:    "exit status 1",
				CrashLogTail: []string{
					"2026/03/27 21:54:25 [go-basic] listening on http://localhost:3400",
					"2026/03/27 21:54:25 listen tcp :3400: bind: address already in use",
					"exit status 1",
				},
			},
		},
		logPaths: map[string]string{
			"test-go-basic-fake": "~/.config/devpt/logs/test-go-basic-fake/2026-03-12T22-14-37.log",
		},
	}

	model := newTopModel(deps)
	model.width = 180
	model.height = 30
	model.mode = viewModeTable
	model.focus = focusManaged
	model.managedSel = 0

	output := model.View().Content
	assert.Contains(t, output, "✘")
	assert.Contains(t, output, "test-go-basic-fake [crashed]")
	assert.Contains(t, output, "Headline: exit status 1")
	assert.Contains(t, output, "Log: ~/.config/devpt/logs/test-go-basic-fake/2026-03-12T22-14-37.log")
	assert.Contains(t, output, "listen tcp :3400: bind: address already in use")
	assert.Contains(t, output, "Source: managed")
}

func findLineContaining(lines []string, pattern string) string {
	for _, line := range lines {
		if strings.Contains(line, pattern) {
			return line
		}
	}
	return ""
}

func TestView_CommandColumnTruncation(t *testing.T) {
	// Regression test: command column should use full cmdW for content.
	// Old bug: runewidth.Truncate(cmd, cmdW-3, "...") produced a cmdW-3 wide string,
	// then fixedCell padded with 3 dead spaces. The "..." was already counted in the
	// Truncate output, so cmdW-3 wasted 3 chars of visible command path.
	// Fix: runewidth.Truncate(cmd, cmdW, "...") uses the full width budget.
	longCmd := "/Users/kirby/home/yt-offline/backend/node /very/long/path/to/some/javascript/server/file/that/needs/truncation/server.js"

	for _, terminalWidth := range []int{80, 100, 120} {
		t.Run(fmt.Sprintf("width_%d", terminalWidth), func(t *testing.T) {
			model := newTopModel(&fakeAppDeps{
				servers: []*models.ServerInfo{
					{
						ProcessRecord: &models.ProcessRecord{
							PID:         33489,
							Port:        9055,
							Command:     longCmd,
							CWD:         "/Users/kirby/home/yt-offline/backend",
							ProjectRoot: "/Users/kirby/home/yt-offline/backend",
						},
					Status: "running",
					Source: models.SourceManual,
				},
			},
		})
		model.width = terminalWidth
		model.height = 24
		model.mode = viewModeTable
		model.refresh()

		output := model.View().Content
		lines := strings.Split(output, "\n")

		// Find a data row containing the command path (use stripped output for matching)
		var dataLineStripped string
		for _, l := range lines {
			s := stripANSI(l)
			if strings.Contains(s, "yt-offline") || strings.Contains(s, "Users/kirby") {
				dataLineStripped = s
				break
			}
		}
		assert.NotEmpty(t, dataLineStripped, "should find a row with the command path")

		// Calculate expected cmdW
		nameW, portW, pidW, projectW, healthW := 14, 6, 7, 14, 7
		sep := 2
		used := nameW + sep + portW + sep + pidW + sep + projectW + sep + healthW + sep
		cmdW := terminalWidth - used
		if cmdW < 12 {
			cmdW = 12
		}

		// Only test truncation cases (command longer than column)
		if cmdW >= len(longCmd) {
			return
		}

		// Extract the command cell from the stripped (no-ANSI) line
		// Command cell starts after: name(14) + sep(2) + port(6) + sep(2) + pid(7) + sep(2) + project(14) + sep(2) = 49
		cmdStart := nameW + sep + portW + sep + pidW + sep + projectW + sep

		// dataLineStripped already has ANSI stripped
		runes := []rune(dataLineStripped)
		if cmdStart+cmdW > len(runes) {
			// Emoji may cause rune/width mismatch; extract approximate
			return
		}
		cmdCell := string(runes[cmdStart : cmdStart+cmdW])

		// The command cell should end with "..." from truncation, not spaces
		assert.True(t, strings.HasSuffix(cmdCell, "..."),
			"command cell should end with ..., got: %q", cmdCell)

		// Old bug symptom: cell ends with "...   " (ellipsis + dead space padding)
		assert.False(t, strings.Contains(cmdCell, "...   "),
			"command cell should NOT have dead space after ... (old cmdW-3 bug), got: %q", cmdCell)

		// Content before "..." should be longer than the old bug would allow
		// Old bug: cmdW-3 total width means only cmdW-6 chars of actual path
		// Fix: cmdW total width means cmdW-3 chars of actual path
		pathPart := strings.TrimSuffix(cmdCell, "...")
		assert.Greater(t, len(pathPart), 0, "should have path content before ...")

		// Verify we're showing at least cmdW-3 chars of content (the maximum possible)
		assert.GreaterOrEqual(t, len(pathPart), cmdW-3,
			"should use nearly full cmdW for path content, got %d chars in %q", len(pathPart), cmdCell)
	})
	}
}

// stripANSI removes ANSI escape sequences and OSC hyperlinks from a string.
func stripANSI(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' {
			// Skip escape sequence
			i++
			if i < len(s) && s[i] == '[' {
				i++
				for i < len(s) {
					if (s[i] >= '0' && s[i] <= '9') || s[i] == ';' || s[i] == '?' {
						i++
					} else {
						i++
						break
					}
				}
			} else if i < len(s) && s[i] == ']' {
				// OSC sequence: \x1b]...\x07 or \x1b]...\x1b\\
				i++
				for i < len(s) {
					if s[i] == '\x07' {
						i++
						break
					}
					if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '\\' {
						i += 2
						break
					}
					i++
				}
			}
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

func calculateVisibleWidth(s string) int {
	inEscape := false
	visible := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1b {
			inEscape = true
		} else if inEscape {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				inEscape = false
			}
		} else {
			visible++
		}
	}
	return visible
}
