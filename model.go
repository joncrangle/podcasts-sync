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
	keys             ui.KeyMap
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
	return model{
		macPodcasts:   ui.CreateList("Mac Podcasts", "mac"),
		drivePodcasts: ui.CreateList("Drive Podcasts", "drive"),
		driveSelector: ui.CreateList("USB Drives", "select"),
		keys:          ui.Keys,
		help:          ui.CreateHelp(),
		progress:      ui.CreateProgress(),
		spinner:       ui.CreateSpinner(),
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
		topMargin := 4
		bottomMargin := 4
		progressHeight := 6

		contentWidth := m.width - horizontalMargins
		contentHeight := m.height - topMargin - progressHeight - bottomMargin
		listHeight := contentHeight

		if m.transferring {
			listHeight -= progressHeight
			m.progress.Width = contentWidth
		}

		listWidth := contentWidth / 2

		// m.errorMsg = fmt.Sprintf("w: %d, h: %d, cw: %d, ch: %d, lw: %d, lh: %d", m.width, m.height, contentWidth, contentHeight, listWidth, listHeight)

		m.macPodcasts.Styles.NoItems = lipgloss.NewStyle().Width(listWidth)
		m.macPodcasts.SetSize(listWidth, listHeight)
		m.drivePodcasts.Styles.NoItems = lipgloss.NewStyle().Width(listWidth)
		m.drivePodcasts.SetSize(listWidth, listHeight)

		if m.state == stateChoosingDrive {
			m.driveSelector.SetSize(60, 10) // Popup size
		} else {
			m.driveSelector.SetSize(contentWidth, topMargin)
		}

		m.help.Width = contentWidth

		return m, nil

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
		case key.Matches(msg, ui.Keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, ui.Keys.Escape):
			if m.state == stateChoosingDrive {
				m.state = stateNormal
			}
			return m, nil

		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll

		case key.Matches(msg, ui.Keys.SelectDrive):
			m.state = stateChoosingDrive
			return m, nil

		case key.Matches(msg, ui.Keys.Refresh):
			return m, nil

		case key.Matches(msg, ui.Keys.Left):
			m.focusIndex = 0
			return m, nil

		case key.Matches(msg, ui.Keys.Right):
			m.focusIndex = 1
			return m, nil

		case key.Matches(msg, ui.Keys.Tab):
			m.focusIndex = (m.focusIndex + 1) % 2
			return m, nil

		//TODO: handle all keys
		// and handle escape to exit drive selection
		case key.Matches(msg, ui.Keys.Space):
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

		case key.Matches(msg, ui.Keys.Sync):
			if !m.transferring {
				m.transferring = true
				return m, startSync(m.macPodcasts.Items(), m.currentDrive)
			}

		}

		if m.state == stateChoosingDrive {
			if key.Matches(msg, ui.Keys.Enter) {
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
	s += ui.DriveStyle(driveInfo) + "\n\n"

	if m.errorMsg != "" {
		s += ui.ErrorStyle.Render(m.errorMsg) + "\n\n"
	}

	// Podcast lists
	if m.focusIndex == 0 {
		ui.LeftStyle = ui.FocusedStyle
		ui.RightStyle = ui.NoStyleStyle
	} else {
		ui.LeftStyle = ui.NoStyleStyle
		ui.RightStyle = ui.FocusedStyle
	}

	macList := ui.LeftStyle.Render(m.macPodcasts.View())
	driveList := ui.RightStyle.Render(m.drivePodcasts.View())
	lists := lipgloss.JoinHorizontal(lipgloss.Top, macList, strings.Repeat(" ", 4), driveList)
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
		s += ui.ProgressStyle(progressInfo)
	}

	// Drive selector popup
	if m.state == stateChoosingDrive {
		popup := ui.PopupStyle(m.driveSelector.View())

		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			popup)
	}

	//TODO: status message

	// Help
	helpView := m.help.View(m.keys)
	height := 8 - strings.Count(s, "\n") - strings.Count(helpView, "\n")
	s += lipgloss.Place(m.width, height, lipgloss.Center, lipgloss.Bottom, helpView)

	return ui.AppStyle.Render(s)
}
