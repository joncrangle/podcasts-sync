package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Catppuccin Mocha color palette
const (
	Flamingo    = "#f2cdcd"
	Pink        = "#f5c2e7"
	Mauve       = "#cba6f7"
	MauveDarker = "#6b5885"
	Red         = "#f38ba8"
	Maroon      = "#eba0ac"
	Peach       = "#fab387"
	Yellow      = "#f9e2af"
	Green       = "#a6e3a1"
	Teal        = "#94e2d5"
	Sky         = "#89dceb"
	Sapphire    = "#74c7ec"
	Blue        = "#89b4fa"
	Lavender    = "#b4befe"
	Text        = "#cdd6f4"
	Subtext1    = "#bac2de"
	Subtext0    = "#a6adc8"
	Overlay2    = "#9399b2"
	Overlay1    = "#7f849c"
	Overlay0    = "#6c7086"
	Surface2    = "#585b70"
	Surface1    = "#45475a"
	Surface0    = "#313244"
	Base        = "#1e1e2e"
	Mantle      = "#181825"
	Crust       = "#11111b"
)

var (
	appStyle = lipgloss.NewStyle().
			Margin(1, 4)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(Red)).Render
	driveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Flamingo)).
			Margin(1, 0).Padding(0, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(Flamingo)).
			Render
	headingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Blue)).
			Margin(1, 0).Padding(0, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(Blue)).
			Render
	debugTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(Red)).
			Bold(true).Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(Red)).Margin(1, 0).Padding(0, 1).Render
	helpStyle  = lipgloss.NewStyle().Padding(1).Render
	popupStyle = lipgloss.NewStyle().
			Padding(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(Pink)).Align(lipgloss.Center)
	debugStyle = lipgloss.NewStyle().
			Padding(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(Pink)).
			Align(lipgloss.Center).Render
)
