package ui

import (
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
	AppStyle = lipgloss.NewStyle().
			MarginLeft(4).MarginTop(1)
	LeftStyle, RightStyle lipgloss.Style
	FocusedStyle          = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color(Pink)).
				Padding(1)
	NoStyleStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			Padding(1)
	StatusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Mauve))
	ErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(Red))
)
