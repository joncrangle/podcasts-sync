package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/joncrangle/podcasts-sync/ui"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type state int

const (
	stateNormal state = iota
	stateChoosingDrive
)

type model struct {
	state            state
	width            int
	height           int
	macPodcasts      list.Model
	drivePodcasts    list.Model
	driveSelector    list.Model
	help             help.Model
	keys             keyMap
	popupKeys        popupKeyMap
	progress         progress.Model
	currentDrive     USBDrive
	drives           []USBDrive
	focusIndex       int // 0 = mac list, 1 = drive list
	transferring     bool
	transferProgress TransferProgress
	spinner          spinner.Model
	statusMsg        string
	errorMsg         string
}

func initialModel() model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle
	driveList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	driveList.SetShowStatusBar(false)
	driveList.SetFilteringEnabled(false)
	driveList.Title = "USB Drives"

	return model{
		macPodcasts:   ui.newPodcastList("Local Podcasts"),
		drivePodcasts: ui.newPodcastList("Drive Podcasts"),
		driveSelector: driveList,
		keys:          keys,
		help:          help.New(),
		popupKeys:     popupKeys,
		progress:      progress.New(progress.WithDefaultGradient()),
		spinner:       sp,
		focusIndex:    0,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		detectDrives,
		loadLocalPodcasts,
		m.spinner.Tick,
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case errMsg:
		m.errorMsg = msg.Error()

	case tea.WindowSizeMsg:
		//TODO: size things better
		m.width = msg.Width
		m.height = msg.Height

		horizontalMargins := 4
		listWidth := (msg.Width - horizontalMargins) / 2
		listHeight := msg.Height - 15 // Space for drive selector, progress bar and help

		m.macPodcasts.SetSize(listWidth, listHeight)
		m.drivePodcasts.SetSize(listWidth, listHeight)
		m.driveSelector.SetSize(msg.Width-horizontalMargins, 10)
		m.progress.Width = msg.Width - horizontalMargins
		m.help.Width = msg.Width

		if m.state == stateChoosingDrive {
			m.driveSelector.SetSize(60, 10) // Popup size
		}

	case driveUpdatedMsg:
		items := make([]list.Item, len(msg))
		for i, d := range msg {
			items[i] = USBDrive{Name: d.Name, MountPath: d.MountPath}
		}
		m.drives = msg
		m.driveSelector.SetItems(items)
		cmds = append(cmds, detectDrives)

	//FIXME: get this working
	case macPodcastsMsg:
		items := make([]list.Item, len(msg))
		for i, p := range msg {
			items[i] = PodcastEpisode{
				ZTitle:         p.ZTitle,
				ShowName:       p.ShowName,
				FilePath:       p.FilePath,
				Published:      p.Published,
				Selected:       false,
				FileSize:       p.FileSize,
				OnDrive:        false,
				DriveState:     0,
				Duration:       p.Duration,
				DateDownloaded: p.DateDownloaded,
				Progress:       0,
				MD5Hash:        p.MD5Hash,
			}
		}
		m.macPodcasts.SetItems(items)
		cmds = append(cmds, loadLocalPodcasts)

	case fileOpMsg:
		m.progress.SetPercent(msg.Progress)
		m.statusMsg = fmt.Sprintf("%s %s: %.0f%%",
			msg.Operation,
			msg.Filename,
			msg.Progress*100)
		return m, nil

	case fileOpCompleteMsg:
		if msg.Error != nil {
			m.statusMsg = fmt.Sprintf("Error during %s: %v", msg.Operation, msg.Error)
		} else {
			m.statusMsg = fmt.Sprintf("%s complete!", msg.Operation)
			// Refresh drive contents
			return m, loadDrivePodcasts(m.currentDrive)
		}
		m.transferring = false
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, keys.Escape):
			if m.state == stateChoosingDrive {
				m.state = stateNormal
			}
			return m, nil

		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll

		case key.Matches(msg, keys.SelectDrive):
			m.state = stateChoosingDrive
			return m, nil

		case key.Matches(msg, keys.Tab):
			m.focusIndex = (m.focusIndex + 1) % 2
			return m, nil

		//TODO: handle all keys
		// and handle escape to exit drive selection
		case key.Matches(msg, keys.Space):
			// Toggle selection of current item
			if m.focusIndex == 0 {
				if i, ok := m.macPodcasts.SelectedItem().(PodcastEpisode); ok {
					i.Selected = !i.Selected
					items := m.macPodcasts.Items()
					for j, item := range items {
						if item.(PodcastEpisode).ZTitle == i.ZTitle {
							items[j] = i
						}
					}
					m.macPodcasts.SetItems(items)
				}
			}

		case key.Matches(msg, keys.Sync):
			if !m.transferring {
				m.transferring = true
				return m, startSync(m.macPodcasts.Items(), m.currentDrive)
			}

		}

		if m.state == stateChoosingDrive {
			if key.Matches(msg, keys.Enter) {
				if i := m.driveSelector.SelectedItem(); i != nil {
					m.currentDrive = i.(USBDrive)
					m.state = stateNormal
					return m, loadDrivePodcasts(m.currentDrive)
				}
			}
		}

	case syncProgressMsg:
		m.progress.SetPercent(float64(msg))
		if msg >= 1.0 {
			m.transferring = false
			m.statusMsg = "Transfer complete!"
			return m, nil
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Handle list updates
	if m.focusIndex == 0 {
		newListModel, cmd := m.macPodcasts.Update(msg)
		m.macPodcasts = newListModel
		cmds = append(cmds, cmd)
	} else {
		newListModel, cmd := m.drivePodcasts.Update(msg)
		m.drivePodcasts = newListModel
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var s string

	// Drive selector
	driveInfo := fmt.Sprintf("Current Drive: %s", m.currentDrive.Name)
	if m.currentDrive.Name == "" {
		if len(m.drives) > 0 {
			m.currentDrive = m.drives[0]
			driveInfo = fmt.Sprintf("Current Drive: %s", m.currentDrive.Name)
		} else {
			driveInfo = "No drives detected"
		}
	}
	s += driveStyle.Render(driveInfo) + "\n\n"

	if m.errorMsg != "" {
		s += errorStyle.Render(m.errorMsg) + "\n\n"
	}

	// Podcast lists
	if m.focusIndex == 0 {
		leftStyle = focusedStyle
		rightStyle = noStyleStyle
	} else {
		leftStyle = noStyleStyle
		rightStyle = focusedStyle
	}

	macList := leftStyle.Render(m.macPodcasts.View())
	driveList := rightStyle.Render(m.drivePodcasts.View())
	lists := listStyle(lipgloss.JoinHorizontal(lipgloss.Top, macList, driveList))
	s += lists + "\n\n"

	// Progress bar
	if m.transferring {
		progressInfo := fmt.Sprintf(
			"\nTransferring: %s\n"+
				"Progress: %.1f%% (%d/%d files)\n"+
				"Speed: %.1f MB/s\n"+
				"Remaining: %s\n"+
				"Transferred: %s / %s",
			m.transferProgress.CurrentFile,
			m.transferProgress.CurrentProgress*100,
			m.transferProgress.FilesDone,
			m.transferProgress.TotalFiles,
			m.transferProgress.Speed/1024/1024,
			m.transferProgress.TimeRemaining.Round(time.Second),
			formatBytes(m.transferProgress.BytesTransferred),
			formatBytes(m.transferProgress.TotalBytes),
		)
		s += progressStyle.Render(progressInfo)
	}

	// Drive selector popup
	if m.state == stateChoosingDrive {
		driveSelectorView := m.driveSelector.View()
		helpView := m.help.View(m.popupKeys)

		popup := popupStyle.Render(lipgloss.JoinVertical(lipgloss.Top, driveSelectorView, helpView))

		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			popup)
	}

	helpView := m.help.View(m.keys)
	height := 8 - strings.Count(s, "\n") - strings.Count(helpView, "\n")
	s += helpStyle.Render(lipgloss.Place(m.width, height, lipgloss.Center, lipgloss.Bottom, helpView))

	return s
}
