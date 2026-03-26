package cli

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCommandModeAcceptsRuneKeys(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"b", "q", "s", "n"} {
		m := &topModel{
			mode: viewModeCommand,
		}

		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
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

	m := &topModel{
		mode: viewModeSearch,
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	updated, ok := next.(*topModel)
	if !ok {
		t.Fatalf("expected *topModel, got %T", next)
	}
	if updated.searchQuery != "s" {
		t.Fatalf("expected search query to include rune key, got %q", updated.searchQuery)
	}
}
