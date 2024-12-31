package ui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

func CreateHelp() help.Model {
	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color(Yellow))
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.Color(Subtext0))
	h.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color(Peach))
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
	Refresh     key.Binding
	Delete      key.Binding
	BatchSync   key.Binding
	BatchDelete key.Binding
	Validate    key.Binding
	Help        key.Binding
	Quit        key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Sync, k.BatchSync, k.SelectDrive, k.Help, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Sync, k.BatchSync, k.SelectDrive},
		{k.Help, k.Quit},
	}
}

// TODO: Add refresh for apple podcasts
var Keys = KeyMap{
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
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
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
	Sync: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "sync selected"),
	),
	BatchSync: key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("S", "sync all"),
	),
	BatchDelete: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "delete all"),
	),
	Validate: key.NewBinding(
		key.WithKeys("v"),
		key.WithHelp("v", "validate files"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}
