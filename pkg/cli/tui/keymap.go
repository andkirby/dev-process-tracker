package tui

import "charm.land/bubbles/v2/key"

type keyMap struct {
	Up           key.Binding
	Down         key.Binding
	Tab          key.Binding
	Enter        key.Binding
	Search       key.Binding
	ClearFilter  key.Binding
	Sort         key.Binding
	SortReverse  key.Binding
	Health       key.Binding
	Help         key.Binding
	Add          key.Binding
	Restart      key.Binding
	Stop         key.Binding
	Remove       key.Binding
	Debug        key.Binding
	Back         key.Binding
	Follow       key.Binding
	NextMatch    key.Binding
	PrevMatch    key.Binding
	Confirm      key.Binding
	Cancel       key.Binding
	Quit         key.Binding
	GroupStop    key.Binding
	GroupRestart key.Binding
	GroupRemove  key.Binding
	GroupToggle  key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("up/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("down/j", "move down"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch list"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "logs/start"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		ClearFilter: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("^L", "clear filter"),
		),
		Sort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sort"),
		),
		SortReverse: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "sort reverse"),
		),
		Health: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "health detail"),
		),
		Help: key.NewBinding(
			key.WithKeys("?", "f1"),
			key.WithHelp("?", "toggle help"),
		),
		Add: key.NewBinding(
			key.WithKeys("ctrl+a"),
			key.WithHelp("^A", "add"),
		),
		Restart: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("^R", "restart"),
		),
		Stop: key.NewBinding(
			key.WithKeys("ctrl+e"),
			key.WithHelp("^E", "stop"),
		),
		Remove: key.NewBinding(
			key.WithKeys("x", "delete", "ctrl+d"),
			key.WithHelp("x", "remove managed"),
		),
		Debug: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "debug"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "b"),
			key.WithHelp("esc/b", "back"),
		),
		Follow: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "toggle follow"),
		),
		NextMatch: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next match"),
		),
		PrevMatch: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "prev match"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter", "y"),
			key.WithHelp("enter/y", "confirm"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("n", "esc"),
			key.WithHelp("n/esc", "cancel"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		GroupStop: key.NewBinding(
			key.WithKeys("ctrl+shift+e"),
			key.WithHelp("^⇧E", "group stop"),
		),
		GroupRestart: key.NewBinding(
			key.WithKeys("ctrl+shift+r"),
			key.WithHelp("^⇧R", "group restart"),
		),

		GroupRemove: key.NewBinding(
			key.WithKeys("shift+x"),
			key.WithHelp("⇧X", "group remove"),
		),
		GroupToggle: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "group mode"),
		),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Enter, k.Search, k.Help, k.GroupToggle}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Tab, k.Enter, k.Search, k.ClearFilter},
		{k.Sort, k.SortReverse, k.Health, k.Help, k.Add, k.Restart, k.Stop},
		{k.Remove, k.Debug, k.Back, k.Follow, k.NextMatch, k.PrevMatch},
		{k.Confirm, k.Cancel, k.Quit},
		{k.GroupToggle, k.GroupStop, k.GroupRestart, k.GroupRemove},
	}
}
