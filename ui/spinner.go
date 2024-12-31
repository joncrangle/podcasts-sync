package ui

import (
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

var (
	SpinnerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(Sky))
	ProgressStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Green)).
			PaddingLeft(1).Render
)

func CreateSpinner() spinner.Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = SpinnerStyle
	return sp
}

func CreateProgress() progress.Model {
	return progress.New(progress.WithDefaultGradient())
}
