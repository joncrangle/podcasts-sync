package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

func createHelp() help.Model {
	h := help.New()
	h.ShowAll = false

	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color(Yellow))
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.Color(Subtext0))
	h.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color(Flamingo))
	h.Styles.FullKey = lipgloss.NewStyle().Foreground(lipgloss.Color(Yellow))
	h.Styles.FullDesc = lipgloss.NewStyle().Foreground(lipgloss.Color(Subtext0))
	h.Styles.FullSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color(Peach))
	return h
}

type KeyMap struct {
	Up          key.Binding
	Down        key.Binding
	Left        key.Binding
	Right       key.Binding
	Space       key.Binding
	Enter       key.Binding
	Escape      key.Binding
	Tab         key.Binding
	SelectDrive key.Binding
	Sync        key.Binding
	SyncAll     key.Binding
	Refresh     key.Binding
	Delete      key.Binding
	DeleteAll   key.Binding
	Debug       key.Binding
	Quit        key.Binding
	Progress    key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Tab, k.SelectDrive, k.Refresh, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{}
}

var keys = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "left list"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "right list"),
	),
	Space: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "select"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter", "y"),
		key.WithHelp("enter", "confirm"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc", "n"),
		key.WithHelp("esc", "close"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch focus"),
	),
	SelectDrive: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "select drive"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete selected"),
	),
	DeleteAll: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "delete all"),
	),
	Sync: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "sync selected"),
	),
	SyncAll: key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("S", "sync all"),
	),
	Debug: key.NewBinding(
		key.WithKeys("X"),
		key.WithHelp("X", "debug"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Progress: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "progress"),
	),
}

type MacHelpKeyMap struct{ KeyMap }

func (k MacHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Space, k.Sync, k.SyncAll}
}

var macHelpKeys = MacHelpKeyMap{
	KeyMap: KeyMap{
		Up:      keys.Up,
		Down:    keys.Down,
		Tab:     keys.Tab,
		Space:   keys.Space,
		Sync:    keys.Sync,
		SyncAll: keys.SyncAll,
		Quit:    keys.Quit,
	},
}

type DriveHelpKeyMap struct{ KeyMap }

func (k DriveHelpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Space, k.Delete, k.DeleteAll}
}

var driveHelpKeys = DriveHelpKeyMap{
	KeyMap: KeyMap{
		Up:        keys.Up,
		Down:      keys.Down,
		Tab:       keys.Tab,
		Space:     keys.Space,
		Delete:    keys.Delete,
		DeleteAll: keys.DeleteAll,
		Quit:      keys.Quit,
	},
}

type ConfirmKeyMap struct {
	Yes key.Binding
	No  key.Binding
}

func (k ConfirmKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Yes, k.No}
}

func (k ConfirmKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{}
}

var confirmKeys = ConfirmKeyMap{
	Yes: key.NewBinding(
		key.WithKeys("y", "enter"),
		key.WithHelp("y/enter", "yes"),
	),
	No: key.NewBinding(
		key.WithKeys("n", "esc"),
		key.WithHelp("n/esc", "no"),
	),
}

type TransferKeyMap struct {
	Cancel key.Binding
}

func (k TransferKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Cancel}
}

func (k TransferKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{}
}

var transferKeys = TransferKeyMap{
	Cancel: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
}
