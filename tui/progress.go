package tui

import (
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

var progressInfoStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(Subtext0)).
	Padding(1, 0)

func createProgress() progress.Model {
	p := progress.New(progress.WithScaledGradient(MauveDarker, Mauve))
	p.PercentageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(Mauve))
	return p
}
