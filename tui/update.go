package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/joncrangle/podcasts-sync/internal"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if cmd := m.handleListUpdates(msg); cmd != nil {
		return m, cmd
	}

	switch msg := msg.(type) {
	case ErrMsg:
		return m.handleError(msg)
	case DebugMsg:
		return m.handleDebug(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, m.updateLayoutDimensions()
	case DrivesPollMsg:
		return m, tea.Batch(getDrives, pollDrivesCmd(5000))
	case DriveUpdatedMsg:
		return m.handleDriveUpdate(msg)
	case DrivePodcastsMsg:
		return m.handleDrivePodcasts(msg)
	case MacPodcastsMsg:
		return m.handleMacPodcasts(msg)
	case FileOpMsg:
		return m.handleFileOp(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd
	}
	return m, nil
}

func (m *Model) handleError(msg ErrMsg) (tea.Model, tea.Cmd) {
	if m.state != normal {
		m.state = normal
	}
	m.errorMsg = msg.Error()
	return m, nil
}

func (m *Model) handleDebug(msg DebugMsg) (tea.Model, tea.Cmd) {
	debug := internal.Debug(msg)
	m.debugMsgs = append(m.debugMsgs, debug)

	items := make([]list.Item, len(m.debugMsgs))
	for i, d := range m.debugMsgs {
		items[i] = internal.Debug{
			DTitle:       d.DTitle,
			DDescription: d.DDescription,
		}
	}
	m.debug.SetItems(items)
	return m, nil
}

func (m *Model) handleListUpdates(msg tea.Msg) tea.Cmd {
	if m.state == transferring || m.state == driveSelection {
		return nil
	}

	var cmd tea.Cmd
	switch m.focusIndex {
	case 0:
		m.macPodcasts, cmd = m.macPodcasts.Update(msg)
	case 1:
		m.drivePodcasts, cmd = m.drivePodcasts.Update(msg)
	}
	return cmd
}

func (m *Model) handleDriveUpdate(msg DriveUpdatedMsg) (tea.Model, tea.Cmd) {
	if m.loading.macPodcasts {
		return m, getDrives
	}

	if internal.USBDrivesEqual(m.drives, msg) || len(msg) == 0 {
		return m, nil
	}

	m.drives = msg
	m.driveSelector.SetItems(m.createDriveItems(msg))
	m.loading.drives = false

	// Set current drive to first drive if it's not set
	if m.currentDrive.Name == "" {
		m.currentDrive = m.drives[0]
		m.loading.drivePodcasts = true
		return m, tea.Sequence(getMacPodcasts, getDrivePodcasts(m.currentDrive, m.podcasts))
	}
	// Handle drive state changes
	found := false
	for _, drive := range m.drives {
		if drive.Name == m.currentDrive.Name && drive.MountPath == m.currentDrive.MountPath {
			found = true
			break
		}
	}
	if !found {
		m.currentDrive = m.drives[0]
		m.drivePodcasts.SetItems(nil)
		m.podcastsDrive = nil
	}

	return m, nil
}

func (m *Model) createDriveItems(drives []internal.USBDrive) []list.Item {
	items := make([]list.Item, len(drives))
	for i, d := range drives {
		items[i] = internal.USBDrive{
			Name:      d.Name,
			MountPath: d.MountPath,
			Folder:    d.Folder,
		}
	}
	return items
}

func (m *Model) handleDrivePodcasts(msg DrivePodcastsMsg) (tea.Model, tea.Cmd) {
	m.podcastsDrive = msg.PodcastsDrive
	m.drivePodcasts.SetItems(m.createPodcastItems(msg.PodcastsDrive))
	m.loading.drivePodcasts = false
	m.loading.macPodcasts = true

	if len(msg.PodcastsDrive) == 0 || len(msg.Podcasts) == 0 {
		return m, nil
	}

	return m, updateMacPodcasts(msg.Podcasts)
}

func (m *Model) handleMacPodcasts(msg MacPodcastsMsg) (tea.Model, tea.Cmd) {
	m.podcasts = msg
	m.macPodcasts.SetItems(m.createPodcastItems(msg))
	m.loading.macPodcasts = false
	return m, m.updateLayoutDimensions()
}

func (m *Model) createPodcastItems(podcasts []internal.PodcastEpisode) []list.Item {
	items := make([]list.Item, len(podcasts))
	for i, p := range podcasts {
		items[i] = internal.PodcastEpisode{
			ZTitle:    p.ZTitle,
			ShowName:  p.ShowName,
			FilePath:  p.FilePath,
			Published: p.Published,
			Selected:  p.Selected,
			FileSize:  p.FileSize,
			OnDrive:   p.OnDrive,
			Duration:  p.Duration,
		}
	}
	return items
}

func (m *Model) handleFileOp(msg FileOpMsg) (tea.Model, tea.Cmd) {
	switch msg.Operation {
	case "sync":
		return m.handleSync(msg)
	case "delete":
		m.state = normal
		m.loading.drivePodcasts = true
		return m, getDrivePodcasts(m.currentDrive, m.podcasts)
	default:
		m.state = normal
		return m, nil
	}
}

func (m *Model) handleSync(msg FileOpMsg) (tea.Model, tea.Cmd) {
	if m.state != transferring {
		return m, nil
	}

	if msg.Msg.Complete {
		for i := range m.podcasts {
			m.podcasts[i].Selected = false
		}
		for i := range m.macPodcasts.Items() {
			if ep, ok := m.macPodcasts.Items()[i].(internal.PodcastEpisode); ok {
				ep.Selected = false
				m.macPodcasts.Items()[i] = ep
			}
		}
		m.state = normal
		m.progress.SetPercent(0)
		m.transferProgress = internal.TransferProgress{}
		m.loading.drivePodcasts = true
		var cmds []tea.Cmd
		cmds = append(cmds, getDrivePodcasts(m.currentDrive, m.podcasts))
		if m.dbgEnabled {
			cmds = append(cmds, addDebugMsg("FileOpMsg", fmt.Sprintf("Operation: %s, Complete: %t, Error: %v", msg.Operation, msg.Msg.Complete, msg.Msg.Error)))
		}
		return m, tea.Batch(cmds...)
	}

	m.transferProgress = msg.Msg.Progress

	var cmds []tea.Cmd
	cmds = append(cmds, m.progress.SetPercent(m.transferProgress.CurrentProgress), m.syncManager.wait())

	if m.dbgEnabled {
		cmds = append(cmds, addDebugMsg("FileOpMsg", fmt.Sprintf("Operation: %s, BytesTransferred: %.1f, Error: %v", msg.Operation, float64(msg.Msg.Progress.BytesTransferred), msg.Msg.Error)))
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) handleDeletePodcasts() (tea.Model, tea.Cmd) {
	var selected []internal.PodcastEpisode
	for _, p := range m.podcastsDrive {
		if p.Selected {
			selected = append(selected, p)
		}
	}
	return m, deletePodcasts(selected)
}

func (m *Model) handlePodcastSelection() (tea.Model, tea.Cmd) {
	var (
		listToUpdate *list.Model
		sourceList   *[]internal.PodcastEpisode
	)

	switch m.focusIndex {
	case 0:
		listToUpdate = &m.macPodcasts
		sourceList = &m.podcasts
	case 1:
		listToUpdate = &m.drivePodcasts
		sourceList = &m.podcastsDrive
	}

	if listToUpdate != nil && sourceList != nil {
		if selectedItem := listToUpdate.SelectedItem(); selectedItem != nil {
			if episode, ok := selectedItem.(internal.PodcastEpisode); ok {
				episode.Selected = !episode.Selected
				items := listToUpdate.Items()
				for j, item := range items {
					if ep, ok := item.(internal.PodcastEpisode); ok && ep.FilePath == episode.FilePath {
						items[j] = episode
						break
					}
				}
				listToUpdate.SetItems(items)

				for k, podcast := range *sourceList {
					if podcast.FilePath == episode.FilePath {
						(*sourceList)[k] = episode
						break
					}
				}
			}
		}
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		if m.state == transferring {
			return m, tea.Sequence(m.syncManager.cancel(), tea.Quit)
		}
		return m, tea.Quit
	case key.Matches(msg, keys.Escape):
		if m.state == transferring {
			for i := range m.podcasts {
				m.podcasts[i].Selected = false
			}
			for i := range m.macPodcasts.Items() {
				if ep, ok := m.macPodcasts.Items()[i].(internal.PodcastEpisode); ok {
					ep.Selected = false
					m.macPodcasts.Items()[i] = ep
				}
			}
			m.state = normal
			m.progress.SetPercent(0)
			m.loading.drivePodcasts = true
			return m, tea.Sequence(m.syncManager.cancel(), getDrivePodcasts(m.currentDrive, m.podcasts))
		}
		m.state = normal
		return m, nil
	case key.Matches(msg, keys.SelectDrive):
		if m.state != transferring {
			m.state = driveSelection
		}
		return m, nil
	case key.Matches(msg, keys.Debug):
		if m.dbgEnabled && m.state != transferring {
			m.state = debug
		}
		return m, nil
	case key.Matches(msg, keys.Progress):
		if m.dbgEnabled {
			m.state = transferring
		}
		return m, nil
	case key.Matches(msg, keys.Up):
		if m.state == driveSelection {
			m.driveSelector.CursorUp()
		}
		if m.state == debug {
			m.debug.CursorUp()
		}
		if m.focusIndex == 0 {
			m.macPodcasts.CursorUp()
		} else {
			m.drivePodcasts.CursorUp()
		}
		return m, nil
	case key.Matches(msg, keys.Down):
		if m.state == driveSelection {
			m.driveSelector.CursorDown()
		}
		if m.state == debug {
			m.debug.CursorDown()
		}
		if m.focusIndex == 0 {
			m.macPodcasts.CursorDown()
		} else {
			m.drivePodcasts.CursorDown()
		}
		return m, nil
	case key.Matches(msg, keys.Left):
		m.focusIndex = 0
	case key.Matches(msg, keys.Right):
		m.focusIndex = 1
		return m, nil
	case key.Matches(msg, keys.Tab):
		m.focusIndex = (m.focusIndex + 1) % 2
		return m, nil
	case key.Matches(msg, confirmKeys.No):
		m.state = normal
		return m, nil
	case key.Matches(msg, keys.Enter):
		if m.state == driveSelection && len(m.drives) > 0 {
			m.currentDrive = m.driveSelector.SelectedItem().(internal.USBDrive)
			m.loading.drivePodcasts = true
			m.state = normal
			return m, tea.Sequence(getMacPodcasts, getDrivePodcasts(m.currentDrive, m.podcasts))
		}
		if m.state == confirm {
			return m.handleDeletePodcasts()
		}
		return m, nil
	case key.Matches(msg, confirmKeys.Yes):
		if m.state == confirm {
			return m.handleDeletePodcasts()
		}
		return m, nil
	case key.Matches(msg, keys.Refresh):
		m.loading.macPodcasts = true
		m.loading.drivePodcasts = true
		m.errorMsg = ""
		return m, tea.Sequence(getMacPodcasts, getDrivePodcasts(m.currentDrive, m.podcasts))
	case key.Matches(msg, keys.Space):
		return m.handlePodcastSelection()
	case key.Matches(msg, keys.Sync):
		if m.state != transferring {
			anySelected := false
			for i := range m.podcasts {
				if m.podcasts[i].Selected {
					anySelected = true
					break
				}
			}
			if anySelected {
				var selected []internal.PodcastEpisode
				for _, p := range m.podcasts {
					if p.Selected {
						selected = append(selected, p)
					}
				}
				m.state = transferring
				return m, m.syncManager.start(selected, m.currentDrive)
			}
		}
		return m, nil
	case key.Matches(msg, keys.SyncAll):
		if m.state != transferring {
			for i := range m.podcasts {
				m.podcasts[i].Selected = true
			}
			m.state = transferring
			return m, m.syncManager.start(m.podcasts, m.currentDrive)
		}
		return m, nil
	case key.Matches(msg, keys.Delete):
		anySelected := false
		for i := range m.podcastsDrive {
			if m.podcastsDrive[i].Selected {
				anySelected = true
				break
			}
		}
		if anySelected {
			m.state = confirm
		}
		return m, nil
	case key.Matches(msg, keys.DeleteAll):
		if len(m.podcastsDrive) == 0 {
			return m, nil
		}
		for i := range m.podcastsDrive {
			m.podcastsDrive[i].Selected = true
		}
		m.state = confirm
		return m, nil
	}
	return m, nil
}
