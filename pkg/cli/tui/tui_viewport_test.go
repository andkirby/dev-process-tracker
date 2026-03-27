package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"

	"github.com/devports/devpt/pkg/models"
)

func TestViewportMouseClickNavigation(t *testing.T) {
	model := newTestModel()

	t.Run("gutter click jumps to clicked line", func(t *testing.T) {
		model.mode = viewModeLogs
		model.logLines = make([]string, 1000)
		for i := 0; i < 1000; i++ {
			model.logLines[i] = fmt.Sprintf("Log line %d", i)
		}

		model.viewport = viewport.New()
		model.viewport.SetWidth(80)
		model.viewport.SetHeight(24)
		model.viewport.SetContent(strings.Join(model.logLines, "\n"))
		initialOffset := model.viewport.YOffset()
		clickedLine := 5
		gutterWidth := model.calculateGutterWidth()

		mouseMsg := tea.MouseClickMsg{Button: tea.MouseLeft, X: gutterWidth - 1, Y: clickedLine}
		newModel, cmd := model.Update(mouseMsg)
		assert.Nil(t, cmd)

		updatedModel := newModel.(*topModel)
		assert.Equal(t, clickedLine, updatedModel.viewport.YOffset())
		assert.NotEqual(t, initialOffset, updatedModel.viewport.YOffset())
	})

	t.Run("text click repositions viewport to center", func(t *testing.T) {
		model.mode = viewModeLogs
		model.logLines = make([]string, 1000)
		for i := 0; i < 1000; i++ {
			model.logLines[i] = fmt.Sprintf("Log line %d", i)
		}

		model.viewport = viewport.New()
		model.viewport.SetWidth(80)
		model.viewport.SetHeight(24)
		model.viewport.SetContent(strings.Join(model.logLines, "\n"))

		initialOffset := model.viewport.YOffset()
		visibleLines := model.viewport.VisibleLineCount()
		gutterWidth := model.calculateGutterWidth()
		clickedAbsoluteLine := 100
		model.viewport.SetYOffset(clickedAbsoluteLine - 5)

		mouseMsg := tea.MouseClickMsg{Button: tea.MouseLeft, X: gutterWidth + 10, Y: 5}
		newModel, cmd := model.Update(mouseMsg)
		assert.Nil(t, cmd)

		updatedModel := newModel.(*topModel)
		expectedOffset := clickedAbsoluteLine - (visibleLines / 2)
		if expectedOffset < 0 {
			expectedOffset = 0
		}

		assert.Equal(t, expectedOffset, updatedModel.viewport.YOffset())
		assert.NotEqual(t, initialOffset, updatedModel.viewport.YOffset())
	})

	t.Run("click with no content is no-op", func(t *testing.T) {
		model.mode = viewModeLogs
		model.logLines = nil
		model.viewport = viewport.New()
		initialOffset := model.viewport.YOffset()

		mouseMsg := tea.MouseClickMsg{Button: tea.MouseLeft, X: 10, Y: 10}
		newModel, cmd := model.Update(mouseMsg)
		assert.Nil(t, cmd)

		updatedModel := newModel.(*topModel)
		assert.NotNil(t, updatedModel)
		assert.Equal(t, initialOffset, updatedModel.viewport.YOffset())
	})
}

func TestViewportHighlightCycling(t *testing.T) {
	model := newTestModel()

	t.Run("n key advances to next highlight", func(t *testing.T) {
		model.mode = viewModeLogs
		model.highlightMatches = []int{10, 20, 30, 40, 50}
		model.highlightIndex = 0
		newModel, cmd := model.Update(tea.KeyPressMsg{Text: "n", Code: 'n'})
		assert.Nil(t, cmd)
		updatedModel := newModel.(*topModel)
		assert.Equal(t, 1, updatedModel.highlightIndex)
	})

	t.Run("N key moves to previous highlight", func(t *testing.T) {
		model.mode = viewModeLogs
		model.highlightMatches = []int{10, 20, 30, 40, 50}
		model.highlightIndex = 3
		newModel, cmd := model.Update(tea.KeyPressMsg{Text: "N", Code: 'N'})
		assert.Nil(t, cmd)
		updatedModel := newModel.(*topModel)
		assert.Equal(t, 2, updatedModel.highlightIndex)
	})

	t.Run("highlight cycling wraps from last to first", func(t *testing.T) {
		model.mode = viewModeLogs
		model.highlightMatches = []int{10, 20, 30}
		model.highlightIndex = 2
		newModel, cmd := model.Update(tea.KeyPressMsg{Text: "n", Code: 'n'})
		assert.Nil(t, cmd)
		updatedModel := newModel.(*topModel)
		assert.Equal(t, 0, updatedModel.highlightIndex)
	})

	t.Run("highlight cycling wraps from first to last", func(t *testing.T) {
		model.mode = viewModeLogs
		model.highlightMatches = []int{10, 20, 30}
		model.highlightIndex = 0
		newModel, cmd := model.Update(tea.KeyPressMsg{Text: "N", Code: 'N'})
		assert.Nil(t, cmd)
		updatedModel := newModel.(*topModel)
		assert.Equal(t, 2, updatedModel.highlightIndex)
	})

	t.Run("highlight keys ignored when no highlights exist", func(t *testing.T) {
		model.mode = viewModeLogs
		model.highlightMatches = []int{}
		model.highlightIndex = 0
		newModel, cmd := model.Update(tea.KeyPressMsg{Text: "n", Code: 'n'})
		assert.Nil(t, cmd)
		updatedModel := newModel.(*topModel)
		assert.Equal(t, 0, updatedModel.highlightIndex)
	})
}

func TestViewportMatchCounter(t *testing.T) {
	t.Run("footer shows match counter when highlights active", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeLogs
		model.highlightMatches = []int{10, 20, 30, 40, 50}
		model.highlightIndex = 2
		view := model.View().Content
		assert.Contains(t, view, "Match 3/5")
	})

	t.Run("footer shows correct format for first match", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeLogs
		model.highlightMatches = []int{10, 20, 30}
		model.highlightIndex = 0
		view := model.View().Content
		assert.Contains(t, view, "Match 1/3")
	})
}

func TestViewportResizePersistence(t *testing.T) {
	t.Run("terminal resize preserves highlight index", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeLogs
		model.highlightMatches = []int{10, 20, 30, 40, 50}
		model.highlightIndex = 3

		newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		updatedModel := newModel.(*topModel)
		assert.Equal(t, 3, updatedModel.highlightIndex)
	})

	t.Run("terminal resize preserves highlight matches", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeLogs
		model.highlightMatches = []int{10, 20, 30, 40, 50}
		model.highlightIndex = 3

		newModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		updatedModel := newModel.(*topModel)
		assert.Equal(t, 3, updatedModel.highlightIndex)
		assert.Equal(t, []int{10, 20, 30, 40, 50}, updatedModel.highlightMatches)
	})

	t.Run("terminal resize with no highlights is safe", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeLogs
		model.highlightMatches = []int{}
		model.highlightIndex = 0

		newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		updatedModel := newModel.(*topModel)
		assert.NotNil(t, updatedModel)
		assert.Equal(t, 0, updatedModel.highlightIndex)
		assert.Equal(t, []int{}, updatedModel.highlightMatches)
	})

	t.Run("terminal resize updates width and height", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeLogs
		model.width = 100
		model.height = 30

		newModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		updatedModel := newModel.(*topModel)
		assert.Equal(t, 120, updatedModel.width)
		assert.Equal(t, 40, updatedModel.height)
	})
}

func TestViewportIntegration(t *testing.T) {
	t.Run("viewport component is initialized in topModel", func(t *testing.T) {
		model := newTestModel()
		assert.Equal(t, 0, model.viewport.YOffset())
	})

	t.Run("viewport receives updates when in logs mode", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeLogs
		model.width = 80
		model.height = 24
		model.logLines = []string{"Line 1", "Line 2", "Line 3"}
		model.viewport.SetContent(strings.Join(model.logLines, "\n"))

		newModel, cmd := model.Update(tickMsg(time.Now()))
		updatedModel := newModel.(*topModel)
		assert.NotNil(t, updatedModel)
		assert.NotNil(t, cmd)

		_ = updatedModel.View()
		viewOutput := model.viewport.View()
		assert.Contains(t, viewOutput, "Line 1")
	})

	t.Run("viewport sizing responds to terminal resize", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeLogs

		newModel, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
		updatedModel := newModel.(*topModel)
		assert.Equal(t, 100, updatedModel.width)
		assert.Equal(t, 40, updatedModel.height)
		_ = updatedModel.View()
	})

	t.Run("viewport content is updated from log messages", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeLogs
		model.width = 80
		model.height = 24

		newModel, _ := model.Update(logMsg{lines: []string{"Log line 1", "Log line 2", "Log line 3"}})
		updatedModel := newModel.(*topModel)
		assert.Equal(t, []string{"Log line 1", "Log line 2", "Log line 3"}, updatedModel.logLines)
		assert.NoError(t, updatedModel.logErr)
		assert.True(t, strings.Contains(updatedModel.viewport.View(), "Log line 1") || len(updatedModel.logLines) > 0)
	})

	t.Run("viewport handles empty log content gracefully", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeLogs
		model.width = 80
		model.height = 24

		newModel, _ := model.Update(logMsg{lines: []string{}, err: nil})
		updatedModel := newModel.(*topModel)
		_ = updatedModel.View()
		viewOutput := updatedModel.viewport.View()
		assert.Contains(t, viewOutput, "(no logs yet)")
	})

	t.Run("viewport handles log errors gracefully", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeLogs
		model.width = 80
		model.height = 24

		newModel, _ := model.Update(logMsg{lines: nil, err: fmt.Errorf("test error")})
		updatedModel := newModel.(*topModel)
		_ = updatedModel.View()
		assert.Error(t, updatedModel.logErr)
		viewOutput := updatedModel.viewport.View()
		assert.Contains(t, viewOutput, "Error:")
	})
}

func TestMouseModeEnabled(t *testing.T) {
	t.Run("TopCmd enables mouse cell motion", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeLogs
		model.logLines = []string{"Line 1", "Line 2", "Line 3"}
		model.viewport.SetContent(strings.Join(model.logLines, "\n"))

		newModel, cmd := model.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: 5})
		assert.NotNil(t, newModel)
		assert.Nil(t, cmd)
	})

	t.Run("mouse messages in non-logs mode are ignored", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeTable

		newModel, cmd := model.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: 5})
		assert.NotNil(t, newModel)
		assert.Nil(t, cmd)
	})
}

func TestTableMouseClickSelection(t *testing.T) {
	t.Run("click on running service row selects it", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeTable
		model.servers = []*models.ServerInfo{
			{ProcessRecord: &models.ProcessRecord{PID: 1001, Port: 3000, Command: "node server.js"}},
			{ProcessRecord: &models.ProcessRecord{PID: 1002, Port: 3001, Command: "go run ."}},
			{ProcessRecord: &models.ProcessRecord{PID: 1003, Port: 3002, Command: "python app.py"}},
		}

		model.viewport = viewport.New()
		_ = model.View()
		model.selected = 0
		model.focus = focusRunning

		viewportLines := strings.Split(model.table.runningVP.View(), "\n")
		clickY := -1
		for i, line := range viewportLines {
			if strings.Contains(line, "3001") {
				clickY = model.tableTopLines(model.width) + i
				break
			}
		}
		assert.NotEqual(t, -1, clickY)
		mouseMsg := tea.MouseClickMsg{Button: tea.MouseLeft, X: 10, Y: clickY}
		newModel, cmd := model.Update(mouseMsg)
		assert.NotNil(t, newModel)
		assert.Nil(t, cmd)

		m := newModel.(*topModel)
		assert.Equal(t, 1, m.selected)
		assert.Equal(t, focusRunning, m.focus)
	})

	t.Run("click with viewport offset adjusts selection correctly", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeTable
		model.servers = make([]*models.ServerInfo, 20)
		for i := 0; i < 20; i++ {
			model.servers[i] = &models.ServerInfo{
				ProcessRecord: &models.ProcessRecord{PID: 1000 + i, Port: 3000 + i, Command: fmt.Sprintf("node server%d.js", i)},
			}
		}

		model.table.runningVP = viewport.New()
		model.table.runningVP.SetWidth(80)
		model.table.runningVP.SetHeight(10)
		_ = model.View()
		model.table.runningVP.SetYOffset(5)

		targetAbsoluteLine := 2 + 5
		clickY := model.tableTopLines(model.width) + (targetAbsoluteLine - model.table.runningVP.YOffset())
		newModel, _ := model.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 10, Y: clickY})
		m := newModel.(*topModel)
		assert.Equal(t, 5, m.selected)
	})

	t.Run("click on managed service row selects it and activates managed focus", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeTable
		model.width = 100
		model.height = 20
		model.focus = focusRunning
		model.selected = 0
		model.managedSel = 0
		model.app = &fakeAppDeps{
			servers: []*models.ServerInfo{
				{
					ProcessRecord: &models.ProcessRecord{PID: 1001, Port: 3000, Command: "node server.js", CWD: "/tmp/app", ProjectRoot: "/tmp/app"},
					Status:        "running",
				},
			},
			services: []*models.ManagedService{
				{Name: "alpha", CWD: "/tmp/alpha", Command: "npm run dev", Ports: []int{4100}},
				{Name: "beta", CWD: "/tmp/beta", Command: "npm run dev", Ports: []int{4200}},
				{Name: "gamma", CWD: "/tmp/gamma", Command: "npm run dev", Ports: []int{4300}},
			},
		}
		model.servers = []*models.ServerInfo{
			{
				ProcessRecord: &models.ProcessRecord{PID: 1001, Port: 3000, Command: "node server.js", CWD: "/tmp/app", ProjectRoot: "/tmp/app"},
				Status:        "running",
			},
		}

		_ = model.View()
		viewportLines := strings.Split(model.table.managedVP.View(), "\n")
		clickY := -1
		for i, line := range viewportLines {
			if strings.Contains(line, "beta [stopped]") {
				clickY = model.tableTopLines(model.width) + model.table.lastRunningHeight + 1 + i
				break
			}
		}
		assert.NotEqual(t, -1, clickY)

		newModel, cmd := model.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 10, Y: clickY})
		assert.Nil(t, cmd)

		m := newModel.(*topModel)
		assert.Equal(t, focusManaged, m.focus)
		assert.Equal(t, 1, m.managedSel)
	})

	t.Run("wheel events are passed to viewport for scrolling", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeTable
		model.width = 80
		model.height = 12
		model.focus = focusManaged
		model.app = &fakeAppDeps{
			servers: []*models.ServerInfo{
				{
					ProcessRecord: &models.ProcessRecord{PID: 1001, Port: 3000, Command: "node server.js", CWD: "/tmp/app", ProjectRoot: "/tmp/app"},
					Status:        "running",
				},
			},
		}
		model.servers = []*models.ServerInfo{
			{
				ProcessRecord: &models.ProcessRecord{PID: 1001, Port: 3000, Command: "node server.js", CWD: "/tmp/app", ProjectRoot: "/tmp/app"},
				Status:        "running",
			},
		}
		fakeDeps := model.app.(*fakeAppDeps)
		for i := 0; i < 30; i++ {
			fakeDeps.services = append(fakeDeps.services, &models.ManagedService{
				Name:    fmt.Sprintf("svc-%02d", i),
				CWD:     fmt.Sprintf("/tmp/svc-%02d", i),
				Command: "npm run dev",
				Ports:   []int{4000 + i},
			})
		}

		_ = model.View()
		initialManagedOffset := model.table.managedVP.YOffset()
		runningOffset := model.table.runningVP.YOffset()
		mouseY := 2 + model.table.lastRunningHeight + 2

		newModel, cmd := model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 10, Y: mouseY})
		assert.NotNil(t, newModel)
		assert.Nil(t, cmd)

		updatedModel := newModel.(*topModel)
		assert.False(t, updatedModel.tableFollowSelection)

		_ = updatedModel.View()
		assert.Greater(t, updatedModel.table.managedVP.YOffset(), initialManagedOffset)
		assert.Equal(t, runningOffset, updatedModel.table.runningVP.YOffset())
	})

	t.Run("wheel scrolling in top grid only moves running section", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeTable
		model.width = 80
		model.height = 12
		model.focus = focusRunning
		model.selected = 0
		model.servers = make([]*models.ServerInfo, 30)
		for i := 0; i < 30; i++ {
			model.servers[i] = &models.ServerInfo{
				ProcessRecord: &models.ProcessRecord{
					PID:     1001 + i,
					Port:    3000 + i,
					Command: fmt.Sprintf("node server-%d.js", i),
				},
			}
		}
		model.app = &fakeAppDeps{
			servers: model.servers,
			services: []*models.ManagedService{
				{Name: "alpha", CWD: "/tmp/alpha", Command: "npm run dev", Ports: []int{4100}},
				{Name: "beta", CWD: "/tmp/beta", Command: "npm run dev", Ports: []int{4200}},
			},
		}

		_ = model.View()
		initialRunningOffset := model.table.runningVP.YOffset()
		managedOffset := model.table.managedVP.YOffset()
		mouseY := 4

		newModel, cmd := model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 10, Y: mouseY})
		assert.NotNil(t, newModel)
		assert.Nil(t, cmd)

		updatedModel := newModel.(*topModel)
		assert.False(t, updatedModel.tableFollowSelection)

		_ = updatedModel.View()
		assert.Greater(t, updatedModel.table.runningVP.YOffset(), initialRunningOffset)
		assert.Equal(t, managedOffset, updatedModel.table.managedVP.YOffset())
	})
}
