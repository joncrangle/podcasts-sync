package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/joncrangle/podcasts-sync/internal"
)

type DebugMsg internal.Debug

func addDebugMsg(title string, description string) tea.Cmd {
	return func() tea.Msg {
		return DebugMsg(internal.Debug{DTitle: title, DDescription: description})
	}
}
