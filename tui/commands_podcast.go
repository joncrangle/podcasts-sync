package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/joncrangle/podcasts-sync/internal"
)

type ErrMsg struct {
	err error
}

func (e ErrMsg) Error() string {
	if e.err == nil {
		return "unknown error"
	}
	return e.err.Error()
}

type MacPodcastsMsg []internal.PodcastEpisode

func getMacPodcasts() tea.Msg {
	podcasts, err := internal.LoadMacPodcasts()
	if err != nil {
		return ErrMsg{err}
	}

	podcasts, err = internal.LoadLocalPodcasts(podcasts)
	if err != nil {
		return ErrMsg{err}
	}

	return MacPodcastsMsg(podcasts)
}

func updateMacPodcasts(podcasts []internal.PodcastEpisode) tea.Cmd {
	return func() tea.Msg {
		podcasts, err := internal.LoadLocalPodcasts(podcasts)
		if err != nil {
			return ErrMsg{err}
		}
		return MacPodcastsMsg(podcasts)
	}
}
