package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/joncrangle/podcasts-sync/internal"
)

type DebugMsg internal.Debug

type ErrMsg struct {
	err error
}

func addDebugMsg(title string, description string) tea.Cmd {
	return func() tea.Msg {
		return DebugMsg(internal.Debug{DTitle: title, DDescription: description})
	}
}

func (e ErrMsg) Error() string {
	if e.err == nil {
		return "unknown error"
	}
	return e.err.Error()
}
