package main

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

// Catppuccin Mocha color palette
const (
	Flamingo = "#f2cdcd"
	Pink     = "#f5c2e7"
	Mauve    = "#cba6f7"
	Red      = "#f38ba8"
	Maroon   = "#eba0ac"
	Peach    = "#fab387"
	Yellow   = "#f9e2af"
	Green    = "#a6e3a1"
	Teal     = "#94e2d5"
	Sky      = "#89dceb"
	Sapphire = "#74c7ec"
	Blue     = "#89b4fa"
	Lavender = "#b4befe"
	Text     = "#cdd6f4"
	Subtext1 = "#bac2de"
	Subtext0 = "#a6adc8"
	Overlay2 = "#9399b2"
	Overlay1 = "#7f849c"
	Overlay0 = "#6c7086"
	Surface2 = "#585b70"
	Surface1 = "#45475a"
	Surface0 = "#313244"
	Base     = "#1e1e2e"
	Mantle   = "#181825"
	Crust    = "#11111b"
)

var (
	listStyle             = lipgloss.NewStyle().PaddingLeft(4).PaddingRight(4).Render
	leftStyle, rightStyle lipgloss.Style
	focusedStyle          = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color(Flamingo)).
				Padding(1)
	noStyleStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			Padding(1)
	driveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Mauve)).
			PaddingLeft(4).PaddingTop(1)
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Teal))
	progressStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Green)).
			PaddingLeft(1)
	popupStyle = lipgloss.NewStyle().
			Padding(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(Pink)).
			Width(60).
			Align(lipgloss.Center)
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(Sky))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(Sky)).PaddingLeft(4).PaddingBottom(1)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(Red))
)

type keyMap struct {
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
	Delete      key.Binding
	BatchSync   key.Binding
	BatchDelete key.Binding
	Validate    key.Binding
	Help        key.Binding
	Quit        key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// TODO: Add more keys to the help menu
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right}, // first column
		{k.Help, k.Quit},                // second column
	}
}

// TODO: Add refresh for apple podcasts
var keys = keyMap{
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

type popupKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Escape key.Binding
	Help   key.Binding
	Quit   key.Binding
}

func (k popupKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Escape, k.Enter, k.Help}
}

func (k popupKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Quit},
		{k.Escape, k.Enter, k.Help},
	}
}

var popupKeys = popupKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "confirm"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "close"),
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
