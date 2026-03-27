package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

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
