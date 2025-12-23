package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keybindings for the TUI
type KeyMap struct {
	// Global navigation
	Quit    key.Binding
	Help    key.Binding
	Back    key.Binding
	Refresh key.Binding

	// View switching (number keys)
	Dashboard   key.Binding
	Mocks       key.Binding
	Recordings  key.Binding
	Streams     key.Binding
	Traffic     key.Binding
	Connections key.Binding
	Logs        key.Binding

	// List navigation
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Top      key.Binding
	Bottom   key.Binding

	// Actions
	Enter  key.Binding
	Select key.Binding
	Toggle key.Binding

	// CRUD operations
	New    key.Binding
	Edit   key.Binding
	Delete key.Binding
	Copy   key.Binding

	// Filtering/Search
	Filter key.Binding
	Clear  key.Binding

	// Server operations
	StartStop    key.Binding
	ToggleRecord key.Binding
	PauseResume  key.Binding

	// Tab navigation (for forms)
	NextField key.Binding
	PrevField key.Binding
}

// DefaultKeyMap returns the default keybindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		// Global navigation
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "refresh"),
		),

		// View switching
		Dashboard: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "dashboard"),
		),
		Mocks: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "mocks"),
		),
		Recordings: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "recordings"),
		),
		Streams: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "streams"),
		),
		Traffic: key.NewBinding(
			key.WithKeys("5"),
			key.WithHelp("5", "traffic"),
		),
		Connections: key.NewBinding(
			key.WithKeys("6"),
			key.WithHelp("6", "connections"),
		),
		Logs: key.NewBinding(
			key.WithKeys("7"),
			key.WithHelp("7", "logs"),
		),

		// List navigation
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn", "page down"),
		),
		Top: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("g/home", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("G/end", "bottom"),
		),

		// Actions
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter", "select"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("enter", "t"),
			key.WithHelp("enter/t", "toggle"),
		),

		// CRUD operations
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Copy: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy"),
		),

		// Filtering/Search
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Clear: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "clear"),
		),

		// Server operations
		StartStop: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "start/stop"),
		),
		ToggleRecord: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "record"),
		),
		PauseResume: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "pause/resume"),
		),

		// Tab navigation
		NextField: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next field"),
		),
		PrevField: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev field"),
		),
	}
}

// ShortHelp returns a quick help text
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns all keybindings organized by section
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown},
		{k.Enter, k.Back, k.Quit, k.Help},
		{k.Dashboard, k.Mocks, k.Traffic},
	}
}
