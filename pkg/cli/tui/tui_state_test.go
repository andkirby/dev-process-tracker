package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/devports/devpt/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestTUISimpleUpdate(t *testing.T) {
	model := newTestModel()

	t.Run("tab switches focus between running and managed", func(t *testing.T) {
		initialFocus := model.focus
		newModel, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		assert.Nil(t, cmd)

		updatedModel := newModel.(*topModel)
		assert.NotEqual(t, initialFocus, updatedModel.focus)
		if initialFocus == focusRunning {
			assert.Equal(t, focusManaged, updatedModel.focus)
		} else {
			assert.Equal(t, focusRunning, updatedModel.focus)
		}
	})

	t.Run("escape key in logs mode returns to table", func(t *testing.T) {
		model.mode = viewModeLogs
		newModel, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
		assert.Nil(t, cmd)
		updatedModel := newModel.(*topModel)
		assert.Equal(t, viewModeTable, updatedModel.mode)
	})

	t.Run("forward slash enters search mode", func(t *testing.T) {
		model.mode = viewModeTable
		newModel, cmd := model.Update(tea.KeyPressMsg{Text: "/", Code: '/'})
		assert.NotNil(t, cmd)
		updatedModel := newModel.(*topModel)
		assert.Equal(t, viewModeSearch, updatedModel.mode)
	})

	t.Run("question mark enters help mode", func(t *testing.T) {
		model.mode = viewModeTable
		newModel, cmd := model.Update(tea.KeyPressMsg{Text: "?", Code: '?'})
		assert.Nil(t, cmd)
		updatedModel := newModel.(*topModel)
		assert.Equal(t, modalHelp, updatedModel.activeModalKind())
	})

	t.Run("s key cycles through sort modes", func(t *testing.T) {
		model.mode = viewModeTable
		initialSort := model.sortBy
		newModel, cmd := model.Update(tea.KeyPressMsg{Text: "s", Code: 's'})
		assert.Nil(t, cmd)
		updatedModel := newModel.(*topModel)
		assert.NotEqual(t, initialSort, updatedModel.sortBy)
	})

	t.Run("enter opens logs for running selection", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeTable
		model.focus = focusRunning
		model.selected = 0

		newModel, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		assert.NotNil(t, cmd)

		updatedModel := newModel.(*topModel)
		assert.Equal(t, viewModeLogs, updatedModel.mode)
		assert.Equal(t, 1001, updatedModel.logPID)
	})

	t.Run("enter starts service for managed selection", func(t *testing.T) {
		model := newTopModel(&fakeAppDeps{
			servers: []*models.ServerInfo{
				{
					ManagedService: &models.ManagedService{Name: "test-svc", CWD: "/tmp/app", Command: "npm run dev", Ports: []int{3000}},
					ProcessRecord:  &models.ProcessRecord{PID: 1001, Port: 3000, Command: "node server.js", CWD: "/tmp/app", ProjectRoot: "/tmp/app"},
				},
			},
			services: []*models.ManagedService{
				{Name: "test-svc", CWD: "/tmp/app", Command: "npm run dev", Ports: []int{3000}},
			},
		})
		model.mode = viewModeTable
		model.focus = focusManaged
		model.managedSel = 0

		newModel, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		assert.Nil(t, cmd)

		updatedModel := newModel.(*topModel)
		assert.Equal(t, viewModeTable, updatedModel.mode)
		assert.Contains(t, updatedModel.cmdStatus, `Started "test-svc"`)
	})
}

func TestTUIKeySequence(t *testing.T) {
	t.Run("navigate and return to table", func(t *testing.T) {
		model := newTestModel()
		initialMode := model.mode

		newModel, _ := model.Update(tea.KeyPressMsg{Text: "/", Code: '/'})
		model = newModel.(*topModel)
		assert.Equal(t, viewModeSearch, model.mode)

		newModel, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
		model = newModel.(*topModel)
		assert.Equal(t, initialMode, model.mode)
	})

	t.Run("help mode and exit", func(t *testing.T) {
		model := newTestModel()

		newModel, _ := model.Update(tea.KeyPressMsg{Text: "?", Code: '?'})
		model = newModel.(*topModel)
		assert.Equal(t, modalHelp, model.activeModalKind())

		newModel, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
		model = newModel.(*topModel)
		assert.Equal(t, viewModeTable, model.mode)
		assert.Nil(t, model.modal)
	})
}

func TestTUIQuitKey(t *testing.T) {
	model := newTestModel()

	t.Run("q key returns quit command", func(t *testing.T) {
		_, cmd := model.Update(tea.KeyPressMsg{Text: "q", Code: 'q'})
		assert.NotNil(t, cmd)
	})

	t.Run("ctrl+c returns quit command", func(t *testing.T) {
		_, cmd := model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
		assert.NotNil(t, cmd)
	})
}

func TestTUIViewRendering(t *testing.T) {
	model := newTestModel()
	model.width = 100
	model.height = 40

	t.Run("table view contains expected elements", func(t *testing.T) {
		model.mode = viewModeTable
		output := model.View()
		assert.Contains(t, output.Content, "Dev Process Tracker")
		assert.Contains(t, output.Content, "Name")
		assert.Contains(t, output.Content, "Port")
		assert.Contains(t, output.Content, "PID")
	})

	t.Run("help view contains help text", func(t *testing.T) {
		model.openHelpModal()
		output := model.View()
		assert.Contains(t, output.Content, "Help")
		assert.Contains(t, output.Content, "switch list")
	})
}

func TestViewportStateTransitions(t *testing.T) {
	t.Run("viewport state initialization", func(t *testing.T) {
		model := newTestModel()
		_ = model
		t.Skip("TODO: Verify viewport state fields exist - OBL-highlight-state")
	})

	t.Run("highlight index boundary conditions", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeLogs
		model.highlightMatches = []int{10, 20, 30}
		model.highlightIndex = 0
		model.highlightIndex = len(model.highlightMatches) - 1
		_ = model
		t.Skip("TODO: Test boundary conditions - Edge-2")
	})

	t.Run("highlight index with empty matches", func(t *testing.T) {
		model := newTestModel()
		model.mode = viewModeLogs
		model.highlightMatches = []int{}
		model.highlightIndex = 0
		_ = model
		t.Skip("TODO: Handle empty highlights - Edge case")
	})
}

func TestSortCycling(t *testing.T) {
	model := newTestModel()

	t.Run("cycleSort ascending to reverse to recent", func(t *testing.T) {
		// Start with recent (default)
		assert.Equal(t, sortRecent, model.sortBy)
		assert.False(t, model.sortReverse)

		// Click name column -> ascending (yellow)
		model.cycleSort(sortName)
		assert.Equal(t, sortName, model.sortBy)
		assert.False(t, model.sortReverse)

		// Click same column again -> reverse (orange)
		model.cycleSort(sortName)
		assert.Equal(t, sortName, model.sortBy)
		assert.True(t, model.sortReverse)

		// Click same column again -> reset to recent
		model.cycleSort(sortName)
		assert.Equal(t, sortRecent, model.sortBy)
		assert.False(t, model.sortReverse)
	})

	t.Run("clicking different column resets to ascending", func(t *testing.T) {
		model.sortBy = sortName
		model.sortReverse = true

		// Click different column -> ascending
		model.cycleSort(sortPort)
		assert.Equal(t, sortPort, model.sortBy)
		assert.False(t, model.sortReverse)
	})

	t.Run("s key cycles sort modes without reverse", func(t *testing.T) {
		model.sortBy = sortRecent
		model.sortReverse = false

		// 's' key should cycle through modes and reset reverse
		newModel, _ := model.Update(tea.KeyPressMsg{Code: 's'})
		updated := newModel.(*topModel)
		assert.Equal(t, sortName, updated.sortBy)
		assert.False(t, updated.sortReverse)

		newModel, _ = updated.Update(tea.KeyPressMsg{Code: 's'})
		updated = newModel.(*topModel)
		assert.Equal(t, sortProject, updated.sortBy)
		assert.False(t, updated.sortReverse)
	})
}

func TestSortDirectionToggle(t *testing.T) {
	model := newTestModel()

	t.Run("toggle flips reverse without changing column", func(t *testing.T) {
		model.sortBy = sortName
		model.sortReverse = false

		model.toggleSortDirection()
		assert.Equal(t, sortName, model.sortBy)
		assert.True(t, model.sortReverse)

		model.toggleSortDirection()
		assert.Equal(t, sortName, model.sortBy)
		assert.False(t, model.sortReverse)
	})

	t.Run("toggle is no-op in recent mode", func(t *testing.T) {
		model.sortBy = sortRecent
		model.sortReverse = false

		model.toggleSortDirection()
		assert.Equal(t, sortRecent, model.sortBy)
		assert.False(t, model.sortReverse)
	})

	t.Run("toggle preserves column across multiple flips", func(t *testing.T) {
		model.sortBy = sortPort
		model.sortReverse = false

		model.toggleSortDirection()
		model.toggleSortDirection()
		model.toggleSortDirection()

		assert.Equal(t, sortPort, model.sortBy)
		assert.True(t, model.sortReverse)
	})

	t.Run("toggle works on every sortable column", func(t *testing.T) {
		columns := []sortMode{sortName, sortProject, sortPort, sortHealth}
		for _, col := range columns {
			model.sortBy = col
			model.sortReverse = false

			model.toggleSortDirection()
			assert.Equal(t, col, model.sortBy, "column changed after toggle for %s", sortModeLabel(col))
			assert.True(t, model.sortReverse, "reverse not set for %s", sortModeLabel(col))
		}
	})
}

func TestSortDirectionToggleViaKey(t *testing.T) {
	model := newTestModel()
	model.mode = viewModeTable

	t.Run("S key toggles direction for current column", func(t *testing.T) {
		model.sortBy = sortName
		model.sortReverse = false

		newModel, _ := model.Update(tea.KeyPressMsg{Text: "S", Code: 'S'})
		updated := newModel.(*topModel)
		assert.Equal(t, sortName, updated.sortBy)
		assert.True(t, updated.sortReverse)
	})

	t.Run("S key preserves column", func(t *testing.T) {
		model.sortBy = sortProject
		model.sortReverse = false

		newModel, _ := model.Update(tea.KeyPressMsg{Text: "S", Code: 'S'})
		updated := newModel.(*topModel)
		assert.Equal(t, sortProject, updated.sortBy)
		assert.True(t, updated.sortReverse)
	})

	t.Run("S key is no-op in recent mode", func(t *testing.T) {
		model.sortBy = sortRecent
		model.sortReverse = false

		newModel, _ := model.Update(tea.KeyPressMsg{Text: "S", Code: 'S'})
		updated := newModel.(*topModel)
		assert.Equal(t, sortRecent, updated.sortBy)
		assert.False(t, updated.sortReverse)
	})

	t.Run("S and s are independent operations", func(t *testing.T) {
		model.sortBy = sortRecent
		model.sortReverse = false

		// s -> Name ascending
		newModel, _ := model.Update(tea.KeyPressMsg{Text: "s", Code: 's'})
		updated := newModel.(*topModel)
		assert.Equal(t, sortName, updated.sortBy)
		assert.False(t, updated.sortReverse)

		// S -> Name descending
		newModel, _ = updated.Update(tea.KeyPressMsg{Text: "S", Code: 'S'})
		updated = newModel.(*topModel)
		assert.Equal(t, sortName, updated.sortBy)
		assert.True(t, updated.sortReverse)

		// s -> Project ascending (column switch resets reverse)
		newModel, _ = updated.Update(tea.KeyPressMsg{Text: "s", Code: 's'})
		updated = newModel.(*topModel)
		assert.Equal(t, sortProject, updated.sortBy)
		assert.False(t, updated.sortReverse)
	})
}

func TestSortColumnSwitchResetsDirection(t *testing.T) {
	model := newTestModel()
	model.mode = viewModeTable

	t.Run("s key resets reverse when switching columns", func(t *testing.T) {
		model.sortBy = sortName
		model.sortReverse = true

		newModel, _ := model.Update(tea.KeyPressMsg{Text: "s", Code: 's'})
		updated := newModel.(*topModel)
		assert.Equal(t, sortProject, updated.sortBy)
		assert.False(t, updated.sortReverse)
	})

	t.Run("s key wraps around to recent and resets reverse", func(t *testing.T) {
		model.sortBy = sortHealth
		model.sortReverse = true

		newModel, _ := model.Update(tea.KeyPressMsg{Text: "s", Code: 's'})
		updated := newModel.(*topModel)
		assert.Equal(t, sortRecent, updated.sortBy)
		assert.False(t, updated.sortReverse)
	})
}

func TestSortPersistenceAcrossRefresh(t *testing.T) {
	model := newTestModel()
	model.width = 100
	model.height = 40
	model.mode = viewModeTable

	t.Run("sort state survives tick refresh", func(t *testing.T) {
		model.sortBy = sortName
		model.sortReverse = true

		newModel, _ := model.Update(tickMsg(time.Now()))
		updated := newModel.(*topModel)
		assert.Equal(t, sortName, updated.sortBy)
		assert.True(t, updated.sortReverse)
	})

	t.Run("sort state survives multiple refreshes", func(t *testing.T) {
		model.sortBy = sortPort
		model.sortReverse = true

		for i := 0; i < 5; i++ {
			newModel, _ := model.Update(tickMsg(time.Now()))
			model = newModel.(*topModel)
		}
		assert.Equal(t, sortPort, model.sortBy)
		assert.True(t, model.sortReverse)
	})
}

func TestColumnAtX(t *testing.T) {
	model := newTestModel()
	model.width = 120

	tests := []struct {
		name     string
		x        int
		wantSort sortMode
	}{
		{"name column", 5, sortName},
		{"port column", 18, sortPort},
		{"pid column", 26, sortRecent},
		{"project column", 40, sortProject},
		{"health column", 115, sortHealth},
		{"out of bounds", 200, sortMode(-1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := model.columnAtX(tt.x)
			assert.Equal(t, tt.wantSort, got)
		})
	}
}
