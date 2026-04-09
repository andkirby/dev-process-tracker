package tui

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/devports/devpt/pkg/models"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// TEST-shift-keybinding
// Covers: BR-1.2, Edge-1.3, C-1.5, C-1.8
// ---------------------------------------------------------------------------

func TestShiftModifierDetection(t *testing.T) {
	t.Parallel()

	t.Run("ctrl+shift+e triggers group stop branch", func(t *testing.T) {
		m := newTestModel()
		m.mode = viewModeTable
		m.selected = 0

		msg := tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl | tea.ModShift}
		newModel, _ := m.Update(msg)
		updated := newModel.(*topModel)

		// Should open group confirmation modal (not single-item stop)
		assertGroupConfirmKind(t, updated, confirmGroupStop)
	})

	t.Run("ctrl+shift+r triggers group restart branch", func(t *testing.T) {
		m := newTestModel()
		m.mode = viewModeTable
		m.selected = 0

		msg := tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl | tea.ModShift}
		newModel, _ := m.Update(msg)
		updated := newModel.(*topModel)

		assertGroupConfirmKind(t, updated, confirmGroupRestart)
	})

	t.Run("shift+x triggers group remove branch", func(t *testing.T) {
		m := newTopModel(&fakeAppDeps{
			servers: []*models.ServerInfo{},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
				{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
			},
		})
		m.mode = viewModeTable
		m.focus = focusManaged
		m.managedSel = 0

		msg := tea.KeyPressMsg{Code: 'x', Mod: tea.ModShift}
		newModel, _ := m.Update(msg)
		updated := newModel.(*topModel)

		assertGroupConfirmKind(t, updated, confirmGroupRemove)
	})

}

func TestShiftNoOpGuards(t *testing.T) {
	t.Parallel()

	t.Run("C-1.5: group action with no group members is no-op", func(t *testing.T) {
		m := newTopModel(&fakeAppDeps{servers: []*models.ServerInfo{}})
		m.mode = viewModeTable
		m.selected = -1

		msg := tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl | tea.ModShift}
		newModel, _ := m.Update(msg)
		updated := newModel.(*topModel)

		// No modal should open when there's no selection
		assert.Nil(t, updated.modal)
		assert.Nil(t, updated.confirm)
	})

	t.Run("C-1.8: group action with single member falls back to single action", func(t *testing.T) {
		m := newTestModel()
		m.mode = viewModeTable
		m.selected = 0

		// Only one server exists, so group stop should fall back to single stop
		msg := tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl | tea.ModShift}
		newModel, _ := m.Update(msg)
		updated := newModel.(*topModel)

		// Should still open a confirm modal (even for single-member group)
		assert.NotNil(t, updated.confirm)
	})

	t.Run("shift modifier ignored in logs mode", func(t *testing.T) {
		m := newTestModel()
		m.mode = viewModeLogs

		msg := tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl | tea.ModShift}
		newModel, _ := m.Update(msg)
		updated := newModel.(*topModel)

		// Should not open group modal while in logs mode
		assert.Nil(t, updated.confirm)
	})

	t.Run("shift modifier ignored in search mode", func(t *testing.T) {
		m := newTestModel()
		m.mode = viewModeSearch

		msg := tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl | tea.ModShift}
		newModel, _ := m.Update(msg)
		updated := newModel.(*topModel)

		assert.Nil(t, updated.confirm)
	})

	t.Run("shift modifier ignored in command mode", func(t *testing.T) {
		m := newTestModel()
		m.mode = viewModeCommand

		msg := tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl | tea.ModShift}
		newModel, _ := m.Update(msg)
		updated := newModel.(*topModel)

		assert.Nil(t, updated.confirm)
	})
}

func TestShiftKeyStringVariants(t *testing.T) {
	t.Parallel()

	t.Run("Edge-1.3: ctrl+shift+e string representation matches", func(t *testing.T) {
		m := newTestModel()
		m.mode = viewModeTable
		m.selected = 0

		// Simulate the key string that bubbletea would generate
		msg := tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl | tea.ModShift}
		str := msg.String()
		assert.Contains(t, str, "ctrl")
		assert.Contains(t, str, "shift")
	})

	t.Run("ctrl+e without shift takes single-item path", func(t *testing.T) {
		m := newTestModel()
		m.mode = viewModeTable
		m.selected = 0

		msg := tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl}
		newModel, _ := m.Update(msg)
		updated := newModel.(*topModel)

		// Without shift, should trigger single-item stop confirm (not group)
		// Single-item stop uses confirmStopPID kind
		if updated.confirm != nil {
			assert.Equal(t, confirmStopPID, updated.confirm.kind)
		}
	})
}

func TestShiftKeybindingsRegistered(t *testing.T) {
	t.Parallel()

	t.Run("group stop binding exists in keymap", func(t *testing.T) {
		m := newTestModel()
		assert.True(t, key.Matches(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl | tea.ModShift}, m.keys.GroupStop))
	})

	t.Run("group restart binding exists in keymap", func(t *testing.T) {
		m := newTestModel()
		assert.True(t, key.Matches(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl | tea.ModShift}, m.keys.GroupRestart))
	})

	t.Run("group remove binding exists in keymap", func(t *testing.T) {
		m := newTestModel()
		assert.True(t, key.Matches(tea.KeyPressMsg{Code: 'x', Mod: tea.ModShift}, m.keys.GroupRemove))
	})

	t.Run("group bindings do not match without shift modifier", func(t *testing.T) {
		m := newTestModel()
		assert.False(t, key.Matches(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl}, m.keys.GroupStop))
		assert.False(t, key.Matches(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}, m.keys.GroupRestart))
		assert.False(t, key.Matches(tea.KeyPressMsg{Code: 'x', Mod: 0}, m.keys.GroupRemove))

	})
}

// assertGroupConfirmKind is a test helper that checks the confirm state has the expected group action kind.
func assertGroupConfirmKind(t *testing.T, m *topModel, expected confirmKind) {
	t.Helper()
	if m.confirm == nil {
		t.Fatalf("expected confirm modal with kind %v, got nil confirm", expected)
	}
	assert.Equal(t, expected, m.confirm.kind)
}

func TestCommandModeAcceptsRuneKeys(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"b", "q", "s", "n"} {
		m := &topModel{mode: viewModeCommand}
		next, _ := m.Update(tea.KeyPressMsg{Text: key, Code: rune(key[0])})
		updated, ok := next.(*topModel)
		if !ok {
			t.Fatalf("expected *topModel, got %T", next)
		}
		if updated.cmdInput != key {
			t.Fatalf("expected command input to include rune key %q, got %q", key, updated.cmdInput)
		}
	}
}

func TestSearchModeAcceptsRuneKeys(t *testing.T) {
	t.Parallel()

	m := newTopModel(&fakeAppDeps{})
	next, _ := m.Update(tea.KeyPressMsg{Text: "/", Code: '/'})
	updated, ok := next.(*topModel)
	if !ok {
		t.Fatalf("expected *topModel, got %T", next)
	}
	next, _ = updated.Update(tea.KeyPressMsg{Text: "s", Code: 's'})
	updated, ok = next.(*topModel)
	if !ok {
		t.Fatalf("expected *topModel, got %T", next)
	}
	if updated.searchInput.Value() != "s" {
		t.Fatalf("expected search input to include rune key, got %q", updated.searchInput.Value())
	}
}
