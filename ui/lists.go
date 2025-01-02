package ui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

var (
	DriveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(Mauve)).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(Mauve)).
			Render
	PopupStyle = lipgloss.NewStyle().
			Padding(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(Pink)).
			Width(60).
			Align(lipgloss.Center).Render
	FocusedStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(Pink)).
			Padding(1)
	NoStyleStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			Padding(1)
)

func CreateList(title string, kind string) list.Model {
	delegate := list.NewDefaultDelegate()

	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color(Mauve)).Bold(true)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.BorderLeftForeground(lipgloss.Color(Mauve))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color(Subtext0))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.BorderLeftForeground(lipgloss.Color(Mauve))

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = title
	l.SetFilteringEnabled(false)
	l.Help = CreateHelp()
	l.Styles.NoItems = lipgloss.NewStyle().Foreground(lipgloss.Color(Text))
	switch kind {
	case "mac":
		l.AdditionalShortHelpKeys = func() []key.Binding {
			return []key.Binding{Keys.Refresh, Keys.Space}
		}
		l.AdditionalFullHelpKeys = func() []key.Binding {
			return []key.Binding{Keys.Refresh, Keys.Space}
		}
	case "drive":
		l.AdditionalShortHelpKeys = func() []key.Binding {
			return []key.Binding{Keys.Delete, Keys.BatchDelete}
		}
		l.AdditionalFullHelpKeys = func() []key.Binding {
			return []key.Binding{Keys.Delete, Keys.BatchDelete}
		}
	case "select":
		l.AdditionalShortHelpKeys = func() []key.Binding {
			return []key.Binding{Keys.Escape}
		}
		l.AdditionalFullHelpKeys = func() []key.Binding {
			return []key.Binding{Keys.Escape}
		}
	}
	return l
}
