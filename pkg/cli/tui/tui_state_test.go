package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
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
		assert.Nil(t, cmd)
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
