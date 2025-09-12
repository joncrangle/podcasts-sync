package tui

import (
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
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

func createSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(Mauve))
	return s
}
